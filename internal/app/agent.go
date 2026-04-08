package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/nexctl/agent/internal/collector"
	"github.com/nexctl/agent/internal/config"
	"github.com/nexctl/agent/internal/security"
	"github.com/nexctl/agent/internal/store"
	"github.com/nexctl/agent/internal/transport/httpclient"
	"github.com/nexctl/agent/internal/transport/wsclient"
	"go.uber.org/zap"
)

// ErrRestartAfterUpdate 表示已成功替换二进制，进程应退出以便由外部拉起新版本（与 nezhahq agent 行为一致）。
var ErrRestartAfterUpdate = errors.New("restart after self-update")

// 与 nezhahq agent 默认随机检查间隔（分钟）一致：minUpdateInterval ~ maxUpdateInterval
const (
	minSelfUpdateIntervalMin = 1440
	maxSelfUpdateIntervalMin = 2880
)

// RuntimeCollector defines the collector capability required by the agent.
type RuntimeCollector interface {
	CollectIdentity(ctx context.Context) (*collector.Identity, error)
	CollectRuntimeState(ctx context.Context) (*collector.RuntimeState, error)
}

// Agent coordinates registration, websocket connection, periodic reporting, and optional self-update.
type Agent struct {
	cfg        config.AgentConfig
	logger     *zap.Logger
	store      store.CredentialStore
	collector  RuntimeCollector
	register   *httpclient.Client
	ws         *wsclient.Client
	credential *store.Credential
}

// NewAgent creates the agent runtime.
func NewAgent(cfg config.AgentConfig) (*Agent, error) {
	layout := store.Layout{
		DataDir:       cfg.DataDir,
		ConfigDir:     cfg.ConfigDir,
		CredentialDir: cfg.CredentialDir,
		LogDir:        cfg.LogDir,
	}
	if err := layout.Ensure(); err != nil {
		return nil, err
	}

	logger, err := NewLogger(cfg.LogDir, "agent.log")
	if err != nil {
		return nil, err
	}

	nodeKey, err := layout.EnsureNodeKey()
	if err != nil {
		return nil, err
	}

	return &Agent{
		cfg:       cfg,
		logger:    logger,
		store:     store.NewFileCredentialStore(cfg.CredentialDir),
		collector: collector.New(nodeKey, cfg.NodeName, cfg.InstallToken, cfg.EnrollmentToken, cfg.AgentVersion),
		register:  httpclient.New(cfg),
		ws:        wsclient.New(cfg, logger),
	}, nil
}

// Run starts the agent lifecycle.
func (a *Agent) Run(ctx context.Context) error {
	if !a.cfg.DisableAutoUpdate {
		if _, err := ParseBuildVersion(); err == nil {
			if doSelfUpdate(ctx, a.logger, a.cfg, true) {
				return ErrRestartAfterUpdate
			}
			go a.selfUpdateLoop(ctx)
		} else {
			a.logger.Debug("self-update disabled: version is not semver", zap.String("version", Version))
		}
	}

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

	wsDialURL := resolveWebSocketDialURL(a.logger, a.cfg, credential.WSURL)

	go func() {
		if err := a.ws.Run(ctx, wsDialURL, credential.AgentID, credential.AgentSecret); err != nil && ctx.Err() == nil {
			a.logger.Error("ws client exited", zap.Error(err))
		}
	}()

	go a.heartbeatLoop(ctx)
	go a.runtimeStateLoop(ctx)

	<-ctx.Done()
	return ctx.Err()
}

func (a *Agent) selfUpdateLoop(ctx context.Context) {
	var interval time.Duration
	if a.cfg.SelfUpdatePeriodMinutes > 0 {
		interval = time.Duration(a.cfg.SelfUpdatePeriodMinutes) * time.Minute
	} else {
		interval = time.Duration(rand.Intn(maxSelfUpdateIntervalMin-minSelfUpdateIntervalMin)+minSelfUpdateIntervalMin) * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if doSelfUpdate(ctx, a.logger, a.cfg, true) {
				os.Exit(1)
			}
		}
	}
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
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

func (a *Agent) runtimeStateLoop(ctx context.Context) {
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
