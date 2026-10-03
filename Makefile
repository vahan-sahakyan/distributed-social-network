SERVICES = gateway-service posts-service comments-service likes-service feed-service users-service media-service notification-service event-writer-service cache-rebuilder-service search-service

# optional groups, off unless asked for: make up OBS=1 SEARCH=1 EVENTS=1 TOOLS=1, or ALL=1.
# compose runs them as profiles; cluster-up and cluster-profile hand them to the root-local app
ifeq ($(ALL),1)
OBS := 1
SEARCH := 1
EVENTS := 1
TOOLS := 1
endif
on = $(filter 1,$($(1)))

COMPOSE = docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml
# kibana needs elasticsearch, so it runs only with both
PROFILES = $(if $(call on,OBS),--profile obs) $(if $(call on,SEARCH),--profile search) \
	$(if $(call on,EVENTS),--profile events) $(if $(call on,TOOLS),--profile tools) \
	$(if $(and $(call on,TOOLS),$(call on,SEARCH)),--profile kibana)
# addresses of switched-off services stay empty, which the services read as "off"
GROUP_ENV = OTEL_EXPORTER_OTLP_ENDPOINT=$(if $(call on,OBS),http://jaeger:4318) \
	SEARCH_SERVICE_GRPC_ADDR=$(if $(call on,SEARCH),search-service:9091) \
	CACHE_REBUILDER_SERVICE_GRPC_ADDR=$(if $(call on,EVENTS),cache-rebuilder-service:9089)

.PHONY: proto
proto:
	@export PATH="$${PATH}:/opt/homebrew/bin:$$(go env GOPATH)/bin" && \
	protoc \
		--go_out=./pkg \
		--go_opt=module=github.com/vahan-sahakyan/distributed-social-network/pkg \
		--go-grpc_out=./pkg \
		--go-grpc_opt=module=github.com/vahan-sahakyan/distributed-social-network/pkg \
		-I proto \
		proto/users/users.proto \
		proto/posts/posts.proto \
		proto/comments/comments.proto \
		proto/likes/likes.proto \
		proto/feed/feed.proto \
		proto/media/media.proto \
		proto/notifications/notifications.proto \
		proto/cache_rebuilder/cache_rebuilder.proto \
		proto/search/search.proto
	@echo "gRPC code generation complete."
	@$(MAKE) dockerfiles


.PHONY: build
build:
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		(cd services/$$svc && go build -o ../../bin/$$svc ./cmd) || exit 1; \
	done


.PHONY: test
test:
	@echo "Testing pkg..." && (cd pkg && go test ./...) || exit 1
	@for svc in $(SERVICES); do \
		echo "Testing $$svc..."; \
		(cd services/$$svc && go test ./...) || exit 1; \
	done


.PHONY: lint
lint:
	@echo "Linting pkg..." && (cd pkg && golangci-lint run ./...) || exit 1
	@for svc in $(SERVICES); do \
		echo "Linting $$svc..."; \
		(cd services/$$svc && golangci-lint run ./...) || exit 1; \
	done


.PHONY: infra-up
infra-up:
	docker compose -f infrastructure/docker-compose.yml $(PROFILES) up -d


.PHONY: infra-down
infra-down:
	COMPOSE_PROFILES='*' docker compose -f infrastructure/docker-compose.yml down


# core + the chosen groups, and stops groups left running from an earlier run;
# start is the same without building images (containers keep their data either way)
.PHONY: up start
up: BUILD = --build
up start:
	$(GROUP_ENV) $(COMPOSE) $(PROFILES) up -d $(BUILD)
	@on=$$($(COMPOSE) $(PROFILES) config --services); \
	off=$$(COMPOSE_PROFILES='*' $(COMPOSE) config --services | grep -vxF "$$on"); \
	if [ -n "$$off" ]; then COMPOSE_PROFILES='*' $(COMPOSE) stop $$off; fi


# stop, down and down-clean act on every group, whatever ran
.PHONY: stop
stop:
	COMPOSE_PROFILES='*' $(COMPOSE) stop


.PHONY: down
down:
	COMPOSE_PROFILES='*' $(COMPOSE) down


.PHONY: down-clean
down-clean:
	COMPOSE_PROFILES='*' $(COMPOSE) down -v


.PHONY: demo
demo:
	@bash scripts/demo.sh


DURATION ?= 120
WORKERS ?= 4

.PHONY: load
load:
	@bash scripts/load.sh $(DURATION) $(WORKERS)

# parked messages: make dlq; make dlq CMD="replay --topic like.created"; CMD="skip"
CMD ?= list
.PHONY: dlq
dlq:
	@cd tools/dlq && go run . $(CMD)


.PHONY: ui
ui:
	kubectl port-forward service/gateway-service 8080:8080 &
	cd ui && npm run dev


# Full fresh start: wipe volumes, rebuild (services apply their own migrations on startup)
.PHONY: fresh
fresh: down-clean up
	@echo ""
	@echo "System starting! Run 'make demo' once services are healthy."


# Wipe all data and restart (no demo)
.PHONY: reset
reset: down-clean up
	@echo "All data wiped and services restarted."


.PHONY: dockerfiles
dockerfiles:
	@bash scripts/gen-dockerfiles.sh


# local k3d cluster synced by Argo CD from the gitops repo (bootstrap/root-local.yaml)
# root-local waits for the repo server, else it stalls on a comparison error until the next 3-min refresh
ARGOCD_VERSION ?= v3.5.3
GITOPS_RAW = https://raw.githubusercontent.com/vahan-sahakyan/distributed-social-network-gitops/main
# the machine's CA for https on the local cluster; it outlives clusters, so it is trusted once
LOCAL_CA = $(HOME)/.config/dsn/local-ca

.PHONY: cluster-up
cluster-up: local-ca
	k3d cluster create dsn -p "8081:80@loadbalancer" -p "8443:443@loadbalancer"
	kubectl create namespace cert-manager
	kubectl -n cert-manager create secret tls dsn-local-ca --cert=$(LOCAL_CA).crt --key=$(LOCAL_CA).key
	# openbao: a fresh unseal key per cluster, and the public dev values its first start seeds from
	kubectl create namespace openbao
	openssl rand 32 | kubectl -n openbao create secret generic openbao-unseal --from-file=key=/dev/stdin
	curl -fsSL $(GITOPS_RAW)/envs/local/openbao-seed.env | kubectl -n openbao create secret generic openbao-seed --from-env-file=/dev/stdin
	kubectl create namespace argocd
	kubectl apply -n argocd --server-side -f https://raw.githubusercontent.com/argoproj/argo-cd/$(ARGOCD_VERSION)/manifests/install.yaml
	kubectl wait --for=condition=Established crd/applications.argoproj.io --timeout=60s
	kubectl -n argocd rollout status deploy/argocd-repo-server --timeout=180s
	kubectl apply -f $(GITOPS_RAW)/bootstrap/root-local.yaml
	@$(MAKE) --no-print-directory cluster-profile
	@echo ""
	@echo "Argo CD is syncing, the app comes up on https://localhost:8443 in a few minutes (make forward for the Argo CD UI)"
	@echo "Browsers trust it after make trust-ca (once per machine)"

# switches the optional groups of the cluster: make cluster-profile OBS=1 SEARCH=1 (unset = off).
# They live on the hand-applied root-local app, so no commit; Argo adds and prunes the
# groups, and the volumes of stateful ones are kept for when they come back
bool = $(if $(call on,$(1)),true,false)
.PHONY: cluster-profile
cluster-profile:
	kubectl -n argocd patch application root-local --type merge -p '{"spec":{"source":{"helm":{"valuesObject":{"obs":$(call bool,OBS),"tools":$(call bool,TOOLS),"search":$(call bool,SEARCH),"events":$(call bool,EVENTS)}}}}}'

.PHONY: local-ca
local-ca: $(LOCAL_CA).crt

$(LOCAL_CA).crt:
	@mkdir -p $(dir $(LOCAL_CA))
	openssl req -x509 -newkey rsa:2048 -nodes -days 3650 -subj "/CN=DSN local CA" \
		-addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" \
		-keyout $(LOCAL_CA).key -out $(LOCAL_CA).crt
	@chmod 600 $(LOCAL_CA).key

# macOS: adds the local CA to the system keychain (asks for your password)
.PHONY: trust-ca
trust-ca: local-ca
	sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain $(LOCAL_CA).crt


.PHONY: cluster-down
cluster-down:
	k3d cluster delete dsn


# the cluster on the same localhost ports compose publishes, plus Argo CD: <namespace>/<kind>/<name>=<ports>
FORWARDS = \
	argocd/svc/argocd-server=9443:443 \
	dsn/svc/gateway-service=8080 \
	dsn/svc/posts-service=9081 \
	dsn/svc/feed-service=9082 \
	dsn/svc/comments-service=9083 \
	dsn/svc/likes-service=9084 \
	dsn/svc/users-service=9085 \
	dsn/svc/media-service=9086 \
	dsn/svc/notification-service=9087 \
	dsn/svc/cache-rebuilder-service=9089 \
	dsn/svc/search-service=9091 \
	dsn/svc/keycloak=8180:8080 \
	dsn/svc/comments-db=5433:5432 \
	dsn/svc/likes-db=5434:5432 \
	dsn/svc/users-db=5436:5432 \
	dsn/svc/notifications-db=5437:5432 \
	dsn/svc/posts-db=9042 \
	dsn/svc/clickhouse=8123,9009:9000 \
	dsn/svc/redpanda=9644 \
	dsn/svc/redpanda-console=8888:8080 \
	dsn/svc/elasticsearch=9200 \
	dsn/svc/valkey=6379 \
	dsn/svc/minio=9000,9001 \
	dsn/svc/prometheus=9090 \
	dsn/svc/grafana=3000 \
	dsn/svc/loki=3100 \
	dsn/deploy/alloy=12345 \
	dsn/svc/jaeger=16686,4317,4318 \
	dsn/svc/kibana=5601

# skips what isn't deployed (kibana is optional); ctrl-c stops all; waits out a fresh cluster-up (argo server starting, password not yet generated)
.PHONY: forward
forward:
	@kubectl -n argocd rollout status deploy/argocd-server --timeout=180s >/dev/null
	@kubectl -n argocd wait --for=create secret/argocd-initial-admin-secret --timeout=60s >/dev/null
	@echo "Argo CD           https://localhost:9443  admin / $$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d)"
	@echo "Gateway API       http://localhost:8080"
	@echo "Keycloak          http://localhost:8180/auth  (admin / admin)"
	@echo "Grafana           http://localhost:3000  (admin / admin)"
	@echo "Prometheus        http://localhost:9090"
	@echo "Jaeger            http://localhost:16686"
	@echo "Redpanda Console  http://localhost:8888"
	@echo "MinIO Console     http://localhost:9001  (minioadmin / minioadmin)"
	@echo "Elasticsearch     http://localhost:9200"
	@echo "Kibana            http://localhost:5601  (if enabled)"
	@echo "gRPC              localhost:9081-9091"
	@echo "Postgres          localhost:5433 comments, 5434 likes, 5436 users, 5437 notifications  (postgres / postgres)"
	@echo "Scylla, ClickHouse, Loki, Alloy, MinIO S3, Valkey, Redpanda admin: their compose ports, see the README"
	@for f in $(FORWARDS); do \
		target=$${f%%=*}; \
		kubectl -n $${target%%/*} get $${target#*/} >/dev/null 2>&1 || continue; \
		kubectl -n $${target%%/*} port-forward $${target#*/} $$(echo $${f#*=} | tr , ' ') >/dev/null & \
	done; \
	wait


.PHONY: tidy
tidy:
	@for svc in $(SERVICES); do \
		echo "Tidying $$svc..."; \
		(cd services/$$svc && go mod tidy) || exit 1; \
	done
	cd pkg && go mod tidy
