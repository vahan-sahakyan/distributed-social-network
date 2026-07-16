SERVICES = gateway-service posts-service comments-service likes-service feed-service users-service media-service notification-service event-writer-service cache-rebuilder-service

.PHONY: build run stop test lint up down infra-up infra-down migrate demo fresh tidy proto ui

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

build:
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		cd services/$$svc && go build -o ../../bin/$$svc ./cmd && cd ../..; \
	done

test:
	@for svc in $(SERVICES); do \
		echo "Testing $$svc..."; \
		cd services/$$svc && go test ./... && cd ../..; \
	done

lint:
	@for svc in $(SERVICES); do \
		echo "Linting $$svc..."; \
		cd services/$$svc && golangci-lint run ./... && cd ../..; \
	done

infra-up:
	docker compose -f infrastructure/docker-compose.yml up -d

infra-down:
	docker compose -f infrastructure/docker-compose.yml down

up:
	docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml up -d --build

down:
	docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml down

down-clean:
	docker compose -f infrastructure/docker-compose.yml -f infrastructure/docker-compose.services.yml down -v

migrate:
	@bash scripts/migrate.sh

demo:
	@bash scripts/demo.sh

ui:
	cd ui && npm run dev

# Full fresh start: wipe volumes, rebuild, migrate, demo
fresh: down-clean up
	@echo "Waiting for infrastructure to initialize..."
	@sleep 10
	@$(MAKE) migrate
	@echo ""
	@echo "System ready! Run 'make demo' to exercise all services."

# Wipe all data and restart (no demo)
reset: down-clean up
	@echo "Waiting for infrastructure to initialize..."
	@sleep 10
	@$(MAKE) migrate
	@echo "All data wiped and services restarted."

tidy:
	@for svc in $(SERVICES); do \
		echo "Tidying $$svc..."; \
		cd services/$$svc && go mod tidy && cd ../..; \
	done
	cd pkg && go mod tidy
