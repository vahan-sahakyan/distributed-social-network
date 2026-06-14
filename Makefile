SERVICES = gateway-service posts-service comments-service likes-service feed-service users-service media-service notification-service

.PHONY: build run stop test lint

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

tidy:
	@for svc in $(SERVICES); do \
		echo "Tidying $$svc..."; \
		cd services/$$svc && go mod tidy && cd ../..; \
	done
	cd pkg && go mod tidy
