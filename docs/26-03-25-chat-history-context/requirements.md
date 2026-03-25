# Requirements: Chat History Context

## Business Need
Users often refer back to entities mentioned in previous responses (e.g., "delete that" or "her age is 25"). They also issue mixed intent requests like "Forget about Alice, and who is Bob?". The system currently evaluates requests in isolation.

## User Stories
1. **As a User**, I want to include multiple intents in a single chat (e.g. asking a question and saving a fact) so that I don't have to separate my thoughts.
2. **As a User**, I want the LLM to understand context from previous chat messages so that I can use pronouns or refer to recent topics.
3. **As a System Administrator**, I want chat histories (both user requests and system responses) stored persistently so they can be analyzed later.
4. **As the System**, I want to periodically re-analyze previous chat history to extract deeper relationships that might have been missed in real-time.

## Acceptance Criteria
- `engine.Request` must process mixed intents by continuing execution rather than early returning.
- The system must capture and store the `User` and `Assistant` conversation history in a `MessageStore` or directly within `MemorySession`.
- `engine.Request` must inject recent conversation history into the `ExtractEntities`, `ExtractRelationships`, and `Think` prompts.
- A background or explicit trigger must exist to re-evaluate history for `ExtractRelationships`.
