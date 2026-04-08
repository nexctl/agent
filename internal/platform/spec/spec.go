package spec

import (
	"context"

	"github.com/nexctl/agent/internal/terminal"
)

// ServiceInfo is the platform-neutral service state model.
type ServiceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Adapter defines platform-dependent operations.
type Adapter interface {
	QueryService(ctx context.Context, name string) (*ServiceInfo, error)
	ListServices(ctx context.Context) ([]ServiceInfo, error)
	StartService(ctx context.Context, name string) error
	StopService(ctx context.Context, name string) error
	RestartService(ctx context.Context, name string) error
	EnableService(ctx context.Context, name string) error
	DisableService(ctx context.Context, name string) error
	OpenTerminal(ctx context.Context, shell string) (terminal.Session, error)
	WriteFile(ctx context.Context, path string, content []byte) error
	ReadFile(ctx context.Context, path string) ([]byte, error)
}
