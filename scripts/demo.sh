#!/usr/bin/env bash
set -euo pipefail

# End-to-end demo of the Distributed Social Network.
# Exercises: users, follows, posts, likes, comments, media, feed, notifications,
# event streaming (Redpanda), event store (ClickHouse), and observability (Prometheus).
#
# Usage: ./scripts/demo.sh
#
# Prerequisites:
#   make up          # start all containers
#   make migrate     # run database migrations

BASE_URL="${GATEWAY_URL:-http://localhost:8080}"
API="$BASE_URL/api/v1"

# Colors
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

section() { echo -e "\n${CYAN}━━━ $1 ━━━${NC}"; }
step()    { echo -e "${GREEN}▸ $1${NC}"; }
info()    { echo -e "${YELLOW}  $1${NC}"; }

# Wait for gateway to be healthy
echo -e "${YELLOW}Waiting for gateway to be ready...${NC}"
for i in $(seq 1 60); do
  if curl -sf "$BASE_URL/health" >/dev/null 2>&1; then
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: Gateway not ready after 60s"
    exit 1
  fi
  sleep 1
done

# Wait for backend services via gateway
echo -e "${YELLOW}Waiting for backend services...${NC}"
for i in $(seq 1 30); do
  # Try a GET that should return 200 even with no data
  if curl -sf "$API/users/nonexistent" 2>/dev/null | grep -q "error\|not"; then
    break
  fi
  # Also try the health endpoint of users-service through gateway
  if curl -sf "$API/feed/user/test" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
echo -e "${GREEN}All services ready!${NC}"

# Helper to POST JSON and extract field
post_json() {
  local response
  response=$(curl -s -X POST "$1" -H "Content-Type: application/json" -d "$2")
  if echo "$response" | grep -q '"error"'; then
    echo "ERROR: $response" >&2
    exit 1
  fi
  echo "$response"
}

get_json() {
  curl -s "$1"
}

extract() {
  python3 -c "import json,sys; print(json.load(sys.stdin)['$1'])"
}

pretty() {
  python3 -m json.tool
}

# ─────────────────────────────────────────────────────────────────────────────
section "1. CREATE USERS"
# ─────────────────────────────────────────────────────────────────────────────

step "Creating Alice (software engineer)..."
ALICE_RAW=$(post_json "$API/users/" '{"username":"alice","display_name":"Alice Johnson","bio":"Software engineer & open source enthusiast"}')
ALICE_ID=$(echo "$ALICE_RAW" | extract id)
echo "$ALICE_RAW" | pretty
info "Alice ID: $ALICE_ID"

step "Creating Bob (DevOps wizard)..."
BOB_RAW=$(post_json "$API/users/" '{"username":"bob","display_name":"Bob Smith","bio":"DevOps wizard, coffee addict"}')
BOB_ID=$(echo "$BOB_RAW" | extract id)
echo "$BOB_RAW" | pretty
info "Bob ID: $BOB_ID"

step "Creating Charlie (full-stack dev)..."
CHARLIE_RAW=$(post_json "$API/users/" '{"username":"charlie","display_name":"Charlie Davis","bio":"Full-stack developer & writer"}')
CHARLIE_ID=$(echo "$CHARLIE_RAW" | extract id)
echo "$CHARLIE_RAW" | pretty
info "Charlie ID: $CHARLIE_ID"

# ─────────────────────────────────────────────────────────────────────────────
section "2. FOLLOW RELATIONSHIPS"
# ─────────────────────────────────────────────────────────────────────────────

step "Bob follows Alice..."
curl -sf -X POST "$API/users/$ALICE_ID/follow" \
  -H "Content-Type: application/json" \
  -d "{\"follower_id\":\"$BOB_ID\"}" -o /dev/null -w "  HTTP %{http_code}\n"

step "Charlie follows Alice..."
curl -sf -X POST "$API/users/$ALICE_ID/follow" \
  -H "Content-Type: application/json" \
  -d "{\"follower_id\":\"$CHARLIE_ID\"}" -o /dev/null -w "  HTTP %{http_code}\n"

step "Alice follows Bob..."
curl -sf -X POST "$API/users/$BOB_ID/follow" \
  -H "Content-Type: application/json" \
  -d "{\"follower_id\":\"$ALICE_ID\"}" -o /dev/null -w "  HTTP %{http_code}\n"

step "Verifying: Alice's followers"
get_json "$API/users/$ALICE_ID/followers" | pretty

step "Verifying: Alice's following"
get_json "$API/users/$ALICE_ID/following" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "3. CREATE POSTS"
# ─────────────────────────────────────────────────────────────────────────────

step "Alice posts about microservices..."
POST1_RAW=$(post_json "$API/posts/" "{\"author_id\":\"$ALICE_ID\",\"text\":\"Just deployed our new microservices architecture! 10 services running with full observability. #distributed #golang\"}")
POST1_ID=$(echo "$POST1_RAW" | extract id)
echo "$POST1_RAW" | pretty

step "Alice posts a pro tip..."
POST2_RAW=$(post_json "$API/posts/" "{\"author_id\":\"$ALICE_ID\",\"text\":\"Pro tip: Always add Prometheus metrics to your services from day one. You'll thank yourself later.\"}")
POST2_ID=$(echo "$POST2_RAW" | extract id)
echo "$POST2_RAW" | pretty

step "Bob posts about Docker..."
POST3_RAW=$(post_json "$API/posts/" "{\"author_id\":\"$BOB_ID\",\"text\":\"Docker Compose + Go services = chefs kiss. Our local dev environment spins up 24 containers in seconds.\"}")
POST3_ID=$(echo "$POST3_RAW" | extract id)
echo "$POST3_RAW" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "4. LIKES"
# ─────────────────────────────────────────────────────────────────────────────

step "Bob likes Alice's microservices post..."
post_json "$API/likes/" "{\"entity_id\":\"$POST1_ID\",\"user_id\":\"$BOB_ID\"}" | pretty

step "Charlie likes Alice's microservices post..."
post_json "$API/likes/" "{\"entity_id\":\"$POST1_ID\",\"user_id\":\"$CHARLIE_ID\"}" | pretty

step "Alice likes Bob's Docker post..."
post_json "$API/likes/" "{\"entity_id\":\"$POST3_ID\",\"user_id\":\"$ALICE_ID\"}" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "5. COMMENTS"
# ─────────────────────────────────────────────────────────────────────────────

step "Bob comments on Alice's post..."
post_json "$API/comments/" "{\"entity_id\":\"$POST1_ID\",\"user_id\":\"$BOB_ID\",\"text\":\"This is incredible! How long did the migration take?\"}" | pretty

step "Charlie comments on Alice's post..."
post_json "$API/comments/" "{\"entity_id\":\"$POST1_ID\",\"user_id\":\"$CHARLIE_ID\",\"text\":\"Love the architecture! Would you recommend ScyllaDB for the posts store?\"}" | pretty

step "Alice replies on Bob's post..."
post_json "$API/comments/" "{\"entity_id\":\"$POST3_ID\",\"user_id\":\"$ALICE_ID\",\"text\":\"Thanks! Go fast compile times make iteration a breeze.\"}" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "6. MEDIA UPLOAD"
# ─────────────────────────────────────────────────────────────────────────────

step "Uploading a test file to MinIO via media-service..."
echo "Hello from the Distributed Social Network! 🌐" > /tmp/dsn-demo-upload.txt
curl -sf -X POST "$API/media/upload" -F "file=@/tmp/dsn-demo-upload.txt" | pretty
rm -f /tmp/dsn-demo-upload.txt

# ─────────────────────────────────────────────────────────────────────────────
section "7. READ OPERATIONS"
# ─────────────────────────────────────────────────────────────────────────────

step "Get Alice's post by ID..."
get_json "$API/posts/$POST1_ID" | pretty

step "Get comments on Alice's post..."
get_json "$API/comments/entity/$POST1_ID" | pretty

step "Get Bob's user profile..."
get_json "$API/users/$BOB_ID" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "8. FEED SERVICE"
# ─────────────────────────────────────────────────────────────────────────────

step "Bob's home feed (Memcached-backed, populated via event fanout)..."
get_json "$API/feed/user/$BOB_ID" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "9. NOTIFICATIONS"
# ─────────────────────────────────────────────────────────────────────────────

step "Alice's notifications (from likes/comments events)..."
get_json "$API/notifications/$ALICE_ID" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "10. EVENT STREAMING (Redpanda → ClickHouse)"
# ─────────────────────────────────────────────────────────────────────────────

step "Redpanda topics:"
docker exec infrastructure-redpanda-1 rpk topic list 2>/dev/null

step "ClickHouse event store stats:"
echo -n "  Total events: "
docker exec infrastructure-clickhouse-1 clickhouse-client --query "SELECT count(*) FROM feed_events"
echo "  Events by type:"
docker exec infrastructure-clickhouse-1 clickhouse-client --query "SELECT event_type, count(*) as cnt FROM feed_events GROUP BY event_type ORDER BY cnt DESC"

# ─────────────────────────────────────────────────────────────────────────────
section "11. OBSERVABILITY"
# ─────────────────────────────────────────────────────────────────────────────

step "Prometheus targets:"
curl -s http://localhost:9090/api/v1/targets | python3 -c "
import json, sys
data = json.load(sys.stdin)
for t in sorted(data['data']['activeTargets'], key=lambda x: x['labels']['job']):
    print(f\"  {t['labels']['job']:30s} {t['health']}\")"

step "HTTP request counts by service:"
curl -s http://localhost:9090/api/v1/query --data-urlencode 'query=sum by (job)(http_requests_total)' | python3 -c "
import json, sys
data = json.load(sys.stdin)
for r in sorted(data.get('data',{}).get('result',[]), key=lambda x: -float(x['value'][1])):
    print(f\"  {r['metric'].get('job','?'):30s} {r['value'][1]} requests\")" 2>/dev/null || echo "  (no request metrics yet)"

# ─────────────────────────────────────────────────────────────────────────────
section "12. SYSTEM OVERVIEW"
# ─────────────────────────────────────────────────────────────────────────────

echo "
┌─────────────────────────────────────────────────────────────────────────────┐
│  DISTRIBUTED SOCIAL NETWORK                                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Gateway:           $BASE_URL                                    │
│  Prometheus:        http://localhost:9090                                    │
│  Grafana:           http://localhost:3000  (admin/admin)                     │
│  Jaeger:            http://localhost:16686                                   │
│  Redpanda Console:  http://localhost:8888                                    │
│  MinIO Console:     http://localhost:9001  (minioadmin/minioadmin)           │
│                                                                             │
│  Services: gateway, posts, feed, comments, likes, users, media,             │
│            notifications, event-writer, cache-rebuilder                      │
│                                                                             │
│  Infra: ScyllaDB, PostgreSQL×4, ClickHouse, Redpanda, Memcached, MinIO,     │
│         Prometheus, Grafana, Loki, Jaeger                                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
"

echo -e "${GREEN}✓ Demo complete!${NC}"
