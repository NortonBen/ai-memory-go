# Implementation Plan: Chat History Context

## Goal Description
Enhance `engine.Request` to handle multiple, mixed intents, seamlessly store conversational history (user and assistant), and use that history to inject rich context into LLM extractions.

## Proposed Changes

### Storage Layer
- Modify `schema/schema.go` to introduce `Message`.
- Extend JSON/SQL adapters to support storing chat messages associated with a session ID.

### Engine Routing (`engine/engine.go`)
- **[MODIFY] defaultMemoryEngine.Request**:
    - Fetch recent history from `e.store.GetSessionMessages(ctx, sessionID)`.
    - Pass history to the `LLMExtractor`.
    - Remove the `return ...` inside the `if intent.IsDelete` conditional to allow the pipeline to continue.
    - Append the final `schema.ThinkResult` response back into the history store.

### Extractor Prompts
- **[MODIFY] extractor/basic_extractor.go**: Modify all system prompts to accept `<CHAT_HISTORY>` blocks for contextual anchor point resolution.

## Verification Plan
### Automated Tests
- Create an `engine_test.go` test where a user says "Forget about Alice. What is Bob's age?".
- Verify that both the graph deletion takes place and the `ThinkResult` contains the Answer about Bob.
- Create tests for contextual extraction where a follow-up request uses a pronoun referencing a prior mock message.

### Manual Verification
Ensure that running `go run cmd/main.go chat` appropriately retrieves context for conversations involving reference chaining.
