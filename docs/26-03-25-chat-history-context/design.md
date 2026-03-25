# Design: Chat History Context

## Architecture Additions

### 1. Schema Extensions (`schema/schema.go`)
- Create a `Message` struct to hold `Role`, `Content`, and `Timestamp`.
- Add an array of `Message` or references to message IDs in `schema.MemorySession`.

### 2. Storage Updates (`storage/interface.go`, `storage/postgres_adapter` etc.)
- Update the Storage providers to persist the new `Message` objects associated with a session.
- Implement methods like `AddMessageToSession` and `GetSessionMessages`.

### 3. Mixed Intent Routing (`engine/engine.go`)
- `engine.Request` will change its control flow. Instead of returning early after a delete or a query, it will:
    1. Check for `IsDelete` and execute targeted deletions.
    2. Check for `IsQuery` / `NeedsVectorStorage` / Statements.
    3. Generate the final output using a synthesized result that combines the outcome of multiple intents.
    4. Save the user's input request and the system's generated response to the session history.

### 4. Contextual Extraction (`extractor/basic_extractor.go`)
- Retrieve recent `Message`s from the storage provider and format them into the prompt for `ExtractRequestIntent`, `ExtractEntities`, and `ExtractRelationships` to resolve references.
