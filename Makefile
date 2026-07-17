SERVICES = gateway-service posts-service comments-service likes-service feed-service users-service media-service notification-service event-writer-service cache-rebuilder-service

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
		proto/cache_rebuilder/cache_rebuilder.proto
	@echo "gRPC code generation complete."
	@$(MAKE) dockerfiles


.PHONY: build
build:
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		cd services/$$svc && go build -o ../../bin/$$svc ./cmd && cd ../..; \
	done


.PHONY: test
test:
	@for svc in $(SERVICES); do \
		echo "Testing $$svc..."; \
		cd services/$$svc && go test ./... && cd ../..; \
	done


.PHONY: lint
lint:
	@for svc in $(SERVICES); do \
		echo "Linting $$svc..."; \
		cd services/$$svc && golangci-lint run ./... && cd ../..; \
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


.PHONY: down-clean
down-clean:
	docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml down -v


.PHONY: demo
demo:
	@bash scripts/demo.sh


.PHONY: ui
ui:
	kubectl port-forward service/gateway-service 8080:8080 &
	cd ui && npm run dev


# Full fresh start: wipe volumes, rebuild, migrate, demo
.PHONY: fresh
fresh: down-clean up
	@echo "Waiting for infrastructure to initialize..."
	@sleep 10
	@$(MAKE) migrate
	@echo ""
	@echo "System ready! Run 'make demo' to exercise all services."


# Wipe all data and restart (no demo)
.PHONY: reset
reset: down-clean up
	@echo "Waiting for infrastructure to initialize..."
	@sleep 10
	@$(MAKE) migrate
	@echo "All data wiped and services restarted."


.PHONY: dockerfiles
dockerfiles:
	@bash scripts/gen-dockerfiles.sh


.PHONY: images
images:
	@for svc in $(SERVICES); do \
		echo "Building image $$svc..."; \
		docker build -t distributed-social-network/$$svc:latest -f services/$$svc/Dockerfile . ; \
	done


.PHONY: k3d-load
k3d-load:
	@for svc in $(SERVICES); do \
		echo "Loading $$svc into k3d..."; \
		k3d image import distributed-social-network/$$svc:latest -c dsn; \
	done


.PHONY: k8s-infra-up
k8s-infra-up:
	helm upgrade --install dsn-infra deploy/kubernetes/infra/


.PHONY: k8s-infra-down
k8s-infra-down:
	helm uninstall dsn-infra


.PHONY: k8s-up
k8s-up: k3d-load
	helm upgrade --install dsn-infra deploy/kubernetes/infra/
	helm upgrade --install dsn deploy/kubernetes/services/


.PHONY: k8s-down
k8s-down:
	helm uninstall dsn dsn-infra


.PHONY: tidy
tidy:
	@for svc in $(SERVICES); do \
		echo "Tidying $$svc..."; \
		cd services/$$svc && go mod tidy && cd ../..; \
	done
	cd pkg && go mod tidy
