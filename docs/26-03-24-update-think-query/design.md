# Architecture Design: ThinkQuery and Graph Traversal

## 1. Schema Updates

**`schema.go`**:

```go
// ThinkQuery defines parameters for the reasoning and answer generation
type ThinkQuery struct {
 Text      string `json:"text"`
 SessionID string `json:"session_id"`
 Limit     int    `json:"limit"`     // Limit for vector search
 HopDepth  int    `json:"hop_depth"` // Depth of graph traversal (1 or 2)
}
```

## 2. Interface Changes

**`MemoryEngine Interface`**:

```go
Think(ctx context.Context, query *schema.ThinkQuery) (*schema.ThinkResult, error)
```

## 3. Data Flow for `Think` Method

1. **Validation**: Check for LLM Provider.
2. **Setup**: Map `ThinkQuery` to `SearchQuery` for the internal call.
3. **Core Search**: Invoke `e.Search()` to obtain Top-K DataPoints and implicitly score them.
4. **Explicit Traversal (Bước 3)**:
   - Extract entities from `query.Text` to find `anchorNodeIDs`.
   - Traverse Graph up to `query.HopDepth` hops to find neighbor Nodes.
   - Collect these explicitly found nodes, filtering out duplicates.
5. **Context Assembly (Bước 4)**:
   - Insert Vector Search DataPoints.
   - Insert Graph Entities (Nodes and explicitly stated relationships like "Công thức", "Dấu hiệu nhận biết").
6. **LLM Generation**: Prompt the LLM using this deeply enriched context to yield `Reasoning` and `Answer`.
