package security

import (
	"crypto/rand"
	"encoding/hex"
)

// RequestID returns a random request ID suitable for transport messages.
func RequestID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}
