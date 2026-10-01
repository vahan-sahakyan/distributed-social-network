package main

import (
	"encoding/json"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
)

// parked is one message on a DLQ topic.
type parked struct {
	DLQTopic  string
	Partition int
	Offset    int64
	Letter    broker.DeadLetter
	Pending   bool
}

func parse(msg kafka.Message, committed int64) (parked, error) {
	var d broker.DeadLetter
	err := json.Unmarshal(msg.Value, &d)
	return parked{DLQTopic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset, Letter: d, Pending: msg.Offset >= committed}, err
}

// identity names the original event: one replay per event, however many groups parked it.
func (p parked) identity() string {
	if p.Letter.EventID != "" {
		return "event-id:" + p.Letter.EventID
	}
	return "origin:" + p.Letter.Origin()
}

// plan turns pending letters into the messages to republish on their source topics.
func plan(letters []parked) []broker.Message {
	seen := map[string]bool{}
	var msgs []broker.Message
	for _, p := range letters {
		if !p.Pending || seen[p.identity()] {
			continue
		}
		seen[p.identity()] = true
		m := broker.Message{Topic: p.Letter.Topic, Key: p.Letter.Key, Value: []byte(p.Letter.Payload), ID: p.Letter.EventID}
		if m.ID == "" {
			// lets consumers that saw the original recognize the replay
			m.ReplayOf = p.Letter.Origin()
		}
		msgs = append(msgs, m)
	}
	return msgs
}
