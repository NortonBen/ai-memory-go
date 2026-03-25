# Requirements: Agentic Request Update (Think & Delete)

## Overview
This feature expands the `engine.Request` method, evolving it from a passive memory persistence function into an active, intelligent boundary. When a user sends a message, the engine must distinguish between a statement (to remember), a query (to answer), and a deletion command (to forget).

## User Stories
1. **As a user**, I want to ask questions during my chat session, and the engine should "Think" to provide an answer based on its graph/vector contexts.
2. **As a user**, I want the engine to confirm what it learned if I make a factual statement, making the interaction feel conversational.
3. **As a user**, I want to command the engine to forget specific data (e.g., "Forget what I said about XYZ"), and the engine should remove the corresponding entities and data points.

## Acceptance Criteria
- [ ] `schema.RequestIntent` must include `IsQuery` (bool), `IsDelete` (bool), and `DeleteTargets` ([]string).
- [ ] LLM `ExtractRequestIntent` prompt must accurately identify questions vs statements vs deletion targets.
- [ ] `MemoryEngine.Request` must return `(*schema.ThinkResult, error)`.
- [ ] If `IsQuery` is true, the `Request` method internally calls `Think` with the user query, and returns the result.
- [ ] If `IsDelete` is true, the `Request` method searches for relevant graph nodes matching `DeleteTargets` and deletes them. It should also attempt to delete associated Vector `DataPoints`.
- [ ] If neither is true (standard statement), the `Request` method performs the usual graph/vector extraction, and returns a synthetic confirmation (e.g. "I have memorized this.")
- [ ] The `Request` method correctly handles compound requests (e.g. "Forget my old address, my new one is XYZ. Can you confirm?") or delegates to the most dominant intent.
