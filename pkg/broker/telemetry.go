package broker

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("github.com/vahan-sahakyan/distributed-social-network/pkg/broker")

var (
	published = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_messages_published_total",
		Help: "Messages published, by topic and result (ok, error).",
	}, []string{"topic", "result"})

	consumed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_messages_consumed_total",
		Help: "Messages consumed, by final result (ok, dlq, dropped, error).",
	}, []string{"topic", "group", "result"})

	failedAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "broker_handler_failures_total",
		Help: "Failed handler attempts, retried or not.",
	}, []string{"topic", "group"})

	handlerDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "broker_handler_duration_seconds",
		Help:    "Duration of a single handler attempt.",
		Buckets: prometheus.DefBuckets,
	}, []string{"topic", "group"})
)

// initMetrics creates every series at zero, so increase() sees the first failure.
func initMetrics(cfg ConsumerConfig) {
	for _, r := range []string{"ok", "dlq", "dropped", "error"} {
		consumed.WithLabelValues(cfg.Topic, cfg.GroupID, r)
	}
	failedAttempts.WithLabelValues(cfg.Topic, cfg.GroupID)
}

func result(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// headerCarrier lets the otel propagator read and write kafka message headers.
type headerCarrier struct {
	headers *[]kafka.Header
}

func (c headerCarrier) Get(key string) string {
	for _, h := range *c.headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c headerCarrier) Set(key, value string) {
	for i, h := range *c.headers {
		if h.Key == key {
			(*c.headers)[i].Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, kafka.Header{Key: key, Value: []byte(value)})
}

func (c headerCarrier) Keys() []string {
	keys := make([]string, len(*c.headers))
	for i, h := range *c.headers {
		keys[i] = h.Key
	}
	return keys
}

// startConsumerSpan continues the producer's trace, so a request and the events it
// caused show up as one trace.
func startConsumerSpan(ctx context.Context, cfg ConsumerConfig, msg *kafka.Message) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, headerCarrier{&msg.Headers})
	return tracer.Start(ctx, "process "+cfg.Topic,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", cfg.Topic),
			attribute.String("messaging.consumer.group.name", cfg.GroupID),
			attribute.Int("messaging.destination.partition.id", msg.Partition),
			attribute.Int64("messaging.kafka.offset", msg.Offset),
		),
	)
}
