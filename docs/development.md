# Development Guide

[<- README](../README.md) · [Architecture](architecture.md) · [Services](services.md) · [API](api.md) · [Infrastructure](infrastructure.md) · **Development**

---

## Prerequisites

- **Docker** & **Docker Compose** (v2)
- **Go 1.24+** (for local builds/tests)
- **Node 22** (UI)
- **curl** (for testing APIs)
- **python3** (used by demo script for JSON formatting)
- **protoc v7.35.1** (`brew install protobuf` / `brew upgrade protobuf` -> 35.1)
- **protoc-gen-go v1.36.11** (`go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11`)
- **protoc-gen-go-grpc v1.5.1** (`go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1`)

> These exact versions ensure generated stubs match what's committed under `pkg/grpc/`.
> After changing any `.proto` file, regenerate with `make proto`.

## First-Time Setup

```bash
git clone https://github.com/vahan-sahakyan/distributed-social-network.git
cd distributed-social-network

# Build and start everything (services apply their migrations on startup)
make up

# Exercise the whole flow once services are healthy
make demo
```

## Daily Workflow

```bash
# Start the system (rebuilds changed services)
make up

# Pause / resume without removing or rebuilding containers
make stop
make start

# Remove containers (preserves data)
make down

# Full reset (wipes all data)
make fresh
```

## UI

```bash
cd ui
npm install
npm run dev     # Vite dev server, proxies /api, /health, /images to localhost:8080
npm run lint    # oxlint
npm run build
```

With compose running, the dev server talks to the compose gateway. `make ui` instead port-forwards a Kubernetes `gateway-service` to 8080 before starting Vite, so use it only against a cluster, with compose stopped.

## Project Layout

Each service follows the same structure:

```
services/<name>/
├── Dockerfile         Multi-stage build (Go -> alpine), pkg/ COPY lines generated
├── go.mod             Module with a replace directive for ../../pkg
├── cmd/
│   └── main.go        Entry point, wiring, gRPC + health/metrics servers
├── internal/
│   ├── grpcserver/    gRPC API implementation
│   ├── service/       Business logic
│   ├── repository/    Database access layer
│   ├── model/         Data structures
│   ├── consumer/      Redpanda consumer (if applicable)
│   └── storage/       Object storage (media-service only)
└── migrations/
    ├── 001_*.sql      Schema definitions
    └── sql.go         Embeds the SQL for startup migration
```

Shared code lives in `pkg/`:

```
pkg/
├── broker/     Producer, at-least-once Consume with retries + DLQ, EnsureTopics
├── cache/      Memcached client, feed cache key builders
├── database/   Postgres, ScyllaDB, ClickHouse connections + Migrate* helpers
├── events/     Topic names
├── grpc/       Generated gRPC stubs (from proto/, via make proto)
└── id/         Random hex IDs and deterministic IDs
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

Docker builds and CI run with `GOWORK=off`, so each module's own `go.mod`/`go.sum` must be complete.

## Adding a New Service

1. Create the directory structure:
   ```bash
   mkdir -p services/my-service/{cmd,internal/{grpcserver,service,repository,model},migrations}
   ```

2. Initialize the module and add the `replace` directive:
   ```bash
   cd services/my-service
   go mod init github.com/vahan-sahakyan/distributed-social-network/my-service
   ```
   ```go
   replace github.com/vahan-sahakyan/distributed-social-network/pkg => ../../pkg
   ```

3. Add to `go.work`:
   ```
   use ./services/my-service
   ```

4. Define the API in `proto/my/my.proto`, add it to the `proto` target in the `Makefile`, and run `make proto`

5. Implement the server in `internal/grpcserver/`, serve it on `GRPC_PORT`, and serve `/health` + `/metrics` on `PORT`

6. Add migration SQL under `migrations/`, embed it via `migrations/sql.go`, and apply it on startup with the matching `pkg/database.Migrate*` helper

7. Create a `Dockerfile` from an existing service, then run `make dockerfiles` to generate its `pkg/` COPY lines

8. Add to `infrastructure/docker-compose.services.yml`:
   ```yaml
   my-service:
     build:
       context: ../
       dockerfile: services/my-service/Dockerfile
     restart: on-failure
     environment:
       PORT: "8090"
       GRPC_PORT: "9090"
       # ... other env vars
     depends_on:
       - <database>
   ```

9. Add a gRPC client and routes in `services/gateway-service/cmd/main.go`

10. Add a Prometheus scrape target in `monitoring/prometheus/prometheus.yml`

11. Add it to `SERVICES` in the `Makefile`, the module matrix in `.github/workflows/ci.yml`, the image matrix in `.github/workflows/publish.yml`, and `services:` in `deploy/kubernetes/services/values.yaml`

## Running Tests

```bash
# pkg + all services
make test

# Single module, as CI runs it
cd services/users-service && GOWORK=off go test -race ./...
```

CI (`.github/workflows/ci.yml`) runs `gofmt`, `go vet` and `go test -race` per module, checks that the Dockerfiles match `make dockerfiles`, and lints and builds the UI.

## Building Locally

```bash
# All services -> bin/ directory
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

See [DataGrip](datagrip.md) for connecting a DB client.

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
curl -s http://localhost:8080/api/v1/users/by-username/alice | python3 -m json.tool

# Directly to a service over gRPC (no server reflection, so pass the proto)
grpcurl -plaintext -import-path proto -proto users/users.proto \
  -d '{"username": "alice"}' localhost:9085 users.UsersService/GetUserByUsername
```

`docs/postman-collection.json` has gRPC requests for every service.

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

Services connect to their database once at startup and exit if it is not ready. `restart: on-failure` restarts them until it is. Wait 10-15 seconds after `make up`.

### Tables not found

Each service applies its own embedded migrations on startup. If a database was not ready yet, the service exits and `restart: on-failure` retries it.

### ScyllaDB slow to start

ScyllaDB takes 30-60 seconds to initialize. posts-service restarts until it can connect and migrate.

### Port conflicts

If ports 8080, 9090, 3000, etc. are in use, stop conflicting services or modify the port mappings in `docker-compose.yml`.

### "build output cmd already exists"

This happens when running `go build ./...` inside a service that has a `cmd/` directory. Use explicit output: `go build -o /tmp/test ./cmd`.

## Kubernetes Deployment

Two Helm charts, deployed by Argo CD from [distributed-social-network-gitops](https://github.com/vahan-sahakyan/distributed-social-network-gitops):

| Chart | Contents |
|---|---|
| `deploy/kubernetes/infra` | Postgres x4, Scylla, Redpanda, ClickHouse, MinIO, Memcached |
| `deploy/kubernetes/services` | the 10 services, UI, ingress |

The ingress routes `/api`, `/health` and `/images` to the gateway and everything else to the UI.

Images: `ghcr.io/vahan-sahakyan/distributed-social-network/<name>:<sha>` (linux/arm64), published on every push by `.github/workflows/publish.yml`. On `main` the workflow also commits the new sha to the gitops repo, which Argo CD syncs.

Standalone install on a local cluster (plain dev secrets, `createDevSecrets: true` by default):

```bash
k3d cluster create dsn -p "8081:80@loadbalancer"
helm install infra deploy/kubernetes/infra -n dsn --create-namespace
helm install services deploy/kubernetes/services -n dsn --set image.tag=main
# app on http://localhost:8081
```
