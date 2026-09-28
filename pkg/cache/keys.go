package cache

import "fmt"

// HomeFeedKey is the cache key for the posts by userID and everyone they follow.
// Every service that reads or writes a feed cache must go through these two
// helpers: duplicating the format strings is how the two keys drifted apart.
func HomeFeedKey(userID string) string {
	return fmt.Sprintf("feed:%s", userID)
}

// UserPostsKey is the cache key for the posts authored by userID.
func UserPostsKey(userID string) string {
	return fmt.Sprintf("userposts:%s", userID)
}
