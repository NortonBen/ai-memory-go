# Tasks: History Multi-DB implementation

## Phase 1: Database Setup
- [x] [POSTGRES-HIST-001] Update `PostgresAdapter.setupTables` to create `session_messages` table and index.

## Phase 2: Implementation
- [x] [POSTGRES-HIST-002] Implement `AddMessageToSession` in `postgresql_adapter.go`.
- [x] [POSTGRES-HIST-003] Implement `GetSessionMessages` in `postgresql_adapter.go` fetching ordered by `created_at ASC`.

## Phase 3: Testing
- [x] [POSTGRES-HIST-004] Run relational storage test suite (`go test ./storage/...`) to verify SQLite and PostgreSQL adapter behavior.
