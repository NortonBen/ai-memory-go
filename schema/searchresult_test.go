package schema

import (
	"encoding/json"
	"testing"
	"time"
)

// TestNewSearchResult tests the creation of a new SearchResult
func TestNewSearchResult(t *testing.T) {
	dp := &DataPoint{
		ID:        "test-dp-1",
		Content:   "Test content",
		SessionID: "session-1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	sr := NewSearchResult(dp, 0.85, ModeHybridSearch)

	if sr.DataPoint != dp {
		t.Error("DataPoint not set correctly")
	}
	if sr.Score != 0.85 {
		t.Errorf("Expected score 0.85, got %f", sr.Score)
	}
	if sr.Mode != ModeHybridSearch {
		t.Errorf("Expected mode %s, got %s", ModeHybridSearch, sr.Mode)
	}
	if sr.Relationships == nil {
		t.Error("Relationships should be initialized")
	}
	if sr.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

// TestSearchResultValidation tests the validation of SearchResult
func TestSearchResultValidation(t *testing.T) {
	tests := []struct {
		name    string
		sr      *SearchResult
		wantErr bool
	}{
		{
			name: "valid search result",
			sr: &SearchResult{
				DataPoint: &DataPoint{
					ID:        "test-1",
					Content:   "Test",
					SessionID: "session-1",
				},
				Score: 0.75,
				Mode:  ModeSemanticSearch,
			},
			wantErr: false,
		},
		{
			name: "missing datapoint",
			sr: &SearchResult{
				Score: 0.75,
				Mode:  ModeSemanticSearch,
			},
			wantErr: true,
		},
		{
			name: "invalid score - negative",
			sr: &SearchResult{
				DataPoint: &DataPoint{
					ID:        "test-1",
					Content:   "Test",
					SessionID: "session-1",
				},
				Score: -0.5,
				Mode:  ModeSemanticSearch,
			},
			wantErr: true,
		},
		{
			name: "invalid score - too high",
			sr: &SearchResult{
				DataPoint: &DataPoint{
					ID:        "test-1",
					Content:   "Test",
					SessionID: "session-1",
				},
				Score: 1.5,
				Mode:  ModeSemanticSearch,
			},
			wantErr: true,
		},
		{
			name: "missing mode",
			sr: &SearchResult{
				DataPoint: &DataPoint{
					ID:        "test-1",
					Content:   "Test",
					SessionID: "session-1",
				},
				Score: 0.75,
			},
			wantErr: true,
		},
		{
			name: "invalid vector score",
			sr: &SearchResult{
				DataPoint: &DataPoint{
					ID:        "test-1",
					Content:   "Test",
					SessionID: "session-1",
				},
				Score:       0.75,
				Mode:        ModeSemanticSearch,
				VectorScore: 1.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sr.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSearchResultClone tests deep cloning of SearchResult
func TestSearchResultClone(t *testing.T) {
	original := &SearchResult{
		DataPoint: &DataPoint{
			ID:        "test-1",
			Content:   "Original content",
			SessionID: "session-1",
			Metadata:  map[string]interface{}{"key": "value"},
		},
		Score:         0.85,
		Mode:          ModeHybridSearch,
		Explanation:   "Test explanation",
		VectorScore:   0.9,
		GraphScore:    0.8,
		TemporalScore: 0.7,
		UserScore:     0.6,
		Rank:          1,
		Relationships: []RelationshipContext{
			{
				Type:        EdgeTypeRelatedTo,
				Target:      "target-1",
				TargetLabel: "Target Label",
				Weight:      0.8,
				Description: "Test relationship",
			},
		},
		Neighborhood: &NeighborhoodSummary{
			DirectCount:   5,
			IndirectCount: 10,
			TopConcepts:   []string{"concept1", "concept2"},
			SummaryText:   "Test summary",
			Metadata:      map[string]interface{}{"key": "value"},
		},
		Metadata: map[string]interface{}{
			"test": "value",
		},
		TraversalPath: []string{"node1", "node2"},
	}

	clone := original.Clone()

	// Verify deep copy
	if clone.DataPoint == original.DataPoint {
		t.Error("DataPoint should be deep copied")
	}
	if clone.DataPoint.Content != original.DataPoint.Content {
		t.Error("DataPoint content should match")
	}

	// Modify clone and verify original is unchanged
	clone.Score = 0.5
	clone.DataPoint.Content = "Modified content"
	clone.Relationships[0].Weight = 0.5
	clone.Neighborhood.DirectCount = 100
	clone.Metadata["test"] = "modified"
	clone.TraversalPath[0] = "modified"

	if original.Score == 0.5 {
		t.Error("Original score should not be modified")
	}
	if original.DataPoint.Content == "Modified content" {
		t.Error("Original DataPoint content should not be modified")
	}
	if original.Relationships[0].Weight == 0.5 {
		t.Error("Original relationship weight should not be modified")
	}
	if original.Neighborhood.DirectCount == 100 {
		t.Error("Original neighborhood count should not be modified")
	}
	if original.Metadata["test"] == "modified" {
		t.Error("Original metadata should not be modified")
	}
	if original.TraversalPath[0] == "modified" {
		t.Error("Original traversal path should not be modified")
	}
}

// TestSearchResultJSON tests JSON serialization and deserialization
func TestSearchResultJSON(t *testing.T) {
	original := &SearchResult{
		DataPoint: &DataPoint{
			ID:        "test-1",
			Content:   "Test content",
			SessionID: "session-1",
		},
		Score:       0.85,
		Mode:        ModeHybridSearch,
		Explanation: "Test explanation",
		Relationships: []RelationshipContext{
			{
				Type:   EdgeTypeRelatedTo,
				Target: "target-1",
				Weight: 0.8,
			},
		},
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	// Test ToJSON
	jsonStr, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonMap); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Test FromJSON
	restored, err := SearchResultFromJSON(jsonStr)
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}

	if restored.Score != original.Score {
		t.Errorf("Score mismatch: got %f, want %f", restored.Score, original.Score)
	}
	if restored.Mode != original.Mode {
		t.Errorf("Mode mismatch: got %s, want %s", restored.Mode, original.Mode)
	}
	if restored.DataPoint.ID != original.DataPoint.ID {
		t.Errorf("DataPoint ID mismatch: got %s, want %s", restored.DataPoint.ID, original.DataPoint.ID)
	}
}

// TestAddRelationship tests adding relationships to SearchResult
func TestAddRelationship(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test",
		SessionID: "session-1",
	}, 0.85, ModeHybridSearch)

	sr.AddRelationship(EdgeTypeRelatedTo, "target-1", "Target 1", 0.9, "Related concept")
	sr.AddRelationship(EdgeTypeSynonym, "target-2", "Target 2", 0.8, "Synonym")

	if len(sr.Relationships) != 2 {
		t.Errorf("Expected 2 relationships, got %d", len(sr.Relationships))
	}

	if sr.Relationships[0].Type != EdgeTypeRelatedTo {
		t.Errorf("Expected first relationship type %s, got %s", EdgeTypeRelatedTo, sr.Relationships[0].Type)
	}
	if sr.Relationships[1].Type != EdgeTypeSynonym {
		t.Errorf("Expected second relationship type %s, got %s", EdgeTypeSynonym, sr.Relationships[1].Type)
	}
}

// TestSetNeighborhood tests setting neighborhood summary
func TestSetNeighborhood(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test",
		SessionID: "session-1",
	}, 0.85, ModeHybridSearch)

	topConcepts := []string{"concept1", "concept2", "concept3"}
	sr.SetNeighborhood(5, 10, topConcepts, "Test neighborhood summary")

	if sr.Neighborhood == nil {
		t.Fatal("Neighborhood should be set")
	}
	if sr.Neighborhood.DirectCount != 5 {
		t.Errorf("Expected DirectCount 5, got %d", sr.Neighborhood.DirectCount)
	}
	if sr.Neighborhood.IndirectCount != 10 {
		t.Errorf("Expected IndirectCount 10, got %d", sr.Neighborhood.IndirectCount)
	}
	if len(sr.Neighborhood.TopConcepts) != 3 {
		t.Errorf("Expected 3 top concepts, got %d", len(sr.Neighborhood.TopConcepts))
	}
	if sr.Neighborhood.SummaryText != "Test neighborhood summary" {
		t.Errorf("Unexpected summary text: %s", sr.Neighborhood.SummaryText)
	}
}

// TestSetScoreBreakdown tests setting multi-factor score breakdown
func TestSetScoreBreakdown(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test",
		SessionID: "session-1",
	}, 0.85, ModeHybridSearch)

	sr.SetScoreBreakdown(0.9, 0.8, 0.7, 0.6)

	if sr.VectorScore != 0.9 {
		t.Errorf("Expected VectorScore 0.9, got %f", sr.VectorScore)
	}
	if sr.GraphScore != 0.8 {
		t.Errorf("Expected GraphScore 0.8, got %f", sr.GraphScore)
	}
	if sr.TemporalScore != 0.7 {
		t.Errorf("Expected TemporalScore 0.7, got %f", sr.TemporalScore)
	}
	if sr.UserScore != 0.6 {
		t.Errorf("Expected UserScore 0.6, got %f", sr.UserScore)
	}
}

// TestCalculateFinalScore tests final score calculation with default weights
func TestCalculateFinalScore(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test",
		SessionID: "session-1",
	}, 0.0, ModeHybridSearch)

	sr.SetScoreBreakdown(0.8, 0.6, 0.4, 0.2)
	finalScore := sr.CalculateFinalScore()

	// Expected: 0.8*0.4 + 0.6*0.3 + 0.4*0.2 + 0.2*0.1 = 0.32 + 0.18 + 0.08 + 0.02 = 0.60
	expected := 0.60
	epsilon := 0.0001
	if finalScore < expected-epsilon || finalScore > expected+epsilon {
		t.Errorf("Expected final score %f, got %f", expected, finalScore)
	}
}

// TestCalculateFinalScoreWithWeights tests final score calculation with custom weights
func TestCalculateFinalScoreWithWeights(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test",
		SessionID: "session-1",
	}, 0.0, ModeHybridSearch)

	sr.SetScoreBreakdown(0.8, 0.6, 0.4, 0.2)
	finalScore := sr.CalculateFinalScoreWithWeights(0.5, 0.3, 0.1, 0.1)

	// Expected: 0.8*0.5 + 0.6*0.3 + 0.4*0.1 + 0.2*0.1 = 0.4 + 0.18 + 0.04 + 0.02 = 0.64
	expected := 0.64
	epsilon := 0.0001
	if finalScore < expected-epsilon || finalScore > expected+epsilon {
		t.Errorf("Expected final score %f, got %f", expected, finalScore)
	}
}

// TestGetRelationshipsByType tests filtering relationships by type
func TestGetRelationshipsByType(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test",
		SessionID: "session-1",
	}, 0.85, ModeHybridSearch)

	sr.AddRelationship(EdgeTypeRelatedTo, "target-1", "Target 1", 0.9, "Related")
	sr.AddRelationship(EdgeTypeSynonym, "target-2", "Target 2", 0.8, "Synonym")
	sr.AddRelationship(EdgeTypeRelatedTo, "target-3", "Target 3", 0.7, "Related")

	relatedRels := sr.GetRelationshipsByType(EdgeTypeRelatedTo)
	if len(relatedRels) != 2 {
		t.Errorf("Expected 2 RELATED_TO relationships, got %d", len(relatedRels))
	}

	synonymRels := sr.GetRelationshipsByType(EdgeTypeSynonym)
	if len(synonymRels) != 1 {
		t.Errorf("Expected 1 SYNONYM relationship, got %d", len(synonymRels))
	}
}

// TestHasRelationshipTo tests checking for relationships to specific targets
func TestHasRelationshipTo(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test",
		SessionID: "session-1",
	}, 0.85, ModeHybridSearch)

	sr.AddRelationship(EdgeTypeRelatedTo, "target-1", "Target 1", 0.9, "Related")
	sr.AddRelationship(EdgeTypeSynonym, "target-2", "Target 2", 0.8, "Synonym")

	if !sr.HasRelationshipTo("target-1") {
		t.Error("Should have relationship to target-1")
	}
	if !sr.HasRelationshipTo("target-2") {
		t.Error("Should have relationship to target-2")
	}
	if sr.HasRelationshipTo("target-3") {
		t.Error("Should not have relationship to target-3")
	}
}

// TestGetContextSummary tests generating context summary
func TestGetContextSummary(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test content",
		SessionID: "session-1",
	}, 0.85, ModeHybridSearch)

	sr.AddRelationship(EdgeTypeRelatedTo, "target-1", "Target 1", 0.9, "Related")
	sr.SetNeighborhood(5, 10, []string{"concept1", "concept2"}, "Test summary")

	summary := sr.GetContextSummary()

	if summary == "" {
		t.Error("Summary should not be empty")
	}
	// Summary should contain key information
	if len(summary) < 50 {
		t.Error("Summary seems too short")
	}
}

// TestIsRelevant tests relevance threshold checking
func TestIsRelevant(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test",
		SessionID: "session-1",
	}, 0.75, ModeHybridSearch)

	if !sr.IsRelevant(0.7) {
		t.Error("Should be relevant with threshold 0.7")
	}
	if !sr.IsRelevant(0.75) {
		t.Error("Should be relevant with threshold 0.75")
	}
	if sr.IsRelevant(0.8) {
		t.Error("Should not be relevant with threshold 0.8")
	}
}

// TestGetTraversalDepth tests getting traversal depth
func TestGetTraversalDepth(t *testing.T) {
	sr := NewSearchResult(&DataPoint{
		ID:        "test-1",
		Content:   "Test",
		SessionID: "session-1",
	}, 0.85, ModeHybridSearch)

	sr.TraversalPath = []string{"node1", "node2", "node3"}

	depth := sr.GetTraversalDepth()
	if depth != 3 {
		t.Errorf("Expected traversal depth 3, got %d", depth)
	}
}

// TestSearchResultWithAllFields tests SearchResult with all fields populated
func TestSearchResultWithAllFields(t *testing.T) {
	sr := &SearchResult{
		DataPoint: &DataPoint{
			ID:        "test-1",
			Content:   "Complete test content",
			SessionID: "session-1",
			Metadata:  map[string]interface{}{"source": "test"},
		},
		Score:         0.85,
		Mode:          ModeContextualRAG,
		Explanation:   "This result was found through contextual RAG",
		VectorScore:   0.9,
		GraphScore:    0.8,
		TemporalScore: 0.7,
		UserScore:     0.6,
		QueryTime:     100 * time.Millisecond,
		Rank:          1,
		Relationships: []RelationshipContext{
			{
				Type:        EdgeTypeRelatedTo,
				Target:      "target-1",
				TargetLabel: "Related Concept",
				Weight:      0.9,
				Description: "Strongly related",
			},
		},
		Neighborhood: &NeighborhoodSummary{
			DirectCount:   5,
			IndirectCount: 10,
			TopConcepts:   []string{"concept1", "concept2", "concept3"},
			SummaryText:   "Rich neighborhood context",
			Metadata:      map[string]interface{}{"density": 0.8},
		},
		Metadata: map[string]interface{}{
			"search_type": "hybrid",
			"timestamp":   time.Now().Unix(),
		},
		TraversalPath: []string{"anchor", "hop1", "hop2"},
	}

	// Validate the complete structure
	if err := sr.Validate(); err != nil {
		t.Errorf("Complete SearchResult should be valid: %v", err)
	}

	// Test JSON serialization with all fields
	jsonStr, err := sr.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Verify all fields are present in JSON
	var jsonMap map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonMap); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	requiredFields := []string{"datapoint", "score", "mode", "relationships", "neighborhood", "metadata"}
	for _, field := range requiredFields {
		if _, exists := jsonMap[field]; !exists {
			t.Errorf("JSON missing required field: %s", field)
		}
	}
}
