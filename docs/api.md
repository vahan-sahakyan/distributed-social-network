# API Reference

[<- README](../README.md) · [Architecture](architecture.md) · [Services](services.md) · **API** · [Infrastructure](infrastructure.md) · [Development](development.md) · [Observability](observability.md) · [Search](search.md)

---

All endpoints are served by the gateway at `http://localhost:8080` (compose) or the cluster's `dsn` Gateway, whose HTTPRoute sends `/api`, `/health` and `/images` to the gateway and everything else to the UI.

## Base URL

```
http://localhost:8080/api/v1
```

## Conventions

- **Authentication:** endpoints marked (auth) need `Authorization: Bearer <access token>` from Keycloak (realm `dsn`, see [Authentication](#authentication)). They act as the token's user: `user_id`, `author_id` or `follower_id` in a body is ignored. Missing, expired or foreign tokens get `401`. Everything else is public.
- **Success:** resource JSON with 200, 201 or 204.
- **Error:** `{"error": "<message>"}`. gRPC codes map to `NotFound` -> 404, `InvalidArgument` -> 400, `AlreadyExists` -> 409, `Unavailable` -> 503, `DeadlineExceeded` -> 504, anything else -> 500. Backend calls time out after 5s (cache rebuild: 2m), and wait for a restarting service within that time instead of failing at once; a service that stays down answers 504 after 5s. An unparseable body is 400. 5xx bodies are generic, `{"error": "internal error", "trace_id": "..."}`; the cause is in the gateway log under that trace id.
- **Validation (400):** required ids and text must be non-blank; `username` is 1-32 letters, digits or underscores; `bio` <= 500, post `text` <= 5000 (a post needs `text` or `image_id`), comment `text` <= 2000 characters; users can't follow themselves.
- **Responses are protobuf messages encoded with `encoding/json`:**
  - fields with zero values are omitted (`"likes": 0` does not appear, nor does an empty `image_id`)
  - timestamps are objects: `"created_at": {"seconds": 1781488793, "nanos": 301000000}`
  - an empty list is `null`

---

## Authentication

Accounts live in Keycloak (`http://localhost:8180/auth` in compose, `/auth` on the cluster host). Browsers use the `dsn-ui` client (authorization code + PKCE); local scripts can use the `dsn-cli` password grant:

```bash
curl -s -X POST http://localhost:8180/auth/realms/dsn/protocol/openid-connect/token \
  -d grant_type=password -d client_id=dsn-cli -d username=alice -d password=password
```

Tokens last 15 minutes. A new account has no profile until it calls `POST /api/v1/users/`. Decision record: [ADR 0003](adr/0003-authentication-oidc-keycloak.md).

---

## Users

### Create Profile (auth)

```http
POST /api/v1/users/
Authorization: Bearer <token>
Content-Type: application/json

{
  "bio": "Software engineer"
}
```

**Response** `201 Created`:
```json
{
  "id": "ac9b4762-85c6-43fe-a280-c452e609d2d3",
  "username": "alice",
  "bio": "Software engineer",
  "created_at": {"seconds": 1781488793, "nanos": 301000000}
}
```

> `id` is the token's subject and `username` its `preferred_username`. `409` if the caller already has a profile.

### Get Own Profile (auth)

```http
GET /api/v1/me
```

**Response** `200 OK`: the profile, as above. `404` until the caller created one.

### Get User

```http
GET /api/v1/users/:id
GET /api/v1/users/by-username/:username
```

**Response** `200 OK`: the user, as above. `404` if not found.

### Follow / Unfollow User (auth)

```http
POST   /api/v1/users/:id/follow
DELETE /api/v1/users/:id/follow
```

**Response** `204 No Content`

> The caller follows or unfollows `:id`.

### Get Followers

```http
GET /api/v1/users/:id/followers
```

**Response** `200 OK`:
```json
{
  "followers": [
    "c3abbc40aa9c8de72e21ee92d3f4e5cf",
    "6455a7fbd62afb913ac64e945adf4c6b"
  ]
}
```

### Get Following

```http
GET /api/v1/users/:id/following
```

**Response** `200 OK`:
```json
{
  "following": [
    "c3abbc40aa9c8de72e21ee92d3f4e5cf"
  ]
}
```

---

## Posts

### Create Post (auth)

```http
POST /api/v1/posts/
Content-Type: application/json

{
  "text": "Hello distributed world!",
  "image_id": "optional-media-id"
}
```

**Response** `201 Created`:
```json
{
  "id": "8316cac68f930d1006c9bcac26a6b3c9",
  "text": "Hello distributed world!",
  "author_id": "30a46156e6b96a2a9d2c96bc765ab511",
  "image_id": "optional-media-id",
  "created_at": {"seconds": 1781488795, "nanos": 290000000}
}
```

**Side effect:** Publishes `post.created`.

### Get Post

```http
GET /api/v1/posts/:id
```

**Response** `200 OK`: the post, as above. `404` if not found.

---

## Comments

### Create Comment (auth)

```http
POST /api/v1/comments/
Content-Type: application/json

{
  "entity_id": "8316cac68f930d1006c9bcac26a6b3c9",
  "text": "Great post!"
}
```

**Response** `201 Created`:
```json
{
  "id": "43a6738dae6bbd937aae44165416de4c",
  "user_id": "c3abbc40aa9c8de72e21ee92d3f4e5cf",
  "entity_id": "8316cac68f930d1006c9bcac26a6b3c9",
  "text": "Great post!",
  "created_at": {"seconds": 1781488803, "nanos": 14000000}
}
```

**Side effect:** Publishes `comment.created`.

> `404` if the post doesn't exist.

### Get Comments by Entity

```http
GET /api/v1/comments/entity/:entity_id
```

**Response** `200 OK`: a JSON array of comments, as above.

---

## Likes

### Like (auth)

```http
POST /api/v1/likes/
Content-Type: application/json

{
  "entity_id": "8316cac68f930d1006c9bcac26a6b3c9"
}
```

**Response** `201 Created`:
```json
{
  "id": "b8da2d908eb072667027407e5045a6f7",
  "user_id": "c3abbc40aa9c8de72e21ee92d3f4e5cf",
  "entity_id": "8316cac68f930d1006c9bcac26a6b3c9"
}
```

**Side effect:** Publishes `like.created`.

> Idempotent: a repeated like returns 201 but stores nothing and publishes no event. `404` if the post doesn't exist ([ADR 0004](adr/0004-post-existence-check.md)); unlike is not checked.

### Unlike (auth)

```http
DELETE /api/v1/likes/
Content-Type: application/json

{
  "entity_id": "8316cac68f930d1006c9bcac26a6b3c9"
}
```

**Response** `204 No Content`

**Side effect:** Publishes `like.deleted` if a like was removed.

### Check Like (auth)

```http
GET /api/v1/likes/check?entity_id=:entity_id
```

**Response** `200 OK`:
```json
{
  "liked": true
}
```

---

## Media

### Upload File (auth)

```http
POST /api/v1/media/upload
Content-Type: multipart/form-data

file: <binary file data>
```

**Response** `201 Created`:
```json
{
  "id": "48d2ac6e2c945a7d707136a03d9ae2c9",
  "url": "/images/48d2ac6e2c945a7d707136a03d9ae2c9"
}
```

> Max file size: 60 MB. Files are stored in MinIO bucket `images`.

### Get Media URL

```http
GET /api/v1/media/:id
```

**Response** `200 OK`: `{"id": "...", "url": "/images/<id>"}`

### Get File

```http
GET /images/:id
```

Served by the gateway from the public-read MinIO bucket.

---

## Feed

### Get User Feed

```http
GET /api/v1/feed/user/:user_id
```

**Response** `200 OK`: a JSON array of the user's own posts, newest first:
```json
[
  {
    "post_id": "8316cac68f930d1006c9bcac26a6b3c9",
    "author_id": "30a46156e6b96a2a9d2c96bc765ab511",
    "text": "Hello distributed world!",
    "likes_count": 2,
    "comments_count": 1,
    "image_url": "48d2ac6e2c945a7d707136a03d9ae2c9",
    "created_at": {"seconds": 1781488795, "nanos": 290000000}
  }
]
```

> `image_url` currently holds the media id, not a URL. `likes_count`, `comments_count` and `image_url` are omitted when zero or empty.

### Get Home Feed (auth)

```http
GET /api/v1/feed/home
```

Same response format. Returns posts from the users the given user follows, plus their own.

> Feeds are served from Memcached. If the cache is cold, `POST /api/v1/rebuild` repopulates it from the event store.

---

## Notifications

### Get Notifications (auth)

```http
GET /api/v1/notifications
```

**Response** `200 OK`:
```json
{
  "notifications": [
    {
      "id": "abc123",
      "user_id": "30a46156e6b96a2a9d2c96bc765ab511",
      "type": "like",
      "actor_id": "c3abbc40aa9c8de72e21ee92d3f4e5cf",
      "entity_id": "8316cac68f930d1006c9bcac26a6b3c9",
      "created_at": {"seconds": 1781488801, "nanos": 500000000}
    }
  ]
}
```

> Generated asynchronously from `like.created` and `comment.created`, for the post author. Returns the 50 most recent. `read` is omitted while false.

---

## Search

Served by search-service from Elasticsearch; see [Search](search.md) for how results are ranked. Results are eventually consistent: a new post or user is searchable about a second after it is created. Search is an optional group (`SEARCH=1`); while it is off, every route here answers 503 `{"error": "search is disabled"}`.

### Search Posts

```http
GET /api/v1/search/posts?q=deploying&limit=20
GET /api/v1/search/posts?q=%23golang
```

`q` is required: words (stemmed, typo tolerant) or `#tag` (exact hashtag). `limit` defaults to 20, max 50.

**Response** `200 OK`:
```json
{
  "total": 1,
  "posts": [
    {
      "id": "8316cac68f930d1006c9bcac26a6b3c9",
      "author_id": "30a46156e6b96a2a9d2c96bc765ab511",
      "text": "Just deployed our new microservices architecture! #distributed #golang",
      "hashtags": ["distributed", "golang"],
      "created_at": {"seconds": 1781488801, "nanos": 500000000},
      "highlight": "Just <em>deployed</em> our new microservices architecture! #distributed #golang",
      "score": 2.69
    }
  ]
}
```

> `highlight` is omitted when only a hashtag matched. Hits carry `author_id`, not the username.

### Search Users

```http
GET /api/v1/search/users?q=ali&limit=20
```

`q` is required: a username prefix (a leading `@` is ignored) or words from the bio.

**Response** `200 OK`:
```json
{
  "total": 1,
  "users": [
    {"id": "30a46156e6b96a2a9d2c96bc765ab511", "username": "alice", "bio": "Software engineer", "created_at": {"seconds": 1781488793}, "score": 4.37}
  ]
}
```

### Trending Hashtags

```http
GET /api/v1/search/hashtags/trending?hours=24&limit=10
```

Hashtags by number of posts created in the last `hours` (default 24, max 720). `limit` defaults to 10, max 50.

**Response** `200 OK`:
```json
{"hashtags": [{"hashtag": "golang", "posts": 12}, {"hashtag": "distributed", "posts": 7}]}
```

---

## Cache Rebuilder

### Trigger Rebuild (auth)

```http
POST /api/v1/rebuild
POST /api/v1/rebuild?user_id=:user_id
```

**Response** `200 OK`:
```json
{
  "status": "rebuild complete"
}
```

> Without `user_id`, rebuilds feed caches from recent ClickHouse events. With `user_id`, rebuilds that user's home feed and own posts, and returns `"user feed rebuilt"`. Needs the event store group (`EVENTS=1`); while it is off this answers 503 `{"error": "event store is disabled"}`.

---

## Reset (dev)

```http
POST /api/v1/reset
```

**Response** `200 OK`: `{"status": "reset complete"}`

> Only registered when the gateway runs with `ALLOW_RESET=true` (compose, and the local k3d cluster via `envs/local` in the gitops repo); `404` elsewhere. Calls `Reset` on every service: truncates all tables and the ClickHouse event store, flushes Memcached and empties the search indices. Uploaded files in MinIO and Keycloak accounts are kept, so users recreate their profile on the next login. Every running service is attempted (switched-off groups are skipped); if any fail it answers 500 `reset failed for <services>` with a `trace_id`.

---

## Health & Metrics

```http
GET /health        -> {"status": "ok"}
GET /metrics       -> Prometheus text format
```

The gateway serves both on 8080. Every other service serves them on its own HTTP port (8081-8089, search 8091), not through the gateway.
