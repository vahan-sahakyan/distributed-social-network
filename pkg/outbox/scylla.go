package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gocql/gocql"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// ScyllaDB has no multi-table transactions, so the event goes into the same
// LOGGED BATCH as the data. The outbox is partitioned by minute and read from the
// last relayed id onward, so polls skip the tombstones of relayed rows. Before a
// minute is closed it is swept once from the start, which catches a write that
// landed late with an earlier id, and then dropped whole (ADR 0002).

const (
	scyllaTTL = 7 * 24 * time.Hour
	// rows younger than this wait a poll, so concurrent writes mostly leave in id order
	scyllaSettle = 200 * time.Millisecond
	// a minute is closed for good once it is drained and this old
	scyllaClose = 2 * time.Minute
	scyllaLease = 15 * time.Second
)

// ScyllaSchema creates the outbox tables in keyspace.
func ScyllaSchema(keyspace string) string {
	return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.outbox (
    bucket bigint,
    id timeuuid,
    topic text,
    key text,
    payload blob,
    headers map<text, text>,
    PRIMARY KEY (bucket, id)
) WITH default_time_to_live = %[2]d AND gc_grace_seconds = 3600;

CREATE TABLE IF NOT EXISTS %[1]s.outbox_state (
    name text PRIMARY KEY,
    owner text,
    cursor bigint
)`, keyspace, int(scyllaTTL.Seconds()))
}

func bucketOf(t time.Time) int64 { return t.Unix() / 60 }

// AddToBatch appends the event to b, a LOGGED batch that also writes the data.
func AddToBatch(ctx context.Context, b *gocql.Batch, topic, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding %s event: %w", topic, err)
	}
	headers := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, headers)
	now := time.Now()
	b.Query(`INSERT INTO outbox (bucket, id, topic, key, payload, headers) VALUES (?, ?, ?, ?, ?, ?)`,
		bucketOf(now), gocql.UUIDFromTime(now), topic, key, data, map[string]string(headers))
	return nil
}

// ScyllaRelay publishes outbox rows in id order. A lease row makes sure only one
// relay per keyspace publishes at a time.
type ScyllaRelay struct {
	db        *gocql.Session
	publisher Publisher
	service   string
	owner     string

	BatchSize int
	Idle      time.Duration

	cursor   int64
	lastID   map[int64]gocql.UUID
	leaseEnd time.Time
}

func NewScyllaRelay(db *gocql.Session, publisher Publisher, service string) *ScyllaRelay {
	return &ScyllaRelay{
		db:        db,
		publisher: publisher,
		service:   service,
		owner:     gocql.TimeUUID().String(),
		BatchSize: 100,
		Idle:      100 * time.Millisecond,
		lastID:    map[int64]gocql.UUID{},
	}
}

// Run relays until ctx is done; failures keep the rows and back off.
func (r *ScyllaRelay) Run(ctx context.Context) {
	go r.reportBacklog(ctx)

	backoff := time.Second
	for {
		n, err := r.relayOnce(ctx, time.Now())
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

// relayOnce publishes up to one batch from the oldest open minute and returns how
// many rows it relayed.
func (r *ScyllaRelay) relayOnce(ctx context.Context, now time.Time) (int, error) {
	leader, err := r.holdLease(ctx, now)
	if err != nil || !leader {
		return 0, err
	}
	if r.cursor == 0 {
		if err := r.loadCursor(ctx, now); err != nil {
			return 0, err
		}
	}

	settled := gocql.MaxTimeUUID(now.Add(-scyllaSettle))
	for b := r.cursor; b <= bucketOf(now); b++ {
		// the oldest minute, once old enough, is read from its start: a final sweep
		closing := b == r.cursor && b < bucketOf(now.Add(-scyllaClose))
		after, resume := r.lastID[b]
		if closing || !resume {
			after = gocql.MinTimeUUID(time.Unix(b*60, 0).Add(-time.Second))
		}
		msgs, ids, err := r.read(ctx, b, after, settled)
		if err != nil {
			return 0, err
		}
		if len(msgs) > 0 {
			if err := r.publisher.PublishMessages(ctx, msgs); err != nil {
				return 0, err
			}
			if err := r.db.Query(`DELETE FROM outbox WHERE bucket = ? AND id IN ?`, b, ids).WithContext(ctx).Exec(); err != nil {
				return 0, err
			}
			if last := ids[len(ids)-1]; !closing || resume {
				r.lastID[b] = last
			}
			return len(msgs), nil
		}
		if closing {
			if err := r.close(ctx, b); err != nil {
				return 0, err
			}
		}
	}
	return 0, nil
}

func (r *ScyllaRelay) read(ctx context.Context, bucket int64, after, settled gocql.UUID) ([]broker.Message, []gocql.UUID, error) {
	iter := r.db.Query(`SELECT id, topic, key, payload, headers FROM outbox
		WHERE bucket = ? AND id > ? AND id <= ? LIMIT ?`, bucket, after, settled, r.BatchSize).WithContext(ctx).Iter()

	var (
		msgs    []broker.Message
		ids     []gocql.UUID
		id      gocql.UUID
		topic   string
		key     string
		payload []byte
		headers map[string]string
	)
	for iter.Scan(&id, &topic, &key, &payload, &headers) {
		msgs = append(msgs, broker.Message{
			Ctx:   otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers)),
			Topic: topic,
			Key:   key,
			Value: payload,
		})
		ids = append(ids, id)
		// the driver scans into these in place; the messages keep the previous ones
		payload, headers = nil, nil
	}
	return msgs, ids, iter.Close()
}

// holdLease takes or renews the relay lease and reports whether this relay holds it.
func (r *ScyllaRelay) holdLease(ctx context.Context, now time.Time) (bool, error) {
	if now.Before(r.leaseEnd.Add(-scyllaLease / 3)) {
		return true, nil
	}
	ttl := int(scyllaLease.Seconds())
	held := !r.leaseEnd.IsZero()
	var applied bool
	var err error
	if held {
		applied, err = r.db.Query(`UPDATE outbox_state USING TTL ? SET owner = ? WHERE name = 'lease' IF owner = ?`,
			ttl, r.owner, r.owner).WithContext(ctx).ScanCAS(new(string))
	} else {
		applied, err = r.db.Query(`INSERT INTO outbox_state (name, owner) VALUES ('lease', ?) IF NOT EXISTS USING TTL ?`,
			r.owner, ttl).WithContext(ctx).ScanCAS(new(string), new(int64), new(string))
	}
	if err != nil {
		return false, err
	}
	if !applied {
		// someone else holds it; our view of the stream may be stale when we get it back
		r.leaseEnd, r.cursor = time.Time{}, 0
		clear(r.lastID)
		return false, nil
	}
	r.leaseEnd = now.Add(scyllaLease)
	return true, nil
}

func (r *ScyllaRelay) loadCursor(ctx context.Context, now time.Time) error {
	var cursor int64
	err := r.db.Query(`SELECT cursor FROM outbox_state WHERE name = 'cursor'`).WithContext(ctx).Scan(&cursor)
	if err != nil && !errors.Is(err, gocql.ErrNotFound) {
		return err
	}
	// rows older than the TTL are gone; a first start has nothing older than now
	oldest := bucketOf(now.Add(-scyllaTTL))
	if cursor == 0 {
		cursor = bucketOf(now.Add(-scyllaClose))
	}
	r.cursor = max(cursor, oldest)
	return nil
}

// close drops a drained minute, row tombstones included, and moves the cursor past it.
func (r *ScyllaRelay) close(ctx context.Context, bucket int64) error {
	if err := r.db.Query(`DELETE FROM outbox WHERE bucket = ?`, bucket).WithContext(ctx).Exec(); err != nil {
		return err
	}
	if err := r.db.Query(`UPDATE outbox_state SET cursor = ? WHERE name = 'cursor'`, bucket+1).WithContext(ctx).Exec(); err != nil {
		return err
	}
	delete(r.lastID, bucket)
	r.cursor = bucket + 1
	return nil
}

func (r *ScyllaRelay) reportBacklog(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if count, age, err := r.backlog(ctx, time.Now()); err == nil {
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

func (r *ScyllaRelay) backlog(ctx context.Context, now time.Time) (int64, float64, error) {
	from := r.cursor
	if from == 0 {
		from = bucketOf(now.Add(-scyllaClose))
	}
	var total int64
	var age float64
	for b := from; b <= bucketOf(now); b++ {
		var n int64
		if err := r.db.Query(`SELECT count(*) FROM outbox WHERE bucket = ?`, b).WithContext(ctx).Scan(&n); err != nil {
			return 0, 0, err
		}
		if n > 0 && total == 0 {
			var id gocql.UUID
			if err := r.db.Query(`SELECT id FROM outbox WHERE bucket = ? LIMIT 1`, b).WithContext(ctx).Scan(&id); err == nil {
				age = now.Sub(id.Time()).Seconds()
			}
		}
		total += n
	}
	return total, age, nil
}
