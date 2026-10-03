# Development Guide

[<- README](../README.md) · [Architecture](architecture.md) · [Services](services.md) · [API](api.md) · [Infrastructure](infrastructure.md) · **Development** · [Observability](observability.md) · [Search](search.md)

---

## Prerequisites

- **Docker** & **Docker Compose** (v2)
- **Go 1.27+** (for local builds/tests; an older Go downloads 1.27 automatically via `GOTOOLCHAIN=auto`)
- **Node 22** (UI)
- **curl** (for testing APIs)
- **python3** (used by demo script for JSON formatting)
- **golangci-lint v2.14.0** built with Go 1.27 (`GOTOOLCHAIN=go1.27.1 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`), for `make lint`
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
# Start the core (rebuilds changed services); add groups as needed: OBS=1 TOOLS=1 SEARCH=1 EVENTS=1, ALL=1
make up
make up OBS=1       # the same plus observability; groups left out are stopped

# Pause / resume without removing or rebuilding containers (start takes the same flags)
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

With compose running, the dev server talks to the compose gateway and logs in through the compose Keycloak (`VITE_KEYCLOAK_URL` in `ui/.env.development`; a production build uses `<origin>/auth`). `make demo` creates alice, bob and charlie with password `password`. `make ui` instead port-forwards a Kubernetes `gateway-service` to 8080 before starting Vite, so use it only against a cluster, with compose stopped.

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
├── broker/         Producer, at-least-once Consume/ConsumeBatch with retries + DLQ, EnsureTopics
├── cache/          Valkey client, feed cache keys and the read/write helpers both feed services share
├── database/       Postgres, ScyllaDB, ClickHouse connections (retried) + Migrate* helpers
├── events/         Topic names
├── grpc/           Generated gRPC stubs (from proto/, via make proto)
├── id/             Random hex IDs and deterministic IDs
├── observability/  Logger, tracer, instrumented gRPC server/dial options
├── outbox/         Transactional outbox: Enqueue/Relay (Postgres), AddToBatch/ScyllaRelay (ScyllaDB)
├── retry/          Backoff until a dependency answers or ctx is done
└── validate/       Request field checks returning InvalidArgument
```

`tools/dlq` is a CLI for parked messages (`make dlq`): `list`, `replay` and `skip`, optionally `--topic <source topic>`. Replay progress is the `dlq-replay` consumer group's offsets on the `.dlq` topics.

## Go Workspace

The project uses Go workspaces (`go.work`) to link all modules:

```
go 1.27.0

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

7. Copy any service's `Dockerfile`, then run `make dockerfiles` to regenerate it for the new service

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

CI (`.github/workflows/ci.yml`) runs `gofmt`, `go vet`, golangci-lint (`.golangci.yml`, v2.14.0) and `go test -race` per module, checks that the Dockerfiles match `make dockerfiles`, and lints and builds the UI.

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
# Through gateway: public reads need nothing
curl -s http://localhost:8080/api/v1/users/by-username/alice | python3 -m json.tool

# writes need a token; scripts/lib/auth.sh has helpers (kc_login, kc_ensure_user, kc_token)
source scripts/lib/auth.sh
curl -s -X POST http://localhost:8080/api/v1/posts/ -H "Authorization: Bearer $(kc_token alice)" \
  -H 'Content-Type: application/json' -d '{"text":"hi"}'

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

### Services not ready right after `make up`

Services wait for their database, MinIO or Kafka at startup (`pkg/retry`: backoff up to 10s, logged as `... failed, retrying`) and serve `/health` only once connected. ScyllaDB takes 30-60 seconds, so posts-service is usually last. A service that keeps logging retries points at its dependency: `docker logs infrastructure-<service>-1`.

### Port conflicts

If ports 8080, 9090, 3000, etc. are in use, stop conflicting services or modify the port mappings in `docker-compose.yml`.

### "build output cmd already exists"

This happens when running `go build ./...` inside a service that has a `cmd/` directory. Use explicit output: `go build -o /tmp/test ./cmd`.

## Kubernetes Deployment

Three Helm charts, deployed by Argo CD from [distributed-social-network-gitops](https://github.com/vahan-sahakyan/distributed-social-network-gitops):

| Chart | Contents |
|---|---|
| `deploy/kubernetes/infra` | Postgres x5 (one for Keycloak), Scylla, Redpanda, ClickHouse, MinIO, Valkey, Elasticsearch, Keycloak |
| `deploy/kubernetes/services` | the 11 services, UI, the `dsn` Gateway and its HTTPRoute |
| `deploy/kubernetes/observability` | Prometheus, Grafana, Loki, Alloy, Jaeger, Redpanda Console, optional Kibana; configs from `monitoring/` |

Traffic enters through Gateway API: the services chart creates the `dsn` Gateway (class `traefik`, k3s's bundled Traefik with its Gateway provider turned on by `platform-k3s/traefik.yaml` in the gitops repo) and an HTTPRoute for `route.host` that sends `/api`, `/health` and `/images` to the gateway, `/auth` to Keycloak and everything else to the UI. The observability chart adds one HTTPRoute per UI on `<name>.<domain>`. With `route.tls`, the chart requests one cert-manager certificate (`route.clusterIssuer`) for `route.host` plus `route.extraHosts`, serves it on one HTTPS listener, and redirects the app's HTTP to it (`route.httpsPort` when HTTPS isn't on 443). Locally `extraHosts` lists the UI hosts (`grafana.localhost`, ...) so they get HTTPS as well; a `*.localhost` wildcard doesn't work, TLS clients reject wildcards under single-label names. For another controller, set `route.gateway.className` and its listener ports, or `route.gateway.create: false` with `route.parentRefs` to attach to a shared Gateway.

Images: `ghcr.io/vahan-sahakyan/distributed-social-network/<name>:<sha>` (linux/arm64), published on every push by `.github/workflows/publish.yml`. On `main` the workflow also commits the new sha to the gitops repo, which Argo CD syncs.

Local cluster, synced by Argo CD the same way as prod (`bootstrap/root-local.yaml` in the gitops repo):

```bash
make stop           # free compose's memory first
make cluster-up     # k3d + Argo CD, synced in a few minutes; takes the group flags, e.g. OBS=1
make cluster-profile SEARCH=1   # switch groups on a running cluster; unset flags turn groups off
make trust-ca       # once per machine: browsers accept the cluster's certificates
make forward        # compose's localhost ports + Argo CD on https://localhost:9443
make cluster-down
# app on https://localhost:8443 (http://localhost:8081 redirects), Keycloak on https://localhost:8443/auth
# minio, and grafana, prometheus, jaeger, redpanda, kibana with their groups, on https://<name>.localhost:8443
make demo CLUSTER=1 # demo users and data, also make load CLUSTER=1
```

It runs the commit prod runs, with `envs/local` values on top of prod's; only the bootstrap inputs differ, and the optional groups are off unless switched on. `apps-local` in the gitops repo is a Helm chart whose switches (`obs`, `tools`, `search`, `events`) sit on the hand-applied `root-local` app: `cluster-up` and `cluster-profile` patch them, so toggling needs no commit, and Argo CD adds or prunes the groups (stateful ones keep their volumes). `make cluster-up` loads:
- `dsn-local-ca`: the machine's CA (generated once in `~/.config/dsn/local-ca.{crt,key}`, kept across clusters so it is trusted once) for the `local-ca` ClusterIssuer, instead of Let's Encrypt
- `openbao-unseal`: a fresh static seal key for OpenBao
- `openbao-seed`: the public dev values from the gitops repo's `envs/local/openbao-seed.env`, which OpenBao writes into its kv store on first start

Secrets then flow as in prod: OpenBao -> External Secrets (`openbao` ClusterSecretStore) -> the charts' `ExternalSecret`s (`secrets` in each chart's values) -> the Secrets the pods read. Nothing secret is rendered from chart values.

Changes reach the cluster through git only. To try an unpushed chart change, stop Argo CD from reverting it and apply the working tree's chart with the local values:

```bash
kubectl -n argocd patch app root-local --type merge -p '{"spec":{"syncPolicy":null}}'
kubectl -n argocd patch app services --type merge -p '{"spec":{"syncPolicy":null}}'
E=../distributed-social-network-gitops/envs
helm template services deploy/kubernetes/services -n dsn -f $E/prod/services-values.yaml -f $E/local/services-values.yaml \
  | kubectl -n dsn apply --server-side --force-conflicts -f -
# back to git
kubectl apply -f https://raw.githubusercontent.com/vahan-sahakyan/distributed-social-network-gitops/main/bootstrap/root-local.yaml
```
