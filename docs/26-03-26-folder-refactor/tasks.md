# Tasks: Folder Refactor

## Phase 1: Planning & Setup
- [ ] Create refactor directory in `docs/` (`requirements.md`, `design.md`, `tasks.md`, `implementation_plan.md`)
- [ ] Analyze `extractor` dependencies to identify core types for `extractor/types`
- [ ] Analyze `parser` dependencies to identify core types for `parser/types`

## Phase 2: Extractor Refactor
- [ ] Move core types into `extractor/types/` (Config, Message, State)
- [ ] Move core interfaces into `extractor/` (LLMProvider, EmbeddingProvider)
- [ ] Create `extractor/providers/` sub-packages:
  - [ ] Move OpenAI: `provider_openai.go`, `provider_openai_embedding.go`, tests. Fix package/imports.
  - [ ] Move Anthropic: `provider_anthropic.go`, tests. Fix package/imports.
  - [ ] Move Gemini: `provider_gemini.go`, tests. Fix package/imports.
  - [ ] Move Ollama: `provider_ollama.go`, tests. Fix package/imports.
  - [ ] Move DeepSeek, OpenRouter, LMStudio. Fix package/imports.
- [ ] Move `quality_scorer.go` to `extractor/scoring/`. Fix package/imports.
- [ ] Move `health_check.go` to `extractor/health/`. Fix package/imports.
- [ ] Update `provider_registry.go` and `provider_factory.go`. Fix package/imports.
- [ ] Move documentation `.md` and `SUMMARY.md` files to `extractor/docs/`.
- [ ] Fix imports in `engine/` and other modules.

## Phase 3: Parser Refactor
- [ ] Move core types into `parser/types/`
- [ ] Create `parser/formats/`:
  - [ ] Move `pdf.go`, `formats.go`, tests. Fix package/imports.
- [ ] Create `parser/features/`:
  - [ ] Move `cache.go`, tests.
  - [ ] Move `worker_pool.go`, tests.
  - [ ] Move `deduplication.go`, tests.
- [ ] Create `parser/benchmarks/`:
  - [ ] Move all benchmark tests and performance runners.
- [ ] Move `streaming.go`, `detection.go`, `chunking.go` to appropriate sub-packages or keep in core.
- [ ] Move documentation files to `parser/docs/`.
- [ ] Fix imports in `engine/` and other modules.

## Phase 4: Verification
- [ ] Run `go build ./...` to verify compilation.
- [ ] Run `go test ./...` to verify all tests pass.
- [ ] Verify no circular dependencies remain.
- [ ] Verify documentation still points to valid sections or update where needed.
