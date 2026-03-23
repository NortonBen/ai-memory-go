# SQLite Adapter Requirements

## User Stories
1. Là ứng dụng Desktop, tôi muốn lưu relational data bằng file cục bộ SQLite.

## Acceptance Criteria
- [ ] Không phụ thuộc CGO hoặc dùng mattn/go-sqlite3.
- [ ] Implement `RelationalStore`.
- [ ] Hỗ trợ lock management tốt cho concurrency.