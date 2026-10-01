// Package outbox publishes events through a table written in the same transaction
// as the data they describe, so a commit can't lose its event (ADR 0002).
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Schema creates the outbox table; apply it with the service's migrations.
const Schema = `
CREATE TABLE IF NOT EXISTS outbox (
    id BIGSERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    key TEXT NOT NULL,
    payload BYTEA NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// Execer is satisfied by pgx.Tx; pass the transaction that writes the data.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Enqueue stores payload as JSON for publishing to topic once tx commits, along
// with the current trace context.
func Enqueue(ctx context.Context, tx Execer, topic, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding %s event: %w", topic, err)
	}
	headers := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, headers)
	h, err := json.Marshal(headers)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO outbox (topic, key, payload, headers) VALUES ($1, $2, $3, $4)`,
		topic, key, data, h,
	); err != nil {
		return fmt.Errorf("enqueueing %s event: %w", topic, err)
	}
	return nil
}

var (
	backlog = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "outbox_backlog",
		Help: "Events waiting in the outbox.",
	}, []string{"service"})
	oldestAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "outbox_oldest_age_seconds",
		Help: "Age of the oldest event waiting in the outbox.",
	}, []string{"service"})
)

// Publisher is satisfied by *broker.Producer.
type Publisher interface {
	PublishMessages(ctx context.Context, msgs []broker.Message) error
}

// lockID is the advisory lock a relay holds while publishing; locks are per
// database, and each service has its own.
const lockID int64 = 0x6f7574626f78

// Relay publishes outbox rows in id order and deletes them once the broker acked.
type Relay struct {
	db        *pgxpool.Pool
	publisher Publisher
	service   string

	BatchSize int
	// Idle is the pause after a poll that found less than a full batch.
	Idle time.Duration
}

func NewRelay(db *pgxpool.Pool, publisher Publisher, service string) *Relay {
	return &Relay{
		db:        db,
		publisher: publisher,
		service:   service,
		BatchSize: 100,
		Idle:      100 * time.Millisecond,
	}
}

// Run relays until ctx is done. A failed publish is retried with backoff; the rows
// stay in the outbox meanwhile.
func (r *Relay) Run(ctx context.Context) {
	go r.reportBacklog(ctx)

	backoff := time.Second
	for {
		n, err := r.relayOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		pause := r.Idle
		switch {
		case err != nil:
			slog.WarnContext(ctx, "relaying outbox", "error", err, "retry_in", backoff.String())
			pause = backoff
			backoff = min(backoff*2, 30*time.Second)
		case n == r.BatchSize:
			backoff, pause = time.Second, 0
		default:
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(pause):
		}
	}
}

type row struct {
	id      int64
	topic   string
	key     string
	payload []byte
	headers map[string]string
}

// relayOnce publishes one batch and returns how many rows it relayed.
func (r *Relay) relayOnce(ctx context.Context) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// one relay per database at a time, so events leave in commit order
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, lockID).Scan(&locked); err != nil {
		return 0, err
	}
	if !locked {
		return 0, nil
	}

	rows, err := tx.Query(ctx,
		`SELECT id, topic, key, payload, headers FROM outbox ORDER BY id LIMIT $1`, r.BatchSize)
	if err != nil {
		return 0, err
	}
	batch, err := pgx.CollectRows(rows, func(rs pgx.CollectableRow) (row, error) {
		var x row
		err := rs.Scan(&x.id, &x.topic, &x.key, &x.payload, &x.headers)
		return x, err
	})
	if err != nil || len(batch) == 0 {
		return 0, err
	}

	msgs := make([]broker.Message, len(batch))
	ids := make([]int64, len(batch))
	for i, x := range batch {
		msgs[i] = broker.Message{
			Ctx:   otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(x.headers)),
			Topic: x.topic,
			Key:   x.key,
			Value: x.payload,
		}
		ids[i] = x.id
	}
	if err := r.publisher.PublishMessages(ctx, msgs); err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM outbox WHERE id = ANY($1)`, ids); err != nil {
		return 0, err
	}
	return len(batch), tx.Commit(ctx)
}

func (r *Relay) reportBacklog(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		var count int64
		var age float64
		err := r.db.QueryRow(ctx,
			`SELECT count(*), coalesce(extract(epoch FROM now() - min(created_at)), 0) FROM outbox`,
		).Scan(&count, &age)
		if err == nil {
			backlog.WithLabelValues(r.service).Set(float64(count))
			oldestAge.WithLabelValues(r.service).Set(age)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
