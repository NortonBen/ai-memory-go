# Danh sách Công việc: Tích hợp Gemini

## Backend
- [ ] Mở rộng `extractor.ProviderType` thêm giá trị `ProviderGemini`.
- [ ] Thêm file `vector/gemini_embedder.go` tạo hàm khởi tạo `NewGeminiEmbeddingProvider`.
- [ ] Thêm file `extractor/gemini.go` chứa logic `NewGeminiProvider` tương thích OpenAI.
- [ ] Sửa `extractor/provider_factory.go` ở mệnh đề `switch ProviderType`, gọi `createGeminiProvider()`.
- [ ] Tạo module chạy minh hoạ: `examples/knowledge_bot_gemini/main.go`.

## Kiểm chứng (Testing)
- [ ] Test thử `GEMINI_API_KEY` gọi Endpoint `https://generativelanguage.googleapis.com/v1beta/openai/v1/chat/completions`.
- [ ] Viết Makefile hoặc Script hướng dẫn chạy `go run` example.
