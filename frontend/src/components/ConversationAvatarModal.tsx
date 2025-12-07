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
    if (!currentConversation) return;
    
    try {
      if (participatingIds.has(avatarId)) {
        await removeAvatarFromConversation(avatarId);
      } else {
        await addAvatarToConversation(avatarId);
      }
    } catch {
      // エラーはコンテキストで処理
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
                  <View style={[styles.checkbox, isParticipating && styles.checkboxActive]}>
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
            {avatars.length === 0 && (
              <Text style={styles.emptyText}>No avatars available. Create some avatars first.</Text>
            )}
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
    backgroundColor: 'transparent',
  },
  checkboxActive: {
    borderColor: '#3b82f6',
    backgroundColor: '#3b82f6',
  },
  checkmark: {
    color: '#fff',
    fontWeight: 'bold',
    fontSize: 14,
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
  emptyText: {
    color: '#64748b',
    fontStyle: 'italic',
    textAlign: 'center',
    paddingVertical: 20,
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
