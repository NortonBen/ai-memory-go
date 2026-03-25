# Requirements: Context-Aware Request Pipeline

## User Stories

- **As a User**, I want the AI to understand who "I" am and who "others" are in our conversation history.
- **As a Developer**, I want the engine to automatically build a relationship graph between the bot, the current user, and mentioned entities.

## Business Rules

- Before any memory extraction, the engine MUST fetch recent chat history.
- The engine MUST analyze relationships (Bot-User, User-User) from the current message and local context.
- Relationships MUST be stored in the Knowledge Graph as a "relationship graph" before processing queries.
- Queries MUST use the constructed relationship graph to resolve ambiguous references (e.g., "my dog" -> "Alice's dog").

## Acceptance Criteria
- [ ] The engine correctly identifies the "current" user and the "bot" in the relationship graph.
