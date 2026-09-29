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
	"go.opentelemetry.io/otel/codes"
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
}

func (c *ConsumerConfig) applyDefaults() {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.Backoff <= 0 {
		c.Backoff = 200 * time.Millisecond
	}
}

// Consume reads cfg.Topic until ctx is done, committing each message only after
// the handler has accepted it. It blocks.
func Consume(ctx context.Context, cfg ConsumerConfig, handler Handler) {
	cfg.applyDefaults()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: strings.Split(cfg.Brokers, ","),
		Topic:   cfg.Topic,
		GroupID: cfg.GroupID,
	})
	defer reader.Close()

	initMetrics(cfg)

	// readBackoff keeps a broker outage from spinning this loop at full speed.
	readBackoff := cfg.Backoff
	for {
		// FetchMessage, unlike ReadMessage, leaves the offset uncommitted.
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.WarnContext(ctx, "reading message", "topic", cfg.Topic, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(readBackoff):
			}
			if readBackoff < 30*time.Second {
				readBackoff *= 2
			}
			continue
		}
		readBackoff = cfg.Backoff

		msgCtx, span := startConsumerSpan(ctx, cfg, &msg)
		err = handleWithRetry(msgCtx, cfg, handler, msg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
		if err != nil {
			// Leaving the offset uncommitted is deliberate: the message is
			// redelivered rather than silently skipped.
			if ctx.Err() != nil {
				return
			}
			continue
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.ErrorContext(ctx, "committing offset", "topic", cfg.Topic, "offset", msg.Offset, "error", err)
		}
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
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

	dead := map[string]any{
		"topic":     cfg.Topic,
		"partition": msg.Partition,
		"offset":    msg.Offset,
		"key":       string(msg.Key),
		"payload":   string(msg.Value),
		"error":     cause.Error(),
		"failed_at": time.Now().UTC(),
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
