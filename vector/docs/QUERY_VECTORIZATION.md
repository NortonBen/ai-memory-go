# Query Vectorization

## Overview

Query vectorization is **Step 1** of the 4-step search pipeline in the AI Memory Integration system. It converts search queries into vector embeddings that can be used for semantic similarity search.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    4-Step Search Pipeline                        │
├─────────────────────────────────────────────────────────────────┤
│ Step 1: Input Processing & Vectorization (THIS IMPLEMENTATION)  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ Query Text   │→ │ QueryVectorizer│→│ProcessedQuery│          │
│  │              │  │ + AutoEmbedder │  │ with Vector  │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
├─────────────────────────────────────────────────────────────────┤
│ Step 2: Hybrid Search (Next Task)                               │
│  - Vector similarity search using query vector                   │
│  - Entity-based graph node matching                              │
│  - Anchor node combination                                       │
├─────────────────────────────────────────────────────────────────┤
│ Step 3: Graph Traversal (Future Task)                           │
│  - 1-hop and 2-hop neighbor discovery                            │
│  - Enriched node context assembly                                │
├─────────────────────────────────────────────────────────────────┤
│ Step 4: Context Assembly (Future Task)                          │
│  - Multi-factor reranking                                        │
│  - Rich context building for LLM                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### QueryVectorizer

The main component that handles query vectorization.

```go
type QueryVectorizer struct {
    embedder *AutoEmbedder
}
```

**Key Methods:**
- `VectorizeQuery(ctx, queryText)` - Converts a query to a vector
- `VectorizeProcessedQuery(ctx, queryText)` - Creates a ProcessedQuery with vector
- `VectorizeBatchQueries(ctx, queries)` - Vectorizes multiple queries efficiently
- `GetDimensions()` - Returns embedding dimensions
- `GetModel()` - Returns the model name
- `Health(ctx)` - Checks if the vectorizer is operational

### AutoEmbedder Integration

QueryVectorizer uses the AutoEmbedder system for provider abstraction:

- **Multiple Providers**: OpenAI, Ollama, DeepSeek
- **Automatic Fallback**: If primary provider fails, tries fallback providers
- **Caching**: Embeddings are cached to avoid redundant API calls
- **Batch Processing**: Efficient batch embedding generation

## Usage

### Basic Query Vectorization

```go
// Create AutoEmbedder with provider
cache := NewInMemoryEmbeddingCache()
embedder := NewAutoEmbedder("openai", cache)

openaiProvider := NewOpenAIEmbeddingProvider("your-api-key", "text-embedding-3-small")
embedder.AddProvider("openai", openaiProvider)

// Create query vectorizer
vectorizer := NewQueryVectorizer(embedder)

// Vectorize a query
ctx := context.Background()
vector, err := vectorizer.VectorizeQuery(ctx, "What is present perfect?")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Generated %d-dimensional vector\n", len(vector))
```

### Creating ProcessedQuery

```go
// Create a ProcessedQuery with vector embedding
processedQuery, err := vectorizer.VectorizeProcessedQuery(ctx, "How to use past simple?")
if err != nil {
    log.Fatal(err)
}

// ProcessedQuery is ready for Step 2: Hybrid Search
fmt.Printf("Original: %s\n", processedQuery.OriginalText)
fmt.Printf("Vector: %d dimensions\n", len(processedQuery.Vector))
```

### Batch Vectorization

```go
// Vectorize multiple queries efficiently
queries := []string{
    "What is present perfect?",
    "How to use past simple?",
    "Difference between present and past?",
}

embeddings, err := vectorizer.VectorizeBatchQueries(ctx, queries)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Vectorized %d queries\n", len(embeddings))
```

### Multi-Provider Setup with Fallback

```go
// Create AutoEmbedder with multiple providers
cache := NewInMemoryEmbeddingCache()
embedder := NewAutoEmbedder("openai", cache)

// Add primary provider (OpenAI)
openaiProvider := NewOpenAIEmbeddingProvider("your-openai-key", "text-embedding-3-small")
embedder.AddProvider("openai", openaiProvider)

// Add fallback provider (Ollama for local processing)
ollamaProvider := NewOllamaEmbeddingProvider("http://localhost:11434", "nomic-embed-text")
embedder.AddProvider("ollama", ollamaProvider)
embedder.AddFallback("ollama")

vectorizer := NewQueryVectorizer(embedder)

// If OpenAI fails, automatically tries Ollama
vector, err := vectorizer.VectorizeQuery(ctx, "Test query")
```

## Supported Embedding Providers

### OpenAI
- **Models**: text-embedding-3-small (1536 dim), text-embedding-3-large (3072 dim), text-embedding-ada-002 (1536 dim)
- **Pros**: High quality, widely supported
- **Cons**: Requires API key, costs money

### Ollama
- **Models**: nomic-embed-text, mxbai-embed-large, etc.
- **Pros**: Free, runs locally, no API key needed
- **Cons**: Requires local Ollama installation

### DeepSeek
- **Models**: deepseek-chat with embedding support
- **Pros**: Cost-effective, good quality
- **Cons**: Requires API key

## Error Handling

The QueryVectorizer implements comprehensive error handling:

```go
vector, err := vectorizer.VectorizeQuery(ctx, queryText)
if err != nil {
    // Possible errors:
    // - Empty query text
    // - Embedding generation failure
    // - All providers failed (with fallback)
    // - Context cancellation
    log.Printf("Vectorization failed: %v", err)
}
```

## Caching

Embeddings are automatically cached to improve performance:

```go
// First call - generates embedding
vector1, _ := vectorizer.VectorizeQuery(ctx, "What is present perfect?")

// Second call - uses cached embedding (much faster)
vector2, _ := vectorizer.VectorizeQuery(ctx, "What is present perfect?")

// Vectors are identical
```

Cache features:
- **In-memory caching** with TTL (24 hours default)
- **Automatic cleanup** of expired entries
- **Thread-safe** for concurrent access
- **SHA-256 hashing** for cache keys

## Integration with Search Pipeline

Query vectorization is the first step in the search pipeline:

```go
// Step 1: Input Processing & Vectorization
processedQuery, err := vectorizer.VectorizeProcessedQuery(ctx, queryText)

// Step 2: Hybrid Search (uses processedQuery.Vector)
// - Vector similarity search in vector store
// - Entity-based graph node matching
// - Combine into anchor nodes

// Step 3: Graph Traversal (uses anchor nodes)
// - 1-hop and 2-hop neighbor discovery
// - Build enriched node context

// Step 4: Context Assembly (uses enriched nodes)
// - Multi-factor reranking
// - Build rich context for LLM
```

## Performance Considerations

### Batch Processing
Use `VectorizeBatchQueries` for multiple queries:
- More efficient than individual calls
- Reduces API overhead
- Better throughput

### Caching
- Embeddings are cached automatically
- Repeated queries are instant
- Cache size grows with unique queries

### Provider Selection
- **OpenAI**: Best quality, but costs money
- **Ollama**: Free and local, but requires setup
- **DeepSeek**: Good balance of cost and quality

## Testing

Comprehensive test coverage includes:
- Unit tests for all methods
- Mock providers for testing
- Edge cases (empty queries, failures, etc.)
- Different embedding dimensions
- Caching behavior
- Context cancellation
- Special characters and Unicode
- Large batch processing

Run tests:
```bash
go test -v ./vector -run TestVectorize
```

## Future Enhancements

Potential improvements for future tasks:
- [ ] Query preprocessing (normalization, cleaning)
- [ ] Language detection and handling
- [ ] Query expansion and reformulation
- [ ] Semantic query understanding
- [ ] Integration with entity extraction (Step 2)
- [ ] Query intent classification

## Related Components

- **AutoEmbedder** (`vector/embedder.go`) - Provider abstraction and caching
- **EmbeddingProvider** (`vector/vector.go`) - Provider interface
- **OpenAIEmbeddingProvider** (`vector/openai_embedder.go`) - OpenAI implementation
- **ProcessedQuery** (`schema/schema.go`) - Query structure with vector

## References

- Design Document: `.kiro/specs/ai-memory-integration/design.md`
- Requirements: `.kiro/specs/ai-memory-integration/requirements.md`
- Tasks: `.kiro/specs/ai-memory-integration/tasks.md` (Task 11.1.1)
