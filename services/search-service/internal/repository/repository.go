package repository

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esapi"
	"github.com/vahan-sahakyan/distributed-social-network/search-service/internal/model"
)

// Reads and writes go through these aliases, never the versioned index names, so a
// mapping change is a new indices/<alias>-vN.json, a reindex and an alias swap.
const (
	postsAlias = "posts"
	usersAlias = "users"
)

//go:embed indices/*.json
var indices embed.FS

type Repository struct {
	es *elasticsearch.Client
}

func New(es *elasticsearch.Client) *Repository {
	return &Repository{es: es}
}

// EnsureIndices creates each versioned index with its alias, retrying until
// Elasticsearch answers or ctx is done. Existing indices are left untouched.
func (r *Repository) EnsureIndices(ctx context.Context) error {
	files, err := fs.Glob(indices, "indices/*.json")
	if err != nil {
		return err
	}
	for _, f := range files {
		name := strings.TrimSuffix(path.Base(f), ".json")
		body, err := indices.ReadFile(f)
		if err != nil {
			return err
		}
		for {
			err := r.createIndex(ctx, name, body)
			if err == nil {
				break
			}
			slog.WarnContext(ctx, "waiting for elasticsearch", "index", name, "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	return nil
}

func (r *Repository) createIndex(ctx context.Context, name string, body []byte) error {
	res, err := r.es.Indices.Create(name,
		r.es.Indices.Create.WithContext(ctx),
		r.es.Indices.Create.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if err := responseError(res); err != nil {
		var esErr *Error
		if errors.As(err, &esErr) && esErr.Type == "resource_already_exists_exception" {
			return nil
		}
		return fmt.Errorf("creating index %s: %w", name, err)
	}
	slog.InfoContext(ctx, "created index", "index", name)
	return nil
}

// IndexPosts upserts posts by id, so a redelivered event rewrites the same document.
func (r *Repository) IndexPosts(ctx context.Context, posts []model.Post) error {
	docs := make([]doc, len(posts))
	for i, p := range posts {
		docs[i] = doc{id: p.ID, source: p}
	}
	return r.bulk(ctx, postsAlias, docs)
}

// IndexUsers upserts users by id.
func (r *Repository) IndexUsers(ctx context.Context, users []model.User) error {
	docs := make([]doc, len(users))
	for i, u := range users {
		docs[i] = doc{id: u.ID, source: u}
	}
	return r.bulk(ctx, usersAlias, docs)
}

func (r *Repository) SearchPosts(ctx context.Context, query string, limit int) ([]model.PostHit, int64, error) {
	var resp searchResponse[model.Post]
	if err := r.search(ctx, postsAlias, postsQuery(query, limit), &resp); err != nil {
		return nil, 0, err
	}
	hits := make([]model.PostHit, len(resp.Hits.Hits))
	for i, h := range resp.Hits.Hits {
		h.Source.ID = h.ID
		hits[i] = model.PostHit{Post: h.Source, Score: h.Score}
		if fragments := h.Highlight["text"]; len(fragments) > 0 {
			hits[i].Highlight = fragments[0]
		}
	}
	return hits, resp.Hits.Total.Value, nil
}

func (r *Repository) SearchUsers(ctx context.Context, query string, limit int) ([]model.UserHit, int64, error) {
	var resp searchResponse[model.User]
	if err := r.search(ctx, usersAlias, usersQuery(query, limit), &resp); err != nil {
		return nil, 0, err
	}
	hits := make([]model.UserHit, len(resp.Hits.Hits))
	for i, h := range resp.Hits.Hits {
		h.Source.ID = h.ID
		hits[i] = model.UserHit{User: h.Source, Score: h.Score}
	}
	return hits, resp.Hits.Total.Value, nil
}

func (r *Repository) TrendingHashtags(ctx context.Context, hours, limit int) ([]model.HashtagCount, error) {
	var resp struct {
		Aggregations struct {
			Hashtags struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"hashtags"`
		} `json:"aggregations"`
	}
	if err := r.search(ctx, postsAlias, trendingQuery(hours, limit), &resp); err != nil {
		return nil, err
	}
	counts := make([]model.HashtagCount, len(resp.Aggregations.Hashtags.Buckets))
	for i, b := range resp.Aggregations.Hashtags.Buckets {
		counts[i] = model.HashtagCount{Hashtag: b.Key, Posts: b.DocCount}
	}
	return counts, nil
}

// Reset deletes every document, keeping the indices and their mappings.
func (r *Repository) Reset(ctx context.Context) error {
	res, err := r.es.DeleteByQuery([]string{postsAlias, usersAlias},
		strings.NewReader(`{"query":{"match_all":{}}}`),
		r.es.DeleteByQuery.WithContext(ctx),
		r.es.DeleteByQuery.WithConflicts("proceed"),
		r.es.DeleteByQuery.WithRefresh(true),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return responseError(res)
}

type searchResponse[T any] struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID        string              `json:"_id"`
			Score     float64             `json:"_score"`
			Source    T                   `json:"_source"`
			Highlight map[string][]string `json:"highlight"`
		} `json:"hits"`
	} `json:"hits"`
}

func (r *Repository) search(ctx context.Context, index string, query map[string]any, out any) error {
	body, err := json.Marshal(query)
	if err != nil {
		return err
	}
	res, err := r.es.Search(
		r.es.Search.WithContext(ctx),
		r.es.Search.WithIndex(index),
		r.es.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err := responseError(res); err != nil {
		return fmt.Errorf("searching %s: %w", index, err)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

type doc struct {
	id     string
	source any
}

// bulkBody is the NDJSON _bulk payload: an action line, then the document, per doc.
func bulkBody(index string, docs []doc) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, d := range docs {
		action := map[string]map[string]string{"index": {"_index": index, "_id": d.id}}
		if err := enc.Encode(action); err != nil {
			return nil, err
		}
		if err := enc.Encode(d.source); err != nil {
			return nil, err
		}
	}
	return &buf, nil
}

type bulkResponse struct {
	Errors bool `json:"errors"`
	Items  []map[string]struct {
		ID     string `json:"_id"`
		Status int    `json:"status"`
		Error  *struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	} `json:"items"`
}

// failures summarizes the rejected items; _bulk answers 200 even when some fail.
func (b bulkResponse) failures() error {
	if !b.Errors {
		return nil
	}
	var reasons []string
	for _, item := range b.Items {
		for _, result := range item {
			if result.Error != nil {
				reasons = append(reasons, fmt.Sprintf("%s: %s %s", result.ID, result.Error.Type, result.Error.Reason))
			}
		}
	}
	shown := reasons
	if len(shown) > 3 {
		shown = shown[:3]
	}
	return fmt.Errorf("%d of %d documents rejected: %s", len(reasons), len(b.Items), strings.Join(shown, "; "))
}

func (r *Repository) bulk(ctx context.Context, index string, docs []doc) error {
	if len(docs) == 0 {
		return nil
	}
	body, err := bulkBody(index, docs)
	if err != nil {
		return err
	}
	res, err := r.es.Bulk(body, r.es.Bulk.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err := responseError(res); err != nil {
		return fmt.Errorf("bulk indexing into %s: %w", index, err)
	}

	var resp bulkResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return err
	}
	return resp.failures()
}

// Error is an Elasticsearch error response.
type Error struct {
	Status int
	Type   string
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("elasticsearch %d %s: %s", e.Status, e.Type, e.Reason)
}

func responseError(res *esapi.Response) error {
	if !res.IsError() {
		return nil
	}
	var body struct {
		Error struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil || body.Error.Type == "" {
		return &Error{Status: res.StatusCode, Type: "unknown", Reason: res.Status()}
	}
	return &Error{Status: res.StatusCode, Type: body.Error.Type, Reason: body.Error.Reason}
}
