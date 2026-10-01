# ADR 0004: Check that a post exists before liking or commenting

- **Status:** accepted
- **Date:** 2026-10-02

## Context

likes-service and comments-service validated that `entity_id` was present (#16), not that the post exists. A like or comment on a random id was stored, published, fanned out, and parked by consumers that couldn't resolve its author.

## Options

| Option | For | Against |
|--------|-----|---------|
| **Synchronous `GetPost` from likes/comments** | Rejected at the source with a 404; no junk rows or events | likes/comments depend on posts-service availability; one more hop per write |
| Check in the gateway | Same result | Puts a domain rule in the API layer; other callers of likes-service skip it |
| Asynchronous: consumers drop events for unknown posts | No runtime coupling | Junk stays in likes-db/comments-db and in the event store; the client gets a 201 for a write that went nowhere |
| Local read model: likes/comments keep the set of post ids from `post.created` | No runtime coupling, local check | A post is unknown until its event arrives, so a like right after posting is rejected; another copy of data to keep in sync |

## Decision

likes-service and comments-service call posts-service `GetPost` before writing (`pkg/posts.Checker`):
- `NotFound` -> the write is rejected with `NotFound` (gateway: 404 "post not found")
- any other error (posts-service down) -> the write fails as unavailable; it is never reported as a missing post
- unlike is not checked, so likes on a deleted post can still be removed

Consumers (feed, notification) treat `NotFound` from posts-service as permanent and skip the event instead of retrying and parking it: the post was deleted or wiped by a reset.

## Consequences

- Likes and comments need posts-service: while it is down they fail (504 after the 5s call timeout) instead of being accepted. A restarting posts-service only delays them (#48)
- One extra gRPC call per like or comment (single-digit ms locally)
- The local read model stays the option if posts-service availability starts to matter for likes
