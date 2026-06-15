# Development Guide

## Prerequisites

- **Docker** & **Docker Compose** (v2)
- **Go 1.24+** (for local builds/tests)
- **curl** (for testing APIs)
- **python3** (used by demo script for JSON formatting)

## First-Time Setup

```bash
git clone <repository-url>
cd distributed-social-network

# Start everything from scratch (builds containers, runs migrations)
make fresh

# Verify all services are healthy
make demo
```

## Daily Workflow

```bash
# Start the system (rebuilds changed services)
make up

# Apply migrations (idempotent, safe to re-run)
make migrate

# Stop everything (preserves data)
make down

# Full reset (wipes all data)
make fresh
```

## Project Layout

Each service follows the same structure:

```
services/<name>/
├── Dockerfile         Multi-stage build (Go → minimal image)
├── go.mod             Module with local replace directives
├── cmd/
│   └── main.go        Entry point, wiring, server setup
├── internal/
│   ├── handler/       HTTP handlers (Fiber routes)
│   ├── service/       Business logic
│   ├── repository/    Database access layer
│   ├── model/         Data structures
│   ├── consumer/      Kafka/Redpanda consumer (if applicable)
│   └── storage/       Object storage (media-service only)
└── migrations/
    └── 001_*.sql      Schema definitions
```

Shared code lives in `pkg/`:

```
pkg/
├── broker/producer.go   Kafka producer wrapper
├── cache/redis.go       Memcached client (named redis.go historically)
├── database/postgres.go PostgreSQL connection pool
└── id/id.go             Hex ID generation
```

## Go Workspace

The project uses Go workspaces (`go.work`) to link all modules:

```
go 1.24

use (
    ./pkg
    ./services/gateway-service
    ./services/posts-service
    ./services/feed-service
    ./services/comments-service
    ./services/likes-service
    ./services/users-service
    ./services/media-service
    ./services/notification-service
    ./services/event-writer-service
    ./services/cache-rebuilder-service
)
```

Each service's `go.mod` has a `replace` directive pointing to the local `pkg/`:

```go
replace github.com/vahan-sahakyan/distributed-social-network/pkg => ../../pkg
```

## Adding a New Service

1. Create the directory structure:
   ```bash
   mkdir -p services/my-service/{cmd,internal/{handler,service,repository,model},migrations}
   ```

2. Initialize the module:
   ```bash
   cd services/my-service
   go mod init github.com/vahan-sahakyan/distributed-social-network/my-service
   ```

3. Add the `replace` directive in `go.mod`:
   ```go
   replace github.com/vahan-sahakyan/distributed-social-network/pkg => ../../pkg
   ```

4. Add to `go.work`:
   ```
   use ./services/my-service
   ```

5. Create `Dockerfile` (copy from an existing service)

6. Add to `infrastructure/docker-compose.services.yml`:
   ```yaml
   my-service:
     build:
       context: ../
       dockerfile: services/my-service/Dockerfile
     restart: on-failure
     environment:
       PORT: "8090"
       # ... other env vars
     depends_on:
       - <database>
   ```

7. Add route in `services/gateway-service/cmd/main.go`

8. Add Prometheus scrape target in `monitoring/prometheus/prometheus.yml`

9. Add to `SERVICES` list in `Makefile`

10. Add migration SQL to `scripts/migrate.sh`

## Running Tests

```bash
# All services
make test

# Single service
cd services/users-service && go test ./...
```

## Building Locally

```bash
# All services → bin/ directory
make build

# Single service
cd services/posts-service && go build -o ../../bin/posts-service ./cmd
```

## Debugging

### View service logs

```bash
# All services
docker compose -f infrastructure/docker-compose.yml \
  -f infrastructure/docker-compose.services.yml logs -f

# Single service
docker compose -f infrastructure/docker-compose.yml \
  -f infrastructure/docker-compose.services.yml logs -f users-service
```

### Check service status

```bash
docker compose -f infrastructure/docker-compose.yml \
  -f infrastructure/docker-compose.services.yml ps
```

### Access databases directly

```bash
# PostgreSQL
docker exec -it infrastructure-users-db-1 psql -U postgres -d users

# ScyllaDB
docker exec -it infrastructure-posts-db-1 cqlsh

# ClickHouse
docker exec -it infrastructure-clickhouse-1 clickhouse-client
```

### Inspect Redpanda topics

```bash
# List topics
docker exec infrastructure-redpanda-1 rpk topic list

# Consume messages
docker exec infrastructure-redpanda-1 rpk topic consume post.created --num 5
```

### Test endpoints directly

```bash
# Through gateway
curl -s http://localhost:8080/api/v1/users/ | python3 -m json.tool

# Directly to service (bypass gateway)
curl -s http://localhost:8085/api/v1/users/ | python3 -m json.tool
```

### Check Prometheus targets

```bash
curl -s http://localhost:9090/api/v1/targets | python3 -c "
import json, sys
data = json.load(sys.stdin)
for t in data['data']['activeTargets']:
    print(f\"{t['labels']['job']:30s} {t['health']}\")"
```

## Common Issues

### Services crashing on startup

Services may fail to connect if databases aren't ready yet. The `restart: on-failure` policy handles this — services will retry automatically. Wait 10-15 seconds after `make up`.

### Tables not found

Run `make migrate` after starting containers. The migration script waits for databases to be ready before applying schemas.

### ScyllaDB slow to start

ScyllaDB takes 30-60 seconds to initialize. The migrate script waits up to 120 seconds for it.

### Port conflicts

If ports 8080, 9090, 3000, etc. are in use, stop conflicting services or modify the port mappings in `docker-compose.yml`.

### "build output cmd already exists"

This happens when running `go build ./...` inside a service that has a `cmd/` directory. Use explicit output: `go build -o /tmp/test ./cmd`.

## Kubernetes Deployment

A Helm chart is available at `deploy/kubernetes/`:

```bash
helm install dsn ./deploy/kubernetes \
  --set image.tag=latest \
  --values deploy/kubernetes/values.yaml
```

See `deploy/kubernetes/values.yaml` for configuration options.
