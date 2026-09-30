package consumer

import (
	"testing"

	"github.com/segmentio/kafka-go"
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
