package fileutil

import (
	"fmt"
	"os"
)

// EnsureDir creates a directory if it does not already exist.
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("ensure dir %s: %w", path, err)
	}
	return nil
}
