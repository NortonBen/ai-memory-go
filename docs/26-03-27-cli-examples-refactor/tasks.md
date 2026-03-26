# Tasks: CLI and Examples Refactor

## Phase 1: Preparation & Verification
- [ ] Verification of existing `extractor` factory accessibility.
- [ ] Identification of all impacted example files.

## Phase 2: CLI Implementation
- [ ] Refactor `internal/cli/config.go` to use `extractor` factories.
- [ ] Update `InitEngine` to correctly map all `viper` settings.
- [ ] Verify CLI compilation.

## Phase 3: Examples Implementation
- [ ] Update `examples/quickstart/main.go`.
- [ ] Update `examples/knowledge_bot_gemini/main.go`.
- [ ] Update `examples/knowledge_bot_openrouter/main.go`.
- [ ] Update `examples/sqlite_lmstudio/main.go`.

## Phase 4: Final Verification
- [ ] Run `go build ./...` to ensure no compilation errors.
- [ ] Test CLI with a local provider (if possible) or mock.
- [ ] Update `walkthrough.md`.
