# Design: Engine Initialization & CLI Completion

## Architecture Context
The core CLI operations exist under `internal/cli/`. The application entrypoint should be isolated to `cmd/ai-memory-cli/main.go`. 
The `InitEngine` function within `internal/cli/config.go` acts as our dependency injection container mapping standard configs into operational structs.

## Dependency Injection (InitEngine)
Currently, `InitEngine` handles VectorStore and GraphStore. We will extend it to include:
1. **Embedder**: Inject `GeminiEmbeddingProvider` or `SentenceTransformerEmbeddingProvider` depending on `config.embedder.provider`. Will require an API key either from config or `os.Getenv("GEMINI_API_KEY")`.
2. **LLM**: Inject `DeepSeekProvider`, `OpenAIProvider`, or `OllamaProvider` depending on `config.llm.provider`. API key driven from config or ENV.
3. **Extractor**: Initialize `NewMemoryExtractor(llmProvider, promptTemplate)`
4. **MemoryEngine**: Bring them all together `engine.NewMemoryEngine(graph, vector, extractor, embedder, config)`.

## Config Additions
Extend the YAML format in `createDefaultConfig`:
```yaml
llm:
  provider: "openai" # deepseek, openai, ollama
  api_key: ""
  endpoint: ""
  model: "gpt-4o"
embedder:
  provider: "gemini" # gemini, sentence-transformers
  api_key: ""
  model: "models/text-embedding-004"
```

## CLI Implementation
Update `add.go`, `search.go`, `cognify.go` to directly call engine routines like `MemoryEngine.AddMemory` and `MemoryEngine.Search` and print real outputs, replacing fake placeholders.
