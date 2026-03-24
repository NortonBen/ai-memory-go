# Architecture & System Design: Gemini Integration

## Tổng quan Thiết kế
Google Gemini gần đây đã hỗ trợ [OpenAI-compatible REST API](https://developers.googleblog.com/en/gemini-is-now-accessible-from-the-openai-library/). Do đó, thay vì tải trực tiếp thư viện Google SDKồng kềnh, ta sẽ tái sử dụng cấu trúc `BaseProvider` dùng giao thức OpenAI hiện hành.

### Data Flow
1. **Embedding**: Truyền văn bản qua cổng `https://generativelanguage.googleapis.com/v1beta/openai/` dùng chuẩn payload của OpenAI embedding (VD: mô hình `text-embedding-004`).
2. **Extraction Engine**: Dùng LLM `gemini-2.5-flash` nhận dữ liệu text qua cổng tương thích OpenAI Chat Completions API của hệ thống Gemini backend.
3. Các Factory pattern sẽ khởi tạo Provider theo `ProviderType = "gemini"`.

## Chi tiết Mở rộng Thư viện:
1. `extractor/gemini.go`: Định nghĩa Provider API bọc `OpenAIProvider` nhưng ép buộc config BaseURL mặc định.
2. `vector/gemini_embedder.go`: Tương tự LLM, định nghĩa Provider hỗ trợ vector cho Gemini Models.
3. `examples/knowledge_bot_gemini/`: Mã mẫu với `GEMINI_API_KEY`, khởi tạo MemoryEngine qua Gemini Vector và Gemini LLM.
