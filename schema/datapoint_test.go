package schema

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// DataPoint Tests
// ============================================================================

// TestDataPointCreation tests basic DataPoint creation and initialization
func TestDataPointCreation(t *testing.T) {
	metadata := map[string]interface{}{
		"source": "user_input",
		"topic":  "grammar",
	}

	dp := &DataPoint{
		ID:               "dp_123",
		Content:          "Present Perfect tense usage",
		ContentType:      "text",
		Metadata:         metadata,
		SessionID:        "session_1",
		UserID:           "user_1",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		ProcessingStatus: StatusPending,
	}

	assert.NotEmpty(t, dp.ID)
	assert.Equal(t, "Present Perfect tense usage", dp.Content)
	assert.Equal(t, "text", dp.ContentType)
	assert.Equal(t, metadata, dp.Metadata)
	assert.Equal(t, "session_1", dp.SessionID)
	assert.Equal(t, "user_1", dp.UserID)
	assert.Equal(t, StatusPending, dp.ProcessingStatus)
	assert.False(t, dp.CreatedAt.IsZero())
	assert.False(t, dp.UpdatedAt.IsZero())
}

// TestDataPointWithEmbedding tests DataPoint with embedding vector
func TestDataPointWithEmbedding(t *testing.T) {
	embedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

	dp := &DataPoint{
		ID:          "dp_123",
		Content:     "Test content",
		ContentType: "text",
		Metadata:    map[string]interface{}{},
		Embedding:   embedding,
		SessionID:   "session_1",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	assert.NotNil(t, dp.Embedding)
	assert.Equal(t, 5, len(dp.Embedding))
	assert.Equal(t, float32(0.1), dp.Embedding[0])
	assert.Equal(t, float32(0.5), dp.Embedding[4])
}

// TestDataPointWithRelationships tests DataPoint with relationships
func TestDataPointWithRelationships(t *testing.T) {
	relationships := []Relationship{
		{
			Type:   EdgeTypeRelatedTo,
			Target: "dp_456",
			Weight: 0.8,
			Metadata: map[string]interface{}{
				"context": "grammar learning",
			},
		},
		{
			Type:   EdgeTypeSynonym,
			Target: "dp_789",
			Weight: 0.9,
		},
	}

	dp := &DataPoint{
		ID:            "dp_123",
		Content:       "Test content",
		ContentType:   "text",
		Metadata:      map[string]interface{}{},
		Relationships: relationships,
		SessionID:     "session_1",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	assert.NotNil(t, dp.Relationships)
	assert.Equal(t, 2, len(dp.Relationships))
	assert.Equal(t, EdgeTypeRelatedTo, dp.Relationships[0].Type)
	assert.Equal(t, "dp_456", dp.Relationships[0].Target)
	assert.Equal(t, 0.8, dp.Relationships[0].Weight)
	assert.Equal(t, EdgeTypeSynonym, dp.Relationships[1].Type)
}

// TestDataPointProcessingStatus tests all processing status values
func TestDataPointProcessingStatus(t *testing.T) {
	tests := []struct {
		name   string
		status ProcessingStatus
	}{
		{"pending", StatusPending},
		{"processing", StatusProcessing},
		{"completed", StatusCompleted},
		{"failed", StatusFailed},
		{"retrying", StatusRetrying},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dp := &DataPoint{
				ID:               "dp_123",
				Content:          "Test",
				SessionID:        "session_1",
				ProcessingStatus: tt.status,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			assert.Equal(t, tt.status, dp.ProcessingStatus)
		})
	}
}

// TestDataPointWithErrorMessage tests DataPoint with error message
func TestDataPointWithErrorMessage(t *testing.T) {
	dp := &DataPoint{
		ID:               "dp_123",
		Content:          "Test content",
		SessionID:        "session_1",
		ProcessingStatus: StatusFailed,
		ErrorMessage:     "Embedding generation failed",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	assert.Equal(t, StatusFailed, dp.ProcessingStatus)
	assert.Equal(t, "Embedding generation failed", dp.ErrorMessage)
}

// TestDataPointValidation tests DataPoint validation logic
func TestDataPointValidation(t *testing.T) {
	tests := []struct {
		name      string
		dataPoint *DataPoint
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid datapoint",
			dataPoint: &DataPoint{
				ID:        "dp_123",
				Content:   "Test content",
				SessionID: "session_1",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			expectErr: false,
		},
		{
			name: "missing ID",
			dataPoint: &DataPoint{
				Content:   "Test content",
				SessionID: "session_1",
			},
			expectErr: true,
			errMsg:    "DataPoint ID is required",
		},
		{
			name: "missing Content",
			dataPoint: &DataPoint{
				ID:        "dp_123",
				SessionID: "session_1",
			},
			expectErr: true,
			errMsg:    "DataPoint Content is required",
		},
		{
			name: "global empty SessionID allowed",
			dataPoint: &DataPoint{
				ID:      "dp_123",
				Content: "Test content",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dataPoint.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestDataPointClone tests deep cloning of DataPoint structs
func TestDataPointClone(t *testing.T) {
	original := &DataPoint{
		ID:          "dp_123",
		Content:     "Original content",
		ContentType: "text",
		Metadata: map[string]interface{}{
			"source": "user",
			"topic":  "grammar",
		},
		Embedding: []float32{0.1, 0.2, 0.3},
		Relationships: []Relationship{
			{
				Type:   EdgeTypeRelatedTo,
				Target: "dp_456",
				Weight: 0.8,
			},
		},
		SessionID:        "session_1",
		UserID:           "user_1",
		CreatedAt:        time.Now().Add(-1 * time.Hour),
		UpdatedAt:        time.Now().Add(-30 * time.Minute),
		ProcessingStatus: StatusCompleted,
		ErrorMessage:     "",
	}

	clone := original.Clone()

	// Verify all fields are copied
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.Content, clone.Content)
	assert.Equal(t, original.ContentType, clone.ContentType)
	assert.Equal(t, original.SessionID, clone.SessionID)
	assert.Equal(t, original.UserID, clone.UserID)
	assert.Equal(t, original.CreatedAt, clone.CreatedAt)
	assert.Equal(t, original.ProcessingStatus, clone.ProcessingStatus)
	assert.Equal(t, original.ErrorMessage, clone.ErrorMessage)

	// Verify UpdatedAt is refreshed
	assert.True(t, clone.UpdatedAt.After(original.UpdatedAt))

	// Verify deep copy of metadata
	assert.Equal(t, original.Metadata["source"], clone.Metadata["source"])
	clone.Metadata["source"] = "modified"
	assert.NotEqual(t, original.Metadata["source"], clone.Metadata["source"])

	// Verify deep copy of embedding
	assert.Equal(t, len(original.Embedding), len(clone.Embedding))
	clone.Embedding[0] = 0.9
	assert.NotEqual(t, original.Embedding[0], clone.Embedding[0])

	// Verify deep copy of relationships
	assert.Equal(t, len(original.Relationships), len(clone.Relationships))
	clone.Relationships[0].Weight = 0.5
	assert.NotEqual(t, original.Relationships[0].Weight, clone.Relationships[0].Weight)
}

// TestDataPointToJSON tests JSON serialization
func TestDataPointToJSON(t *testing.T) {
	dp := &DataPoint{
		ID:          "dp_123",
		Content:     "Test content",
		ContentType: "text",
		Metadata: map[string]interface{}{
			"source": "user",
		},
		Embedding: []float32{0.1, 0.2, 0.3},
		SessionID: "session_1",
		UserID:    "user_1",
		CreatedAt: time.Now().Round(time.Second),
		UpdatedAt: time.Now().Round(time.Second),
		Relationships: []Relationship{
			{
				Type:   EdgeTypeRelatedTo,
				Target: "dp_456",
				Weight: 0.8,
			},
		},
		ProcessingStatus: StatusCompleted,
	}

	jsonStr, err := dp.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonStr)

	// Verify it's valid JSON
	var decoded map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "dp_123", decoded["id"])
	assert.Equal(t, "Test content", decoded["content"])
}

// TestDataPointFromJSON tests JSON deserialization
func TestDataPointFromJSON(t *testing.T) {
	jsonStr := `{
		"id": "dp_123",
		"content": "Test content",
		"content_type": "text",
		"metadata": {"source": "user"},
		"embedding": [0.1, 0.2, 0.3],
		"session_id": "session_1",
		"user_id": "user_1",
		"created_at": "2024-01-01T00:00:00Z",
		"updated_at": "2024-01-01T00:00:00Z",
		"relationships": [
			{
				"type": "RELATED_TO",
				"target": "dp_456",
				"weight": 0.8
			}
		],
		"processing_status": "completed"
	}`

	dp, err := DataPointFromJSON(jsonStr)
	require.NoError(t, err)
	assert.NotNil(t, dp)
	assert.Equal(t, "dp_123", dp.ID)
	assert.Equal(t, "Test content", dp.Content)
	assert.Equal(t, "text", dp.ContentType)
	assert.Equal(t, "session_1", dp.SessionID)
	assert.Equal(t, "user_1", dp.UserID)
	assert.Equal(t, StatusCompleted, dp.ProcessingStatus)
	assert.Equal(t, 3, len(dp.Embedding))
	assert.Equal(t, 1, len(dp.Relationships))
}

// TestDataPointJSONRoundtrip tests JSON serialization and deserialization roundtrip
func TestDataPointJSONRoundtrip(t *testing.T) {
	original := &DataPoint{
		ID:          "dp_123",
		Content:     "Test content",
		ContentType: "text",
		Metadata: map[string]interface{}{
			"source": "user",
			"count":  float64(42), // JSON numbers are float64
		},
		Embedding: []float32{0.1, 0.2, 0.3},
		SessionID: "session_1",
		UserID:    "user_1",
		CreatedAt: time.Now().Round(time.Second),
		UpdatedAt: time.Now().Round(time.Second),
		Relationships: []Relationship{
			{
				Type:   EdgeTypeRelatedTo,
				Target: "dp_456",
				Weight: 0.8,
			},
		},
		ProcessingStatus: StatusCompleted,
		ErrorMessage:     "",
	}

	// Serialize to JSON
	jsonStr, err := original.ToJSON()
	require.NoError(t, err)

	// Deserialize back
	decoded, err := DataPointFromJSON(jsonStr)
	require.NoError(t, err)

	// Verify fields match
	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.Content, decoded.Content)
	assert.Equal(t, original.ContentType, decoded.ContentType)
	assert.Equal(t, original.SessionID, decoded.SessionID)
	assert.Equal(t, original.UserID, decoded.UserID)
	assert.Equal(t, original.ProcessingStatus, decoded.ProcessingStatus)
	assert.Equal(t, len(original.Embedding), len(decoded.Embedding))
	assert.Equal(t, len(original.Relationships), len(decoded.Relationships))
}

// TestDataPointMetadataIsolation tests that metadata is properly isolated
func TestDataPointMetadataIsolation(t *testing.T) {
	dp1 := &DataPoint{
		ID:        "dp_1",
		Content:   "Content 1",
		SessionID: "session_1",
		Metadata: map[string]interface{}{
			"key": "value1",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	dp2 := &DataPoint{
		ID:        "dp_2",
		Content:   "Content 2",
		SessionID: "session_1",
		Metadata: map[string]interface{}{
			"key": "value2",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Modify dp1 metadata
	dp1.Metadata["key"] = "modified"
	dp1.Metadata["extra"] = "value"

	// Verify dp2 is unaffected
	assert.Equal(t, "value2", dp2.Metadata["key"])
	assert.Nil(t, dp2.Metadata["extra"])
}

// TestDataPointEmbeddingIsolation tests that embeddings are properly isolated
func TestDataPointEmbeddingIsolation(t *testing.T) {
	dp1 := &DataPoint{
		ID:        "dp_1",
		Content:   "Content 1",
		SessionID: "session_1",
		Embedding: []float32{0.1, 0.2, 0.3},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	dp2 := &DataPoint{
		ID:        "dp_2",
		Content:   "Content 2",
		SessionID: "session_1",
		Embedding: []float32{0.4, 0.5, 0.6},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Modify dp1 embedding
	dp1.Embedding[0] = 0.9

	// Verify dp2 is unaffected
	assert.Equal(t, float32(0.4), dp2.Embedding[0])
}

// TestDataPointRelationshipsIsolation tests that relationships are properly isolated
func TestDataPointRelationshipsIsolation(t *testing.T) {
	dp1 := &DataPoint{
		ID:        "dp_1",
		Content:   "Content 1",
		SessionID: "session_1",
		Relationships: []Relationship{
			{Type: EdgeTypeRelatedTo, Target: "dp_2", Weight: 0.5},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	dp2 := &DataPoint{
		ID:        "dp_2",
		Content:   "Content 2",
		SessionID: "session_1",
		Relationships: []Relationship{
			{Type: EdgeTypeSynonym, Target: "dp_3", Weight: 0.8},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Modify dp1 relationships
	dp1.Relationships[0].Weight = 0.9

	// Verify dp2 is unaffected
	assert.Equal(t, 0.8, dp2.Relationships[0].Weight)
}

// TestDataPointEmptyFields tests DataPoint with empty optional fields
func TestDataPointEmptyFields(t *testing.T) {
	dp := &DataPoint{
		ID:        "dp_123",
		Content:   "Test content",
		SessionID: "session_1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	assert.Empty(t, dp.ContentType)
	assert.Nil(t, dp.Metadata)
	assert.Nil(t, dp.Embedding)
	assert.Empty(t, dp.UserID)
	assert.Nil(t, dp.Relationships)
	assert.Empty(t, dp.ProcessingStatus)
	assert.Empty(t, dp.ErrorMessage)

	// Should still be valid
	require.NoError(t, dp.Validate())
}

// TestDataPointTimestamps tests that timestamps are properly set
func TestDataPointTimestamps(t *testing.T) {
	before := time.Now()
	dp := &DataPoint{
		ID:        "dp_123",
		Content:   "Test content",
		SessionID: "session_1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	after := time.Now()

	assert.False(t, dp.CreatedAt.IsZero())
	assert.False(t, dp.UpdatedAt.IsZero())
	assert.True(t, dp.CreatedAt.After(before) || dp.CreatedAt.Equal(before))
	assert.True(t, dp.CreatedAt.Before(after) || dp.CreatedAt.Equal(after))
	assert.True(t, dp.UpdatedAt.After(before) || dp.UpdatedAt.Equal(before))
	assert.True(t, dp.UpdatedAt.Before(after) || dp.UpdatedAt.Equal(after))
}

// TestDataPointWithLargeEmbedding tests DataPoint with realistic embedding size
func TestDataPointWithLargeEmbedding(t *testing.T) {
	// Typical embedding sizes: 768 (BERT), 1536 (OpenAI ada-002)
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = float32(i) * 0.001
	}

	dp := &DataPoint{
		ID:        "dp_123",
		Content:   "Test content",
		SessionID: "session_1",
		Embedding: embedding,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	assert.Equal(t, 768, len(dp.Embedding))
	assert.Equal(t, float32(0.0), dp.Embedding[0])
	assert.Equal(t, float32(0.767), dp.Embedding[767])
}

// TestDataPointWithMultipleRelationships tests DataPoint with many relationships
func TestDataPointWithMultipleRelationships(t *testing.T) {
	relationships := make([]Relationship, 10)
	for i := range relationships {
		relationships[i] = Relationship{
			Type:   EdgeTypeRelatedTo,
			Target: "dp_" + string(rune(i)),
			Weight: float64(i) * 0.1,
		}
	}

	dp := &DataPoint{
		ID:            "dp_123",
		Content:       "Test content",
		SessionID:     "session_1",
		Relationships: relationships,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	assert.Equal(t, 10, len(dp.Relationships))
	assert.Equal(t, 0.0, dp.Relationships[0].Weight)
	assert.Equal(t, 0.9, dp.Relationships[9].Weight)
}

// TestDataPointContentTypes tests various content types
func TestDataPointContentTypes(t *testing.T) {
	contentTypes := []string{
		"text",
		"markdown",
		"code",
		"json",
		"node",
		"document",
	}

	for _, contentType := range contentTypes {
		t.Run(contentType, func(t *testing.T) {
			dp := &DataPoint{
				ID:          "dp_123",
				Content:     "Test content",
				ContentType: contentType,
				SessionID:   "session_1",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			assert.Equal(t, contentType, dp.ContentType)
			require.NoError(t, dp.Validate())
		})
	}
}

// TestDataPointProcessingStatusTransitions tests status transitions
func TestDataPointProcessingStatusTransitions(t *testing.T) {
	dp := &DataPoint{
		ID:               "dp_123",
		Content:          "Test content",
		SessionID:        "session_1",
		ProcessingStatus: StatusPending,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Pending -> Processing
	dp.ProcessingStatus = StatusProcessing
	assert.Equal(t, StatusProcessing, dp.ProcessingStatus)

	// Processing -> Completed
	dp.ProcessingStatus = StatusCompleted
	assert.Equal(t, StatusCompleted, dp.ProcessingStatus)

	// Can also go to Failed
	dp.ProcessingStatus = StatusFailed
	dp.ErrorMessage = "Processing error"
	assert.Equal(t, StatusFailed, dp.ProcessingStatus)
	assert.NotEmpty(t, dp.ErrorMessage)

	// Failed -> Retrying
	dp.ProcessingStatus = StatusRetrying
	assert.Equal(t, StatusRetrying, dp.ProcessingStatus)
}

// TestDataPointWithComplexMetadata tests DataPoint with nested metadata
func TestDataPointWithComplexMetadata(t *testing.T) {
	metadata := map[string]interface{}{
		"source": "user_input",
		"tags":   []string{"grammar", "learning", "english"},
		"stats": map[string]interface{}{
			"views":  42,
			"rating": 4.5,
		},
		"timestamp": time.Now().Unix(),
	}

	dp := &DataPoint{
		ID:        "dp_123",
		Content:   "Test content",
		SessionID: "session_1",
		Metadata:  metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	assert.Equal(t, "user_input", dp.Metadata["source"])
	assert.NotNil(t, dp.Metadata["tags"])
	assert.NotNil(t, dp.Metadata["stats"])
	assert.NotNil(t, dp.Metadata["timestamp"])
}

// TestDataPointInvalidJSON tests error handling for invalid JSON
func TestDataPointInvalidJSON(t *testing.T) {
	invalidJSON := `{"id": "dp_123", "content": "test", invalid}`

	dp, err := DataPointFromJSON(invalidJSON)
	require.Error(t, err)
	assert.Nil(t, dp)
}

// TestDataPointEmptyJSON tests error handling for empty JSON
func TestDataPointEmptyJSON(t *testing.T) {
	dp, err := DataPointFromJSON("")
	require.Error(t, err)
	assert.Nil(t, dp)
}
