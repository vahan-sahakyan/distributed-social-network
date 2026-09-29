package broker

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestHeaderCarrierPropagatesTrace(t *testing.T) {
	prop := propagation.TraceContext{}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})
	msg := kafka.Message{Headers: []kafka.Header{{Key: "other", Value: []byte("x")}}}

	prop.Inject(trace.ContextWithSpanContext(context.Background(), sc), headerCarrier{&msg.Headers})
	prop.Inject(trace.ContextWithSpanContext(context.Background(), sc), headerCarrier{&msg.Headers})
	got := trace.SpanContextFromContext(prop.Extract(context.Background(), headerCarrier{&msg.Headers}))

	if got.TraceID() != sc.TraceID() || got.SpanID() != sc.SpanID() {
		t.Errorf("extracted %v, want %v", got, sc)
	}
	if len(msg.Headers) != 2 {
		t.Errorf("re-injecting should overwrite, got headers %v", msg.Headers)
	}
}
