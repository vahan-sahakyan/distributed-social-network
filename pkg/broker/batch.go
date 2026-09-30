package broker

import (
	"context"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// BatchHandler processes messages in order. Returning an error retries the batch.
type BatchHandler func(ctx context.Context, msgs []kafka.Message) error

// ConsumeBatch is Consume for handlers that are cheaper per batch, like bulk inserts.
// It hands over up to cfg.BatchSize messages, or whatever arrived within cfg.BatchWait
// of the first one, and commits them together. A batch that exhausts its attempts is
// retried message by message, so only the messages that still fail are parked.
func ConsumeBatch(ctx context.Context, cfg ConsumerConfig, handler BatchHandler) {
	cfg.applyDefaults()
	reader, ok := openReader(ctx, cfg)
	if !ok {
		return
	}
	defer reader.Close()

	backoff := cfg.Backoff
	for {
		msgs, ok := fetchBatch(ctx, cfg, reader, &backoff)
		if !ok {
			return
		}
		if err := handleBatch(ctx, cfg, handler, msgs); err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		commit(ctx, cfg, reader, msgs...)
	}
}

func fetchBatch(ctx context.Context, cfg ConsumerConfig, r fetcher, backoff *time.Duration) ([]kafka.Message, bool) {
	first, ok := fetch(ctx, cfg, r, backoff)
	if !ok {
		return nil, false
	}

	msgs := []kafka.Message{first}
	waitCtx, cancel := context.WithTimeout(ctx, cfg.BatchWait)
	defer cancel()
	for len(msgs) < cfg.BatchSize {
		// ends on the wait deadline; a read error is left to the next fetch
		msg, err := r.FetchMessage(waitCtx)
		if err != nil {
			break
		}
		msgs = append(msgs, msg)
	}
	return msgs, true
}

// handleBatch returns an error only when the batch must not be committed.
func handleBatch(ctx context.Context, cfg ConsumerConfig, handler BatchHandler, msgs []kafka.Message) error {
	single := func(ctx context.Context, msg kafka.Message) error {
		return handler(ctx, []kafka.Message{msg})
	}

	// a lone message keeps the producer's trace as its parent
	if len(msgs) == 1 {
		msgCtx, span := startConsumerSpan(ctx, cfg, &msgs[0])
		err := handleWithRetry(msgCtx, cfg, single, msgs[0])
		endSpan(span, err)
		return err
	}

	batchCtx, span := startBatchSpan(ctx, cfg, msgs)
	err := batchWithRetry(batchCtx, cfg, handler, msgs)
	endSpan(span, err)
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	for i := range msgs {
		msgCtx, span := startConsumerSpan(ctx, cfg, &msgs[i])
		err := handleWithRetry(msgCtx, cfg, single, msgs[i])
		endSpan(span, err)
		if err != nil {
			return err
		}
	}
	return nil
}

func batchWithRetry(ctx context.Context, cfg ConsumerConfig, handler BatchHandler, msgs []kafka.Message) error {
	backoff := cfg.Backoff
	var err error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		start := time.Now()
		err = handler(ctx, msgs)
		handlerDuration.WithLabelValues(cfg.Topic, cfg.GroupID).Observe(time.Since(start).Seconds())
		if err == nil {
			consumed.WithLabelValues(cfg.Topic, cfg.GroupID, "ok").Add(float64(len(msgs)))
			return nil
		}
		failedAttempts.WithLabelValues(cfg.Topic, cfg.GroupID).Inc()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.WarnContext(ctx, "batch attempt failed", "topic", cfg.Topic, "size", len(msgs),
			"attempt", attempt, "max_attempts", cfg.MaxAttempts, "error", err)

		if attempt < cfg.MaxAttempts {
			if !sleep(ctx, backoff) {
				return ctx.Err()
			}
			backoff *= 2
		}
	}
	return err
}
