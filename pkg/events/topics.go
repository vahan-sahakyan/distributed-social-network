// Package events names the Kafka topics the services exchange domain events on.
// Keeping them here means a renamed topic is a compile error rather than a
// producer and a consumer silently disagreeing at runtime.
package events

const (
	PostCreated    = "post.created"
	LikeCreated    = "like.created"
	LikeDeleted    = "like.deleted"
	CommentCreated = "comment.created"
)

// All is every topic the platform uses, for services that consume the full stream.
var All = []string{PostCreated, LikeCreated, LikeDeleted, CommentCreated}

// DLQ is the dead-letter topic for messages a consumer could not process.
func DLQ(topic string) string {
	return topic + ".dlq"
}
