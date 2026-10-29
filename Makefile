.PHONY: all build test clean run-upstream run-gateway benchmark

BINARY_DIR=bin
GO=go

all: test build

build:
	@echo "Building binaries..."
	@mkdir -p $(BINARY_DIR)
	$(GO) build -v -o $(BINARY_DIR)/flowgate ./cmd/flowgate
	$(GO) build -v -o $(BINARY_DIR)/upstream ./cmd/upstream
	$(GO) build -v -o $(BINARY_DIR)/loadgen ./cmd/loadgen

test:
	@echo "Running unit tests with race detector..."
	$(GO) test -v -race -cover ./...

run-upstream:
	$(GO) run ./cmd/upstream -port 9090 -latency 15 -capacity 50

run-gateway:
	$(GO) run ./cmd/flowgate -port 8080 -upstream http://127.0.0.1:9090 -initial-limit 50

benchmark:
	$(GO) run ./cmd/loadgen -url http://127.0.0.1:8080/work -concurrency 500 -duration 10s

clean:
	@echo "Cleaning up..."
	@rm -rf $(BINARY_DIR) coverage.txt
