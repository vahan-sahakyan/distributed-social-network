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

// Message is an encoded event that carries the context it was created in, whose
// span becomes the parent of its produce span.
type Message struct {
	Ctx   context.Context
	Topic string
	Key   string
	Value []byte
	// ID identifies the event across republishes; sent as the event-id header
	ID string
	// ReplayOf is sent as the replay-of header, for replays of events without an ID
	ReplayOf string
}

// PublishMessages writes msgs, keeping their order within each topic. It returns an
// error if any topic's write failed; messages of other topics may have been written.
func (p *Producer) PublishMessages(ctx context.Context, msgs []Message) error {
	var errs []error
	for _, group := range byTopic(msgs) {
		errs = append(errs, p.writeGroup(ctx, group))
	}
	return errors.Join(errs...)
}

// byTopic groups msgs per topic in first-seen order, preserving order within each group.
func byTopic(msgs []Message) [][]Message {
	index := map[string]int{}
	var groups [][]Message
	for _, m := range msgs {
		i, ok := index[m.Topic]
		if !ok {
			i = len(groups)
			index[m.Topic] = i
			groups = append(groups, nil)
		}
		groups[i] = append(groups[i], m)
	}
	return groups
}

func (p *Producer) writeGroup(ctx context.Context, group []Message) error {
	topic := group[0].Topic
	spans := make([]trace.Span, len(group))
	out := make([]kafka.Message, len(group))
	for i, m := range group {
		parent := m.Ctx
		if parent == nil {
			parent = ctx
		}
		spanCtx, span := tracer.Start(parent, "publish "+topic,
			trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.destination.name", topic),
			),
		)
		spans[i] = span
		out[i] = kafka.Message{Key: []byte(m.Key), Value: m.Value}
		if m.ID != "" {
			out[i].Headers = append(out[i].Headers, kafka.Header{Key: EventIDHeader, Value: []byte(m.ID)})
		}
		if m.ReplayOf != "" {
			out[i].Headers = append(out[i].Headers, kafka.Header{Key: ReplayOfHeader, Value: []byte(m.ReplayOf)})
		}
		otel.GetTextMapPropagator().Inject(spanCtx, headerCarrier{&out[i].Headers})
	}

	err := p.getWriter(topic).WriteMessages(ctx, out...)
	published.WithLabelValues(topic, result(err)).Add(float64(len(group)))
	for _, span := range spans {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
	if err != nil {
		return fmt.Errorf("publishing %d messages to %s: %w", len(group), topic, err)
	}
	return nil
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
