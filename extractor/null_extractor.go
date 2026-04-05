package extractor

import (
	"context"

	"github.com/NortonBen/ai-memory-go/schema"
)

// NullExtractor is a no-op implementation of LLMExtractor.
// It returns empty results for all extraction calls without contacting any LLM.
//
// Use it when you want embedding-only Cognify (vector search / RAG) without
// the LLM entity and relationship extraction step that populates the knowledge graph.
//
// Example:
//
//	eng := engine.NewMemoryEngineWithStores(
//	    extractor.NewNullExtractor(), // no LLM needed
//	    harrierEmbedder,
//	    relStore, graphStore, vecStore,
//	    engine.EngineConfig{MaxWorkers: 4},
//	)
type NullExtractor struct{}

// NewNullExtractor returns an LLMExtractor that does nothing.
func NewNullExtractor() LLMExtractor {
	return &NullExtractor{}
}

func (n *NullExtractor) ExtractEntities(_ context.Context, _ string) ([]schema.Node, error) {
	return nil, nil
}

func (n *NullExtractor) ExtractRelationships(_ context.Context, _ string, _ []schema.Node) ([]schema.Edge, error) {
	return nil, nil
}

func (n *NullExtractor) ExtractBridgingRelationship(_ context.Context, _, _ string) (*schema.Edge, error) {
	return nil, nil
}

func (n *NullExtractor) ExtractRequestIntent(_ context.Context, _ string) (*schema.RequestIntent, error) {
	return nil, nil
}

func (n *NullExtractor) CompareEntities(_ context.Context, _ schema.Node, _ schema.Node) (*schema.ConsistencyResult, error) {
	return &schema.ConsistencyResult{Action: schema.ResolutionKeepSeparate}, nil
}

func (n *NullExtractor) ExtractWithSchema(_ context.Context, _ string, _ interface{}) (interface{}, error) {
	return nil, nil
}

func (n *NullExtractor) AnalyzeQuery(_ context.Context, _ string) (*schema.ThinkQueryAnalysis, error) {
	return &schema.ThinkQueryAnalysis{}, nil
}

func (n *NullExtractor) SetProvider(_ LLMProvider) {}

func (n *NullExtractor) GetProvider() LLMProvider { return nil }
