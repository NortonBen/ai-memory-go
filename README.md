# AI Memory Integration - Go Library

[![CI](https://github.com/NortonBen/ai-memory-go/actions/workflows/ci.yml/badge.svg)](https://github.com/NortonBen/ai-memory-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/NortonBen/ai-memory-go)](https://goreportcard.com/report/github.com/NortonBen/ai-memory-go)
[![codecov](https://codecov.io/gh/NortonBen/ai-memory-go/branch/main/graph/badge.svg)](https://codecov.io/gh/NortonBen/ai-memory-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/NortonBen/ai-memory-go.svg)](https://pkg.go.dev/github.com/NortonBen/ai-memory-go)

A Go-native AI Memory library inspired by Cognee's architecture, providing persistent knowledge graph memory for AI agents.

## Overview

This library implements a Data-Driven Pipeline using Go's concurrency features for high-performance parallel processing. It provides four core operations (Add, Cognify, Memify, Search) and supports multiple LLM providers, pluggable storage backends, and seamless integration with Wails desktop applications and Go AI services.

## Module Information

- **Module**: `github.com/NortonBen/ai-memory-go`
- **Go Version**: 1.25.0
- **Status**: In Development (Phase 1: Core Foundation)

## Core Features

- **Data-Driven Pipeline**: Add → Cognify → Memify → Search pipeline
- **Multi-Provider LLM Support**: OpenAI, Anthropic, Gemini, Ollama, DeepSeek, Mistral, Bedrock
- **Hybrid Storage**: Graph databases, vector stores, and relational databases
- **Go-Native**: Pure Go implementation with idiomatic interfaces
- **Production-Ready**: Built for high-throughput AI services

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/NortonBen/ai-memory-go"
)

func main() {
    fmt.Println("AI Memory Integration - Go Library")
    // More functionality coming soon...
}
```

## Development

### Running Tests

```bash
# Run all tests
make test

# Run tests with race detection
make test-race

# Generate coverage report
make coverage
```

### Code Quality

```bash
# Run linter
make lint

# Format code
make fmt

# Run security scan
make security
```

### Building

```bash
# Build the project
make build

# Run all CI checks locally
make ci
```

For more commands, run `make help`.

## Development Status

This project is currently in **Phase 1: Core Foundation**. The following tasks are being implemented:

- [x] Task 1.1.1: Create main module `github.com/NortonBen/ai-memory-go`
- [ ] Task 1.1.2: Set up package structure
- [ ] Task 1.1.3: Configure Go modules and dependencies
- [x] Task 1.1.4: Set up CI/CD pipeline with GitHub Actions

## Architecture

The library follows a layered architecture based on Data-Driven Pipeline principles:

```
Memory API Layer (Add, Cognify, Memify, Search)
├── Data-Driven Pipeline Core
├── Package Structure (parser, schema, extractor, graph, vector, storage)
└── Storage Layer (Graph, Vector, Relational)
```

## License

[License information to be added]

## Contributing

[Contributing guidelines to be added]