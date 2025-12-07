const API_BASE = '/api';

export interface Avatar {
  id: number;
  name: string;
  prompt: string;
  openai_assistant_id?: string;
  created_at: string;
}

export interface Conversation {
  id: number;
  title: string;
  thread_id?: string;
  created_at: string;
}

export interface Message {
  id: number;
  sender_type: 'user' | 'avatar';
  sender_id?: number;
  sender_name?: string;
  content: string;
  created_at: string;
}

export interface SendMessageResponse {
  user_message: Message;
  avatar_responses?: Message[];
}

// SSEイベント型
export type SSEEventType = 'message' | 'avatar_joined' | 'avatar_left' | 'connected';

export interface SSEMessageEvent {
  type: 'message';
  data: Message;
}

export interface SSEAvatarJoinedEvent {
  type: 'avatar_joined';
  data: { avatar_id: number; avatar_name: string };
}

export interface SSEAvatarLeftEvent {
  type: 'avatar_left';
  data: { avatar_id: number };
}

export type SSEEvent = SSEMessageEvent | SSEAvatarJoinedEvent | SSEAvatarLeftEvent;

class ApiService {
  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const response = await fetch(`${API_BASE}${endpoint}`, {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    });

    if (!response.ok) {
      const error = await response.text();
      throw new Error(error || `HTTP error ${response.status}`);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return response.json();
  }

  // アバターエンドポイント
  async getAvatars(): Promise<Avatar[]> {
    return this.request<Avatar[]>('/avatars');
  }

  async createAvatar(name: string, prompt: string): Promise<Avatar> {
    return this.request<Avatar>('/avatars', {
      method: 'POST',
      body: JSON.stringify({ name, prompt }),
    });
  }

  async updateAvatar(id: number, name: string, prompt: string): Promise<Avatar> {
    return this.request<Avatar>(`/avatars/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ name, prompt }),
    });
  }

  async deleteAvatar(id: number): Promise<void> {
    return this.request<void>(`/avatars/${id}`, {
      method: 'DELETE',
    });
  }

  // 会話エンドポイント
  async getConversations(): Promise<Conversation[]> {
    return this.request<Conversation[]>('/conversations');
  }

  async createConversation(
    title: string,
    avatarIds: number[] = []
  ): Promise<Conversation> {
    return this.request<Conversation>('/conversations', {
      method: 'POST',
      body: JSON.stringify({ title, avatar_ids: avatarIds }),
    });
  }

  async deleteConversation(id: number): Promise<void> {
    return this.request<void>(`/conversations/${id}`, {
      method: 'DELETE',
    });
  }

  // メッセージエンドポイント
  async getMessages(conversationId: number): Promise<Message[]> {
    return this.request<Message[]>(`/conversations/${conversationId}/messages`);
  }

  async sendMessage(conversationId: number, content: string): Promise<SendMessageResponse> {
    return this.request<SendMessageResponse>(`/conversations/${conversationId}/messages`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    });
  }

  async interruptConversation(conversationId: number): Promise<void> {
    return this.request<void>(`/conversations/${conversationId}/interrupt`, {
      method: 'POST',
    });
  }

  // 会話アバター管理エンドポイント
  async getConversationAvatars(conversationId: number): Promise<Avatar[]> {
    return this.request<Avatar[]>(`/conversations/${conversationId}/avatars`);
  }

  async addAvatarToConversation(conversationId: number, avatarId: number): Promise<void> {
    return this.request<void>(`/conversations/${conversationId}/avatars`, {
      method: 'POST',
      body: JSON.stringify({ avatar_id: avatarId }),
    });
  }

  async removeAvatarFromConversation(conversationId: number, avatarId: number): Promise<void> {
    return this.request<void>(`/conversations/${conversationId}/avatars/${avatarId}`, {
      method: 'DELETE',
    });
  }

  // リアルタイム更新のためのSSE購読
  subscribeToMessages(
    conversationId: number,
    onMessage: (message: Message) => void,
    onAvatarJoined?: (data: { avatar_id: number; avatar_name: string }) => void,
    onAvatarLeft?: (data: { avatar_id: number }) => void,
    onError?: (error: Error) => void
  ): () => void {
    const eventSource = new EventSource(`${API_BASE}/conversations/${conversationId}/events`);

    eventSource.addEventListener('message', (e) => {
      try {
        const message = JSON.parse(e.data) as Message;
        onMessage(message);
      } catch (err) {
        console.error('メッセージイベントのパースに失敗:', err);
      }
    });

    eventSource.addEventListener('avatar_joined', (e) => {
      try {
        const data = JSON.parse(e.data) as { avatar_id: number; avatar_name: string };
        onAvatarJoined?.(data);
      } catch (err) {
        console.error('avatar_joinedイベントのパースに失敗:', err);
      }
    });

    eventSource.addEventListener('avatar_left', (e) => {
      try {
        const data = JSON.parse(e.data) as { avatar_id: number };
        onAvatarLeft?.(data);
      } catch (err) {
        console.error('avatar_leftイベントのパースに失敗:', err);
      }
    });

    eventSource.addEventListener('connected', () => {
      console.log('SSE接続完了 conversation_id:', conversationId);
    });

    eventSource.onerror = () => {
      if (onError) {
        onError(new Error('SSE接続エラー'));
      }
    };

    // 購読解除関数を返す
    return () => {
      eventSource.close();
      console.log('SSE切断 conversation_id:', conversationId);
    };
  }
}

export const api = new ApiService();
