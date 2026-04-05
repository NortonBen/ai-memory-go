.PHONY: help test lint build clean fmt security coverage install-tools

# Default target
help:
	@echo "AI Memory Integration - Makefile Commands"
	@echo ""
	@echo "Available targets:"
	@echo "  make test          - Run all tests"
	@echo "  make test-race     - Run tests with race detection"
	@echo "  make lint          - Run golangci-lint"
	@echo "  make fmt           - Format code"
	@echo "  make build         - Build the project"
	@echo "  make security      - Run security scan"
	@echo "  make coverage      - Generate coverage report"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make install-tools - Install development tools"
	@echo "  make ci            - Run all CI checks locally"

build-cli:
	go build -o ai-memory-cli cmd/ai-memory-cli/main.go

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -v -race ./...

# Run tests with coverage
coverage:
	@echo "Generating coverage report..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
lint:
	@echo "Running golangci-lint..."
	golangci-lint run --timeout=5m

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Build project
build:
	@echo "Building project..."
	go build -v ./...
	go build -o ai-memory-go -v .

# Run security scan
security:
	@echo "Running security scan..."
	gosec ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f ai-memory-go
	rm -f coverage.out coverage.html
	go clean -cache -testcache

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Tools installed successfully"

# Run all CI checks locally
ci: fmt lint test-race build security
	@echo ""
	@echo "✓ All CI checks passed!"
	@echo ""
	@echo "Your code is ready to push."

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Update dependencies
update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

build-harrier-onnx:
	source .venv/bin/activate && HF_HOME="$(pwd)/data/hf_cache" TRANSFORMERS_CACHE="$(pwd)/data/hf_cache" python3 scripts/export_harrier_onnx.py \
		--model microsoft/harrier-oss-v1-270m \
		--output data/harrier \
		--seq-len 512 2>&1