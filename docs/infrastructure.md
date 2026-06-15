# Infrastructure

## Container Overview

The system runs **24 containers** via two Docker Compose files:

```
infrastructure/
├── docker-compose.yml           # 14 infra containers
└── docker-compose.services.yml  # 10 app service containers
```

## Infrastructure Containers

### Databases

| Container | Image | Internal Port | External Port | Purpose |
|-----------|-------|--------------|---------------|---------|
| posts-db | `scylladb/scylla:latest` | 9042 | 9042 | Posts storage (wide-column) |
| comments-db | `postgres:16-alpine` | 5432 | 5433 | Comments storage |
| likes-db | `postgres:16-alpine` | 5432 | 5434 | Likes storage |
| users-db | `postgres:16-alpine` | 5432 | 5436 | Users + follows storage |
| notifications-db | `postgres:16-alpine` | 5432 | 5437 | Notifications storage |

All PostgreSQL instances use:
- User: `postgres`
- Password: `postgres`
- Each has its own named database matching the service

### Event & Analytics

| Container | Image | Ports | Purpose |
|-----------|-------|-------|---------|
| redpanda | `redpandadata/redpanda:latest` | 19092 (kafka), 9644 (admin) | Event streaming (Kafka API) |
| redpanda-console | `redpandadata/console:latest` | 8888 | Topic browser UI |
| clickhouse | `clickhouse/clickhouse-server:latest` | 8123 (HTTP), 9009 (native) | Event store / analytics |

### Cache & Storage

| Container | Image | Ports | Purpose |
|-----------|-------|-------|---------|
| memcached | `memcached:1.6-alpine` | 11211 | Feed cache |
| minio | `minio/minio:latest` | 9000 (API), 9001 (console) | Object storage for media |

### Observability

| Container | Image | Port | Purpose |
|-----------|-------|------|---------|
| prometheus | `prom/prometheus:latest` | 9090 | Metrics collection |
| grafana | `grafana/grafana:latest` | 3000 | Dashboards & visualization |
| loki | `grafana/loki:latest` | 3100 | Log aggregation |
| jaeger | `jaegertracing/all-in-one:latest` | 16686, 4318 | Distributed tracing |

## Application Containers

All app services are built from multi-stage Dockerfiles (`Go build → scratch/alpine`) with:
- `restart: on-failure` — automatic restart if DB isn't ready yet
- Named network: `infrastructure_default` (all containers share one network)

| Container | Exposed Port | Depends On |
|-----------|-------------|------------|
| gateway-service | 8080 (mapped) | All other services |
| posts-service | 8081 (internal) | posts-db, redpanda |
| feed-service | 8082 (internal) | redpanda, memcached |
| comments-service | 8083 (internal) | comments-db, redpanda |
| likes-service | 8084 (internal) | likes-db, redpanda |
| users-service | 8085 (internal) | users-db |
| media-service | 8086 (internal) | minio, redpanda |
| notification-service | 8087 (internal) | notifications-db, redpanda |
| event-writer-service | 8088 (internal) | redpanda, clickhouse |
| cache-rebuilder-service | 8089 (internal) | clickhouse, memcached |

## Networking

All containers run on the `infrastructure_default` Docker bridge network. Services reference each other by container/service name (e.g., `posts-db:9042`, `redpanda:9092`).

Only the gateway (8080) and observability tools are exposed to the host.

## Volumes

Persistent named volumes for all stateful services:

```
posts-db-data, comments-db-data, likes-db-data, users-db-data,
notifications-db-data, clickhouse-data, redpanda-data,
minio-data, prometheus-data, grafana-data, loki-data
```

Use `make down-clean` to wipe all volumes.

## Database Schemas

Migrations are managed by `scripts/migrate.sh` and are idempotent (uses `CREATE IF NOT EXISTS`).

### PostgreSQL — users-db

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    bio TEXT,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE follows (
    follower_id TEXT NOT NULL,
    followee_id TEXT NOT NULL,
    PRIMARY KEY (follower_id, followee_id)
);
```

### PostgreSQL — comments-db

```sql
CREATE TABLE comments (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    text TEXT NOT NULL,
    likes INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL
);
```

### PostgreSQL — likes-db

```sql
CREATE TABLE likes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    UNIQUE (user_id, entity_id)
);
```

### PostgreSQL — notifications-db

```sql
CREATE TABLE notifications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL
);
```

### ScyllaDB — posts-db

```sql
CREATE KEYSPACE posts
    WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE posts.posts (
    id TEXT PRIMARY KEY,
    text TEXT,
    author_id TEXT,
    image_id TEXT,
    likes INT,
    comments INT,
    created_at TIMESTAMP
);
```

### ClickHouse — event store

```sql
CREATE TABLE feed_events (
    event_id UUID,
    event_type String,
    post_id String,
    user_id String,
    likes_delta Int32,
    comments_delta Int32,
    created_at DateTime
) ENGINE = MergeTree()
ORDER BY (post_id, created_at);
```

### Redpanda Topics

| Topic | Partitions | Purpose |
|-------|-----------|---------|
| `post.created` | 3 | New post events |
| `like.created` | 3 | New like events |
| `comment.created` | 3 | New comment events |

## Prometheus Configuration

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: "gateway-service"
    static_configs:
      - targets: ["gateway-service:8080"]
  - job_name: "posts-service"
    static_configs:
      - targets: ["posts-service:8081"]
  # ... (all 10 services)
```

Each service exposes `/metrics` via the `fiberprometheus` middleware.
