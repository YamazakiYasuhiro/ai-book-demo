# Web UI 改善実装計画書

## 1. 概要

### 1.1 目的

`01-004-UIImprovements.md` に記載された要件に基づき、以下のUI改善を実装する。

1. チャットルームの管理機能の改善（削除ボタン、アバター管理）
2. チャットメッセージ表示の改善（自動スクロール、入力フォーム固定配置）
3. エンターキーの日本語入力（IME）対応
4. リアルタイムメッセージ更新（Server-Sent Events）

### 1.2 現状の課題

| 課題 | 現在の実装 | 目標 |
|------|-----------|------|
| ルーム削除 | 長押しでのみ削除可能 | xボタンをタブに追加 |
| アバター管理 | 作成時のみ選択可能 | 作成後もアバターの追加/退室が可能 |
| メッセージスクロール | 一応自動スクロールあり | より確実な自動スクロール |
| 入力フォーム配置 | スクロール領域内 | 画面下部に固定 |
| エンターキー処理 | 単純なキーイベント | IME変換確定を考慮 |
| リアルタイム更新 | ユーザ発言時のみ更新 | アバター発言時にも自動更新 |

### 1.3 スコープ

**実装する機能:**
- ルームタブに削除ボタン（xマーク）を追加
- アバター管理モーダルの実装
- 入力フォームの固定配置レイアウト
- IME対応のキーイベントハンドリング
- Server-Sent Events (SSE) によるリアルタイム通知
- SSEを受信するフロントエンドの実装

**対象外（将来検討）:**
- WebSocket への移行
- メッセージの編集・削除機能
- タイピングインジケーター

---

## 2. 技術設計

### 2.1 アーキテクチャ概要

```
┌─────────────────────────────────────────────────────────────┐
│                      Frontend (React)                        │
├─────────────────────────────────────────────────────────────┤
│  ┌────────────────┐  ┌────────────────┐  ┌───────────────┐ │
│  │   ChatArea     │  │  MessageList   │  │ AvatarManager │ │
│  │   (updated)    │  │   (updated)    │  │  (as-is)      │ │
│  └────────────────┘  └────────────────┘  └───────────────┘ │
│           │                   │                             │
│           ▼                   ▼                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              AppContext (updated)                        ││
│  │  - SSE connection management                             ││
│  │  - Real-time message handling                            ││
│  │  - Conversation avatar management                        ││
│  └─────────────────────────────────────────────────────────┘│
│                            │                                 │
│                            ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              api.ts (updated)                            ││
│  │  - addAvatarToConversation()                             ││
│  │  - removeAvatarFromConversation()                        ││
│  │  - getConversationAvatars()                              ││
│  │  - subscribeToMessages() [SSE]                           ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Backend (Go)                            │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐│
│  │                   router.go (updated)                    ││
│  │  GET  /api/conversations/{id}/events  [NEW - SSE]        ││
│  └─────────────────────────────────────────────────────────┘│
│                            │                                 │
│                            ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              conversation_events.go [NEW]                ││
│  │  - SSE endpoint handler                                  ││
│  │  - Event broadcasting                                    ││
│  └─────────────────────────────────────────────────────────┘│
│                            │                                 │
│                            ▼                                 │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              event_broadcaster.go [NEW]                  ││
│  │  - Client connection management                          ││
│  │  - Event distribution                                    ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Server-Sent Events (SSE) 設計

#### 2.2.1 イベント種類

| イベント名 | 説明 | ペイロード |
|------------|------|-----------|
| `message` | 新しいメッセージの追加 | `Message` オブジェクト |
| `avatar_joined` | アバターが会話に参加 | `{ avatar_id, avatar_name }` |
| `avatar_left` | アバターが会話から退室 | `{ avatar_id }` |

#### 2.2.2 SSE エンドポイント

```
GET /api/conversations/{id}/events
```

**レスポンス例:**
```
event: message
data: {"id":123,"sender_type":"avatar","sender_id":1,"sender_name":"Alice","content":"Hello!","created_at":"2025-12-06T10:00:00Z"}

event: avatar_joined
data: {"avatar_id":2,"avatar_name":"Bob"}
```

#### 2.2.3 バックエンドイベントブロードキャスター設計

```go
// internal/api/event_broadcaster.go

// EventBroadcaster manages SSE clients and broadcasts events
type EventBroadcaster struct {
    mu      sync.RWMutex
    clients map[int64]map[chan Event]struct{} // conversationID -> clients
}

// Event represents an SSE event
type Event struct {
    Type string      `json:"type"`
    Data interface{} `json:"data"`
}

// Subscribe adds a client to receive events for a conversation
func (b *EventBroadcaster) Subscribe(conversationID int64) chan Event

// Unsubscribe removes a client from receiving events
func (b *EventBroadcaster) Unsubscribe(conversationID int64, ch chan Event)

// Broadcast sends an event to all clients watching a conversation
func (b *EventBroadcaster) Broadcast(conversationID int64, event Event)
```

### 2.3 フロントエンド設計

#### 2.3.1 IME対応キーイベント処理

```typescript
// ChatArea.tsx - IME composition handling

const [isComposing, setIsComposing] = useState(false);

const handleCompositionStart = () => setIsComposing(true);
const handleCompositionEnd = () => setIsComposing(false);

const handleKeyDown = (e: KeyboardEvent) => {
  // Only send on Enter when not composing (IME not active)
  if (e.key === 'Enter' && !isComposing && !e.shiftKey && !loading) {
    e.preventDefault();
    handleSend();
  }
};
```

#### 2.3.2 レイアウト構造（入力フォーム固定）

```
┌───────────────────────────────────────────────┐
│  Conversation Tabs  [Tab1] [Tab2×] [+ New]    │
├───────────────────────────────────────────────┤
│                                               │
│             Message List                      │
│           (ScrollView, flex: 1)               │
│                                               │
│                                               │
├───────────────────────────────────────────────┤
│  [Input Field                    ] [⚙️] [Send] │
│  (Fixed at bottom)                            │
└───────────────────────────────────────────────┘
```

#### 2.3.3 アバター管理モーダル

```typescript
// ConversationAvatarModal component structure
interface ConversationAvatarModalProps {
  visible: boolean;
  conversationId: number;
  onClose: () => void;
}

// Features:
// - List all avatars with checkboxes
// - Show current participation status
// - Add/remove avatars on checkbox toggle
```

#### 2.3.4 API サービス拡張

```typescript
// services/api.ts - New methods

// Get avatars in a conversation
async getConversationAvatars(conversationId: number): Promise<Avatar[]>

// Add avatar to conversation
async addAvatarToConversation(conversationId: number, avatarId: number): Promise<void>

// Remove avatar from conversation
async removeAvatarFromConversation(conversationId: number, avatarId: number): Promise<void>

// Subscribe to conversation events (SSE)
subscribeToMessages(
  conversationId: number,
  onMessage: (message: Message) => void,
  onError?: (error: Error) => void
): () => void  // Returns unsubscribe function
```

#### 2.3.5 AppContext 拡張

```typescript
// context/AppContext.tsx - New state and methods

interface AppContextType extends AppState {
  // Existing methods...
  
  // New: Conversation avatar management
  conversationAvatars: Avatar[];
  loadConversationAvatars: (conversationId: number) => Promise<void>;
  addAvatarToConversation: (avatarId: number) => Promise<void>;
  removeAvatarFromConversation: (avatarId: number) => Promise<void>;
  
  // New: SSE subscription management (internal)
  // Automatically subscribes/unsubscribes when currentConversation changes
}
```

---

## 3. 実装フェーズ

### Phase 1: UI改善（フロントエンドのみ）（優先度: 高）

**目標**: ルーム削除ボタン、入力フォーム固定、IME対応

**タスク**:
1. `ChatArea.tsx` にルームタブの削除ボタン（x）を追加
2. 入力フォームを画面下部に固定配置するレイアウト変更
3. IME対応のキーイベントハンドリング実装
4. 自動スクロールの改善

**成果物**:
- `frontend/src/components/ChatArea.tsx` の更新

### Phase 2: アバター管理機能（優先度: 高）

**目標**: 会話中のアバター追加/退室機能

**タスク**:
1. `api.ts` に新しいAPIメソッド追加
2. アバター管理モーダルコンポーネントの作成
3. `AppContext.tsx` に会話アバター管理機能を追加
4. `ChatArea.tsx` にアバター管理ボタンを追加

**成果物**:
- `frontend/src/services/api.ts` の更新
- `frontend/src/components/ConversationAvatarModal.tsx` の新規作成
- `frontend/src/context/AppContext.tsx` の更新
- `frontend/src/components/ChatArea.tsx` の更新

### Phase 3: SSE基盤（バックエンド）（優先度: 高）

**目標**: Server-Sent Events インフラの構築

**タスク**:
1. `event_broadcaster.go` の実装
2. `conversation_events.go` の実装（SSEエンドポイント）
3. `router.go` にSSEエンドポイント追加
4. 既存のメッセージ送信処理からイベント発行を追加

**成果物**:
- `backend/internal/api/event_broadcaster.go` の新規作成
- `backend/internal/api/event_broadcaster_test.go` の新規作成
- `backend/internal/api/conversation_events.go` の新規作成
- `backend/internal/api/conversation_events_test.go` の新規作成
- `backend/internal/api/router.go` の更新
- `backend/internal/api/conversation.go` の更新（イベント発行追加）

### Phase 4: リアルタイム更新（フロントエンド）（優先度: 高）

**目標**: SSEによるリアルタイムメッセージ更新

**タスク**:
1. `api.ts` にSSE接続メソッド追加
2. `AppContext.tsx` にSSE購読管理を追加
3. 会話切り替え時の購読管理

**成果物**:
- `frontend/src/services/api.ts` の更新
- `frontend/src/context/AppContext.tsx` の更新

### Phase 5: テストと検証（優先度: 高）

**目標**: 実装の動作確認

**タスク**:
1. フロントエンドコンポーネントのテスト作成/更新
2. バックエンドAPIのテスト作成
3. 結合テストの追加
4. 手動検証

**成果物**:
- テストファイルの更新/作成
- 動作確認完了

---

## 4. タスク一覧

| ID | タスク | フェーズ | 依存 | 優先度 | 見積もり |
|----|-------|---------|------|--------|---------|
| T1 | ルームタブに削除ボタン追加 | P1 | - | 高 | 20分 |
| T2 | 入力フォーム固定レイアウト | P1 | - | 高 | 30分 |
| T3 | IME対応キーイベント実装 | P1 | - | 高 | 20分 |
| T4 | 自動スクロール改善 | P1 | - | 中 | 15分 |
| T5 | api.ts アバター管理API追加 | P2 | - | 高 | 20分 |
| T6 | ConversationAvatarModal作成 | P2 | T5 | 高 | 45分 |
| T7 | AppContext アバター管理追加 | P2 | T5 | 高 | 30分 |
| T8 | ChatArea アバター管理ボタン追加 | P2 | T6,T7 | 高 | 20分 |
| T9 | event_broadcaster.go 実装 | P3 | - | 高 | 45分 |
| T10 | conversation_events.go 実装 | P3 | T9 | 高 | 30分 |
| T11 | router.go SSEルート追加 | P3 | T10 | 高 | 10分 |
| T12 | conversation.go イベント発行追加 | P3 | T9 | 高 | 20分 |
| T13 | api.ts SSE接続メソッド追加 | P4 | T10 | 高 | 30分 |
| T14 | AppContext SSE購読管理追加 | P4 | T13 | 高 | 30分 |
| T15 | バックエンドテスト作成 | P5 | T9-T12 | 高 | 45分 |
| T16 | フロントエンドテスト更新 | P5 | T1-T8,T13,T14 | 中 | 30分 |
| T17 | 結合テスト追加 | P5 | T15 | 中 | 30分 |
| T18 | 手動検証 | P5 | T1-T17 | 高 | 30分 |

**合計見積もり時間**: 約8時間

---

## 5. 実装詳細

### 5.1 ChatArea.tsx の更新（Phase 1 & 2）

```tsx
// Key changes to ChatArea.tsx

// 1. IME composition state
const [isComposing, setIsComposing] = useState(false);
const [showAvatarModal, setShowAvatarModal] = useState(false);

// 2. IME event handlers
const handleCompositionStart = () => setIsComposing(true);
const handleCompositionEnd = () => setIsComposing(false);

// 3. Updated key handler with IME support
const handleKeyDown = (e: { nativeEvent: { key: string } }) => {
  if (e.nativeEvent.key === 'Enter' && !isComposing && !loading) {
    handleSend();
  }
};

// 4. Conversation tab with delete button
{conversations.map((conv) => (
  <View key={conv.id} style={styles.convTabWrapper}>
    <TouchableOpacity
      style={[
        styles.convTab,
        currentConversation?.id === conv.id && styles.convTabActive,
      ]}
      onPress={() => selectConversation(conv)}
    >
      <Text style={styles.convTabText} numberOfLines={1}>
        {conv.title}
      </Text>
    </TouchableOpacity>
    <TouchableOpacity
      style={styles.deleteTabButton}
      onPress={() => deleteConversation(conv.id)}
    >
      <Text style={styles.deleteTabButtonText}>×</Text>
    </TouchableOpacity>
  </View>
))}

// 5. Input area with avatar management button
<View style={styles.inputArea}>
  <TextInput
    style={styles.input}
    value={message}
    onChangeText={setMessage}
    placeholder="Type a message... (use @avatarname to mention)"
    placeholderTextColor="#64748b"
    onKeyPress={handleKeyDown}
    onCompositionStart={handleCompositionStart}
    onCompositionEnd={handleCompositionEnd}
  />
  <TouchableOpacity
    style={styles.avatarManageButton}
    onPress={() => setShowAvatarModal(true)}
  >
    <Text style={styles.avatarManageButtonText}>⚙️</Text>
  </TouchableOpacity>
  <TouchableOpacity
    style={[styles.sendButton, loading && styles.disabledButton]}
    onPress={handleSend}
    disabled={loading || !message.trim()}
  >
    <Text style={styles.sendButtonText}>Send</Text>
  </TouchableOpacity>
</View>

// 6. Layout change - container uses flex column with fixed input at bottom
const styles = StyleSheet.create({
  container: {
    flex: 1,
    flexDirection: 'column',
  },
  messageArea: {
    flex: 1,  // Takes remaining space
  },
  inputArea: {
    flexDirection: 'row',
    padding: 16,
    backgroundColor: '#1e293b',
    borderTopWidth: 1,
    borderTopColor: '#334155',
    // No flex: fixed at bottom
  },
  // ... other styles
});
```

### 5.2 ConversationAvatarModal.tsx（新規）

```tsx
// frontend/src/components/ConversationAvatarModal.tsx

import React, { useEffect, useState } from 'react';
import {
  StyleSheet,
  View,
  Text,
  TouchableOpacity,
  Modal,
  ScrollView,
} from 'react-native';
import { useApp } from '../context/AppContext';

interface Props {
  visible: boolean;
  onClose: () => void;
}

const ConversationAvatarModal: React.FC<Props> = ({ visible, onClose }) => {
  const {
    avatars,
    currentConversation,
    conversationAvatars,
    loadConversationAvatars,
    addAvatarToConversation,
    removeAvatarFromConversation,
    loading,
  } = useApp();

  const [participatingIds, setParticipatingIds] = useState<Set<number>>(new Set());

  useEffect(() => {
    if (visible && currentConversation) {
      loadConversationAvatars(currentConversation.id);
    }
  }, [visible, currentConversation, loadConversationAvatars]);

  useEffect(() => {
    setParticipatingIds(new Set(conversationAvatars.map(a => a.id)));
  }, [conversationAvatars]);

  const handleToggle = async (avatarId: number) => {
    if (participatingIds.has(avatarId)) {
      await removeAvatarFromConversation(avatarId);
    } else {
      await addAvatarToConversation(avatarId);
    }
  };

  return (
    <Modal
      visible={visible}
      transparent
      animationType="fade"
      onRequestClose={onClose}
    >
      <View style={styles.modalOverlay}>
        <View style={styles.modalContent}>
          <Text style={styles.modalTitle}>Manage Avatars</Text>
          <Text style={styles.subtitle}>
            Select which avatars participate in this conversation
          </Text>

          <ScrollView style={styles.avatarList}>
            {avatars.map((avatar) => {
              const isParticipating = participatingIds.has(avatar.id);
              return (
                <TouchableOpacity
                  key={avatar.id}
                  style={[
                    styles.avatarItem,
                    isParticipating && styles.avatarItemActive,
                  ]}
                  onPress={() => handleToggle(avatar.id)}
                  disabled={loading}
                >
                  <View style={styles.checkbox}>
                    {isParticipating && <Text style={styles.checkmark}>✓</Text>}
                  </View>
                  <View style={styles.avatarInfo}>
                    <Text style={styles.avatarName}>{avatar.name}</Text>
                    <Text style={styles.avatarPrompt} numberOfLines={1}>
                      {avatar.prompt}
                    </Text>
                  </View>
                </TouchableOpacity>
              );
            })}
          </ScrollView>

          <TouchableOpacity style={styles.closeButton} onPress={onClose}>
            <Text style={styles.closeButtonText}>Done</Text>
          </TouchableOpacity>
        </View>
      </View>
    </Modal>
  );
};

const styles = StyleSheet.create({
  modalOverlay: {
    flex: 1,
    backgroundColor: 'rgba(0, 0, 0, 0.7)',
    justifyContent: 'center',
    alignItems: 'center',
  },
  modalContent: {
    backgroundColor: '#1e293b',
    borderRadius: 12,
    padding: 24,
    width: '90%',
    maxWidth: 450,
    maxHeight: '80%',
  },
  modalTitle: {
    fontSize: 20,
    fontWeight: 'bold',
    color: '#f8fafc',
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 14,
    color: '#94a3b8',
    marginBottom: 16,
  },
  avatarList: {
    maxHeight: 300,
    marginBottom: 16,
  },
  avatarItem: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    backgroundColor: '#334155',
    borderRadius: 8,
    marginBottom: 8,
  },
  avatarItemActive: {
    backgroundColor: '#1e3a5f',
    borderWidth: 1,
    borderColor: '#3b82f6',
  },
  checkbox: {
    width: 24,
    height: 24,
    borderRadius: 4,
    borderWidth: 2,
    borderColor: '#64748b',
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 12,
  },
  checkmark: {
    color: '#3b82f6',
    fontWeight: 'bold',
  },
  avatarInfo: {
    flex: 1,
  },
  avatarName: {
    fontSize: 15,
    fontWeight: '600',
    color: '#f8fafc',
  },
  avatarPrompt: {
    fontSize: 12,
    color: '#94a3b8',
    marginTop: 2,
  },
  closeButton: {
    backgroundColor: '#3b82f6',
    paddingVertical: 12,
    borderRadius: 8,
    alignItems: 'center',
  },
  closeButtonText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 15,
  },
});

export default ConversationAvatarModal;
```

### 5.3 event_broadcaster.go（新規）

```go
// backend/internal/api/event_broadcaster.go

package api

import (
    "encoding/json"
    "log"
    "sync"
)

// Event represents a server-sent event
type Event struct {
    Type string      `json:"type"`
    Data interface{} `json:"data"`
}

// EventBroadcaster manages SSE clients and broadcasts events
type EventBroadcaster struct {
    mu      sync.RWMutex
    clients map[int64]map[chan Event]struct{} // conversationID -> clients
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster() *EventBroadcaster {
    return &EventBroadcaster{
        clients: make(map[int64]map[chan Event]struct{}),
    }
}

// Subscribe adds a client to receive events for a conversation
func (b *EventBroadcaster) Subscribe(conversationID int64) chan Event {
    b.mu.Lock()
    defer b.mu.Unlock()

    ch := make(chan Event, 10) // Buffered channel

    if b.clients[conversationID] == nil {
        b.clients[conversationID] = make(map[chan Event]struct{})
    }
    b.clients[conversationID][ch] = struct{}{}

    log.Printf("[SSE] Client subscribed conversation_id=%d total_clients=%d",
        conversationID, len(b.clients[conversationID]))

    return ch
}

// Unsubscribe removes a client from receiving events
func (b *EventBroadcaster) Unsubscribe(conversationID int64, ch chan Event) {
    b.mu.Lock()
    defer b.mu.Unlock()

    if clients, ok := b.clients[conversationID]; ok {
        delete(clients, ch)
        close(ch)
        if len(clients) == 0 {
            delete(b.clients, conversationID)
        }
    }

    log.Printf("[SSE] Client unsubscribed conversation_id=%d", conversationID)
}

// Broadcast sends an event to all clients watching a conversation
func (b *EventBroadcaster) Broadcast(conversationID int64, event Event) {
    b.mu.RLock()
    clients := b.clients[conversationID]
    b.mu.RUnlock()

    if len(clients) == 0 {
        return
    }

    log.Printf("[SSE] Broadcasting event type=%s conversation_id=%d clients=%d",
        event.Type, conversationID, len(clients))

    for ch := range clients {
        select {
        case ch <- event:
        default:
            // Client channel full, skip
            log.Printf("[SSE] Client channel full, skipping event")
        }
    }
}

// FormatSSE formats an event as SSE data
func FormatSSE(event Event) ([]byte, error) {
    data, err := json.Marshal(event.Data)
    if err != nil {
        return nil, err
    }
    return []byte("event: " + event.Type + "\ndata: " + string(data) + "\n\n"), nil
}
```

### 5.4 conversation_events.go（新規）

```go
// backend/internal/api/conversation_events.go

package api

import (
    "log"
    "net/http"
    "strconv"
)

// ConversationEventsHandler handles SSE connections for conversation events
type ConversationEventsHandler struct {
    broadcaster *EventBroadcaster
}

// NewConversationEventsHandler creates a new handler
func NewConversationEventsHandler(broadcaster *EventBroadcaster) *ConversationEventsHandler {
    return &ConversationEventsHandler{
        broadcaster: broadcaster,
    }
}

// HandleEvents handles GET /api/conversations/{id}/events
func (h *ConversationEventsHandler) HandleEvents(w http.ResponseWriter, r *http.Request) {
    conversationID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
    if err != nil {
        log.Printf("[SSE] Invalid conversation ID err=%v", err)
        http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
        return
    }

    log.Printf("[SSE] New connection request conversation_id=%d", conversationID)

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    // Get flusher
    flusher, ok := w.(http.Flusher)
    if !ok {
        log.Printf("[SSE] Streaming not supported")
        http.Error(w, "Streaming not supported", http.StatusInternalServerError)
        return
    }

    // Subscribe to events
    eventCh := h.broadcaster.Subscribe(conversationID)
    defer h.broadcaster.Unsubscribe(conversationID, eventCh)

    // Send initial connection event
    w.Write([]byte("event: connected\ndata: {}\n\n"))
    flusher.Flush()

    // Listen for events and client disconnect
    ctx := r.Context()
    for {
        select {
        case <-ctx.Done():
            log.Printf("[SSE] Client disconnected conversation_id=%d", conversationID)
            return
        case event, ok := <-eventCh:
            if !ok {
                return
            }
            data, err := FormatSSE(event)
            if err != nil {
                log.Printf("[SSE] Failed to format event err=%v", err)
                continue
            }
            w.Write(data)
            flusher.Flush()
        }
    }
}
```

### 5.5 api.ts の更新（SSE接続）

```typescript
// frontend/src/services/api.ts - additions

// Get avatars participating in a conversation
async getConversationAvatars(conversationId: number): Promise<Avatar[]> {
  return this.request<Avatar[]>(`/conversations/${conversationId}/avatars`);
}

// Add avatar to conversation
async addAvatarToConversation(conversationId: number, avatarId: number): Promise<void> {
  return this.request<void>(`/conversations/${conversationId}/avatars`, {
    method: 'POST',
    body: JSON.stringify({ avatar_id: avatarId }),
  });
}

// Remove avatar from conversation
async removeAvatarFromConversation(conversationId: number, avatarId: number): Promise<void> {
  return this.request<void>(`/conversations/${conversationId}/avatars/${avatarId}`, {
    method: 'DELETE',
  });
}

// Subscribe to conversation events via SSE
subscribeToMessages(
  conversationId: number,
  onMessage: (message: Message) => void,
  onError?: (error: Error) => void
): () => void {
  const eventSource = new EventSource(`${API_BASE}/conversations/${conversationId}/events`);

  eventSource.addEventListener('message', (e) => {
    try {
      const message = JSON.parse(e.data) as Message;
      onMessage(message);
    } catch (err) {
      console.error('Failed to parse message event:', err);
    }
  });

  eventSource.addEventListener('error', () => {
    if (onError) {
      onError(new Error('SSE connection error'));
    }
  });

  // Return unsubscribe function
  return () => {
    eventSource.close();
  };
}
```

### 5.6 router.go の更新

```go
// In setupRoutes(), add:

// SSE event routes
r.mux.HandleFunc("GET /api/conversations/{id}/events", r.eventsHandler.HandleEvents)
```

---

## 6. 注意事項

### 6.1 IME対応について

- `onCompositionStart` / `onCompositionEnd` イベントは、React Native for Web では Web の標準イベントとして機能する
- ネイティブ環境（iOS/Android）では異なる実装が必要になる可能性がある
- 現在の実装はWeb環境向けを想定

### 6.2 SSE接続管理

- 会話を切り替えた場合は、前の接続を閉じて新しい接続を開始する
- アプリがバックグラウンドに移行した場合の接続管理を考慮する
- 接続が切断された場合の再接続ロジックを実装する

### 6.3 パフォーマンス考慮

- SSE接続は1会話につき1接続に制限する
- メッセージリストの大量データ時のスクロールパフォーマンスを監視
- イベントブロードキャスターのメモリリークを防ぐため、切断検知を確実に行う

### 6.4 既存APIとの互換性

- 既存の `sendMessage` API は引き続き即時レスポンスを返す
- SSE経由の通知は「追加」の通知経路として機能
- アバターの非同期応答のみがSSE経由で通知される

---

## 7. 今後の拡張案

### 7.1 短期的な拡張

- タイピングインジケーター（「○○が入力中...」表示）
- 接続状態インジケーター
- 再接続ロジックの改善

### 7.2 長期的な拡張

- WebSocket への移行（双方向通信が必要な場合）
- プッシュ通知対応（モバイルアプリ向け）
- メッセージの既読管理
- ファイル添付機能

