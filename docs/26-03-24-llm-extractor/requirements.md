# LLM Extractor Requirements

## User Stories
1. Là hệ thống Pipeline, tôi muốn dùng LLM (OpenAI, Ollama, DeepSeek) để trích xuất entity và text relationships.
2. Model trả về JSON theo schema định nghĩa sẵn.

## Acceptance Criteria
- [ ] Implement `LLMProvider` interface.
- [ ] Cơ chế Retry/Backoff.
- [ ] Hỗ trợ DeepSeek JSON Structured Mode.