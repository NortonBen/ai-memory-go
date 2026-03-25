# Tasks: Agentic Chat History Example

## Phase 1: Setup
- [ ] [EX-HIST-001] Copy base structure from `examples/quickstart` into `examples/chat_history_agent`.

## Phase 2: Implementation
- [ ] [EX-HIST-002] Modify the generic example to loop `bufio.NewScanner(os.Stdin)` and accept inputs repeatedly.
- [ ] [EX-HIST-003] Set a fixed `sessionID` so the memory `engine.Request` persists context over multiple requests and restarts.
- [ ] [EX-HIST-004] Print diagnostic info (intent resolved or success status) before the response to demonstrate the router logic visually.

## Phase 3: Testing
- [ ] [EX-HIST-005] Run the example manually, input several facts, ask a contextual query using a pronoun, and ensure it correctly references the intent history.
