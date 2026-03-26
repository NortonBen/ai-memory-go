# Implementation Plan - Core Modules Refactoring

Refactor the `vector`, `storage`, and `graph` directories by grouping related files (adapters, embedders, utils) into sub-folders. This improves code clarity and follows a more modular package structure.

## Proposed Changes

### 1. Vector Module (`vector/`)
- [NEW] Move all `VectorStore` implementations to `vector/adapters/<name>/`.
- [NEW] Move all `EmbeddingProvider` implementations to `vector/embedders/<name>/`.
- [MODIFY] `vector/vector.go`: Add a registry for stores and embedders to avoid circular dependencies.
- [MODIFY] `vector/factory.go`: Update to use the registry instead of direct instantiation.

### 2. Storage Module (`storage/`)
- [NEW] Move relational adapters to `storage/adapters/<name>/`.
- [NEW] Move shared utilities to `storage/utils/`.
- [MODIFY] `storage/config_factory.go`: Update to use new sub-packages.

### 3. Graph Module (`graph/`)
- [NEW] Move graph adapters to `graph/adapters/<name>/`.
- [MODIFY] `graph/factory.go`: Update to use new sub-packages.

---

### [Component] Vector Reorganization
- **[MOVE]** `pgvector.go`, `pgvector_test.go` -> `vector/adapters/pgvector/`
- **[MOVE]** `qdrant_adapter.go`, `qdrant_adapter_test.go` -> `vector/adapters/qdrant/`
- **[MOVE]** `sqlite_store.go`, `sqlite_store_test.go` -> `vector/adapters/sqlite/`
- **[MOVE]** `redis_store.go` -> `vector/adapters/redis/`
- **[MOVE]** `inmemory_store.go`, `inmemory_store_test.go` -> `vector/adapters/inmemory/`
- **[MOVE]** `openai_embedder.go` -> `vector/embedders/openai/`
- **[MOVE]** `ollama_embedder.go` -> `vector/embedders/ollama/`
- **[MOVE]** `openrouter_embedder.go` -> `vector/embedders/openrouter/`
- **[MOVE]** `lmstudio_embedder.go` -> `vector/embedders/lmstudio/`

### [Component] Storage Reorganization
- **[MOVE]** `postgresql_adapter.go` -> `storage/adapters/postgresql/`
- **[MOVE]** `sqlite_adapter.go` -> `storage/adapters/sqlite/`
- **[MOVE]** `connection_pool.go`, `health_monitor.go` -> `storage/utils/`

### [Component] Graph Reorganization
- **[MOVE]** `neo4j_adapter.go`, `neo4j_batch.go`, `neo4j_edge.go`, `neo4j_node.go`, `neo4j_query.go`, `neo4j_transaction.go`, `neo4j_adapter_test.go` -> `graph/adapters/neo4j/`
- **[MOVE]** `sqlite_adapter.go`, `sqlite_adapter_test.go` -> `graph/adapters/sqlite/`
- **[MOVE]** `redis_store.go` -> `graph/adapters/redis/`
- **[MOVE]** `inmemory.go` -> `graph/adapters/inmemory/`

## Verification Plan

### Automated Tests
- Run `go build ./...` to ensure all imports and package names are correct.
- Run `go test ./vector/... ./storage/... ./graph/...` to verify existing tests still pass.
- Specific check:
    ```bash
    go test ./vector/adapters/pgvector
    go test ./vector/adapters/qdrant
    go test ./storage/adapters/postgresql
    go test ./graph/adapters/neo4j
    ```

### Manual Verification
- Verify that the directory structure in the IDE matches the proposed design.
- Check a few files (e.g., `pgvector.go`) to ensure the `package` declaration has changed to `package pgvector`.
