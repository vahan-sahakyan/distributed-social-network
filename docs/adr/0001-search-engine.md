# ADR 0001: Elasticsearch for post and user search

- **Status:** accepted
- **Date:** 2026-09-30

## Context

Users need to find posts by words and hashtags, and people by username or bio. The data is spread across services: post text in ScyllaDB (posts-service), users in PostgreSQL (users-service). No single store can search across both, and ScyllaDB has no full-text search at all.

Requirements:
- relevance ranking, stemming ("deploying" finds "deployed"), typo tolerance
- username autocomplete from the first letter
- hashtag lookup and "trending in the last N hours"
- no extra load on the write path's databases

## Options

| Option | For | Against |
|--------|-----|---------|
| PostgreSQL full-text search | No new infra | Posts are not in PostgreSQL; would need a copy anyway. Weak fuzzy matching and ranking; aggregations are plain SQL |
| Meilisearch / Typesense | Light, fast, great typo tolerance and autocomplete out of the box | Weaker aggregations and analytics; smaller ecosystem |
| **Elasticsearch** | BM25 ranking, analyzers, fuzzy, edge n-grams, aggregations, function scoring; Kibana for exploring; widely used | Heavy (1 GB heap minimum here), JVM tuning, license (see below) |
| OpenSearch | Same engine lineage as Elasticsearch, Apache 2.0 | Diverging APIs and clients from Elasticsearch 8+ |

## Decision

A new **search-service** keeps Elasticsearch indices as a **read model**, fed by the event stream, the same CQRS pattern the feed uses:

```
posts-service -- post.created --> Redpanda --> search-service -- _bulk --> Elasticsearch
users-service -- user.created -->
gateway -- gRPC SearchPosts / SearchUsers / TrendingHashtags --> search-service -- _search -->
```

- Consumes with `broker.ConsumeBatch`: one `_bulk` request per batch, document `_id` = entity id, so redelivery rewrites the same document
- Versioned indices behind aliases (`posts` -> `posts-v1`): a mapping change is a new index, a reindex and an atomic alias swap
- Strict mappings, so an unexpected field fails loudly (and lands on the DLQ) instead of silently changing the schema

Elasticsearch over OpenSearch because it is what this project wants to learn, the official Go client is current, and nothing here depends on the license. Elasticsearch is available under AGPLv3, SSPL or the Elastic License 2.0; running it unmodified as a backing store is fine under all three. Offering it as a hosted service to third parties is what they restrict.

## Consequences

- **Eventually consistent**: a new post is searchable after the event is consumed plus the 1s index refresh, not when `POST /posts` returns
- **Source of truth stays in the owning services**: the indices can be dropped and rebuilt. Rebuilding from events only works within Redpanda's retention (7 days by default); older data needs a backfill from the services (not built yet)
- **Denormalization is on the reader**: hits carry `author_id`, not the username, since a post event does not include it
- One more JVM service to run and watch: about 1.5 GB of memory locally, `vm.max_map_count` >= 262144 on the host (Docker Desktop has it)
