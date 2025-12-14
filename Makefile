# GoEvals Makefile
# Self-documenting Makefile with standard Go project targets

.PHONY: help build test run clean install dev lint fmt check docker-build docker-run

# Default target - show help
.DEFAULT_GOAL := help

help:  ## Show this help message
	@echo "GoEvals - Available commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Windows users: Use 'task.ps1' instead (PowerShell script with same targets)"

build:  ## Build the binary (output: bin/goevals)
	@echo "Building GoEvals..."
	@mkdir -p bin
	go build -o bin/goevals main.go
	@echo "Build complete: bin/goevals"

test:  ## Run all tests with race detector
	@echo "Running tests..."
	go test -v -race -cover ./...

test-short:  ## Run tests without race detector (faster)
	@echo "Running tests (short mode)..."
	go test -v ./...

run:  ## Run with sample data (requires evals.jsonl)
	@echo "Starting GoEvals dashboard..."
	@if [ -f "evals.jsonl" ]; then \
		go run main.go evals.jsonl; \
	else \
		echo "Error: evals.jsonl not found"; \
		echo "Create sample file or specify path: make run ARGS='path/to/evals.jsonl'"; \
		exit 1; \
	fi

run-empty:  ## Run with empty dashboard (no data file)
	@echo "Starting GoEvals with empty dashboard..."
	@touch /tmp/goevals-empty.jsonl
	go run main.go /tmp/goevals-empty.jsonl

clean:  ## Clean build artifacts and temporary files
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f /tmp/goevals-*.jsonl
	@echo "Clean complete"

install:  ## Install binary to GOPATH/bin
	@echo "Installing to GOPATH/bin..."
	go install
	@echo "Installed: $(shell go env GOPATH)/bin/goevals"

dev:  ## Run with auto-reload (requires 'air' and evals.jsonl file)
	@which air > /dev/null || (echo "Error: 'air' not found. Install with: go install github.com/cosmtrek/air@latest" && exit 1)
	@if [ ! -f "evals.jsonl" ]; then \
		echo "Error: evals.jsonl not found. Run 'make run-empty' instead or create sample file."; \
		exit 1; \
	fi
	@echo "Starting development server with hot reload..."
	@echo "Server will run on http://localhost:3000"
	@echo "Edit .air.toml to change args or config"
	air

lint:  ## Run golangci-lint (install from https://golangci-lint.run/usage/install/)
	@which golangci-lint > /dev/null || (echo "Error: golangci-lint not found. See: https://golangci-lint.run/usage/install/" && exit 1)
	@echo "Running linter..."
	golangci-lint run

fmt:  ## Format code and tidy dependencies
	@echo "Formatting code..."
	gofmt -s -w .
	go mod tidy
	@echo "Format complete"

check: fmt lint test  ## Run fmt, lint, and test (full check before commit)
	@echo "All checks passed!"

docker-build:  ## Build Docker image
	@echo "Building Docker image..."
	docker build -t goevals:latest .
	@echo "Docker image built: goevals:latest"

docker-run:  ## Run in Docker (requires evals.jsonl in current directory)
	@echo "Running GoEvals in Docker..."
	@if [ -f "evals.jsonl" ]; then \
		docker run -p 3000:3000 -v $(PWD)/evals.jsonl:/data/evals.jsonl goevals:latest; \
	else \
		echo "Error: evals.jsonl not found in current directory"; \
		exit 1; \
	fi

ci:  ## CI/CD target - runs in GitHub Actions
	@echo "Running CI checks..."
	go mod download
	go mod verify
	go test -v -race -cover ./...
	go build -o bin/goevals main.go
	@echo "CI checks complete"
