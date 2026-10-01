package consumer

import (
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
)

func TestBuildEvent(t *testing.T) {
	tests := []struct {
		topic        string
		payload      string
		wantPost     string
		wantUser     string
		wantLikes    int32
		wantComments int32
	}{
		{events.PostCreated, `{"id":"p1","author_id":"u1"}`, "p1", "u1", 0, 0},
		{events.LikeCreated, `{"entity_id":"p1","user_id":"u2"}`, "p1", "u2", 1, 0},
		{events.LikeDeleted, `{"entity_id":"p1","user_id":"u2"}`, "p1", "u2", -1, 0},
		{events.CommentCreated, `{"entity_id":"p1","user_id":"u3"}`, "p1", "u3", 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			msg := kafka.Message{Topic: tt.topic, Partition: 1, Offset: 7, Value: []byte(tt.payload)}
			ev, err := buildEvent(tt.topic, msg)
			if err != nil {
				t.Fatal(err)
			}
			if ev.PostID != tt.wantPost || ev.UserID != tt.wantUser {
				t.Errorf("post/user = %q/%q, want %q/%q", ev.PostID, ev.UserID, tt.wantPost, tt.wantUser)
			}
			if ev.LikesDelta != tt.wantLikes || ev.CommentsDelta != tt.wantComments {
				t.Errorf("deltas = %d/%d, want %d/%d", ev.LikesDelta, ev.CommentsDelta, tt.wantLikes, tt.wantComments)
			}
		})
	}
}

func TestBuildEventIDIsStableAcrossRedelivery(t *testing.T) {
	msg := kafka.Message{Topic: events.LikeCreated, Partition: 2, Offset: 99, Value: []byte(`{"entity_id":"p1"}`)}

	first, err := buildEvent(msg.Topic, msg)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := buildEvent(msg.Topic, msg)
	if first.EventID != again.EventID {
		t.Fatalf("redelivered message got a new event_id: %s vs %s", first.EventID, again.EventID)
	}

	msg.Offset++
	next, _ := buildEvent(msg.Topic, msg)
	if next.EventID == first.EventID {
		t.Fatal("distinct messages must get distinct event_ids")
	}
}

func TestBuildEventRejectsMalformedPayload(t *testing.T) {
	if _, err := buildEvent(events.LikeCreated, kafka.Message{Value: []byte("{")}); err == nil {
		t.Fatal("want an error so the message is retried and then parked")
	}
}

func TestBuildEventIDIsStableAcrossRepublish(t *testing.T) {
	header := []kafka.Header{{Key: broker.EventIDHeader, Value: []byte("likes-service:42")}}
	first := kafka.Message{Topic: events.LikeCreated, Partition: 0, Offset: 10, Headers: header, Value: []byte(`{"entity_id":"p1"}`)}
	republished := first
	republished.Partition, republished.Offset = 2, 500

	a, err := buildEvent(first.Topic, first)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := buildEvent(republished.Topic, republished)
	if a.EventID != b.EventID {
		t.Fatalf("republished event got a new event_id: %s vs %s", a.EventID, b.EventID)
	}
}
