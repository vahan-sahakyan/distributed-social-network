package broker

import (
	"reflect"
	"testing"

	"github.com/segmentio/kafka-go"
)

func TestByTopicKeepsOrderWithinTopic(t *testing.T) {
	msgs := []Message{
		{Topic: "a", Key: "1"}, {Topic: "b", Key: "2"}, {Topic: "a", Key: "3"}, {Topic: "b", Key: "4"}, {Topic: "a", Key: "5"},
	}
	var got [][]string
	for _, g := range byTopic(msgs) {
		var keys []string
		for _, m := range g {
			keys = append(keys, m.Topic+m.Key)
		}
		got = append(got, keys)
	}
	want := [][]string{{"a1", "a3", "a5"}, {"b2", "b4"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWriterWaitsForBrokerAcks(t *testing.T) {
	p := NewProducer("localhost:9092")
	defer p.Close()
	if got := p.getWriter("t").RequiredAcks; got != kafka.RequireAll {
		t.Fatalf("RequiredAcks = %v, want RequireAll", got)
	}
}
