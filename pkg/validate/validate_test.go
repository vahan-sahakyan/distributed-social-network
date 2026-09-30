package validate

import (
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRequired(t *testing.T) {
	if err := Required("a", "x", "b", "y"); err != nil {
		t.Fatal(err)
	}
	err := Required("a", "x", "b", "  ")
	if status.Code(err) != codes.InvalidArgument || !strings.Contains(err.Error(), "b is required") {
		t.Errorf("got %v", err)
	}
}

func TestMaxLen(t *testing.T) {
	if err := MaxLen("text", "héllo", 5); err != nil {
		t.Errorf("5 runes should fit 5: %v", err)
	}
	if status.Code(MaxLen("text", "hello!", 5)) != codes.InvalidArgument {
		t.Error("6 runes should not fit 5")
	}
}

func TestUsername(t *testing.T) {
	for _, ok := range []string{"alice", "load_1_2_3", strings.Repeat("a", 32)} {
		if err := Username(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a b", "a-b", "é", strings.Repeat("a", 33)} {
		if status.Code(Username(bad)) != codes.InvalidArgument {
			t.Errorf("%q should be rejected", bad)
		}
	}
}
