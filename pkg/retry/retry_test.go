package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func init() {
	initialBackoff, maxBackoff = time.Millisecond, 4*time.Millisecond
}

func TestDoRetriesUntilSuccess(t *testing.T) {
	calls := 0
	err := Do(context.Background(), "test", func(context.Context) error {
		calls++
		if calls < 4 {
			return errors.New("not yet")
		}
		return nil
	})
	if err != nil || calls != 4 {
		t.Fatalf("err=%v calls=%d, want nil after 4 calls", err, calls)
	}
}

func TestDoStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := Do(ctx, "test", func(context.Context) error { return errors.New("down") })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want the context error", err)
	}
}
