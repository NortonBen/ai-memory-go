# Qdrant Vector Storage Adapter Requirements

## Yêu cầu người dùng (User Stories)

1. **Là một memory engine**, tôi muốn sử dụng Qdrant làm vector database để lưu trữ điểm dữ liệu (data points) cùng vector embeddings tương ứng, từ đó có thể tìm kiếm dữ liệu giống nhau (similarity search) với độ trễ thấp.
2. **Là hệ thống**, tôi muốn có thể kết nối đến Qdrant thông qua gRPC hoặc HTTP (có hỗ trợ TLS/API Key) để đảm bảo đồng bộ, bảo mật và hiệu năng trên cả môi trường local lẫn cloud.
3. **Là một nền tảng memory liên tục**, tôi muốn có thể tìm kiếm vector (Similarity Search) kết hợp với các bộ lọc Metadata (Filtering) để chỉ trả về những kết quả chính xác theo ngữ cảnh (session ID, loại nội dung).
4. **Là một ứng dụng có hiệu năng cao**, tôi muốn hỗ trợ lưu trữ và xoá dữ liệu theo lô (Batch Operations) nhằm tối ưu hoá khi insert dữ liệu hàng loạt.

## Tiêu chí nghiệm thu (Acceptance Criteria)

- [ ] Adapter kết nối thành công đến Qdrant Server sử dụng thư viện Go client chính thức của Qdrant (`github.com/qdrant/go-client`).
- [ ] Phải implement đầy đủ interface `VectorStore` (đã khai báo trong `vector/vector.go`).
- [ ] Triển khai các hàm CRUD Vector cơ bản: `StoreEmbedding`, `GetEmbedding`, `UpdateEmbedding`, `DeleteEmbedding`.
- [ ] Triển khai được tính năng tìm kiếm: `SimilaritySearch` và `SimilaritySearchWithFilter`.
- [ ] Triển khai các toán tử quản lý Collection: `CreateCollection`, `DeleteCollection`, `ListCollections`, `GetCollectionInfo`.
- [ ] Quản lý kết nối gRPC/HTTP tốt, hỗ trợ Close connection và Health check.
- [ ] ID dạng chuỗi (String) của memory engine cần được hash/chuyển đổi thành UUID hoặc integer hợp lệ với chuẩn của Qdrant (PointId).
