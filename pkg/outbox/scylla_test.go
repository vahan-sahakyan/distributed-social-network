package outbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// scyllaDB connects to OUTBOX_TEST_SCYLLA_HOSTS (e.g. localhost:9042) in a throwaway keyspace.
func scyllaDB(t *testing.T) *gocql.Session {
	t.Helper()
	hosts := os.Getenv("OUTBOX_TEST_SCYLLA_HOSTS")
	if hosts == "" {
		t.Skip("OUTBOX_TEST_SCYLLA_HOSTS not set")
	}
	ks := fmt.Sprintf("outbox_test_%d", time.Now().UnixNano())

	admin := gocql.NewCluster(strings.Split(hosts, ",")...)
	admin.Timeout = 30 * time.Second
	adminSession, err := admin.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := adminSession.Query(`CREATE KEYSPACE ` + ks + ` WITH replication = {'class': 'NetworkTopologyStrategy', 'replication_factor': 1}`).Exec(); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range strings.Split(ScyllaSchema(ks), ";") {
		if err := adminSession.Query(strings.TrimSpace(stmt)).Exec(); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_ = adminSession.Query(`DROP KEYSPACE ` + ks).Exec()
		adminSession.Close()
	})

	cluster := gocql.NewCluster(strings.Split(hosts, ",")...)
	cluster.Keyspace = ks
	cluster.Timeout = 30 * time.Second
	db, err := cluster.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

func enqueueScylla(t *testing.T, db *gocql.Session, ctx context.Context, key string) {
	t.Helper()
	b := db.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	if err := AddToBatch(ctx, b, "t", key, map[string]string{"k": key}); err != nil {
		t.Fatal(err)
	}
	if err := db.ExecuteBatch(b); err != nil {
		t.Fatal(err)
	}
}

func scyllaPending(t *testing.T, db *gocql.Session) int {
	t.Helper()
	var n int
	iter := db.Query(`SELECT id FROM outbox`).Iter()
	for iter.Scan(new(gocql.UUID)) {
		n++
	}
	if err := iter.Close(); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestScyllaRelayPublishesInOrderWithTrace(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	db := scyllaDB(t)
	pub := &fakePublisher{}
	relay := NewScyllaRelay(db, pub, "test")

	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	reqCtx := trace.ContextWithSpanContext(context.Background(),
		trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled}))
	for _, k := range []string{"a", "b", "c"} {
		enqueueScylla(t, db, reqCtx, k)
	}

	n, err := relay.relayOnce(context.Background(), time.Now().Add(time.Second))
	if err != nil || n != 3 {
		t.Fatalf("relayed %d, err %v; want 3", n, err)
	}
	var keys string
	for _, m := range pub.got {
		keys += m.Key
		if !strings.HasPrefix(m.ID, "test:") {
			t.Errorf("message %s has event id %q, want test:<row id>", m.Key, m.ID)
		}
		if want := `{"k":"` + m.Key + `"}`; string(m.Value) != want {
			t.Errorf("message %s carries %s, want %s", m.Key, m.Value, want)
		}
		if trace.SpanContextFromContext(m.Ctx).TraceID() != traceID {
			t.Errorf("message %s lost the request trace", m.Key)
		}
	}
	if keys != "abc" {
		t.Errorf("published %q, want abc", keys)
	}
	if n := scyllaPending(t, db); n != 0 {
		t.Errorf("%d rows left after relaying", n)
	}
}

func TestScyllaRelayKeepsRowsWhenPublishFails(t *testing.T) {
	db := scyllaDB(t)
	enqueueScylla(t, db, context.Background(), "a")

	relay := NewScyllaRelay(db, &fakePublisher{err: errors.New("broker down")}, "test")
	if _, err := relay.relayOnce(context.Background(), time.Now().Add(time.Second)); err == nil {
		t.Fatal("want the publish error")
	}
	if n := scyllaPending(t, db); n != 1 {
		t.Errorf("%d rows pending, want 1 kept for retry", n)
	}
}

func TestScyllaRelaySweepCatchesLateRows(t *testing.T) {
	db := scyllaDB(t)
	pub := &fakePublisher{}
	relay := NewScyllaRelay(db, pub, "test")
	start := time.Now()

	enqueueScylla(t, db, context.Background(), "on-time")
	if n, err := relay.relayOnce(context.Background(), start.Add(time.Second)); err != nil || n != 1 {
		t.Fatalf("relayed %d, err %v", n, err)
	}

	// a write that lands after the relay moved past its id, e.g. a replayed batch;
	// kept in the on-time row's minute, which an earlier minute would not test
	late := start.Add(-500 * time.Millisecond)
	if minuteStart := time.Unix(bucketOf(start)*60, 0); late.Before(minuteStart) {
		late = minuteStart
	}
	if err := db.Query(`INSERT INTO outbox (bucket, id, topic, key, payload, headers) VALUES (?, ?, 't', 'late', ?, {})`,
		bucketOf(late), gocql.UUIDFromTime(late), []byte(`{}`)).Exec(); err != nil {
		t.Fatal(err)
	}
	if n, _ := relay.relayOnce(context.Background(), start.Add(2*time.Second)); n != 0 {
		t.Fatalf("a normal poll resumes after the last id; relayed %d", n)
	}

	afterClose := start.Add(scyllaClose + 2*time.Minute)
	if n, err := relay.relayOnce(context.Background(), afterClose); err != nil || n != 1 || pub.got[1].Key != "late" {
		t.Fatalf("closing sweep relayed %d (err %v), want the late row", n, err)
	}
	if _, err := relay.relayOnce(context.Background(), afterClose); err != nil {
		t.Fatal(err)
	}
	if relay.cursor <= bucketOf(late) {
		t.Errorf("cursor %d did not move past the drained minute %d", relay.cursor, bucketOf(late))
	}
}

func TestScyllaRelayYieldsToTheLeaseHolder(t *testing.T) {
	db := scyllaDB(t)
	enqueueScylla(t, db, context.Background(), "a")
	now := time.Now().Add(time.Second)

	first, second := &fakePublisher{}, &fakePublisher{}
	if n, err := NewScyllaRelay(db, first, "test").relayOnce(context.Background(), now); err != nil || n != 1 {
		t.Fatalf("first relay: %d, %v", n, err)
	}
	enqueueScylla(t, db, context.Background(), "b")
	if n, err := NewScyllaRelay(db, second, "test").relayOnce(context.Background(), now.Add(time.Second)); err != nil || n != 0 {
		t.Fatalf("second relay relayed %d (err %v) while the first held the lease", n, err)
	}
}
