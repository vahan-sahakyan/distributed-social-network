package broker

import (
	"strconv"

	"github.com/segmentio/kafka-go"
)

// EventIDHeader carries the outbox row an event came from, so a republish of the
// same row is recognizable at a new offset.
const EventIDHeader = "event-id"

// DedupeKey returns the parts that identify msg's event for deduplication: its
// event-id header, or, for messages published without one, its topic, partition
// and offset (which only catches redelivery of the same message).
func DedupeKey(msg kafka.Message) []string {
	for _, h := range msg.Headers {
		if h.Key == EventIDHeader && len(h.Value) > 0 {
			return []string{EventIDHeader, string(h.Value)}
		}
	}
	return []string{msg.Topic, strconv.Itoa(msg.Partition), strconv.FormatInt(msg.Offset, 10)}
}
