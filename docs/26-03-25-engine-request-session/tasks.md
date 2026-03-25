# Tasks: engine.Request Implementation

## Backend (Engine Core)
- [x] [ENGINE-001] Mở rộng `schema.ExtractionResult` để hỗ trợ cờ `NeedsVector`.
- [x] [ENGINE-002] Cập nhật interface `MemoryEngine` trong `engine/interface.go` thêm hàm `Request`.
- [x] [ENGINE-003] Triển khai `RequestOption` và các helper function liên quan.
- [x] [ENGINE-004] Cập nhật Prompt cho `LLM Extractor` để nhận diện ý định lưu trữ (Vector storage intent).
- [x] [ENGINE-005] Implement logic cho `Request` trong `internal/engine/engine.go`:
    - Gọi Extractor.
    - Cập nhật Graph (Entities/Relations).
    - Kiểm tra `NeedsVector` và xử lý lưu trữ Vector Store.
- [x] [ENGINE-006] Viết unit test cho hàm `Request`.

## Documentation
- [x] Cập nhật README hoặc hướng dẫn sử dụng API cho hàm `Request`.
