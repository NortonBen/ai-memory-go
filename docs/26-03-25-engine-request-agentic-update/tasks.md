# Tasks: Agentic Request Update

## Backend (Schema & Interface)
- [x] [AGENTIC-001] Extend `schema.RequestIntent` with `IsQuery`, `IsDelete`, and `DeleteTargets`.
- [x] [AGENTIC-002] Modify interface `engine.MemoryEngine` so `Request` returns `(*schema.ThinkResult, error)`.

## Backend (Extractor Logic)
- [x] [AGENTIC-003] Update `extractor/basic_extractor.go:ExtractRequestIntent` prompt to classify statements, questions, and delete commands accurately.

## Backend (Engine Core)
- [x] [AGENTIC-004] Refactor `engine/engine.go:Request` method signature.
- [x] [AGENTIC-005] Implement **Deletion Routing**: For each target in `DeleteTargets`, search and wipe out Graph nodes and `VectorStore` entities.
- [x] [AGENTIC-006] Implement **Think Routing**: Call internal `Think` function and return the valid response.
- [x] [AGENTIC-007] Implement **Statement Fallback**: Return synthetic confirmation `ThinkResult` for normal memorizations.

## Testing & Validation
- [x] [AGENTIC-008] Update `engine_test.go` to match the new signature of `Request` and add assertions for query intent, and delete intent.
- [x] [AGENTIC-009] Write explicit unit test for Deletion via `Request`.
