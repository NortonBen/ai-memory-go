# Engine Core Design

## Kiến trúc
- `MemoryEngine` struct chứa `Extractor`, `Embedder`, `StorageFactory`.
- Xử lý concurrency với Goroutines qua WorkerPool.