package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceHandlerAddsIDs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newTraceHandler(slog.NewJSONHandler(&buf, nil))).With("service", "svc")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})

	logger.InfoContext(trace.ContextWithSpanContext(context.Background(), sc), "traced")
	logger.Info("untraced")

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	var traced, untraced map[string]any
	if err := json.Unmarshal(lines[0], &traced); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lines[1], &untraced); err != nil {
		t.Fatal(err)
	}
	if traced["trace_id"] != sc.TraceID().String() || traced["span_id"] != sc.SpanID().String() || traced["service"] != "svc" {
		t.Errorf("traced line: %v", traced)
	}
	if _, ok := untraced["trace_id"]; ok {
		t.Errorf("untraced line has trace_id: %v", untraced)
	}
}
