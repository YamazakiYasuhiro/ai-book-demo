package utils

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SSEEvent はServer-Sent Eventを表す
type SSEEvent struct {
	Type string
	Data string
}

// SSEClient はSSE接続を管理するクライアント
type SSEClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSSEClient は新しいSSEクライアントを作成する
func NewSSEClient(baseURL string) *SSEClient {
	return &SSEClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 0, // SSEはlong-liveなので、タイムアウトなし
		},
	}
}

// SSEConnection はアクティブなSSE接続を表す
type SSEConnection struct {
	resp     *http.Response
	scanner  *bufio.Scanner
	eventCh  chan SSEEvent
	errCh    chan error
	ctx      context.Context
	cancel   context.CancelFunc
}

// Connect はSSEエンドポイントに接続する
func (c *SSEClient) Connect(ctx context.Context, path string) (*SSEConnection, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	connCtx, cancel := context.WithCancel(ctx)
	conn := &SSEConnection{
		resp:    resp,
		scanner: bufio.NewScanner(resp.Body),
		eventCh: make(chan SSEEvent, 10),
		errCh:   make(chan error, 1),
		ctx:     connCtx,
		cancel:  cancel,
	}

	// バックグラウンドでイベントを読み取る
	go conn.readEvents()

	return conn, nil
}

// readEvents はSSEストリームからイベントを読み取る
func (conn *SSEConnection) readEvents() {
	defer close(conn.eventCh)
	defer close(conn.errCh)

	var eventType string
	var eventData strings.Builder

	for conn.scanner.Scan() {
		select {
		case <-conn.ctx.Done():
			return
		default:
		}

		line := conn.scanner.Text()

		if line == "" {
			// 空行はイベントの終了を示す
			if eventData.Len() > 0 {
				event := SSEEvent{
					Type: eventType,
					Data: strings.TrimSpace(eventData.String()),
				}
				select {
				case conn.eventCh <- event:
				case <-conn.ctx.Done():
					return
				}
			}
			eventType = ""
			eventData.Reset()
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			eventData.WriteString(strings.TrimPrefix(line, "data:"))
		}
	}

	if err := conn.scanner.Err(); err != nil {
		select {
		case conn.errCh <- err:
		case <-conn.ctx.Done():
		}
	}
}

// Events はイベントチャネルを返す
func (conn *SSEConnection) Events() <-chan SSEEvent {
	return conn.eventCh
}

// Errors はエラーチャネルを返す
func (conn *SSEConnection) Errors() <-chan error {
	return conn.errCh
}

// Close は接続を閉じる
func (conn *SSEConnection) Close() {
	conn.cancel()
	conn.resp.Body.Close()
}

// WaitForEvent は指定されたタイプのイベントを待つ
func (conn *SSEConnection) WaitForEvent(eventType string, timeout time.Duration) (*SSEEvent, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case event, ok := <-conn.eventCh:
			if !ok {
				return nil, fmt.Errorf("connection closed")
			}
			if event.Type == eventType {
				return &event, nil
			}
		case err := <-conn.errCh:
			return nil, err
		case <-timer.C:
			return nil, fmt.Errorf("timeout waiting for event type: %s", eventType)
		case <-conn.ctx.Done():
			return nil, conn.ctx.Err()
		}
	}
}

// WaitForMessageEvent はメッセージイベントを待ち、JSONをパースする
func (conn *SSEConnection) WaitForMessageEvent(timeout time.Duration) (map[string]any, error) {
	event, err := conn.WaitForEvent("message", timeout)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(event.Data), &data); err != nil {
		return nil, fmt.Errorf("failed to parse message data: %w", err)
	}

	return data, nil
}

