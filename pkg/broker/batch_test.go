package broker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

// fakeFetcher serves n messages, then blocks until ctx is done.
type fakeFetcher struct {
	next, n int64
}

func (f *fakeFetcher) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if f.next < f.n {
		f.next++
		return kafka.Message{Offset: f.next - 1}, nil
	}
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func TestFetchBatchCapsAtBatchSize(t *testing.T) {
	cfg := testConfig()
	cfg.BatchSize, cfg.BatchWait = 3, time.Second
	backoff := cfg.Backoff

	msgs, ok := fetchBatch(context.Background(), cfg, &fakeFetcher{n: 10}, &backoff)
	if !ok || len(msgs) != 3 {
		t.Fatalf("got %d messages (ok=%v), want 3", len(msgs), ok)
	}
}

func TestFetchBatchReturnsWhatArrivedWithinWait(t *testing.T) {
	cfg := testConfig()
	cfg.BatchSize, cfg.BatchWait = 100, 20*time.Millisecond
	backoff := cfg.Backoff

	start := time.Now()
	msgs, ok := fetchBatch(context.Background(), cfg, &fakeFetcher{n: 2}, &backoff)
	if !ok || len(msgs) != 2 {
		t.Fatalf("got %d messages (ok=%v), want 2", len(msgs), ok)
	}
	if time.Since(start) > time.Second {
		t.Errorf("waited %v, want about BatchWait", time.Since(start))
	}
}

func TestFetchBatchStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := testConfig()
	backoff := cfg.Backoff

	if _, ok := fetchBatch(ctx, cfg, &fakeFetcher{}, &backoff); ok {
		t.Fatal("want ok=false once ctx is done")
	}
}

func TestHandleBatchIsolatesPoisonMessage(t *testing.T) {
	cfg := testConfig()
	msgs := []kafka.Message{{Offset: 0}, {Offset: 1}, {Offset: 2}}
	var batchCalls int
	var handled []int64
	handler := func(_ context.Context, batch []kafka.Message) error {
		if len(batch) > 1 {
			batchCalls++
			return errors.New("batch has a bad row")
		}
		if batch[0].Offset == 1 {
			return errors.New("bad row")
		}
		handled = append(handled, batch[0].Offset)
		return nil
	}

	// with no DLQ the poison message is dropped, so the batch is safe to commit
	if err := handleBatch(context.Background(), cfg, handler, msgs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if batchCalls != cfg.MaxAttempts {
		t.Errorf("batch tried %d times, want %d", batchCalls, cfg.MaxAttempts)
	}
	if len(handled) != 2 || handled[0] != 0 || handled[1] != 2 {
		t.Errorf("handled %v, want [0 2]", handled)
	}
}

func TestHandleBatchSucceedsInOneCall(t *testing.T) {
	calls := 0
	handler := func(_ context.Context, batch []kafka.Message) error {
		calls++
		if len(batch) != 3 {
			t.Errorf("got batch of %d, want 3", len(batch))
		}
		return nil
	}

	if err := handleBatch(context.Background(), testConfig(), handler, make([]kafka.Message, 3)); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("handler called %d times, want 1", calls)
	}
}
