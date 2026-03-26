# Implementation Plan: CLI and Examples Refactor

## Proposed Changes

### [internal/cli](file:///Users/benji/Projects/ai-memory-brain/internal/cli/)
#### [MODIFY] [config.go](file:///Users/benji/Projects/ai-memory-brain/internal/cli/config.go)
- Transition `InitEngine` to strictly use `extractor` factories.
- Ensure `EmbeddingProviderConfig` and `ProviderConfig` are fully populated from `viper`.

### [examples](file:///Users/benji/Projects/ai-memory-brain/examples/)
#### [MODIFY] [quickstart/main.go](file:///Users/benji/Projects/ai-memory-brain/examples/quickstart/main.go)
- Replace `vector.NewLMStudioEmbeddingProvider` with factory call.
- Use `extractor.NewProviderFactory()` for LLM setup.

#### [MODIFY] [knowledge_bot_gemini/main.go](file:///Users/benji/Projects/ai-memory-brain/examples/knowledge_bot_gemini/main.go)
- Standardize Gemini initialization via factory.

#### [MODIFY] [knowledge_bot_openrouter/main.go](file:///Users/benji/Projects/ai-memory-brain/examples/knowledge_bot_openrouter/main.go)
- Update provider creation to match refactored pattern.

## Verification Plan

### Automated Tests
- Run `go build ./...` from the project root.
- Execute unit tests in `internal/cli`.

### Manual Verification
- Run `ai-memory config --init` and ensure the generated config is valid.
- Run one of the updated examples (e.g., `go run examples/quickstart/main.go`) to verify runtime behavior.
