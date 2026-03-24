package schema

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessedQueryCreation tests basic ProcessedQuery creation and initialization
func TestProcessedQueryCreation(t *testing.T) {
	entities := []*Node{
		NewNode(NodeTypeGrammarRule, map[string]interface{}{
			"name": "Present Perfect",
		}),
		NewNode(NodeTypeUser, map[string]interface{}{
			"name": "User",
		}),
	}

	keywords := []string{"cách dùng", "hiện tại hoàn thành", "đã học"}
	vector := []float32{0.1, 0.3, -0.2, 0.5, 0.8}

	pq := &ProcessedQuery{
		OriginalText: "Cách dùng thì Hiện tại hoàn thành mà tôi đã học là gì?",
		Vector:       vector,
		Entities:     entities,
		Keywords:     keywords,
		Language:     "vi",
		Intent:       "question",
		Metadata: map[string]interface{}{
			"source": "user_query",
		},
		ProcessedAt: time.Now(),
	}

	assert.NotEmpty(t, pq.OriginalText)
	assert.Equal(t, 5, len(pq.Vector))
	assert.Equal(t, 2, len(pq.Entities))
	assert.Equal(t, 3, len(pq.Keywords))
	assert.Equal(t, "vi", pq.Language)
	assert.Equal(t, "question", pq.Intent)
	assert.NotNil(t, pq.Metadata)
	assert.False(t, pq.ProcessedAt.IsZero())
}

// TestProcessedQueryValidation tests ProcessedQuery validation logic
func TestProcessedQueryValidation(t *testing.T) {
	tests := []struct {
		name      string
		query     *ProcessedQuery
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid query",
			query: &ProcessedQuery{
				OriginalText: "What is Present Perfect?",
				Vector:       []float32{0.1, 0.2, 0.3},
				Entities:     []*Node{},
				Keywords:     []string{"present", "perfect"},
				Metadata:     map[string]interface{}{},
				ProcessedAt:  time.Now(),
			},
			expectErr: false,
		},
		{
			name: "missing original text",
			query: &ProcessedQuery{
				Vector:      []float32{0.1, 0.2, 0.3},
				Entities:    []*Node{},
				Keywords:    []string{"test"},
				ProcessedAt: time.Now(),
			},
			expectErr: true,
			errMsg:    "ProcessedQuery OriginalText is required",
		},
		{
			name: "missing vector",
			query: &ProcessedQuery{
				OriginalText: "What is Present Perfect?",
				Entities:     []*Node{},
				Keywords:     []string{"test"},
				ProcessedAt:  time.Now(),
			},
			expectErr: true,
			errMsg:    "ProcessedQuery Vector is required",
		},
		{
			name: "empty vector",
			query: &ProcessedQuery{
				OriginalText: "What is Present Perfect?",
				Vector:       []float32{},
				Entities:     []*Node{},
				Keywords:     []string{"test"},
				ProcessedAt:  time.Now(),
			},
			expectErr: true,
			errMsg:    "ProcessedQuery Vector is required",
		},
		{
			name: "nil metadata auto-initialized",
			query: &ProcessedQuery{
				OriginalText: "What is Present Perfect?",
				Vector:       []float32{0.1, 0.2, 0.3},
				ProcessedAt:  time.Now(),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tt.query.Metadata, "Metadata should be initialized")
			}
		})
	}
}

// TestProcessedQueryJSONSerialization tests JSON marshaling and unmarshaling
func TestProcessedQueryJSONSerialization(t *testing.T) {
	entities := []*Node{
		NewNode(NodeTypeGrammarRule, map[string]interface{}{
			"name": "Present Perfect",
		}),
	}

	pq := &ProcessedQuery{
		OriginalText: "How to use Present Perfect?",
		Vector:       []float32{0.1, 0.3, -0.2, 0.5},
		Entities:     entities,
		Keywords:     []string{"present", "perfect", "usage"},
		Language:     "en",
		Intent:       "question",
		Metadata: map[string]interface{}{
			"source":     "user_query",
			"confidence": 0.95,
		},
		ProcessedAt: time.Now().Round(time.Second),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(pq)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Unmarshal back
	var decoded ProcessedQuery
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, pq.OriginalText, decoded.OriginalText)
	assert.Equal(t, len(pq.Vector), len(decoded.Vector))
	assert.Equal(t, pq.Vector[0], decoded.Vector[0])
	assert.Equal(t, len(pq.Entities), len(decoded.Entities))
	assert.Equal(t, len(pq.Keywords), len(decoded.Keywords))
	assert.Equal(t, pq.Language, decoded.Language)
	assert.Equal(t, pq.Intent, decoded.Intent)
	assert.Equal(t, pq.Metadata["source"], decoded.Metadata["source"])
}

// TestProcessedQueryWithEmptyFields tests ProcessedQuery with optional empty fields
func TestProcessedQueryWithEmptyFields(t *testing.T) {
	pq := &ProcessedQuery{
		OriginalText: "Test query",
		Vector:       []float32{0.1, 0.2},
		Entities:     []*Node{},
		Keywords:     []string{},
		ProcessedAt:  time.Now(),
	}

	err := pq.Validate()
	require.NoError(t, err)

	assert.Empty(t, pq.Entities)
	assert.Empty(t, pq.Keywords)
	assert.Empty(t, pq.Language)
	assert.Empty(t, pq.Intent)
	assert.NotNil(t, pq.Metadata)
}

// TestProcessedQueryWithMultipleEntities tests ProcessedQuery with multiple entities
func TestProcessedQueryWithMultipleEntities(t *testing.T) {
	entities := []*Node{
		NewNode(NodeTypeGrammarRule, map[string]interface{}{
			"name": "Present Perfect",
		}),
		NewNode(NodeTypeConcept, map[string]interface{}{
			"name": "Tense",
		}),
		NewNode(NodeTypeUser, map[string]interface{}{
			"name": "User",
		}),
	}

	pq := &ProcessedQuery{
		OriginalText: "Explain Present Perfect tense to me",
		Vector:       []float32{0.1, 0.2, 0.3, 0.4, 0.5},
		Entities:     entities,
		Keywords:     []string{"explain", "present", "perfect", "tense"},
		Language:     "en",
		Intent:       "explanation",
		ProcessedAt:  time.Now(),
	}

	err := pq.Validate()
	require.NoError(t, err)

	assert.Equal(t, 3, len(pq.Entities))
	assert.Equal(t, NodeTypeGrammarRule, pq.Entities[0].Type)
	assert.Equal(t, NodeTypeConcept, pq.Entities[1].Type)
	assert.Equal(t, NodeTypeUser, pq.Entities[2].Type)
}

// TestProcessedQueryVectorDimensions tests ProcessedQuery with different vector dimensions
func TestProcessedQueryVectorDimensions(t *testing.T) {
	tests := []struct {
		name      string
		dimension int
	}{
		{"small vector (3-dim)", 3},
		{"medium vector (128-dim)", 128},
		{"large vector (768-dim)", 768},
		{"xlarge vector (1536-dim)", 1536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vector := make([]float32, tt.dimension)
			for i := range vector {
				vector[i] = float32(i) * 0.01
			}

			pq := &ProcessedQuery{
				OriginalText: "Test query",
				Vector:       vector,
				ProcessedAt:  time.Now(),
			}

			err := pq.Validate()
			require.NoError(t, err)
			assert.Equal(t, tt.dimension, len(pq.Vector))
		})
	}
}

// TestProcessedQueryMetadata tests ProcessedQuery metadata handling
func TestProcessedQueryMetadata(t *testing.T) {
	pq := &ProcessedQuery{
		OriginalText: "Test query",
		Vector:       []float32{0.1, 0.2, 0.3},
		Metadata: map[string]interface{}{
			"source":      "user_input",
			"session_id":  "session_123",
			"user_id":     "user_456",
			"confidence":  0.95,
			"model":       "text-embedding-ada-002",
			"timestamp":   time.Now().Unix(),
			"retry_count": 0,
		},
		ProcessedAt: time.Now(),
	}

	err := pq.Validate()
	require.NoError(t, err)

	assert.Equal(t, "user_input", pq.Metadata["source"])
	assert.Equal(t, "session_123", pq.Metadata["session_id"])
	assert.Equal(t, "user_456", pq.Metadata["user_id"])
	assert.Equal(t, 0.95, pq.Metadata["confidence"])
	assert.Equal(t, "text-embedding-ada-002", pq.Metadata["model"])
	assert.NotNil(t, pq.Metadata["timestamp"])
	assert.Equal(t, 0, pq.Metadata["retry_count"])
}

// TestProcessedQueryLanguageDetection tests ProcessedQuery with different languages
func TestProcessedQueryLanguageDetection(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		language string
	}{
		{"English", "How to use Present Perfect?", "en"},
		{"Vietnamese", "Cách dùng thì Hiện tại hoàn thành?", "vi"},
		{"Spanish", "¿Cómo usar el Present Perfect?", "es"},
		{"French", "Comment utiliser le Present Perfect?", "fr"},
		{"German", "Wie benutzt man das Present Perfect?", "de"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pq := &ProcessedQuery{
				OriginalText: tt.text,
				Vector:       []float32{0.1, 0.2, 0.3},
				Language:     tt.language,
				ProcessedAt:  time.Now(),
			}

			err := pq.Validate()
			require.NoError(t, err)
			assert.Equal(t, tt.language, pq.Language)
		})
	}
}

// TestProcessedQueryIntentTypes tests ProcessedQuery with different intent types
func TestProcessedQueryIntentTypes(t *testing.T) {
	intents := []string{
		"question",
		"explanation",
		"example",
		"definition",
		"comparison",
		"practice",
		"feedback",
	}

	for _, intent := range intents {
		t.Run(intent, func(t *testing.T) {
			pq := &ProcessedQuery{
				OriginalText: "Test query for " + intent,
				Vector:       []float32{0.1, 0.2, 0.3},
				Intent:       intent,
				ProcessedAt:  time.Now(),
			}

			err := pq.Validate()
			require.NoError(t, err)
			assert.Equal(t, intent, pq.Intent)
		})
	}
}

// TestProcessedQueryKeywordExtraction tests ProcessedQuery with various keyword sets
func TestProcessedQueryKeywordExtraction(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		keywords []string
	}{
		{
			name:     "simple keywords",
			text:     "What is Present Perfect?",
			keywords: []string{"present", "perfect"},
		},
		{
			name:     "multiple keywords",
			text:     "How to use Present Perfect tense in English grammar?",
			keywords: []string{"use", "present", "perfect", "tense", "english", "grammar"},
		},
		{
			name:     "single keyword",
			text:     "Grammar",
			keywords: []string{"grammar"},
		},
		{
			name:     "no keywords",
			text:     "Test",
			keywords: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pq := &ProcessedQuery{
				OriginalText: tt.text,
				Vector:       []float32{0.1, 0.2, 0.3},
				Keywords:     tt.keywords,
				ProcessedAt:  time.Now(),
			}

			err := pq.Validate()
			require.NoError(t, err)
			assert.Equal(t, len(tt.keywords), len(pq.Keywords))
		})
	}
}

// TestProcessedQueryTimestamp tests ProcessedQuery timestamp handling
func TestProcessedQueryTimestamp(t *testing.T) {
	before := time.Now()
	pq := &ProcessedQuery{
		OriginalText: "Test query",
		Vector:       []float32{0.1, 0.2, 0.3},
		ProcessedAt:  time.Now(),
	}
	after := time.Now()

	assert.False(t, pq.ProcessedAt.IsZero())
	assert.True(t, pq.ProcessedAt.After(before) || pq.ProcessedAt.Equal(before))
	assert.True(t, pq.ProcessedAt.Before(after) || pq.ProcessedAt.Equal(after))
}

// TestProcessedQueryWithNilEntities tests ProcessedQuery with nil entities
func TestProcessedQueryWithNilEntities(t *testing.T) {
	pq := &ProcessedQuery{
		OriginalText: "Test query",
		Vector:       []float32{0.1, 0.2, 0.3},
		Entities:     nil,
		ProcessedAt:  time.Now(),
	}

	err := pq.Validate()
	require.NoError(t, err)
	assert.Nil(t, pq.Entities)
}

// TestProcessedQueryWithNilKeywords tests ProcessedQuery with nil keywords
func TestProcessedQueryWithNilKeywords(t *testing.T) {
	pq := &ProcessedQuery{
		OriginalText: "Test query",
		Vector:       []float32{0.1, 0.2, 0.3},
		Keywords:     nil,
		ProcessedAt:  time.Now(),
	}

	err := pq.Validate()
	require.NoError(t, err)
	assert.Nil(t, pq.Keywords)
}

// TestProcessedQueryComplexScenario tests ProcessedQuery with a complex real-world scenario
func TestProcessedQueryComplexScenario(t *testing.T) {
	// Simulate a complex English learning query
	entities := []*Node{
		NewNode(NodeTypeGrammarRule, map[string]interface{}{
			"name":        "Present Perfect",
			"description": "Have/Has + Past Participle",
		}),
		NewNode(NodeTypeConcept, map[string]interface{}{
			"name": "Tense",
		}),
		NewNode(NodeTypeUserPreference, map[string]interface{}{
			"name":       "User Learning History",
			"difficulty": "intermediate",
		}),
	}

	// Simulate a 768-dimensional embedding vector
	vector := make([]float32, 768)
	for i := range vector {
		vector[i] = float32(i) * 0.001
	}

	pq := &ProcessedQuery{
		OriginalText: "Cách dùng thì Hiện tại hoàn thành mà tôi đã học là gì?",
		Vector:       vector,
		Entities:     entities,
		Keywords:     []string{"cách dùng", "hiện tại hoàn thành", "đã học"},
		Language:     "vi",
		Intent:       "recall",
		Metadata: map[string]interface{}{
			"source":            "user_query",
			"session_id":        "session_abc123",
			"user_id":           "user_xyz789",
			"confidence":        0.92,
			"model":             "text-embedding-ada-002",
			"processing_time":   "45ms",
			"entity_count":      3,
			"keyword_count":     3,
			"language_detected": true,
			"intent_detected":   true,
		},
		ProcessedAt: time.Now(),
	}

	// Validate the complex query
	err := pq.Validate()
	require.NoError(t, err)

	// Verify all components
	assert.NotEmpty(t, pq.OriginalText)
	assert.Equal(t, 768, len(pq.Vector))
	assert.Equal(t, 3, len(pq.Entities))
	assert.Equal(t, 3, len(pq.Keywords))
	assert.Equal(t, "vi", pq.Language)
	assert.Equal(t, "recall", pq.Intent)
	assert.NotNil(t, pq.Metadata)
	assert.Equal(t, "session_abc123", pq.Metadata["session_id"])
	assert.Equal(t, 0.92, pq.Metadata["confidence"])

	// Test JSON serialization of complex query
	jsonData, err := json.Marshal(pq)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var decoded ProcessedQuery
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)
	assert.Equal(t, pq.OriginalText, decoded.OriginalText)
	assert.Equal(t, len(pq.Vector), len(decoded.Vector))
	assert.Equal(t, len(pq.Entities), len(decoded.Entities))
}

// TestProcessedQueryMetadataIsolation tests that metadata is properly isolated
func TestProcessedQueryMetadataIsolation(t *testing.T) {
	pq1 := &ProcessedQuery{
		OriginalText: "Query 1",
		Vector:       []float32{0.1, 0.2},
		Metadata: map[string]interface{}{
			"source": "query1",
		},
		ProcessedAt: time.Now(),
	}

	pq2 := &ProcessedQuery{
		OriginalText: "Query 2",
		Vector:       []float32{0.3, 0.4},
		Metadata: map[string]interface{}{
			"source": "query2",
		},
		ProcessedAt: time.Now(),
	}

	// Modify pq1 metadata
	pq1.Metadata["source"] = "modified_query1"
	pq1.Metadata["extra"] = "value"

	// Verify pq2 is unaffected
	assert.Equal(t, "query2", pq2.Metadata["source"])
	assert.Nil(t, pq2.Metadata["extra"])
}

// TestProcessedQueryVectorPrecision tests vector value precision
func TestProcessedQueryVectorPrecision(t *testing.T) {
	vector := []float32{
		0.123456789,
		-0.987654321,
		0.0,
		1.0,
		-1.0,
	}

	pq := &ProcessedQuery{
		OriginalText: "Test query",
		Vector:       vector,
		ProcessedAt:  time.Now(),
	}

	err := pq.Validate()
	require.NoError(t, err)

	// Verify vector values are preserved
	assert.InDelta(t, 0.123456789, pq.Vector[0], 0.0001)
	assert.InDelta(t, -0.987654321, pq.Vector[1], 0.0001)
	assert.Equal(t, float32(0.0), pq.Vector[2])
	assert.Equal(t, float32(1.0), pq.Vector[3])
	assert.Equal(t, float32(-1.0), pq.Vector[4])
}
