#!/usr/bin/env bash
set -euo pipefail

# Migrate all databases for the distributed social network.
# Usage: ./scripts/migrate.sh
#
# Requires: docker compose containers running (at least infra)

COMPOSE="docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "==> Waiting for databases to be ready..."

wait_for_pg() {
  local container=$1
  local db=$2
  for i in $(seq 1 30); do
    if docker exec "$container" pg_isready -U postgres -d "$db" &>/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "ERROR: $container did not become ready in 30s"
  return 1
}

wait_for_scylla() {
  local container=$1
  for i in $(seq 1 60); do
    if docker exec "$container" cqlsh -e "DESCRIBE KEYSPACES" &>/dev/null; then
      return 0
    fi
    sleep 2
  done
  echo "ERROR: $container (ScyllaDB) did not become ready in 120s"
  return 1
}

wait_for_clickhouse() {
  local container=$1
  for i in $(seq 1 30); do
    if docker exec "$container" clickhouse-client --query "SELECT 1" &>/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "ERROR: $container (ClickHouse) did not become ready in 30s"
  return 1
}

# --- PostgreSQL databases ---
echo "  Waiting for users-db..."
wait_for_pg infrastructure-users-db-1 users

echo "  Waiting for comments-db..."
wait_for_pg infrastructure-comments-db-1 comments

echo "  Waiting for likes-db..."
wait_for_pg infrastructure-likes-db-1 likes

echo "  Waiting for notifications-db..."
wait_for_pg infrastructure-notifications-db-1 notifications

# --- ScyllaDB ---
echo "  Waiting for posts-db (ScyllaDB)..."
wait_for_scylla infrastructure-posts-db-1

# --- ClickHouse ---
echo "  Waiting for clickhouse..."
wait_for_clickhouse infrastructure-clickhouse-1

echo ""
echo "==> Running migrations..."

# Users
echo "  [users-db] Creating tables..."
docker exec -i infrastructure-users-db-1 psql -U postgres -d users <<'SQL'
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    bio TEXT,
    created_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS follows (
    follower_id TEXT NOT NULL,
    followee_id TEXT NOT NULL,
    PRIMARY KEY (follower_id, followee_id)
);
CREATE INDEX IF NOT EXISTS idx_follows_followee ON follows (followee_id);
SQL

# Comments
echo "  [comments-db] Creating tables..."
docker exec -i infrastructure-comments-db-1 psql -U postgres -d comments <<'SQL'
CREATE TABLE IF NOT EXISTS comments (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    text TEXT NOT NULL,
    likes INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comments_entity_id ON comments (entity_id);
CREATE INDEX IF NOT EXISTS idx_comments_created_at ON comments (created_at DESC);
SQL

# Likes
echo "  [likes-db] Creating tables..."
docker exec -i infrastructure-likes-db-1 psql -U postgres -d likes <<'SQL'
CREATE TABLE IF NOT EXISTS likes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    UNIQUE (user_id, entity_id)
);
CREATE INDEX IF NOT EXISTS idx_likes_entity_id ON likes (entity_id);
CREATE INDEX IF NOT EXISTS idx_likes_user_id ON likes (user_id);
SQL

# Notifications
echo "  [notifications-db] Creating tables..."
docker exec -i infrastructure-notifications-db-1 psql -U postgres -d notifications <<'SQL'
CREATE TABLE IF NOT EXISTS notifications (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications (user_id, created_at DESC);
SQL

# Posts (ScyllaDB)
echo "  [posts-db] Creating keyspace and table..."
docker exec infrastructure-posts-db-1 cqlsh -e "
CREATE KEYSPACE IF NOT EXISTS posts
    WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};"

docker exec infrastructure-posts-db-1 cqlsh -e "
CREATE TABLE IF NOT EXISTS posts.posts (
    id TEXT PRIMARY KEY,
    text TEXT,
    author_id TEXT,
    image_id TEXT,
    likes INT,
    comments INT,
    created_at TIMESTAMP
);"

docker exec infrastructure-posts-db-1 cqlsh -e "
CREATE INDEX IF NOT EXISTS idx_posts_author_id ON posts.posts (author_id);" 2>/dev/null || true

# ClickHouse (event store)
echo "  [clickhouse] Creating tables..."
docker exec -i infrastructure-clickhouse-1 clickhouse-client --multiquery <<'SQL'
CREATE TABLE IF NOT EXISTS feed_events
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

CREATE MATERIALIZED VIEW IF NOT EXISTS current_post_state
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
SQL

# Redpanda topics
echo "  [redpanda] Creating topics..."
docker exec infrastructure-redpanda-1 rpk topic create post.created like.created comment.created -p 3 2>/dev/null || true

echo ""
echo "==> All migrations complete!"
