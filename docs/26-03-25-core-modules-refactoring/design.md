# Design - Core Modules Refactoring

## Proposed Directory Structure

### 1. Vector Module (`vector/`)
To separate core interfaces, query logic, and specific provider implementations.

```
vector/
├── adapters/            # Vector store implementations (VectorStore interface)
│   ├── pgvector/
│   │   ├── pgvector.go
│   │   └── pgvector_test.go
│   ├── qdrant/
│   │   ├── qdrant_adapter.go
│   │   └── qdrant_adapter_test.go
│   ├── sqlite/
│   │   ├── sqlite_store.go
│   │   └── sqlite_store_test.go
│   ├── redis/
│   │   └── redis_store.go
│   └── inmemory/
│       ├── inmemory_store.go
│       └── inmemory_store_test.go
├── embedders/           # Embedding providers (EmbeddingProvider interface)
│   ├── openai/
│   │   └── openai_embedder.go
│   ├── ollama/
│   │   └── ollama_embedder.go
│   ├── openrouter/
│   │   └── openrouter_embedder.go
│   └── lmstudio/
│       └── lmstudio_embedder.go
├── core/                # Core logic (optional, or keep at root)
│   ├── query_vectorizer.go
│   └── query_vectorizer_test.go
├── vector.go            # Root interfaces and configs (package vector)
├── factory.go           # Updated to import sub-packages
├── providers.go
└── docs/                # Moved .md files here
```

### 2. Storage Module (`storage/`)
To separate relational database adapters and shared utilities.

```
storage/
├── adapters/
│   ├── postgresql/
│   │   └── postgresql_adapter.go
│   └── sqlite/
│       └── sqlite_adapter.go
├── utils/               # Shared utilities
│   ├── connection_pool.go
│   ├── health_monitor.go
│   └── ...
├── storage.go           # Core interfaces
├── connections.go
├── config_factory.go
└── docs/                # Moved .md files here
```

### 3. Graph Module (`graph/`)
To separate graph database adapters.

```
graph/
├── adapters/
│   ├── neo4j/
│   │   ├── neo4j_adapter.go
│   │   ├── neo4j_batch.go
│   │   └── ...
│   ├── sqlite/
│   │   └── sqlite_adapter.go
│   ├── redis/
│   │   └── redis_store.go
│   └── inmemory/
│       └── inmemory.go
├── graph.go             # Core interfaces
└── factory.go           # Updated to import sub-packages
```

## Implementation Details

### Package Naming
- Each sub-directory will become a new Go package.
- Example: `vector/adapters/pgvector/pgvector.go` will be `package pgvector`.
- This requires updating all imports: `import "github.com/.../vector/adapters/pgvector"`.

### Dependency Injection (Factory)
- The `factory.go` in each root directory (vector, storage, graph) will serve as the entry point.
- It will import all sub-packages to instantiate the requested implementation based on configuration.
- Note: This might introduce many imports in the factory, which is acceptable for a factory pattern.

### Migration Strategy
1. Create new directories.
2. Move files one by one (or in logical groups).
3. Update package declarations in moved files.
4. Update `factory.go` and core files.
5. Use `grep` or `sed` to update imports across the rest of the codebase (e.g., in `engine/`, `cmd/`).
6. Run `go mod tidy`.
7. Run tests.
