# Yêu cầu & Nghiệp vụ: Cập nhật Memify (Khắc sâu Trí Nhớ)

## Tóm tắt vấn đề
Hiện tại, đường ống AI Memory (Pipeline) gộp chung quá trình Trích xuất (Entity/Relationship Extraction) và Lưu trữ đồ thị cộng giải quyết xung đột (Consistency Reasoning) vào cùng một `AddTask` (trạng thái chạy ngầm sau khi gọi `Add`). Hàm `engine.Memify()` lúc này gần như vô tác dụng (chỉ đơn giản là cập nhật CSDL `ProcessingStatus = Completed`).
Do đó, hệ thống cần được tái cơ cấu để phân chia rõ ràng trách nhiệm của từng giai đoạn:
1. `Add`: Sinh ra DataPoint chứa văn bản thô, thiết lập trạng thái `Pending`.
2. `Cognify` (Xử lý Nhận thức): Liên kết hệ thống LLM để đọc text, trích xuất Entity và Relationships. Kết quả được nháp (draft) lưu trực tiếp vào DataPoint. Trạng thái `Cognified`.
3. `Memify` (Khắc sâu Trí nhớ): Quét các DataPoint ở trạng thái `Cognified`. Lấy các Entity draft, tiến hành nhúng Vector, chạy Consistency Reasoning (So sánh với trí nhớ cũ) rồi chèn các node/mối quan hệ cuối cùng vào `GraphStore` và `VectorStore`. Trạng thái thay đổi thành `Completed`.

## User Stories
* **Là hệ thống AI Core**, tôi muốn `Memify()` là kênh duy nhất tác động vật lý lên `GraphStore` và `VectorStore` (dùng Consistency Reasoning) để tránh việc tranh chấp dữ liệu (Race Condition) khi Trích xuất (Cognify) bị kéo dài.
* **Là Data Engineer**, tôi muốn chia đôi Task Pipeline (CognifyTask -> MemifyTask) trong `worker_pool` để dễ dàng Debug lỗi Extract riêng và lỗi Store riêng.

## Acceptance Criteria
1. Lệnh `engine.Cognify()` chỉ thực hiện extract Node/Relationship và gán chúng vào thuộc tính `Nodes` của DataPoint, sau đó cập nhật Status = `Cognified`.
2. Lệnh `engine.Memify()` nhận các `Cognified DataPoints`, thực hiện mã hoá Vector, duyệt Consistency Reasoning, cuối cùng Update `GraphStore` + `VectorStore` rồi đặt trạng thái `Completed`.
3. WorkerPool hỗ trợ chuyển tiếp tự động từ `CognifyTask` sang `MemifyTask`, giữ nguyên Flow `WaitAdd(true)` vẫn chạy trơn tru từ A-Z.
4. DataPoint giữ lại các mảng `Nodes` và `Relationships` trong Model Relational SQL (hoặc qua table phụ/ JSONB) để không bị bay màu khi tắt app giữa chừng sau khi Cognified.
