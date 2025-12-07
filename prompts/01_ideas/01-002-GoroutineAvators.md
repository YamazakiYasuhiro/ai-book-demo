# マルチスレッドアバターモデル要件書

## 1. 背景

### 1.1 現状の課題

現在のシステムでは、ユーザーがメッセージを送信した際に、同期的に1人のアバターが即座に応答する仕組みになっている。この設計には以下の課題がある：

1. **単一応答の制限**: ユーザーのメッセージに対して、常に1人のアバターしか応答できない
2. **能動的な会話の欠如**: アバターは受動的にしか発言できず、自発的な会話参加ができない
3. **アルファベット限定のメンション**: メンション機能が `@[a-zA-Z0-9_]+` のパターンで実装されており、日本語名のアバターをメンションできない
4. **固定的なアバター参加**: チャットルーム作成時にのみアバターを選択でき、後から変更できない
5. **リソース管理の不在**: 同時実行される処理の管理機構がない

### 1.2 目指す姿

複数のアバターがそれぞれ独立したGoroutineとして動作し、定期的に会話を監視して自律的に応答を判断するシステムを構築する。これにより、より自然で活発なマルチアバター会話を実現する。

## 2. 要件

### 2.1 機能要件

#### FR-001: マルチスレッドアバター監視
- 各アバターは独立したGoroutineとして動作する
- 設定可能な間隔（デフォルト10秒）で会話を監視する
- 自分以外のメッセージを検出したら、応答判断を行う

#### FR-002: 応答判断ロジック
アバターは以下の2つの条件で応答を判断する：

1. **メンションによる強制応答**
   - メッセージに `@アバター名` が含まれている場合、該当アバターは必ず応答する
   - アバター名はマルチバイト文字（日本語等）にも対応する

2. **LLM判断による応答**
   - メンションがない場合、LLMにメッセージを解析させる
   - アバターの役割・性格設定に基づき、応答責任があるかを判定
   - 高い確率で応答すべきと判断した場合のみ応答する

#### FR-003: メンションのマルチバイト対応
- `@太郎`、`@花子`、`@Alice` など、任意の言語のアバター名をメンション可能にする
- Unicode文字クラスを使用した正規表現で実装

#### FR-004: 動的なアバター参加管理
- チャットルーム作成後もアバターの参加・退室が可能
- フロントエンドでチェックボックスによる選択UIを提供
- 参加/退室時にGoroutineを適切に起動/停止

#### FR-005: チャットルーム削除
- チャットルームを削除できる機能を提供
- 削除時に関連するすべてのGoroutineを終了する
- OpenAIのThread/Assistantリソースもクリーンアップ

#### FR-006: Goroutineライフサイクル管理
- **起動タイミング**:
  - チャットルーム作成時
  - アバターがルームに参加した時
  - サーバ起動時（既存のルーム/アバターに対して）
- **終了タイミング**:
  - チャットルーム削除時
  - アバターがルームから退室した時
  - サーバシャットダウン時

### 2.2 非機能要件

#### NFR-001: リソース効率
- Goroutineリークを防止する
- 不要なGoroutineは速やかに終了する
- メモリ使用量を適切に管理する

#### NFR-002: 競合制御
- 複数アバターの同時応答を適切に処理する
- SQLiteへのアクセスは既存のセマフォ機構を活用
- OpenAI APIへの同時リクエストを制御

#### NFR-003: グレースフルシャットダウン
- サーバ終了時にすべてのGoroutineが完了を待つ
- 実行中の処理を適切にキャンセルする
- contextパッケージによるキャンセル伝播

#### NFR-004: 可観測性
- Goroutineの状態をログ出力
- エラー発生時の適切なハンドリングと記録

## 3. 設計

### 3.1 アーキテクチャ概要

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
│  │                                                       │ │
│  │  ┌─────────────────────────────────────────────────┐  │ │
│  │  │ Conversation 2                                  │  │ │
│  │  │  └── AvatarWatcher (Alice)                     │  │ │
│  │  └─────────────────────────────────────────────────┘  │ │
│  └───────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 コンポーネント設計

#### 3.2.1 WatcherManager

Goroutineのライフサイクルを一元管理するコンポーネント。

```go
// WatcherManager はアバターの監視Goroutineを管理する
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

// 主要メソッド
func NewManager(db *db.DB, assistant *assistant.Client, interval time.Duration) *Manager
func (m *Manager) StartWatcher(conversationID, avatarID int64) error
func (m *Manager) StopWatcher(conversationID, avatarID int64) error
func (m *Manager) StopRoomWatchers(conversationID int64) error
func (m *Manager) InitializeAll() error
func (m *Manager) Shutdown() error
```

#### 3.2.2 AvatarWatcher

個別のアバターの監視ロジックを担当するコンポーネント。

```go
// AvatarWatcher は特定のアバターの会話監視を行う
type AvatarWatcher struct {
    conversationID int64
    avatar         models.Avatar
    db             *db.DB
    assistant      *assistant.Client
    interval       time.Duration
    lastMessageID  int64  // 最後に確認したメッセージID
    ctx            context.Context
    cancel         context.CancelFunc
}

// 主要メソッド
func NewAvatarWatcher(ctx context.Context, conversationID int64, avatar models.Avatar, ...) *AvatarWatcher
func (w *AvatarWatcher) Start()
func (w *AvatarWatcher) Stop()
func (w *AvatarWatcher) checkAndRespond() error
func (w *AvatarWatcher) shouldRespond(message *models.Message) (bool, error)
func (w *AvatarWatcher) generateResponse(message *models.Message) error
```

#### 3.2.3 応答判断フロー

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

### 3.3 メンション解析の設計

#### 現在の実装
```go
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_]+)`)
```

#### 新しい実装
```go
// Unicode対応: 最初の文字は letter、後続は letter, number, underscore
var mentionRegex = regexp.MustCompile(`@(\p{L}[\p{L}\p{N}_]*)`)
```

**対応例**:
- `@Alice` → "Alice"
- `@太郎` → "太郎"
- `@田中花子` → "田中花子"
- `@User_123` → "User_123"

### 3.4 LLM判断のプロンプト設計

```
あなたは「{アバター名}」というキャラクターです。

【あなたの設定】
{アバターのプロンプト/性格設定}

【タスク】
以下のメッセージを読み、あなたがこのメッセージに応答すべきかどうかを判断してください。

判断基準:
- あなたの専門分野や役割に関連する内容か
- あなたに直接話しかけられているか
- あなたが有益な情報を提供できるか
- 会話の流れ上、あなたが発言すべきか

【メッセージ】
{ユーザーまたは他のアバターのメッセージ}

【回答】
応答すべき場合は "yes"、そうでない場合は "no" とだけ答えてください。
```

### 3.5 API設計

#### 新規エンドポイント

```
POST   /api/conversations/{id}/avatars
       リクエスト: { "avatar_id": 123 }
       レスポンス: 204 No Content
       説明: アバターをチャットルームに参加させる

DELETE /api/conversations/{id}/avatars/{avatar_id}
       レスポンス: 204 No Content
       説明: アバターをチャットルームから退室させる

GET    /api/conversations/{id}/avatars
       レスポンス: [{ "id": 1, "name": "太郎", ... }, ...]
       説明: チャットルームに参加しているアバター一覧
```

#### 変更されるエンドポイント

```
POST   /api/conversations
       変更: 作成後、参加アバターのGoroutineを起動

DELETE /api/conversations/{id}
       変更: 削除前、関連するGoroutineをすべて停止
```

### 3.6 データベース変更

既存のテーブル構造で対応可能。追加のテーブルは不要。

`conversation_avatars` テーブルを活用:
```sql
CREATE TABLE conversation_avatars (
    conversation_id INTEGER,
    avatar_id INTEGER,
    PRIMARY KEY (conversation_id, avatar_id)
);
```

### 3.7 フロントエンド設計

チャット画面にアバター管理パネルを追加:

```
┌─────────────────────────────────────────────────────────────┐
│  チャットルーム: 企画会議                                    │
├─────────────────────┬───────────────────────────────────────┤
│  参加アバター        │  メッセージ                           │
│  ┌───────────────┐  │  ┌─────────────────────────────────┐  │
│  │ ☑ 太郎        │  │  │ [太郎]: こんにちは             │  │
│  │ ☑ 花子        │  │  │ [You]: @花子 意見をください    │  │
│  │ ☐ 次郎        │  │  │ [花子]: はい、私の意見は...    │  │
│  │ ☐ Alice       │  │  │ [太郎]: 私も補足します...      │  │
│  └───────────────┘  │  └─────────────────────────────────┘  │
│                     │  ┌─────────────────────────────────┐  │
│  ※チェックで即時   │  │ メッセージを入力...        [送信]│  │
│    参加/退室       │  └─────────────────────────────────┘  │
└─────────────────────┴───────────────────────────────────────┘
```

## 4. ディレクトリ構成

```
backend/internal/
├── api/
│   ├── avatar.go              # 既存
│   ├── conversation.go        # 変更: Watcher連携追加
│   └── conversation_avatar.go # 新規: 参加/退室API
├── assistant/
│   └── client.go              # 既存
├── db/
│   ├── conversation.go        # 既存（RemoveAvatarFromConversation追加）
│   └── db.go                  # 既存
├── logic/
│   ├── mention.go             # 変更: マルチバイト対応
│   └── responder.go           # 既存
├── models/
│   └── models.go              # 既存
└── watcher/                   # 新規パッケージ
    ├── manager.go             # WatcherManager
    ├── manager_test.go
    ├── avatar_watcher.go      # AvatarWatcher
    └── avatar_watcher_test.go
```

## 5. 実装順序

1. **Phase 1: メンションのマルチバイト対応**
   - テスト作成 → 実装

2. **Phase 2: WatcherManager基盤**
   - Goroutine管理の基本機能
   - テスト作成 → 実装

3. **Phase 3: AvatarWatcher**
   - 監視ロジック
   - LLM判断機能
   - テスト作成 → 実装

4. **Phase 4: API変更**
   - アバター参加/退室API
   - 既存APIとの統合
   - テスト作成 → 実装

5. **Phase 5: サーバ統合**
   - 起動時の初期化
   - シャットダウン処理

6. **Phase 6: フロントエンド**
   - アバター選択UI
   - 統合テスト

