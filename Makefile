SERVICES = gateway-service posts-service comments-service likes-service feed-service users-service media-service notification-service event-writer-service cache-rebuilder-service search-service

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
	docker compose -f infrastructure/docker-compose.yml up -d


.PHONY: infra-down
infra-down:
	docker compose -f infrastructure/docker-compose.yml down


.PHONY: up
up:
	docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml up -d --build


.PHONY: down
down:
	docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml down


# stop/start keep containers, so start is instant and nothing is rebuilt
.PHONY: stop
stop:
	docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml stop


.PHONY: start
start:
	docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml start


.PHONY: down-clean
down-clean:
	docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml down -v


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


# Kibana for poking at the search indices (Dev Tools), not started by make up
.PHONY: kibana
kibana:
	docker compose -f infrastructure/docker-compose.yml --profile kibana up -d kibana
	@echo "Kibana: http://localhost:5601/app/dev_tools#/console"


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

.PHONY: cluster-up
cluster-up:
	k3d cluster create dsn -p "8081:80@loadbalancer"
	kubectl create namespace argocd
	kubectl apply -n argocd --server-side -f https://raw.githubusercontent.com/argoproj/argo-cd/$(ARGOCD_VERSION)/manifests/install.yaml
	kubectl wait --for=condition=Established crd/applications.argoproj.io --timeout=60s
	kubectl -n argocd rollout status deploy/argocd-repo-server --timeout=180s
	kubectl apply -f $(GITOPS_RAW)/bootstrap/root-local.yaml
	@echo ""
	@echo "Argo CD is syncing, the app comes up on http://localhost:8081 in a few minutes (make argocd-ui to watch)"


.PHONY: cluster-down
cluster-down:
	k3d cluster delete dsn


# waits out a fresh cluster-up: the server still starting, the password not yet generated
.PHONY: argocd-ui
argocd-ui:
	@kubectl -n argocd rollout status deploy/argocd-server --timeout=180s
	@kubectl -n argocd wait --for=create secret/argocd-initial-admin-secret --timeout=60s >/dev/null
	@echo "https://localhost:8443  admin / $$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d)"
	kubectl -n argocd port-forward svc/argocd-server 8443:443


# cluster databases on the compose ports, so the same DataGrip connections work; ctrl-c stops all
.PHONY: db-forward
db-forward:
	@kubectl -n dsn port-forward svc/comments-db 5433:5432 & \
	kubectl -n dsn port-forward svc/likes-db 5434:5432 & \
	kubectl -n dsn port-forward svc/users-db 5436:5432 & \
	kubectl -n dsn port-forward svc/notifications-db 5437:5432 & \
	kubectl -n dsn port-forward svc/posts-db 9042:9042 & \
	kubectl -n dsn port-forward svc/clickhouse 8123:8123 9009:9000 & \
	wait


.PHONY: tidy
tidy:
	@for svc in $(SERVICES); do \
		echo "Tidying $$svc..."; \
		(cd services/$$svc && go mod tidy) || exit 1; \
	done
	cd pkg && go mod tidy
