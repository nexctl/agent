package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/nexctl/agent/internal/config"
	"github.com/nexctl/agent/internal/upgrader"
	"go.uber.org/zap"
)

// Supervisor monitors and restarts agentd, and prepares upgrade directories.
type Supervisor struct {
	cfg      config.SupervisorConfig
	logger   *zap.Logger
	upgrader *upgrader.Service
}

// NewSupervisor creates a supervisor application.
func NewSupervisor(cfg config.SupervisorConfig) (*Supervisor, error) {
	logger, err := NewLogger(cfg.LogDir, "supervisor.log")
	if err != nil {
		return nil, err
	}

	upgraderSvc := upgrader.New(upgrader.Layout{
		ReleaseDir:  cfg.ReleaseDir,
		RollbackDir: cfg.RollbackDir,
		CurrentDir:  cfg.CurrentDir,
	})

	return &Supervisor{
		cfg:      cfg,
		logger:   logger,
		upgrader: upgraderSvc,
	}, nil
}

// Run starts supervisor process management.
func (s *Supervisor) Run(ctx context.Context) error {
	if err := s.upgrader.Prepare(ctx); err != nil {
		return fmt.Errorf("prepare upgrade layout: %w", err)
	}

	restartCount := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.cfg.MaxRestartBurst > 0 && restartCount >= s.cfg.MaxRestartBurst {
			return fmt.Errorf("agentd restart burst exceeded")
		}

		cmd := exec.CommandContext(ctx, s.cfg.AgentdBin, "-config", s.cfg.AgentdConfig)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		s.logger.Info("starting agentd", zap.String("bin", s.cfg.AgentdBin))
		err := cmd.Run()
		if ctx.Err() != nil {
			return ctx.Err()
		}

		restartCount++
		s.logger.Warn("agentd exited, scheduling restart", zap.Error(err), zap.Int("restart_count", restartCount))

		timer := time.NewTimer(restartDelay(s.cfg.RestartDelaySeconds))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func restartDelay(seconds int) time.Duration {
	if seconds <= 0 {
		return 3 * time.Second
	}
	return time.Duration(seconds) * time.Second
}
