# マルチスレッドアバターモデル 実装計画書

## 1. 概要

### 1.1 目的

`01-002-GoroutineAvators.md` に記載された要件に基づき、複数のアバターがそれぞれ独立したGoroutineとして動作し、定期的に会話を監視して自律的に応答を判断するシステムを実装する。

### 1.2 現状の課題

| 課題 | 現在の実装 | 目標 |
|------|-----------|------|
| 単一応答の制限 | 同期的に1人のアバターのみ応答 | 複数アバターが自律的に応答判断 |
| 能動的な会話の欠如 | 受動的にしか発言できない | 定期監視による自発的参加 |
| メンションの制限 | `@[a-zA-Z0-9_]+` のみ | Unicode対応（日本語名等） |
| 固定的なアバター参加 | 作成時のみ選択可能 | 動的な参加/退室が可能 |
| リソース管理不在 | Goroutine管理機構なし | ライフサイクル管理 |

### 1.3 スコープ

**実装する機能:**
- WatcherManager による Goroutine ライフサイクル管理
- AvatarWatcher による会話監視と応答判断
- メンションのマルチバイト対応
- アバター動的参加/退室 API
- グレースフルシャットダウン

**対象外（将来検討）:**
- WebSocket/SSE によるリアルタイム通知
- フロントエンドでの動的更新

---

## 2. 技術設計

### 2.1 アーキテクチャ概要

```
┌─────────────────────────────────────────────────────────────┐
│                         Server                              │
│  ┌───────────────────────────────────────────────────────┐ │
│  │                   WatcherManager                       │ │
│  │  ┌─────────────────────────────────────────────────┐  │ │
│  │  │ Conversation 1                                  │  │ │
│  │  │  ┌──────────────┐  ┌──────────────┐            │  │ │
│  │  │  │AvatarWatcher │  │AvatarWatcher │            │  │ │
│  │  │  │   (太郎)     │  │   (花子)     │  ...       │  │ │
│  │  │  │  goroutine   │  │  goroutine   │            │  │ │
│  │  │  └──────┬───────┘  └──────┬───────┘            │  │ │
│  │  │         │                 │                    │  │ │
│  │  │         │  10秒ごとに     │                    │  │ │
│  │  │         ▼                 ▼                    │  │ │
│  │  │  ┌─────────────────────────────────────────┐   │  │ │
│  │  │  │              DB (messages)              │   │  │ │
│  │  │  └─────────────────────────────────────────┘   │  │ │
│  │  └─────────────────────────────────────────────────┘  │ │
│  └───────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 コンポーネント設計

#### 2.2.1 WatcherManager

Goroutineのライフサイクルを一元管理するコンポーネント。

```go
// WatcherManager manages avatar watcher goroutines
type WatcherManager struct {
    db        *db.DB
    assistant *assistant.Client
    watchers  map[watcherKey]*AvatarWatcher
    mu        sync.RWMutex
    interval  time.Duration
    ctx       context.Context
    cancel    context.CancelFunc
}

type watcherKey struct {
    ConversationID int64
    AvatarID       int64
}

// Main methods
func NewManager(db *db.DB, assistant *assistant.Client, interval time.Duration) *WatcherManager
func (m *WatcherManager) StartWatcher(conversationID, avatarID int64) error
func (m *WatcherManager) StopWatcher(conversationID, avatarID int64) error
func (m *WatcherManager) StopRoomWatchers(conversationID int64) error
func (m *WatcherManager) InitializeAll(ctx context.Context) error
func (m *WatcherManager) Shutdown() error
func (m *WatcherManager) WatcherCount() int
```

#### 2.2.2 AvatarWatcher

個別のアバターの監視ロジックを担当。

```go
// AvatarWatcher monitors conversation for a specific avatar
type AvatarWatcher struct {
    conversationID int64
    avatar         models.Avatar
    db             *db.DB
    assistant      *assistant.Client
    interval       time.Duration
    lastMessageID  int64
    ctx            context.Context
    cancel         context.CancelFunc
    wg             sync.WaitGroup
}

// Main methods
func NewAvatarWatcher(ctx context.Context, conversationID int64, avatar models.Avatar, ...) *AvatarWatcher
func (w *AvatarWatcher) Start()
func (w *AvatarWatcher) Stop()
func (w *AvatarWatcher) checkAndRespond() error
func (w *AvatarWatcher) shouldRespond(message *models.Message) (bool, error)
func (w *AvatarWatcher) generateResponse(message *models.Message) error
```

### 2.3 応答判断フロー

```
┌─────────────────────────────────────────────────────────────┐
│                   checkAndRespond()                         │
├─────────────────────────────────────────────────────────────┤
│  1. DBから最新メッセージを取得                               │
│     └─ lastMessageID より新しいもの                         │
│                                                             │
│  2. 自分以外のメッセージをフィルタリング                     │
│     └─ sender_type != 'avatar' OR sender_id != 自分のID    │
│                                                             │
│  3. 各メッセージに対して shouldRespond() を実行              │
│     ├─ メンションチェック: @自分の名前 が含まれるか？        │
│     │   └─ YES → 応答する                                  │
│     │                                                       │
│     └─ LLM判断: 応答責任があるか？                          │
│         └─ YES (確率高) → 応答する                         │
│                                                             │
│  4. 応答する場合、generateResponse() を実行                  │
│     ├─ OpenAI Assistants API で応答生成                    │
│     └─ DBにメッセージを保存                                 │
│                                                             │
│  5. lastMessageID を更新                                    │
└─────────────────────────────────────────────────────────────┘
```

### 2.4 LLM判断のプロンプト設計

```
You are "{avatar_name}" character.

【Your Settings】
{avatar's prompt/personality}

【Task】
Read the following message and determine whether you should respond to it.

Criteria:
- Is the content related to your specialty or role?
- Are you being directly addressed?
- Can you provide useful information?
- Should you speak based on the conversation flow?

【Message】
{user or other avatar's message}

【Answer】
Answer only "yes" if you should respond, or "no" if not.
```

---

## 3. 実装詳細

### 3.1 ディレクトリ構成

```
backend/internal/
├── api/
│   ├── avatar.go                # existing
│   ├── conversation.go          # modify: add Watcher integration
│   ├── conversation_avatar.go   # NEW: avatar join/leave API
│   └── router.go                # modify: add new routes
├── assistant/
│   └── client.go                # existing
├── db/
│   ├── conversation.go          # modify: add RemoveAvatarFromConversation, GetMessagesAfter
│   └── db.go                    # existing
├── logic/
│   ├── mention.go               # modify: Unicode support
│   └── responder.go             # existing
├── models/
│   └── models.go                # existing
└── watcher/                     # NEW package
    ├── manager.go               # WatcherManager
    ├── manager_test.go
    ├── avatar_watcher.go        # AvatarWatcher
    └── avatar_watcher_test.go
```

### 3.2 変更対象ファイル一覧

| ファイル | 種別 | 変更内容 |
|----------|------|----------|
| `internal/watcher/manager.go` | 新規 | WatcherManager 実装 |
| `internal/watcher/manager_test.go` | 新規 | WatcherManager テスト |
| `internal/watcher/avatar_watcher.go` | 新規 | AvatarWatcher 実装 |
| `internal/watcher/avatar_watcher_test.go` | 新規 | AvatarWatcher テスト |
| `internal/logic/mention.go` | 変更 | Unicode対応の正規表現 |
| `internal/logic/mention_test.go` | 変更 | マルチバイト文字のテスト追加 |
| `internal/db/conversation.go` | 変更 | RemoveAvatarFromConversation, GetMessagesAfter 追加 |
| `internal/db/conversation_test.go` | 変更 | 新機能のテスト追加 |
| `internal/api/conversation_avatar.go` | 新規 | アバター参加/退室ハンドラー |
| `internal/api/conversation_avatar_test.go` | 新規 | ハンドラーテスト |
| `internal/api/router.go` | 変更 | 新規ルート追加、WatcherManager 連携 |
| `internal/api/conversation.go` | 変更 | Watcher起動/停止の連携 |
| `cmd/server/main.go` | 変更 | WatcherManager初期化、グレースフルシャットダウン |

### 3.3 API設計

#### 新規エンドポイント

```
POST   /api/conversations/{id}/avatars
       Request:  { "avatar_id": 123 }
       Response: 204 No Content
       Description: Add avatar to chat room

DELETE /api/conversations/{id}/avatars/{avatar_id}
       Response: 204 No Content
       Description: Remove avatar from chat room

GET    /api/conversations/{id}/avatars
       Response: [{ "id": 1, "name": "太郎", ... }, ...]
       Description: List avatars in chat room
```

#### 変更されるエンドポイント

```
POST   /api/conversations
       Change: Start Goroutines for participating avatars after creation

DELETE /api/conversations/{id}
       Change: Stop all related Goroutines before deletion
```

### 3.4 データベース変更

既存テーブルで対応可能。追加のテーブルは不要。

**新規DB関数:**

```go
// RemoveAvatarFromConversation removes an avatar from a conversation
func (d *DB) RemoveAvatarFromConversation(conversationID, avatarID int64) error

// GetMessagesAfter retrieves messages with ID greater than the given ID
func (d *DB) GetMessagesAfter(conversationID int64, afterID int64) ([]models.Message, error)

// GetAllConversationAvatars retrieves all conversation-avatar pairs (for initialization)
func (d *DB) GetAllConversationAvatars() ([]models.ConversationAvatar, error)
```

---

## 4. 実装フェーズ

### Phase 1: メンションのマルチバイト対応

**目的**: 日本語名のアバターをメンションできるようにする

**現在の実装** (`internal/logic/mention.go`):
```go
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_]+)`)
```

**変更後の実装**:
```go
// Unicode support: first char is letter, subsequent chars are letter, number, or underscore
var mentionRegex = regexp.MustCompile(`@(\p{L}[\p{L}\p{N}_]*)`)
```

**対応例**:
- `@Alice` → "Alice"
- `@太郎` → "太郎"
- `@田中花子` → "田中花子"
- `@User_123` → "User_123"

**タスク**:

| 順番 | タスク | ファイル |
|------|--------|----------|
| 1-1 | マルチバイト対応テスト追加 | `mention_test.go` |
| 1-2 | 正規表現をUnicode対応に変更 | `mention.go` |
| 1-3 | テスト実行・確認 | - |

**テスト追加内容**:
```go
func TestParseMentions_MultiByte(t *testing.T) {
    testCases := []struct {
        name     string
        content  string
        expected []string
    }{
        {"Japanese single name", "@太郎 こんにちは", []string{"太郎"}},
        {"Japanese full name", "@田中花子 さん", []string{"田中花子"}},
        {"Mixed languages", "@Alice @太郎 hello", []string{"Alice", "太郎"}},
        {"Korean name", "@김철수 안녕하세요", []string{"김철수"}},
        {"Name with underscore", "@花子_123 test", []string{"花子_123"}},
    }
    // ...
}
```

**見積もり**: 約15分

---

### Phase 2: データベース層の拡張

**目的**: アバター退室と増分メッセージ取得機能を追加

**タスク**:

| 順番 | タスク | ファイル |
|------|--------|----------|
| 2-1 | RemoveAvatarFromConversation テスト追加 | `conversation_test.go` |
| 2-2 | RemoveAvatarFromConversation 実装 | `conversation.go` |
| 2-3 | GetMessagesAfter テスト追加 | `conversation_test.go` |
| 2-4 | GetMessagesAfter 実装 | `conversation.go` |
| 2-5 | GetAllConversationAvatars テスト追加 | `conversation_test.go` |
| 2-6 | GetAllConversationAvatars 実装 | `conversation.go` |

**実装コード**:

```go
// RemoveAvatarFromConversation removes an avatar from a conversation
func (d *DB) RemoveAvatarFromConversation(conversationID, avatarID int64) error {
    return d.WithLock(func() error {
        result, err := d.db.Exec(
            `DELETE FROM conversation_avatars WHERE conversation_id = ? AND avatar_id = ?`,
            conversationID, avatarID,
        )
        if err != nil {
            return err
        }
        rows, err := result.RowsAffected()
        if err != nil {
            return err
        }
        if rows == 0 {
            return sql.ErrNoRows
        }
        return nil
    })
}

// GetMessagesAfter retrieves messages with ID greater than the given ID
func (d *DB) GetMessagesAfter(conversationID int64, afterID int64) ([]models.Message, error) {
    return WithLockResult(d, func() ([]models.Message, error) {
        rows, err := d.db.Query(
            `SELECT id, conversation_id, sender_type, sender_id, content, created_at 
            FROM messages 
            WHERE conversation_id = ? AND id > ?
            ORDER BY id ASC`,
            conversationID, afterID,
        )
        if err != nil {
            return nil, err
        }
        defer rows.Close()
        
        var messages []models.Message
        for rows.Next() {
            var msg models.Message
            var senderID sql.NullInt64
            var senderType string
            if err := rows.Scan(&msg.ID, &msg.ConversationID, &senderType, &senderID, &msg.Content, &msg.CreatedAt); err != nil {
                return nil, err
            }
            msg.SenderType = models.SenderType(senderType)
            if senderID.Valid {
                id := senderID.Int64
                msg.SenderID = &id
            }
            messages = append(messages, msg)
        }
        return messages, rows.Err()
    })
}

// GetAllConversationAvatars retrieves all conversation-avatar pairs
func (d *DB) GetAllConversationAvatars() ([]models.ConversationAvatar, error) {
    return WithLockResult(d, func() ([]models.ConversationAvatar, error) {
        rows, err := d.db.Query(
            `SELECT conversation_id, avatar_id FROM conversation_avatars`,
        )
        if err != nil {
            return nil, err
        }
        defer rows.Close()
        
        var pairs []models.ConversationAvatar
        for rows.Next() {
            var pair models.ConversationAvatar
            if err := rows.Scan(&pair.ConversationID, &pair.AvatarID); err != nil {
                return nil, err
            }
            pairs = append(pairs, pair)
        }
        return pairs, rows.Err()
    })
}
```

**見積もり**: 約20分

---

### Phase 3: WatcherManager基盤

**目的**: Goroutine管理の基本機能を実装

**タスク**:

| 順番 | タスク | ファイル |
|------|--------|----------|
| 3-1 | watcher パッケージ作成 | `internal/watcher/` |
| 3-2 | WatcherManager 構造体定義 | `manager.go` |
| 3-3 | NewManager テスト・実装 | `manager.go`, `manager_test.go` |
| 3-4 | StartWatcher テスト・実装 | `manager.go`, `manager_test.go` |
| 3-5 | StopWatcher テスト・実装 | `manager.go`, `manager_test.go` |
| 3-6 | StopRoomWatchers テスト・実装 | `manager.go`, `manager_test.go` |
| 3-7 | Shutdown テスト・実装 | `manager.go`, `manager_test.go` |
| 3-8 | InitializeAll テスト・実装 | `manager.go`, `manager_test.go` |

**主要実装コード**:

```go
package watcher

import (
    "context"
    "log"
    "sync"
    "time"
    
    "multi-avatar-chat/internal/assistant"
    "multi-avatar-chat/internal/db"
)

// WatcherManager manages avatar watcher goroutines
type WatcherManager struct {
    db        *db.DB
    assistant *assistant.Client
    watchers  map[watcherKey]*AvatarWatcher
    mu        sync.RWMutex
    interval  time.Duration
    ctx       context.Context
    cancel    context.CancelFunc
}

type watcherKey struct {
    ConversationID int64
    AvatarID       int64
}

// NewManager creates a new WatcherManager
func NewManager(database *db.DB, assistantClient *assistant.Client, interval time.Duration) *WatcherManager {
    ctx, cancel := context.WithCancel(context.Background())
    return &WatcherManager{
        db:        database,
        assistant: assistantClient,
        watchers:  make(map[watcherKey]*AvatarWatcher),
        interval:  interval,
        ctx:       ctx,
        cancel:    cancel,
    }
}

// StartWatcher starts a new watcher for the given conversation and avatar
func (m *WatcherManager) StartWatcher(conversationID, avatarID int64) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    key := watcherKey{ConversationID: conversationID, AvatarID: avatarID}
    
    // Check if already running
    if _, exists := m.watchers[key]; exists {
        log.Printf("[WatcherManager] Watcher already exists conversation_id=%d avatar_id=%d", conversationID, avatarID)
        return nil
    }
    
    // Get avatar info from DB
    avatar, err := m.db.GetAvatar(avatarID)
    if err != nil {
        return err
    }
    
    // Create and start watcher
    watcher := NewAvatarWatcher(m.ctx, conversationID, *avatar, m.db, m.assistant, m.interval)
    watcher.Start()
    
    m.watchers[key] = watcher
    log.Printf("[WatcherManager] Watcher started conversation_id=%d avatar_id=%d avatar_name=%s", 
        conversationID, avatarID, avatar.Name)
    
    return nil
}

// StopWatcher stops the watcher for the given conversation and avatar
func (m *WatcherManager) StopWatcher(conversationID, avatarID int64) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    key := watcherKey{ConversationID: conversationID, AvatarID: avatarID}
    
    watcher, exists := m.watchers[key]
    if !exists {
        return nil
    }
    
    watcher.Stop()
    delete(m.watchers, key)
    log.Printf("[WatcherManager] Watcher stopped conversation_id=%d avatar_id=%d", conversationID, avatarID)
    
    return nil
}

// StopRoomWatchers stops all watchers for a conversation
func (m *WatcherManager) StopRoomWatchers(conversationID int64) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for key, watcher := range m.watchers {
        if key.ConversationID == conversationID {
            watcher.Stop()
            delete(m.watchers, key)
            log.Printf("[WatcherManager] Watcher stopped conversation_id=%d avatar_id=%d", 
                key.ConversationID, key.AvatarID)
        }
    }
    
    return nil
}

// InitializeAll starts watchers for all existing conversation-avatar pairs
func (m *WatcherManager) InitializeAll(ctx context.Context) error {
    pairs, err := m.db.GetAllConversationAvatars()
    if err != nil {
        return err
    }
    
    log.Printf("[WatcherManager] Initializing %d watchers", len(pairs))
    
    for _, pair := range pairs {
        if err := m.StartWatcher(pair.ConversationID, pair.AvatarID); err != nil {
            log.Printf("[WatcherManager] Failed to start watcher conversation_id=%d avatar_id=%d err=%v",
                pair.ConversationID, pair.AvatarID, err)
        }
    }
    
    return nil
}

// Shutdown stops all watchers gracefully
func (m *WatcherManager) Shutdown() error {
    log.Printf("[WatcherManager] Shutting down...")
    m.cancel()
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for key, watcher := range m.watchers {
        watcher.Stop()
        log.Printf("[WatcherManager] Watcher stopped conversation_id=%d avatar_id=%d", 
            key.ConversationID, key.AvatarID)
    }
    m.watchers = make(map[watcherKey]*AvatarWatcher)
    
    log.Printf("[WatcherManager] Shutdown complete")
    return nil
}

// WatcherCount returns the number of active watchers
func (m *WatcherManager) WatcherCount() int {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return len(m.watchers)
}
```

**見積もり**: 約45分

---

### Phase 4: AvatarWatcher

**目的**: 個別アバターの監視ロジックを実装

**タスク**:

| 順番 | タスク | ファイル |
|------|--------|----------|
| 4-1 | AvatarWatcher 構造体定義 | `avatar_watcher.go` |
| 4-2 | NewAvatarWatcher テスト・実装 | `avatar_watcher.go`, `avatar_watcher_test.go` |
| 4-3 | Start/Stop テスト・実装 | `avatar_watcher.go`, `avatar_watcher_test.go` |
| 4-4 | checkAndRespond テスト・実装 | `avatar_watcher.go`, `avatar_watcher_test.go` |
| 4-5 | shouldRespond (メンションチェック) テスト・実装 | `avatar_watcher.go`, `avatar_watcher_test.go` |
| 4-6 | shouldRespond (LLM判断) テスト・実装 | `avatar_watcher.go`, `avatar_watcher_test.go` |
| 4-7 | generateResponse テスト・実装 | `avatar_watcher.go`, `avatar_watcher_test.go` |

**主要実装コード**:

```go
package watcher

import (
    "context"
    "log"
    "strings"
    "sync"
    "time"
    
    "multi-avatar-chat/internal/assistant"
    "multi-avatar-chat/internal/db"
    "multi-avatar-chat/internal/logic"
    "multi-avatar-chat/internal/models"
)

// AvatarWatcher monitors conversation for a specific avatar
type AvatarWatcher struct {
    conversationID int64
    avatar         models.Avatar
    db             *db.DB
    assistant      *assistant.Client
    interval       time.Duration
    lastMessageID  int64
    ctx            context.Context
    cancel         context.CancelFunc
    wg             sync.WaitGroup
}

// NewAvatarWatcher creates a new AvatarWatcher
func NewAvatarWatcher(
    parentCtx context.Context,
    conversationID int64,
    avatar models.Avatar,
    database *db.DB,
    assistantClient *assistant.Client,
    interval time.Duration,
) *AvatarWatcher {
    ctx, cancel := context.WithCancel(parentCtx)
    return &AvatarWatcher{
        conversationID: conversationID,
        avatar:         avatar,
        db:             database,
        assistant:      assistantClient,
        interval:       interval,
        ctx:            ctx,
        cancel:         cancel,
    }
}

// Start begins the monitoring loop
func (w *AvatarWatcher) Start() {
    w.wg.Add(1)
    go w.run()
}

// Stop stops the monitoring loop
func (w *AvatarWatcher) Stop() {
    w.cancel()
    w.wg.Wait()
}

func (w *AvatarWatcher) run() {
    defer w.wg.Done()
    
    log.Printf("[AvatarWatcher] Started conversation_id=%d avatar_id=%d avatar_name=%s interval=%v",
        w.conversationID, w.avatar.ID, w.avatar.Name, w.interval)
    
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-w.ctx.Done():
            log.Printf("[AvatarWatcher] Stopped conversation_id=%d avatar_id=%d", 
                w.conversationID, w.avatar.ID)
            return
        case <-ticker.C:
            if err := w.checkAndRespond(); err != nil {
                log.Printf("[AvatarWatcher] Error during check conversation_id=%d avatar_id=%d err=%v",
                    w.conversationID, w.avatar.ID, err)
            }
        }
    }
}

// checkAndRespond checks for new messages and responds if appropriate
func (w *AvatarWatcher) checkAndRespond() error {
    // Get new messages since last check
    messages, err := w.db.GetMessagesAfter(w.conversationID, w.lastMessageID)
    if err != nil {
        return err
    }
    
    if len(messages) == 0 {
        return nil
    }
    
    log.Printf("[AvatarWatcher] Found %d new messages conversation_id=%d avatar_id=%d",
        len(messages), w.conversationID, w.avatar.ID)
    
    // Process each message
    for _, msg := range messages {
        // Update lastMessageID
        if msg.ID > w.lastMessageID {
            w.lastMessageID = msg.ID
        }
        
        // Skip own messages
        if msg.SenderType == models.SenderTypeAvatar && msg.SenderID != nil && *msg.SenderID == w.avatar.ID {
            continue
        }
        
        // Check if should respond
        shouldRespond, err := w.shouldRespond(&msg)
        if err != nil {
            log.Printf("[AvatarWatcher] Error checking shouldRespond message_id=%d err=%v", msg.ID, err)
            continue
        }
        
        if shouldRespond {
            if err := w.generateResponse(&msg); err != nil {
                log.Printf("[AvatarWatcher] Error generating response message_id=%d err=%v", msg.ID, err)
            }
        }
    }
    
    return nil
}

// shouldRespond determines if the avatar should respond to the message
func (w *AvatarWatcher) shouldRespond(message *models.Message) (bool, error) {
    // Check for direct mention
    mentionedNames := logic.ParseMentions(message.Content)
    for _, name := range mentionedNames {
        if strings.EqualFold(name, w.avatar.Name) {
            log.Printf("[AvatarWatcher] Mentioned in message message_id=%d avatar_name=%s",
                message.ID, w.avatar.Name)
            return true, nil
        }
    }
    
    // If no assistant configured, skip LLM judgment
    if w.assistant == nil || w.avatar.OpenAIAssistantID == "" {
        return false, nil
    }
    
    // LLM-based judgment
    return w.shouldRespondLLM(message)
}

// shouldRespondLLM uses LLM to determine if avatar should respond
func (w *AvatarWatcher) shouldRespondLLM(message *models.Message) (bool, error) {
    prompt := w.buildJudgmentPrompt(message.Content)
    
    // Use a simple completion request for judgment
    response, err := w.assistant.SimpleCompletion(prompt)
    if err != nil {
        return false, err
    }
    
    answer := strings.TrimSpace(strings.ToLower(response))
    return answer == "yes", nil
}

// buildJudgmentPrompt creates the prompt for response judgment
func (w *AvatarWatcher) buildJudgmentPrompt(messageContent string) string {
    return `You are "` + w.avatar.Name + `" character.

【Your Settings】
` + w.avatar.Prompt + `

【Task】
Read the following message and determine whether you should respond to it.

Criteria:
- Is the content related to your specialty or role?
- Are you being directly addressed?
- Can you provide useful information?
- Should you speak based on the conversation flow?

【Message】
` + messageContent + `

【Answer】
Answer only "yes" if you should respond, or "no" if not.`
}

// generateResponse generates and saves a response from the avatar
func (w *AvatarWatcher) generateResponse(message *models.Message) error {
    log.Printf("[AvatarWatcher] Generating response conversation_id=%d avatar_id=%d message_id=%d",
        w.conversationID, w.avatar.ID, message.ID)
    
    // Get conversation for thread ID
    conv, err := w.db.GetConversation(w.conversationID)
    if err != nil {
        return err
    }
    
    if conv.ThreadID == "" || w.avatar.OpenAIAssistantID == "" {
        log.Printf("[AvatarWatcher] Cannot generate response: missing thread_id or assistant_id")
        return nil
    }
    
    // Create a run
    run, err := w.assistant.CreateRun(conv.ThreadID, w.avatar.OpenAIAssistantID)
    if err != nil {
        return err
    }
    
    // Wait for completion (30 second timeout)
    _, err = w.assistant.WaitForRun(conv.ThreadID, run.ID, 30*time.Second)
    if err != nil {
        return err
    }
    
    // Get response
    responseContent, err := w.assistant.GetLatestAssistantMessage(conv.ThreadID)
    if err != nil {
        return err
    }
    
    // Save to database
    avatarID := w.avatar.ID
    savedMsg, err := w.db.CreateMessage(w.conversationID, models.SenderTypeAvatar, &avatarID, responseContent)
    if err != nil {
        return err
    }
    
    log.Printf("[AvatarWatcher] Response generated conversation_id=%d avatar_id=%d response_message_id=%d",
        w.conversationID, w.avatar.ID, savedMsg.ID)
    
    return nil
}
```

**見積もり**: 約60分

---

### Phase 5: API変更（アバター参加/退室）

**目的**: アバターの動的参加/退室APIを実装

**タスク**:

| 順番 | タスク | ファイル |
|------|--------|----------|
| 5-1 | ConversationAvatarHandler 構造体定義 | `conversation_avatar.go` |
| 5-2 | AddAvatar ハンドラー テスト・実装 | `conversation_avatar.go`, `conversation_avatar_test.go` |
| 5-3 | RemoveAvatar ハンドラー テスト・実装 | `conversation_avatar.go`, `conversation_avatar_test.go` |
| 5-4 | ListAvatars ハンドラー テスト・実装 | `conversation_avatar.go`, `conversation_avatar_test.go` |
| 5-5 | ルーター更新 | `router.go` |

**実装コード**:

```go
package api

import (
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "strconv"
    
    "multi-avatar-chat/internal/db"
    "multi-avatar-chat/internal/watcher"
)

// ConversationAvatarHandler handles avatar participation in conversations
type ConversationAvatarHandler struct {
    db      *db.DB
    watcher *watcher.WatcherManager
}

// NewConversationAvatarHandler creates a new handler
func NewConversationAvatarHandler(database *db.DB, watcherManager *watcher.WatcherManager) *ConversationAvatarHandler {
    return &ConversationAvatarHandler{
        db:      database,
        watcher: watcherManager,
    }
}

// AddAvatarRequest represents the request body for adding an avatar
type AddAvatarRequest struct {
    AvatarID int64 `json:"avatar_id"`
}

// AddAvatar handles POST /api/conversations/{id}/avatars
func (h *ConversationAvatarHandler) AddAvatar(w http.ResponseWriter, r *http.Request) {
    log.Printf("[API] AddAvatar started")
    
    conversationID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
    if err != nil {
        http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
        return
    }
    
    var req AddAvatarRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    // Verify conversation exists
    _, err = h.db.GetConversation(conversationID)
    if err == sql.ErrNoRows {
        http.Error(w, "Conversation not found", http.StatusNotFound)
        return
    }
    if err != nil {
        http.Error(w, "Failed to get conversation", http.StatusInternalServerError)
        return
    }
    
    // Verify avatar exists
    _, err = h.db.GetAvatar(req.AvatarID)
    if err == sql.ErrNoRows {
        http.Error(w, "Avatar not found", http.StatusNotFound)
        return
    }
    if err != nil {
        http.Error(w, "Failed to get avatar", http.StatusInternalServerError)
        return
    }
    
    // Add avatar to conversation
    if err := h.db.AddAvatarToConversation(conversationID, req.AvatarID); err != nil {
        http.Error(w, "Failed to add avatar", http.StatusInternalServerError)
        return
    }
    
    // Start watcher
    if h.watcher != nil {
        if err := h.watcher.StartWatcher(conversationID, req.AvatarID); err != nil {
            log.Printf("[API] Warning: failed to start watcher err=%v", err)
        }
    }
    
    log.Printf("[API] Avatar added conversation_id=%d avatar_id=%d", conversationID, req.AvatarID)
    w.WriteHeader(http.StatusNoContent)
}

// RemoveAvatar handles DELETE /api/conversations/{id}/avatars/{avatar_id}
func (h *ConversationAvatarHandler) RemoveAvatar(w http.ResponseWriter, r *http.Request) {
    log.Printf("[API] RemoveAvatar started")
    
    conversationID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
    if err != nil {
        http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
        return
    }
    
    avatarID, err := strconv.ParseInt(r.PathValue("avatar_id"), 10, 64)
    if err != nil {
        http.Error(w, "Invalid avatar ID", http.StatusBadRequest)
        return
    }
    
    // Stop watcher first
    if h.watcher != nil {
        if err := h.watcher.StopWatcher(conversationID, avatarID); err != nil {
            log.Printf("[API] Warning: failed to stop watcher err=%v", err)
        }
    }
    
    // Remove from database
    if err := h.db.RemoveAvatarFromConversation(conversationID, avatarID); err != nil {
        if err == sql.ErrNoRows {
            http.Error(w, "Avatar not in conversation", http.StatusNotFound)
            return
        }
        http.Error(w, "Failed to remove avatar", http.StatusInternalServerError)
        return
    }
    
    log.Printf("[API] Avatar removed conversation_id=%d avatar_id=%d", conversationID, avatarID)
    w.WriteHeader(http.StatusNoContent)
}

// ListAvatars handles GET /api/conversations/{id}/avatars
func (h *ConversationAvatarHandler) ListAvatars(w http.ResponseWriter, r *http.Request) {
    conversationID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
    if err != nil {
        http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
        return
    }
    
    avatars, err := h.db.GetConversationAvatars(conversationID)
    if err != nil {
        http.Error(w, "Failed to get avatars", http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(avatars)
}
```

**ルーター更新** (`router.go`):

```go
// Add routes in setupRoutes()
r.mux.HandleFunc("GET /api/conversations/{id}/avatars", r.conversationAvatarHandler.ListAvatars)
r.mux.HandleFunc("POST /api/conversations/{id}/avatars", r.conversationAvatarHandler.AddAvatar)
r.mux.HandleFunc("DELETE /api/conversations/{id}/avatars/{avatar_id}", r.conversationAvatarHandler.RemoveAvatar)
```

**見積もり**: 約45分

---

### Phase 6: サーバ統合

**目的**: サーバ起動時の初期化とグレースフルシャットダウンを実装

**タスク**:

| 順番 | タスク | ファイル |
|------|--------|----------|
| 6-1 | WatcherManager をRouterに統合 | `router.go` |
| 6-2 | 会話作成時にWatcher起動 | `conversation.go` |
| 6-3 | 会話削除時にWatcher停止 | `conversation.go` |
| 6-4 | サーバ起動時の InitializeAll | `main.go` |
| 6-5 | グレースフルシャットダウン実装 | `main.go` |

**main.go の変更**:

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    "time"

    "multi-avatar-chat/internal/api"
    "multi-avatar-chat/internal/assistant"
    "multi-avatar-chat/internal/config"
    "multi-avatar-chat/internal/db"
    "multi-avatar-chat/internal/watcher"
)

func main() {
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        log.Printf("Warning: Failed to load config: %v (continuing without OpenAI)", err)
        cfg = &config.Config{
            DBPath:    getEnvOrDefault("DB_PATH", "data/app.db"),
            StaticDir: getEnvOrDefault("STATIC_DIR", "static"),
        }
    }

    // Ensure data directory exists
    dbDir := filepath.Dir(cfg.DBPath)
    if err := os.MkdirAll(dbDir, 0755); err != nil {
        log.Fatalf("Failed to create data directory: %v", err)
    }

    // Initialize database
    database, err := db.NewDB(cfg.DBPath)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer database.Close()

    // Run migrations
    if err := database.Migrate(); err != nil {
        log.Fatalf("Failed to migrate database: %v", err)
    }
    log.Println("Database migrated successfully")

    // Initialize OpenAI client (optional)
    var assistantClient *assistant.Client
    if cfg.OpenAI.APIKey != "" {
        assistantClient = assistant.NewClient(cfg.OpenAI.APIKey)
        log.Println("OpenAI client initialized")
    } else {
        log.Println("Warning: OpenAI API key not configured, assistant features disabled")
    }

    // Initialize WatcherManager
    watcherInterval := 10 * time.Second // Default interval
    if intervalStr := os.Getenv("WATCHER_INTERVAL"); intervalStr != "" {
        if d, err := time.ParseDuration(intervalStr); err == nil {
            watcherInterval = d
        }
    }
    watcherManager := watcher.NewManager(database, assistantClient, watcherInterval)
    log.Printf("WatcherManager initialized with interval=%v", watcherInterval)

    // Initialize all watchers for existing conversations
    ctx := context.Background()
    if err := watcherManager.InitializeAll(ctx); err != nil {
        log.Printf("Warning: Failed to initialize watchers: %v", err)
    }
    log.Printf("Watchers initialized: count=%d", watcherManager.WatcherCount())

    // Create router
    router := api.NewRouter(database, assistantClient, cfg.StaticDir, watcherManager)

    // Setup server
    port := getEnvOrDefault("PORT", "8080")
    server := &http.Server{
        Addr:    ":" + port,
        Handler: router,
    }

    // Handle graceful shutdown
    done := make(chan bool, 1)
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-quit
        log.Println("Server is shutting down...")

        // Shutdown watchers first
        if err := watcherManager.Shutdown(); err != nil {
            log.Printf("Error shutting down watchers: %v", err)
        }

        // Shutdown HTTP server with timeout
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        if err := server.Shutdown(ctx); err != nil {
            log.Fatalf("Server forced to shutdown: %v", err)
        }

        close(done)
    }()

    log.Printf("Server starting on port %s", port)
    log.Printf("Static files served from: %s", cfg.StaticDir)

    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Server failed to start: %v", err)
    }

    <-done
    log.Println("Server stopped gracefully")
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

**見積もり**: 約45分

---

### Phase 7: OpenAI SimpleCompletion 追加

**目的**: LLM判断のためのシンプルなCompletion機能を追加

**タスク**:

| 順番 | タスク | ファイル |
|------|--------|----------|
| 7-1 | SimpleCompletion テスト追加 | `client_test.go` |
| 7-2 | SimpleCompletion 実装 | `client.go` |

**実装コード**:

```go
// SimpleCompletion sends a simple completion request for judgment
func (c *Client) SimpleCompletion(prompt string) (string, error) {
    log.Printf("[Assistant] SimpleCompletion started prompt_length=%d", len(prompt))
    
    reqBody := map[string]any{
        "model": "gpt-4o-mini",
        "messages": []map[string]string{
            {"role": "user", "content": prompt},
        },
        "max_tokens": 10,
    }
    
    body, err := json.Marshal(reqBody)
    if err != nil {
        return "", err
    }
    
    req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
    if err != nil {
        return "", err
    }
    
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        respBody, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("OpenAI API error: %s", string(respBody))
    }
    
    var result struct {
        Choices []struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
        } `json:"choices"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }
    
    if len(result.Choices) == 0 {
        return "", fmt.Errorf("no response from OpenAI")
    }
    
    content := result.Choices[0].Message.Content
    log.Printf("[Assistant] SimpleCompletion completed response=%q", content)
    
    return content, nil
}
```

**見積もり**: 約20分

---

### Phase 8: フロントエンド対応（将来）

**目的**: アバター選択UIの実装（今回は対象外、将来検討）

フロントエンドの変更は本計画のスコープ外とし、別途計画を作成する。

必要な変更概要:
- チャット画面にアバター管理パネルを追加
- チェックボックスによる参加/退室の切り替え
- API呼び出しの実装

---

## 5. テスト計画

### 5.1 ユニットテスト

| コンポーネント | テストファイル | テスト項目 |
|----------------|----------------|------------|
| ParseMentions | `mention_test.go` | マルチバイト文字対応 |
| WatcherManager | `manager_test.go` | Start/Stop/Shutdown |
| AvatarWatcher | `avatar_watcher_test.go` | checkAndRespond, shouldRespond |
| ConversationAvatarHandler | `conversation_avatar_test.go` | AddAvatar, RemoveAvatar, ListAvatars |

### 5.2 統合テスト

| シナリオ | 検証内容 |
|----------|----------|
| アバター参加 | APIでアバター追加 → Goroutine起動 → 監視開始 |
| アバター退室 | APIでアバター削除 → Goroutine停止 |
| チャットルーム削除 | 関連するすべてのGoroutineが停止 |
| サーバ再起動 | 既存の会話に対してWatcherが再起動 |
| グレースフルシャットダウン | すべてのWatcherが正常終了 |

### 5.3 手動テスト

1. **基本動作**: メッセージ送信 → 10秒後にアバターが応答判断
2. **メンション**: `@太郎` → 太郎が必ず応答
3. **動的参加**: チェックボックス切り替え → 即座に参加/退室
4. **シャットダウン**: Ctrl+C → ログで正常終了を確認

---

## 6. エラーハンドリング

| エラーケース | 対応 |
|-------------|------|
| OpenAI APIキーが未設定 | LLM判断をスキップ、メンションのみで判定 |
| アバターにAssistant IDがない | そのアバターの応答生成をスキップ |
| DB接続エラー | エラーログを出力し、次回チェックまで待機 |
| LLM判断タイムアウト | エラーログを出力、応答なしで続行 |
| Goroutine パニック | recover() でキャッチ、エラーログ出力 |

---

## 7. 見積もり

| フェーズ | 内容 | 見積もり時間 |
|---------|------|-------------|
| Phase 1 | メンションのマルチバイト対応 | 15分 |
| Phase 2 | データベース層の拡張 | 20分 |
| Phase 3 | WatcherManager基盤 | 45分 |
| Phase 4 | AvatarWatcher | 60分 |
| Phase 5 | API変更（アバター参加/退室） | 45分 |
| Phase 6 | サーバ統合 | 45分 |
| Phase 7 | OpenAI SimpleCompletion | 20分 |
| - | テスト・デバッグ | 60分 |
| **合計** | | **約5時間10分** |

---

## 8. リスクと対策

| リスク | 影響 | 対策 |
|--------|------|------|
| Goroutineリーク | メモリ増大 | WaitGroup使用、テストで検証 |
| 同時応答の競合 | 重複応答 | メッセージID管理、DB排他制御 |
| OpenAI APIレート制限 | 応答遅延 | 適切なinterval設定、リトライ |
| SQLite同時アクセス | デッドロック | 既存のセマフォ機構を継続利用 |
| 大量Goroutine起動 | リソース枯渇 | 最大Watcher数の制限（将来） |

---

## 9. 完了条件

| 項目 | 完了条件 |
|------|----------|
| Phase 1 | `@太郎` でメンションが認識される |
| Phase 2 | RemoveAvatarFromConversation, GetMessagesAfter が動作 |
| Phase 3 | WatcherManager がGoroutineを正しく起動・停止 |
| Phase 4 | AvatarWatcher がメッセージを検出し応答判断を実行 |
| Phase 5 | API経由でアバター参加/退室が可能 |
| Phase 6 | サーバ起動時に既存Watcherが起動、シャットダウン時に正常終了 |
| Phase 7 | LLM判断によるyes/no判定が動作 |

---

## 10. 依存関係図

```
Phase 1 (Mention Unicode)
    │
    ▼
Phase 2 (DB Extension)
    │
    ├───────────────────────────┐
    ▼                           ▼
Phase 3 (WatcherManager) ◄── Phase 7 (SimpleCompletion)
    │
    ▼
Phase 4 (AvatarWatcher)
    │
    ▼
Phase 5 (API Changes)
    │
    ▼
Phase 6 (Server Integration)
    │
    ▼
Phase 8 (Frontend - Future)
```

