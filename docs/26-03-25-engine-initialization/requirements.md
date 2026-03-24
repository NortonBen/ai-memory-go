# Requirements: Engine Initialization & CLI Completion

## User Stories
1. **As a user**, I want to run `go run cmd/ai-memory-cli/main.go add "text"` and have it correctly instantiate all system components (Vector DB, Graph DB, LLM Extractor, Text Embedder) and store the memory.
2. **As a user**, I want the CLI seamlessly read configuration parameters for LLMs and embedders (e.g. OpenAI/Gemini keys) from `~/.ai-memory.yaml` or ENV variables so it can process memory.
3. **As a user**, I want commands like `add`, `search`, `delete`, and `cognify` to actually execute business logic inside `MemoryEngine` instead of just returning stub outputs.
4. **As a developer**, I want a clear entrypoint in `cmd/ai-memory-cli` to build a static binary.

## Acceptance Criteria
- [ ] A new directory `cmd/ai-memory-cli` exists with a functional `main.go`.
- [ ] `internal/cli/config.go`'s `InitEngine` function successfully wires `LLMProvider`, `Extractor`, `Embedder` alongside `VectorStore` and `GraphStore`.
- [ ] `add.go` successfully adds text and triggers the engine's `ProcessContext`.
- [ ] `search.go` correctly interacts with the engine's search API.
- [ ] `cognify.go` starts memory processing.
- [ ] `yaml` configurations or environment variables are utilized for `llm` and `embedder` initialization.
