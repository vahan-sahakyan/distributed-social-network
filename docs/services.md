# Services

[<- README](../README.md) · [Architecture](architecture.md) · **Services** · [API](api.md) · [Infrastructure](infrastructure.md) · [Development](development.md)

---

## Overview

| Service | HTTP port | gRPC port | Data store | Role |
|---------|-----------|-----------|------------|------|
| gateway-service | 8080 | - | - | Public HTTP API, calls services over gRPC |
| posts-service | 8081 | 9081 | ScyllaDB | Posts + event publishing |
| feed-service | 8082 | 9082 | Memcached | Home and user feeds (event consumer) |
| comments-service | 8083 | 9083 | PostgreSQL | Comments + event publishing |
| likes-service | 8084 | 9084 | PostgreSQL | Likes + event publishing |
| users-service | 8085 | 9085 | PostgreSQL | User profiles + follow graph |
| media-service | 8086 | 9086 | MinIO | File uploads |
| notification-service | 8087 | 9087 | PostgreSQL | Notifications (event consumer) |
| event-writer-service | 8088 | - | ClickHouse | Event store writer (event consumer) |
| cache-rebuilder-service | 8089 | 9089 | ClickHouse -> Memcached | Feed cache reconstruction |

Backend services serve their API over gRPC (`GRPC_PORT`, definitions in `proto/`). The HTTP port (`PORT`) serves only `/health` and `/metrics`, except on the gateway.

---

## gateway-service

**Role:** Single public entry point. Exposes the REST/JSON API (see [API](api.md)) and translates each request into a gRPC call.

**Stack:** Go Fiber + gRPC clients (generated from `proto/`, in `pkg/grpc/`)

**Routes:**
```
/api/v1/users/*          -> users-service (gRPC)
/api/v1/posts/*          -> posts-service (gRPC)
/api/v1/comments/*       -> comments-service (gRPC)
/api/v1/likes/*          -> likes-service (gRPC)
/api/v1/feed/*           -> feed-service (gRPC)
/api/v1/media/*          -> media-service (gRPC, uploads streamed in 32 KB chunks)
/api/v1/notifications/*  -> notification-service (gRPC)
/api/v1/rebuild          -> cache-rebuilder-service (gRPC)
/api/v1/reset            -> Reset on every service (dev)
/images/*                -> MinIO (HTTP proxy to the public-read bucket)
/health                  -> local ({"status":"ok"})
/metrics                 -> Prometheus metrics
```

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Listen port |
| `USERS_SERVICE_GRPC_ADDR` | localhost:9085 | users-service |
| `POSTS_SERVICE_GRPC_ADDR` | localhost:9081 | posts-service |
| `COMMENTS_SERVICE_GRPC_ADDR` | localhost:9083 | comments-service |
| `LIKES_SERVICE_GRPC_ADDR` | localhost:9084 | likes-service |
| `FEED_SERVICE_GRPC_ADDR` | localhost:9082 | feed-service |
| `MEDIA_SERVICE_GRPC_ADDR` | localhost:9086 | media-service |
| `NOTIFICATIONS_SERVICE_GRPC_ADDR` | localhost:9087 | notification-service |
| `CACHE_REBUILDER_SERVICE_GRPC_ADDR` | localhost:9089 | cache-rebuilder-service |
| `MINIO_ENDPOINT` | localhost:9000 | MinIO, for `/images/*` |
| `MINIO_BUCKET` | images | Bucket served under `/<bucket>/*` |

---

## posts-service

**Role:** Post creation and retrieval. Publishes `post.created`.

**Stack:** gRPC + ScyllaDB (gocql) + Redpanda producer

**Data model:**
```go
type Post struct {
    ID        string    // hex ID
    Text      string
    AuthorID  string
    ImageID   string    // optional, media-service id
    CreatedAt time.Time
}
```

Like and comment counts are not stored on the post; feed-service keeps them in the feed cache.

**Event published:** `post.created` -> Post JSON

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8081 | Health/metrics port |
| `GRPC_PORT` | 9081 | gRPC port |
| `SCYLLA_HOSTS` | localhost:9042 | ScyllaDB contact points |
| `SCYLLA_KEYSPACE` | posts | Keyspace name |
| `KAFKA_BROKERS` | - | Redpanda brokers |

---

## feed-service

**Role:** Serves home and user feeds from Memcached. Consumes events to keep them current.

**Stack:** gRPC + Memcached + Redpanda consumer

**Pattern:** Fanout-on-write. On `post.created` it fetches the author's followers from users-service and appends the post to `userposts:<author>` and to `feed:<id>` for each follower and the author. Like/comment events adjust the counts on that post in the same caches. Cache writes use memcache CAS so concurrent consumers do not overwrite each other.

**Consumer groups:** `feed-service-posts`, `feed-service-likes`, `feed-service-unlikes`, `feed-service-comments`
**Topics consumed:** `post.created`, `like.created`, `like.deleted`, `comment.created`

**Cache keys** (built only via `pkg/cache`): `feed:<user_id>` (home feed), `userposts:<user_id>` (user's own posts)

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8082 | Health/metrics port |
| `GRPC_PORT` | 9082 | gRPC port |
| `KAFKA_BROKERS` | - | Redpanda brokers |
| `MEMCACHED_ADDR` | localhost:11211 | Memcached address |
| `USERS_SERVICE_GRPC_ADDR` | localhost:9085 | users-service (followers) |
| `POSTS_SERVICE_GRPC_ADDR` | localhost:9081 | posts-service (post author) |

---

## comments-service

**Role:** Comments on posts. Publishes `comment.created`.

**Stack:** gRPC + PostgreSQL (pgx) + Redpanda producer

**Data model:**
```go
type Comment struct {
    ID        string
    UserID    string
    EntityID  string    // the post being commented on
    Text      string
    Likes     int
    CreatedAt time.Time
}
```

**Event published:** `comment.created` -> Comment JSON

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8083 | Health/metrics port |
| `GRPC_PORT` | 9083 | gRPC port |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `KAFKA_BROKERS` | - | Redpanda brokers |

---

## likes-service

**Role:** Likes and unlikes. A unique constraint on (user_id, entity_id) makes both idempotent; an event is published only when a row was actually inserted or deleted.

**Stack:** gRPC + PostgreSQL (pgx) + Redpanda producer

**Data model:**
```go
type Like struct {
    ID       string
    UserID   string
    EntityID string    // post being liked
}
```

**Events published:** `like.created`, `like.deleted` -> Like JSON (keyed by entity_id)

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8084 | Health/metrics port |
| `GRPC_PORT` | 9084 | gRPC port |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `KAFKA_BROKERS` | - | Redpanda brokers |

---

## users-service

**Role:** User profiles and follow relationships.

**Stack:** gRPC + PostgreSQL (pgx)

**Data model:**
```go
type User struct {
    ID        string
    Username  string
    Bio       string
    CreatedAt time.Time
}
```

**Follow graph** stored in a `follows(follower_id, followee_id)` join table.

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8085 | Health/metrics port |
| `GRPC_PORT` | 9085 | gRPC port |
| `DATABASE_URL` | - | PostgreSQL connection string |

---

## media-service

**Role:** Stores uploads in MinIO and returns a bucket path (`/images/<id>`), which the gateway serves. Accepts uploads up to 60 MB.

**Stack:** gRPC (client-streaming upload) + MinIO SDK

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8086 | Health/metrics port |
| `GRPC_PORT` | 9086 | gRPC port |
| `MINIO_ENDPOINT` | - | MinIO server |
| `MINIO_ACCESS_KEY` | - | MinIO access key |
| `MINIO_SECRET_KEY` | - | MinIO secret key |
| `MINIO_BUCKET` | - | Storage bucket (`images` in compose and Helm) |

---

## notification-service

**Role:** Consumes like/comment events and creates notification records for the post author.

**Stack:** gRPC + PostgreSQL (pgx) + Redpanda consumer

**Consumer group:** `notification-service`
**Topics consumed:** `like.created`, `comment.created` (recipient = post author, resolved via posts-service; self-actions skipped)

**Data model:**
```go
type Notification struct {
    ID        string
    UserID    string    // recipient
    Type      string    // "like" or "comment"
    ActorID   string    // who performed the action
    EntityID  string    // what was liked/commented
    Read      bool
    CreatedAt time.Time
}
```

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8087 | Health/metrics port |
| `GRPC_PORT` | 9087 | gRPC port |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `KAFKA_BROKERS` | - | Redpanda brokers |
| `POSTS_SERVICE_GRPC_ADDR` | localhost:9081 | posts-service (post author) |

---

## event-writer-service

**Role:** Persists all domain events to ClickHouse. Consumer only, no gRPC API.

**Stack:** ClickHouse + Redpanda consumer, Fiber for health/metrics

**Consumer group:** `event-writer-service`
**Topics consumed:** `post.created`, `like.created`, `like.deleted`, `comment.created`

**Writes to ClickHouse:**
```sql
INSERT INTO feed_events (event_id, event_type, post_id, user_id, likes_delta, comments_delta, created_at)
```

`event_id` is derived from topic/partition/offset (`pkg/id.Deterministic`), so a redelivered message produces a duplicate row readers can collapse.

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8088 | Health/metrics port |
| `KAFKA_BROKERS` | - | Redpanda brokers |
| `CLICKHOUSE_ADDR` | localhost:9000 | ClickHouse native port |
| `CLICKHOUSE_DB` | default | Database name |

---

## cache-rebuilder-service

**Role:** Rebuilds Memcached feed caches from the ClickHouse event store. Triggered on demand via `POST /api/v1/rebuild` (all feeds, or one user's with `user_id`).

**Stack:** gRPC + ClickHouse (read) + Memcached (write)

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8089 | Health/metrics port |
| `GRPC_PORT` | 9089 | gRPC port |
| `CLICKHOUSE_ADDR` | localhost:9000 | ClickHouse native port |
| `CLICKHOUSE_DB` | default | Database name |
| `MEMCACHED_ADDR` | localhost:11211 | Memcached address |
| `USERS_SERVICE_GRPC_ADDR` | localhost:9085 | users-service (followers, following) |
| `POSTS_SERVICE_GRPC_ADDR` | localhost:9081 | posts-service (post details) |

---

## Common Patterns

All backend services share:

1. **Graceful shutdown** via `signal.NotifyContext(SIGINT, SIGTERM)` (not the gateway)
2. **Prometheus metrics** at `/metrics` via `fiberprometheus`
3. **Health endpoint** at `/health` returning `{"status":"ok"}`
4. **Embedded migrations** applied on startup with `pkg/database.Migrate*` (services that own a database)
5. **Shared `pkg/` library** for broker, cache keys, database, events, gRPC stubs, and ID generation
