package outbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type fakePublisher struct {
	got []broker.Message
	err error
}

func (f *fakePublisher) PublishMessages(_ context.Context, msgs []broker.Message) error {
	if f.err != nil {
		return f.err
	}
	f.got = append(f.got, msgs...)
	return nil
}

// testDB connects to OUTBOX_TEST_DATABASE_URL in a throwaway schema, e.g.
// postgres://postgres:postgres@localhost:5433/comments?sslmode=disable
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("OUTBOX_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("OUTBOX_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("outbox_test_%d", time.Now().UnixNano())

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close(context.Background())
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(ctx, Schema); err != nil {
		t.Fatal(err)
	}
	return db
}

func enqueue(t *testing.T, db *pgxpool.Pool, ctx context.Context, commit bool, keys ...string) {
	t.Helper()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if err := Enqueue(ctx, tx, "t", k, map[string]string{"k": k}); err != nil {
			t.Fatal(err)
		}
	}
	if commit {
		err = tx.Commit(ctx)
	} else {
		err = tx.Rollback(ctx)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func pending(t *testing.T, db *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), "SELECT count(*) FROM outbox").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRelayPublishesCommittedEventsInOrder(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	db := testDB(t)
	pub := &fakePublisher{}
	relay := NewRelay(db, pub, "test")

	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	reqCtx := trace.ContextWithSpanContext(context.Background(),
		trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled}))

	enqueue(t, db, reqCtx, false, "rolled-back")
	enqueue(t, db, reqCtx, true, "a", "b", "c")

	n, err := relay.relayOnce(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("relayed %d, err %v; want 3", n, err)
	}
	var keys string
	for _, m := range pub.got {
		keys += m.Key
		if want := `{"k":"` + m.Key + `"}`; string(m.Value) != want {
			t.Errorf("message %s carries %s, want %s", m.Key, m.Value, want)
		}
		if got := trace.SpanContextFromContext(m.Ctx).TraceID(); got != traceID {
			t.Errorf("message %s lost the request trace: %s", m.Key, got)
		}
	}
	if keys != "abc" {
		t.Errorf("published %q, want abc in commit order", keys)
	}
	if pending(t, db) != 0 {
		t.Error("published rows were not deleted")
	}
}

func TestRelayKeepsRowsWhenPublishFails(t *testing.T) {
	db := testDB(t)
	relay := NewRelay(db, &fakePublisher{err: errors.New("broker down")}, "test")
	enqueue(t, db, context.Background(), true, "a", "b")

	if _, err := relay.relayOnce(context.Background()); err == nil {
		t.Fatal("want the publish error")
	}
	if n := pending(t, db); n != 2 {
		t.Errorf("%d rows pending, want 2 kept for retry", n)
	}
}

func TestRelayYieldsToTheLockHolder(t *testing.T) {
	db := testDB(t)
	pub := &fakePublisher{}
	enqueue(t, db, context.Background(), true, "a")

	ctx := context.Background()
	holder, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockID); err != nil {
		t.Fatal(err)
	}

	n, err := NewRelay(db, pub, "test").relayOnce(ctx)
	if err != nil || n != 0 || len(pub.got) != 0 {
		t.Fatalf("relayed %d (err %v) while another relay held the lock", n, err)
	}
}
