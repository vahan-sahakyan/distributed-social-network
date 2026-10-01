package broker

import (
	"reflect"
	"testing"

	"github.com/segmentio/kafka-go"
)

func TestDedupeKey(t *testing.T) {
	withID := func(offset int64) kafka.Message {
		return kafka.Message{Topic: "like.created", Partition: 1, Offset: offset,
			Headers: []kafka.Header{{Key: "traceparent", Value: []byte("x")}, {Key: EventIDHeader, Value: []byte("likes-service:42")}}}
	}
	if !reflect.DeepEqual(DedupeKey(withID(7)), DedupeKey(withID(9))) {
		t.Error("a republish of the same event at another offset must get the same key")
	}
	plain := kafka.Message{Topic: "like.created", Partition: 1, Offset: 7}
	if got := DedupeKey(plain); !reflect.DeepEqual(got, []string{"like.created", "1", "7"}) {
		t.Errorf("without the header: %v, want topic/partition/offset", got)
	}
}
