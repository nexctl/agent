package serviceop

import (
	"context"

	"github.com/nexctl/agent/internal/platform"
)

// Service exposes service management operations via the platform adapter.
type Service struct {
	platform platform.Adapter
}

// New creates a service operation service.
func New(adapter platform.Adapter) *Service {
	return &Service{platform: adapter}
}

// Query returns a single service state.
func (s *Service) Query(ctx context.Context, name string) (*platform.ServiceInfo, error) {
	return s.platform.QueryService(ctx, name)
}
