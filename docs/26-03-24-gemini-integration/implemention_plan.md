# Kế hoạch Thực thi (Implementation Plan): Gemini Integration

## Đánh giá Trạng thái
Cần người dùng gõ "Approve" để AI thực thi code.

## Proposed Changes
### Component `extractor`
#### [MODIFY] `extractor/extractor.go`
- Bổ sung `ProviderGemini ProviderType = "gemini"` vào danh sách Constant Enum.

#### [NEW] `extractor/gemini.go`
- Định nghĩa hàm `NewGeminiProvider` bọc HTTP logic của OpenAI API.

#### [MODIFY] `extractor/provider_factory.go`
- Thêm routing cho tuỳ chọn Init "gemini" Factory.

### Component `vector`
#### [NEW] `vector/gemini_embedder.go`
- Định nghĩa Struct bọc logic Embedding thông qua OpenAI URL của Gemini.

### Demos
#### [NEW] `examples/knowledge_bot_gemini/main.go`
- Scripts test hệ luỵ của Engine từ đầu đến cuối với `gemini-2.5-flash`.

## Automation
- Sẽ gọi trực tiếp Request tới OpenAI endpoint do Google quản lý.
