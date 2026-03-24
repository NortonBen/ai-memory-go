# Embedding Provider Dependencies

This document describes the embedding provider dependencies added for the AI Memory Integration project.

## Added Dependencies

### 1. OpenAI Embeddings
- **Package**: `github.com/sashabaranov/go-openai v1.41.2`
- **Purpose**: Official Go SDK for OpenAI API, providing type-safe access to embedding models
- **Models Supported**: 
  - text-embedding-ada-002 (1536 dimensions)
  - text-embedding-3-small (1536 dimensions)
  - text-embedding-3-large (3072 dimensions)
- **Implementation**: See `vector/openai_embedder.go`

### 2. Ollama Embeddings
- **Package**: No external SDK (uses standard HTTP client)
- **Purpose**: Local embedding generation using Ollama's API
- **Endpoint**: `http://localhost:11434` (default)
- **Implementation**: See `extractor/ollama.go`
- **Note**: Ollama's official SDK requires Go 1.24+, so we use direct HTTP calls to maintain Go 1.23 compatibility

### 3. DeepSeek Embeddings
- **Package**: No external SDK (uses standard HTTP client)
- **Purpose**: DeepSeek API for embeddings and structured extraction
- **Endpoint**: `https://api.deepseek.com/v1/chat/completions`
- **Implementation**: See `extractor/deepseek.go`
- **Note**: DeepSeek follows OpenAI's API format, so no additional SDK is required

## Go Version Compatibility

- **Minimum Go Version**: 1.23 (as specified in requirements)
- **Toolchain Used**: go1.24.1 (for development)
- **Compatibility**: All dependencies are compatible with Go 1.23+

## Verification

To verify dependencies are properly installed:

```bash
# Download all dependencies
go mod download

# List embedding provider dependencies
go list -m github.com/sashabaranov/go-openai

# Build the vector package
go build ./vector
```

## Usage

The embedding providers are integrated through the `AutoEmbedder` system in `vector/embedder.go`, which provides:
- Automatic provider selection
- Fallback support
- Caching
- Batch embedding generation

Example:
```go
import "github.com/NortonBen/ai-memory-go/vector"

// Create AutoEmbedder with OpenAI as primary provider
embedder := vector.NewAutoEmbedder("openai", cache)
embedder.AddProvider("openai", vector.NewOpenAIEmbeddingProvider(apiKey, "text-embedding-3-small"))

// Generate embedding
embedding, err := embedder.GenerateEmbedding(ctx, "Hello, world!")
```
