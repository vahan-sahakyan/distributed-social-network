# Services

[← README](../README.md) · [Architecture](architecture.md) · **Services** · [API](api.md) · [Infrastructure](infrastructure.md) · [Development](development.md)

---

## Overview

| Service | Port | Database | Role |
|---------|------|----------|------|
| gateway-service | 8080 | — | API gateway / reverse proxy |
| posts-service | 8081 | ScyllaDB | Post CRUD + event publishing |
| feed-service | 8082 | Memcached | Feed assembly from cache |
| comments-service | 8083 | PostgreSQL | Comment CRUD + event publishing |
| likes-service | 8084 | PostgreSQL | Like CRUD + event publishing |
| users-service | 8085 | PostgreSQL | User profiles + follow graph |
| media-service | 8086 | MinIO | File upload/retrieval |
| notification-service | 8087 | PostgreSQL | Notification storage (event consumer) |
| event-writer-service | 8088 | ClickHouse | Event sourcing (event consumer) |
| cache-rebuilder-service | 8089 | ClickHouse → Memcached | Feed cache reconstruction |

---

## gateway-service

**Role:** Single entry point for all API traffic. Routes requests to backend services via reverse proxy.

**Stack:** Go Fiber + `proxy.Forward`

**Routes:**
```
/api/v1/posts/*          → posts-service:8081
/api/v1/feed/*           → feed-service:8082
/api/v1/comments/*       → comments-service:8083
/api/v1/likes/*          → likes-service:8084
/api/v1/users/*          → users-service:8085
/api/v1/media/*          → media-service:8086
/api/v1/notifications/*  → notification-service:8087
/health                  → local ({"status":"ok"})
/metrics                 → Prometheus metrics
```

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Listen port |
| `POSTS_SERVICE_URL` | http://localhost:8081 | Posts backend |
| `FEED_SERVICE_URL` | http://localhost:8082 | Feed backend |
| `COMMENTS_SERVICE_URL` | http://localhost:8083 | Comments backend |
| `LIKES_SERVICE_URL` | http://localhost:8084 | Likes backend |
| `USERS_SERVICE_URL` | http://localhost:8085 | Users backend |
| `MEDIA_SERVICE_URL` | http://localhost:8086 | Media backend |
| `NOTIFICATIONS_SERVICE_URL` | http://localhost:8087 | Notifications backend |

---

## posts-service

**Role:** Manages post creation and retrieval. Publishes `post.created` events.

**Stack:** Go Fiber + ScyllaDB (gocql) + Redpanda producer

**Data model:**
```go
type Post struct {
    ID        string    // hex ID
    Text      string
    AuthorID  string
    ImageID   string    // optional, links to media-service
    Likes     int
    Comments  int
    CreatedAt time.Time
}
```

**Event published:** `post.created` → full Post JSON

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8081 | Listen port |
| `SCYLLA_HOSTS` | localhost:9042 | ScyllaDB contact points |
| `SCYLLA_KEYSPACE` | posts | Keyspace name |
| `KAFKA_BROKERS` | localhost:19092 | Redpanda brokers |

---

## feed-service

**Role:** Assembles user feeds from Memcached. Consumes `post.created` events to fan out posts to follower caches.

**Stack:** Go Fiber + Memcached + Redpanda consumer

**Pattern:** Fanout-on-write — when a post is created, the consumer writes it to each follower's cached feed.

**Consumer group:** `feed-service`  
**Topics consumed:** `post.created`

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8082 | Listen port |
| `KAFKA_BROKERS` | localhost:19092 | Redpanda brokers |
| `MEMCACHED_ADDR` | localhost:11211 | Memcached address |

---

## comments-service

**Role:** Comment CRUD. Publishes `comment.created` events.

**Stack:** Go Fiber + PostgreSQL (pgx) + Redpanda producer

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

**Event published:** `comment.created` → full Comment JSON

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8083 | Listen port |
| `DATABASE_URL` | — | PostgreSQL connection string |
| `KAFKA_BROKERS` | localhost:19092 | Redpanda brokers |

---

## likes-service

**Role:** Like creation (idempotent via unique constraint). Publishes `like.created` events.

**Stack:** Go Fiber + PostgreSQL (pgx) + Redpanda producer

**Data model:**
```go
type Like struct {
    ID       string
    UserID   string
    EntityID string    // post or comment being liked
}
```

**Event published:** `like.created` → full Like JSON

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8084 | Listen port |
| `DATABASE_URL` | — | PostgreSQL connection string |
| `KAFKA_BROKERS` | localhost:19092 | Redpanda brokers |

---

## users-service

**Role:** User profiles and follow relationships.

**Stack:** Go Fiber + PostgreSQL (pgx)

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
| `PORT` | 8085 | Listen port |
| `DATABASE_URL` | — | PostgreSQL connection string |

---

## media-service

**Role:** File uploads to MinIO object storage. Returns a URL for retrieval.

**Stack:** Go Fiber + MinIO SDK + Redpanda producer

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8086 | Listen port |
| `KAFKA_BROKERS` | localhost:19092 | Redpanda brokers |
| `MINIO_ENDPOINT` | localhost:9000 | MinIO server |
| `MINIO_ACCESS_KEY` | minioadmin | MinIO access key |
| `MINIO_SECRET_KEY` | minioadmin | MinIO secret key |
| `MINIO_BUCKET` | images | Storage bucket |

---

## notification-service

**Role:** Consumes like/comment events and creates notification records for the affected user.

**Stack:** Go Fiber + PostgreSQL (pgx) + Redpanda consumer

**Consumer group:** `notification-service`  
**Topics consumed:** `like.created`, `comment.created`

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
| `PORT` | 8087 | Listen port |
| `DATABASE_URL` | — | PostgreSQL connection string |
| `KAFKA_BROKERS` | localhost:19092 | Redpanda brokers |

---

## event-writer-service

**Role:** Persists all domain events to ClickHouse as an append-only event store. No HTTP API (consumer-only).

**Stack:** Go Fiber (health/metrics only) + ClickHouse + Redpanda consumer

**Consumer group:** `event-writer-service`  
**Topics consumed:** `post.created`, `like.created`, `comment.created`

**Writes to ClickHouse:**
```sql
INSERT INTO feed_events (event_id, event_type, post_id, user_id, likes_delta, comments_delta, created_at)
```

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8088 | Listen port |
| `KAFKA_BROKERS` | localhost:19092 | Redpanda brokers |
| `CLICKHOUSE_ADDR` | localhost:9000 | ClickHouse native port |
| `CLICKHOUSE_DB` | default | Database name |

---

## cache-rebuilder-service

**Role:** Rebuilds Memcached feed caches from ClickHouse event store. Triggered via API or scheduled.

**Stack:** Go Fiber + ClickHouse (read) + Memcached (write)

**Environment:**
| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8089 | Listen port |
| `CLICKHOUSE_ADDR` | localhost:9000 | ClickHouse native port |
| `CLICKHOUSE_DB` | default | Database name |
| `MEMCACHED_ADDR` | localhost:11211 | Memcached address |

---

## Common Patterns

All services share:

1. **Graceful shutdown** via `signal.NotifyContext(SIGINT, SIGTERM)`
2. **Prometheus metrics** at `/metrics` via `fiberprometheus`
3. **Health endpoint** at `/health` returning `{"status":"ok"}`
4. **Structured logging** via Fiber's logger middleware
5. **Shared `pkg/` library** for database, cache, broker, and ID generation
