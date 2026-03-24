# Kiến trúc Hệ thống: Cập nhật Memify

## Tái thiết kế Pipeline (Pipeline Redesign)

Kiến trúc hiện tại:
`engine.Add` --> Gọi worker `AddTask` --> [ Vectorize Text + Extract Entities + Consistency Reasoning + Save Graph/Vector + Extract Edges + Save Graph + Mark Completed ]

Kiến trúc mới chia để trị (Divide & Conquer):
`engine.Add` --> `engine.Cognify` --> `engine.Memify`

### 1. The Cognify Protocol
*   Nhiệm vụ: Extract and Store Nháp (Draft).
*   Thực thi (`CognifyTask` trong worker_pool):
    *   Lấy Text từ `DataPoint`.
    *   Tạo Vector Embedding của Text lưu vào RelationalStore/DataPoint.
    *   Sử dụng LLM trích xuất `Entities`.
    *   Sử dụng LLM trích xuất `Relationships`.
    *   Serialization List Entities & Relationships lưu vào `DataPoint` (có thể dưới dạng cột JSONB (DataPoint.Nodes và DataPoint.Relationships) trong table `datapoints` của RelationalStore).
    *   Đổi Cột Status thành `StatusCognified`.

### 2. The Memify Protocol
*   Nhiệm vụ: Global Graph Promotion (Khắc sâu vào trí nhớ).
*   Thực thi (`MemifyTask` trong worker_pool):
    *   Đọc DataPoint với Status `Cognified`, lấy mảng `Nodes` và `Relationships`.
    *   Đối chiếu `Nodes` với hệ thống thông qua `Consistency Reasoning` (`VectorStore.SearchVector`, kiểm tra khoảng cách < 0.2).
    *   Phân loại `UPDATE`, `CONTRADICT`, `IGNORE`, `KEEP_SEPARATE`.
    *   Cập nhật các node vào `VectorStore` và `GraphStore`.
    *   Cạo (Scrape) mảng `Relationships` và push thẳng lên `GraphStore`.
    *   Đổi Cột Status thành `StatusCompleted`.

## Các Thay đổi Chi tiết:
*   `engine/engine.go`: Viết lại body của hàm `Memify` và `Cognify`.
*   `engine/worker_pool.go`: Giết bỏ `AddTask`, tạo mới hai Task `CognifyTask` và `MemifyTask`.
*   `schema/memory.go`: Bổ sung biến enum Status `StatusCognified` = "cognified". Đảm bảo Entity có thể encode JSON.
*   `storage/sqlite_adapter.go`: Cần chắc chắn `UpdateDataPoint` có thể lưu/cập nhật mảng Nodes và Relationships dạng JSON Text Type (hoặc thêm các hàm CreateDataPointNode tạm thời). *Lưu ý: Sqlite3 không có JSONB native cao cấp, nhưng text field lưu dạng json là hoàn hảo cho List Items Draft.*
