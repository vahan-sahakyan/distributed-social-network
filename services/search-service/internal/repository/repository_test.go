package repository

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBulkBody(t *testing.T) {
	buf, err := bulkBody("posts", []doc{
		{id: "a", source: map[string]string{"text": "one"}},
		{id: "b", source: map[string]string{"text": "two"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (action + source per doc): %q", len(lines), buf.String())
	}
	if lines[0] != `{"index":{"_id":"a","_index":"posts"}}` || lines[3] != `{"text":"two"}` {
		t.Errorf("unexpected body:\n%s", buf.String())
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Error("_bulk needs a trailing newline")
	}
}

func TestBulkFailures(t *testing.T) {
	var ok, partial bulkResponse
	_ = json.Unmarshal([]byte(`{"errors":false,"items":[{"index":{"_id":"a","status":201}}]}`), &ok)
	_ = json.Unmarshal([]byte(`{"errors":true,"items":[
		{"index":{"_id":"a","status":201}},
		{"index":{"_id":"b","status":400,"error":{"type":"strict_dynamic_mapping_exception","reason":"field [x] not allowed"}}}
	]}`), &partial)

	if err := ok.failures(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err := partial.failures()
	if err == nil || !strings.Contains(err.Error(), "1 of 2") || !strings.Contains(err.Error(), "b: strict_dynamic_mapping_exception") {
		t.Errorf("got %v", err)
	}
}

func TestPostsQuery(t *testing.T) {
	text := mustJSON(t, postsQuery(" scylla ", 5))
	tag := mustJSON(t, postsQuery("#GoLang", 5))

	if !strings.Contains(text, `"multi_match"`) || !strings.Contains(text, `"query":"scylla"`) || !strings.Contains(text, `"size":5`) {
		t.Errorf("text query: %s", text)
	}
	if !strings.Contains(tag, `"term":{"hashtags":"golang"}`) || strings.Contains(tag, "multi_match") {
		t.Errorf("hashtag query: %s", tag)
	}
	if !strings.Contains(text, `"gauss"`) {
		t.Errorf("want a recency decay: %s", text)
	}
}

func TestUsersQueryStripsAt(t *testing.T) {
	q := mustJSON(t, usersQuery("@Alice", 10))
	if !strings.Contains(q, `"value":"alice"`) || strings.Contains(q, "@") {
		t.Errorf("got %s", q)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
