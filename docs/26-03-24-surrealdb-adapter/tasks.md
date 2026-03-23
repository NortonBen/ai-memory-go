# SurrealDB Adapter Implementation Tasks

## Giai đoạn 1: Khởi tạo Storage Adapter
- [ ] Thêm dependencies driver `go get github.com/surrealdb/surrealdb.go`.
- [ ] Viết file `graph/surrealdb_adapter.go` định nghĩa struct `SurrealDBStore`.
- [ ] Implement hàm kết nối (dùng WS/HTTP connection), `Health()`, và `Close()`.

## Giai đoạn 2: Phát triển Nodes & Edges Operations (CRUD)
- [ ] Viết hàm `StoreNode` và `GetNode` (dùng SurrealQL).
- [ ] Viết hàm `UpdateNode` và `DeleteNode`.
- [ ] Viết hàm `CreateRelationship` (sử dụng câu lệnh `RELATE`).
- [ ] Viết hàm `GetRelationship`, `UpdateRelationship`, `DeleteRelationship`.

## Giai đoạn 3: Phát triển Graph Traversal & Analytics
- [ ] Implement query `TraverseGraph` sử dụng cú pháp `->edge->node` của SurrealQL.
- [ ] Implement `FindConnected` và `FindPath`.
- [ ] Implement `FindNodesByType`, `FindNodesByProperty`, `FindNodesByEntity`.
- [ ] Bổ sung các hàm lấy metrics `GetNodeCount`, `GetEdgeCount`.

## Giai đoạn 4: Quản lý Batch & Giao dịch
- [ ] Triển khai `StoreBatch` cho nodes (dùng câu lệnh INSERT multi hoặc Transaction).
- [ ] Triển khai `StoreBatch` cho edges.
- [ ] Triển khai `BeginTransaction`, `Commit`, `Rollback` theo chuẩn SurrealQL.

## Giai đoạn 5: Testing (Kiểm thử)
- [x] Viết các unit tests kiểm thử cú pháp query SurrealQL sinh ra.
- [x] Bổ sung integration testing (local instance hoặc mock db).
