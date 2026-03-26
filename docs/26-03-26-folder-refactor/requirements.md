# Requirements & Business Logic: Folder Refactor

## User Stories
As a developer, I want all providers and parsers to be in their own sub-directories so that I can manage them independently and reduce the total file count in the root packages.

## Acceptance Criteria
- `extractor/` contains only core interfaces and the manager/registry.
- All LLM and Embedding providers are moved to `extractor/providers/`.
- All documentation files are moved to `extractor/docs/`.
- `parser/` contains only core interfaces and unified logic.
- All specific format parsers are moved to `parser/formats/`.
- All parser features (cache, worker pool) are in `parser/features/`.
- All benchmark and profiling tests are moved to `parser/benchmarks/`.
- The project builds and all tests pass after the refactor.
- Imports in other modules (like `engine`) are updated to use the new packages.

## Business Rules
- Providers of the same type (e.g., OpenAI LLM and Embedding) should be grouped together if possible.
- Core types and interfaces must stay in the root of the package or in a `core/` package to avoid circular dependencies.
