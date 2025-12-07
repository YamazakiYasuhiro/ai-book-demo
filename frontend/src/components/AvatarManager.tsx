import React, { useState } from 'react';
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
import { Avatar } from '../services/api';

const AvatarManager: React.FC = () => {
  const { avatars, createAvatar, updateAvatar, deleteAvatar, loading } = useApp();
  const [showModal, setShowModal] = useState(false);
  const [editingAvatar, setEditingAvatar] = useState<Avatar | null>(null);
  const [name, setName] = useState('');
  const [prompt, setPrompt] = useState('');

  const handleOpenCreate = () => {
    setEditingAvatar(null);
    setName('');
    setPrompt('');
    setShowModal(true);
  };

  const handleOpenEdit = (avatar: Avatar) => {
    setEditingAvatar(avatar);
    setName(avatar.name);
    setPrompt(avatar.prompt);
    setShowModal(true);
  };

  const handleSave = async () => {
    if (!name.trim() || !prompt.trim()) return;

    try {
      if (editingAvatar) {
        await updateAvatar(editingAvatar.id, name.trim(), prompt.trim());
      } else {
        await createAvatar(name.trim(), prompt.trim());
      }
      setShowModal(false);
      setName('');
      setPrompt('');
      setEditingAvatar(null);
    } catch {
      // Error is handled by context
    }
  };

  const handleDelete = async (avatar: Avatar) => {
    try {
      await deleteAvatar(avatar.id);
    } catch {
      // Error is handled by context
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.headerTitle}>Avatars</Text>
        <TouchableOpacity
          style={styles.addButton}
          onPress={handleOpenCreate}
          disabled={loading}
        >
          <Text style={styles.addButtonText}>+ New</Text>
        </TouchableOpacity>
      </View>

      <ScrollView style={styles.list}>
        {avatars.map((avatar) => (
          <View key={avatar.id} style={styles.avatarItem}>
            <View style={styles.avatarInfo}>
              <Text style={styles.avatarName}>{avatar.name}</Text>
              <Text style={styles.avatarPrompt} numberOfLines={2}>
                {avatar.prompt}
              </Text>
            </View>
            <View style={styles.avatarActions}>
              <TouchableOpacity
                style={styles.editButton}
                onPress={() => handleOpenEdit(avatar)}
              >
                <Text style={styles.editButtonText}>Edit</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={styles.deleteButton}
                onPress={() => handleDelete(avatar)}
              >
                <Text style={styles.deleteButtonText}>×</Text>
              </TouchableOpacity>
            </View>
          </View>
        ))}
        {avatars.length === 0 && (
          <Text style={styles.emptyText}>No avatars yet. Create one!</Text>
        )}
      </ScrollView>

      <Modal
        visible={showModal}
        transparent
        animationType="fade"
        onRequestClose={() => setShowModal(false)}
      >
        <View style={styles.modalOverlay}>
          <View style={styles.modalContent}>
            <Text style={styles.modalTitle}>
              {editingAvatar ? 'Edit Avatar' : 'Create Avatar'}
            </Text>
            
            <Text style={styles.label}>Name</Text>
            <TextInput
              style={styles.input}
              value={name}
              onChangeText={setName}
              placeholder="Avatar name..."
              placeholderTextColor="#64748b"
            />
            
            <Text style={styles.label}>Prompt</Text>
            <TextInput
              style={[styles.input, styles.textArea]}
              value={prompt}
              onChangeText={setPrompt}
              placeholder="Describe the avatar's personality..."
              placeholderTextColor="#64748b"
              multiline
              numberOfLines={4}
            />
            
            <View style={styles.modalActions}>
              <TouchableOpacity
                style={styles.cancelButton}
                onPress={() => setShowModal(false)}
              >
                <Text style={styles.cancelButtonText}>Cancel</Text>
              </TouchableOpacity>
              <TouchableOpacity
                style={[styles.saveButton, loading && styles.disabledButton]}
                onPress={handleSave}
                disabled={loading}
              >
                <Text style={styles.saveButtonText}>
                  {loading ? 'Saving...' : 'Save'}
                </Text>
              </TouchableOpacity>
            </View>
          </View>
        </View>
      </Modal>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 16,
    borderBottomWidth: 1,
    borderBottomColor: '#334155',
  },
  headerTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#f8fafc',
  },
  addButton: {
    backgroundColor: '#3b82f6',
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 6,
  },
  addButtonText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 14,
  },
  list: {
    flex: 1,
    padding: 16,
  },
  avatarItem: {
    backgroundColor: '#334155',
    borderRadius: 8,
    padding: 12,
    marginBottom: 12,
  },
  avatarInfo: {
    marginBottom: 8,
  },
  avatarName: {
    fontSize: 16,
    fontWeight: '600',
    color: '#f8fafc',
    marginBottom: 4,
  },
  avatarPrompt: {
    fontSize: 13,
    color: '#94a3b8',
  },
  avatarActions: {
    flexDirection: 'row',
    justifyContent: 'flex-end',
    gap: 8,
  },
  editButton: {
    backgroundColor: '#475569',
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 4,
  },
  editButtonText: {
    color: '#f8fafc',
    fontSize: 13,
  },
  deleteButton: {
    backgroundColor: '#dc2626',
    paddingHorizontal: 10,
    paddingVertical: 6,
    borderRadius: 4,
  },
  deleteButtonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: 'bold',
  },
  emptyText: {
    textAlign: 'center',
    color: '#64748b',
    marginTop: 24,
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
    maxWidth: 400,
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
    marginBottom: 6,
  },
  input: {
    backgroundColor: '#334155',
    borderRadius: 8,
    padding: 12,
    color: '#f8fafc',
    fontSize: 15,
    marginBottom: 16,
  },
  textArea: {
    height: 100,
    textAlignVertical: 'top',
  },
  modalActions: {
    flexDirection: 'row',
    justifyContent: 'flex-end',
    gap: 12,
    marginTop: 8,
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
  saveButton: {
    backgroundColor: '#3b82f6',
    paddingHorizontal: 20,
    paddingVertical: 10,
    borderRadius: 6,
  },
  saveButtonText: {
    color: '#fff',
    fontWeight: '600',
  },
  disabledButton: {
    opacity: 0.6,
  },
});

export default AvatarManager;

