package platform

import (
	"runtime"

	"github.com/nexctl/agent/internal/platform/darwin"
	"github.com/nexctl/agent/internal/platform/linux"
	"github.com/nexctl/agent/internal/platform/spec"
	"github.com/nexctl/agent/internal/platform/windows"
)

// ServiceInfo re-exports the shared platform-neutral service state model.
type ServiceInfo = spec.ServiceInfo

// Adapter re-exports the platform adapter contract.
type Adapter = spec.Adapter

// New creates the current platform adapter.
func New() Adapter {
	switch runtime.GOOS {
	case "windows":
		return windows.New()
	case "darwin":
		return darwin.New()
	default:
		return linux.New()
	}
}
