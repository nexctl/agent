package wsclient

import (
	"context"
	"encoding/json"
	"fmt"
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
	// MessageTypeTaskDispatch is reserved for future task delivery.
	MessageTypeTaskDispatch = "task_dispatch"
	// MessageTypeFileDispatch is reserved for future file operations.
	MessageTypeFileDispatch = "file_dispatch"
	// MessageTypeUpgradeCommand is reserved for future upgrade commands.
	MessageTypeUpgradeCommand = "upgrade_command"
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

// Client manages the websocket connection and reconnection loop.
type Client struct {
	cfg    config.AgentConfig
	logger *zap.Logger
	sendCh chan Message
}

// New creates a websocket client.
func New(cfg config.AgentConfig, logger *zap.Logger) *Client {
	return &Client{
		cfg:    cfg,
		logger: logger,
		sendCh: make(chan Message, 128),
	}
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

		conn, err := c.dial(rawURL, agentID, agentSecret)
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

func (c *Client) dial(rawURL, agentID, agentSecret string) (*websocket.Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse ws url: %w", err)
	}

	header := http.Header{}
	header.Set("User-Agent", "NexCtl-Agent/"+strings.TrimSpace(c.cfg.AgentVersion))
	header.Set("X-NexCtl-Agent-Id", agentID)
	header.Set("X-NexCtl-Agent-Secret", agentSecret)
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	return conn, nil
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
			var ack map[string]any
			if err := conn.ReadJSON(&ack); err != nil {
				errCh <- err
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
			c.logger.Debug("websocket message", zap.Any("message", ack))
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
