package id

import (
	"crypto/rand"
	"encoding/hex"
)

// New generates a random 16-byte hex ID.
func New() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
