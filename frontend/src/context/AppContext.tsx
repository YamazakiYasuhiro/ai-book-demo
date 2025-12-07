import React, { createContext, useContext, useState, useEffect, useCallback, useRef, ReactNode } from 'react';
import { api, Avatar, Conversation, Message } from '../services/api';

interface AppState {
  avatars: Avatar[];
  conversations: Conversation[];
  currentConversation: Conversation | null;
  messages: Message[];
  conversationAvatars: Avatar[];
  loading: boolean;
  error: string | null;
}

interface AppContextType extends AppState {
  // アバターアクション
  loadAvatars: () => Promise<void>;
  createAvatar: (name: string, prompt: string) => Promise<void>;
  updateAvatar: (id: number, name: string, prompt: string) => Promise<void>;
  deleteAvatar: (id: number) => Promise<void>;
  
  // 会話アクション
  loadConversations: () => Promise<void>;
  createConversation: (title: string, avatarIds?: number[]) => Promise<void>;
  selectConversation: (conversation: Conversation | null) => Promise<void>;
  deleteConversation: (id: number) => Promise<void>;
  
  // 会話アバター管理
  loadConversationAvatars: (conversationId: number) => Promise<void>;
  addAvatarToConversation: (avatarId: number) => Promise<void>;
  removeAvatarFromConversation: (avatarId: number) => Promise<void>;
  
  // メッセージアクション
  sendMessage: (content: string) => Promise<void>;
  
  // エラーハンドリング
  clearError: () => void;
}

const AppContext = createContext<AppContextType | undefined>(undefined);

export const useApp = (): AppContextType => {
  const context = useContext(AppContext);
  if (!context) {
    throw new Error('useApp must be used within an AppProvider');
  }
  return context;
};

interface AppProviderProps {
  children: ReactNode;
}

export const AppProvider: React.FC<AppProviderProps> = ({ children }) => {
  const [state, setState] = useState<AppState>({
    avatars: [],
    conversations: [],
    currentConversation: null,
    messages: [],
    conversationAvatars: [],
    loading: false,
    error: null,
  });

  // SSE購読解除関数を保持
  const unsubscribeRef = useRef<(() => void) | null>(null);

  const setLoading = (loading: boolean) => setState(s => ({ ...s, loading }));
  const setError = (error: string | null) => setState(s => ({ ...s, error }));

  // アバターアクション
  const loadAvatars = useCallback(async () => {
    try {
      setLoading(true);
      const avatars = await api.getAvatars();
      setState(s => ({ ...s, avatars: avatars || [] }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load avatars');
    } finally {
      setLoading(false);
    }
  }, []);

  const createAvatar = useCallback(async (name: string, prompt: string) => {
    try {
      setLoading(true);
      await api.createAvatar(name, prompt);
      await loadAvatars();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create avatar');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [loadAvatars]);

  const updateAvatar = useCallback(async (id: number, name: string, prompt: string) => {
    try {
      setLoading(true);
      await api.updateAvatar(id, name, prompt);
      await loadAvatars();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update avatar');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [loadAvatars]);

  const deleteAvatar = useCallback(async (id: number) => {
    try {
      setLoading(true);
      await api.deleteAvatar(id);
      await loadAvatars();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete avatar');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [loadAvatars]);

  // 会話アクション
  const loadConversations = useCallback(async () => {
    try {
      setLoading(true);
      const conversations = await api.getConversations();
      setState(s => ({ ...s, conversations: conversations || [] }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load conversations');
    } finally {
      setLoading(false);
    }
  }, []);

  const createConversation = useCallback(async (title: string, avatarIds: number[] = []) => {
    try {
      setLoading(true);
      const conversation = await api.createConversation(title, avatarIds);
      await loadConversations();
      setState(s => ({ ...s, currentConversation: conversation, messages: [], conversationAvatars: [] }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create conversation');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [loadConversations]);

  const selectConversation = useCallback(async (conversation: Conversation | null) => {
    try {
      setState(s => ({ ...s, currentConversation: conversation, messages: [], conversationAvatars: [] }));
      if (conversation) {
        setLoading(true);
        const messages = await api.getMessages(conversation.id);
        setState(s => ({ ...s, messages: messages || [] }));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load messages');
    } finally {
      setLoading(false);
    }
  }, []);

  const deleteConversation = useCallback(async (id: number) => {
    try {
      setLoading(true);
      await api.deleteConversation(id);
      await loadConversations();
      setState(s => {
        if (s.currentConversation?.id === id) {
          return { ...s, currentConversation: null, messages: [], conversationAvatars: [] };
        }
        return s;
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete conversation');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [loadConversations]);

  // 会話アバター管理
  const loadConversationAvatars = useCallback(async (conversationId: number) => {
    try {
      setLoading(true);
      const avatars = await api.getConversationAvatars(conversationId);
      setState(s => ({ ...s, conversationAvatars: avatars || [] }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load conversation avatars');
    } finally {
      setLoading(false);
    }
  }, []);

  const addAvatarToConversation = useCallback(async (avatarId: number) => {
    if (!state.currentConversation) return;
    
    try {
      setLoading(true);
      await api.addAvatarToConversation(state.currentConversation.id, avatarId);
      await loadConversationAvatars(state.currentConversation.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add avatar to conversation');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [state.currentConversation, loadConversationAvatars]);

  const removeAvatarFromConversation = useCallback(async (avatarId: number) => {
    if (!state.currentConversation) return;
    
    try {
      setLoading(true);
      await api.removeAvatarFromConversation(state.currentConversation.id, avatarId);
      await loadConversationAvatars(state.currentConversation.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove avatar from conversation');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [state.currentConversation, loadConversationAvatars]);

  // メッセージアクション
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
        messages: s.messages
          .map(m => (m.id === optimisticMessage.id ? response.user_message : m))
          .concat(response.avatar_responses || []),
      }));
    } catch (err) {
      // エラー時は楽観的メッセージを削除
      setState(s => ({
        ...s,
        messages: s.messages.filter(m => m.id !== optimisticMessage.id),
      }));
      setError(err instanceof Error ? err.message : 'Failed to send message');
      throw err;
    } finally {
      setLoading(false);
    }
  }, [state.currentConversation]);

  const clearError = useCallback(() => setError(null), []);

  // SSE購読管理 - 会話が変更されたときに購読
  useEffect(() => {
    // 以前の購読をクリーンアップ
    if (unsubscribeRef.current) {
      unsubscribeRef.current();
      unsubscribeRef.current = null;
    }

    // 新しい会話を購読
    if (state.currentConversation) {
      const conversationId = state.currentConversation.id;
      
      const unsubscribe = api.subscribeToMessages(
        conversationId,
        // 新しいメッセージ受信時
        (message: Message) => {
          setState(s => {
            // 重複を避けるため、既存のメッセージかチェック
            const exists = s.messages.some(m => m.id === message.id);
            if (exists) return s;
            return { ...s, messages: [...s.messages, message] };
          });
        },
        // アバター参加時
        (data) => {
          console.log('アバター参加:', data);
          // 会話アバターを再読み込み
          loadConversationAvatars(conversationId);
        },
        // アバター退室時
        (data) => {
          console.log('アバター退室:', data);
          // 会話アバターを再読み込み
          loadConversationAvatars(conversationId);
        },
        // エラー時
        (error) => {
          console.error('SSEエラー:', error);
          // SSE接続エラーはユーザーに表示しない
          // ユーザーがメッセージ送信時にはポーリングで動作する
        }
      );

      unsubscribeRef.current = unsubscribe;
    }

    // アンマウント時または会話変更時にクリーンアップ
    return () => {
      if (unsubscribeRef.current) {
        unsubscribeRef.current();
        unsubscribeRef.current = null;
      }
    };
  }, [state.currentConversation, loadConversationAvatars]);

  // 初期読み込み
  useEffect(() => {
    loadAvatars();
    loadConversations();
  }, [loadAvatars, loadConversations]);

  const value: AppContextType = {
    ...state,
    loadAvatars,
    createAvatar,
    updateAvatar,
    deleteAvatar,
    loadConversations,
    createConversation,
    selectConversation,
    deleteConversation,
    loadConversationAvatars,
    addAvatarToConversation,
    removeAvatarFromConversation,
    sendMessage,
    clearError,
  };

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
};
