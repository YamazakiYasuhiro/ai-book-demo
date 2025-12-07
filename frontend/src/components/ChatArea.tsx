import React, { useState, useRef } from 'react';
import {
  StyleSheet,
  View,
  Text,
  TextInput,
  TouchableOpacity,
  ScrollView,
  Modal,
} from 'react-native';
import { useApp } from '../context/AppContext';
import { api } from '../services/api';
import MessageList from './MessageList';
import ConversationAvatarModal from './ConversationAvatarModal';

const ChatArea: React.FC = () => {
  const {
    conversations,
    currentConversation,
    avatars,
    createConversation,
    selectConversation,
    deleteConversation,
    sendMessage,
    loading,
  } = useApp();

  const [interrupting, setInterrupting] = useState(false);

  const [message, setMessage] = useState('');
  const [showNewChat, setShowNewChat] = useState(false);
  const [showAvatarModal, setShowAvatarModal] = useState(false);
  const [newChatTitle, setNewChatTitle] = useState('');
  const [selectedAvatars, setSelectedAvatars] = useState<number[]>([]);
  const inputRef = useRef<TextInput>(null);

  const handleSend = async () => {
    if (!message.trim() || !currentConversation) return;

    const content = message.trim();
    setMessage('');

    try {
      await sendMessage(content);
    } catch {
      // エラーはコンテキストで処理
    }
  };

  const handleCreateChat = async () => {
    if (!newChatTitle.trim()) return;

    try {
      await createConversation(newChatTitle.trim(), selectedAvatars);
      setShowNewChat(false);
      setNewChatTitle('');
      setSelectedAvatars([]);
    } catch {
      // エラーはコンテキストで処理
    }
  };

  const toggleAvatarSelection = (id: number) => {
    setSelectedAvatars((prev) =>
      prev.includes(id) ? prev.filter((a) => a !== id) : [...prev, id]
    );
  };

  // 会話削除ハンドラー
  const handleDeleteConversation = (conversationId: number, e: React.BaseSyntheticEvent) => {
    e.stopPropagation();
    deleteConversation(conversationId);
  };

  const handleInterrupt = async () => {
    if (!currentConversation || interrupting) return;

    try {
      setInterrupting(true);
      await api.interruptConversation(currentConversation.id);
    } catch (err) {
      console.error('Failed to interrupt conversation:', err);
    } finally {
      setInterrupting(false);
    }
  };

  return (
    <View style={styles.container}>
      {/* 会話リストヘッダー */}
      <View style={styles.convHeader}>
        <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.convList}>
          {conversations.map((conv) => (
            <View key={conv.id} style={styles.convTabWrapper}>
              <TouchableOpacity
                style={[
                  styles.convTab,
                  currentConversation?.id === conv.id && styles.convTabActive,
                ]}
                onPress={() => selectConversation(conv)}
              >
                <Text
                  style={[
                    styles.convTabText,
                    currentConversation?.id === conv.id && styles.convTabTextActive,
                  ]}
                  numberOfLines={1}
                >
                  {conv.title}
                </Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={styles.deleteTabButton}
                onPress={(e) => handleDeleteConversation(conv.id, e)}
              >
                <Text style={styles.deleteTabButtonText}>×</Text>
              </TouchableOpacity>
            </View>
          ))}
        </ScrollView>
        <TouchableOpacity
          style={styles.newChatButton}
          onPress={() => setShowNewChat(true)}
        >
          <Text style={styles.newChatButtonText}>+ New Chat</Text>
        </TouchableOpacity>
      </View>

      {/* チャットコンテンツ */}
      {currentConversation ? (
        <View style={styles.chatContent}>
          <View style={styles.messageArea}>
            <MessageList />
          </View>
          {/* 入力エリアのスペーサー（固定位置のための高さ確保） */}
          <View style={styles.inputSpacer} />
          <View style={styles.inputArea}>
            <TouchableOpacity
              style={[styles.interruptButton, interrupting && styles.disabledButton]}
              onPress={handleInterrupt}
              disabled={interrupting}
            >
              <Text style={styles.interruptButtonText}>⏹</Text>
            </TouchableOpacity>
            <TextInput
              ref={inputRef}
              style={styles.input}
              value={message}
              onChangeText={setMessage}
              placeholder="Type a message... (use @avatarname to mention)"
              placeholderTextColor="#64748b"
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
        </View>
      ) : (
        <View style={styles.emptyState}>
          <Text style={styles.emptyTitle}>Welcome to Multi-Avatar Chat</Text>
          <Text style={styles.emptySubtitle}>
            Select a conversation or create a new one to get started
          </Text>
          <TouchableOpacity
            style={styles.createChatButton}
            onPress={() => setShowNewChat(true)}
          >
            <Text style={styles.createChatButtonText}>Create New Chat</Text>
          </TouchableOpacity>
        </View>
      )}

      {/* 新規チャットモーダル */}
      <Modal
        visible={showNewChat}
        transparent
        animationType="fade"
        onRequestClose={() => setShowNewChat(false)}
      >
        <View style={styles.modalOverlay}>
          <View style={styles.modalContent}>
            <Text style={styles.modalTitle}>New Conversation</Text>
            
            <Text style={styles.label}>Title</Text>
            <TextInput
              style={styles.modalInput}
              value={newChatTitle}
              onChangeText={setNewChatTitle}
              placeholder="Conversation title..."
              placeholderTextColor="#64748b"
            />
            
            <Text style={styles.label}>Select Avatars</Text>
            <View style={styles.avatarSelection}>
              {avatars.map((avatar) => (
                <TouchableOpacity
                  key={avatar.id}
                  style={[
                    styles.avatarChip,
                    selectedAvatars.includes(avatar.id) && styles.avatarChipSelected,
                  ]}
                  onPress={() => toggleAvatarSelection(avatar.id)}
                >
                  <Text
                    style={[
                      styles.avatarChipText,
                      selectedAvatars.includes(avatar.id) && styles.avatarChipTextSelected,
                    ]}
                  >
                    {avatar.name}
                  </Text>
                </TouchableOpacity>
              ))}
              {avatars.length === 0 && (
                <Text style={styles.noAvatarsText}>
                  Create some avatars first
                </Text>
              )}
            </View>
            
            <View style={styles.modalActions}>
              <TouchableOpacity
                style={styles.cancelButton}
                onPress={() => setShowNewChat(false)}
              >
                <Text style={styles.cancelButtonText}>Cancel</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={[styles.createButton, loading && styles.disabledButton]}
                onPress={handleCreateChat}
                disabled={loading || !newChatTitle.trim()}
              >
                <Text style={styles.createButtonText}>Create</Text>
              </TouchableOpacity>
            </View>
          </View>
        </View>
      </Modal>

      {/* アバター管理モーダル */}
      <ConversationAvatarModal
        visible={showAvatarModal}
        onClose={() => setShowAvatarModal(false)}
      />
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    flexDirection: 'column',
    overflow: 'hidden',
    // @ts-expect-error - Web専用: flexboxでスクロールを有効にする
    minHeight: 0,
    // @ts-expect-error - Web専用: ブラウザの高さを100%に固定
    height: '100vh',
  },
  convHeader: {
    flexDirection: 'row',
    backgroundColor: '#1e293b',
    borderBottomWidth: 1,
    borderBottomColor: '#334155',
    paddingVertical: 8,
    paddingHorizontal: 12,
    alignItems: 'center',
  },
  convList: {
    flex: 1,
  },
  convTabWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    marginRight: 8,
  },
  convTab: {
    backgroundColor: '#334155',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderTopLeftRadius: 6,
    borderBottomLeftRadius: 6,
    maxWidth: 130,
  },
  convTabActive: {
    backgroundColor: '#3b82f6',
  },
  convTabText: {
    color: '#94a3b8',
    fontSize: 14,
  },
  convTabTextActive: {
    color: '#fff',
    fontWeight: '600',
  },
  deleteTabButton: {
    backgroundColor: '#475569',
    paddingHorizontal: 8,
    paddingVertical: 8,
    borderTopRightRadius: 6,
    borderBottomRightRadius: 6,
  },
  deleteTabButtonText: {
    color: '#94a3b8',
    fontSize: 14,
    fontWeight: 'bold',
  },
  newChatButton: {
    backgroundColor: '#22c55e',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 6,
    marginLeft: 8,
  },
  newChatButtonText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 13,
  },
  chatContent: {
    flex: 1,
    flexDirection: 'column',
    overflow: 'hidden',
    // @ts-expect-error - Web専用: flexboxでスクロールを有効にする
    minHeight: 0,
  },
  messageArea: {
    flex: 1,
    overflow: 'hidden',
    // @ts-expect-error - Web専用: flexboxでスクロールを有効にする
    minHeight: 0,
    // @ts-expect-error - Web専用: 入力エリアの高さ分を確保
    paddingBottom: 80,
  },
  inputSpacer: {
    // @ts-expect-error - Web専用: 入力エリアの高さ分のスペース
    height: 80,
  },
  inputArea: {
    flexDirection: 'row',
    padding: 16,
    backgroundColor: '#1e293b',
    borderTopWidth: 1,
    borderTopColor: '#334155',
    alignItems: 'center',
    // @ts-expect-error - Web専用: 画面下部に固定
    position: 'fixed',
    bottom: 0,
    left: 0,
    right: 0,
  },
  interruptButton: {
    backgroundColor: '#ef4444',
    paddingHorizontal: 12,
    paddingVertical: 10,
    borderRadius: 8,
    marginRight: 8,
    justifyContent: 'center',
    alignItems: 'center',
  },
  interruptButtonText: {
    color: '#fff',
    fontSize: 18,
  },
  input: {
    flex: 1,
    backgroundColor: '#334155',
    borderRadius: 8,
    paddingHorizontal: 16,
    paddingVertical: 12,
    color: '#f8fafc',
    fontSize: 15,
    marginRight: 8,
  },
  avatarManageButton: {
    backgroundColor: '#475569',
    paddingHorizontal: 12,
    paddingVertical: 10,
    borderRadius: 8,
    marginRight: 8,
    justifyContent: 'center',
    alignItems: 'center',
  },
  avatarManageButtonText: {
    fontSize: 18,
  },
  sendButton: {
    backgroundColor: '#3b82f6',
    paddingHorizontal: 20,
    paddingVertical: 12,
    borderRadius: 8,
    justifyContent: 'center',
  },
  sendButtonText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 15,
  },
  disabledButton: {
    opacity: 0.6,
  },
  emptyState: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 24,
  },
  emptyTitle: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#f8fafc',
    marginBottom: 8,
  },
  emptySubtitle: {
    fontSize: 16,
    color: '#94a3b8',
    textAlign: 'center',
    marginBottom: 24,
  },
  createChatButton: {
    backgroundColor: '#3b82f6',
    paddingHorizontal: 24,
    paddingVertical: 12,
    borderRadius: 8,
  },
  createChatButtonText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 16,
  },
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
  },
  modalTitle: {
    fontSize: 20,
    fontWeight: 'bold',
    color: '#f8fafc',
    marginBottom: 20,
  },
  label: {
    fontSize: 14,
    fontWeight: '600',
    color: '#94a3b8',
    marginBottom: 8,
  },
  modalInput: {
    backgroundColor: '#334155',
    borderRadius: 8,
    padding: 12,
    color: '#f8fafc',
    fontSize: 15,
    marginBottom: 16,
  },
  avatarSelection: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginBottom: 20,
  },
  avatarChip: {
    backgroundColor: '#334155',
    paddingHorizontal: 14,
    paddingVertical: 8,
    borderRadius: 20,
    borderWidth: 2,
    borderColor: '#334155',
  },
  avatarChipSelected: {
    borderColor: '#3b82f6',
    backgroundColor: '#1e3a5f',
  },
  avatarChipText: {
    color: '#94a3b8',
    fontSize: 14,
  },
  avatarChipTextSelected: {
    color: '#3b82f6',
    fontWeight: '600',
  },
  noAvatarsText: {
    color: '#64748b',
    fontStyle: 'italic',
  },
  modalActions: {
    flexDirection: 'row',
    justifyContent: 'flex-end',
    gap: 12,
  },
  cancelButton: {
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 6,
  },
  cancelButtonText: {
    color: '#94a3b8',
    fontWeight: '600',
  },
  createButton: {
    backgroundColor: '#3b82f6',
    paddingHorizontal: 20,
    paddingVertical: 10,
    borderRadius: 6,
  },
  createButtonText: {
    color: '#fff',
    fontWeight: '600',
  },
});

export default ChatArea;
