# Distributed Social Network — Local System Design POC

## Project Goal
Build a fully local, production-style distributed social network backend demonstrating advanced distributed systems architecture.

Core principles:
- Microservices
- CQRS
- Event Driven Architecture
- Eventual Consistency
- Self-Healing Cache Layer
- Horizontal Scalability

Deployment:
- Docker Compose
- Kubernetes (k3d / Minikube)

---

## Infrastructure Stack

- API Gateway: Traefik / Nginx
- Messaging: Redpanda
- Posts DB: ScyllaDB
- Comments DB: PostgreSQL
- Likes DB: PostgreSQL
- Users DB: PostgreSQL
- Object Storage: MinIO
- Feed Cache: Memcached
- Event Store + Recovery: ClickHouse
- Monitoring: Prometheus + Grafana + Loki + Jaeger

---

## High Level Architecture

```mermaid
flowchart TB
    client[Client]
    gateway[API Gateway]
    client --> gateway

    gateway --> posts[posts-service]
    gateway --> users[users-service]
    gateway --> media[media-service]
    gateway --> likes[likes-service]
    gateway --> comments[comments-service]
    gateway --> feed[feed-service]

    posts --> scylla[(ScyllaDB)]
    users --> usersdb[(PostgreSQL)]
    media --> minio[(MinIO)]
    likes --> likesdb[(PostgreSQL)]
    comments --> commentsdb[(PostgreSQL)]

    posts --> redpanda[Redpanda]
    likes --> redpanda
    comments --> redpanda
    media --> redpanda

    redpanda --> feed
    redpanda --> writer[event-writer-service]

    writer --> clickhouse[(ClickHouse)]
    feed --> memcached[(Memcached)]

    clickhouse --> rebuilder[cache-rebuilder-service]
    rebuilder --> memcached
```

---

## Event Flow

```mermaid
sequenceDiagram
    actor User
    participant API as API Gateway
    participant Posts as posts-service
    participant DB as ScyllaDB
    participant Broker as Redpanda
    participant Feed as feed-service
    participant Cache as Memcached
    participant Writer as event-writer-service
    participant CH as ClickHouse

    User->>API: Create Post
    API->>Posts: POST /posts
    Posts->>DB: Store Post
    Posts->>Broker: publish post.created
    Broker->>Feed: consume event
    Feed->>Cache: update feed cache
    Broker->>Writer: consume event
    Writer->>CH: store event
```

---

## Cache Recovery

ClickHouse acts as both:
- Analytics store
- Event sourcing backbone for cache reconstruction

```mermaid
flowchart TD
    memcached[Memcached Failure]
    detector[cache-rebuilder-service]
    clickhouse[(ClickHouse Materialized State)]
    rebuild[Reconstruct Cache State]
    warm[Warm Hot Feed Cache]
    traffic[Resume Traffic]

    memcached --> detector
    detector --> clickhouse
    clickhouse --> rebuild
    rebuild --> warm
    warm --> traffic
```

---

## ClickHouse Schema

```sql
CREATE TABLE feed_events
(
    event_id UUID,
    event_type String,
    post_id String,
    user_id String,
    likes_delta Int32,
    comments_delta Int32,
    created_at DateTime
)
ENGINE = MergeTree()
ORDER BY (post_id, created_at);
```

Materialized view:

```sql
CREATE MATERIALIZED VIEW current_post_state
ENGINE = AggregatingMergeTree()
ORDER BY post_id
AS
SELECT
    post_id,
    countIf(event_type='like.created') as likes,
    countIf(event_type='comment.created') as comments,
    max(created_at) as last_update
FROM feed_events
GROUP BY post_id;
```

---

## Service List

- gateway-service
- posts-service
- comments-service
- likes-service
- feed-service
- users-service
- media-service
- notification-service
- event-writer-service
- cache-rebuilder-service

---

## Future Improvements

- Multi-node Scylla cluster
- ClickHouse replication
- Multi-node Memcached
- Recommendation engine
- Search service
- WebSocket notifications
- Chaos testing
