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

# Export DeBERTa NER models (chọn size theo nhu cầu RAM)
# ─────────────────────────────────────────────────────────────────────────
# base (~400 MB, ~0.7 GB RSS) — khuyến nghị cho development
export-deberta-base:
	source .venv/bin/activate && python3 scripts/export_deberta_onnx.py \
		--size base --output data/deberta-ner-base --seq-len 256

# base + INT8 quantize (~100 MB, ~0.2 GB RSS) — nhỏ nhất
export-deberta-base-q:
	source .venv/bin/activate && python3 scripts/export_deberta_onnx.py \
		--size base --quantize --output data/deberta-ner-base-q --seq-len 256

# large (~1.6 GB, ~2.5 GB RSS) — chính xác nhất
export-deberta-large:
	source .venv/bin/activate && python3 scripts/export_deberta_onnx.py \
		--size large --output data/deberta-ner --seq-len 256

# large + INT8 quantize (~400 MB, ~0.6 GB RSS)
export-deberta-large-q:
	source .venv/bin/activate && python3 scripts/export_deberta_onnx.py \
		--size large --quantize --output data/deberta-ner-q --seq-len 256

# Quantize models đã export sang INT8 (không cần download lại)
quantize-deberta:
	source .venv/bin/activate && python3 -c "\
import shutil, os; \
from onnxruntime.quantization import quantize_dynamic, QuantType; \
os.makedirs('data/deberta-ner-q', exist_ok=True); \
quantize_dynamic('data/deberta-ner/model.onnx', 'data/deberta-ner-q/model.onnx', weight_type=QuantType.QInt8); \
[shutil.copy(f'data/deberta-ner/{f}', 'data/deberta-ner-q/') for f in os.listdir('data/deberta-ner') if f != 'model.onnx']; \
print('Done:', round(os.path.getsize('data/deberta-ner-q/model.onnx')/1e6), 'MB')"

quantize-harrier:
	source .venv/bin/activate && python3 -c "\
import shutil, os; \
from onnxruntime.quantization import quantize_dynamic, QuantType; \
os.makedirs('data/harrier-q', exist_ok=True); \
quantize_dynamic('data/harrier/model.onnx', 'data/harrier-q/model.onnx', weight_type=QuantType.QInt8); \
[shutil.copy(f'data/harrier/{f}', 'data/harrier-q/') for f in os.listdir('data/harrier') if f != 'model.onnx']; \
print('Done:', round(os.path.getsize('data/harrier-q/model.onnx')/1e6), 'MB')"

quantize-all: quantize-deberta quantize-harrier

.PHONY: export-deberta-base export-deberta-base-q export-deberta-large export-deberta-large-q quantize-deberta quantize-harrier quantize-all