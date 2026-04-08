package processutil

import "os/exec"

// Command returns a new exec.Cmd.
func Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
