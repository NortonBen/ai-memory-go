package extractor

import (
	"context"

	"github.com/NortonBen/ai-memory-go/schema"
)

// HybridGraphExtractor dùng graphExt cho trích xuất thực thể/cạnh (ví dụ DeBERTa ONNX)
// và llmExt cho mọi thao tác cần LLM (think, request, intent, …).
type HybridGraphExtractor struct {
	graphExt LLMExtractor
	llmExt   *BasicExtractor
}

// NewHybridGraphExtractor tạo bộ trích xuất kết hợp: graph (NER/heuristic) + LLM.
func NewHybridGraphExtractor(graphExt LLMExtractor, llmExt *BasicExtractor) *HybridGraphExtractor {
	return &HybridGraphExtractor{graphExt: graphExt, llmExt: llmExt}
}

func (h *HybridGraphExtractor) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	return h.graphExt.ExtractEntities(ctx, text)
}

func (h *HybridGraphExtractor) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	return h.graphExt.ExtractRelationships(ctx, text, entities)
}

func (h *HybridGraphExtractor) ExtractBridgingRelationship(ctx context.Context, question, answer string) (*schema.Edge, error) {
	return h.llmExt.ExtractBridgingRelationship(ctx, question, answer)
}

func (h *HybridGraphExtractor) ExtractRequestIntent(ctx context.Context, text string) (*schema.RequestIntent, error) {
	return h.llmExt.ExtractRequestIntent(ctx, text)
}

func (h *HybridGraphExtractor) CompareEntities(ctx context.Context, existing, newEntity schema.Node) (*schema.ConsistencyResult, error) {
	return h.llmExt.CompareEntities(ctx, existing, newEntity)
}

func (h *HybridGraphExtractor) ExtractWithSchema(ctx context.Context, text string, schemaStruct interface{}) (interface{}, error) {
	return h.llmExt.ExtractWithSchema(ctx, text, schemaStruct)
}

func (h *HybridGraphExtractor) AnalyzeQuery(ctx context.Context, text string) (*schema.ThinkQueryAnalysis, error) {
	return h.llmExt.AnalyzeQuery(ctx, text)
}

func (h *HybridGraphExtractor) SetProvider(provider LLMProvider) {
	h.llmExt.SetProvider(provider)
}

func (h *HybridGraphExtractor) GetProvider() LLMProvider {
	return h.llmExt.GetProvider()
}
