package cache

import "fmt"

// HomeFeedKey is the cache key for the posts by userID and everyone they follow.
// Every service that reads or writes the cache must go through these helpers:
// duplicating the format strings is how keys drifted apart before.
func HomeFeedKey(userID string) string {
	return fmt.Sprintf("feed:%s", userID)
}

// UserPostsKey is the cache key for the posts authored by userID.
func UserPostsKey(userID string) string {
	return fmt.Sprintf("userposts:%s", userID)
}

// PostKey is the cache key of a post and its counts, shared by every feed that shows it.
func PostKey(postID string) string {
	return fmt.Sprintf("post:%s", postID)
}
