import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import App from '../App';

// Mock react-native components
jest.mock('react-native', () => ({
  StyleSheet: {
    create: (styles: Record<string, unknown>) => styles,
  },
  View: ({ children, style, testID }: { children?: React.ReactNode; style?: unknown; testID?: string }) => (
    <div style={style as React.CSSProperties} data-testid={testID}>{children}</div>
  ),
  Text: ({ children, style }: { children?: React.ReactNode; style?: unknown }) => (
    <span style={style as React.CSSProperties}>{children}</span>
  ),
  TextInput: (props: { testID?: string; placeholder?: string; value?: string; onChangeText?: (text: string) => void }) => (
    <input 
      data-testid={props.testID} 
      placeholder={props.placeholder}
      value={props.value}
      onChange={(e) => props.onChangeText?.(e.target.value)}
    />
  ),
  TouchableOpacity: ({ children, onPress, testID }: { children?: React.ReactNode; onPress?: () => void; testID?: string }) => (
    <button onClick={onPress} data-testid={testID}>{children}</button>
  ),
  ScrollView: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  FlatList: ({ data, renderItem, keyExtractor }: { 
    data: unknown[]; 
    renderItem: (info: { item: unknown; index: number }) => React.ReactNode; 
    keyExtractor?: (item: unknown, index: number) => string 
  }) => (
    <div>
      {data?.map((item, index) => (
        <div key={keyExtractor ? keyExtractor(item, index) : index}>
          {renderItem({ item, index })}
        </div>
      ))}
    </div>
  ),
  ActivityIndicator: () => <div data-testid="loading-indicator">Loading...</div>,
  Pressable: ({ children, onPress }: { children?: React.ReactNode; onPress?: () => void }) => (
    <button onClick={onPress}>{children}</button>
  ),
}));

// Mock API service
jest.mock('../services/api', () => ({
  avatarApi: {
    list: jest.fn().mockResolvedValue({ avatars: [] }),
    create: jest.fn(),
    delete: jest.fn(),
  },
  conversationApi: {
    list: jest.fn().mockResolvedValue({ conversations: [] }),
    create: jest.fn(),
    getMessages: jest.fn().mockResolvedValue({ messages: [] }),
    sendMessage: jest.fn(),
    delete: jest.fn(),
  },
}));

describe('App', () => {
  it('renders the application title', () => {
    render(<App />);
    expect(screen.getByText('Multi-Avatar Chat')).toBeInTheDocument();
  });

  it('renders the main layout structure', () => {
    render(<App />);
    // Check that the app renders without crashing
    expect(screen.getByText('Multi-Avatar Chat')).toBeInTheDocument();
  });
});

