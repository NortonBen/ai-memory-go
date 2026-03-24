# Tasks: Consistency Reasoning

## Backend Tasks

- [ ] **Task 1: Update Schema Interfaces**
  - Define `ResolutionAction` (UPDATE, CONTRADICT, IGNORE).
  - Define `ConsistencyResult` struct in `schema.go`.
- [ ] **Task 2: LLMExtractor Enhancements**
  - Add `CompareEntities` interface method to `extractor.go`.
  - Implement `CompareEntities` in existing AI providers (`lmstudio`, `openai`, `ollama` etc.).
  - Create the system prompt that forces the LLM to choose between UPDATE, CONTRADICT, or IGNORE based on Entity diffs.
- [ ] **Task 3: MemoryEngine Orchestration**
  - Refactor the `Cognify`/`Memify` pipeline. Before calling `GraphStore.UpsertNode`, do a `VectorStore.SimilaritySearch` (threshold = 0.1).
  - If a hit occurs, execute `CompareEntities`.
- [ ] **Task 4: GraphStore Operations**
  - Implement `CONTRADICTS` edge creation in `Neo4j` and `SQLite` graph adapters when action is CONTRADICT.
  - Implement proper overwrite mechanisms for `UPDATE` action (ensuring Vector DB vectors are re-synced if properties change).
- [ ] **Task 5: Configuration Options**
  - Add `WithConsistencyThreshold(float32)` to `AddOptions` so the 0.1 vector threshold can be tuned dynamically.

## Testing & Verification

- [ ] **Task 6: E2E Validation**
  - Create an `examples/consistency_reasoning` script demonstrating the updates and contradictions.
  - Assert that the correct Graph relationship (`CONTRADICTS`) is established when conflicting facts are provided.
