# Tasks - Chat History Bug Fix

## Backend Tasks
- [ ] Update `ExtractRequestIntent` prompt in `extractor/basic_extractor.go` to explicitly require pronoun resolution from history.
- [ ] Refactor `engine/request.go:findOrCreateEntityNode` to use both `name` and `entity` property searches to prevent duplicate nodes.
- [ ] Modify `engine/search.go:retrieveContext` to include a fixed window of recent chat history (e.g., 6 messages) in the final LLM prompt.
- [ ] Add a `Cognify` call in `engine/request.go` for all newly extracted entities to ensure they are immediate searchable via vector.

## Verification Tasks
- [ ] Create `scripts/test_dog_repro.go` to simulate the user's specific scenario.
- [ ] Run the repro script and verify it passes.
- [ ] Restart `chat_history_agent` (clean DB) and manually verify the fix.
