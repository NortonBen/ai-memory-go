# Tasks: Engine Initialization & CLI Completion

## Planning
- [ ] Define requirements and design specs.

## Configuration & DI Container
- [ ] Add `llm` and `embedder` structs to the generic configuration parsing inside `internal/cli/config.go`.
- [ ] Expand `createDefaultConfig` to supply basic mock configurations for `llm` and `embedder`.
- [ ] Update `InitEngine` to instantiate the appropriate `LLMProvider` (handling API keys via config or ENV).
- [ ] Update `InitEngine` to instantiate the appropriate `Embedder` (handling API keys and model selection).
- [ ] Combine all stores and models to instantiate `MemoryEngine` and return it.

## Runner Application
- [ ] Create `cmd/ai-memory-cli/main.go` serving as the main binary entry point calling `cli.Execute()`.

## CLI Command Overhauls
- [ ] Overhaul `internal/cli/add.go` to insert context natively against the active session and process it via the `MemoryEngine`.
- [ ] Overhaul `internal/cli/search.go` to conduct graph/vector lookups and output results beautifully.
- [ ] Overhaul `internal/cli/cognify.go` to process the memory queue synchronously or asynchronously via the engine.
