# Design: Multi-DB config and Redis native stores

## Configuration Matrix
`~/.ai-memory.yaml` will expand to include configs for all providers.

```yaml
db:
  datadir: "~/.ai-memory/data"
  vector:
    provider: sqlite # sqlite, redis, postgres, qdrant
  graph:
    provider: sqlite # sqlite, neo4j, redis
  postgres:
    dsn: "postgres://user:pass@localhost:5432/memory"
  qdrant:
    endpoint: "localhost:6334"
  neo4j:
    uri: "neo4j://localhost:7687"
    user: "neo4j"
    password: "password"
  redis:
    endpoint: "localhost:6379"
    password: ""
```

## Redis Vector Store Architecture
RediSearch operates natively over hashes.
Keys: `vec:{id}`
In `CreateCollection()`, execute:
```redis
FT.CREATE idx:vector ON HASH PREFIX 1 vec: SCHEMA embedding VECTOR HNSW 6 TYPE FLOAT32 DIM {dim} DISTANCE_METRIC L2 metadata TEXT
```

During `StoreEmbedding`, we `HSET vec:{id} embedding {bytes}`. Float arrays will be converted to `[]byte` via `encoding/binary` as Redis strictly requires binary blobs for VECTOR type values.

## Redis Graph Store Architecture
Without relying on `RedisGraph` (EOL) or `FalkorDB`, we implement graphs manually:
- Nodes: `node:{id}` (string-serialized JSON containing `schema.Node`).
- Relationships storage: Hash map of `edge_id -> schema.Edge (JSON)`, plus adjacency sets.
- Outbound relations: `SMEMBERS out:{node_id}` yielding `{connected_node_id}` to facilitate simple graph walking during `TraverseGraph`. 

The `TraverseGraph` method will use a classic BFS implementation loaded into memory natively using pipelined Redis lookups across `out:*` and `node:*` keys.
