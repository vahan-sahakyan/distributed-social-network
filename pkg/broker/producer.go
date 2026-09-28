package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
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
	w := p.getWriter(topic)

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: data,
	})
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
		log.Printf("waiting for kafka to create topics %v: %v", topics, err)
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
