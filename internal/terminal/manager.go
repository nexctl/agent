package terminal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nexctl/agent/internal/security"
	"github.com/nexctl/agent/internal/transport/wsclient"
	"go.uber.org/zap"
)

const (
	msgTerminalOpen   = "terminal_open"
	msgTerminalInput  = "terminal_input"
	msgTerminalResize = "terminal_resize"
	msgTerminalClose  = "terminal_close"
	msgTerminalOutput = "terminal_output"
	msgTerminalExit   = "terminal_exit"
)

// Sender 向控制面上报终端输出（通常为 wsclient.Client）。
type Sender interface {
	Send(msg wsclient.Message) error
}

type sessionEntry struct {
	gen    uint64
	sess   Session
	cancel context.CancelFunc
}

var sessionGen atomic.Uint64

type openPayload struct {
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

type inputPayload struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

type resizePayload struct {
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

type closePayload struct {
	SessionID string `json:"session_id"`
}

// Manager 处理控制面下发的终端消息并驱动本机 PTY。
type Manager struct {
	mu       sync.Mutex
	sender   Sender
	logger   *zap.Logger
	shell    string
	sessions map[string]*sessionEntry
}

// NewManager 创建终端管理器；shell 为空则各平台使用默认 shell（见 openPTY）。
func NewManager(sender Sender, logger *zap.Logger, shell string) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		sender:   sender,
		logger:   logger,
		shell:    shell,
		sessions: make(map[string]*sessionEntry),
	}
}

// HandleServerMessage 处理服务端经 Agent WebSocket 下发的终端类消息。
func (m *Manager) HandleServerMessage(msg wsclient.Message) error {
	switch msg.Type {
	case msgTerminalOpen:
		return m.handleOpen(msg)
	case msgTerminalInput:
		return m.handleInput(msg)
	case msgTerminalResize:
		return m.handleResize(msg)
	case msgTerminalClose:
		return m.handleClose(msg)
	default:
		return nil
	}
}

func (m *Manager) handleOpen(msg wsclient.Message) error {
	var p openPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return err
	}
	if p.SessionID == "" {
		return nil
	}

	m.mu.Lock()
	if old, ok := m.sessions[p.SessionID]; ok {
		old.cancel()
		_ = old.sess.Close()
		delete(m.sessions, p.SessionID)
	}
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	sess, err := OpenPTY(ctx, m.shell, p.Cols, p.Rows)
	if err != nil {
		cancel()
		m.logger.Warn("terminal_open 失败", zap.Error(err))
		return nil
	}

	gen := sessionGen.Add(1)
	m.mu.Lock()
	m.sessions[p.SessionID] = &sessionEntry{gen: gen, sess: sess, cancel: cancel}
	m.mu.Unlock()

	go m.readLoop(p.SessionID, gen, sess)
	return nil
}

func (m *Manager) handleInput(msg wsclient.Message) error {
	var p inputPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return err
	}
	if p.SessionID == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(p.Data)
	if err != nil {
		return err
	}
	m.mu.Lock()
	e, ok := m.sessions[p.SessionID]
	m.mu.Unlock()
	if !ok || e == nil {
		return nil
	}
	_, err = e.sess.Write(raw)
	return err
}

func (m *Manager) handleResize(msg wsclient.Message) error {
	var p resizePayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return err
	}
	if p.SessionID == "" || p.Cols <= 0 || p.Rows <= 0 {
		return nil
	}
	m.mu.Lock()
	e, ok := m.sessions[p.SessionID]
	m.mu.Unlock()
	if !ok || e == nil {
		return nil
	}
	return e.sess.Resize(p.Cols, p.Rows)
}

func (m *Manager) handleClose(msg wsclient.Message) error {
	var p closePayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return err
	}
	if p.SessionID == "" {
		return nil
	}
	m.mu.Lock()
	e, ok := m.sessions[p.SessionID]
	if ok {
		delete(m.sessions, p.SessionID)
	}
	m.mu.Unlock()
	if !ok || e == nil {
		return nil
	}
	e.cancel()
	return nil
}

func (m *Manager) readLoop(sessionID string, myGen uint64, sess Session) {
	defer func() {
		m.mu.Lock()
		if cur, ok := m.sessions[sessionID]; ok && cur.gen == myGen {
			delete(m.sessions, sessionID)
		}
		m.mu.Unlock()
		_ = sess.Close()
	}()

	buf := make([]byte, 32*1024)
	for {
		n, err := sess.Read(buf)
		if n > 0 {
			m.sendOutput(sessionID, buf[:n])
		}
		if err != nil {
			code := 0
			if err != io.EOF && !errors.Is(err, io.ErrClosedPipe) {
				code = 1
			}
			m.sendExit(sessionID, code)
			return
		}
	}
}

func (m *Manager) sendOutput(sessionID string, data []byte) {
	payload, err := json.Marshal(struct {
		SessionID string `json:"session_id"`
		Data      string `json:"data"`
	}{SessionID: sessionID, Data: base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		return
	}
	_ = m.sender.Send(wsclient.Message{
		Type:      msgTerminalOutput,
		RequestID: security.RequestID(),
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	})
}

func (m *Manager) sendExit(sessionID string, code int) {
	payload, err := json.Marshal(struct {
		SessionID string `json:"session_id"`
		Code      int    `json:"code"`
	}{SessionID: sessionID, Code: code})
	if err != nil {
		return
	}
	_ = m.sender.Send(wsclient.Message{
		Type:      msgTerminalExit,
		RequestID: security.RequestID(),
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	})
}
