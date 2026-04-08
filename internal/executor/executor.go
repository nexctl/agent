package executor

import (
	"context"
	"os/exec"
)

// Executor executes local commands on behalf of the agent.
type Executor interface {
	RunCommand(ctx context.Context, name string, args ...string) ([]byte, error)
}

// LocalExecutor is the default command executor.
type LocalExecutor struct{}

// New creates a local executor.
func New() *LocalExecutor {
	return &LocalExecutor{}
}

// RunCommand executes a command and returns combined output.
func (e *LocalExecutor) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
