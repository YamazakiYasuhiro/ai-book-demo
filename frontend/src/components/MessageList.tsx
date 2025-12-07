import React, { useRef, useEffect } from 'react';
import { StyleSheet, View, Text, ScrollView } from 'react-native';
import { useApp } from '../context/AppContext';

const MessageList: React.FC = () => {
  const { messages, avatars } = useApp();
  const scrollViewRef = useRef<ScrollView>(null);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    setTimeout(() => {
      scrollViewRef.current?.scrollToEnd({ animated: true });
    }, 100);
  }, [messages]);

  const getAvatarColor = (senderId: number | undefined): string => {
    if (!senderId) return '#3b82f6';
    const colors = ['#f59e0b', '#10b981', '#8b5cf6', '#ec4899', '#06b6d4', '#f97316'];
    return colors[senderId % colors.length];
  };

  const getSenderName = (message: typeof messages[0]): string => {
    if (message.sender_type === 'user') {
      return 'You';
    }
    if (message.sender_name) {
      return message.sender_name;
    }
    if (message.sender_id) {
      const avatar = avatars.find((a) => a.id === message.sender_id);
      if (avatar) return avatar.name;
    }
    return 'Avatar';
  };

  const formatTime = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  // Highlight @mentions in message content
  const renderContent = (content: string) => {
    const parts = content.split(/(@\w+)/g);
    return parts.map((part, index) => {
      if (part.startsWith('@')) {
        return (
          <Text key={index} style={styles.mention}>
            {part}
          </Text>
        );
      }
      return <Text key={index}>{part}</Text>;
    });
  };

  return (
    <ScrollView
      ref={scrollViewRef}
      style={styles.container}
      contentContainerStyle={styles.content}
    >
      {messages.map((message) => {
        const isUser = message.sender_type === 'user';
        const avatarColor = isUser ? '#3b82f6' : getAvatarColor(message.sender_id);

        return (
          <View
            key={message.id}
            style={[
              styles.messageWrapper,
              isUser ? styles.userWrapper : styles.avatarWrapper,
            ]}
          >
            <View
              style={[
                styles.messageBubble,
                isUser ? styles.userBubble : styles.avatarBubble,
              ]}
            >
              <View style={styles.messageHeader}>
                <View
                  style={[styles.avatarBadge, { backgroundColor: avatarColor }]}
                >
                  <Text style={styles.avatarInitial}>
                    {getSenderName(message).charAt(0).toUpperCase()}
                  </Text>
                </View>
                <Text style={styles.senderName}>{getSenderName(message)}</Text>
                <Text style={styles.timestamp}>{formatTime(message.created_at)}</Text>
              </View>
              <Text style={styles.messageText}>{renderContent(message.content)}</Text>
            </View>
          </View>
        );
      })}
      {messages.length === 0 && (
        <View style={styles.emptyContainer}>
          <Text style={styles.emptyText}>
            No messages yet. Start the conversation!
          </Text>
          <Text style={styles.emptyHint}>
            Tip: Use @avatarname to mention specific avatars
          </Text>
        </View>
      )}
    </ScrollView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    // @ts-expect-error - Web専用: flexboxでスクロールを有効にする
    minHeight: 0,
    overflow: 'auto',
    // @ts-expect-error - Web専用: 高さを制限してスクロール可能にする
    maxHeight: '100%',
  },
  content: {
    padding: 16,
    paddingBottom: 24,
    // flexGrowを削除して、コンテンツが拡張しないようにする
  },
  messageWrapper: {
    marginBottom: 16,
  },
  userWrapper: {
    alignItems: 'flex-end',
  },
  avatarWrapper: {
    alignItems: 'flex-start',
  },
  messageBubble: {
    maxWidth: '80%',
    borderRadius: 12,
    padding: 12,
  },
  userBubble: {
    backgroundColor: '#1e3a5f',
    borderBottomRightRadius: 4,
  },
  avatarBubble: {
    backgroundColor: '#334155',
    borderBottomLeftRadius: 4,
  },
  messageHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 6,
  },
  avatarBadge: {
    width: 24,
    height: 24,
    borderRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 8,
  },
  avatarInitial: {
    color: '#fff',
    fontSize: 12,
    fontWeight: 'bold',
  },
  senderName: {
    color: '#f8fafc',
    fontWeight: '600',
    fontSize: 14,
    flex: 1,
  },
  timestamp: {
    color: '#64748b',
    fontSize: 11,
  },
  messageText: {
    color: '#e2e8f0',
    fontSize: 15,
    lineHeight: 22,
  },
  mention: {
    color: '#60a5fa',
    fontWeight: '600',
  },
  emptyContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    paddingVertical: 60,
  },
  emptyText: {
    color: '#64748b',
    fontSize: 16,
    marginBottom: 8,
  },
  emptyHint: {
    color: '#475569',
    fontSize: 14,
  },
});

export default MessageList;

