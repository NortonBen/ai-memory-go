# Design: History Multi-DB implementation

## Architecture
- Currently `AddMessageToSession` and `GetSessionMessages` are present in `storage.Storage` interface and implemented for SQLite.
- Need to extend `PostgresAdapter` in `storage/postgresql_adapter.go`.
- Table schema in PostgreSQL:
  ```sql
  CREATE TABLE IF NOT EXISTS session_messages (
      id VARCHAR(255) PRIMARY KEY,
      session_id VARCHAR(255) NOT NULL,
      role VARCHAR(50) NOT NULL,
      content TEXT NOT NULL,
      created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
  );
  CREATE INDEX IF NOT EXISTS idx_session_messages_session_id ON session_messages(session_id);
  ```

## Data Flow
- When `engine.Request` executes, `GetSessionMessages` retrieves history to inject into Prompt context.
- After processing, `engine.Request` executes `AddMessageToSession` twice (once User, once Assistant) to persist interactions.
