package linux

import (
	"bytes"
	"context"
	"os"
	"os/exec"

	platformspec "github.com/nexctl/agent/internal/platform/spec"
	"github.com/nexctl/agent/internal/terminal"
)

// Adapter implements Linux-specific operations.
type Adapter struct{}

// New creates a Linux adapter.
func New() *Adapter {
	return &Adapter{}
}

// QueryService queries a single systemd service.
func (a *Adapter) QueryService(ctx context.Context, name string) (*platformspec.ServiceInfo, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", name)
	output, err := cmd.Output()
	status := "unknown"
	if err == nil {
		status = string(bytes.TrimSpace(output))
	}
	return &platformspec.ServiceInfo{Name: name, Status: status}, nil
}

// ListServices lists services. This is a minimal placeholder in phase 1.
func (a *Adapter) ListServices(_ context.Context) ([]platformspec.ServiceInfo, error) {
	return []platformspec.ServiceInfo{}, nil
}

// StartService starts a system service.
func (a *Adapter) StartService(ctx context.Context, name string) error {
	return exec.CommandContext(ctx, "systemctl", "start", name).Run()
}

// StopService stops a system service.
func (a *Adapter) StopService(ctx context.Context, name string) error {
	return exec.CommandContext(ctx, "systemctl", "stop", name).Run()
}

// RestartService restarts a system service.
func (a *Adapter) RestartService(ctx context.Context, name string) error {
	return exec.CommandContext(ctx, "systemctl", "restart", name).Run()
}

// EnableService enables a system service.
func (a *Adapter) EnableService(ctx context.Context, name string) error {
	return exec.CommandContext(ctx, "systemctl", "enable", name).Run()
}

// DisableService disables a system service.
func (a *Adapter) DisableService(ctx context.Context, name string) error {
	return exec.CommandContext(ctx, "systemctl", "disable", name).Run()
}

// OpenTerminal opens a PTY-backed terminal session.
func (a *Adapter) OpenTerminal(ctx context.Context, shell string) (terminal.Session, error) {
	return terminal.OpenPTY(ctx, shell)
}

// WriteFile writes a file to disk.
func (a *Adapter) WriteFile(_ context.Context, path string, content []byte) error {
	return os.WriteFile(path, content, 0o644)
}

// ReadFile reads a file from disk.
func (a *Adapter) ReadFile(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}
