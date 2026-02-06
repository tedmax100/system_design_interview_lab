.PHONY: all build run test clean deps docker-up docker-down docker-logs docker-build \
        docker-clean docker-rebuild dev smoke-test k6-test fmt lint help

# Variables
BINARY_NAME=exchange-service
GO=go
DOCKER_COMPOSE=docker compose

# Default target
all: deps build

# Install dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# Build the binary
build:
	$(GO) build -o bin/$(BINARY_NAME) ./cmd/server

# Run locally
run: build
	./bin/$(BINARY_NAME)

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
	rm -f coverage.out coverage.html

# Format code
fmt:
	$(GO) fmt ./...

# Lint code
lint:
	golangci-lint run ./...

#
# Docker commands
#

# Start all services
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

# Rebuild and restart
docker-rebuild: docker-build
	$(DOCKER_COMPOSE) up -d

# Remove containers and volumes
docker-clean:
	$(DOCKER_COMPOSE) down -v --remove-orphans

#
# Testing
#

# Initialize test wallets (run after service is up)
init-wallets:
	@echo "Initializing test wallets..."
	@curl -s -X POST http://localhost:8080/v1/wallet/init \
		-H "Content-Type: application/json" \
		-d '{"user_id": "user1", "cash_balance": 10000000, "holdings": {"AAPL": 5000, "GOOG": 3000}}' | jq .
	@curl -s -X POST http://localhost:8080/v1/wallet/init \
		-H "Content-Type: application/json" \
		-d '{"user_id": "user2", "cash_balance": 10000000, "holdings": {"AAPL": 5000, "GOOG": 3000}}' | jq .
	@echo "Done!"

# Run k6 smoke test (simulation scenario)
smoke-test:
	k6 run scripts/k6-smoke-test.js

# Run k6 load test
k6-test:
	k6 run scripts/k6-load-test.js

# Quick API test
api-test:
	@echo "=== Health Check ==="
	@curl -s http://localhost:8080/health | jq .
	@echo "\n=== Place Sell Order ==="
	@curl -s -X POST http://localhost:8080/v1/order \
		-H "Content-Type: application/json" \
		-d '{"symbol": "AAPL", "side": "sell", "price": 10010, "quantity": 1000, "user_id": "user1"}' | jq .
	@echo "\n=== Order Book ==="
	@curl -s "http://localhost:8080/v1/marketdata/orderBook/L2?symbol=AAPL&depth=5" | jq .

# Help
help:
	@echo "Stock Exchange Service - Available Commands:"
	@echo ""
	@echo "Development:"
	@echo "  make deps          - Install Go dependencies"
	@echo "  make build         - Build the binary"
	@echo "  make run           - Build and run locally"
	@echo "  make dev           - Run with live reload (requires air)"
	@echo "  make test          - Run unit tests"
	@echo "  make test-cover    - Run tests with coverage"
	@echo "  make fmt           - Format code"
	@echo "  make lint          - Run linter"
	@echo "  make clean         - Remove build artifacts"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up     - Start all services"
	@echo "  make docker-down   - Stop all services"
	@echo "  make docker-logs   - View service logs"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-clean  - Remove containers and volumes"
	@echo ""
	@echo "Testing:"
	@echo "  make init-wallets  - Initialize test wallets"
	@echo "  make api-test      - Quick API smoke test"
	@echo "  make smoke-test    - Run k6 smoke test (simulation)"
	@echo "  make k6-test       - Run k6 load test"
	@echo ""
	@echo "Quick Start:"
	@echo "  make docker-up && make init-wallets && make smoke-test"
