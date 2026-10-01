package broker

import (
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	// EventIDHeader carries the outbox row an event came from, so a republish of
	// the same row is recognizable at a new offset.
	EventIDHeader = "event-id"
	// ReplayOfHeader marks a replay of an event published without an event id:
	// "<topic>/<partition>/<offset>" of the original message.
	ReplayOfHeader = "replay-of"
)

// DeadLetter is what a consumer parks on <topic>.dlq after the last failed attempt.
type DeadLetter struct {
	Topic     string    `json:"topic"`
	Partition int       `json:"partition"`
	Offset    int64     `json:"offset"`
	Key       string    `json:"key"`
	Payload   string    `json:"payload"`
	Error     string    `json:"error"`
	FailedAt  time.Time `json:"failed_at"`
	Group     string    `json:"group,omitempty"`
	EventID   string    `json:"event_id,omitempty"`
	ReplayOf  string    `json:"replay_of,omitempty"`
}

// Origin is the original message's coordinates, "<topic>/<partition>/<offset>".
func (d DeadLetter) Origin() string {
	if d.ReplayOf != "" {
		return d.ReplayOf
	}
	return d.Topic + "/" + strconv.Itoa(d.Partition) + "/" + strconv.FormatInt(d.Offset, 10)
}

// DedupeKey returns the parts that identify msg's event for deduplication: its
// event-id header; for a replay of a message without one, the original's topic,
// partition and offset; otherwise its own (which only catches redelivery).
func DedupeKey(msg kafka.Message) []string {
	if id := header(msg, EventIDHeader); id != "" {
		return []string{EventIDHeader, id}
	}
	if origin := header(msg, ReplayOfHeader); origin != "" {
		if parts := strings.Split(origin, "/"); len(parts) == 3 {
			return parts
		}
	}
	return []string{msg.Topic, strconv.Itoa(msg.Partition), strconv.FormatInt(msg.Offset, 10)}
}

func header(msg kafka.Message, key string) string {
	for _, h := range msg.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
