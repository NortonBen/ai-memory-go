# Design: engine.Request Session Memory

## Architecture

Hàm `Request` sẽ là một phương thức cấp cao trên `MemoryEngine`, phối hợp giữa `Extractor`, `GraphStore`, và `VectorStore`.

### Data Flow
1. **Input**: `ctx`, `content` (chat history/interaction), `options` (SessionID, metadata).
2. **Extraction**: 
   - Gọi `Extractor.ExtractRequest(content)` (Cần thêm phương thức này hoặc tham số mới cho Extractor) để nhận về:
     - `Entities`: Danh sách thực thể.
     - `Relations`: Quan hệ giữa các thực thể.
     - `NeedsVectorStorage`: Boolean (True nếu user yêu cầu lưu trữ tài liệu).
3. **Graph Storage**: 
   - Chuyển đổi Entities/Relations thành các DataPoints hoặc gọi trực tiếp Graph Store để cập nhật.
   - Sử dụng cơ chế `Memify` hiện có để hợp nhất kiến thức.
4. **Vector Storage** (Conditional):
   - Nếu `NeedsVectorStorage == true`, thực hiện `Add` nội dung vào Vector Store thông qua pipeline thông thường.

## API Changes

### MemoryEngine Interface
```go
type MemoryEngine interface {
    // ... hiện có
    Request(ctx context.Context, content string, opts ...RequestOption) error
}
```

### RequestOptions
```go
type RequestOptions struct {
    SessionID string
    Metadata  map[string]interface{}
}
```

### Extractor Interface (Cần xem xét mở rộng)
Cần cập nhật `schema.ExtractionResult` để bao gồm trường `NeedsVectorStorage`.

## Data Model Updates
- `schema.ExtractionResult` nên được bổ sung thêm Metadata hoặc Intent flags.
