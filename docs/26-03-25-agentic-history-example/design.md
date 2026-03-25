# Design: Agentic Chat History Example

## Architecture
- Will be located at `examples/chat_history_agent`.
- Use `sqlite_adapter` for persistent local storage, ensuring both graph structure and `session_messages` persist.
- Use `OpenAIProvider` or a default generic `LLMProvider` config pointing to a reliable multi-turn local model like Ollama or a `.env` configured OpenAI API.

## Data Flow
1. Load configuration (SQLite, LLM, Vector).
2. Initialize `engine.DefaultEngine` using `builder`.
3. Create a unique/fixed `sessionID` (e.g. `user_session_1`) so memory persists across restarts of the executable.
4. Loop:
   - Read from `os.Stdin`
   - Yield break on `/exit`
   - Feed `engine.Request(ctx, sessionID, input)`
   - Print `result.Answer` or `result.Message` based on `IsThink` boolean.
   - If error, print error and continue.
