# Yêu cầu & Nghiệp vụ: Cập nhật tích hợp Gemini

Bổ sung API Provider của Google Gemini vào hệ thống AI Memory Brain thông qua chuẩn tương thích OpenAI REST endpoint (`https://generativelanguage.googleapis.com/v1beta/openai/`).
Từ đó có thể sử dụng sức mạnh của mô hình ngữ cảnh lớn (Gemini 2.5 Flash) cho các luồng Extractors và Vector Embedder.

## User Stories:
1. Là một lập trình viên AI, tôi muốn hệ thống hỗ trợ API của Gemini Engine thông qua endpoint tương thích OpenAI để tôi có thể dễ dàng chuyển đổi sang Google Model với chi phí rẻ và chất lượng nội dung tốt.
2. Là người dùng, tôi cần xem một file ví dụ `examples/knowledge_bot_gemini/main.go` mô tả đầu cuối cách tích hợp Gemini Extract/Embed vào MemoryEngine của dự án.

## Acceptance Criteria:
- Hệ thống cần mở rộng package `vector` để hỗ trợ custom hostname cho `GeminiEmbeddingProvider` qua chuẩn API OpenAI.
- Hệ thống cần mở rộng package `extractor` để hỗ trợ `ProviderGemini` và hàm `NewGeminiProvider` nhận API endpoint và model type chuẩn xác tương tích cho Gemini.
- Bổ sung `ProviderGemini` vào `ProviderFactory`.
- Demo hoạt động thành công quy trình Knowledge Graph Builder 4 bước (Add, Cognify, Memify, Search) bằng GEMINI_API_KEY.
