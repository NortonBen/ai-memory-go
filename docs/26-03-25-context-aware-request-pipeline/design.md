# Design: Context-Aware Request Pipeline

## Architecture Overview
The `Request` pipeline in `engine/request.go` will be restructured to prioritize context and relationship analysis.

### New Pipeline Flow
1. **Context Fetching**: Retrieve recent chat history for the session.
2. **Intent Classification & Relationship Extraction**:
   - Send current message + history to LLM.
   - LLM classifies Intent (Statement, Query, Delete) AND extracts potential relationships (bot-user, user-user).
3. **Relationship Processing**:
   - If relationships are found, run a dedicated analysis to update the Knowledge Graph.
4. **Action Routing**:
   - **Statement**: Cognify/Save memory.
   - **Delete**: Execute deletion logic.
   - **Query**:
     - *If relationship analysis was needed, wait for it to complete.*
     - Proceed to `Think` phase using the updated graph context.

## Component Changes

### 1. `schema/schema.go`
- Update `RequestIntent` to include a `Relationships` field (e.g., `[]Relationship`).

### 2. `extractor/basic_extractor.go`
- Modify `ExtractRequestIntent` prompt to specifically look for "who is talking to whom" and "who is mentioned".
- Define the relationship types: `BotToUser`, `UserToUser`.

### 3. `engine/request.go`
- Reorder logic in `Request()` method.
- Implement the "defer" mechanism for queries waiting on relationship analysis.

## Data Flow Diagram
```mermaid
sequenceDiagram
    participant U as User
    participant E as Engine
    participant S as Search/Context
    participant L as LLM Extractor
    participant G as Graph Store
    participant T as Think Pipeline

    U->>E: Message (e.g., "cho nhà tôi tên gì")
    E->>S: Fetch History
    S-->>E: History
    E->>L: Classify Intent + Extact Relationships (History + Message)
    L-->>E: Intent (Query), Relationships (Finds "nhà tôi có chó Vàng")
    
    rect rgb(200, 230, 255)
        Note over E,G: Relationship Analysis Phase
        E->>G: Update Graph with Relationships
    end
    
    Note over E,T: Query deferred until Graph updated
    
    E->>T: Think (using updated Graph)
    T-->>E: Answer
    E->>U: Result
```
