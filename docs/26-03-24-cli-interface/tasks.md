# Tasks

## Setup
- [x] Initialize Cobra and Viper dependencies in `go.mod` (`go get github.com/spf13/cobra@latest github.com/spf13/viper@latest github.com/fatih/color@latest github.com/schollz/progressbar/v3@latest`)
- [x] Create `cmd/ai-memory-cli/main.go` entry point.
- [x] Set up root command (`internal/cli/root.go`).

## Config Management
- [x] Implement `internal/cli/config.go` with Viper integration for initializing memory config.
- [x] Add `config` subcommand (`config.go`) with `--init`.

## Commands
- [x] Implement `add` subcommand (`internal/cli/add.go`) - handles text, file, url input.
- [x] Implement `cognify` subcommand (`internal/cli/cognify.go`) - handles background processing, progress bars.
- [x] Implement `search` subcommand (`internal/cli/search.go`) - parses search params, handles output formatting (JSON/table).
- [x] Implement `delete` subcommand (`internal/cli/delete.go`) - requires confirmation prompt unless `--force`.
- [x] Implement `session` subcommand (`internal/cli/session.go`) - manages active session states locally.

## Polish
- [x] Add help text and examples for all commands.
- [x] Handle error states gracefully displaying colored errors.
