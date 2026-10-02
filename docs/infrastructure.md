# Infrastructure

[<- README](../README.md) · [Architecture](architecture.md) · [Services](services.md) · [API](api.md) · **Infrastructure** · [Development](development.md) · [Observability](observability.md) · [Search](search.md)

---

## Container Overview

Locally the system runs **30 containers** (plus optional Kibana) via two Docker Compose files:

```
infrastructure/
├── docker-compose.yml           # 19 infra containers (redpanda-init exits after setup), Kibana behind a profile
└── docker-compose.services.yml  # 11 app service containers
```

The UI is not in compose; run it with `npm run dev` (see [Development](development.md)). In Kubernetes, the Helm charts under `deploy/kubernetes/` run the same data stores and services plus the UI, without the observability containers.

## Infrastructure Containers

### Databases

| Container | Image | Internal Port | Host Port | Purpose |
|-----------|-------|--------------|---------------|---------|
| posts-db | `scylladb/scylla:2026.1.5` | 9042 | 9042 | Posts storage (wide-column) |
| comments-db | `postgres:16.14-alpine` | 5432 | 5433 | Comments storage |
| likes-db | `postgres:16.14-alpine` | 5432 | 5434 | Likes storage |
| users-db | `postgres:16.14-alpine` | 5432 | 5436 | Users + follows storage |
| notifications-db | `postgres:16.14-alpine` | 5432 | 5437 | Notifications storage |
| keycloak-db | `postgres:16.14-alpine` | 5432 | - | Keycloak's own store |

All PostgreSQL instances use:
- User: `postgres`
- Password: `postgres`
- Each has its own named database matching the service

ScyllaDB runs with `--smp 1 --memory 512M --overprovisioned 1`.

### Identity

| Container | Image | Host Ports | Purpose |
|-----------|-------|-------|---------|
| keycloak | `quay.io/keycloak/keycloak:26.7.5` | 8180 | OIDC provider, served under `/auth`; admin console admin/admin |

The realm (`deploy/kubernetes/infra/files/dsn-realm.json`, shared with Helm) is imported only into an empty `keycloak-db`; after editing it, recreate the volume (`docker compose -f infrastructure/docker-compose.yml rm -sf keycloak keycloak-db && docker volume rm infrastructure_keycloak-data`) or change the realm in the admin console. Tokens carry the issuer `http://localhost:8180/auth/realms/dsn` however Keycloak is reached (`KC_HOSTNAME`). In Helm, set `keycloak.uiUrl` (infra chart) to the UI's public origin, and `ingress.host` (services chart) so the gateway checks the issuer.

### Event & Analytics

| Container | Image | Host Ports | Purpose |
|-----------|-------|-------|---------|
| redpanda | `redpandadata/redpanda:v26.1.10` | 19092 (kafka, external listener), 9644 (admin) | Event streaming (Kafka API) |
| redpanda-console | `redpandadata/console:latest` | 8888 | Topic browser UI |
| redpanda-init | `redpandadata/redpanda:v26.1.10` | - | One-shot: enables per-group consumer lag metrics |
| clickhouse | `clickhouse/clickhouse-server:26.5.1.882` | 8123 (HTTP), 9009 -> 9000 (native) | Event store |

Inside the network, services reach Redpanda at `redpanda:9092` and ClickHouse at `clickhouse:9000`.

### Search

| Container | Image | Host Ports | Purpose |
|-----------|-------|-------|---------|
| elasticsearch | `elasticsearch:9.5.3` | 9200 | Search indices (single node, 1 GB heap, security off) |
| kibana | `kibana:9.5.3` | 5601 | Optional (`make kibana`, compose profile `kibana`): Dev Tools console for the indices |

See [Search](search.md).

### Cache & Storage

| Container | Image | Host Ports | Purpose |
|-----------|-------|-------|---------|
| memcached | `memcached:1.6.45-alpine` | 11211 | Feed cache |
| minio | `ghcr.io/vahan-sahakyan/distributed-social-network/minio:RELEASE.2025-09-07T16-13-09Z` | 9000 (API), 9001 (console) | Object storage for media |

MinIO's upstream images and binaries are no longer published, so the image is built from source (`deploy/images/minio`) by the publish workflow, once per release, for amd64 and arm64.

### Observability

| Container | Image | Host Port | Purpose |
|-----------|-------|------|---------|
| prometheus | `prom/prometheus:v3.12.0` | 9090 | Metrics, alert rules |
| grafana | `grafana/grafana:13.0.2` | 3000 | Provisioned datasources + DSN Overview dashboard |
| loki | `grafana/loki:3.7.2` | 3100 | Log storage |
| alloy | `grafana/alloy:v1.20.1` | 12345 | Ships container logs to Loki via the Docker socket |
| jaeger | `jaegertracing/jaeger:2.20.0` | 16686 (UI), 4317/4318 (OTLP) | Traces, in-memory |

See [Observability](observability.md).

## Application Containers

All app services are built from multi-stage Dockerfiles (`golang:1.27-alpine3.24` -> `alpine:3.24`). Each Dockerfile copies only the `pkg/` directories its service imports and downloads modules from `go.mod`/`go.sum` before copying sources, so code changes reuse the download layer; BuildKit cache mounts keep modules and the Go build cache across `go.mod` changes. The Dockerfiles are generated (`make dockerfiles`) and CI fails if they drift.

- services wait for their dependencies in-process (`pkg/retry`); `restart: on-failure` only covers real crashes
- All containers share the default compose network

| Container | HTTP Port | gRPC Port (host-mapped) | Depends On |
|-----------|-------------|-------------|------------|
| gateway-service | 8080 (host-mapped) | - | all services except event-writer |
| posts-service | 8081 | 9081 | posts-db, redpanda |
| feed-service | 8082 | 9082 | redpanda, memcached, users-service, posts-service |
| comments-service | 8083 | 9083 | comments-db, redpanda |
| likes-service | 8084 | 9084 | likes-db, redpanda |
| users-service | 8085 | 9085 | users-db |
| media-service | 8086 | 9086 | minio |
| notification-service | 8087 | 9087 | notifications-db, redpanda, posts-service |
| event-writer-service | 8088 | - | redpanda, clickhouse |
| cache-rebuilder-service | 8089 | 9089 | clickhouse, memcached, users-service, posts-service |
| search-service | 8091 | 9091 | elasticsearch, redpanda |

## Networking

Services reference each other by compose service name (e.g., `posts-db:9042`, `redpanda:9092`, `users-service:9085`).

Exposed to the host: the gateway (8080), the gRPC ports 9081-9089 and 9091 (for tools like grpcurl or Postman), every data store's port, and the observability tools. The services' HTTP ports (8081-8089, 8091) stay internal; Prometheus scrapes them on the compose network.

## Volumes

Persistent named volumes for all stateful services:

```
posts-db-data, comments-db-data, likes-db-data, users-db-data,
notifications-db-data, clickhouse-data, redpanda-data,
minio-data, prometheus-data, grafana-data, loki-data, elasticsearch-data,
keycloak-data
```

`make down-clean` (and `make fresh`) wipe all volumes.

## Database Schemas

Each service embeds its migrations (`services/*/migrations/`) and applies them on startup. They are idempotent (`IF NOT EXISTS`).

### PostgreSQL - users-db

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

CREATE INDEX idx_follows_followee ON follows (followee_id);
```

### PostgreSQL - comments-db

```sql
CREATE TABLE comments (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    text TEXT NOT NULL,
    likes INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_comments_entity_id ON comments (entity_id);
CREATE INDEX idx_comments_created_at ON comments (created_at DESC);
```

### PostgreSQL - likes-db

```sql
CREATE TABLE likes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    UNIQUE (user_id, entity_id)
);

CREATE INDEX idx_likes_entity_id ON likes (entity_id);
CREATE INDEX idx_likes_user_id ON likes (user_id);
```

### PostgreSQL - notifications-db

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

CREATE INDEX idx_notifications_user ON notifications (user_id, created_at DESC);
```

### ScyllaDB - posts-db

```sql
CREATE KEYSPACE posts
    WITH replication = {'class': 'NetworkTopologyStrategy', 'replication_factor': 1};

CREATE TABLE posts.posts (
    id TEXT PRIMARY KEY,
    text TEXT,
    author_id TEXT,
    image_id TEXT,
    created_at TIMESTAMP
);

CREATE INDEX idx_posts_author_id ON posts.posts (author_id);
```

### ClickHouse - event store

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

Created on startup by `pkg/broker.EnsureTopics`, replication factor 1.

| Topic | Partitions | Purpose |
|-------|-----------|---------|
| `post.created` | 3 | New post events |
| `like.created` | 3 | New like events |
| `like.deleted` | 3 | Unlike events |
| `comment.created` | 3 | New comment events |
| `<topic>.dlq` | 3 | Messages that failed 3 attempts; created when a consumer with a DLQ starts, nothing consumes them yet |

## Monitoring Configuration

| File | Purpose |
|------|---------|
| `monitoring/prometheus/prometheus.yml` | Scrapes the 11 services on their HTTP ports; reloads on edit (`--config.auto-reload`), Redpanda `/public_metrics`, Prometheus itself |
| `monitoring/prometheus/alerts.yml` | Alert rules |
| `monitoring/grafana/provisioning/` | Datasources (Prometheus, Loki, Jaeger, cross-linked) and dashboard provider |
| `monitoring/grafana/dashboards/dsn-overview.json` | DSN Overview dashboard, also Grafana's home |
| `monitoring/alloy/config.alloy` | Docker log discovery -> Loki, labelled `service` and `project` |
| `monitoring/alloy/kubernetes.alloy` | The k8s equivalent: pod logs of the namespace -> Loki, labelled `service`, `namespace` and `pod` |

The `deploy/kubernetes/observability` chart reads these through its `files/` symlink, so compose and the cluster share them.

Services export traces to `OTEL_EXPORTER_OTLP_ENDPOINT` (`http://jaeger:4318` in compose); unset, tracing is off.
