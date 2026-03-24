# Architecture and Design

## 1. Overview
The CLI Interface provides a command-line wrapper around the existing `MemoryEngine` (`ai-memory-go`), exposing the core functionalities (add, cognify, search, delete) to users in a terminal environment without duplicating business logic.

## 2. Technology Choices
- **CLI Framework:** `github.com/spf13/cobra` 
  - Standard in Go ecosystem. Built-in support for nested routes, flag parsing, and bash completion.
- **Configuration Parsing:** `github.com/spf13/viper` 
  - Loads configuration robustly from multiple sources: JSON/YAML files, ENV variables, and CLI Flags.
- **Output Styling:** 
  - Colors/Terminal formatting: `github.com/fatih/color`
  - Progress bar: `github.com/schollz/progressbar/v3` (for cognify jobs)

## 3. Component Architecture
- **Command Parser (Cobra Tree):** 
  - Defines commands (`add`, `cognify`, `search`, `delete`, `config`, `session`) and handles usage strings and auto-generated help menus.
- **Config Manager:**
  - Loads config from `~/.ai-memory/config.yaml` or current directory. Creates the internal `config.Config` required to instantiate the `MemoryEngine`.
- **Memory Engine Adapter:**
  - A lightweight abstraction in the CLI package that initializes connections to PostgreSQL/SQLite, LLM models (OpenAI/Ollama), and Qdrant/Weaviate via the existing project structure based on viper config.
- **Output Formatter:**
  - Standardized JSON, YAML, or pretty-table printing for search results. Handles `--format` flag globally.

## 4. Command Specifications
1. `ai-memory-cli add <content> [--file path] [--url path]`
   - Parses flags, reads text, delegates to `MemoryEngine.Add()`.
2. `ai-memory-cli cognify [--id ID]`
   - Without ID: calls `MemoryEngine.Cognify()` to process all pending nodes. Uses progress bar to stream status if possible.
3. `ai-memory-cli search <query> [--limit int] [--mode string]`
   - Calls `MemoryEngine.Search()`. Reformats the `SearchResult` struct to CLI output.
4. `ai-memory-cli delete [--id ID] [--all] [--session ID] [--force]`
   - Uses terminal prompt for confirmation unless `--force` is used.
5. `ai-memory-cli session [list|create|switch]`
   - Mutates a local state file indicating the current active workspace/session.
6. `ai-memory-cli config [--init]`
   - Dumps JSON/YAML config or initializes a template.

## 5. Error Handling
- Use structured Go errors but map them to human-readable colored output.
- Avoid stack traces unless `--verbose` flag is passed globally.
