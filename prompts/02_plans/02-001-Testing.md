# 動作確認計画書

## 1. 問題の特定

### 1.1 発見した問題点

コードベースを詳細に調査した結果、**アバター応答が生成されない根本原因**を特定しました：

`backend/internal/api/conversation.go` の `SendMessage` ハンドラ（190-243行目）において：

1. ユーザーメッセージはDBに正しく保存されている
2. OpenAI Threadにもメッセージが追加されている
3. **しかし、アバターに応答を生成させる処理が完全に欠如している**

具体的には：
- `assistant.CreateRun()` が呼び出されていない（LLMに応答を生成させる処理）
- `logic.SelectResponders()` が使用されていない（どのアバターが応答すべきか判定する処理）
- 生成された応答をDBに保存する処理がない

### 1.2 現在のメッセージフロー

```
[ユーザー入力] 
    ↓
[Frontend: api.sendMessage()]
    ↓ POST /api/conversations/{id}/messages
[Backend: ConversationHandler.SendMessage()]
    ↓
[DB: CreateMessage() - ユーザーメッセージ保存]
    ↓
[Assistant: CreateMessage() - OpenAI Threadへ追加]
    ↓
[終了] ← ここでアバター応答生成処理が欠如
```

---

## 2. ログ追加計画

問題の詳細調査と今後のデバッグのため、以下の箇所にログを追加します。

### 2.1 HTTPミドルウェア層（リクエストログ）

**ファイル**: `backend/internal/api/router.go`

**追加内容**:
- 全HTTPリクエストのログ（メソッド、パス、ステータスコード、処理時間）
- CORSプリフライトリクエストのログ

**実装方法**:
```go
// ServeHTTP メソッド内にログ追加
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    start := time.Now()
    log.Printf("[HTTP] %s %s started", req.Method, req.URL.Path)
    
    // ... 既存処理 ...
    
    log.Printf("[HTTP] %s %s completed in %v", req.Method, req.URL.Path, time.Since(start))
}
```

### 2.2 APIハンドラ層

**ファイル**: `backend/internal/api/conversation.go`

**追加箇所と内容**:

| 関数 | ログ追加箇所 | ログ内容 |
|------|-------------|---------|
| `SendMessage` | 関数開始時 | 会話ID、受信メッセージ内容 |
| `SendMessage` | DB保存後 | 保存されたメッセージID |
| `SendMessage` | OpenAI Thread追加前 | Thread ID |
| `SendMessage` | OpenAI Thread追加後/エラー時 | 成功/失敗メッセージ |
| `SendMessage` | 関数終了時 | 処理完了メッセージ |
| `GetMessages` | 関数開始時 | 会話ID |
| `Create` | 関数開始/終了時 | タイトル、アバターID |

### 2.3 アシスタント層（OpenAI API通信）

**ファイル**: `backend/internal/assistant/thread.go`

**追加箇所と内容**:

| 関数 | ログ追加箇所 | ログ内容 |
|------|-------------|---------|
| `CreateThread` | 開始時 | Thread作成開始 |
| `CreateThread` | 成功時 | 作成されたThread ID |
| `CreateMessage` | 開始時 | Thread ID、メッセージ内容（先頭50文字） |
| `CreateMessage` | 成功時 | 作成されたMessage ID |
| `CreateRun` | 開始時 | Thread ID、Assistant ID |
| `CreateRun` | 成功時 | Run ID、Status |
| `WaitForRun` | ポーリング毎 | Run ID、現在のStatus |

**ファイル**: `backend/internal/assistant/client.go`

| 関数 | ログ追加箇所 | ログ内容 |
|------|-------------|---------|
| `CreateAssistant` | 開始時 | アシスタント名 |
| `CreateAssistant` | 成功時 | 作成されたAssistant ID |
| `handleError` | エラー時 | HTTPステータス、エラー本文 |

### 2.4 DB層

**ファイル**: `backend/internal/db/conversation.go`

**追加箇所と内容**:

| 関数 | ログ追加箇所 | ログ内容 |
|------|-------------|---------|
| `CreateMessage` | 成功時 | 会話ID、送信者タイプ、メッセージID |
| `GetConversationAvatars` | 成功時 | 会話ID、取得されたアバター数 |

---

## 3. 実装手順

### Phase 1: ログ追加（調査用）

1. **`router.go`** にHTTPリクエストログミドルウェアを追加
2. **`conversation.go`** (api) に詳細ログを追加
3. **`thread.go`** にOpenAI API通信ログを追加
4. **`client.go`** にエラーログ強化を追加
5. **`conversation.go`** (db) にDB操作ログを追加

### Phase 2: 動作確認

1. サーバーを再起動
2. Web UIからメッセージを送信
3. ターミナルのログを確認し、処理フローを把握
4. 問題箇所を特定

### Phase 3: 問題修正（別タスク）

ログ確認後、アバター応答生成機能の実装が必要な場合：
- `SendMessage` にアバター応答生成ロジックを追加
- `CreateRun`、`WaitForRun` の呼び出し
- 応答メッセージのDB保存

---

## 4. ログフォーマット

一貫性のあるログフォーマットを使用：

```
[カテゴリ] メッセージ key=value key=value ...
```

例：
```
[HTTP] Request started method=POST path=/api/conversations/1/messages
[API] SendMessage called conversation_id=1 content_length=50
[Assistant] CreateMessage sending thread_id=thread_xxx content_preview="こんにちは..."
[DB] Message created conversation_id=1 message_id=5 sender_type=user
[API] SendMessage completed conversation_id=1 duration=150ms
```

---

## 5. 想定される次のステップ

ログを追加して確認した後、以下のいずれかの対応が必要と予想されます：

### 5.1 アバター応答機能が未実装の場合

`SendMessage` に以下のロジックを追加：

1. 会話に参加しているアバターを取得
2. メンションを解析して応答すべきアバターを特定
3. 各アバターに対して `CreateRun` を実行
4. `WaitForRun` で完了を待機
5. 応答メッセージを取得してDBに保存
6. フロントエンドに応答を返す（またはポーリング用に保存のみ）

### 5.2 OpenAI APIキーが未設定の場合

`settings/secrets/openai.yaml` の設定を確認し、APIキーを正しく設定

### 5.3 その他のエラーがある場合

ログを分析して具体的なエラー原因を特定

