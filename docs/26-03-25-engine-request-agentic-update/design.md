# Design: Agentic Request Update

## Architecture
The update relies on pushing the complexity of "intent inference" entirely into the `LLMExtractor`. The `Request` method becomes a router for that intent.

### 1. Schema Extensions (`schema/schema.go`)
Extend `RequestIntent` to output more granular intent dimensions:
```go
type RequestIntent struct {
	NeedsVectorStorage bool     `json:"needs_vector_storage"`
	IsQuery            bool     `json:"is_query"`
	IsDelete           bool     `json:"is_delete"`
	DeleteTargets      []string `json:"delete_targets,omitempty"`
	Reasoning          string   `json:"reasoning,omitempty"`
}
```

### 2. Extractor Updates (`extractor/basic_extractor.go`)
Rewrite the `ExtractRequestIntent` prompt to instruct the LLM to output a JSON struct mapping to the updated `RequestIntent`.

### 3. Core Engine Changes (`engine/interface.go`, `engine/engine.go`)
Update the `MemoryEngine` interface:
```diff
- Request(ctx context.Context, sessionID string, content string, opts ...RequestOption) error
+ Request(ctx context.Context, sessionID string, content string, opts ...RequestOption) (*schema.ThinkResult, error)
```

In `defaultMemoryEngine.Request`:
- **Step 1:** Call `ExtractRequestIntent(ctx, content)`.
- **Step 2:** If `IsDelete` is true -> Iterate through `DeleteTargets`. Use `GraphStore.FindNodesByEntity/FindNodesByProperty` -> `DeleteNode`. Call `VectorStore.SearchDataPoints` with threshold to find relevant records, and `DeleteMemory`.
- **Step 3:** If it's a statement/fact -> Run normal extraction (`ExtractEntities`, `NeedsVectorStorage` logic).
- **Step 4:** If `IsQuery` is true -> Call `e.Think(ctx, query)` where query contains the original content, returning the `ThinkResult`. If `IsQuery` is false, formulate a synthetic `ThinkResult` with a basic acknowledgment.

### 4. Implementation Plan (`implementation_plan.md`)
We will track implementation explicitly using the standardized checklist pattern in `tasks.md`.
