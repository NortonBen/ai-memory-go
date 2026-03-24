# Architecture Design: Redis Store Implementation

## 1. Config Structure
The current `viper` layout dictates a single tier for `db`, generally pointing to SQLite. The new structural mappings:

```yaml
db:
  datadir: "~/.ai-memory/data"
  vector:
    provider: "sqlite" # or "redis"
  graph:
    provider: "sqlite" # or "redis"
  redis:
    endpoint: "localhost:6379"
    password: ""
```

## 2. Factory Injection
In `internal/cli/config.go`, the initialization function `InitEngine` reads the `db.vector.provider` and `db.graph.provider`.
```go
vecProvider := viper.GetString("db.vector.provider")
if vecProvider == "redis" {
    vecStore = vector.NewRedisVectorStore(...) 
} else {
    vecStore = vector.NewSQLiteVectorStore(...)
}
```

## 3. Data Representation (Redis Vector)
- Relies on **RediSearch API** for `FT.CREATE` and `FT.SEARCH`.
- KNN vector metric using HASH fields holding raw float32 arrays encoded into bytes.

## 4. Data Representation (Redis Graph)
- Uses Redis Sets & Hashes for raw adjacency representation (native).
- **Node Hash:** `node:{id}` -> map of attributes.
- **Edge Sets:** `out:{fromId}` -> sets of strings encoding `toId|relationType`.
