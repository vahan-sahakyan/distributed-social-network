package broker

import (
	"testing"

	"github.com/segmentio/kafka-go"
)

func TestWriterWaitsForBrokerAcks(t *testing.T) {
	p := NewProducer("localhost:9092")
	defer p.Close()
	if got := p.getWriter("t").RequiredAcks; got != kafka.RequireAll {
		t.Fatalf("RequiredAcks = %v, want RequireAll", got)
	}
}
