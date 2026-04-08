//go:build windows

package terminal

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
)

type pipeSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	waitCh chan struct{}
}

func openPTY(ctx context.Context, shell string, cols, rows int) (Session, error) {
	_ = cols
	_ = rows
	if shell == "" {
		shell = strings.TrimSpace(os.Getenv("COMSPEC"))
		if shell == "" {
			shell = `C:\Windows\System32\cmd.exe`
		}
	}

	cmd := exec.CommandContext(ctx, shell)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	s := &pipeSession{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		waitCh: make(chan struct{}),
	}
	go func() {
		_ = cmd.Wait()
		close(s.waitCh)
	}()
	return s, nil
}

func (s *pipeSession) Read(p []byte) (int, error) {
	return s.stdout.Read(p)
}

func (s *pipeSession) Write(p []byte) (int, error) {
	return s.stdin.Write(p)
}

func (s *pipeSession) Resize(int, int) error {
	return nil
}

func (s *pipeSession) Close() error {
	_ = s.stdin.Close()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	<-s.waitCh
	return nil
}
