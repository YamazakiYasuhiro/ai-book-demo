# アバター応答生成機能 追加計画書

## 1. 概要

### 1.1 目的

ユーザーがメッセージを送信した際に、チャットルームに参加しているアバターが自動的に応答を生成する機能を実装する。

### 1.2 現状の問題

`SendMessage` ハンドラでユーザーメッセージは保存されるが、アバターからの応答を生成する処理が欠如している。

### 1.3 期待する動作

```
[ユーザー] 「こんにちは」
    ↓
[システム] メッセージ保存 → アバター選定 → LLM呼び出し → 応答保存
    ↓
[アバター: おかあさん] 「こんにちは！今日はどうしたの？」
```

---

## 2. 技術設計

### 2.1 処理フロー

```
┌─────────────────────────────────────────────────────────────────────┐
│                         SendMessage Handler                          │
├─────────────────────────────────────────────────────────────────────┤
│ 1. ユーザーメッセージをDBに保存                                      │
│ 2. OpenAI Threadにメッセージを追加                                   │
│ 3. ★ 新規: 応答すべきアバターを選定                                 │
│ 4. ★ 新規: 各アバターに対してLLM応答を生成                          │
│ 5. ★ 新規: 応答メッセージをDBに保存                                 │
│ 6. ★ 新規: 応答メッセージをレスポンスに含める                       │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 アバター選定ロジック

既存の `logic.SelectResponders()` を活用：

| ケース | 動作 |
|--------|------|
| `@おかあさん` のようなメンションあり | メンションされたアバターのみ応答 |
| メンションなし | 全参加アバターが応答対象（最初の1名のみ応答） |
| メンションが無効なアバター名 | 全参加アバターが応答対象（フォールバック） |

### 2.3 OpenAI Assistants API フロー

```
┌──────────────────────────────────────────────────────────────────────┐
│ アバター応答生成フロー（各アバターごと）                              │
├──────────────────────────────────────────────────────────────────────┤
│ 1. CreateRun(threadID, assistantID) → Run作成                        │
│ 2. WaitForRun(threadID, runID, timeout) → 完了待機                   │
│ 3. ListMessages(threadID) → 最新メッセージ取得                       │
│ 4. 応答テキストを抽出                                                │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 3. 実装詳細

### 3.1 変更対象ファイル

| ファイル | 変更内容 |
|----------|----------|
| `internal/api/conversation.go` | `SendMessage` にアバター応答生成ロジックを追加 |
| `internal/assistant/thread.go` | `ListMessages` にログ追加、`GetLatestAssistantMessage` 新規追加 |

### 3.2 SendMessage 関数の変更

**変更前（288-291行目）:**
```go
// NOTE: Avatar response generation is NOT implemented here
// This is why avatars don't respond to messages
log.Printf("[API] WARNING: Avatar response generation is NOT implemented - avatars will not respond")
```

**変更後:**
```go
// Generate avatar responses
avatarResponses := h.generateAvatarResponses(conv, avatars, req.Content)
```

### 3.3 新規関数: generateAvatarResponses

```go
// generateAvatarResponses generates responses from avatars
// Returns a slice of messages created by avatars
func (h *ConversationHandler) generateAvatarResponses(
    conv *models.Conversation,
    avatars []models.Avatar,
    userContent string,
) []MessageResponse {
    if h.assistant == nil || conv.ThreadID == "" {
        log.Printf("[API] Skipping avatar response: assistant not configured")
        return nil
    }

    if len(avatars) == 0 {
        log.Printf("[API] Skipping avatar response: no avatars in conversation")
        return nil
    }

    // Select which avatars should respond
    responders := logic.SelectResponders(userContent, avatars)
    log.Printf("[API] Selected responders count=%d", len(responders))

    // For now, only first responder generates a response (to avoid multiple simultaneous runs)
    if len(responders) == 0 {
        return nil
    }

    responder := responders[0]
    log.Printf("[API] Generating response from avatar name=%q assistant_id=%s", 
        responder.Name, responder.OpenAIAssistantID)

    // Check if avatar has OpenAI Assistant ID
    if responder.OpenAIAssistantID == "" {
        log.Printf("[API] Avatar has no OpenAI assistant ID, skipping avatar_id=%d", responder.ID)
        return nil
    }

    // Create a run for the avatar to respond
    run, err := h.assistant.CreateRun(conv.ThreadID, responder.OpenAIAssistantID)
    if err != nil {
        log.Printf("[API] Failed to create run err=%v", err)
        return nil
    }
    log.Printf("[API] Run created run_id=%s", run.ID)

    // Wait for run to complete (30 second timeout)
    completedRun, err := h.assistant.WaitForRun(conv.ThreadID, run.ID, 30*time.Second)
    if err != nil {
        log.Printf("[API] Run failed or timed out err=%v", err)
        return nil
    }
    log.Printf("[API] Run completed run_id=%s status=%s", completedRun.ID, completedRun.Status)

    // Get the latest assistant message
    responseContent, err := h.assistant.GetLatestAssistantMessage(conv.ThreadID)
    if err != nil {
        log.Printf("[API] Failed to get assistant message err=%v", err)
        return nil
    }
    log.Printf("[API] Got assistant response content_length=%d", len(responseContent))

    // Save avatar message to database
    avatarID := responder.ID
    avatarMsg, err := h.db.CreateMessage(conv.ID, models.SenderTypeAvatar, &avatarID, responseContent)
    if err != nil {
        log.Printf("[API] Failed to save avatar message err=%v", err)
        return nil
    }
    log.Printf("[API] Avatar message saved message_id=%d avatar_id=%d", avatarMsg.ID, avatarID)

    return []MessageResponse{{
        ID:         avatarMsg.ID,
        SenderType: string(avatarMsg.SenderType),
        SenderID:   avatarMsg.SenderID,
        SenderName: responder.Name,
        Content:    avatarMsg.Content,
        CreatedAt:  avatarMsg.CreatedAt.Format(time.RFC3339),
    }}
}
```

### 3.4 新規関数: GetLatestAssistantMessage

`internal/assistant/thread.go` に追加:

```go
// GetLatestAssistantMessage retrieves the most recent assistant message from a thread
func (c *Client) GetLatestAssistantMessage(threadID string) (string, error) {
    log.Printf("[Assistant] GetLatestAssistantMessage started thread_id=%s", threadID)

    messages, err := c.ListMessages(threadID)
    if err != nil {
        log.Printf("[Assistant] GetLatestAssistantMessage failed: list messages err=%v", err)
        return "", err
    }

    // Messages are returned in reverse chronological order
    // Find the first assistant message
    for _, msg := range messages {
        if msg.Role == "assistant" && len(msg.Content) > 0 {
            for _, content := range msg.Content {
                if content.Type == "text" && content.Text != nil {
                    log.Printf("[Assistant] GetLatestAssistantMessage found message_id=%s", msg.ID)
                    return content.Text.Value, nil
                }
            }
        }
    }

    log.Printf("[Assistant] GetLatestAssistantMessage: no assistant message found")
    return "", fmt.Errorf("no assistant message found in thread")
}
```

### 3.5 レスポンス形式の変更

**現状のレスポンス:**
```json
{
    "id": 7,
    "sender_type": "user",
    "content": "こんにちは",
    "created_at": "2025-12-05T06:51:36Z"
}
```

**変更後のレスポンス:**
```json
{
    "user_message": {
        "id": 7,
        "sender_type": "user",
        "content": "こんにちは",
        "created_at": "2025-12-05T06:51:36Z"
    },
    "avatar_responses": [
        {
            "id": 8,
            "sender_type": "avatar",
            "sender_id": 1,
            "sender_name": "おかあさん",
            "content": "こんにちは！今日はどうしたの？",
            "created_at": "2025-12-05T06:51:38Z"
        }
    ]
}
```

### 3.6 新規レスポンス構造体

```go
// SendMessageResponse represents the response for sending a message
type SendMessageResponse struct {
    UserMessage     MessageResponse   `json:"user_message"`
    AvatarResponses []MessageResponse `json:"avatar_responses,omitempty"`
}
```

---

## 4. 実装手順

### Phase 1: バックエンド実装

| 順番 | タスク | ファイル |
|------|--------|----------|
| 1 | `GetLatestAssistantMessage` 関数を追加 | `thread.go` |
| 2 | `SendMessageResponse` 構造体を追加 | `conversation.go` |
| 3 | `generateAvatarResponses` 関数を追加 | `conversation.go` |
| 4 | `SendMessage` を変更してアバター応答を生成 | `conversation.go` |
| 5 | レスポンス形式を新構造体に変更 | `conversation.go` |

### Phase 2: フロントエンド対応

| 順番 | タスク | ファイル |
|------|--------|----------|
| 1 | APIレスポンス型を更新 | `services/api.ts` |
| 2 | `sendMessage` 呼び出し後の処理を更新 | `context/AppContext.tsx` |

### Phase 3: 動作確認

1. Docker Compose で再ビルド
2. Web UIでメッセージ送信
3. ログでアバター応答生成を確認
4. 応答がUIに表示されることを確認

---

## 5. エラーハンドリング

| エラーケース | 対応 |
|-------------|------|
| OpenAI APIキーが未設定 | アバター応答をスキップ、ユーザーメッセージのみ保存 |
| アバターにAssistant IDがない | そのアバターをスキップ |
| Run がタイムアウト | エラーログを出力、応答なしで続行 |
| Run が失敗 | エラーログを出力、応答なしで続行 |
| メッセージ取得失敗 | エラーログを出力、応答なしで続行 |

**設計方針:** アバター応答生成が失敗しても、ユーザーメッセージの保存は成功させる。

---

## 6. 将来の拡張（今回は実装しない）

- [ ] 複数アバターの同時応答（並列Run実行）
- [ ] アバター間の会話チェーン（DiscussionMode活用）
- [ ] ストリーミングレスポンス
- [ ] 応答生成の非同期化（WebSocket/SSE）

---

## 7. テスト計画

### 7.1 手動テスト

1. **基本動作**: メッセージ送信 → アバター応答が表示される
2. **メンション**: `@おかあさん` → おかあさんのみが応答
3. **複数アバター**: 2人参加 → 最初の1人が応答
4. **エラー時**: APIキーなし → ユーザーメッセージのみ表示

### 7.2 ログ確認項目

```
[API] Selected responders count=1
[API] Generating response from avatar name="おかあさん" assistant_id=asst_xxx
[API] Run created run_id=run_xxx
[API] Run completed run_id=run_xxx status=completed
[API] Got assistant response content_length=50
[API] Avatar message saved message_id=8 avatar_id=1
```

---

## 8. 見積もり

| フェーズ | 作業時間 |
|---------|----------|
| Phase 1: バックエンド | 約20分 |
| Phase 2: フロントエンド | 約10分 |
| Phase 3: 動作確認 | 約10分 |
| **合計** | **約40分** |

