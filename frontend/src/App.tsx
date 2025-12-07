import React from 'react';
import { StyleSheet, View, Text } from 'react-native';
import { AppProvider } from './context/AppContext';
import AvatarManager from './components/AvatarManager';
import ChatArea from './components/ChatArea';

const App: React.FC = () => {
  return (
    <AppProvider>
      <View style={styles.container}>
        <View style={styles.header}>
          <Text style={styles.title}>Multi-Avatar Chat</Text>
        </View>
        <View style={styles.content}>
          <View style={styles.sidebar}>
            <AvatarManager />
          </View>
          <View style={styles.main}>
            <ChatArea />
          </View>
        </View>
      </View>
    </AppProvider>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f172a',
    // @ts-expect-error - Web専用: 画面全体を占めるようにする
    height: '100vh',
    overflow: 'hidden',
  },
  header: {
    backgroundColor: '#1e293b',
    padding: 16,
    borderBottomWidth: 1,
    borderBottomColor: '#334155',
  },
  title: {
    fontSize: 24,
    fontWeight: 'bold',
    color: '#f8fafc',
  },
  content: {
    flex: 1,
    flexDirection: 'row',
    overflow: 'hidden',
    // @ts-expect-error - Web専用: flexboxでスクロールを有効にする
    minHeight: 0,
  },
  sidebar: {
    width: 320,
    backgroundColor: '#1e293b',
    borderRightWidth: 1,
    borderRightColor: '#334155',
    overflow: 'hidden',
    // @ts-expect-error - Web専用: flexboxでスクロールを有効にする
    minHeight: 0,
  },
  main: {
    flex: 1,
    backgroundColor: '#0f172a',
    overflow: 'hidden',
    // @ts-expect-error - Web専用: flexboxでスクロールを有効にする
    minHeight: 0,
  },
});

export default App;

