package cache

import "testing"

func TestKeys(t *testing.T) {
	if got := HomeFeedKey("u1"); got != "feed:u1" {
		t.Errorf("HomeFeedKey = %q", got)
	}
	if got := UserPostsKey("u1"); got != "userposts:u1" {
		t.Errorf("UserPostsKey = %q", got)
	}
}
