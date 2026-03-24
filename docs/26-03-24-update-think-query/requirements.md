# Requirements: Update ThinkQuery and Graph Traversal (Bước 3)

## 1. Overview
The current `MemoryEngine.Think` implementation relies on a generic `SearchQuery` and passes only the content of the `DataPoint`s to the LLM. The graph traversal (Step 3) is currently used internally just to boost `DataPoint` relevance scores, leaving the graph structure (e.g., related concept nodes like "Công thức", "Dấu hiệu nhận biết") hidden from the LLM.

This feature will:
- Introduce a specialized `ThinkQuery` containing only necessary parameters (Text, SessionID, Limit, HopDepth).
- Update the "Graph Traversal" step so that explicitly-discovered graph nodes (1-hop or 2-hop) are appended directly into the Context sent to the LLM.

## 2. User Stories
- As a Developer, I want to use `ThinkQuery` so that I can explicitly define parameters specifically suited for LLM reasoning (like HopDepth), without cluttering it with generic full-text search settings.
- As a User asking a question, I want the LLM to know about related concepts in my knowledge graph (like "Lỗi sai trước đó của bạn") so that the AI's reasoning is deeply integrated with the explicit graph edges, rather than just semantic chunks.

## 3. Acceptance Criteria
- [ ] `ThinkQuery` struct is created in `schema/schema.go` with `Text`, `SessionID`, `Limit`, and `HopDepth` fields.
- [ ] `MemoryEngine.Think` interface is updated to accept `*schema.ThinkQuery`.
- [ ] The `Think` implementation explicitly extracts neighbor nodes up to `HopDepth`.
- [ ] The neighbor nodes (Types and relationships) are appended to the `contextBuilder` String.
- [ ] The LLM correctly receives and incorporates the explicit graph nodes into its `ThinkResult.Reasoning`.
