package consumer

import (
	"reflect"
	"testing"
)

func TestHashtags(t *testing.T) {
	tests := []struct {
		text string
		want []string
	}{
		{"no tags here", []string{}},
		{"#Go and #golang, #go again", []string{"go", "golang"}},
		{"unicode #Ünïcode_1 ok", []string{"ünïcode_1"}},
		{"trailing # and #", []string{}},
	}
	for _, tt := range tests {
		if got := hashtags(tt.text); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("hashtags(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestBuildPost(t *testing.T) {
	p, err := buildPost([]byte(`{"id":"p1","text":"hi #Go","author_id":"u1","created_at":"2026-01-02T03:04:05Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "p1" || p.AuthorID != "u1" || !reflect.DeepEqual(p.Hashtags, []string{"go"}) || p.CreatedAt.Year() != 2026 {
		t.Errorf("got %+v", p)
	}
	for _, bad := range []string{`{`, `{"text":"no id"}`} {
		if _, err := buildPost([]byte(bad)); err == nil {
			t.Errorf("%s: want an error so the message is retried and then parked", bad)
		}
	}
}

func TestBuildUser(t *testing.T) {
	u, err := buildUser([]byte(`{"id":"u1","username":"alice","bio":"hi"}`))
	if err != nil || u.ID != "u1" || u.Username != "alice" {
		t.Errorf("got %+v, %v", u, err)
	}
	if _, err := buildUser([]byte(`{"username":"x"}`)); err == nil {
		t.Error("want an error for a user event without id")
	}
}
