# Tasks - Core Modules Refactoring

## Phase 1: Vector Module Reorganization
- [ ] Create `vector/adapters/` and sub-directories: `pgvector`, `qdrant`, `sqlite`, `redis`, `inmemory`.
- [ ] Move store files:
    - `pgvector.go`, `pgvector_test.go` -> `vector/adapters/pgvector/`
    - `qdrant_adapter.go`, `qdrant_adapter_test.go` -> `vector/adapters/qdrant/`
    - `sqlite_store.go`, `sqlite_store_test.go` -> `vector/adapters/sqlite/`
    - `redis_store.go` -> `vector/adapters/redis/`
    - `inmemory_store.go`, `inmemory_store_test.go` -> `vector/adapters/inmemory/`
- [ ] Create `vector/embedders/` and sub-directories: `openai`, `ollama`, `openrouter`, `lmstudio`.
- [ ] Move embedder files:
    - `openai_embedder.go` -> `vector/embedders/openai/`
    - `ollama_embedder.go` -> `vector/embedders/ollama/`
    - `openrouter_embedder.go` -> `vector/embedders/openrouter/`
    - `lmstudio_embedder.go` -> `vector/embedders/lmstudio/`
- [ ] Create `vector/docs/` and move `.md` files.
- [ ] Update package names in all moved files.

## Phase 2: Storage Module Reorganization
- [ ] Create `storage/adapters/` and sub-directories: `postgresql`, `sqlite`.
- [ ] Move adapter files:
    - `postgresql_adapter.go` -> `storage/adapters/postgresql/`
    - `sqlite_adapter.go` -> `storage/adapters/sqlite/`
- [ ] Create `storage/utils/` and move `connection_pool.go`, `health_monitor.go`, and their tests.
- [ ] Create `storage/docs/` and move `.md` files.
- [ ] Update package names.

## Phase 3: Graph Module Reorganization
- [ ] Create `graph/adapters/` and sub-directories: `neo4j`, `sqlite`, `redis`, `inmemory`.
- [ ] Move adapter files:
    - `neo4j_adapter.go`, `neo4j_batch.go`, etc. -> `graph/adapters/neo4j/`
    - `sqlite_adapter.go`, `sqlite_adapter_test.go` -> `graph/adapters/sqlite/`
    - `redis_store.go` -> `graph/adapters/redis/`
    - `inmemory.go` -> `graph/adapters/inmemory/`
- [ ] Update package names.

## Phase 4: Integration & Fixing Imports
- [ ] Update `vector/factory.go`, `storage/config_factory.go`, `graph/factory.go` with new imports.
- [ ] Global search and replace for imports across the project.
- [ ] Run `go mod tidy`.

## Phase 5: Verification
- [ ] Build the project: `go build ./...`
- [ ] Run unit tests: `go test ./vector/... ./storage/... ./graph/...`
- [ ] Run integration tests (if environment available).
