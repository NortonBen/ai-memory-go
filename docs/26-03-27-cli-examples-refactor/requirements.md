# Requirements: CLI and Examples Refactor

Update the CLI and examples to align with the refactored `extractor` package infrastructure.

## User Stories

1. **CLI Consistency**: As a user of the `ai-memory` CLI, I want the tool to correctly initialize the memory engine using the latest `extractor` provider configurations so that I can use modern LLM and embedding providers.
2. **Migration Alignment**: As a developer, I want all example code to demonstrate the recommended way of initializing providers and the memory engine using the new `extractor` factory and registry patterns.
3. **Redundancy Reduction**: As a maintainer, I want to eliminate redundant provider initialization code in examples by leveraging the centralized `extractor` registry.

## Acceptance Criteria

### CLI Updates
- [ ] `internal/cli/config.go` `InitEngine` function successfully instantiates `LLMProvider` and `EmbeddingProvider` using the new `extractor` factories.
- [ ] Provider configurations (endpoint, model, API key) are correctly mapped from the CLI configuration to the `extractor` config structs.
- [ ] The CLI successfully compiles and runs with the refactored `extractor` package.

### Examples Updates
- [ ] `examples/quickstart/main.go` uses `extractor` factories for Ollama/LM Studio initialization.
- [ ] `examples/knowledge_bot_gemini/main.go` uses `extractor` factory for Gemini initialization.
- [ ] `examples/knowledge_bot_openrouter/main.go` uses `extractor` factory for OpenRouter initialization.
- [ ] `examples/sqlite_lmstudio/main.go` covers consolidated provider logic.

### Structural Integrity
- [ ] All updated examples compile and execute successfully.
- [ ] No regression in functionality for existing supported providers (OpenAI, Gemini, Anthropic, Ollama, LM Studio).
