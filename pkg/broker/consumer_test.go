package broker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func testConfig() ConsumerConfig {
	cfg := ConsumerConfig{Topic: "t", MaxAttempts: 3, Backoff: time.Millisecond}
	cfg.applyDefaults()
	return cfg
}

func TestHandleWithRetry(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name      string
		failFirst int // handler fails this many times, then succeeds
		wantCalls int
	}{
		{"succeeds first time", 0, 1},
		{"recovers after transient failures", 2, 3},
		{"gives up after MaxAttempts", 10, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			handler := func(context.Context, kafka.Message) error {
				calls++
				if calls <= tt.failFirst {
					return boom
				}
				return nil
			}

			// With no DLQ an exhausted message is dropped, so it is still safe to commit.
			if err := handleWithRetry(context.Background(), testConfig(), handler, kafka.Message{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if calls != tt.wantCalls {
				t.Errorf("handler called %d times, want %d", calls, tt.wantCalls)
			}
		})
	}
}

func TestHandleWithRetryStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	handler := func(context.Context, kafka.Message) error {
		calls++
		cancel()
		return errors.New("boom")
	}

	// A cancelled consumer must not commit: the message belongs to whoever
	// picks the partition up next.
	if err := handleWithRetry(ctx, testConfig(), handler, kafka.Message{}); err == nil {
		t.Fatal("want an error so the offset is left uncommitted")
	}
	if calls != 1 {
		t.Errorf("handler called %d times after cancel, want 1", calls)
	}
}

func TestApplyDefaults(t *testing.T) {
	var cfg ConsumerConfig
	cfg.applyDefaults()
	if cfg.MaxAttempts != 3 || cfg.Backoff != 200*time.Millisecond {
		t.Errorf("got MaxAttempts=%d Backoff=%v", cfg.MaxAttempts, cfg.Backoff)
	}
}

func TestEnsureTopicsRejectsEmptyBrokers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, brokers := range []string{"", "  "} {
		if err := EnsureTopics(ctx, brokers, "t"); err == nil || errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("brokers %q: want an immediate config error, got %v", brokers, err)
		}
	}
}
