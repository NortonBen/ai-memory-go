# Tasks: Implement Multi-DB Config and Redis Stores

## Configuration Updates (`internal/cli/config.go`)

- [x] Expand `createDefaultConfig` to include keys for neo4j, postgres, and qdrant.
- [x] Refactor `InitEngine` graph logic to support `neo4j`.
- [x] Refactor `InitEngine` vector logic to support `postgres` and `qdrant`.

## Redis Vector Store Implementation

- [x] Implement robust caching and float32[] to byte mapping via `math` or `encoding/binary` for `StoreEmbedding`.
- [x] Implement `CreateCollection` executing `FT.CREATE` command manually via `rdb.Do`.
- [x] Implement `SearchSimilar` with KNN syntax string formatting executed via `rdb.Do("FT.SEARCH", ...)`.
- [x] Parse `FT.SEARCH` reply structures mapped back into `SimilarityResult`.
- [x] Cleanup stubs to properly hook up to `GetEmbedding` and `Delete`.

## Redis Graph Store Implementation

- [x] Refactor graph nodes logic: properly parse `schema.Node` returning instances from `GetNode`.
- [x] Expand manual BFS tree logic inside `TraverseGraph` executing `SADD` based adjaency tracking.
- [x] Implement `GetRelationship` and updates using the central global associative hashes.

## Final Review

- [x] Verify test compatibility and build compilation.
