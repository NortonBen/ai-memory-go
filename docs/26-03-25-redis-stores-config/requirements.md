# Requirements: Redis Store Configurations

## Overview
The platform (`ai-memory-go`) current supports SQLite/PostgreSQL as database implementations for relational, vector, and graph stores. We need to expose configuration capabilities so that a user can choose `redis` as an alternate backing store for vectors and graphs.

## User Stories
1. **As a system administrator**, I want to configure the AI Memory CLI to point to my Redis instance so that I can use Redis for vector embeddings and graph traversal.
2. **As a developer**, I want the config file (`~/.ai-memory.yaml`) to have structured fields (`db.vector.provider` and `db.graph.provider`) so that it seamlessly switches storage backend contexts at runtime.

## Acceptance Criteria
- [ ] Users can edit `~/.ai-memory.yaml` to set `db.vector.provider: redis`.
- [ ] Users can edit `~/.ai-memory.yaml` to set `db.graph.provider: redis`.
- [ ] Users can specify `db.redis.endpoint: "localhost:6379"` in the configuration.
- [ ] The CLI `add` command properly executes end-to-end when Redis is configured.
- [ ] Fallbacks gracefully to SQLite if the providers in config are undefined or explicit `sqlite`.
