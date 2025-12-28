.PHONY: all build run test clean deps docker-up docker-down docker-logs docker-build \
        nats-up nats-down k6-test k6-build init-accounts lint fmt help \
        observability-up observability-down observability-logs

# Variables
BINARY_NAME=wallet-service
GO=go
DOCKER_COMPOSE=docker compose
K6=./bin/k6
XK6=xk6

# Default target
all: deps build

# Install dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# Build the binary
build:
	$(GO) build -o bin/$(BINARY_NAME) ./cmd/server

# Run locally (requires NATS to be running)
run: build
	mkdir -p data
	./bin/$(BINARY_NAME) -gin-mode=debug

# Run with live reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Run tests
test:
	$(GO) test -v -race ./...

# Run tests with coverage
test-cover:
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf data/
	rm -f coverage.out coverage.html

# Lint code
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GO) fmt ./...

#
# Docker commands
#

# Start all services with Docker Compose
docker-up:
	$(DOCKER_COMPOSE) up -d

# Stop all services
docker-down:
	$(DOCKER_COMPOSE) down

# View logs
docker-logs:
	$(DOCKER_COMPOSE) logs -f

# Build Docker image
docker-build:
	$(DOCKER_COMPOSE) build

# Rebuild and restart services
docker-rebuild: docker-build
	$(DOCKER_COMPOSE) up -d

# Remove all containers and volumes
docker-clean:
	$(DOCKER_COMPOSE) down -v --remove-orphans

#
# NATS only (for local development)
#

# Start NATS only
nats-up:
	$(DOCKER_COMPOSE) up -d nats

# Stop NATS
nats-down:
	$(DOCKER_COMPOSE) stop nats

#
# Observability Stack
#

# Start observability stack (Prometheus, Grafana, Tempo, Loki)
observability-up:
	$(DOCKER_COMPOSE) up -d prometheus grafana tempo loki otel-collector
	@echo ""
	@echo "Observability stack started!"
	@echo "  Grafana:    http://localhost:3000 (admin/admin)"
	@echo "  Prometheus: http://localhost:9091"
	@echo "  Tempo:      http://localhost:3200"
	@echo "  Loki:       http://localhost:3100"

# Stop observability stack
observability-down:
	$(DOCKER_COMPOSE) stop prometheus grafana tempo loki otel-collector

# View observability logs
observability-logs:
	$(DOCKER_COMPOSE) logs -f prometheus grafana tempo loki otel-collector

# Check metrics endpoint
metrics-check:
	@echo "=== Application Metrics ==="
	@curl -s http://localhost:9090/metrics | head -50
	@echo ""
	@echo "=== Prometheus Targets ==="
	@curl -s http://localhost:9091/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'

#
# xk6 Build (custom k6 with UUID extension)
#

# Install xk6 tool
xk6-install:
	$(GO) install go.k6.io/xk6/cmd/xk6@latest

# Build custom k6 with UUID extension
k6-build: xk6-install
	@echo "Building custom k6 with UUID extension..."
	cd xk6-uuid && $(GO) mod tidy
	$(XK6) build --with github.com/nathanyu/xk6-uuid=./xk6-uuid --output bin/k6
	@echo "Custom k6 built at bin/k6"

# Build k6 using Docker (no local Go required)
k6-build-docker:
	@echo "Building custom k6 with Docker..."
	docker run --rm -v $(PWD):/workspace -w /workspace \
		grafana/xk6:latest build \
		--with github.com/nathanyu/xk6-uuid=/workspace/xk6-uuid \
		--output /workspace/bin/k6
	@echo "Custom k6 built at bin/k6"

#
# Testing
#

# Initialize test accounts (run after service is up)
init-accounts:
	@echo "Initializing test accounts..."
	@curl -s -X POST http://localhost:8080/v1/wallet/init \
		-H "Content-Type: application/json" \
		-d '{"account": "alice", "balance": 100000}' | jq .
	@curl -s -X POST http://localhost:8080/v1/wallet/init \
		-H "Content-Type: application/json" \
		-d '{"account": "bob", "balance": 100000}' | jq .
	@curl -s -X POST http://localhost:8080/v1/wallet/init \
		-H "Content-Type: application/json" \
		-d '{"account": "charlie", "balance": 100000}' | jq .
	@echo "Done!"

# Run k6 load test
k6-test:
	$(K6) run scripts/k6-load-test.js

# Run k6 stress test
k6-stress:
	$(K6) run scripts/k6-stress-test.js

# Quick API test
api-test:
	@echo "=== Health Check ==="
	@curl -s http://localhost:8080/health | jq .
	@echo "\n=== Get All Balances ==="
	@curl -s http://localhost:8080/v1/wallet/balances | jq .
	@echo "\n=== Transfer Test ==="
	@curl -s -X POST http://localhost:8080/v1/wallet/transfer \
		-H "Content-Type: application/json" \
		-d '{"from_account": "alice", "to_account": "bob", "amount": 100}' | jq .
	@echo "\n=== Get Alice Balance ==="
	@curl -s http://localhost:8080/v1/wallet/balance/alice | jq .
	@echo "\n=== Get Bob Balance ==="
	@curl -s http://localhost:8080/v1/wallet/balance/bob | jq .

# Run integration tests
integration-test:
	$(GO) test -v -tags=integration ./test/...

#
# Development workflow
#

# Full development setup
setup: deps nats-up
	@echo "Development environment ready!"
	@echo "Run 'make run' to start the service"

# Quick restart (rebuild and run)
restart: build
	@pkill -f $(BINARY_NAME) || true
	./bin/$(BINARY_NAME) -gin-mode=debug &

# Help
help:
	@echo "Digital Wallet Service - Available Commands:"
	@echo ""
	@echo "Development:"
	@echo "  make deps          - Install Go dependencies"
	@echo "  make build         - Build the binary"
	@echo "  make run           - Build and run locally"
	@echo "  make dev           - Run with live reload (requires air)"
	@echo "  make test          - Run unit tests"
	@echo "  make test-cover    - Run tests with coverage"
	@echo "  make lint          - Run linter"
	@echo "  make fmt           - Format code"
	@echo "  make clean         - Remove build artifacts"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up     - Start all services"
	@echo "  make docker-down   - Stop all services"
	@echo "  make docker-logs   - View service logs"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-clean  - Remove containers and volumes"
	@echo ""
	@echo "NATS (local dev):"
	@echo "  make nats-up       - Start NATS only"
	@echo "  make nats-down     - Stop NATS"
	@echo ""
	@echo "Observability:"
	@echo "  make observability-up   - Start Prometheus, Grafana, Tempo, Loki"
	@echo "  make observability-down - Stop observability stack"
	@echo "  make observability-logs - View observability logs"
	@echo "  make metrics-check      - Check metrics endpoint"
	@echo ""
	@echo "Testing:"
	@echo "  make init-accounts - Initialize test accounts"
	@echo "  make api-test      - Quick API smoke test"
	@echo "  make k6-build      - Build custom k6 with UUID extension"
	@echo "  make k6-build-docker - Build k6 using Docker"
	@echo "  make k6-test       - Run k6 load test"
	@echo "  make k6-stress     - Run k6 stress test"
	@echo ""
	@echo "Quick Start:"
	@echo "  make setup         - Full development setup"
	@echo "  make k6-build && make k6-test - Build k6 and run load test"
