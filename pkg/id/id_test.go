package id

import (
	"encoding/hex"
	"testing"
)

func TestDeterministic(t *testing.T) {
	a := Deterministic("like.created", "0", "42")
	if a != Deterministic("like.created", "0", "42") {
		t.Fatal("same parts must give the same id")
	}
	if a == Deterministic("like.created", "0", "43") {
		t.Fatal("different offsets must give different ids")
	}
	// Joining with a separator keeps ("ab","c") and ("a","bc") apart.
	if Deterministic("ab", "c") == Deterministic("a", "bc") {
		t.Fatal("part boundaries must affect the id")
	}
	if len(a) != len(New()) {
		t.Fatalf("len %d, want the %d chars New produces", len(a), len(New()))
	}
	if _, err := hex.DecodeString(a); err != nil {
		t.Fatalf("not hex: %v", err)
	}
}
