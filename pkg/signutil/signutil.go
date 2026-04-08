package signutil

import (
	"crypto/sha256"
	"encoding/hex"
)

// SHA256Hex returns the SHA256 hex digest of the input.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
