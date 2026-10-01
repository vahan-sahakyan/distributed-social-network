package main

import (
	"testing"

	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
)

func TestPlanReplaysEachEventOnce(t *testing.T) {
	letter := func(group, eventID string, offset int64) broker.DeadLetter {
		return broker.DeadLetter{Topic: "like.created", Partition: 1, Offset: offset, Key: "p1", Payload: `{"x":1}`, Group: group, EventID: eventID}
	}
	letters := []parked{
		{Letter: letter("feed-service-likes", "likes-service:9", 7), Pending: true},
		{Letter: letter("notification-service", "likes-service:9", 7), Pending: true},
		{Letter: letter("feed-service-likes", "", 8), Pending: true},
		{Letter: letter("notification-service", "", 8), Pending: true},
		{Letter: letter("feed-service-likes", "likes-service:10", 9), Pending: false},
	}

	msgs := plan(letters)
	if len(msgs) != 2 {
		t.Fatalf("planned %d replays, want 2 (one per pending event)", len(msgs))
	}
	if msgs[0].ID != "likes-service:9" || msgs[0].ReplayOf != "" {
		t.Errorf("event with an id: %+v, want the id and no replay-of", msgs[0])
	}
	if msgs[1].ID != "" || msgs[1].ReplayOf != "like.created/1/8" {
		t.Errorf("event without an id: %+v, want replay-of like.created/1/8", msgs[1])
	}
	if msgs[0].Topic != "like.created" || msgs[0].Key != "p1" || string(msgs[0].Value) != `{"x":1}` {
		t.Errorf("replay must go to the source topic with the original key and payload: %+v", msgs[0])
	}
}
