#!/usr/bin/env bash
# no -e: workers keep going while a service is stopped for chaos testing
set -uo pipefail

# Steady mixed traffic for the dashboards: follows, posts, likes, comments,
# feed and notification reads, plus a few 404s. Each worker signs in as its own
# Keycloak user (created up front, ~2s each).
#
# Usage: ./scripts/load.sh [seconds] [workers]    (defaults: 120s, 4 workers)
#   or:  make load DURATION=300 WORKERS=8

DURATION="${1:-120}"
WORKERS="${2:-4}"
API="${GATEWAY_URL:-http://localhost:8080}/api/v1"
RUN="$(date +%s)"

source "$(dirname "$0")/lib/auth.sh"

if ! curl -sf "${API%/api/v1}/health" >/dev/null; then
  echo "gateway not reachable at ${API%/api/v1}, run 'make up' first" >&2
  exit 1
fi

id_of() { grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4; }

kc_login
echo "load: creating ${WORKERS} users"
declare -a NAMES IDS
for w in $(seq 1 "$WORKERS"); do
  NAMES[w]="load_${RUN}_${w}"
  kc_ensure_user "${NAMES[w]}" || exit 1
  token=$(kc_token "${NAMES[w]}") || exit 1
  IDS[w]=$(curl -s -X POST "$API/users/" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d '{"bio":"load"}' | id_of)
done

worker() {
  local w="$1" end=$((SECONDS + DURATION)) i=0 target token issued
  local other="${IDS[$(( w % WORKERS + 1 ))]}"
  token=$(kc_token "${NAMES[w]}"); issued=$SECONDS
  post() { curl -s -X POST "$API/$1" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$2"; }
  get() { curl -s "$API/$1" -H "Authorization: Bearer $token" >/dev/null; }

  post "users/$other/follow" '{}' >/dev/null
  while ((SECONDS < end)); do
    i=$((i + 1))
    # tokens live 15 minutes
    if ((SECONDS - issued > 600)); then token=$(kc_token "${NAMES[w]}"); issued=$SECONDS; fi

    post posts/ "{\"text\":\"load $w/$i #load\"}" >/dev/null
    # the followed user's latest post, so likes and comments notify someone else
    target=$(curl -s "$API/feed/user/$other" | grep -o '"post_id":"[^"]*"' | head -1 | cut -d'"' -f4)
    if [[ -n "$target" ]]; then
      post likes/ "{\"entity_id\":\"$target\"}" >/dev/null
      post comments/ "{\"entity_id\":\"$target\",\"text\":\"nice $i\"}" >/dev/null
      get "comments/entity/$target"
    fi

    get feed/home
    get notifications
    ((i % 5 == 0)) && get "users/missing-$i"
  done
  echo "worker $w: $i iterations"
}

echo "load: ${WORKERS} workers for ${DURATION}s against $API"
for w in $(seq 1 "$WORKERS"); do
  worker "$w" &
done
wait
