# アバターのプロンプト改善 - 実装計画

## 概要

本計画は、アバターのLLM呼び出しにおけるプロンプトの改善、メッセージ管理方法の見直し、監視タイミングのランダム化を実装するものである。

## 現状分析

### 現在の実装

1. **プロンプト構成** (`backend/internal/watcher/avatar_watcher.go`)
   - `buildJudgmentPrompt`: 応答判断用プロンプト
     - 自身のアバター名: ✅ 含まれている
     - 参加アバター一覧: ❌ 含まれていない
     - 議題（チャットルーム名）: ❌ 含まれていない

2. **メッセージ管理** (`backend/internal/assistant/thread.go`)
   - 現在はすべてのメッセージを `role: "user"` として送信している
   - アバター自身の発言とそれ以外を区別していない

3. **監視間隔** (`backend/internal/watcher/manager.go`, `avatar_watcher.go`)
   - `WatcherManager`で固定の`interval`を使用
   - 現在は10秒固定

## 実装計画

### フェーズ1: プロンプトへのコンテキスト情報追加

#### 1.1 AvatarWatcherの拡張

**対象ファイル**: `backend/internal/watcher/avatar_watcher.go`

**変更内容**:
1. `AvatarWatcher`構造体に以下のフィールドを追加：
   - `conversationTitle string` - チャットルーム名（議題）
   - `participantNames []string` - 参加者名一覧（アバター名 + ユーザ）

2. `NewAvatarWatcher`関数の引数を拡張して、上記情報を受け取る

3. `buildJudgmentPrompt`関数を拡張して、以下の情報をプロンプトに含める：
   - 議題（チャットルーム名）
   - 参加アバター一覧（ユーザを含む）

**テストファイル**: `backend/internal/watcher/avatar_watcher_test.go`

#### 1.2 WatcherManagerの更新

**対象ファイル**: `backend/internal/watcher/manager.go`

**変更内容**:
1. `StartWatcher`関数で、会話タイトルと参加者一覧を取得
2. `NewAvatarWatcher`呼び出し時に上記情報を渡す

**テストファイル**: `backend/internal/watcher/manager_test.go`

#### 1.3 プロンプトの改善例

```
【基本設定】
あなたは「{アバター名}」です。

【議題】
{チャットルーム名}

【参加者】
- ユーザ
- (Avatar) {アバター名1}
- (Avatar) {アバター名2}
...

【あなたの性格・設定】
{アバターのプロンプト}

【タスク】
以下のメッセージを読み、あなたが応答すべきかを判断してください。
...
```

---

### フェーズ2: メッセージの管理とAPIへの渡し方

#### 2.1 概念の整理

| 呼称 | 説明 |
|------|------|
| ユーザ発言 | 本システムのユーザが入力したメッセージ |
| アバター発言 | アバターが生成したメッセージ |
| ユーザメッセージ | Assistant APIで`role: "user"`として送信するメッセージ |
| システムメッセージ | Assistant APIで`role: "assistant"`として送信するメッセージ |

#### 2.2 メッセージの役割分け

- **そのアバター自身の発言** → `role: "assistant"`（システムメッセージ）
- **ユーザ発言 + 他アバターの発言** → `role: "user"`（ユーザメッセージ）

#### 2.3 ユーザメッセージのフォーマット

```
Name: ユーザ
Message:
{メッセージ内容}
```

```
Name: (Avatar) {アバター名}
Message:
{メッセージ内容}
```

#### 2.4 Assistant APIクライアントの拡張

**対象ファイル**: `backend/internal/assistant/thread.go`

**変更内容**:
1. `CreateMessageWithRole`関数を新規追加：
   - `role`パラメータを受け取る（"user" または "assistant"）
   - 既存の`CreateMessage`は内部で`role: "user"`として実装

2. `CreateMessageRequest`構造体に`Role`フィールドを追加（すでに存在）

**テストファイル**: `backend/internal/assistant/thread_test.go`

#### 2.5 メッセージフォーマッタの作成

**新規ファイル**: `backend/internal/logic/message_formatter.go`

**関数**:
```go
// FormatUserMessage formats a user's message for OpenAI API
func FormatUserMessage(content string) string

// FormatAvatarMessage formats another avatar's message for OpenAI API
func FormatAvatarMessage(avatarName, content string) string
```

**テストファイル**: `backend/internal/logic/message_formatter_test.go`

#### 2.6 generateResponse関数の更新

**対象ファイル**: `backend/internal/watcher/avatar_watcher.go`

**変更内容**:
1. メッセージを送信する前に、以下の処理を追加：
   - データベースから過去のメッセージを取得
   - 自身のアバター発言と他の発言を分離
   - 他の発言にはフォーマットを適用してユーザメッセージとして追加
   - 自身の過去の発言はシステムメッセージとして追加

**注意**: OpenAI Assistant APIでは、スレッドにメッセージを追加する際、過去のメッセージ履歴は自動的に保持される。そのため、既存のスレッドに対してはメッセージの追加方法を変更するアプローチではなく、新しいRunを作成する際のコンテキストとして情報を渡す方法を検討する。

#### 2.7 代替案: Additional Instructionsの活用

OpenAI Assistants APIでは、`CreateRun`時に`additional_instructions`パラメータを使用して、実行時のコンテキストを追加できる。

**対象ファイル**: `backend/internal/assistant/thread.go`

**変更内容**:
1. `CreateRunRequest`構造体に`AdditionalInstructions`フィールドを追加
2. `CreateRunWithContext`関数を新規追加：
   - 会話の履歴をフォーマットして`additional_instructions`に含める

**テストファイル**: `backend/internal/assistant/thread_test.go`

---

### フェーズ3: メッセージ監視タイミングのランダム化

#### 3.1 ランダム間隔の実装

**対象ファイル**: `backend/internal/watcher/avatar_watcher.go`

**変更内容**:
1. `run`関数内のtickerをランダム間隔に変更
2. 各チェック後に次の間隔を5秒〜20秒でランダム生成

**実装例**:
```go
func (w *AvatarWatcher) getRandomInterval() time.Duration {
    min := 5 * time.Second
    max := 20 * time.Second
    return min + time.Duration(rand.Int63n(int64(max-min)))
}

func (w *AvatarWatcher) run() {
    defer w.wg.Done()
    
    // Initialize lastMessageID
    ...
    
    for {
        select {
        case <-w.ctx.Done():
            return
        case <-time.After(w.getRandomInterval()):
            if err := w.checkAndRespond(); err != nil {
                log.Printf(...)
            }
        }
    }
}
```

**テストファイル**: `backend/internal/watcher/avatar_watcher_test.go`

#### 3.2 WatcherManagerの更新

**対象ファイル**: `backend/internal/watcher/manager.go`

**変更内容**:
1. `interval`フィールドの使用を見直し
2. 必要に応じて`minInterval`と`maxInterval`フィールドに変更
3. または、`interval`を基準値として使用し、各Watcherがランダム化する

---

## TDD実装順序

各変更はTDDアプローチで実装する。

### ステップ1: メッセージフォーマッタ（フェーズ2.5）

1. **Red**: `message_formatter_test.go`で`FormatUserMessage`と`FormatAvatarMessage`のテストを作成
2. **Green**: `message_formatter.go`を実装
3. **Refactor**: 必要に応じてリファクタリング

### ステップ2: ランダム間隔（フェーズ3.1）

1. **Red**: `avatar_watcher_test.go`にランダム間隔の範囲テストを追加
2. **Green**: `getRandomInterval`関数を実装
3. **Green**: `run`関数を更新
4. **Refactor**: 既存テストが通ることを確認

### ステップ3: プロンプトコンテキスト拡張（フェーズ1）

1. **Red**: `avatar_watcher_test.go`で新しいプロンプト形式のテストを作成
2. **Green**: `AvatarWatcher`構造体とコンストラクタを拡張
3. **Green**: `buildJudgmentPrompt`を更新
4. **Refactor**: `manager.go`の`StartWatcher`を更新

### ステップ4: CreateRun拡張（フェーズ2.7）

1. **Red**: `thread_test.go`で`CreateRunWithContext`のテストを作成
2. **Green**: `CreateRunRequest`に`AdditionalInstructions`を追加
3. **Green**: `CreateRunWithContext`関数を実装
4. **Refactor**: 既存のRun作成処理との整合性を確認

### ステップ5: generateResponse更新（フェーズ2.6）

1. **Red**: `avatar_watcher_test.go`でコンテキスト付きRun作成のテストを追加
2. **Green**: `generateResponse`を更新し、履歴を含むRunを作成
3. **Refactor**: エラーハンドリングの改善

---

## ファイル変更一覧

| ファイル | 変更種別 | 概要 |
|----------|----------|------|
| `backend/internal/logic/message_formatter.go` | 新規 | メッセージフォーマット関数 |
| `backend/internal/logic/message_formatter_test.go` | 新規 | フォーマッタのテスト |
| `backend/internal/assistant/thread.go` | 修正 | CreateRunWithContext追加 |
| `backend/internal/assistant/thread_test.go` | 修正 | 新機能のテスト追加 |
| `backend/internal/watcher/avatar_watcher.go` | 修正 | プロンプト拡張、ランダム間隔 |
| `backend/internal/watcher/avatar_watcher_test.go` | 修正 | 新機能のテスト追加 |
| `backend/internal/watcher/manager.go` | 修正 | コンテキスト情報の取得・伝達 |
| `backend/internal/watcher/manager_test.go` | 修正 | 新機能のテスト追加 |

---

## 完了条件

1. すべての単体テストが成功すること
2. すべての結合テストが成功すること
3. `./build.sh` がオプションなしで成功すること
4. アバターのプロンプトに以下が含まれること：
   - 自身のアバター名
   - 同じチャットルームに参加しているアバター名一覧（ユーザ含む）
   - 議題（チャットルーム名）
5. メッセージがフォーマットに従って区別されていること
6. 監視間隔が5秒〜20秒のランダムになっていること

---

## リスクと対策

| リスク | 影響度 | 対策 |
|--------|--------|------|
| OpenAI Assistant APIの仕様変更 | 高 | API呼び出しをラップし、変更時の影響を局所化 |
| ランダム間隔によるテストの不安定化 | 中 | テスト時は固定シードまたはモック化 |
| メッセージ履歴が長くなった場合のパフォーマンス | 中 | 履歴の最大件数を設定可能にする |

