package repository

import (
	"fmt"
	"strings"
)

// postsQuery matches text (stemmed, typo tolerant, exact words weighted higher) or,
// for "#tag", the hashtag exactly. Relevance is multiplied by a recency decay: a
// week-old post scores half of an otherwise equal new one.
func postsQuery(query string, limit int) map[string]any {
	query = strings.TrimSpace(query)
	var match map[string]any
	if tag, ok := strings.CutPrefix(query, "#"); ok && tag != "" {
		match = map[string]any{"term": map[string]any{"hashtags": strings.ToLower(tag)}}
	} else {
		match = map[string]any{"multi_match": map[string]any{
			"query":     query,
			"fields":    []string{"text", "text.exact^2"},
			"fuzziness": "AUTO",
		}}
	}

	return map[string]any{
		"size":             limit,
		"track_total_hits": true,
		"query": map[string]any{"function_score": map[string]any{
			"query": match,
			"functions": []any{map[string]any{"gauss": map[string]any{
				"created_at": map[string]any{"origin": "now", "scale": "7d", "decay": 0.5},
			}}},
			"boost_mode": "multiply",
		}},
		"highlight": map[string]any{
			"fields": map[string]any{"text": map[string]any{"number_of_fragments": 0}},
		},
	}
}

// usersQuery ranks an exact username first, then username prefixes, then bio words.
func usersQuery(query string, limit int) map[string]any {
	query = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(query), "@"))
	return map[string]any{
		"size":             limit,
		"track_total_hits": true,
		"query": map[string]any{"bool": map[string]any{
			"should": []any{
				map[string]any{"term": map[string]any{"username.exact": map[string]any{"value": strings.ToLower(query), "boost": 10}}},
				map[string]any{"match": map[string]any{"username": map[string]any{"query": query, "boost": 3}}},
				map[string]any{"match": map[string]any{"bio": map[string]any{"query": query, "fuzziness": "AUTO"}}},
			},
			"minimum_should_match": 1,
		}},
	}
}

// trendingQuery counts hashtags on posts created in the last hours.
func trendingQuery(hours, limit int) map[string]any {
	return map[string]any{
		"size": 0,
		"query": map[string]any{"range": map[string]any{
			"created_at": map[string]any{"gte": fmt.Sprintf("now-%dh", hours)},
		}},
		"aggs": map[string]any{"hashtags": map[string]any{
			"terms": map[string]any{"field": "hashtags", "size": limit},
		}},
	}
}
