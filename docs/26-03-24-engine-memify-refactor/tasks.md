# Danh sách Công việc (Tasks)

## Backend / Core Engine

### Schema Updates
- [ ] Thêm trạng thái `StatusCognified` (cognified) vào enum trong `schema/memory.go` (hay file tương ứng khai báo Status).
- [ ] Đảm bảo `DataPoint` struct có hỗ trợ Serialize cho Properties (Nodes và Relationships) hoặc bổ sung trường JSON `NodesJSON`, `EdgesJSON`.

### Storage / Adapter Updates
- [ ] Cập nhật `storage/sqlite_adapter.go`: `CreateDataPoint` và `UpdateDataPoint` chèn được trường Node(s) / Edge(s) dưới dạng chuỗi JSON thô (Raw Message) vào CSDL nếu DataPoint struct thay đổi thiết kế chứa Graph phụ.
- [ ] (Tuỳ chọn) Đảm bảo truy vấn `GetDataPoint` lấy đầy đủ data Nodes/Edges.

### Worker Pool Updates (engine/worker_pool.go)
- [ ] Tách `AddTask` thành `CognifyTask`. (Chỉ thực hiện LLM extraction, không dính líu GraphStore. Lưu mảng result vào DataPoint. Tự động Enqueue `MemifyTask`).
- [ ] Sinh mới `MemifyTask`. (Đọc mảng Entities từ DataPoint, chạy vòng lặp Consistency Reasoning (Compare Entities qua Vector DB). Chèn vô GraphDB. Chèn Relashionships).

### Engine Interface (engine/engine.go)
- [ ] Chỉnh sửa `engine.Add`: Chỉ insert `DataPoint` pending. Đẩy task tuỳ theo mode sync/async (WaitAdd=true).
- [ ] Chỉnh sửa `engine.Cognify`: Queues `CognifyTask`.
- [ ] Chỉnh sửa `engine.Memify`: Queues `MemifyTask` hoặc run code trực tiếp phụ thuộc thiết kế. Đổi status cuối.

### Examples & Verification
- [ ] Thay đổi cấu hình LM Studio trong `examples/.../main.go` nếu cần để tránh lỗi Embedder không hoạt động khi Qwen load.
- [ ] Kiểm thử e2e với file `examples/knowledge_graph_builder/main.go`, đảm bảo in ra log đúng thứ tự: ADDED -> COGNIFIED -> MEMIFIED.
