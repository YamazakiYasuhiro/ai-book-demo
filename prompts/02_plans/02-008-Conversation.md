# 会話に関する処理の変更計画

## 概要

アバターのプロンプト改善、ユーザメッセージの即時表示、中断ボタンの追加、レイアウトの改善を実現します。

## 目標

1. アバターのプロンプトに「`Name: ユーザ` となっているメッセージがユーザの意見です。アバターはこれを最重視して発言をするように」という要素を追加
2. ユーザの発言を即座にWebUIに表示
3. 中断ボタンを追加し、アバターのGoroutineに思考停止シグナルを送り、現在リクエストしているLLMの処理を強制中断
4. レイアウトを改善し、ブラウザのスクロールを防ぎ、メッセージ一覧を固定高さでスクロール可能にする

## Phase 1: アバターのプロンプト変更

### Step 1.1: アバター作成時のプロンプトにユーザ重視の指示を追加

**目的**: アバターがユーザの意見を最重視するようにプロンプトを変更

**変更ファイル**: `backend/internal/api/avatar.go`

**変更内容**:
- `Create`メソッドで、プロンプトにユーザ重視の指示を追加
- プロンプトの先頭または適切な位置に以下のテキストを追加:
  ```
  【重要】`Name: ユーザ` となっているメッセージがユーザの意見です。あなたはこれを最重視して発言をする必要があります。ユーザの意見を尊重し、それに基づいて応答してください。
  ```

**実装詳細**:
- `Create`メソッド内で、`req.Prompt`を受け取った後、上記のテキストを追加
- 既存のプロンプトとの整合性を保つため、追加テキストはプロンプトの先頭に配置

**テスト**:
- アバター作成時にプロンプトが正しく追加されていることを確認
- OpenAI Assistant APIに送信されるプロンプトをログで確認

### Step 1.2: 既存アバターのプロンプト更新機能（オプション）

**目的**: 既存のアバターにもプロンプトを更新できるようにする

**変更ファイル**: `backend/internal/api/avatar.go`

**変更内容**:
- `Update`メソッドでも同様にプロンプトを更新する際に、ユーザ重視の指示が含まれているかチェック
- 含まれていない場合は自動的に追加（オプション）

**考慮事項**:
- 既存アバターのプロンプトを自動的に変更するか、手動更新のみとするかは要件による
- 今回は新規作成時のみ対応し、既存アバターは手動更新とする

## Phase 2: ユーザメッセージの即時表示

### Step 2.1: フロントエンドでの即時表示実装

**目的**: ユーザがメッセージを送信したら、APIレスポンスを待たずに即座に表示

**変更ファイル**: `frontend/src/context/AppContext.tsx`

**変更内容**:
- `sendMessage`関数を修正
- APIリクエストを送信する前に、ユーザメッセージをローカル状態に追加（楽観的更新）
- APIレスポンスが返ってきたら、実際のメッセージIDで更新
- エラーが発生した場合は、楽観的に追加したメッセージを削除

**実装詳細**:
```typescript
const sendMessage = useCallback(async (content: string) => {
  if (!state.currentConversation) return;
  
  // 楽観的にユーザメッセージを追加
  const optimisticMessage: Message = {
    id: Date.now(), // 一時的なID
    sender_type: 'user',
    content: content,
    created_at: new Date().toISOString(),
  };
  
  setState(s => ({ ...s, messages: [...s.messages, optimisticMessage] }));
  
  try {
    setLoading(true);
    const response = await api.sendMessage(state.currentConversation.id, content);
    
    // 楽観的メッセージを実際のメッセージに置き換え
    setState(s => ({
      ...s,
      messages: s.messages.map(m => 
        m.id === optimisticMessage.id ? response.user_message : m
      ).concat(response.avatar_responses || [])
    }));
  } catch (err) {
    // エラー時は楽観的メッセージを削除
    setState(s => ({
      ...s,
      messages: s.messages.filter(m => m.id !== optimisticMessage.id)
    }));
    setError(err instanceof Error ? err.message : 'Failed to send message');
    throw err;
  } finally {
    setLoading(false);
  }
}, [state.currentConversation]);
```

**テスト**:
- ユーザメッセージが即座に表示されることを確認
- APIレスポンス後に正しいメッセージIDで更新されることを確認
- エラー時に楽観的メッセージが削除されることを確認

### Step 2.2: バックエンドのレスポンス最適化（オプション）

**目的**: バックエンドのレスポンスを最適化して、ユーザメッセージの保存とレスポンスを高速化

**変更ファイル**: `backend/internal/api/conversation.go`

**変更内容**:
- `SendMessage`メソッドで、ユーザメッセージの保存を最優先
- アバターへのメッセージ送信は非同期で処理（既にWatcherManagerが動作している場合は不要）
- レスポンスを早期に返す

**考慮事項**:
- 現在はWatcherManagerが動作している場合、同期レスポンスは生成されない
- ユーザメッセージの保存は既に最優先で処理されているため、大きな変更は不要

## Phase 3: 中断ボタンの実装

### Step 3.1: OpenAI Assistants APIのRunキャンセル機能の実装

**目的**: 実行中のRunをキャンセルできるようにする

**変更ファイル**: `backend/internal/assistant/thread.go`

**変更内容**:
- `CancelRun`メソッドを追加
- OpenAI Assistants APIの`POST /threads/{thread_id}/runs/{run_id}/cancel`エンドポイントを呼び出す

**実装詳細**:
```go
// CancelRun cancels a running run
func (c *Client) CancelRun(threadID, runID string) error {
    log.Printf("[Assistant] CancelRun started thread_id=%s run_id=%s", threadID, runID)
    
    req, err := http.NewRequest(http.MethodPost, 
        baseURL+"/threads/"+threadID+"/runs/"+runID+"/cancel", nil)
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }
    
    c.setHeaders(req)
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return c.handleError(resp)
    }
    
    log.Printf("[Assistant] CancelRun completed thread_id=%s run_id=%s", threadID, runID)
    return nil
}
```

**テスト**:
- 実行中のRunをキャンセルできることを確認
- キャンセル後のRunのステータスが`cancelled`になることを確認

### Step 3.2: AvatarWatcherでのRunキャンセル対応

**目的**: AvatarWatcherが実行中のRunをキャンセルできるようにする

**変更ファイル**: `backend/internal/watcher/avatar_watcher.go`

**変更内容**:
- `AvatarWatcher`に実行中のRunIDを追跡するフィールドを追加
- `generateResponse`メソッドで、Runを作成したらRunIDを保存
- `Stop`メソッドで、実行中のRunがあればキャンセル
- contextキャンセル時にRunをキャンセルする処理を追加

**実装詳細**:
- `AvatarWatcher`構造体に`currentRunID`と`currentThreadID`フィールドを追加（mutexで保護）
- `generateResponse`でRunを作成したら、これらのフィールドを更新
- `Stop`メソッドまたはcontextキャンセル時に、実行中のRunをキャンセル

**考慮事項**:
- 複数のRunが同時に実行されることはない（`WaitForActiveRunsToComplete`で待機）
- 1つのRunIDのみを追跡すれば十分

### Step 3.3: WatcherManagerでの中断機能の実装

**目的**: 会話に所属する全アバターのWatcherを中断できるようにする

**変更ファイル**: `backend/internal/watcher/manager.go`

**変更内容**:
- `InterruptRoomWatchers`メソッドを追加
- 指定された会話の全Watcherに対して、実行中のRunをキャンセルし、Watcherを停止

**実装詳細**:
```go
// InterruptRoomWatchers interrupts all watchers for a conversation
// This cancels any active LLM runs and stops the watchers
func (m *WatcherManager) InterruptRoomWatchers(conversationID int64) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    interruptedCount := 0
    for key, watcher := range m.watchers {
        if key.ConversationID == conversationID {
            watcher.Interrupt() // 新しいメソッドを追加
            interruptedCount++
        }
    }
    
    log.Printf("[WatcherManager] InterruptRoomWatchers completed conversation_id=%d interrupted_count=%d",
        conversationID, interruptedCount)
    return nil
}
```

**変更ファイル**: `backend/internal/watcher/avatar_watcher.go`

**変更内容**:
- `Interrupt`メソッドを追加
- 実行中のRunをキャンセルし、contextをキャンセル

### Step 3.4: APIエンドポイントの追加

**目的**: 中断ボタンから呼び出せるAPIエンドポイントを追加

**変更ファイル**: `backend/internal/api/conversation.go`

**変更内容**:
- `Interrupt`メソッドを追加
- `POST /api/conversations/{id}/interrupt`エンドポイントを実装
- `WatcherManager`の`InterruptRoomWatchers`を呼び出す

**変更ファイル**: `backend/internal/api/router.go`

**変更内容**:
- 新しいエンドポイントをルーターに追加

### Step 3.5: フロントエンドでの中断ボタン実装

**目的**: WebUIに中断ボタンを追加

**変更ファイル**: `frontend/src/services/api.ts`

**変更内容**:
- `interruptConversation`メソッドを追加

**変更ファイル**: `frontend/src/components/ChatArea.tsx`

**変更内容**:
- 中断ボタンを追加（入力エリアの近くに配置）
- 中断ボタンをクリックしたら、`interruptConversation` APIを呼び出す
- ローディング状態を表示

**実装詳細**:
- 中断ボタンは、会話が選択されている時のみ表示
- ボタンのスタイルは、警告色（例: 赤色）を使用
- アイコンは「停止」を表す記号（例: ⏹）を使用

**テスト**:
- 中断ボタンをクリックしたら、実行中のアバターの処理が中断されることを確認
- 中断後、新しいメッセージが生成されないことを確認

## Phase 4: レイアウトの改善

### Step 4.1: ChatAreaのレイアウト修正

**目的**: ブラウザのスクロールを防ぎ、固定レイアウトにする

**変更ファイル**: `frontend/src/components/ChatArea.tsx`

**変更内容**:
- `container`スタイルを修正して、`height: 100vh`を設定
- `chatContent`を`flex: 1`にして、残りのスペースを使用
- `inputArea`を固定位置（画面下部）に配置
- `messageArea`を`flex: 1`にして、残りのスペースを使用し、`overflow: hidden`を設定

**実装詳細**:
```typescript
const styles = StyleSheet.create({
  container: {
    flex: 1,
    height: '100vh', // Web専用
    flexDirection: 'column',
    overflow: 'hidden',
  },
  chatContent: {
    flex: 1,
    flexDirection: 'column',
    overflow: 'hidden',
    minHeight: 0, // Web専用
  },
  messageArea: {
    flex: 1,
    overflow: 'hidden',
    minHeight: 0, // Web専用
  },
  inputArea: {
    flexDirection: 'row',
    padding: 16,
    backgroundColor: '#1e293b',
    borderTopWidth: 1,
    borderTopColor: '#334155',
    alignItems: 'center',
    // 固定位置（画面下部）
    position: 'fixed', // Web専用
    bottom: 0,
    left: 0,
    right: 0,
  },
});
```

**考慮事項**:
- React Native for Webでは、`position: 'fixed'`が使用可能
- `minHeight: 0`は、flexboxでスクロールを有効にするために必要

### Step 4.2: MessageListのレイアウト修正

**目的**: メッセージ一覧を固定高さでスクロール可能にする

**変更ファイル**: `frontend/src/components/MessageList.tsx`

**変更内容**:
- `container`スタイルを修正して、`flex: 1`を設定
- `contentContainerStyle`を修正して、パディングを維持
- `ScrollView`の`style`で高さを制限

**実装詳細**:
```typescript
const styles = StyleSheet.create({
  container: {
    flex: 1,
    minHeight: 0, // Web専用
    overflow: 'auto', // Web専用
  },
  content: {
    padding: 16,
    paddingBottom: 24,
    // flexGrowを削除して、コンテンツが拡張しないようにする
  },
});
```

**考慮事項**:
- `contentContainerStyle`に`flexGrow: 1`がある場合は削除
- `ScrollView`が親の高さを超えないようにする

### Step 4.3: レイアウトのテストと調整

**目的**: レイアウトが正しく動作することを確認し、必要に応じて調整

**テスト項目**:
- ブラウザのスクロールバーが表示されないことを確認
- メッセージ一覧が固定高さでスクロール可能であることを確認
- 入力フォームが画面下部に固定されていることを確認
- 新しいメッセージが追加されても、ブラウザの高さが変わらないことを確認
- メッセージが少ない場合でも、レイアウトが崩れないことを確認

**調整事項**:
- メッセージ一覧の高さが適切か確認
- 入力エリアの高さが適切か確認
- レスポンシブデザインへの対応（オプション）

## 実装順序

1. **Phase 1**: アバターのプロンプト変更（最も簡単）
2. **Phase 4**: レイアウトの改善（UIの改善）
3. **Phase 2**: ユーザメッセージの即時表示（UXの改善）
4. **Phase 3**: 中断ボタンの実装（最も複雑）

## テスト計画

### 単体テスト

1. **アバターのプロンプト変更**
   - `backend/internal/api/avatar_test.go`にテストを追加
   - プロンプトにユーザ重視の指示が追加されていることを確認

2. **中断機能**
   - `backend/internal/assistant/thread_test.go`に`CancelRun`のテストを追加
   - `backend/internal/watcher/avatar_watcher_test.go`に`Interrupt`のテストを追加
   - `backend/internal/watcher/manager_test.go`に`InterruptRoomWatchers`のテストを追加

3. **APIエンドポイント**
   - `backend/internal/api/conversation_test.go`に`Interrupt`のテストを追加

### 結合テスト

1. **中断機能の統合テスト**
   - `tests/integration/`に中断機能のテストを追加
   - アバターが応答中に中断ボタンをクリックしたら、処理が中断されることを確認

2. **レイアウトのテスト**
   - ブラウザで実際に動作を確認
   - 様々な画面サイズでテスト

## 注意事項

1. **プロンプトの変更**
   - 既存のアバターのプロンプトを自動的に変更しない（手動更新のみ）
   - プロンプトの追加テキストは、既存のプロンプトと整合性を保つ

2. **中断機能**
   - OpenAI Assistants APIのRunキャンセルは、実行中のRunに対してのみ有効
   - 既に完了したRunはキャンセルできない
   - キャンセル後も、部分的に生成されたメッセージが残る可能性がある

3. **レイアウト**
   - React Native for Webの制限を考慮
   - ブラウザの互換性を確認
   - モバイルデバイスでの動作も確認（オプション）

4. **パフォーマンス**
   - 楽観的更新による状態管理の複雑さを考慮
   - メッセージが大量にある場合のパフォーマンスを確認

## 完了条件

1. アバター作成時に、プロンプトにユーザ重視の指示が自動的に追加される
2. ユーザがメッセージを送信したら、即座にWebUIに表示される
3. 中断ボタンをクリックしたら、実行中のアバターの処理が中断される
4. ブラウザのスクロールバーが表示されず、メッセージ一覧が固定高さでスクロール可能である
5. すべての単体テストと結合テストがパスする
6. `./build.sh`が成功する

