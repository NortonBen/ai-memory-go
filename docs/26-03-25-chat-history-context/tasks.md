# Tasks: Chat History Context

## Phase 2: Execution

- [x] [HISTORY-001] Extend `schema.Message` and `schema.MemorySession`
- [x] [HISTORY-002] Modify interface `storage.Storage`
- [x] [HISTORY-003] Update adapter to `session_messages` table (SQLite)
- [x] [HISTORY-004] Refactor `engine/engine.go:Request` to inject recent chat history into Extractor prompt
- [x] [HISTORY-005] Refactor `engine/engine.go:Request` sequential control flow (mixed intent fallback)
- [x] [HISTORY-006] Write unit tests `TestRequestHistoryContext`

## Phase 3: Extractor Prompt Engineering

- [x] [HISTORY-007] Inject the loaded chat history into the prompts for `ExtractRequestIntent`, `ExtractEntities`, and `Think`.
- [x] [HISTORY-008] Write tests for Context injection mapping pronouns correctly to recent entities.
