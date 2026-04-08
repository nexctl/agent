//go:build !windows

package terminal

import (
	"context"
	"os/exec"

	"github.com/creack/pty"
)

type ptySession struct {
	f interface {
		Read(p []byte) (int, error)
		Write(p []byte) (int, error)
		Close() error
	}
}

func openPTY(ctx context.Context, shell string) (Session, error) {
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.CommandContext(ctx, shell)
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	return &ptySession{f: f}, nil
}

func (s *ptySession) Read(p []byte) (int, error) {
	return s.f.Read(p)
}

func (s *ptySession) Write(p []byte) (int, error) {
	return s.f.Write(p)
}

func (s *ptySession) Close() error {
	return s.f.Close()
}
