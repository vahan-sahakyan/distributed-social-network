#!/usr/bin/env bash
# Keycloak helpers for local scripts: create users and get their access tokens.
# Users are created with kcadm inside the container (admin/admin, local only);
# tokens come from the dsn-cli password grant.

KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8180/auth}"
KEYCLOAK_REALM="${KEYCLOAK_REALM:-dsn}"
KEYCLOAK_CONTAINER="${KEYCLOAK_CONTAINER:-infrastructure-keycloak-1}"
DEMO_PASSWORD="${DEMO_PASSWORD:-password}"

_kcadm() { docker exec "$KEYCLOAK_CONTAINER" /opt/keycloak/bin/kcadm.sh "$@" --config /tmp/kcadm.config; }

kc_login() {
  _kcadm config credentials --server http://localhost:8080/auth --realm master \
    --user "${KEYCLOAK_ADMIN:-admin}" --password "${KEYCLOAK_ADMIN_PASSWORD:-admin}" >/dev/null 2>&1
}

# kc_ensure_user <username>: creates the user with DEMO_PASSWORD unless it exists; needs kc_login first
kc_ensure_user() {
  if [[ -z "$(_kcadm get users -r "$KEYCLOAK_REALM" -q "username=$1" -q exact=true --fields id --format csv --noquotes 2>/dev/null)" ]]; then
    _kcadm create users -r "$KEYCLOAK_REALM" -s "username=$1" -s enabled=true >/dev/null 2>&1 || return 1
  fi
  _kcadm set-password -r "$KEYCLOAK_REALM" --username "$1" --new-password "$DEMO_PASSWORD" >/dev/null
}

# kc_token <username>: access token for the user, via the dsn-cli password grant
kc_token() {
  curl -s -X POST "$KEYCLOAK_URL/realms/$KEYCLOAK_REALM/protocol/openid-connect/token" \
    -d grant_type=password -d client_id=dsn-cli \
    -d username="$1" -d password="$DEMO_PASSWORD" |
    python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('access_token') or ''); sys.exit(0 if d.get('access_token') else 1)"
}
