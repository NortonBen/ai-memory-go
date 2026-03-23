# Danh sách công việc: Qdrant Adapter

## Giai đoạn 1: Chuẩn bị & Khởi tạo (Backend)

- [ ] Tải thư viện Go client của Qdrant bằng `go get github.com/qdrant/go-client/qdrant`.
- [ ] Thêm thư viện `go get github.com/google/uuid` để hỗ trợ convert ID.
- [ ] Khởi tạo file `vector/qdrant_adapter.go` và định nghĩa cấu trúc `QdrantStore`.
- [ ] Viết hàm constructor `NewQdrantStore(config *VectorConfig)` thiết lập kết nối (gRPC).
- [ ] Implement hàm `Health()` kiểm tra trạng thái cluster và hàm `Close()`.

## Giai đoạn 2: Quản lý Collection

- [ ] Xây dựng các hàm ánh xạ (Helper) parse String ID sang UUID (Qdrant ID).
- [ ] Implement `CreateCollection` cấu hình schema distance_metric, dimension.
- [ ] Implement `DeleteCollection`, `ListCollections`.
- [ ] Implement `GetCollectionInfo`, `GetEmbeddingCount` từ Telemetry của Qdrant.

## Giai đoạn 3: Triển khai CRUD Vector Point

- [ ] Cấu trúc chuẩn hóa `EmbeddingData` sang Qdrant `PointStruct`.
- [ ] Implement `StoreEmbedding` (Insert point) và `UpdateEmbedding` (Upsert point).
- [ ] Implement `GetEmbedding` phân tích Point trả về kèm theo Payload.
- [ ] Implement `DeleteEmbedding`.

## Giai đoạn 4: Quản lý Batch & Query

- [ ] Triển khai thao tác lô với `StoreBatchEmbeddings`.
- [ ] Triển khai xoá lô với `DeleteBatchEmbeddings` (mảng IDs).
- [ ] Tạo hàm helper để convert filters `map[string]interface{}` thành `qdrant.Filter`.
- [ ] Triển khai `SimilaritySearch` (không filter).
- [ ] Triển khai `SimilaritySearchWithFilter` (Kèm filter payload).

## Giai đoạn 5: Testing (Kiểm thử)

- [ ] Bổ sung các test functions trong thư mục `vector` (ví dụ: `qdrant_adapter_test.go`).
- [ ] Bổ sung integration test với Testcontainers-go module Qdrant, hoặc viết hướng dẫn chạy local.
