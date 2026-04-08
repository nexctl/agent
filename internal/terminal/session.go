package terminal

import (
	"context"
	"errors"
	"io"
)

// Session is a platform-neutral terminal session.
type Session interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

type unsupportedSession struct{}

// NewUnsupportedSession creates a placeholder terminal session.
func NewUnsupportedSession() Session {
	return &unsupportedSession{}
}

func (s *unsupportedSession) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (s *unsupportedSession) Write([]byte) (int, error) {
	return 0, errors.New("terminal not supported on this platform yet")
}

func (s *unsupportedSession) Close() error {
	return nil
}

// OpenPTY opens a PTY-backed terminal session when supported by the current build target.
func OpenPTY(ctx context.Context, shell string) (Session, error) {
	return openPTY(ctx, shell)
}
