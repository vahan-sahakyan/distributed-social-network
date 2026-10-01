# ADR 0002: Transactional outbox for domain events

- **Status:** accepted for the PostgreSQL services; posts-service (ScyllaDB) pending
- **Date:** 2026-10-01

## Context

Services write to their database, then publish to Redpanda. The two are separate systems with no shared transaction (the dual-write problem):

| Failure | Result |
|---------|--------|
| Write ok, publish fails (broker down, timeout) | Event lost: feed, notifications, event store and search never see it. Logged and dropped |
| Process dies between write and publish | Event lost, nothing logged |
| Publish ok, write fails (if reordered) | Consumers see an event for data that doesn't exist |

Everything downstream (feed fanout, notifications, ClickHouse, search) is a read model built from events, so a lost event is permanent drift.

## Options

| Option | For | Against |
|--------|-----|---------|
| Retry the publish in-process | Trivial | Still loses events on crash or long outage; blocks the request |
| **Transactional outbox, polling relay** | Event committed atomically with the data; no new infrastructure; easy to observe (it's a table) | Extra write per request; polling latency; at-least-once (a relay crash after publish, before delete, republishes) |
| Outbox + CDC (Debezium on the WAL) | No polling, lower latency | Kafka Connect + Debezium to run; logical replication setup per database |
| Event sourcing (events are the source of truth) | No dual write at all | Rewrites every service's storage model |
| 2PC / XA | Atomic across both | Kafka has no XA; heavy and fragile |

## Decision

Transactional outbox with an in-process polling relay, in `pkg/outbox`:

1. The service writes its row and an `outbox` row (topic, key, payload, trace headers) in one transaction
2. A relay goroutine in the same service reads pending rows in id order, publishes them, and deletes them in the same transaction once the broker acked
3. `pg_try_advisory_xact_lock` lets only one relay per database publish at a time, so per-key order holds with several replicas
4. The request's trace context is stored on the row, so the delayed publish still joins the request's trace

posts-service is excluded: ScyllaDB has no multi-table transactions. Candidates are a LOGGED BATCH into `posts` + an outbox table (atomic, but a tombstone-heavy queue table) or Scylla CDC. Separate decision.

## Consequences

- No event is lost while the database commit succeeds; a Redpanda outage delays events instead of dropping them
- Up to ~100ms extra latency (poll interval) between commit and publish
- **At-least-once**: a relay that crashes between publish and delete republishes the batch. Consumers that dedupe by Kafka offset (event-writer, notification-service) see the republish as a new message
- Producers now wait for the broker's ack (`RequiredAcks: RequireAll`). kafka-go defaults to `RequireNone`, which reported success for writes into a connection to a stopped broker; the relay deleted those rows and the events were gone. This affected every publish before the outbox, too
- One more table per database; `outbox_backlog` and `outbox_oldest_age_seconds` metrics and an `OutboxStuck` alert show a relay that can't publish
