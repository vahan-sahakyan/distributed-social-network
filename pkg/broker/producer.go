package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Producer struct {
	mu      sync.Mutex
	writers map[string]*kafka.Writer
	brokers []string
}

func NewProducer(brokers string) *Producer {
	return &Producer{
		writers: make(map[string]*kafka.Writer),
		brokers: strings.Split(brokers, ","),
	}
}

// Publish sends payload to topic. key routes related events to the same partition, preserving their order.
func (p *Producer) Publish(ctx context.Context, topic, key string, payload any) error {
	ctx, span := tracer.Start(ctx, "publish "+topic,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", topic),
		),
	)
	defer span.End()

	err := p.write(ctx, topic, key, payload)
	published.WithLabelValues(topic, result(err)).Inc()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func (p *Producer) write(ctx context.Context, topic, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := kafka.Message{Key: []byte(key), Value: data}
	otel.GetTextMapPropagator().Inject(ctx, headerCarrier{&msg.Headers})
	return p.getWriter(topic).WriteMessages(ctx, msg)
}

func (p *Producer) getWriter(topic string) *kafka.Writer {
	p.mu.Lock()
	defer p.mu.Unlock()

	if w, ok := p.writers[topic]; ok {
		return w
	}

	w := &kafka.Writer{
		Addr:     kafka.TCP(p.brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{},
		// default of 1s makes every synchronous WriteMessages wait that long
		BatchTimeout: 10 * time.Millisecond,
		// the default, RequireNone, reports success for writes the broker never got
		RequiredAcks: kafka.RequireAll,
	}
	p.writers[topic] = w
	return w
}

func (p *Producer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, w := range p.writers {
		w.Close()
	}
}

// EnsureTopics creates any missing topics, retrying until the broker is reachable or ctx is done.
// An empty brokers string is rejected outright rather than retried forever against no address.
func EnsureTopics(ctx context.Context, brokers string, topics ...string) error {
	if strings.TrimSpace(brokers) == "" {
		return errors.New("no kafka brokers configured (KAFKA_BROKERS is empty)")
	}

	addrs := strings.Split(brokers, ",")
	configs := make([]kafka.TopicConfig, len(topics))
	for i, t := range topics {
		configs[i] = kafka.TopicConfig{Topic: t, NumPartitions: 3, ReplicationFactor: 1}
	}

	client := &kafka.Client{Addr: kafka.TCP(addrs...), Timeout: 10 * time.Second}
	for {
		err := createTopics(ctx, client, configs)
		if err == nil {
			return nil
		}
		slog.WarnContext(ctx, "waiting for kafka to create topics", "topics", topics, "error", err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("creating topics %v: %w", topics, ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}
}

func createTopics(ctx context.Context, client *kafka.Client, configs []kafka.TopicConfig) error {
	resp, err := client.CreateTopics(ctx, &kafka.CreateTopicsRequest{Topics: configs})
	if err != nil {
		return err
	}
	for topic, err := range resp.Errors {
		if err != nil && !errors.Is(err, kafka.TopicAlreadyExists) {
			return fmt.Errorf("%s: %w", topic, err)
		}
	}
	return nil
}
