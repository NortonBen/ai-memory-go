# Requirements: SQLite-Vec Integration

## User Stories
- As a Developer, I want to use the official `sqlite-vec` Go bindings so that the vector store is more performant and easier to maintain.
- As a User, I want fast and accurate vector similarity searches in my AI memory.

## Acceptance Criteria
- [ ] The `SQLiteVectorStore` uses `github.com/asg017/sqlite-vec-go-bindings/cgo`.
- [ ] The migration logic correctly sets up the `vec0` virtual table.
- [ ] `StoreEmbedding` and `SimilaritySearch` maintain their current API and functionality.
- [ ] Existing tests in `sqlite_store_test.go` pass with the new implementation.
- [ ] No regressions in metadata filtering.
