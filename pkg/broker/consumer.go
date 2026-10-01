package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
)

// Handler processes one message. Returning an error makes Consume retry it, and
// the message is only committed once the handler succeeds or it is parked on the
// dead-letter topic - so a failed handler no longer drops the event.
type Handler func(ctx context.Context, msg kafka.Message) error

type ConsumerConfig struct {
	Brokers string
	Topic   string
	GroupID string

	// MaxAttempts is how many times a message is handed to the handler before it
	// goes to the dead-letter topic. Defaults to 3.
	MaxAttempts int
	// Backoff is the pause after the first failed attempt, doubled each retry.
	// Defaults to 200ms.
	Backoff time.Duration
	// DLQ parks messages that exhausted MaxAttempts. When nil they are dropped
	// with a log line, which is the old at-most-once behaviour.
	DLQ *Producer

	// BatchSize caps a ConsumeBatch batch. Defaults to 500.
	BatchSize int
	// BatchWait is how long ConsumeBatch collects after the first message of a
	// batch. Defaults to 200ms.
	BatchWait time.Duration
}

func (c *ConsumerConfig) applyDefaults() {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.Backoff <= 0 {
		c.Backoff = 200 * time.Millisecond
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 500
	}
	if c.BatchWait <= 0 {
		c.BatchWait = 200 * time.Millisecond
	}
}

// Consume reads cfg.Topic until ctx is done, committing each message only after
// the handler has accepted it. It blocks.
func Consume(ctx context.Context, cfg ConsumerConfig, handler Handler) {
	cfg.applyDefaults()
	reader, ok := openReader(ctx, cfg)
	if !ok {
		return
	}
	defer reader.Close()

	backoff := cfg.Backoff
	for {
		msg, ok := fetch(ctx, cfg, reader, &backoff)
		if !ok {
			return
		}

		msgCtx, span := startConsumerSpan(ctx, cfg, &msg)
		err := handleWithRetry(msgCtx, cfg, handler, msg)
		endSpan(span, err)
		if err != nil {
			// Leaving the offset uncommitted is deliberate: the message is
			// redelivered rather than silently skipped.
			if ctx.Err() != nil {
				return
			}
			continue
		}

		commit(ctx, cfg, reader, msg)
	}
}

// openReader creates the dead-letter topic and the group reader. It returns false
// once ctx is done.
func openReader(ctx context.Context, cfg ConsumerConfig) (*kafka.Reader, bool) {
	// the writer doesn't create topics, so parking would fail on a missing DLQ topic
	if cfg.DLQ != nil {
		if err := EnsureTopics(ctx, cfg.Brokers, events.DLQ(cfg.Topic)); err != nil {
			slog.ErrorContext(ctx, "ensuring dead-letter topic", "topic", cfg.Topic, "error", err)
			return nil, false
		}
	}

	initMetrics(cfg)
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(cfg.Brokers, ","),
		Topic:   cfg.Topic,
		GroupID: cfg.GroupID,
	}), true
}

type fetcher interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
}

// fetch blocks for the next message, backing off while the broker is unreachable so
// an outage does not spin the loop. It returns false once ctx is done.
func fetch(ctx context.Context, cfg ConsumerConfig, r fetcher, backoff *time.Duration) (kafka.Message, bool) {
	for {
		// FetchMessage, unlike ReadMessage, leaves the offset uncommitted.
		msg, err := r.FetchMessage(ctx)
		if err == nil {
			*backoff = cfg.Backoff
			return msg, true
		}
		if ctx.Err() != nil {
			return kafka.Message{}, false
		}
		slog.WarnContext(ctx, "reading message", "topic", cfg.Topic, "error", err)
		if !sleep(ctx, *backoff) {
			return kafka.Message{}, false
		}
		if *backoff < 30*time.Second {
			*backoff *= 2
		}
	}
}

func commit(ctx context.Context, cfg ConsumerConfig, reader *kafka.Reader, msgs ...kafka.Message) {
	if err := reader.CommitMessages(ctx, msgs...); err != nil && ctx.Err() == nil {
		slog.ErrorContext(ctx, "committing offsets", "topic", cfg.Topic,
			"last_offset", msgs[len(msgs)-1].Offset, "error", err)
	}
}

// sleep waits d, returning false if ctx is done first.
func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// handleWithRetry runs the handler up to MaxAttempts times, then parks the
// message on the dead-letter topic. It returns an error only when the message
// must not be committed.
func handleWithRetry(ctx context.Context, cfg ConsumerConfig, handler Handler, msg kafka.Message) error {
	backoff := cfg.Backoff
	var err error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		start := time.Now()
		err = handler(ctx, msg)
		handlerDuration.WithLabelValues(cfg.Topic, cfg.GroupID).Observe(time.Since(start).Seconds())
		if err == nil {
			consumed.WithLabelValues(cfg.Topic, cfg.GroupID, "ok").Inc()
			return nil
		}
		failedAttempts.WithLabelValues(cfg.Topic, cfg.GroupID).Inc()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.WarnContext(ctx, "handler attempt failed", "topic", cfg.Topic, "offset", msg.Offset,
			"attempt", attempt, "max_attempts", cfg.MaxAttempts, "error", err)

		if attempt < cfg.MaxAttempts {
			if !sleep(ctx, backoff) {
				return ctx.Err()
			}
			backoff *= 2
		}
	}

	return park(ctx, cfg, msg, err)
}

// park sends an exhausted message to the dead-letter topic. If it cannot be
// parked the error is returned so the caller leaves the offset uncommitted.
func park(ctx context.Context, cfg ConsumerConfig, msg kafka.Message, cause error) error {
	if cfg.DLQ == nil {
		consumed.WithLabelValues(cfg.Topic, cfg.GroupID, "dropped").Inc()
		slog.ErrorContext(ctx, "dropping message, no DLQ configured", "topic", cfg.Topic, "offset", msg.Offset,
			"attempts", cfg.MaxAttempts, "error", cause)
		return nil
	}

	dead := DeadLetter{
		Topic:     cfg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       string(msg.Key),
		Payload:   string(msg.Value),
		Error:     cause.Error(),
		FailedAt:  time.Now().UTC(),
		Group:     cfg.GroupID,
		EventID:   header(msg, EventIDHeader),
		ReplayOf:  header(msg, ReplayOfHeader),
	}
	if err := cfg.DLQ.Publish(ctx, events.DLQ(cfg.Topic), string(msg.Key), dead); err != nil {
		consumed.WithLabelValues(cfg.Topic, cfg.GroupID, "error").Inc()
		err = fmt.Errorf("parking offset %d: %w", msg.Offset, errors.Join(cause, err))
		slog.ErrorContext(ctx, "parking message", "topic", cfg.Topic, "offset", msg.Offset, "error", err)
		return err
	}

	consumed.WithLabelValues(cfg.Topic, cfg.GroupID, "dlq").Inc()
	slog.ErrorContext(ctx, "parked message on DLQ", "topic", cfg.Topic, "offset", msg.Offset,
		"dlq", events.DLQ(cfg.Topic), "attempts", cfg.MaxAttempts, "error", cause)
	return nil
}
