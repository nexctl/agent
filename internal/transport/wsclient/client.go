package wsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nexctl/agent/internal/config"
	"go.uber.org/zap"
)

const (
	// MessageTypeHeartbeat is the heartbeat websocket message type.
	MessageTypeHeartbeat = "heartbeat"
	// MessageTypeRuntimeState is the runtime-state websocket message type.
	MessageTypeRuntimeState = "runtime_state"
	// MessageTypeAck is the generic acknowledgement message type.
	MessageTypeAck = "ack"
	// MessageTypeError is the generic error message type.
	MessageTypeError = "error"
	MessageTypeTaskDispatch = "task_dispatch"
	MessageTypeTaskReport   = "task_report"
	// MessageTypeFileDispatch is reserved for future file operations.
	MessageTypeFileDispatch = "file_dispatch"
	// MessageTypeUpgradeCommand is reserved for future upgrade commands.
	MessageTypeUpgradeCommand = "upgrade_command"

	MessageTypeTerminalOpen   = "terminal_open"
	MessageTypeTerminalInput  = "terminal_input"
	MessageTypeTerminalResize = "terminal_resize"
	MessageTypeTerminalClose  = "terminal_close"
	MessageTypeTerminalOutput = "terminal_output"
	MessageTypeTerminalExit   = "terminal_exit"
)

// Message is the base websocket message envelope.
type Message struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// AckPayload is the generic websocket acknowledgement payload.
type AckPayload struct {
	MessageType string `json:"message_type"`
	Status      string `json:"status"`
}

// ErrorPayload is the generic websocket error payload.
type ErrorPayload struct {
	MessageType string `json:"message_type"`
	Message     string `json:"message"`
}

// TerminalHandler 处理控制面下发的终端类 WebSocket 消息（与浏览器经控制面转发的会话对应）。
type TerminalHandler interface {
	HandleServerMessage(msg Message) error
}

// UpgradeCheckHandler 控制面下发 upgrade_command 时触发一次 GitHub 自更新检查。
type UpgradeCheckHandler func()

// TaskDispatchPayload 控制面下发的任务。
type TaskDispatchPayload struct {
	TaskID   int64  `json:"task_id"`
	TaskType string `json:"task_type"`
	Detail   string `json:"detail,omitempty"`
}

// TaskDispatchHandler 处理 task_dispatch。
type TaskDispatchHandler func(TaskDispatchPayload)

// Client manages the websocket connection and reconnection loop.
type Client struct {
	cfg          config.AgentConfig
	logger       *zap.Logger
	sendCh       chan Message
	terminal     TerminalHandler
	upgradeCheck UpgradeCheckHandler
	taskDispatch TaskDispatchHandler
}

// New creates a websocket client.
func New(cfg config.AgentConfig, logger *zap.Logger) *Client {
	return &Client{
		cfg:    cfg,
		logger: logger,
		sendCh: make(chan Message, 128),
	}
}

// SetTerminalHandler 注册终端消息处理器（须在 Run 前调用）。
func (c *Client) SetTerminalHandler(h TerminalHandler) {
	c.terminal = h
}

// SetUpgradeCheckHandler 注册升级检查回调（须在 Run 前调用）。
func (c *Client) SetUpgradeCheckHandler(h UpgradeCheckHandler) {
	c.upgradeCheck = h
}

// SetTaskDispatchHandler 注册任务下发回调（须在 Run 前调用）。
func (c *Client) SetTaskDispatchHandler(h TaskDispatchHandler) {
	c.taskDispatch = h
}

// Send enqueues a message for websocket delivery.
func (c *Client) Send(msg Message) error {
	select {
	case c.sendCh <- msg:
		return nil
	default:
		return fmt.Errorf("websocket send queue full")
	}
}

// Run maintains the websocket connection and reconnection loop.
func (c *Client) Run(ctx context.Context, rawURL, agentID, agentSecret string) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		conn, err := c.dial(ctx, rawURL, agentID, agentSecret)
		if err != nil {
			c.logger.Warn("dial websocket failed", zap.Error(err))
			if !sleepWithContext(ctx, reconnectDelay(c.cfg.ReconnectIntervalSeconds)) {
				return ctx.Err()
			}
			continue
		}

		c.logger.Info("websocket connected")
		errCh := make(chan error, 2)
		go c.writeLoop(ctx, conn, errCh)
		go c.readLoop(ctx, conn, errCh)

		select {
		case <-ctx.Done():
			_ = conn.Close()
			return ctx.Err()
		case err := <-errCh:
			c.logger.Warn("websocket disconnected", zap.Error(err))
			_ = conn.Close()
			if !sleepWithContext(ctx, reconnectDelay(c.cfg.ReconnectIntervalSeconds)) {
				return ctx.Err()
			}
		}
	}
}

func (c *Client) dial(ctx context.Context, rawURL, agentID, agentSecret string) (*websocket.Conn, error) {
	wsURL, err := c.resolveWebSocketURL(rawURL)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("parse ws url: %w", err)
	}

	header := http.Header{}
	header.Set("User-Agent", "NexCtl-Agent/"+strings.TrimSpace(c.cfg.AgentVersion))
	header.Set("X-NexCtl-Agent-Id", agentID)
	header.Set("X-NexCtl-Agent-Secret", agentSecret)
	if o := websocketOrigin(u); o != "" {
		header.Set("Origin", o)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}

	conn, resp, err := dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		if resp != nil {
			snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			_ = resp.Body.Close()
			c.logger.Warn("websocket handshake failed",
				zap.String("url", u.Redacted()),
				zap.Int("status", resp.StatusCode),
				zap.String("response_snippet", strings.TrimSpace(string(snippet))),
				zap.Error(err),
			)
			return nil, fmt.Errorf("dial websocket: %s (HTTP %d): %w", resp.Status, resp.StatusCode, err)
		}
		return nil, fmt.Errorf("dial websocket: %w", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	return conn, nil
}

// websocketOrigin 生成与 RFC 6454 一致的 Origin（http/https），部分服务端会校验 Origin，缺少时易返回非 101。
func websocketOrigin(u *url.URL) string {
	if u == nil {
		return ""
	}
	ou := *u
	switch strings.ToLower(ou.Scheme) {
	case "ws":
		ou.Scheme = "http"
	case "wss":
		ou.Scheme = "https"
	default:
		return ""
	}
	ou.Path = ""
	ou.RawQuery = ""
	ou.Fragment = ""
	return ou.String()
}

// resolveWebSocketURL 将服务端可能返回的 http(s) URL 转为 ws(s)，或在 ws_url 为空时根据 server_url 推导。
func (c *Client) resolveWebSocketURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return c.deriveWSFromServerURL(c.cfg.ServerURL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse websocket url: %w", err)
	}
	if u.Scheme == "" && strings.HasPrefix(raw, "/") {
		base, err := url.Parse(strings.TrimRight(c.cfg.ServerURL, "/"))
		if err != nil {
			return "", fmt.Errorf("parse server_url for relative ws_url: %w", err)
		}
		u = base.ResolveReference(u)
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// ok
	case "":
		return "", fmt.Errorf("websocket url missing scheme: %q", raw)
	default:
		return "", fmt.Errorf("unsupported websocket url scheme %q (expected ws, wss, http, or https)", u.Scheme)
	}
	return u.String(), nil
}

func (c *Client) deriveWSFromServerURL(serverURL string) (string, error) {
	serverURL = strings.TrimSpace(strings.TrimRight(serverURL, "/"))
	if serverURL == "" {
		return "", fmt.Errorf("server_url is empty, cannot derive websocket url")
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", fmt.Errorf("parse server_url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("server_url scheme must be http or https, got %q", u.Scheme)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = c.cfg.WebSocketPath
	}
	return u.String(), nil
}

func (c *Client) writeLoop(ctx context.Context, conn *websocket.Conn, errCh chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.sendCh:
			if err := conn.WriteJSON(msg); err != nil {
				errCh <- err
				return
			}
		}
	}
}

func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn, errCh chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var msg Message
			if err := conn.ReadJSON(&msg); err != nil {
				errCh <- err
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			switch msg.Type {
			case MessageTypeAck:
				c.logger.Debug("websocket ack", zap.String("request_id", msg.RequestID))
			case MessageTypeError:
				var ep ErrorPayload
				_ = json.Unmarshal(msg.Payload, &ep)
				c.logger.Warn("websocket error from server",
					zap.String("request_id", msg.RequestID),
					zap.String("for_type", ep.MessageType),
					zap.String("message", ep.Message),
				)
			case MessageTypeTerminalOpen, MessageTypeTerminalInput, MessageTypeTerminalResize, MessageTypeTerminalClose:
				if c.terminal != nil {
					if err := c.terminal.HandleServerMessage(msg); err != nil {
						c.logger.Debug("terminal handler", zap.Error(err))
					}
				}
			case MessageTypeUpgradeCommand:
				if c.upgradeCheck != nil {
					go c.upgradeCheck()
				}
			default:
				c.logger.Debug("websocket message", zap.String("type", msg.Type), zap.String("request_id", msg.RequestID))
			}
		}
	}
}

func reconnectDelay(seconds int) time.Duration {
	if seconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
