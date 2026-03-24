# Implementation Plan

## Goal Description
Implement a robust, developer-friendly command-line interface tool (`ai-memory-cli`) that allows terminal access to the core ai-memory-go Engine for knowledge ingestion, processing, and retrieval.

## User Review Required
> [!NOTE]  
> Please review the chosen dependencies (`cobra`, `viper`, `color`, `progressbar`) to make sure they align with project standards.

## Proposed Changes

### CLI Setup & Config Manager
- Install requested dependencies (`spf13/cobra`, `spf13/viper` etc).
#### [NEW] `cmd/ai-memory-cli/main.go`
- Entry point. Evaluates root command.
#### [NEW] `internal/cli/root.go`
- Builds the cobra RootCmd. Sets up global persistent flags (`--config`, `--format`, `--verbose`, `--session`).
#### [NEW] `internal/cli/config.go`
- Viper initialization, config struct mappings, and the `config` subcommand (e.g., `--init`).
#### [NEW] `internal/cli/client.go`
- Helper struct/functions to initialize `MemoryEngine` inside commands from Viper. 

### CLI Commands
#### [NEW] `internal/cli/add.go`
- Implements `add` cobra.Command. Reads string arg, file, or URL.
#### [NEW] `internal/cli/cognify.go`
- Implements `cognify` cobra.Command. Can process a single node ID or global. Uses progress bar.
#### [NEW] `internal/cli/search.go`
- Implements `search` cobra.Command with mode flags. Calls Search and formats output.
#### [NEW] `internal/cli/delete.go`
- Implements `delete` cobra.Command. Checks for `--force` flag. Interactive terminal prompt otherwise.
#### [NEW] `internal/cli/session.go`
- Implements `session` command handlers to store local context.

## Verification Plan

### Automated Tests
- Unit tests for CLI flag parsing functions in `internal/cli_test/` using cobra's native buffer testing mechanisms.

### Manual Verification
1. `go build -o ai-memory-cli ./cmd/ai-memory-cli/main.go`
2. Run `ai-memory-cli config --init` to write a basic boilerplate config.
3. Edit config to point to SQLite + Ollama.
4. Run `ai-memory-cli add "Golang is awesome"` -> Success message.
5. Run `ai-memory-cli cognify` -> Progress bar displayed.
6. Run `ai-memory-cli search "Go"` -> Shows relevant results.
