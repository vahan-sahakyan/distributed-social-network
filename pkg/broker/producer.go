package broker

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writers map[string]*kafka.Writer
	brokers []string
}

func NewProducer(brokers string) *Producer {
	return &Producer{
		writers: make(map[string]*kafka.Writer),
		brokers: strings.Split(brokers, ","),
	}
}

func (p *Producer) Publish(ctx context.Context, topic string, payload any) error {
	w := p.getWriter(topic)

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return w.WriteMessages(ctx, kafka.Message{
		Value: data,
	})
}

func (p *Producer) getWriter(topic string) *kafka.Writer {
	if w, ok := p.writers[topic]; ok {
		return w
	}

	w := &kafka.Writer{
		Addr:     kafka.TCP(p.brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	p.writers[topic] = w
	return w
}

func (p *Producer) Close() {
	for _, w := range p.writers {
		w.Close()
	}
}
