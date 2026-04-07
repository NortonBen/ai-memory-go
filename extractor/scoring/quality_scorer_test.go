package scoring

import (
	"context"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/stretchr/testify/require"
)

func TestDefaultQualityScoringConfig(t *testing.T) {
	cfg := DefaultQualityScoringConfig()
	require.NotNil(t, cfg)
	require.Equal(t, 0.4, cfg.EntityWeight)
	require.Equal(t, 0.4, cfg.RelationshipWeight)
	require.Equal(t, 0.2, cfg.ValidationWeight)
	require.Equal(t, 30, int(cfg.ScoringTimeout.Seconds()))
}

func TestScoreEntityExtraction_AddsIssuesAndRecommendations(t *testing.T) {
	scorer := NewBasicQualityScorer(nil, nil)
	scorer.config.MinEntityConfidence = 0.95

	entities := []schema.Node{
		{
			ID:         "e1",
			Type:       schema.NodeType("Custom"),
			Properties: map[string]interface{}{"name": "AI"},
		},
		{
			ID:         "e2",
			Type:       schema.NodeType("Custom"),
			Properties: map[string]interface{}{"name": "AI"},
		},
	}

	res, err := scorer.ScoreEntityExtraction(context.Background(), "This text has no entity mention", entities)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.EntityScores, 2)
	require.NotEmpty(t, res.Issues)
	require.GreaterOrEqual(t, res.OverallScore, 0.0)
	require.LessOrEqual(t, res.OverallScore, 1.0)
}

func TestScoreRelationshipExtraction_DetectsOrphanAndRecommendations(t *testing.T) {
	scorer := NewBasicQualityScorer(nil, nil)
	scorer.config.MinRelationshipConfidence = 0.95

	entities := []schema.Node{
		{ID: "a", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "Alice"}, Weight: 0.9},
	}
	relationships := []schema.Edge{
		{ID: "r1", From: "a", To: "missing", Type: schema.EdgeTypeRelatedTo, Weight: 0.5},
	}

	res, err := scorer.ScoreRelationshipExtraction(context.Background(), "Alice related", entities, relationships)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.RelationshipScores, 1)
	require.NotEmpty(t, res.Issues)
	require.NotEmpty(t, res.Recommendations)
}

func TestCalculateRelationshipCompleteness(t *testing.T) {
	scorer := NewBasicQualityScorer(nil, nil)
	entities := []schema.Node{
		{ID: "a", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "A"}},
		{ID: "b", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "B"}},
		{ID: "c", Type: schema.NodeTypeConcept, Properties: map[string]interface{}{"name": "C"}},
	}
	relationships := []schema.Edge{
		{ID: "r1", From: "a", To: "b", Type: schema.EdgeTypeRelatedTo, Weight: 1},
		{ID: "r2", From: "b", To: "c", Type: schema.EdgeTypeRelatedTo, Weight: 1},
	}

	score := scorer.calculateRelationshipCompleteness("A B C", entities, relationships)
	require.Greater(t, score, 0.0)
	require.LessOrEqual(t, score, 1.0)
}
