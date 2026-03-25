# Implementation Plan - Chat History Bug Fix

## Proposed Changes

### [Extractor] [extractor/basic_extractor.go](file:///Users/benji/Projects/ai-memory-brain/extractor/basic_extractor.go)
- **Refine Prompt**: Update `ExtractRequestIntent` to include `HISTORY` block and explicitly instruction LLM to resolve pronouns (e.g., "nó", "it").
- **Example Added**: Add specific example `If history says neighbor has dog Black, and last message is 'it's 1y old', link HAS_AGE to Black`.

### [Engine] [engine/request.go](file:///Users/benji/Projects/ai-memory-brain/engine/request.go)
- **findOrCreateEntityNode**: Before creating a new node, search for nodes with property `name` matching the input. If not found, search `entity` property.
- **Cognify**: Call `Cognify` immediately after storing new entities to ensure vector search works for recent additions.

### [Engine] [engine/search.go](file:///Users/benji/Projects/ai-memory-brain/engine/search.go)
- **Context Injection**: In `retrieveContext`, prepend recent history (last 6 messages) to the LLM generation prompt even if no graph/vector results found.

## Verification Plan

### Automated Tests
- `go run ./scripts/test_dog_repro.go`: New script that uses `MemoryEngine.Request` to simulate:
  1. "Hàng xóm có con chó Đen"
  2. "nó 1 tuổi"
  3. "chó hàng xóm thế nào"
- Verify final answer mentions "Đen" and "1 tuổi".

### Manual Verification
- Run `go run ./examples/chat_history_agent/main.go` and type the same sequence.
