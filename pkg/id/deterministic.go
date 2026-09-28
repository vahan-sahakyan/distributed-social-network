package id

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Deterministic returns the same ID for the same parts, in the format New uses.
// Deriving an event ID from its Kafka coordinates instead of generating a random
// one makes a redelivered message produce a row the readers can deduplicate.
func Deterministic(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:16])
}
