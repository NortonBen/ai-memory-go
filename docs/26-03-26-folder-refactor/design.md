# Design Document: Folder Refactor

## Architecture Overview

We will restructure the `extractor` and `parser` packages into a hierarchical structure. 

### Extractor Package Restructure
Current: `extractor/*.go`
New:
- `extractor/`: Core interfaces, common types, config manager.
- `extractor/providers/`: Sub-packages for each provider.
  - `openai/`: OpenAI LLM and OpenAI Embedding.
  - `anthropic/`: Anthropic LLM.
  - `gemini/`: Gemini LLM.
  - `ollama/`: Ollama LLM and Ollama Embedding.
  - ...
- `extractor/registry/`: Factory and registration logic (to avoid circular imports between core and providers).
- `extractor/scoring/`: Quality scorer.
- `extractor/docs/`: All summary and documentation files.

### Parser Package Restructure
Current: `parser/*.go`
New:
- `parser/`: Core parser interfaces and unified logic.
- `parser/formats/`:
  - `pdf/`: PDF parser.
  - `text/`: Text formats.
- `parser/features/`:
  - `cache/`: Caching logic.
  - `workerpool/`: Worker pool implementation.
  - `deduplication/`: Deduplication logic.
- `parser/benchmarks/`: All `*_benchmark_test.go` and performance runner files.
- `parser/docs/`: Documentation.

## Circular Dependency Resolution
In Go, the root package `extractor` should NOT directly import `extractor/providers/openai` IF `openai` needs types from `extractor`.
Instead, move the core types (Interfaces, Config, Message) to a base package like `extractor/types`. 
Then both `extractor` (root) and `extractor/providers/*` will import `extractor/types`.
The `registry` or `factory` will be the top-level part that imports all providers to register them.

## Data Flow
No changes to external Data Flow. This is a internal organization refactoring.
