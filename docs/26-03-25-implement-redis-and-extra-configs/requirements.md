# Requirements: Full Redis Implementation and Extended Multi-DB Configuration

## Overview
Following the introduction of the configuration abstraction for `redis`, the user has requested that the Redis vector and graph implementations must be natively written providing full capabilities, instead of primarily raising "not implemented" errors. Also, the configuration scaffolding in `internal/cli/config.go` should support routing to the other existing first-party databases: Postgres/pgvector, Qdrant, and Neo4j.

## User Stories
1. **As a user**, I want `db.vector.provider` to recognize `postgres`, `qdrant`, and `redis`, falling back on `sqlite`.
2. **As a user**, I want `db.graph.provider` to recognize `neo4j` and `redis`, falling back on `sqlite`.
3. **As a developer**, I want `ai-memory-go` to utilize RediSearch (`FT.CREATE`, `FT.SEARCH`) and native Redis hashes/sets correctly for vector embeddings and graph relations natively when Redis is chosen.

## Acceptance Criteria

- [x] Redis vector store properly marshals/unmarshals float32 embeddings into Redis.
- [x] Redis vector store allows KNN vector searches using `FT.SEARCH` query strings.
- [x] Redis graph store implements node storage and undirected/directed edge associations via sets.
- [x] Configuration scaffolding properly creates and fetches pgvector stores when configured.
- [x] Configuration scaffolding properly creates and fetches qdrant stores when configured.
- [x] Configuration scaffolding properly creates and fetches neo4j stores when configured.
