package upgrader

import (
	"context"
	"os"
)

// Layout defines upgrade and rollback directory layout.
type Layout struct {
	ReleaseDir  string
	RollbackDir string
	CurrentDir  string
}

// Service owns upgrade and rollback lifecycle. Phase 1 only prepares directories.
type Service struct {
	layout Layout
}

// New creates an upgrader service.
func New(layout Layout) *Service {
	return &Service{layout: layout}
}

// Prepare ensures upgrade directories exist.
func (s *Service) Prepare(context.Context) error {
	for _, dir := range []string{s.layout.ReleaseDir, s.layout.RollbackDir, s.layout.CurrentDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// HandleUpgrade is a placeholder for future upgrade orchestration.
func (s *Service) HandleUpgrade(context.Context) error {
	return nil
}
