//go:build windows

package terminal

import "context"

func openPTY(context.Context, string) (Session, error) {
	return NewUnsupportedSession(), nil
}
