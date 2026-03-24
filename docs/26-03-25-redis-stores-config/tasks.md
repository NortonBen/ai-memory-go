# Tasks: Redis Stores Implementation

## Core Config & Scaffolding
- [x] Add `redis` connection wrapper/struct if needed.
- [x] Update `createDefaultConfig()` and `InitEngine()` inside `internal/cli/config.go` with branching for vectors and graphs.

## Vector Store (Redis)
- [x] Create `vector/redis_store.go` implementing `vector.Store`.
- [x] Write `SaveVector` and `SaveVectors` converting embeddings to float byte arrays for Redis HSET.
- [x] Implement `FT.SEARCH` for KNN vector retrieval in `SearchSimilar`.
- [x] Handle vector store memory initialization (`FT.CREATE` indices safely).

## Graph Store (Redis)
- [ ] Create `graph/redis_store.go` implementing `graph.Store`.
- [ ] Implement `AddNode`, `UpdateNode`, `DeleteNode` using `HSET` and `DEL`.
- [ ] Implement `AddEdge`, `GetEdges`, `DeleteEdge`, `GetNeighbors` using lightweight `SADD`, `SREM`, `SMEMBERS` for tracking directed adjacency maps naturally.

## Testing
- [ ] Create automated tests using embedded/mock redis or interface adapters.
- [ ] End-to-end verification running the CLI via Redis locally.
