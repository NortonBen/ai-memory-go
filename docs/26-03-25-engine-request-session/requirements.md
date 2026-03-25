# [engine.Request] Session Chat Memory & Learning

Hàm `engine.Request` được thiết kế để xử lý các lượt hội thoại trong một session chat, trích xuất kiến thức (thực thể, quan hệ) để cập nhật vào Graph Store, và chỉ lưu vào Vector Store nếu người dùng yêu cầu rõ ràng.

## User Stories
- **As a User**, I want the system to remember the context of our conversation (relationships, facts mentioned) so that it can provide more relevant responses later.
- **As a User**, I want to decide when a piece of information should be stored as a permanent document/knowledge base (Vector Store) versus just keeping the relationship context (Graph Store).
- **As a Developer**, I want a single function `Request` to handle the learning process from chat sessions without manually managing graph and vector storage logic.

## Acceptance Criteria
- [ ] Hàm `engine.Request` chấp nhận nội dung chat (có thể bao gồm tin nhắn user và phản hồi bot).
- [ ] Sử dụng Extractor để phân tích nội dung chat.
- [ ] Luôn trích xuất Entities/Relations và cập nhật vào Graph Store.
- [ ] Chỉ lưu vào Vector Store nếu Extractor phát hiện ý định "ghi nhớ/lưu trữ tài liệu" từ người dùng.
- [ ] Hỗ trợ `SessionID` để phân tách ngữ cảnh hội thoại.
- [ ] Phải học được từ phản hồi (feedback-ready structure).

## Business Rules
- Không tự động lưu vào Vector Store trừ khi có yêu cầu (Save cost & noise).
- Graph Store là nơi lưu trữ kiến thức kết nối chính cho Context-base reasoning.
