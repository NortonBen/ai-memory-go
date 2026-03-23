# PgVector Adapter Design

## Kiến trúc
- `PgVectorStore` bọc quanh connection pool của `database/sql` hoặc thư viện ORM như `pgx` hoặc `gorm` có hỗ trợ vector.
- Table schema sẽ có cột dạng `vector(d)` (với d là dimensions).

## API Mapping
- `StoreEmbedding` -> `INSERT INTO... ON CONFLICT DO UPDATE`
- `SimilaritySearch` -> `ORDER BY embedding <-> $1 LIMIT $2`