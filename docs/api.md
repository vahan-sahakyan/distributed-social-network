# API Reference

All endpoints are accessed through the gateway at `http://localhost:8080`.

## Base URL

```
http://localhost:8080/api/v1
```

## Common Response Patterns

**Success:** Returns resource JSON with appropriate HTTP status (200, 201, 204).  
**Error:** Returns `{"error": "<message>"}` with 4xx/5xx status.

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
  "created_at": "2026-06-15T01:59:53.301Z"
}
```

### Get User

```http
GET /api/v1/users/:id
```

**Response** `200 OK`:
```json
{
  "id": "30a46156e6b96a2a9d2c96bc765ab511",
  "username": "alice",
  "bio": "Software engineer",
  "created_at": "2026-06-15T01:59:53.301Z"
}
```

### Follow User

```http
POST /api/v1/users/:id/follow
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
  "image_id": "",
  "likes": 0,
  "comments": 0,
  "created_at": "2026-06-15T01:59:55.290Z"
}
```

**Side effect:** Publishes `post.created` event to Redpanda.

### Get Post

```http
GET /api/v1/posts/:id
```

**Response** `200 OK`:
```json
{
  "id": "8316cac68f930d1006c9bcac26a6b3c9",
  "text": "Hello distributed world!",
  "author_id": "30a46156e6b96a2a9d2c96bc765ab511",
  "likes": 0,
  "comments": 0,
  "created_at": "2026-06-15T01:59:55.29Z"
}
```

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
  "likes": 0,
  "created_at": "2026-06-15T02:00:03.014Z"
}
```

**Side effect:** Publishes `comment.created` event to Redpanda.

### Get Comments by Entity

```http
GET /api/v1/comments/entity/:entity_id
```

**Response** `200 OK`:
```json
[
  {
    "id": "43a6738dae6bbd937aae44165416de4c",
    "user_id": "c3abbc40aa9c8de72e21ee92d3f4e5cf",
    "entity_id": "8316cac68f930d1006c9bcac26a6b3c9",
    "text": "Great post!",
    "likes": 0,
    "created_at": "2026-06-15T02:00:03.014Z"
  }
]
```

---

## Likes

### Create Like

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

**Side effect:** Publishes `like.created` event to Redpanda.

> Likes are idempotent — duplicate (user_id, entity_id) pairs are ignored via unique constraint.

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

> Max file size: 50MB. Files stored in MinIO bucket `images`.

### Get Media

```http
GET /api/v1/media/:id
```

**Response** `200 OK`:
```json
{
  "id": "48d2ac6e2c945a7d707136a03d9ae2c9",
  "url": "/images/48d2ac6e2c945a7d707136a03d9ae2c9"
}
```

---

## Feed

### Get User Feed

```http
GET /api/v1/feed/user/:user_id
```

**Response** `200 OK`:
```json
{
  "posts": [
    {
      "post_id": "8316cac68f930d1006c9bcac26a6b3c9",
      "author_id": "30a46156e6b96a2a9d2c96bc765ab511",
      "text": "Hello distributed world!",
      "likes_count": 2,
      "comments_count": 1,
      "created_at": "2026-06-15T01:59:55.29Z"
    }
  ]
}
```

> Feed is served from Memcached cache. Empty if cache is cold (run cache-rebuilder to populate).

### Get Home Feed

```http
GET /api/v1/feed/home?user_id=:user_id
```

Same response format. Returns posts from users the given user follows.

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
      "read": false,
      "created_at": "2026-06-15T02:00:01.5Z"
    }
  ]
}
```

> Notifications are generated asynchronously from `like.created` and `comment.created` events.

---

## Cache Rebuilder

### Trigger Rebuild

```http
POST /api/v1/rebuild
```

**Response** `200 OK`:
```json
{
  "status": "rebuild complete"
}
```

> Reads all events from ClickHouse and reconstructs Memcached feed caches.

---

## Health & Metrics

Every service exposes:

```http
GET /health        → {"status": "ok"}
GET /metrics       → Prometheus text format
```

These are available directly on each service's port (not through the gateway).
