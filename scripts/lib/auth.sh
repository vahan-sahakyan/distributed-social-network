#!/usr/bin/env bash
# Keycloak helpers for local scripts: create users and get their access tokens.
# Users are created through the admin REST API (admin/admin, local only), so the
# same calls work against compose and the cluster; tokens come from the dsn-cli
# password grant.

KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8180/auth}"
KEYCLOAK_REALM="${KEYCLOAK_REALM:-dsn}"
DEMO_PASSWORD="${DEMO_PASSWORD:-password}"

_access_token() { python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('access_token') or ''); sys.exit(0 if d.get('access_token') else 1)"; }

# admin tokens live 60s, so every admin call takes a fresh one
kc_login() {
  KC_ADMIN_TOKEN=$(curl -s -X POST "$KEYCLOAK_URL/realms/master/protocol/openid-connect/token" \
    -d grant_type=password -d client_id=admin-cli \
    -d username="${KEYCLOAK_ADMIN:-admin}" -d password="${KEYCLOAK_ADMIN_PASSWORD:-admin}" | _access_token)
}

_kc_admin() { curl -sf -H "Authorization: Bearer $KC_ADMIN_TOKEN" -H "Content-Type: application/json" "$@"; }

_kc_user_id() {
  _kc_admin "$KEYCLOAK_URL/admin/realms/$KEYCLOAK_REALM/users?username=$1&exact=true" |
    python3 -c "import json,sys; u=json.load(sys.stdin); print(u[0]['id'] if u else '')"
}

# kc_ensure_user <username>: creates the user with DEMO_PASSWORD unless it exists
kc_ensure_user() {
  local users="$KEYCLOAK_URL/admin/realms/$KEYCLOAK_REALM/users" id
  kc_login || return 1
  id=$(_kc_user_id "$1")
  if [[ -z "$id" ]]; then
    _kc_admin -X POST "$users" -d "{\"username\":\"$1\",\"enabled\":true}" >/dev/null || return 1
    id=$(_kc_user_id "$1")
  fi
  _kc_admin -X PUT "$users/$id/reset-password" \
    -d "{\"type\":\"password\",\"value\":\"$DEMO_PASSWORD\",\"temporary\":false}" >/dev/null
}

# kc_token <username>: access token for the user, via the dsn-cli password grant
kc_token() {
  curl -s -X POST "$KEYCLOAK_URL/realms/$KEYCLOAK_REALM/protocol/openid-connect/token" \
    -d grant_type=password -d client_id=dsn-cli \
    -d username="$1" -d password="$DEMO_PASSWORD" | _access_token
}
