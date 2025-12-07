# Thread管理の変更計画

## 概要

現在はチャットルームごとに1つのThreadを共有している実装を、アバターごとに独立したThreadを持つように変更します。これにより、アバターの並列処理が可能になり、各アバターが独立して会話履歴を管理できるようになります。

## 目標

1. アバターごとに独立したThreadを持つ
2. ユーザメッセージを全アバターのThreadに送信
3. アバター発言を他のアバターのThreadにユーザメッセージとして送信
4. LLMへのインプットをログに出力

## Phase 1: データベース設計の変更

### Step 1.1: データベーススキーマの変更

**目的**: アバターごとのThreadIDを保存できるようにする

**変更内容**:
- `conversation_avatars`テーブルに`thread_id`カラムを追加
- マイグレーション処理を追加

**スキーマ変更**:
```sql
ALTER TABLE conversation_avatars ADD COLUMN thread_id TEXT;
```

**マイグレーション実装** (`backend/internal/db/migrations.go`):
- 既存の`conversation_avatars`テーブルに`thread_id`カラムが存在するかチェック
- 存在しない場合のみ追加
- 既存データの移行処理（既存のconversationのthread_idを各アバターにコピーするか、新規作成）

**考慮事項**:
- 既存の`conversations.thread_id`は後方互換性のため残す（段階的移行）
- 将来的には`conversations.thread_id`は削除可能

### Step 1.2: モデルの変更

**変更ファイル**: `backend/internal/models/models.go`

**変更内容**:
- `ConversationAvatar`モデルに`ThreadID`フィールドを追加

```go
type ConversationAvatar struct {
    ConversationID int64  `json:"conversation_id"`
    AvatarID       int64  `json:"avatar_id"`
    ThreadID       string `json:"thread_id,omitempty"`
}
```

### Step 1.3: データベース操作の変更

**変更ファイル**: `backend/internal/db/conversation.go`

**変更内容**:
- `AddAvatarToConversation`関数を変更して、ThreadIDを受け取るようにする
- `GetConversationAvatars`関数を変更して、ThreadIDも取得するようにする
- 新しい関数`GetAvatarThreadID`を追加（conversation_idとavatar_idからthread_idを取得）

**新しい関数**:
```go
// GetAvatarThreadID retrieves the thread ID for a specific avatar in a conversation
func (d *DB) GetAvatarThreadID(conversationID, avatarID int64) (string, error)

// UpdateAvatarThreadID updates the thread ID for an avatar in a conversation
func (d *DB) UpdateAvatarThreadID(conversationID, avatarID int64, threadID string) error
```

## Phase 2: Thread作成処理の変更

### Step 2.1: 会話作成時のThread作成

**変更ファイル**: `backend/internal/api/conversation.go`

**変更内容**:
- `Create`関数を変更して、各アバターごとにThreadを作成
- 作成したThreadIDを`conversation_avatars`テーブルに保存

**処理フロー**:
1. 会話を作成（`conversations`テーブル）
2. 各アバターに対して：
   - OpenAI Threadを作成
   - `conversation_avatars`テーブルにアバターを追加し、ThreadIDも保存

### Step 2.2: アバター追加時のThread作成

**変更ファイル**: `backend/internal/api/conversation.go`

**変更内容**:
- `AddAvatar`関数（存在する場合）またはアバター追加処理を変更
- アバター追加時にThreadを作成して保存

## Phase 3: メッセージ処理の変更

### Step 3.1: ユーザメッセージ送信処理の変更

**変更ファイル**: `backend/internal/api/conversation.go`

**変更内容**:
- `SendMessage`関数を変更
- ユーザメッセージを全アバターのThreadに送信

**処理フロー**:
1. ユーザメッセージをデータベースに保存
2. 会話に参加している全アバターを取得
3. 各アバターのThreadIDを取得
4. 各アバターのThreadにユーザメッセージとして送信
   - メッセージフォーマット: `Name: ユーザ\nMessage:\n{content}`

### Step 3.2: アバター発言処理の変更

**変更ファイル**: `backend/internal/watcher/avatar_watcher.go`

**変更内容**:
- `generateResponse`関数を変更
- アバターが発言した後、他のアバターのThreadにユーザメッセージとして送信

**処理フロー**:
1. アバターが自分のThreadでレスポンスを生成
2. レスポンスをデータベースに保存
3. 会話に参加している他のアバターを取得
4. 各アバターのThreadIDを取得
5. 各アバターのThreadにユーザメッセージとして送信
   - メッセージフォーマット: `Name: (Avatar) {avatar_name}\nMessage:\n{content}`

### Step 3.3: メッセージフォーマット関数の追加

**変更ファイル**: `backend/internal/logic/message_formatter.go`（既存）または新規作成

**追加関数**:
```go
// FormatUserMessage formats a user message for OpenAI Thread
func FormatUserMessage(content string) string

// FormatAvatarMessage formats an avatar message as a user message for other avatars' threads
func FormatAvatarMessage(avatarName, content string) string
```

**実装内容**:
- ユーザメッセージ: `Name: ユーザ\nMessage:\n{content}`
- アバター発言: `Name: (Avatar) {avatar_name}\nMessage:\n{content}`

## Phase 4: AvatarWatcherの変更

### Step 4.1: ThreadID取得の変更

**変更ファイル**: `backend/internal/watcher/avatar_watcher.go`

**変更内容**:
- `generateResponse`関数を変更
- `conv.ThreadID`の代わりに、アバター固有のThreadIDを取得して使用

**変更箇所**:
- `w.db.GetConversation(w.conversationID)`で取得した`conv.ThreadID`を使用する代わりに
- `w.db.GetAvatarThreadID(w.conversationID, w.avatar.ID)`でアバター固有のThreadIDを取得

### Step 4.2: メッセージ送信処理の追加

**変更内容**:
- アバターがレスポンスを生成した後、他のアバターのThreadにメッセージを送信する処理を追加
- 新しい関数`broadcastMessageToOtherAvatars`を追加

**実装**:
```go
// broadcastMessageToOtherAvatars sends the avatar's message to other avatars' threads
func (w *AvatarWatcher) broadcastMessageToOtherAvatars(content string) error
```

## Phase 5: ログ出力の追加

### Step 5.1: LLMインプットログの追加

**変更ファイル**: `backend/internal/assistant/thread.go`

**変更内容**:
- `CreateMessage`関数にログ出力を追加
- アバター名とThreadID、メッセージ内容をログに出力

**ログフォーマット**:
```
[Assistant] CreateMessage thread_id=%s avatar_name=%s message_preview=%q full_content=%q
```

### Step 5.2: Run作成時のログ追加

**変更ファイル**: `backend/internal/assistant/thread.go`

**変更内容**:
- `CreateRun`関数と`CreateRunWithContext`関数にログ出力を追加
- アバター名、Assistant ID、追加コンテキスト（存在する場合）をログに出力

**ログフォーマット**:
```
[Assistant] CreateRun thread_id=%s avatar_name=%s assistant_id=%s context_length=%d
[Assistant] CreateRunWithContext thread_id=%s avatar_name=%s assistant_id=%s context_length=%d additional_context=%q
```

### Step 5.3: メッセージ一覧取得時のログ追加

**変更ファイル**: `backend/internal/assistant/thread.go`

**変更内容**:
- `ListMessages`関数にログ出力を追加
- 取得したメッセージの概要をログに出力

**ログフォーマット**:
```
[Assistant] ListMessages thread_id=%s avatar_name=%s message_count=%d
```

### Step 5.4: AvatarWatcherでのログ追加

**変更ファイル**: `backend/internal/watcher/avatar_watcher.go`

**変更内容**:
- `generateResponse`関数内で、LLMへのインプットをログに出力
- `buildConversationContext`関数で作成したコンテキストもログに出力

**ログフォーマット**:
```
[AvatarWatcher] LLM Input thread_id=%s avatar_name=%s conversation_context_length=%d
[AvatarWatcher] Broadcasting message to other avatars avatar_name=%s message_id=%d target_count=%d
```

## Phase 6: 後方互換性とマイグレーション

### Step 6.1: 既存データの移行

**目的**: 既存の会話データを新しい構造に移行

**処理**:
1. 既存の`conversations.thread_id`が存在する場合
2. その会話に参加している各アバターに対して：
   - 新しいThreadを作成（または既存のThreadIDをコピー）
   - `conversation_avatars.thread_id`を更新

**注意事項**:
- 既存のThreadを共有するのではなく、各アバター用に新しいThreadを作成することを推奨
- 既存のメッセージ履歴は新しいThreadには引き継がない（新規会話として扱う）

### Step 6.2: 段階的移行

**方針**:
- `conversations.thread_id`は一時的に残す（後方互換性のため）
- 新しい会話では使用しない
- 将来的に削除可能

## Phase 7: テスト

### Step 7.1: データベーステスト

**テストファイル**: `backend/internal/db/conversation_test.go`

**テストケース**:
- `TestAddAvatarToConversationWithThreadID`: ThreadID付きでアバターを追加できること
- `TestGetAvatarThreadID`: アバターのThreadIDを取得できること
- `TestUpdateAvatarThreadID`: アバターのThreadIDを更新できること
- `TestGetConversationAvatarsWithThreadID`: ThreadIDを含めてアバター一覧を取得できること

### Step 7.2: APIテスト

**テストファイル**: `backend/internal/api/conversation_test.go`

**テストケース**:
- `TestCreateConversationWithMultipleAvatars`: 複数アバターで会話作成時に各アバターのThreadが作成されること
- `TestSendMessageToAllAvatarThreads`: ユーザメッセージが全アバターのThreadに送信されること

### Step 7.3: Watcherテスト

**テストファイル**: `backend/internal/watcher/avatar_watcher_test.go`

**テストケース**:
- `TestGenerateResponseWithAvatarThread`: アバター固有のThreadでレスポンスを生成できること
- `TestBroadcastMessageToOtherAvatars`: アバター発言が他のアバターのThreadに送信されること

### Step 7.4: 統合テスト

**テストファイル**: `tests/integration/conversation_test.go`

**テストケース**:
- ユーザメッセージ → 全アバターのThreadに送信 → 各アバターがレスポンス生成 → 他のアバターのThreadに送信
- ログ出力が正しく行われていること

## 実装順序

1. **Phase 1**: データベース設計の変更（スキーマ、モデル、DB操作）
2. **Phase 2**: Thread作成処理の変更
3. **Phase 3**: メッセージ処理の変更
4. **Phase 4**: AvatarWatcherの変更
5. **Phase 5**: ログ出力の追加
6. **Phase 6**: 後方互換性とマイグレーション
7. **Phase 7**: テスト

## 注意事項

1. **Thread管理**: 各アバターが独立したThreadを持つため、ThreadLockManagerの使用箇所を確認し、必要に応じて調整
2. **エラーハンドリング**: Thread作成やメッセージ送信が失敗した場合の処理を適切に実装
3. **パフォーマンス**: 複数のThreadへの並列送信を考慮した実装
4. **ログレベル**: 詳細なログ出力により、デバッグしやすくする
5. **メッセージフォーマット**: `01-005-PromptImprovements.md`で定義されたフォーマットに従う

## 関連ファイル

- `backend/internal/models/models.go`
- `backend/internal/db/migrations.go`
- `backend/internal/db/conversation.go`
- `backend/internal/db/avatar.go`
- `backend/internal/api/conversation.go`
- `backend/internal/assistant/thread.go`
- `backend/internal/watcher/avatar_watcher.go`
- `backend/internal/logic/message_formatter.go`

