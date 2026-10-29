.PHONY: all build test bench clean run-upstream run-gateway benchmark frontend-install frontend-dev frontend-build docker-up docker-down

BINARY_DIR=bin
GO=go

all: test build

build:
	@echo "Building binaries..."
	@mkdir -p $(BINARY_DIR)
	$(GO) build -v -o $(BINARY_DIR)/flowgate ./cmd/flowgate
	$(GO) build -v -o $(BINARY_DIR)/upstream ./cmd/upstream
	$(GO) build -v -o $(BINARY_DIR)/loadgen ./cmd/loadgen

docker-up:
	docker compose up --build

docker-down:
	docker compose down

frontend-install:
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

frontend-dev:
	@echo "Starting frontend dev server..."
	cd frontend && npm run dev

frontend-build:
	@echo "Building frontend for production..."
	cd frontend && npm run build

test:
	@echo "Running unit tests with race detector..."
	$(GO) test -v -race -cover ./cmd/... ./pkg/...

bench:
	@echo "Running performance microbenchmarks..."
	$(GO) test -bench=. -benchmem ./pkg/...

run-upstream:
	$(GO) run ./cmd/upstream -port 9090 -latency 15 -capacity 50

run-gateway:
	$(GO) run ./cmd/flowgate -port 8080 -upstream http://127.0.0.1:9090 -initial-limit 50 -trust-priority=true

benchmark:
	$(GO) run ./cmd/loadgen -url http://127.0.0.1:8080/work -concurrency 500 -duration 10s

clean:
	@echo "Cleaning up..."
	@rm -rf $(BINARY_DIR) coverage.txt
