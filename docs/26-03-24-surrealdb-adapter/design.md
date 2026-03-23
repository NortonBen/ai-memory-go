# SurrealDB Adapter Design

## Architecture

The `SurrealDBStore` struct will implement both the `Storage` and `GraphStore` (and potentially `TransactionalGraphStore`) interfaces.
It will use an official Go client for SurrealDB (e.g., `github.com/surrealdb/surrealdb.go`).

### Data Mapping

- **Nodes**: Mapped to a SurrealDB table `node`.
  - ID: `Node.ID` becomes the record ID: `node:{ID}`.
  - Properties: `Node.Properties` and other metadata (like `chunk_id`, `type`) stored as fields.
- **Edges**: Mapped to a SurrealDB edge table `edge`.
  - Relation: Created via `RELATE node:{src} -> edge -> node:{dst}`.
  - Edge type will be stored as a property on the `edge` record or as the table name itself. To avoid too many edge tables, we might use a generic table like `relationship` and store `edge_type: "RELATES_TO"` as a field.

### Connection

```go
type SurrealDBStore struct {
    db     *surrealdb.DB
    config *GraphConfig // Inherited from storage layer
}
```

### Core Operations

- **StoreNode**: `UPDATE node:$id CONTENT $data` or `CREATE`.
- **CreateRelationship**: `RELATE $src -> relationship -> $dst SET properties = $props`.
- **TraverseGraph**: Leverage SurrealQL graph traversing syntax `SELECT * FROM $start_node->relationship->node`.

### Dependencies

- `github.com/surrealdb/surrealdb.go`

## Security & Performance

- Uses query parameters for all requests to ensure zero injection risk.
- Implements transactions (`BEGIN TRANSACTION ... COMMIT TRANSACTION`) for grouped modifications.
