#!/usr/bin/env bash
set -euo pipefail

# End-to-end demo of the Distributed Social Network.
# Exercises: users, follows, posts, likes, comments, media, feed, notifications,
# event streaming (Redpanda), event store (ClickHouse), and observability (Prometheus).
#
# Usage: ./scripts/demo.sh             (CLUSTER=1 for the local k3d cluster)
#
# Prerequisites:
#   make up          # start all containers (services apply migrations on startup)
#   or make cluster-up, then make demo CLUSTER=1

source "$(dirname "$0")/lib/env.sh"
BASE_URL="$GATEWAY_URL"
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

source "$(dirname "$0")/lib/auth.sh"

# post_json <url> <json> <token>: POST as the token's user, exit on an error response
post_json() {
  local response
  response=$(curl -s -X POST "$1" -H "Content-Type: application/json" -H "Authorization: Bearer $3" -d "$2")
  if echo "$response" | grep -q '"error"'; then
    echo "ERROR: $response" >&2
    exit 1
  fi
  echo "$response"
}

# get_json <url> [token]
get_json() {
  if [[ -n "${2:-}" ]]; then
    curl -s "$1" -H "Authorization: Bearer $2"
  else
    curl -s "$1"
  fi
}

# follow <target id> <token>
follow() {
  curl -sf -X POST "$API/users/$1/follow" -H "Authorization: Bearer $2" -o /dev/null -w "  HTTP %{http_code}\n"
}

# sign_up <username> <bio>: ensures the Keycloak account, prints its token, and
# creates the profile on first run (a rerun finds it via /me)
sign_up() {
  kc_ensure_user "$1" >&2
  local token code
  token=$(kc_token "$1")
  code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/users/" \
    -H "Content-Type: application/json" -H "Authorization: Bearer $token" -d "{\"bio\":\"$2\"}")
  if [[ "$code" != 201 && "$code" != 409 ]]; then
    echo "ERROR: creating profile for $1: HTTP $code" >&2
    exit 1
  fi
  echo "$token"
}

extract() {
  python3 -c "import json,sys; print(json.load(sys.stdin)['$1'])"
}

pretty() {
  python3 -m json.tool
}

# ─────────────────────────────────────────────────────────────────────────────
section "1. SIGN UP (Keycloak accounts + profiles)"
# ─────────────────────────────────────────────────────────────────────────────

kc_login
info "Accounts use password '$DEMO_PASSWORD'; log in to the UI as any of them."

step "Alice (software engineer)..."
ALICE=$(sign_up alice "Software engineer & open source enthusiast")
ALICE_ID=$(get_json "$API/me" "$ALICE" | extract id)
get_json "$API/me" "$ALICE" | pretty

step "Bob (DevOps wizard)..."
BOB=$(sign_up bob "DevOps wizard, coffee addict")
BOB_ID=$(get_json "$API/me" "$BOB" | extract id)
get_json "$API/me" "$BOB" | pretty

step "Charlie (full-stack dev)..."
CHARLIE=$(sign_up charlie "Full-stack developer & writer")
CHARLIE_ID=$(get_json "$API/me" "$CHARLIE" | extract id)
get_json "$API/me" "$CHARLIE" | pretty

step "Writes need a token: posting without one..."
curl -s -X POST "$API/posts/" -H "Content-Type: application/json" -d '{"text":"anonymous"}' -w "  HTTP %{http_code}\n"

# ─────────────────────────────────────────────────────────────────────────────
section "2. FOLLOW RELATIONSHIPS"
# ─────────────────────────────────────────────────────────────────────────────

step "Bob follows Alice..."
follow "$ALICE_ID" "$BOB"

step "Charlie follows Alice..."
follow "$ALICE_ID" "$CHARLIE"

step "Alice follows Bob..."
follow "$BOB_ID" "$ALICE"

step "Verifying: Alice's followers"
get_json "$API/users/$ALICE_ID/followers" | pretty

step "Verifying: Alice's following"
get_json "$API/users/$ALICE_ID/following" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "3. CREATE POSTS"
# ─────────────────────────────────────────────────────────────────────────────

step "Alice posts about microservices..."
POST1_RAW=$(post_json "$API/posts/" '{"text":"Just deployed our new microservices architecture! 11 services running with full observability. #distributed #golang"}' "$ALICE")
POST1_ID=$(echo "$POST1_RAW" | extract id)
echo "$POST1_RAW" | pretty

step "Alice posts a pro tip..."
post_json "$API/posts/" "{\"text\":\"Pro tip: Always add Prometheus metrics to your services from day one. You'll thank yourself later.\"}" "$ALICE" | pretty

step "Bob posts about Docker..."
POST3_RAW=$(post_json "$API/posts/" '{"text":"Docker Compose + Go services = chefs kiss. Our local dev environment spins up 30 containers in seconds."}' "$BOB")
POST3_ID=$(echo "$POST3_RAW" | extract id)
echo "$POST3_RAW" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "4. LIKES"
# ─────────────────────────────────────────────────────────────────────────────

step "Bob likes Alice's microservices post..."
post_json "$API/likes/" "{\"entity_id\":\"$POST1_ID\"}" "$BOB" | pretty

step "Charlie likes Alice's microservices post..."
post_json "$API/likes/" "{\"entity_id\":\"$POST1_ID\"}" "$CHARLIE" | pretty

step "Alice likes Bob's Docker post..."
post_json "$API/likes/" "{\"entity_id\":\"$POST3_ID\"}" "$ALICE" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "5. COMMENTS"
# ─────────────────────────────────────────────────────────────────────────────

step "Bob comments on Alice's post..."
post_json "$API/comments/" "{\"entity_id\":\"$POST1_ID\",\"text\":\"This is incredible! How long did the migration take?\"}" "$BOB" | pretty

step "Charlie comments on Alice's post..."
post_json "$API/comments/" "{\"entity_id\":\"$POST1_ID\",\"text\":\"Love the architecture! Would you recommend ScyllaDB for the posts store?\"}" "$CHARLIE" | pretty

step "Alice replies on Bob's post..."
post_json "$API/comments/" "{\"entity_id\":\"$POST3_ID\",\"text\":\"Thanks! Go fast compile times make iteration a breeze.\"}" "$ALICE" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "6. MEDIA UPLOAD"
# ─────────────────────────────────────────────────────────────────────────────

step "Uploading a test file to MinIO via media-service..."
echo "Hello from the Distributed Social Network! 🌐" > /tmp/dsn-demo-upload.txt
curl -sf -X POST "$API/media/upload" -H "Authorization: Bearer $ALICE" -F "file=@/tmp/dsn-demo-upload.txt" | pretty
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
get_json "$API/feed/home" "$BOB" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "9. NOTIFICATIONS"
# ─────────────────────────────────────────────────────────────────────────────

step "Alice's notifications (from likes/comments events)..."
get_json "$API/notifications" "$ALICE" | pretty

# ─────────────────────────────────────────────────────────────────────────────
section "10. EVENT STREAMING (Redpanda → ClickHouse)"
# ─────────────────────────────────────────────────────────────────────────────

# optional groups (make up OBS=1 TOOLS=1 SEARCH=1 EVENTS=1): sections of switched-off ones are skipped
on_events=0; in_infra clickhouse true >/dev/null 2>&1 && on_events=1
on_search=0; curl -s "$API/search/hashtags/trending" | grep -q 'search is disabled' || on_search=1
on_obs=0;    curl -sf -m3 -o /dev/null "$PROMETHEUS_URL/-/ready" && on_obs=1
on_tools=0;  curl -sf -m3 -o /dev/null "$REDPANDA_CONSOLE_URL" && on_tools=1
off() { info "$1 is off; rerun with $2=1 (make up $2=1, or make cluster-profile $2=1)"; }

step "Redpanda topics:"
in_infra redpanda rpk topic list 2>/dev/null

if ((on_events)); then
  step "ClickHouse event store stats:"
  echo -n "  Total events: "
  in_infra clickhouse clickhouse-client --query "SELECT count(*) FROM feed_events"
  echo "  Events by type:"
  in_infra clickhouse clickhouse-client --query "SELECT event_type, count(*) as cnt FROM feed_events GROUP BY event_type ORDER BY cnt DESC"
else
  off "The event store" EVENTS
fi

# ─────────────────────────────────────────────────────────────────────────────
section "11. SEARCH (Redpanda -> Elasticsearch)"
# ─────────────────────────────────────────────────────────────────────────────

search() {
  curl -s "$API/search/$1" | python3 -c "
import json, sys
d = json.load(sys.stdin)
for h in d.get('posts') or []:
    print('  ', h.get('highlight') or h['text'])
for h in d.get('users') or []:
    print('   @' + h['username'], '-', h.get('bio', ''))
for h in d.get('hashtags') or []:
    print('   #' + h['hashtag'], h['posts'])"
}

if ((on_search)); then
  info "Indexing is asynchronous; giving it a moment..."
  sleep 3
  step "Posts matching \"deploying\" (stemmed to deployed):"
  search "posts?q=deploying"
  step "Posts tagged #golang:"
  search "posts?q=%23golang"
  step "Users starting with \"ch\":"
  search "users?q=ch"
  step "Trending hashtags, last 24h:"
  search "hashtags/trending"
else
  off "Search" SEARCH
fi

# ─────────────────────────────────────────────────────────────────────────────
section "12. OBSERVABILITY"
# ─────────────────────────────────────────────────────────────────────────────

if ((on_obs)); then
step "Prometheus targets:"
curl -s "$PROMETHEUS_URL/api/v1/targets" | python3 -c "
import json, sys
data = json.load(sys.stdin)
for t in sorted(data['data']['activeTargets'], key=lambda x: x['labels']['job']):
    print(f\"  {t['labels']['job']:30s} {t['health']}\")"

step "Waiting one scrape interval for the demo traffic to land..."
sleep 16

step "gRPC calls handled by service:"
curl -s "$PROMETHEUS_URL/api/v1/query" --data-urlencode 'query=sum by (job)(grpc_server_handled_total) > 0' | python3 -c "
import json, sys
data = json.load(sys.stdin)
for r in sorted(data.get('data',{}).get('result',[]), key=lambda x: -float(x['value'][1])):
    print(f\"  {r['metric'].get('job','?'):30s} {r['value'][1]} calls\")" 2>/dev/null || echo "  (no metrics yet)"

step "Traces: the create-post request, through Kafka to every consumer, is one trace in Jaeger"
info "$JAEGER_URL/search?service=gateway-service"
else
  off "Observability" OBS
fi

# ─────────────────────────────────────────────────────────────────────────────
section "13. SYSTEM OVERVIEW"
# ─────────────────────────────────────────────────────────────────────────────

# pads by characters, not bytes: printf would miscount the ×
line() { printf '│  %s%*s│\n' "$1" $((75 - ${#1})) ''; }
row()  { line "$(printf '%-18s %s' "$1" "$2")"; }
echo
echo "┌─────────────────────────────────────────────────────────────────────────────┐"
line "DISTRIBUTED SOCIAL NETWORK"
echo "├─────────────────────────────────────────────────────────────────────────────┤"
line ""
row "Gateway:"          "$BASE_URL"
row "Keycloak:"         "$KEYCLOAK_URL  (admin/admin)"
row ""                  "demo users' password: \"password\""
row "MinIO Console:"    "$MINIO_CONSOLE_URL  (minioadmin/minioadmin)"
if ((on_obs)); then
  row "Prometheus:"     "$PROMETHEUS_URL"
  row "Grafana:"        "$GRAFANA_URL  (admin/admin)"
  row "Jaeger:"         "$JAEGER_URL"
fi
((on_tools)) && row "Redpanda Console:" "$REDPANDA_CONSOLE_URL"
((on_search)) && [[ "${CLUSTER:-}" != 1 ]] && row "Elasticsearch:" "http://localhost:9200"
off_groups="$( ((on_obs)) || printf 'OBS '; ((on_tools)) || printf 'TOOLS '; ((on_search)) || printf 'SEARCH '; ((on_events)) || printf 'EVENTS ')"
[[ -n "$off_groups" ]] && { line ""; line "Off (opt in with <GROUP>=1): $off_groups"; }
line ""
line "Services: gateway, posts, feed, comments, likes, users, media,"
line "          notifications, event-writer, cache-rebuilder, search"
line ""
line "Infra: ScyllaDB, PostgreSQL×4, ClickHouse, Redpanda, Memcached, MinIO,"
line "       Elasticsearch,"
line "       Prometheus, Grafana, Loki, Alloy, Jaeger"
line ""
echo "└─────────────────────────────────────────────────────────────────────────────┘"
echo

echo -e "${GREEN}✓ Demo complete!${NC}"
