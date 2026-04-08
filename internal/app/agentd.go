package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexctl/agent/internal/collector"
	"github.com/nexctl/agent/internal/config"
	"github.com/nexctl/agent/internal/security"
	"github.com/nexctl/agent/internal/store"
	"github.com/nexctl/agent/internal/transport/httpclient"
	"github.com/nexctl/agent/internal/transport/wsclient"
	"go.uber.org/zap"
)

// RuntimeCollector defines the collector capability required by agentd.
type RuntimeCollector interface {
	CollectIdentity(ctx context.Context) (*collector.Identity, error)
	CollectRuntimeState(ctx context.Context) (*collector.RuntimeState, error)
}

// Agentd coordinates registration, websocket connection, and periodic reporting.
type Agentd struct {
	cfg        config.AgentConfig
	logger     *zap.Logger
	store      store.CredentialStore
	collector  RuntimeCollector
	register   *httpclient.Client
	ws         *wsclient.Client
	credential *store.Credential
}

// NewAgentd creates an agentd application.
func NewAgentd(cfg config.AgentConfig) (*Agentd, error) {
	layout := store.Layout{
		DataDir:       cfg.DataDir,
		ConfigDir:     cfg.ConfigDir,
		CredentialDir: cfg.CredentialDir,
		LogDir:        cfg.LogDir,
	}
	if err := layout.Ensure(); err != nil {
		return nil, err
	}

	logger, err := NewLogger(cfg.LogDir, "agentd.log")
	if err != nil {
		return nil, err
	}

	nodeKey, err := layout.EnsureNodeKey()
	if err != nil {
		return nil, err
	}

	return &Agentd{
		cfg:       cfg,
		logger:    logger,
		store:     store.NewFileCredentialStore(cfg.CredentialDir),
		collector: collector.New(nodeKey, cfg.NodeName, cfg.InstallToken, cfg.EnrollmentToken, cfg.AgentVersion),
		register:  httpclient.New(cfg),
		ws:        wsclient.New(cfg, logger),
	}, nil
}

// Run starts the agentd lifecycle.
func (a *Agentd) Run(ctx context.Context) error {
	credential, err := a.store.Load()
	if err != nil {
		return err
	}
	if credential == nil {
		identity, err := a.collector.CollectIdentity(ctx)
		if err != nil {
			return fmt.Errorf("collect identity: %w", err)
		}
		credential, err = a.register.Register(ctx, *identity)
		if err != nil {
			return fmt.Errorf("register agent: %w", err)
		}
		if err := a.store.Save(credential); err != nil {
			return fmt.Errorf("save credential: %w", err)
		}
		a.logger.Info("agent registered", zap.Int64("node_id", credential.NodeID))
	}
	a.credential = credential

	go func() {
		if err := a.ws.Run(ctx, credential.WSURL, credential.AgentID, credential.AgentSecret); err != nil && ctx.Err() == nil {
			a.logger.Error("ws client exited", zap.Error(err))
		}
	}()

	go a.heartbeatLoop(ctx)
	go a.runtimeStateLoop(ctx)

	<-ctx.Done()
	return ctx.Err()
}

func (a *Agentd) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval(a.cfg.HeartbeatIntervalSeconds))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case tickAt := <-ticker.C:
			payload, _ := json.Marshal(map[string]any{"sent_at": tickAt.UTC()})
			_ = a.ws.Send(wsclient.Message{
				Type:      "heartbeat",
				RequestID: security.RequestID(),
				Timestamp: time.Now().UTC(),
				Payload:   payload,
			})
		}
	}
}

func (a *Agentd) runtimeStateLoop(ctx context.Context) {
	ticker := time.NewTicker(runtimeInterval(a.cfg.RuntimeIntervalSeconds))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state, err := a.collector.CollectRuntimeState(ctx)
			if err != nil {
				a.logger.Warn("collect runtime state failed", zap.Error(err))
				continue
			}
			payload, err := json.Marshal(state)
			if err != nil {
				a.logger.Warn("marshal runtime state failed", zap.Error(err))
				continue
			}
			if err := a.ws.Send(wsclient.Message{
				Type:      "runtime_state",
				RequestID: security.RequestID(),
				Timestamp: time.Now().UTC(),
				Payload:   payload,
			}); err != nil {
				a.logger.Warn("send runtime state failed", zap.Error(err))
			}
		}
	}
}

func heartbeatInterval(seconds int) time.Duration {
	if seconds <= 0 {
		return 15 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func runtimeInterval(seconds int) time.Duration {
	if seconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(seconds) * time.Second
}
