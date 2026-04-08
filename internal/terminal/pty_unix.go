//go:build !windows

package terminal

import (
	"context"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type ptySession struct {
	f   *os.File
	cmd *exec.Cmd
}

func openPTY(ctx context.Context, shell string, cols, rows int) (Session, error) {
	if shell == "" {
		shell = "/bin/sh"
	}
	ws := &pty.Winsize{Rows: 40, Cols: 120}
	if rows > 0 && cols > 0 {
		ws = &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
	}

	cmd := exec.CommandContext(ctx, shell)
	f, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, err
	}
	return &ptySession{f: f, cmd: cmd}, nil
}

func (s *ptySession) Read(p []byte) (int, error) {
	return s.f.Read(p)
}

func (s *ptySession) Write(p []byte) (int, error) {
	return s.f.Write(p)
}

func (s *ptySession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return pty.Setsize(s.f, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (s *ptySession) Close() error {
	var err error
	if s.f != nil {
		err = s.f.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return err
}
