# Multi-Avatar Chat Application 実装計画

## 概要

本計画は `01-000-FirstIdea.md` に記載されたMulti-Avatar Chat Applicationを、TDD（テスト駆動開発）アプローチで実装するための詳細な手順を定義する。

## 前提条件

- Docker Desktop がインストールされていること
- Go 1.21以上がローカルにインストールされていること（開発用）
- Node.js 18以上 + yarn がインストールされていること
- OpenAI APIキーが `settings/secrets/openai.yaml` に設定されていること

---

## Phase 1: Docker + 基盤構築

### Step 1.1: Dockerファイル作成

**目的**: 開発・本番環境の基盤を構築

**タスク**:
1. `Dockerfile` を作成
   - マルチステージビルド（Go + Node.js）
   - 最終イメージは軽量なalpineベース
2. `docker-compose.yml` を作成
   - 開発用ボリュームマウント
   - 環境変数設定
   - ポートマッピング（8080）

**成果物**:
- `Dockerfile`
- `docker-compose.yml`
- `.dockerignore`

---

### Step 1.2: Golangプロジェクト初期化

**目的**: バックエンドの基盤とテスト環境を構築

**タスク**:
1. `backend/go.mod` を初期化
2. エントリーポイント `backend/cmd/server/main.go` を作成
3. テストヘルパーを準備
4. 最小限のヘルスチェックエンドポイントを実装（TDD）

**テスト** (`backend/internal/api/health_test.go`):
```go
func TestHealthEndpoint(t *testing.T) {
    // GET /health が 200 を返すこと
}
```

**成果物**:
- `backend/go.mod`, `backend/go.sum`
- `backend/cmd/server/main.go`
- `backend/internal/api/health.go`
- `backend/internal/api/health_test.go`

---

### Step 1.3: SQLite + セマフォ排他制御

**目的**: データベースアクセス層を構築

**タスク**:
1. SQLite接続管理を実装
2. セマフォによる排他制御を実装
3. マイグレーション機能を実装
4. データベーススキーマを定義

**テスト** (`backend/internal/db/db_test.go`):
```go
func TestSemaphoreExclusiveAccess(t *testing.T) {
    // 同時アクセスが排他制御されること
}

func TestMigration(t *testing.T) {
    // テーブルが正しく作成されること
}
```

**スキーマ**:
```sql
CREATE TABLE avatars (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    prompt TEXT NOT NULL,
    openai_assistant_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE conversations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id TEXT,
    title TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE conversation_avatars (
    conversation_id INTEGER NOT NULL,
    avatar_id INTEGER NOT NULL,
    PRIMARY KEY (conversation_id, avatar_id),
    FOREIGN KEY (conversation_id) REFERENCES conversations(id),
    FOREIGN KEY (avatar_id) REFERENCES avatars(id)
);

CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id INTEGER NOT NULL,
    sender_type TEXT NOT NULL, -- 'user' or 'avatar'
    sender_id INTEGER,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id)
);
```

**成果物**:
- `backend/internal/db/db.go`
- `backend/internal/db/db_test.go`
- `backend/internal/db/migrations.go`
- `backend/internal/models/models.go`

---

## Phase 2: OpenAI Assistants API連携

### Step 2.1: Assistants APIクライアント実装

**目的**: OpenAI Assistants APIとの通信層を構築

**タスク**:
1. APIキー読み込み処理を実装
2. Assistants APIクライアントを実装
3. エラーハンドリングを実装

**テスト** (`backend/internal/assistant/client_test.go`):
```go
func TestLoadAPIKey(t *testing.T) {
    // YAMLからAPIキーを読み込めること
}

func TestCreateAssistant(t *testing.T) {
    // Assistantを作成できること（モック使用）
}
```

**成果物**:
- `backend/internal/assistant/client.go`
- `backend/internal/assistant/client_test.go`
- `backend/internal/config/config.go`

---

### Step 2.2: アバターCRUD操作

**目的**: アバター管理機能を実装

**タスク**:
1. アバター作成（DB + OpenAI Assistant作成）
2. アバター取得
3. アバター更新（プロンプト変更時はAssistantも更新）
4. アバター削除（DB + OpenAI Assistant削除）

**テスト** (`backend/internal/api/avatar_test.go`):
```go
func TestCreateAvatar(t *testing.T) {
    // POST /api/avatars でアバターを作成できること
}

func TestGetAvatars(t *testing.T) {
    // GET /api/avatars でアバター一覧を取得できること
}

func TestUpdateAvatar(t *testing.T) {
    // PUT /api/avatars/:id でアバターを更新できること
}

func TestDeleteAvatar(t *testing.T) {
    // DELETE /api/avatars/:id でアバターを削除できること
}
```

**API設計**:
| メソッド | パス | 説明 |
|---------|------|------|
| POST | /api/avatars | アバター作成 |
| GET | /api/avatars | アバター一覧取得 |
| GET | /api/avatars/:id | アバター詳細取得 |
| PUT | /api/avatars/:id | アバター更新 |
| DELETE | /api/avatars/:id | アバター削除 |

**成果物**:
- `backend/internal/api/avatar.go`
- `backend/internal/api/avatar_test.go`

---

### Step 2.3: Thread/Message管理

**目的**: 会話セッションとメッセージ管理を実装

**タスク**:
1. 会話（Thread）作成
2. メッセージ送信
3. メッセージ履歴取得
4. OpenAI Thread APIとの連携

**テスト** (`backend/internal/api/conversation_test.go`):
```go
func TestCreateConversation(t *testing.T) {
    // POST /api/conversations で会話を作成できること
}

func TestSendMessage(t *testing.T) {
    // POST /api/conversations/:id/messages でメッセージを送信できること
}

func TestGetMessages(t *testing.T) {
    // GET /api/conversations/:id/messages でメッセージ一覧を取得できること
}
```

**API設計**:
| メソッド | パス | 説明 |
|---------|------|------|
| POST | /api/conversations | 会話作成 |
| GET | /api/conversations | 会話一覧取得 |
| GET | /api/conversations/:id | 会話詳細取得 |
| DELETE | /api/conversations/:id | 会話削除 |
| POST | /api/conversations/:id/messages | メッセージ送信 |
| GET | /api/conversations/:id/messages | メッセージ一覧取得 |

**成果物**:
- `backend/internal/api/conversation.go`
- `backend/internal/api/conversation_test.go`
- `backend/internal/assistant/thread.go`
- `backend/internal/assistant/thread_test.go`

---

## Phase 3: 応答ロジック

### Step 3.1: メンション解析

**目的**: メッセージ内の `@アバター名` を解析する機能を実装

**タスク**:
1. メンションパターン解析
2. アバター名とのマッチング
3. 複数メンション対応

**テスト** (`backend/internal/logic/mention_test.go`):
```go
func TestParseMentions(t *testing.T) {
    // "@Avatar1 こんにちは" から Avatar1 を抽出できること
}

func TestMultipleMentions(t *testing.T) {
    // "@Avatar1 @Avatar2 質問です" から複数アバターを抽出できること
}

func TestNoMention(t *testing.T) {
    // メンションがない場合は空のリストを返すこと
}
```

**成果物**:
- `backend/internal/logic/mention.go`
- `backend/internal/logic/mention_test.go`

---

### Step 3.2: 応答アバター選択ロジック

**目的**: どのアバターが応答すべきかを決定するロジックを実装

**タスク**:
1. メンションモード：指名されたアバターが応答
2. 通常モード：会話内容を分析して関連アバターを選択
3. 選択ロジックのインターフェース定義

**テスト** (`backend/internal/logic/responder_test.go`):
```go
func TestMentionedAvatarResponds(t *testing.T) {
    // メンションされたアバターが応答対象になること
}

func TestRelevantAvatarSelection(t *testing.T) {
    // 会話内容に関連するアバターが選択されること
}
```

**成果物**:
- `backend/internal/logic/responder.go`
- `backend/internal/logic/responder_test.go`

---

### Step 3.3: ディスカッションモード

**目的**: アバター同士が会話に参加できるモードを実装

**タスク**:
1. ディスカッションモードのフラグ管理
2. アバター応答後の連鎖応答ロジック
3. 無限ループ防止（応答回数制限）

**テスト** (`backend/internal/logic/discussion_test.go`):
```go
func TestDiscussionMode(t *testing.T) {
    // ディスカッションモードでアバター同士が会話すること
}

func TestMaxResponseLimit(t *testing.T) {
    // 最大応答回数に達したら終了すること
}
```

**成果物**:
- `backend/internal/logic/discussion.go`
- `backend/internal/logic/discussion_test.go`

---

## Phase 4: フロントエンド

### Step 4.1: React Native for Webセットアップ

**目的**: フロントエンド開発環境を構築

**タスク**:
1. React Native for Web プロジェクト初期化（yarn使用）
2. TypeScript設定
3. Webpackまたはexpo-cli設定
4. 開発サーバー設定

**コマンド**:
```bash
cd frontend
yarn init -y
yarn add react react-dom react-native react-native-web
yarn add -D typescript @types/react @types/react-dom @types/react-native
```

**成果物**:
- `frontend/package.json`
- `frontend/tsconfig.json`
- `frontend/src/App.tsx`
- `frontend/src/index.tsx`
- `frontend/webpack.config.js` または expo設定

---

### Step 4.2: 共通コンポーネント作成

**目的**: 再利用可能なUIコンポーネントを作成

**タスク**:
1. ボタンコンポーネント
2. テキスト入力コンポーネント
3. モーダルコンポーネント
4. リストコンポーネント

**成果物**:
- `frontend/src/components/Button.tsx`
- `frontend/src/components/TextInput.tsx`
- `frontend/src/components/Modal.tsx`
- `frontend/src/components/List.tsx`

---

### Step 4.3: アバター管理UI

**目的**: アバターの作成・編集・削除UIを実装

**タスク**:
1. アバター一覧表示
2. アバター作成フォーム
3. アバター編集フォーム
4. アバター削除確認

**テスト** (`frontend/src/__tests__/AvatarManager.test.tsx`):
```typescript
test('アバター一覧が表示される', async () => {
  // ...
});

test('新規アバターを作成できる', async () => {
  // ...
});
```

**成果物**:
- `frontend/src/components/AvatarManager.tsx`
- `frontend/src/components/AvatarForm.tsx`
- `frontend/src/__tests__/AvatarManager.test.tsx`

---

### Step 4.4: チャットUI

**目的**: メッセージ送受信のUIを実装

**タスク**:
1. メッセージ一覧表示（スクロール対応）
2. メッセージ入力フォーム
3. メンション入力補完（@入力時にアバター候補表示）
4. 送信中・読み込み中の状態表示

**テスト** (`frontend/src/__tests__/ChatArea.test.tsx`):
```typescript
test('メッセージが表示される', async () => {
  // ...
});

test('メッセージを送信できる', async () => {
  // ...
});

test('@入力でアバター候補が表示される', async () => {
  // ...
});
```

**成果物**:
- `frontend/src/components/ChatArea.tsx`
- `frontend/src/components/MessageList.tsx`
- `frontend/src/components/MessageInput.tsx`
- `frontend/src/components/MentionSuggestion.tsx`
- `frontend/src/__tests__/ChatArea.test.tsx`

---

### Step 4.5: API通信サービス

**目的**: バックエンドAPIとの通信層を実装

**タスク**:
1. APIクライアント基盤
2. アバターAPI
3. 会話API
4. エラーハンドリング

**成果物**:
- `frontend/src/services/api.ts`
- `frontend/src/services/avatarService.ts`
- `frontend/src/services/conversationService.ts`

---

### Step 4.6: シングルページ統合

**目的**: 全コンポーネントを統合してシングルページアプリを完成

**タスク**:
1. レイアウト構築（左：アバター管理、右：チャット）
2. 状態管理（React Context または Zustand）
3. レスポンシブデザイン対応

**成果物**:
- `frontend/src/App.tsx`（統合版）
- `frontend/src/context/AppContext.tsx`
- `frontend/src/styles/`

---

## Phase 5: 統合・デプロイ

### Step 5.1: フロントエンドビルド統合

**目的**: フロントエンドをバックエンドで配信する設定

**タスク**:
1. フロントエンドの本番ビルド設定
2. Golangでの静的ファイル配信設定
3. Dockerfileの更新

**成果物**:
- `Dockerfile`（更新）
- `backend/cmd/server/main.go`（静的ファイル配信追加）

---

### Step 5.2: 統合テスト

**目的**: システム全体の動作確認

**タスク**:
1. E2Eテストの作成
2. Docker環境でのテスト実行
3. パフォーマンステスト

**成果物**:
- `backend/internal/api/integration_test.go`
- `frontend/src/__tests__/e2e/`

---

### Step 5.3: ドキュメント整備

**目的**: プロジェクトのドキュメントを整備

**タスク**:
1. README.md 作成
2. API仕様書作成
3. 環境構築手順書

**成果物**:
- `README.md`
- `docs/API.md`
- `docs/SETUP.md`

---

## タイムライン目安

| Phase | 内容 | 推定日数 |
|-------|------|----------|
| Phase 1 | Docker + 基盤構築 | 2-3日 |
| Phase 2 | OpenAI Assistants連携 | 3-4日 |
| Phase 3 | 応答ロジック | 2-3日 |
| Phase 4 | フロントエンド | 4-5日 |
| Phase 5 | 統合・デプロイ | 2-3日 |
| **合計** | | **13-18日** |

---

## 依存関係図

```
Phase 1.1 (Docker)
    │
    ▼
Phase 1.2 (Golang初期化) ──────────────────┐
    │                                      │
    ▼                                      ▼
Phase 1.3 (SQLite) ───► Phase 2.1 (Assistant Client)
                              │
                              ▼
                        Phase 2.2 (Avatar CRUD)
                              │
                              ▼
                        Phase 2.3 (Thread/Message)
                              │
    ┌─────────────────────────┴─────────────────────────┐
    ▼                                                   ▼
Phase 3.1 (Mention) ──► Phase 3.2 (Responder)    Phase 4.1 (Frontend Setup)
                              │                         │
                              ▼                         ▼
                        Phase 3.3 (Discussion)   Phase 4.2-4.5 (Components)
                              │                         │
                              └────────────┬────────────┘
                                           ▼
                                    Phase 4.6 (Integration)
                                           │
                                           ▼
                                    Phase 5 (Deploy)
```

---

## リスクと対策

| リスク | 影響 | 対策 |
|--------|------|------|
| OpenAI API レート制限 | テスト遅延 | モックを活用したテスト設計 |
| SQLite 同時アクセス問題 | データ破損 | セマフォによる厳格な排他制御 |
| React Native for Web 互換性 | UI崩れ | 早期に検証、Web専用コンポーネント準備 |
| Assistants API 仕様変更 | 実装修正 | APIバージョン固定、抽象化層の設計 |

---

## 完了条件

各Phaseの完了条件:

1. **Phase 1**: `docker-compose up` でコンテナが起動し、`/health` が200を返す
2. **Phase 2**: アバターのCRUDがAPI経由で動作し、OpenAI Assistantが作成される
3. **Phase 3**: メンション指定でアバターが応答、ディスカッションモードが動作
4. **Phase 4**: ブラウザでUIが表示され、チャットが機能する
5. **Phase 5**: Docker単一コンテナで全機能が動作する

