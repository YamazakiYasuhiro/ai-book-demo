# Multi-Avatar Chat Application

## アーキテクチャ

```
                              ┌──────────────────┐
                              │   OpenAI         │
                              │ Assistants API   │
                              │  (外部サービス)   │
                              └────────▲─────────┘
                                       │ HTTPS
┌──────────────────────────────────────┼────────────────────┐
│              Docker Container        │                    │
│  ┌─────────────────┐      ┌─────────┴─────────┐          │
│  │  React Native   │ HTTP │    Golang API     │          │
│  │    for Web      │◄────►│    Server         │          │
│  │  (静的ファイル)  │      └─────────┬─────────┘          │
│  └─────────────────┘                │                    │
│                                     ▼                    │
│                            ┌─────────────────┐           │
│                            │     SQLite      │           │
│                            │   (Semaphore)   │           │
│                            └─────────────────┘           │
└──────────────────────────────────────────────────────────┘
```

## 技術スタック

| レイヤー | 技術 |
|----------|------|
| Container | Docker (単一コンテナ) |
| Frontend | React Native for Web (yarn) |
| Backend | Golang |
| Database | SQLite + Semaphore排他制御 |
| LLM | OpenAI Assistants API (外部API) |
| 開発手法 | テスト駆動開発 (TDD) |

## OpenAI Assistants API 活用

- **Assistant** = アバター（性格プロンプトを設定）
- **Thread** = 会話セッション（会話履歴を自動管理）
- **Message** = 各発言
- **Run** = アシスタントの応答生成

SQLiteには以下を保存:
- アバターのローカル情報 + OpenAI assistant_id
- 会話の thread_id
- メッセージのキャッシュ（高速表示用）

## データベース設計

### テーブル構成

- **avatars**: id, name, prompt, openai_assistant_id, created_at
- **conversations**: id, thread_id, title, created_at
- **conversation_avatars**: conversation_id, avatar_id（会話参加者）
- **messages**: id, conversation_id, sender_type, sender_id, content, created_at

## UI設計（シングルページ）

```
┌─────────────────────────────────────────────────────────────┐
│  Multi-Avatar Chat                                          │
├─────────────────────┬───────────────────────────────────────┤
│  アバター管理        │  チャット                              │
│  ┌───────────────┐  │  ┌─────────────────────────────────┐  │
│  │ + 新規作成    │  │  │ [Avatar1]: こんにちは           │  │
│  ├───────────────┤  │  │ [You]: @Avatar2 質問です        │  │
│  │ Avatar1 [編集]│  │  │ [Avatar2]: はい、答えます       │  │
│  │ Avatar2 [編集]│  │  │ ...                             │  │
│  └───────────────┘  │  └─────────────────────────────────┘  │
│                     │  ┌─────────────────────────────────┐  │
│                     │  │ メッセージ入力...          [送信]│  │
│                     │  └─────────────────────────────────┘  │
└─────────────────────┴───────────────────────────────────────┘
```

## ディレクトリ構成

```
ai-book-demo/
├── Dockerfile
├── docker-compose.yml
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── api/           # HTTPハンドラー
│   │   ├── api_test.go    # APIテスト
│   │   ├── db/            # SQLite + Semaphore
│   │   ├── db_test.go     # DBテスト
│   │   ├── assistant/     # OpenAI Assistants API
│   │   ├── assistant_test.go
│   │   └── models/        # データモデル
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/    # UIコンポーネント
│   │   ├── services/      # API通信
│   │   ├── __tests__/     # フロントエンドテスト
│   │   └── App.tsx
│   └── package.json
└── settings/
    └── secrets/
        └── openai.yaml
```

## 実装ステップ (TDD)

### Phase 1: Docker + 基盤
1. Dockerfile, docker-compose.yml作成
2. Golangプロジェクト初期化 + テスト環境構築
3. SQLite + セマフォ排他制御（テストファースト）

### Phase 2: OpenAI Assistants連携
4. Assistants API連携テスト作成
5. アバター作成/取得/削除の実装
6. Thread/Message管理の実装

### Phase 3: 応答ロジック
7. メンション解析テスト/実装
8. 応答アバター選択ロジック（テストファースト）
9. ディスカッションモード

### Phase 4: フロントエンド
10. React Native for Webセットアップ
11. シングルページUI実装
12. バックエンド統合テスト

## 主要機能

### 1. アバター管理
- アバターの作成・編集・削除
- 性格やプロンプトの設定

### 2. 会話機能
- 新規会話の作成
- メッセージ送信
- メンション機能（`@アバター名`で指名）

### 3. LLM応答ロジック
- **通常モード**: AIが会話内容を分析し、関連するアバターのみが応答
- **メンションモード**: 指名されたアバターが強制応答
- **ディスカッションモード**: アバター同士も会話に参加可能

