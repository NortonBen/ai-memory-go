# Requirements: Agentic Chat History Example

## User Stories
- As a developer exploring the `ai-memory-go` project, I want to see a working example of the new `engine.Request` chat history injection and multi-intent (Query/Statement/Delete) router.
- As a user, I want to run a simple CLI application that retains my identity and resolves pronouns like "he", "it", "this" across multiple turns because my conversational history is saved.

## Acceptance Criteria
- A new example directory is created: `examples/chat_history_agent`.
- The example initializes a memory engine using an LLM Provider (e.g. OpenAI or Ollama).
- It runs a conversational CLI loop where a user can enter statements, queries, and deletion commands.
- Pronouns must correctly resolve referencing previous facts stored in the session memory.
- Deletion targets should properly drop facts from the memory graph and verify missingness on next query.
