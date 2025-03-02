.PHONY: build test lint run-api run-worker clean test-integration test-load

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet

# Binary names
API_BINARY=titan-api
WORKER_BINARY=titan-worker

# Build directories
BUILD_DIR=bin

# Build flags
LDFLAGS=-ldflags "-s -w"

## build: Build both API and Worker binaries
build: build-api build-worker

## build-api: Build the API server binary
build-api:
	@echo "Building API server..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(API_BINARY) ./cmd/api

## build-worker: Build the Worker binary
build-worker:
	@echo "Building Worker..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(WORKER_BINARY) ./cmd/worker

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

## test-integration: Run integration tests (requires Docker)
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration -timeout=10m ./tests/integration/...

## test-load: Run load tests (requires k6)
test-load:
	@echo "Running load tests..."
	@mkdir -p tests/load/results
	@if command -v k6 >/dev/null 2>&1; then \
		k6 run tests/load/sustained.js; \
	else \
		echo "k6 not installed. Install from https://k6.io"; \
		exit 1; \
	fi

## lint: Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Running go vet instead..."; \
		$(GOVET) ./...; \
	fi

## run-api: Run the API server
run-api:
	@echo "Running API server..."
	$(GOCMD) run ./cmd/api

## run-worker: Run the Worker
run-worker:
	@echo "Running Worker..."
	$(GOCMD) run ./cmd/worker

## docker-up: Start all services with Docker Compose
docker-up:
	@echo "Starting services..."
	cd deployments && docker-compose up -d

## docker-down: Stop all services
docker-down:
	@echo "Stopping services..."
	cd deployments && docker-compose down

## docker-logs: View logs
docker-logs:
	cd deployments && docker-compose logs -f

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf tests/load/results

## tidy: Tidy and verify dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	$(GOMOD) verify

## migrate: Run database migrations
migrate:
	@echo "Running migrations..."
	@if command -v migrate >/dev/null 2>&1; then \
		migrate -path internal/database/migrations -database "$$TITAN_POSTGRES_DSN" up; \
	else \
		echo "golang-migrate not installed"; \
		exit 1; \
	fi

## help: Display this help message
help:
	@echo "Titan - Distributed Job Queue System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/ /'
