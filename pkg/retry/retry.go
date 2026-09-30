// Package retry waits out dependencies that are not up yet at startup.
package retry

import (
	"context"
	"log/slog"
	"time"
)

var (
	initialBackoff = 500 * time.Millisecond
	maxBackoff     = 10 * time.Second
)

// Do runs fn until it succeeds or ctx is done, backing off from 500ms to 10s
// between attempts. It returns ctx's error if ctx ends first.
func Do(ctx context.Context, what string, fn func(context.Context) error) error {
	backoff := initialBackoff
	for attempt := 1; ; attempt++ {
		err := fn(ctx)
		if err == nil {
			if attempt > 1 {
				slog.InfoContext(ctx, what+" succeeded", "attempts", attempt)
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.WarnContext(ctx, what+" failed, retrying", "attempt", attempt, "retry_in", backoff.String(), "error", err)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		backoff = min(backoff*2, maxBackoff)
	}
}
