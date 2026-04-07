# AI Memory Integration - Go Library

[![CI](https://github.com/NortonBen/ai-memory-go/actions/workflows/ci.yml/badge.svg)](https://github.com/NortonBen/ai-memory-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/NortonBen/ai-memory-go)](https://goreportcard.com/report/github.com/NortonBen/ai-memory-go)
[![codecov](https://codecov.io/gh/NortonBen/ai-memory-go/branch/main/graph/badge.svg)](https://codecov.io/gh/NortonBen/ai-memory-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/NortonBen/ai-memory-go.svg)](https://pkg.go.dev/github.com/NortonBen/ai-memory-go)

Go library for persistent knowledge-graph + vector memory for AI agents, with a data-driven pipeline, pluggable storage, a first-party CLI, and an MCP server for IDE integration.

## Overview

- **`engine` (`MemoryEngine`)**: **Add**, **Cognify**, **Search**, **Think**, **Request** (chat / intent flow), and **DeleteMemory** (by datapoint id or by session scope).
- **Tiers & search**: **memory tier** metadata (`core`, `general`, `data`, `storage`, …), **labels** on ingest; optional **four-tier** retrieval in engine config and in Think / Request / MCP.
- **Sessions**: named workspaces; CLI keywords **`global` / `shared` / `_`** store **shared** rows (`session_id` empty) while Search / Think / Request keep a stable default engine context for chat history (see `internal/sessionid`, `session` command).
- **CLI (`ai-memory-cli`)**: Cobra + Viper; config file **`~/.ai-memory.yaml`** (default), optional active session in **`~/.ai-memory/session.txt`**.
- **MCP**: `mcp` subcommand runs a Model Context Protocol server over **stdio** (default) or **streamable HTTP**, exposing **`memory_search`**, **`memory_add`**, **`memory_think`**, **`memory_request`**, **`memory_delete`**.

## Module

- **Module**: `github.com/NortonBen/ai-memory-go`
- **Go**: `1.25.0`

## Features

- Ingest → Cognify (embeddings, graph extraction, …) → search / reasoning.
- Multiple embedding / LLM providers (OpenAI, Ollama, LM Studio, OpenRouter, … — see `vector`, `extractor`, and `examples/`).
- Hybrid storage: **graph** adapters (in-memory, SQLite, Redis, Neo4j), **vector** (SQLite + sqlite-vec, Qdrant, …), **relational** (SQLite, PostgreSQL).
- Optional **`view`** command serves a small web UI (`internal/view`).

## Labels and four-tier retrieval

### Labels (classification metadata)

- Add labels at ingest with CLI `add --labels "rule,policy,story-name"` or MCP `memory_add` argument `labels`.
- Labels are normalized and stored in datapoint metadata (`memory_labels`, `primary_label`, `labels_joined`), and are inherited by chunk children during Cognify.
- Labels are for classification and downstream context, **not** an explicit search filter.
- If no explicit tier is provided, labels like `rule` / `policy` can auto-default the datapoint to the `core` memory tier.
- If you need strict tier placement, always pass `--tier` (CLI) or `tier` (MCP), which takes precedence.

### Four-tier retrieval

- Four-tier retrieval merges signals across memory tiers (`core`, `general`, `data`, optional `storage`) plus legacy vector fallback.
- **Tier 1 — `core`**: high-priority memory (rules, policies, durable constraints). This tier gets stronger weighting in ranking and is intended for "must not forget" context.
- **Tier 2 — `general`**: default working knowledge and normal conversational memory.
- **Tier 3 — `data`**: supporting facts/reference material; useful context but lower priority than `core` and `general`.
- **Tier 4 — `storage` (optional)**: colder/archival memory; can be excluded by default and pulled in when enabled or when weak top scores trigger storage fallback.
- In CLI, enable per request via `request --four-tier` (query-intent retrieval path).
- In MCP, enable per call with `four_tier=true` on `memory_request` and `memory_think`.
- Engine-level defaults live in `engine.EngineConfig.FourTier`; per-query options can override this default.
- Current CLI YAML init path (`internal/cli/initRuntime`) does not yet map full engine four-tier knobs from `~/.ai-memory.yaml`; use the Go API for advanced tuning.

### Session scopes (global vs named)

- **Named session (riêng)**: session có tên như `default`, `project-a`, `customer-x`; dữ liệu lưu với `session_id="<name>"`.
- **Global/shared pool**: dùng keyword `global`, `shared`, hoặc `_` khi add để lưu bản ghi dùng chung với `session_id` rỗng.
- Search/Think/Request luôn chạy trên một engine context có tên (mặc định `default`) và tự gộp thêm dữ liệu global/shared.
- Trong CLI: `-s <name>` để dùng session riêng; `-s global` để add vào shared pool (chat/search context vẫn map về `default`).
- Trong MCP: `memory_add` hỗ trợ `global=true` hoặc `session_id="<name>"`; `memory_search` / `memory_think` / `memory_request` hỗ trợ `session_id` để ghi đè ngữ cảnh truy vấn.

## Library quick start

```bash
go get github.com/NortonBen/ai-memory-go@latest
```

For a minimal SQLite + local embedder walkthrough, run `go run ./examples/quickstart/` after pointing the embedder at your environment (see comments in that file).

## CLI

Build the CLI:

```bash
make build-cli
# or: go build -o ai-memory-cli cmd/ai-memory-cli/main.go
```

Global flags: `--config`, `-s` / `--session`, `-v` / `--verbose`, `-f` / `--format` (`text`, `json`, `yaml`).

| Command | Purpose |
|--------|---------|
| `add` | Add content to memory |
| `cognify` | Run Cognify on a datapoint |
| `search` | Semantic / graph / hybrid retrieval (per config) |
| `graph` | Direct graph subgraph query (`graph query <entity>`) |
| `think` | Multi-step reasoning over graph / context |
| `request` | Conversational flow (intent, memory updates, answer) |
| `delete` | Delete by id or wipe a session |
| `session` | No args: print active session; `switch <name>` writes `~/.ai-memory/session.txt`; `list` is currently a stub |
| `config` | Print config; `config --init` writes default `~/.ai-memory.yaml` |
| `analyze-history` | Analyze recent chat history to refine the knowledge graph |
| `view` | Start the viewer HTTP server |
| `mcp` | Run MCP server for Cursor, Claude Desktop, etc. |

**MCP examples**

```bash
./ai-memory-cli mcp --transport stdio
./ai-memory-cli mcp --transport http --listen :8080 --path /mcp
```

**Graph query examples**

```bash
./ai-memory-cli graph query "OpenAI" --depth 2
./ai-memory-cli graph query "NortonBen" --node-type Person --limit 100
```

## Configuration (`~/.ai-memory.yaml`)

The CLI loads **`$HOME/.ai-memory.yaml`** by default. Override with **`--config /path/to/file.yaml`**. Viper **`AutomaticEnv()`** is enabled, so you can override individual keys with environment variables where Viper resolves them (see [Viper environment variables](https://github.com/spf13/viper#working-with-environment-variables)).

Run **`ai-memory-cli config --init`** to create a starter file. The tables below match what **`internal/cli/config.go`** reads when building the engine (`InitEngine` / `initRuntime`).

### Example (defaults from `config --init`)

```yaml
db:
  # After `config --init`, datadir is an absolute path (e.g. /home/you/.ai-memory/data)
  datadir: /path/to/.ai-memory/data
  vector:
    provider: sqlite          # sqlite | redis | postgres | qdrant
  graph:
    provider: sqlite          # sqlite | redis | neo4j
  redis:
    endpoint: localhost:6379
    password: ""
  postgres:
    host: localhost
    port: 5432
    username: postgres
    password: postgres
    database: ai_memory
    collection: vector_embeddings
  qdrant:
    host: localhost
    port: 6334
    collection: ai_memory
  neo4j:
    host: localhost
    port: 7687
    username: neo4j
    password: password
    database: neo4j

llm:
  provider: lmstudio
  endpoint: http://localhost:1234/v1
  model: qwen/qwen3-4b-2507
  api_key: ""

embedder:
  provider: lmstudio
  endpoint: http://localhost:1234/v1
  model: text-embedding-nomic-embed-text-v1.5
  dimensions: 768
  api_key: ""
  onnx:
    model_path: ""
    tokenizer_path: ""
    model_precision: ""       # auto | fp32 | int8 (empty = provider default)
    max_seq_len: 512
    query_task: "Retrieve semantically similar text"
    use_query_instruction: true

graph:
  extractor: ""               # empty or llm (default) | deberta
  deberta:
    model_path: ""
    tokenizer_path: ""
    labels_path: ""
    max_seq_len: 0
    exec_provider: ""         # cpu | coreml | cuda | auto
    model_precision: ""       # auto | fp32 | int8
```

### `db.*` — storage

| Key | Type | Used when | Meaning |
|-----|------|-----------|---------|
| `db.datadir` | string | always | Directory for local SQLite files (`graph.db`, `vectors.db`, `rel.db`). Default if empty: `~/.ai-memory/data`. |
| `db.graph.provider` | string | always | Graph backend: **`sqlite`** (file under `datadir`), **`redis`**, or **`neo4j`**. |
| `db.vector.provider` | string | always | Vector backend: **`sqlite`** (sqlite-vec under `datadir`), **`redis`**, **`postgres`** (pgvector), or **`qdrant`**. |
| `db.redis.endpoint` | string | graph or vector = `redis` | `host:port` (port optional; default Redis port if omitted). |
| `db.redis.password` | string | graph or vector = `redis` | Redis password (empty if none). |
| `db.postgres.*` | | vector = `postgres` | Connection to PostgreSQL + pgvector collection. |
| `db.postgres.host` | string | | Hostname. |
| `db.postgres.port` | int | | Port (e.g. `5432`). |
| `db.postgres.username` | string | | DB user. |
| `db.postgres.password` | string | | DB password. |
| `db.postgres.database` | string | | Database name. |
| `db.postgres.collection` | string | | Logical collection / table name for embeddings (see vector adapter). |
| `db.qdrant.host` | string | vector = `qdrant` | Qdrant host. |
| `db.qdrant.port` | int | | Qdrant gRPC port (template uses `6334`). |
| `db.qdrant.collection` | string | | Collection name. |
| `db.neo4j.host` | string | graph = `neo4j` | **Bolt URI** passed to the driver (e.g. `bolt://localhost:7687`). The init template uses `localhost`; for a real server, prefer a full URI. |
| `db.neo4j.port` | int | | Written by `config --init` but **not** currently appended to the URI in `initRuntime`—put host+port in `host` if needed. |
| `db.neo4j.username` | string | | Neo4j user. |
| `db.neo4j.password` | string | | Neo4j password. |
| `db.neo4j.database` | string | | Written by `config --init`; **not** wired in the current Neo4j graph init path. |

**Relational metadata** (datapoints, etc.) is always **SQLite** at **`{db.datadir}/rel.db`** in this code path—there is no YAML toggle for a different relational backend in `initRuntime`.

### `llm.*` — chat / extraction LLM

Passed to `extractor.ProviderConfig` (`Type`, `Endpoint`, `Model`, `APIKey`).

| Key | Meaning |
|-----|---------|
| `llm.provider` | One of: **`openai`**, **`anthropic`**, **`gemini`**, **`ollama`**, **`deepseek`**, **`mistral`**, **`bedrock`**, **`azure`**, **`cohere`**, **`huggingface`**, **`local`**, **`lmstudio`**, **`openrouter`**, **`custom`** (see `extractor/registry/provider_factory.go`). |
| `llm.endpoint` | API base URL (LM Studio / Ollama / custom gateways). |
| `llm.model` | Model id for that provider. |
| `llm.api_key` | API key when the provider requires it. |

### `embedder.*` — embeddings

| Key | Meaning |
|-----|---------|
| `embedder.provider` | Embedding backend string (e.g. **`openai`**, **`ollama`**, **`lmstudio`**, **`openrouter`**, **`onnx`**, …—see `extractor.EmbeddingProviderType` and `DefaultEmbeddingProviderFactory`). |
| `embedder.endpoint` | Embedding API base URL. |
| `embedder.model` | Embedding model name. |
| `embedder.dimensions` | Vector size; must match the model / stores. Default **`768`** if `0`. For **`onnx`**, runtime forces **`640`** (Harrier-style models). |
| `embedder.api_key` | Key when required by the provider. |

When **`embedder.provider`** is **`onnx`**, these are passed via `CustomOptions`:

| Key | Meaning |
|-----|---------|
| `embedder.onnx.model_path` | Path to the ONNX model file. |
| `embedder.onnx.tokenizer_path` | Tokenizer assets path. |
| `embedder.onnx.model_precision` | `auto`, `fp32`, or `int8` (see ONNX embedder docs). |
| `embedder.onnx.max_seq_len` | Max tokens / sequence length (default `512`). |
| `embedder.onnx.query_task` | Instruction text for query embeddings when instruction mode is on. |
| `embedder.onnx.use_query_instruction` | Whether to use query-specific instruction encoding. |

### `graph.*` — graph extraction mode

| Key | Meaning |
|-----|---------|
| `graph.extractor` | **`""`** or **`llm`**: graph extraction only via the configured LLM (`BasicExtractor`). **`deberta`**: hybrid **DeBERTa NER (ONNX) + LLM** (`HybridGraphExtractor`). Any other value fails startup. |
| `graph.deberta.model_path` | DeBERTa ONNX model path. |
| `graph.deberta.tokenizer_path` | `tokenizer.json` (e.g. HuggingFace export). |
| `graph.deberta.labels_path` | `labels.json` mapping indices to IOB labels. |
| `graph.deberta.max_seq_len` | Max sequence length (model default often `512` if `0`). |
| `graph.deberta.exec_provider` | ONNX Runtime provider: **`cpu`**, **`coreml`**, **`cuda`**, **`auto`**, etc. |
| `graph.deberta.model_precision` | `auto`, `fp32`, or `int8`. |

**Note:** `EngineConfig` (workers, four-tier, chunk concurrency, …) is not populated from this YAML in `initRuntime`; the engine is created with fixed `MaxWorkers: 4`. For advanced engine tuning, use the Go API (`engine.NewMemoryEngineWithStores` with your own `engine.EngineConfig`).

## Development

```bash
make test          # go test ./...
make test-race     # tests with -race
make coverage      # coverage.out + coverage.html
make lint          # golangci-lint
make fmt           # go fmt + goimports
make build         # go build ./... + repo root main package
make build-cli     # ai-memory-cli binary
make security      # gosec
make ci            # fmt, lint, test-race, build, security
```

Run `make help` for the full list.

## Architecture (high level)

```
CLI / MCP / examples
        ↓
   engine (MemoryEngine)
        ↓
 parser · schema · extractor · graph · vector · storage
        ↓
 adapters (SQLite, PostgreSQL, Neo4j, Redis, Qdrant, …)
```

## License

No `LICENSE` file is present in this repository yet; add one when the project chooses a license.

## Contributing

Contributing guidelines are not defined here yet.
