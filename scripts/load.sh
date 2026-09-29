#!/usr/bin/env bash
# no -e: workers keep going while a service is stopped for chaos testing
set -uo pipefail

# Steady mixed traffic for the dashboards: users, follows, posts, likes, comments,
# feed and notification reads, plus a few 404s.
#
# Usage: ./scripts/load.sh [seconds] [workers]    (defaults: 120s, 4 workers)
#   or:  make load DURATION=300 WORKERS=8

DURATION="${1:-120}"
WORKERS="${2:-4}"
API="${GATEWAY_URL:-http://localhost:8080}/api/v1"
RUN="$(date +%s)"

if ! curl -sf "${API%/api/v1}/health" >/dev/null; then
  echo "gateway not reachable at ${API%/api/v1}, run 'make up' first" >&2
  exit 1
fi

post() { curl -s -X POST "$API/$1" -H "Content-Type: application/json" -d "$2"; }
id_of() { grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4; }

worker() {
  local w="$1" end=$((SECONDS + DURATION)) i=0 prev_user="" prev_post="" user post_id
  while ((SECONDS < end)); do
    i=$((i + 1))
    user=$(post users/ "{\"username\":\"load_${RUN}_${w}_${i}\"}" | id_of)
    [[ -z "$user" ]] && { sleep 1; continue; }

    if [[ -n "$prev_user" ]]; then
      post "users/$prev_user/follow" "{\"follower_id\":\"$user\"}" >/dev/null
    fi
    post_id=$(post posts/ "{\"author_id\":\"$user\",\"text\":\"load $w/$i\"}" | id_of)
    if [[ -n "$prev_post" ]]; then
      post likes/ "{\"entity_id\":\"$prev_post\",\"user_id\":\"$user\"}" >/dev/null
      post comments/ "{\"entity_id\":\"$prev_post\",\"user_id\":\"$user\",\"text\":\"nice $i\"}" >/dev/null
    fi

    curl -s "$API/feed/home?user_id=$user" >/dev/null
    curl -s "$API/feed/user/$prev_user" >/dev/null
    curl -s "$API/notifications/$prev_user" >/dev/null
    curl -s "$API/comments/entity/$prev_post" >/dev/null
    ((i % 5 == 0)) && curl -s "$API/users/missing-$i" >/dev/null

    prev_user="$user"
    prev_post="$post_id"
  done
  echo "worker $w: $i iterations"
}

echo "load: ${WORKERS} workers for ${DURATION}s against $API"
for w in $(seq 1 "$WORKERS"); do
  worker "$w" &
done
wait
