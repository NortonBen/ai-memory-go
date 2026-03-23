# PgVector Adapter Requirements

## User Stories
1. Là một memory engine, tôi muốn dùng PostgreSQL với `pgvector` để lưu trữ vector embeddings.
2. Là hệ thống, tôi muốn tái sử dụng connection pool chung của Storage.

## Acceptance Criteria
- [ ] Implement `VectorStore` interface cho pgvector.
- [ ] Hỗ trợ index bằng HNSW hoặc IVFFlat.
- [ ] Unit & Integration tests.