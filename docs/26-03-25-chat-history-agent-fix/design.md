# Design

## Overview
The chat history agent fails to retrieve stored information when queried (e.g., "chó nhà tôi thế nào"). The issue stems from:
1. **Pronoun resolution** – the extractor does not correctly map pronouns to entities across multiple messages.
2. **Context assembly** – `retrieveContext` does not include recent chat history when forming the query for the LLM.
3. **Relationship processing** – `processRelationships` may create duplicate nodes or miss linking ownership.

## Proposed Changes
- **Extractor Prompt**: Extend `ExtractRequestIntent` prompt to enforce pronoun resolution using full chat history.
- **Request Flow**: Ensure the history is passed to both intent extraction and entity extraction (`extractionInput`). Verify ordering.
- **Graph Store**: Update `findOrCreateEntityNode` to search by both `name` and `entity` properties, and avoid duplicate nodes.
- **Search Pipeline**: In `retrieveContext`, increase the number of recent messages considered (e.g., last 6 messages) and ensure they are included in the LLM prompt for answer generation.
- **Testing**: Add integration test that simulates the conversation sequence and asserts the correct answer is returned.

## Files Affected
- `engine/request.go`
- `engine/search.go`
- `extractor/basic_extractor.go`
- Add new test file `examples/chat_history_agent/main_test.go`

## Verification Plan
1. Run the example with the updated code.
2. Verify that after the conversation, querying about the dog returns the stored attributes (name, age, owner).
3. Ensure no panics and that vector/graph scores are combined correctly.
