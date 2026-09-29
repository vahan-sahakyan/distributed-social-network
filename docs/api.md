# API Reference

[<- README](../README.md) · [Architecture](architecture.md) · [Services](services.md) · **API** · [Infrastructure](infrastructure.md) · [Development](development.md) · [Observability](observability.md)

---

All endpoints are served by the gateway at `http://localhost:8080` (compose) or the cluster ingress, which routes `/api`, `/health` and `/images` to the gateway and everything else to the UI.

## Base URL

```
http://localhost:8080/api/v1
```

## Conventions

- **No authentication.** The acting user is passed in the body or query (`user_id`, `follower_id`, `author_id`).
- **Success:** resource JSON with 200, 201 or 204.
- **Error:** `{"error": "<message>"}`. gRPC codes map to `NotFound` -> 404, `InvalidArgument` -> 400, `AlreadyExists` -> 409, anything else -> 500. An unparseable body is 400.
- **Responses are protobuf messages encoded with `encoding/json`:**
  - fields with zero values are omitted (`"likes": 0` does not appear, nor does an empty `image_id`)
  - timestamps are objects: `"created_at": {"seconds": 1781488793, "nanos": 301000000}`
  - an empty list is `null`

---

## Users

### Create User

```http
POST /api/v1/users/
Content-Type: application/json

{
  "username": "alice",
  "bio": "Software engineer"
}
```

**Response** `201 Created`:
```json
{
  "id": "30a46156e6b96a2a9d2c96bc765ab511",
  "username": "alice",
  "bio": "Software engineer",
  "created_at": {"seconds": 1781488793, "nanos": 301000000}
}
```

### Get User

```http
GET /api/v1/users/:id
GET /api/v1/users/by-username/:username
```

**Response** `200 OK`: the user, as above. `404` if not found.

### Follow / Unfollow User

```http
POST   /api/v1/users/:id/follow
DELETE /api/v1/users/:id/follow
Content-Type: application/json

{
  "follower_id": "c3abbc40aa9c8de72e21ee92d3f4e5cf"
}
```

**Response** `204 No Content`

> `:id` is the user being followed. `follower_id` is the user doing the following.

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

### Create Post

```http
POST /api/v1/posts/
Content-Type: application/json

{
  "text": "Hello distributed world!",
  "author_id": "30a46156e6b96a2a9d2c96bc765ab511",
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

### Create Comment

```http
POST /api/v1/comments/
Content-Type: application/json

{
  "user_id": "c3abbc40aa9c8de72e21ee92d3f4e5cf",
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

### Get Comments by Entity

```http
GET /api/v1/comments/entity/:entity_id
```

**Response** `200 OK`: a JSON array of comments, as above.

---

## Likes

### Like

```http
POST /api/v1/likes/
Content-Type: application/json

{
  "user_id": "c3abbc40aa9c8de72e21ee92d3f4e5cf",
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

> Idempotent: a repeated like returns 201 but stores nothing and publishes no event.

### Unlike

```http
DELETE /api/v1/likes/
Content-Type: application/json

{
  "user_id": "c3abbc40aa9c8de72e21ee92d3f4e5cf",
  "entity_id": "8316cac68f930d1006c9bcac26a6b3c9"
}
```

**Response** `204 No Content`

**Side effect:** Publishes `like.deleted` if a like was removed.

### Check Like

```http
GET /api/v1/likes/check?user_id=:user_id&entity_id=:entity_id
```

**Response** `200 OK`:
```json
{
  "liked": true
}
```

---

## Media

### Upload File

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

### Get Home Feed

```http
GET /api/v1/feed/home?user_id=:user_id
```

Same response format. Returns posts from the users the given user follows, plus their own.

> Feeds are served from Memcached. If the cache is cold, `POST /api/v1/rebuild` repopulates it from the event store.

---

## Notifications

### Get User Notifications

```http
GET /api/v1/notifications/:user_id
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

## Cache Rebuilder

### Trigger Rebuild

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

> Without `user_id`, rebuilds feed caches from recent ClickHouse events. With `user_id`, rebuilds that user's home feed and own posts, and returns `"user feed rebuilt"`.

---

## Reset (dev)

```http
POST /api/v1/reset
```

**Response** `200 OK`: `{"status": "reset complete"}`

> Calls `Reset` on every service: truncates all tables and the ClickHouse event store, and flushes Memcached. Uploaded files in MinIO are kept. Not access-controlled, and errors from individual services are ignored.

---

## Health & Metrics

```http
GET /health        -> {"status": "ok"}
GET /metrics       -> Prometheus text format
```

The gateway serves both on 8080. Every other service serves them on its own HTTP port (8081-8089), not through the gateway.
