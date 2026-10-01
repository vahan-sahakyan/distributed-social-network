# ADR 0003: Authentication with OIDC and Keycloak

- **Status:** accepted
- **Date:** 2026-10-01

## Context

The API had no authentication. The acting user was whatever `user_id`, `author_id` or `follower_id` the client sent, so anyone could post, like or follow as anyone, read anyone's notifications, and call `POST /api/v1/reset`.

## Options

| Option | For | Against |
|--------|-----|---------|
| Passwords in users-service, gateway-issued JWTs | No new infrastructure; small change | We own password storage, reset, lockout, MFA; a homegrown token format |
| **OIDC with Keycloak** | Standard protocol (authorization code + PKCE); login, sign-up, password policy, sessions and MFA come with it; services only verify tokens | One more JVM service plus its database (~600 MB); realm config to maintain |
| Managed IdP (Auth0, Cognito, Entra ID) | Nothing to run | External dependency and account; doesn't work offline in compose |
| Signed dev tokens without passwords | Trivial | Anyone can still sign in as anyone |

## Decision

Keycloak is the identity provider; the gateway is the only place tokens are checked.

```
browser --PKCE login--> Keycloak (/auth)
browser --Bearer token--> gateway --verify (JWKS, aud=dsn-api, exp, iss)--> sets acting user from `sub` --gRPC--> services
```

- Realm `dsn` (`deploy/kubernetes/infra/files/dsn-realm.json`, used by compose and Helm):
  - `dsn-ui`: public client, authorization code + PKCE, redirects only to the UI origin
  - `dsn-cli`: password grant, for `make demo` / `make load` only
  - both add the `dsn-api` audience the gateway requires
  - sign-up asks for a username in the users-service format; email optional
- A profile's id is the token's `sub`, and its username is `preferred_username`; `POST /api/v1/users/` creates the caller's profile, `GET /api/v1/me` returns it
- Writes and private reads require a token and ignore any user id in the body; public reads (posts, profiles, feeds of a user, search) don't
- `/reset` exists only with `ALLOW_RESET=true` (compose)

## Consequences

- Impersonation through request bodies is gone; a stolen token works until it expires (15 minutes)
- Services still trust the gateway: anything that reaches their gRPC ports (exposed to the host in compose) can act as anyone. Propagating the token or mTLS between services is a separate step
- Accounts live in Keycloak, profiles in users-service: a reset wipes profiles but not accounts, and the UI asks for a new profile on the next login. Deleting an account doesn't delete its profile
- The realm file is imported only into an empty Keycloak database; later edits go through the admin console or a fresh volume
- The UI can no longer act as several users, so demo seeding moved to `make demo`
