package consumer

import (
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/events"
)

func TestNotificationIDIsStableAcrossRedelivery(t *testing.T) {
	msg := kafka.Message{Topic: events.LikeCreated, Partition: 1, Offset: 42}
	redelivered := msg
	next := msg
	next.Offset++
	other := msg
	other.Topic = events.CommentCreated

	if notificationID(msg) != notificationID(redelivered) {
		t.Fatal("redelivered message got a new id")
	}
	if notificationID(msg) == notificationID(next) || notificationID(msg) == notificationID(other) {
		t.Fatal("distinct messages must get distinct ids")
	}
}

func TestNotificationIDIsStableAcrossRepublish(t *testing.T) {
	header := []kafka.Header{{Key: broker.EventIDHeader, Value: []byte("likes-service:42")}}
	first := kafka.Message{Topic: events.LikeCreated, Partition: 0, Offset: 10, Headers: header}
	republished := first
	republished.Partition, republished.Offset = 2, 500
	other := first
	other.Headers = []kafka.Header{{Key: broker.EventIDHeader, Value: []byte("likes-service:43")}}

	if notificationID(first) != notificationID(republished) {
		t.Fatal("republished event got a new notification id")
	}
	if notificationID(first) == notificationID(other) {
		t.Fatal("distinct events must get distinct ids")
	}
}
