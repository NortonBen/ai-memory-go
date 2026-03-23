# Qdrant Adapter Design (Kiến trúc đồ thị)

## 1. Kiến trúc tổng quan (Architecture)

Qdrant Adapter sẽ đóng vai trò là implementation của `vector.VectorStore`, tích hợp qua `VectorFactory`.
Sử dụng Go client chính thức của Qdrant (`github.com/qdrant/go-client/qdrant`).

**Cấu trúc chính:**

```go
type QdrantStore struct {
    client     qdrant.QdrantClient
    config     *vector.VectorConfig
    collection string
}
```

## 2. Model & Data Mapping

Vector database (Qdrant) sẽ map các entity `EmbeddingData` như sau:

- **Point ID**: Qdrant hỗ trợ ID là số nguyên (64-bit) hoặc UUID. Do engine sử dụng `string ID`, Adapter sẽ sinh ra UUID version 5 dựa vào string ID để tạo tính nhất quán.
- **Vectors**: Lưu đồ thị vector kích thước theo config (ví dụ: 768, 1536). Thường kiểu dữ liệu `[]float32`.
- **Payload**: Thông tin `Metadata` được ánh xạ trực tiếp (map[string]interface{}) sang Payload của Qdrant.

## 3. Luồng dữ liệu (Data Flow)

**3.1 Khởi tạo kết nối (Initialization):**
Sử dụng `qdrant.NewClient()` để tạo kết nối gRPC. Cấu hình host, port, APIKey sẽ đọc từ `VectorConfig`.

**3.2 Insert/Update Embedding:**

- Tạo point: Sinh ID dưới dạng UUID từ chuỗi.
- Đóng gói vector và payload (metadata).
- Gọi hàm `Upsert` của client.

**3.3 Tìm kiếm SimilaritySearch:**

- Xây dựng object `QueryPoints` hoặc `Search`.
- Nếu có filters (trường hợp `SimilaritySearchWithFilter`), parse `map[string]interface{}` thành các điều kiện `Must` hoặc `Should` trong Filter của Qdrant.
- Convert kết quả trả về từ Qdrant (Điểm số, Payload) thành list các object `vector.SimilarityResult`.

## 4. Dependencies

- Driver: `github.com/qdrant/go-client`
- Công cụ sinh UUID: Có thể dùng `github.com/google/uuid` để mã hóa string -> UUID.
