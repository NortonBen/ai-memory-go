# Consistency Reasoning Architecture Design

## 1. System Components Involved

- **Engine (`engine.go`)**: Orchestrates the consistency check after extraction and before storage.
- **LLMExtractor (`extractor/`)**: Requires a new prompt/method to strictly compare two JSON entities.
- **VectorStore (`vector/`)**: Requires efficient threshold-based search (already supported via `SimilaritySearch(limit, threshold)`).
- **GraphStore (`graph/`)**: Needs mapping for the `CONTRADICTS` logic and property overwrite logic (`UPDATE`).

## 2. Data Flow

1. **Extraction (LLM)**: Data point is parsed into `Node Focus (X)` and `Relationships (X -[R]-> Y)`.
2. **Pre-Insertion Vector Check**:
   - For each extracted node (X), generate embeddings.
   - Run `vectorStore.SimilaritySearch(X_embedding, limit=1, threshold=0.1)`.
3. **Consistency Evaluation (LLM)**:
   - IF result exists (ExistingNode `E`), trigger `llmExtractor.CompareEntities(ctx, E.Data, X.Data)`.
4. **Resolution (Graph & Vector)**:
   - **Case 'UPDATE'**:
     - Re-embed X's updated data.
     - GraphStore: `UpdateNode(E.ID, X_Properties)`
     - VectorStore: `UpdateVector(E.ID, X_embedding)`
   - **Case 'CONTRADICT'**:
     - GraphStore: Insert `X` as new node.
     - GraphStore: Insert edge `E -[CONTRADICTS]-> X`.
     - VectorStore: Insert `X`.
   - **Case 'IGNORE'**:
     - Do nothing (or append as observation).

## 3. Interfaces Updated

### `schema/schema.go`

```go
type ResolutionAction string
const (
    ResolutionUpdate     ResolutionAction = "UPDATE"
    ResolutionContradict ResolutionAction = "CONTRADICT"
    ResolutionIgnore     ResolutionAction = "IGNORE"
)

type ConsistencyResult struct {
    Action ResolutionAction `json:"action"`
    Reason string           `json:"reason"`
    MergedData map[string]interface{} `json:"merged_data,omitempty"`
}
```

### `extractor/extractor.go`

```go
type Extractor interface {
    // Existing methods...
    CompareEntities(ctx context.Context, existing Entity, newEntity Entity) (*ConsistencyResult, error)
}
```

## 4. Prisma / Redis (Not strictly applicable as we use Neo4j/SQLite, but documented for compatibility)

- No schema changes to relational schema required, edge logic handled by underlying Graph DB.
