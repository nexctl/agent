package darwin

import (
	"context"
	"errors"
	"os"

	platformspec "github.com/nexctl/agent/internal/platform/spec"
	"github.com/nexctl/agent/internal/terminal"
)

// Adapter implements macOS-specific operations.
type Adapter struct{}

// New creates a Darwin adapter.
func New() *Adapter {
	return &Adapter{}
}

// QueryService returns a minimal placeholder service state.
func (a *Adapter) QueryService(_ context.Context, name string) (*platformspec.ServiceInfo, error) {
	return &platformspec.ServiceInfo{Name: name, Status: "unknown"}, nil
}

// ListServices returns an empty list in phase 1.
func (a *Adapter) ListServices(_ context.Context) ([]platformspec.ServiceInfo, error) {
	return []platformspec.ServiceInfo{}, nil
}

// StartService is reserved for future implementation.
func (a *Adapter) StartService(_ context.Context, _ string) error {
	return errors.New("darwin service start not implemented yet")
}

// StopService is reserved for future implementation.
func (a *Adapter) StopService(_ context.Context, _ string) error {
	return errors.New("darwin service stop not implemented yet")
}

// RestartService is reserved for future implementation.
func (a *Adapter) RestartService(_ context.Context, _ string) error {
	return errors.New("darwin service restart not implemented yet")
}

// EnableService is reserved for future implementation.
func (a *Adapter) EnableService(_ context.Context, _ string) error {
	return errors.New("darwin service enable not implemented yet")
}

// DisableService is reserved for future implementation.
func (a *Adapter) DisableService(_ context.Context, _ string) error {
	return errors.New("darwin service disable not implemented yet")
}

// OpenTerminal opens a PTY-backed terminal session.
func (a *Adapter) OpenTerminal(ctx context.Context, shell string) (terminal.Session, error) {
	return terminal.OpenPTY(ctx, shell, 0, 0)
}

// WriteFile writes a file to disk.
func (a *Adapter) WriteFile(_ context.Context, path string, content []byte) error {
	return os.WriteFile(path, content, 0o644)
}

// ReadFile reads a file from disk.
func (a *Adapter) ReadFile(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}
