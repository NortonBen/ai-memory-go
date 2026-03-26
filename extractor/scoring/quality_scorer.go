// Package scoring - Quality scoring and validation for entity and relationship extraction
package scoring

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/schema"
)

// QualityScorer provides quality scoring and validation for extraction results
type QualityScorer interface {
	// ScoreEntityExtraction scores the quality of entity extraction results
	ScoreEntityExtraction(ctx context.Context, text string, entities []schema.Node) (*EntityQualityScore, error)

	// ScoreRelationshipExtraction scores the quality of relationship extraction results
	ScoreRelationshipExtraction(ctx context.Context, text string, entities []schema.Node, relationships []schema.Edge) (*RelationshipQualityScore, error)

	// ValidateExtraction validates extraction results for consistency and accuracy
	ValidateExtraction(ctx context.Context, text string, entities []schema.Node, relationships []schema.Edge) (*ExtractionValidation, error)

	// ScoreExtractionQuality provides overall quality score for complete extraction
	ScoreExtractionQuality(ctx context.Context, text string, entities []schema.Node, relationships []schema.Edge) (*OverallQualityScore, error)

	// GetQualityMetrics returns quality metrics for monitoring extraction performance
	GetQualityMetrics(ctx context.Context) (*QualityMetrics, error)

	// UpdateQualityFeedback updates quality scoring based on user feedback
	UpdateQualityFeedback(ctx context.Context, extractionID string, feedback *QualityFeedback) error
}

// EntityQualityScore represents quality metrics for entity extraction
type EntityQualityScore struct {
	OverallScore    float64                 `json:"overall_score"`   // 0.0 to 1.0
	Completeness    float64                 `json:"completeness"`    // How many expected entities were found
	Accuracy        float64                 `json:"accuracy"`        // How accurate are the extracted entities
	Consistency     float64                 `json:"consistency"`     // Internal consistency of entity types/properties
	Confidence      float64                 `json:"confidence"`      // Confidence in the extraction results
	EntityScores    map[string]*EntityScore `json:"entity_scores"`   // Per-entity quality scores
	Issues          []QualityIssue          `json:"issues"`          // Identified quality issues
	Recommendations []string                `json:"recommendations"` // Improvement recommendations
	Metadata        map[string]interface{}  `json:"metadata"`        // Additional scoring metadata
	ScoredAt        time.Time               `json:"scored_at"`
}

// EntityScore represents quality metrics for a single entity
type EntityScore struct {
	EntityID     string                 `json:"entity_id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Confidence   float64                `json:"confidence"`   // 0.0 to 1.0
	Relevance    float64                `json:"relevance"`    // How relevant to the text
	Completeness float64                `json:"completeness"` // How complete the entity properties are
	Consistency  float64                `json:"consistency"`  // Consistency with similar entities
	Issues       []string               `json:"issues"`       // Specific issues with this entity
	Metadata     map[string]interface{} `json:"metadata"`     // Additional entity-specific data
}

// RelationshipQualityScore represents quality metrics for relationship extraction
type RelationshipQualityScore struct {
	OverallScore       float64                       `json:"overall_score"`       // 0.0 to 1.0
	Completeness       float64                       `json:"completeness"`        // How many expected relationships were found
	Accuracy           float64                       `json:"accuracy"`            // How accurate are the extracted relationships
	Consistency        float64                       `json:"consistency"`         // Internal consistency of relationship types/weights
	Confidence         float64                       `json:"confidence"`          // Confidence in the extraction results
	RelationshipScores map[string]*RelationshipScore `json:"relationship_scores"` // Per-relationship quality scores
	Issues             []QualityIssue                `json:"issues"`              // Identified quality issues
	Recommendations    []string                      `json:"recommendations"`     // Improvement recommendations
	Metadata           map[string]interface{}        `json:"metadata"`            // Additional scoring metadata
	ScoredAt           time.Time                     `json:"scored_at"`
}

// RelationshipScore represents quality metrics for a single relationship
type RelationshipScore struct {
	RelationshipID   string                 `json:"relationship_id"`
	FromEntity       string                 `json:"from_entity"`
	ToEntity         string                 `json:"to_entity"`
	Type             string                 `json:"type"`
	Confidence       float64                `json:"confidence"`       // 0.0 to 1.0
	Relevance        float64                `json:"relevance"`        // How relevant to the text
	Strength         float64                `json:"strength"`         // Relationship strength assessment
	Bidirectionality float64                `json:"bidirectionality"` // Whether bidirectional relationships are consistent
	Issues           []string               `json:"issues"`           // Specific issues with this relationship
	Metadata         map[string]interface{} `json:"metadata"`         // Additional relationship-specific data
}

// ExtractionValidation represents validation results for extraction
type ExtractionValidation struct {
	IsValid                bool                    `json:"is_valid"`
	ValidationScore        float64                 `json:"validation_score"` // 0.0 to 1.0
	EntityValidation       *EntityValidation       `json:"entity_validation"`
	RelationshipValidation *RelationshipValidation `json:"relationship_validation"`
	CrossValidation        *CrossValidation        `json:"cross_validation"` // Validation across entities and relationships
	Issues                 []QualityIssue          `json:"issues"`           // All validation issues
	Recommendations        []string                `json:"recommendations"`  // Validation-based recommendations
	ValidatedAt            time.Time               `json:"validated_at"`
}

// EntityValidation represents validation results for entities
type EntityValidation struct {
	ValidEntities        []string `json:"valid_entities"`
	InvalidEntities      []string `json:"invalid_entities"`
	DuplicateEntities    []string `json:"duplicate_entities"`
	MissingEntities      []string `json:"missing_entities"`      // Expected but not found
	UnexpectedEntities   []string `json:"unexpected_entities"`   // Found but not expected
	TypeConsistency      float64  `json:"type_consistency"`      // 0.0 to 1.0
	PropertyCompleteness float64  `json:"property_completeness"` // 0.0 to 1.0
}

// RelationshipValidation represents validation results for relationships
type RelationshipValidation struct {
	ValidRelationships       []string `json:"valid_relationships"`
	InvalidRelationships     []string `json:"invalid_relationships"`
	OrphanedRelationships    []string `json:"orphaned_relationships"`    // Relationships with missing entities
	CircularRelationships    []string `json:"circular_relationships"`    // Self-referencing relationships
	ConflictingRelationships []string `json:"conflicting_relationships"` // Contradictory relationships
	TypeConsistency          float64  `json:"type_consistency"`          // 0.0 to 1.0
	WeightConsistency        float64  `json:"weight_consistency"`        // 0.0 to 1.0
}

// CrossValidation represents validation across entities and relationships
type CrossValidation struct {
	EntityRelationshipConsistency float64  `json:"entity_relationship_consistency"` // 0.0 to 1.0
	GraphConnectivity             float64  `json:"graph_connectivity"`              // How well connected the graph is
	SemanticConsistency           float64  `json:"semantic_consistency"`            // Semantic consistency across extraction
	TemporalConsistency           float64  `json:"temporal_consistency"`            // Temporal consistency if applicable
	Issues                        []string `json:"issues"`                          // Cross-validation specific issues
}

// OverallQualityScore represents comprehensive quality assessment
type OverallQualityScore struct {
	OverallScore       float64                `json:"overall_score"`       // 0.0 to 1.0
	EntityScore        float64                `json:"entity_score"`        // Entity extraction quality
	RelationshipScore  float64                `json:"relationship_score"`  // Relationship extraction quality
	ValidationScore    float64                `json:"validation_score"`    // Validation quality
	ConsistencyScore   float64                `json:"consistency_score"`   // Overall consistency
	CompletenessScore  float64                `json:"completeness_score"`  // Overall completeness
	ConfidenceScore    float64                `json:"confidence_score"`    // Overall confidence
	PerformanceMetrics *PerformanceMetrics    `json:"performance_metrics"` // Performance-related metrics
	QualityBreakdown   *QualityBreakdown      `json:"quality_breakdown"`   // Detailed quality breakdown
	Issues             []QualityIssue         `json:"issues"`              // All identified issues
	Recommendations    []string               `json:"recommendations"`     // All recommendations
	Metadata           map[string]interface{} `json:"metadata"`            // Additional metadata
	ScoredAt           time.Time              `json:"scored_at"`
}

// PerformanceMetrics represents performance-related quality metrics
type PerformanceMetrics struct {
	ExtractionTime         time.Duration `json:"extraction_time"`          // Time taken for extraction
	ScoringTime            time.Duration `json:"scoring_time"`             // Time taken for scoring
	ValidationTime         time.Duration `json:"validation_time"`          // Time taken for validation
	TotalProcessingTime    time.Duration `json:"total_processing_time"`    // Total time
	EntityCount            int           `json:"entity_count"`             // Number of entities extracted
	RelationshipCount      int           `json:"relationship_count"`       // Number of relationships extracted
	TextLength             int           `json:"text_length"`              // Length of input text
	TokenCount             int           `json:"token_count"`              // Estimated token count
	EntitiesPerSecond      float64       `json:"entities_per_second"`      // Extraction rate
	RelationshipsPerSecond float64       `json:"relationships_per_second"` // Relationship extraction rate
}

// QualityBreakdown provides detailed quality analysis
type QualityBreakdown struct {
	EntityTypeDistribution       map[string]int     `json:"entity_type_distribution"`       // Distribution of entity types
	RelationshipTypeDistribution map[string]int     `json:"relationship_type_distribution"` // Distribution of relationship types
	ConfidenceDistribution       map[string]int     `json:"confidence_distribution"`        // Distribution of confidence scores
	QualityByEntityType          map[string]float64 `json:"quality_by_entity_type"`         // Quality scores by entity type
	QualityByRelationshipType    map[string]float64 `json:"quality_by_relationship_type"`   // Quality scores by relationship type
	CommonIssues                 map[string]int     `json:"common_issues"`                  // Frequency of common issues
	ImprovementAreas             []string           `json:"improvement_areas"`              // Areas needing improvement
}

// QualityIssue represents a specific quality issue found during scoring/validation
type QualityIssue struct {
	Type           QualityIssueType       `json:"type"`
	Severity       IssueSeverity          `json:"severity"`
	Description    string                 `json:"description"`
	EntityID       string                 `json:"entity_id,omitempty"`
	RelationshipID string                 `json:"relationship_id,omitempty"`
	Suggestion     string                 `json:"suggestion,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// QualityIssueType defines types of quality issues
type QualityIssueType string

const (
	IssueTypeEntityMissing           QualityIssueType = "entity_missing"
	IssueTypeEntityDuplicate         QualityIssueType = "entity_duplicate"
	IssueTypeEntityInconsistent      QualityIssueType = "entity_inconsistent"
	IssueTypeEntityLowConfidence     QualityIssueType = "entity_low_confidence"
	IssueTypeRelationshipMissing     QualityIssueType = "relationship_missing"
	IssueTypeRelationshipOrphaned    QualityIssueType = "relationship_orphaned"
	IssueTypeRelationshipCircular    QualityIssueType = "relationship_circular"
	IssueTypeRelationshipConflicting QualityIssueType = "relationship_conflicting"
	IssueTypeTypeInconsistency       QualityIssueType = "type_inconsistency"
	IssueTypePropertyIncomplete      QualityIssueType = "property_incomplete"
	IssueTypeSemanticInconsistency   QualityIssueType = "semantic_inconsistency"
	IssueTypePerformancePoor         QualityIssueType = "performance_poor"
)

// IssueSeverity defines the severity levels of quality issues
type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "critical"
	SeverityHigh     IssueSeverity = "high"
	SeverityMedium   IssueSeverity = "medium"
	SeverityLow      IssueSeverity = "low"
	SeverityInfo     IssueSeverity = "info"
)

// QualityMetrics represents overall quality metrics for monitoring
type QualityMetrics struct {
	TotalExtractions         int64                        `json:"total_extractions"`
	AverageEntityScore       float64                      `json:"average_entity_score"`
	AverageRelationshipScore float64                      `json:"average_relationship_score"`
	AverageOverallScore      float64                      `json:"average_overall_score"`
	ExtractionSuccessRate    float64                      `json:"extraction_success_rate"`
	ValidationSuccessRate    float64                      `json:"validation_success_rate"`
	CommonIssues             map[QualityIssueType]int64   `json:"common_issues"`
	PerformanceMetrics       *AggregatePerformanceMetrics `json:"performance_metrics"`
	QualityTrends            *QualityTrends               `json:"quality_trends"`
	LastUpdated              time.Time                    `json:"last_updated"`
}

// AggregatePerformanceMetrics represents aggregated performance data
type AggregatePerformanceMetrics struct {
	AverageExtractionTime         time.Duration `json:"average_extraction_time"`
	AverageScoringTime            time.Duration `json:"average_scoring_time"`
	AverageValidationTime         time.Duration `json:"average_validation_time"`
	AverageEntitiesPerSecond      float64       `json:"average_entities_per_second"`
	AverageRelationshipsPerSecond float64       `json:"average_relationships_per_second"`
	TotalProcessingTime           time.Duration `json:"total_processing_time"`
}

// QualityTrends represents quality trends over time
type QualityTrends struct {
	EntityScoreTrend       []float64 `json:"entity_score_trend"`       // Recent entity scores
	RelationshipScoreTrend []float64 `json:"relationship_score_trend"` // Recent relationship scores
	OverallScoreTrend      []float64 `json:"overall_score_trend"`      // Recent overall scores
	ImprovementRate        float64   `json:"improvement_rate"`         // Rate of quality improvement
	TrendDirection         string    `json:"trend_direction"`          // "improving", "declining", "stable"
}

// QualityFeedback represents user feedback for quality improvement
type QualityFeedback struct {
	ExtractionID         string                 `json:"extraction_id"`
	UserID               string                 `json:"user_id,omitempty"`
	OverallRating        float64                `json:"overall_rating"` // 1.0 to 5.0
	EntityFeedback       []EntityFeedback       `json:"entity_feedback"`
	RelationshipFeedback []RelationshipFeedback `json:"relationship_feedback"`
	Comments             string                 `json:"comments,omitempty"`
	Suggestions          []string               `json:"suggestions,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	SubmittedAt          time.Time              `json:"submitted_at"`
}

// EntityFeedback represents feedback for a specific entity
type EntityFeedback struct {
	EntityID    string  `json:"entity_id"`
	IsCorrect   bool    `json:"is_correct"`
	IsRelevant  bool    `json:"is_relevant"`
	IsComplete  bool    `json:"is_complete"`
	Rating      float64 `json:"rating"` // 1.0 to 5.0
	Comments    string  `json:"comments,omitempty"`
	Corrections string  `json:"corrections,omitempty"` // Suggested corrections
}

// RelationshipFeedback represents feedback for a specific relationship
type RelationshipFeedback struct {
	RelationshipID string  `json:"relationship_id"`
	IsCorrect      bool    `json:"is_correct"`
	IsRelevant     bool    `json:"is_relevant"`
	StrengthRating float64 `json:"strength_rating"` // 1.0 to 5.0
	Rating         float64 `json:"rating"`          // 1.0 to 5.0
	Comments       string  `json:"comments,omitempty"`
	Corrections    string  `json:"corrections,omitempty"` // Suggested corrections
}

// BasicQualityScorer implements the QualityScorer interface
type BasicQualityScorer struct {
	provider        extractor.LLMProvider
	config          *QualityScoringConfig
	metrics         *QualityMetrics
	feedbackHistory []QualityFeedback
}

// QualityScoringConfig configures quality scoring behavior
type QualityScoringConfig struct {
	// Scoring weights
	EntityWeight       float64 `json:"entity_weight"`       // Weight for entity quality (default: 0.4)
	RelationshipWeight float64 `json:"relationship_weight"` // Weight for relationship quality (default: 0.4)
	ValidationWeight   float64 `json:"validation_weight"`   // Weight for validation quality (default: 0.2)

	// Quality thresholds
	MinEntityConfidence       float64 `json:"min_entity_confidence"`       // Minimum confidence for entities (default: 0.7)
	MinRelationshipConfidence float64 `json:"min_relationship_confidence"` // Minimum confidence for relationships (default: 0.6)
	MinOverallScore           float64 `json:"min_overall_score"`           // Minimum overall score (default: 0.7)

	// Validation settings
	EnableSemanticValidation bool `json:"enable_semantic_validation"` // Enable semantic consistency checks
	EnablePerformanceScoring bool `json:"enable_performance_scoring"` // Include performance in scoring
	EnableFeedbackLearning   bool `json:"enable_feedback_learning"`   // Learn from user feedback

	// Domain-specific settings
	Domain                    string                 `json:"domain"`                      // Domain for specialized scoring
	ExpectedEntityTypes       []string               `json:"expected_entity_types"`       // Expected entity types for completeness
	ExpectedRelationshipTypes []string               `json:"expected_relationship_types"` // Expected relationship types
	CustomScoringRules        map[string]interface{} `json:"custom_scoring_rules"`        // Custom domain-specific rules

	// Performance settings
	ScoringTimeout        time.Duration `json:"scoring_timeout"`         // Timeout for scoring operations
	ValidationTimeout     time.Duration `json:"validation_timeout"`      // Timeout for validation operations
	EnableParallelScoring bool          `json:"enable_parallel_scoring"` // Enable parallel scoring for performance
}

// NewBasicQualityScorer creates a new basic quality scorer
func NewBasicQualityScorer(provider extractor.LLMProvider, config *QualityScoringConfig) *BasicQualityScorer {
	if config == nil {
		config = DefaultQualityScoringConfig()
	}

	return &BasicQualityScorer{
		provider:        provider,
		config:          config,
		metrics:         &QualityMetrics{},
		feedbackHistory: make([]QualityFeedback, 0),
	}
}

// DefaultQualityScoringConfig returns default configuration for quality scoring
func DefaultQualityScoringConfig() *QualityScoringConfig {
	return &QualityScoringConfig{
		EntityWeight:              0.4,
		RelationshipWeight:        0.4,
		ValidationWeight:          0.2,
		MinEntityConfidence:       0.7,
		MinRelationshipConfidence: 0.6,
		MinOverallScore:           0.7,
		EnableSemanticValidation:  true,
		EnablePerformanceScoring:  true,
		EnableFeedbackLearning:    true,
		Domain:                    "general",
		ExpectedEntityTypes:       []string{"Concept", "Entity", "Word"},
		ExpectedRelationshipTypes: []string{"RELATED_TO", "SIMILAR_TO", "PART_OF"},
		CustomScoringRules:        make(map[string]interface{}),
		ScoringTimeout:            30 * time.Second,
		ValidationTimeout:         15 * time.Second,
		EnableParallelScoring:     true,
	}
}

// ScoreEntityExtraction scores the quality of entity extraction results
func (bqs *BasicQualityScorer) ScoreEntityExtraction(ctx context.Context, text string, entities []schema.Node) (*EntityQualityScore, error) {
	startTime := time.Now()

	// Initialize scoring result
	score := &EntityQualityScore{
		EntityScores:    make(map[string]*EntityScore),
		Issues:          make([]QualityIssue, 0),
		Recommendations: make([]string, 0),
		Metadata:        make(map[string]interface{}),
		ScoredAt:        time.Now(),
	}

	// Score individual entities
	totalConfidence := 0.0
	totalRelevance := 0.0
	totalCompleteness := 0.0
	totalConsistency := 0.0

	for _, entity := range entities {
		entityScore := bqs.scoreIndividualEntity(ctx, text, entity, entities)
		score.EntityScores[entity.ID] = entityScore

		totalConfidence += entityScore.Confidence
		totalRelevance += entityScore.Relevance
		totalCompleteness += entityScore.Completeness
		totalConsistency += entityScore.Consistency

		// Check for issues
		if entityScore.Confidence < bqs.config.MinEntityConfidence {
			issue := QualityIssue{
				Type:        IssueTypeEntityLowConfidence,
				Severity:    SeverityMedium,
				Description: fmt.Sprintf("Entity '%s' has low confidence score: %.2f", entityScore.Name, entityScore.Confidence),
				EntityID:    entity.ID,
				Suggestion:  "Consider reviewing entity extraction prompt or model parameters",
			}
			score.Issues = append(score.Issues, issue)
		}
	}

	entityCount := float64(len(entities))
	if entityCount > 0 {
		score.Confidence = totalConfidence / entityCount
		score.Accuracy = totalRelevance / entityCount
		score.Completeness = totalCompleteness / entityCount
		score.Consistency = totalConsistency / entityCount
	}

	// Calculate overall score
	score.OverallScore = (score.Confidence*0.3 + score.Accuracy*0.3 + score.Completeness*0.2 + score.Consistency*0.2)

	// Add performance metadata
	score.Metadata["scoring_time"] = time.Since(startTime)
	score.Metadata["entity_count"] = len(entities)
	score.Metadata["text_length"] = len(text)

	// Generate recommendations
	score.Recommendations = bqs.generateEntityRecommendations(score)

	return score, nil
}

// scoreIndividualEntity scores a single entity
func (bqs *BasicQualityScorer) scoreIndividualEntity(ctx context.Context, text string, entity schema.Node, allEntities []schema.Node) *EntityScore {
	entityScore := &EntityScore{
		EntityID: entity.ID,
		Type:     string(entity.Type),
		Issues:   make([]string, 0),
		Metadata: make(map[string]interface{}),
	}

	// Get entity name
	if name, ok := entity.Properties["name"].(string); ok {
		entityScore.Name = name
	}

	// Calculate confidence based on entity properties and context
	entityScore.Confidence = bqs.calculateEntityConfidence(text, entity)

	// Calculate relevance to the text
	entityScore.Relevance = bqs.calculateEntityRelevance(text, entity)

	// Calculate completeness of entity properties
	entityScore.Completeness = bqs.calculateEntityCompleteness(entity)

	// Calculate consistency with other entities
	entityScore.Consistency = bqs.calculateEntityConsistency(entity, allEntities)

	// Check for specific issues
	if entityScore.Name == "" {
		entityScore.Issues = append(entityScore.Issues, "Entity missing name property")
	}

	if len(entity.Properties) < 2 {
		entityScore.Issues = append(entityScore.Issues, "Entity has minimal properties")
	}

	return entityScore
}

// calculateEntityConfidence calculates confidence score for an entity
func (bqs *BasicQualityScorer) calculateEntityConfidence(text string, entity schema.Node) float64 {
	confidence := 0.5 // Base confidence

	// Check if entity name appears in text
	if name, ok := entity.Properties["name"].(string); ok {
		if strings.Contains(strings.ToLower(text), strings.ToLower(name)) {
			confidence += 0.3
		}
	}

	// Check entity type appropriateness
	if bqs.isExpectedEntityType(string(entity.Type)) {
		confidence += 0.2
	}

	// Check property completeness
	if len(entity.Properties) >= 3 {
		confidence += 0.1
	}

	return math.Min(confidence, 1.0)
}

// calculateEntityRelevance calculates relevance score for an entity
func (bqs *BasicQualityScorer) calculateEntityRelevance(text string, entity schema.Node) float64 {
	relevance := 0.3 // Base relevance

	// Check text mentions
	if name, ok := entity.Properties["name"].(string); ok {
		textLower := strings.ToLower(text)
		nameLower := strings.ToLower(name)

		if strings.Contains(textLower, nameLower) {
			relevance += 0.4

			// Bonus for multiple mentions
			count := strings.Count(textLower, nameLower)
			if count > 1 {
				relevance += math.Min(float64(count-1)*0.1, 0.3)
			}
		}
	}

	return math.Min(relevance, 1.0)
}

// calculateEntityCompleteness calculates completeness score for an entity
func (bqs *BasicQualityScorer) calculateEntityCompleteness(entity schema.Node) float64 {
	completeness := 0.0

	// Check for required properties
	if _, hasName := entity.Properties["name"]; hasName {
		completeness += 0.4
	}

	if entity.Type != "" {
		completeness += 0.3
	}

	// Bonus for additional properties
	propertyCount := len(entity.Properties)
	if propertyCount >= 2 {
		completeness += 0.2
	}
	if propertyCount >= 4 {
		completeness += 0.1
	}

	return math.Min(completeness, 1.0)
}

// calculateEntityConsistency calculates consistency score for an entity
func (bqs *BasicQualityScorer) calculateEntityConsistency(entity schema.Node, allEntities []schema.Node) float64 {
	consistency := 0.8 // Base consistency

	// Check for duplicate names
	entityName := ""
	if name, ok := entity.Properties["name"].(string); ok {
		entityName = strings.ToLower(name)
	}

	duplicateCount := 0
	for _, other := range allEntities {
		if other.ID == entity.ID {
			continue
		}
		if otherName, ok := other.Properties["name"].(string); ok {
			if strings.ToLower(otherName) == entityName {
				duplicateCount++
			}
		}
	}

	if duplicateCount > 0 {
		consistency -= float64(duplicateCount) * 0.2
	}

	return math.Max(consistency, 0.0)
}

// isExpectedEntityType checks if entity type is expected for the domain
func (bqs *BasicQualityScorer) isExpectedEntityType(entityType string) bool {
	for _, expected := range bqs.config.ExpectedEntityTypes {
		if expected == entityType {
			return true
		}
	}
	return false
}

// generateEntityRecommendations generates recommendations for entity extraction improvement
func (bqs *BasicQualityScorer) generateEntityRecommendations(score *EntityQualityScore) []string {
	recommendations := make([]string, 0)

	if score.Confidence < 0.7 {
		recommendations = append(recommendations, "Consider improving entity extraction prompts to increase confidence")
	}

	if score.Completeness < 0.6 {
		recommendations = append(recommendations, "Enhance entity property extraction to improve completeness")
	}

	if score.Consistency < 0.7 {
		recommendations = append(recommendations, "Review entity deduplication and consistency checks")
	}

	if len(score.Issues) > len(score.EntityScores)/2 {
		recommendations = append(recommendations, "High number of issues detected - consider model fine-tuning")
	}

	return recommendations
}

// ScoreRelationshipExtraction scores the quality of relationship extraction results
func (bqs *BasicQualityScorer) ScoreRelationshipExtraction(ctx context.Context, text string, entities []schema.Node, relationships []schema.Edge) (*RelationshipQualityScore, error) {
	startTime := time.Now()

	// Initialize scoring result
	score := &RelationshipQualityScore{
		RelationshipScores: make(map[string]*RelationshipScore),
		Issues:             make([]QualityIssue, 0),
		Recommendations:    make([]string, 0),
		Metadata:           make(map[string]interface{}),
		ScoredAt:           time.Now(),
	}

	// Create entity lookup map
	entityMap := make(map[string]schema.Node)
	for _, entity := range entities {
		entityMap[entity.ID] = entity
	}

	// Score individual relationships
	totalConfidence := 0.0
	totalRelevance := 0.0
	totalStrength := 0.0
	totalBidirectionality := 0.0

	for _, relationship := range relationships {
		relationshipScore := bqs.scoreIndividualRelationship(ctx, text, relationship, entities, relationships, entityMap)
		score.RelationshipScores[relationship.ID] = relationshipScore

		totalConfidence += relationshipScore.Confidence
		totalRelevance += relationshipScore.Relevance
		totalStrength += relationshipScore.Strength
		totalBidirectionality += relationshipScore.Bidirectionality

		// Check for issues
		if relationshipScore.Confidence < bqs.config.MinRelationshipConfidence {
			issue := QualityIssue{
				Type:           IssueTypeRelationshipMissing,
				Severity:       SeverityMedium,
				Description:    fmt.Sprintf("Relationship '%s' has low confidence score: %.2f", relationship.ID, relationshipScore.Confidence),
				RelationshipID: relationship.ID,
				Suggestion:     "Consider reviewing relationship extraction prompt or model parameters",
			}
			score.Issues = append(score.Issues, issue)
		}

		// Check for orphaned relationships
		if _, fromExists := entityMap[relationship.From]; !fromExists {
			issue := QualityIssue{
				Type:           IssueTypeRelationshipOrphaned,
				Severity:       SeverityHigh,
				Description:    fmt.Sprintf("Relationship '%s' references non-existent source entity", relationship.ID),
				RelationshipID: relationship.ID,
				Suggestion:     "Ensure all relationship entities are properly extracted",
			}
			score.Issues = append(score.Issues, issue)
		}

		if _, toExists := entityMap[relationship.To]; !toExists {
			issue := QualityIssue{
				Type:           IssueTypeRelationshipOrphaned,
				Severity:       SeverityHigh,
				Description:    fmt.Sprintf("Relationship '%s' references non-existent target entity", relationship.ID),
				RelationshipID: relationship.ID,
				Suggestion:     "Ensure all relationship entities are properly extracted",
			}
			score.Issues = append(score.Issues, issue)
		}
	}

	relationshipCount := float64(len(relationships))
	if relationshipCount > 0 {
		score.Confidence = totalConfidence / relationshipCount
		score.Accuracy = totalRelevance / relationshipCount
		score.Completeness = bqs.calculateRelationshipCompleteness(text, entities, relationships)
		score.Consistency = (totalStrength + totalBidirectionality) / (relationshipCount * 2)
	}

	// Calculate overall score
	score.OverallScore = (score.Confidence*0.3 + score.Accuracy*0.3 + score.Completeness*0.2 + score.Consistency*0.2)

	// Add performance metadata
	score.Metadata["scoring_time"] = time.Since(startTime)
	score.Metadata["relationship_count"] = len(relationships)
	score.Metadata["entity_count"] = len(entities)

	// Generate recommendations
	score.Recommendations = bqs.generateRelationshipRecommendations(score)

	return score, nil
}

// scoreIndividualRelationship scores a single relationship
func (bqs *BasicQualityScorer) scoreIndividualRelationship(ctx context.Context, text string, relationship schema.Edge, entities []schema.Node, allRelationships []schema.Edge, entityMap map[string]schema.Node) *RelationshipScore {
	relationshipScore := &RelationshipScore{
		RelationshipID: relationship.ID,
		FromEntity:     relationship.From,
		ToEntity:       relationship.To,
		Type:           string(relationship.Type),
		Issues:         make([]string, 0),
		Metadata:       make(map[string]interface{}),
	}

	// Calculate confidence based on relationship properties and context
	relationshipScore.Confidence = bqs.calculateRelationshipConfidence(text, relationship, entityMap)

	// Calculate relevance to the text
	relationshipScore.Relevance = bqs.calculateRelationshipRelevance(text, relationship, entityMap)

	// Calculate relationship strength
	relationshipScore.Strength = bqs.calculateRelationshipStrength(relationship, entityMap)

	// Calculate bidirectionality consistency
	relationshipScore.Bidirectionality = bqs.calculateBidirectionalityConsistency(relationship, allRelationships)

	// Check for specific issues
	if relationship.From == relationship.To {
		relationshipScore.Issues = append(relationshipScore.Issues, "Self-referencing relationship detected")
	}

	if relationship.Weight <= 0 {
		relationshipScore.Issues = append(relationshipScore.Issues, "Relationship has zero or negative weight")
	}

	return relationshipScore
}

// calculateRelationshipConfidence calculates confidence score for a relationship
func (bqs *BasicQualityScorer) calculateRelationshipConfidence(text string, relationship schema.Edge, entityMap map[string]schema.Node) float64 {
	confidence := 0.4 // Base confidence

	// Check if both entities exist
	fromEntity, fromExists := entityMap[relationship.From]
	toEntity, toExists := entityMap[relationship.To]

	if !fromExists || !toExists {
		return 0.1 // Very low confidence for orphaned relationships
	}

	// Check if relationship type is expected
	if bqs.isExpectedRelationshipType(string(relationship.Type)) {
		confidence += 0.2
	}

	// Check if entities are mentioned together in text
	fromName := ""
	toName := ""
	if name, ok := fromEntity.Properties["name"].(string); ok {
		fromName = strings.ToLower(name)
	}
	if name, ok := toEntity.Properties["name"].(string); ok {
		toName = strings.ToLower(name)
	}

	textLower := strings.ToLower(text)
	if fromName != "" && toName != "" {
		fromIndex := strings.Index(textLower, fromName)
		toIndex := strings.Index(textLower, toName)

		if fromIndex != -1 && toIndex != -1 {
			confidence += 0.3

			// Bonus for proximity
			distance := math.Abs(float64(fromIndex - toIndex))
			if distance < 100 { // Within 100 characters
				confidence += 0.1
			}
		}
	}

	return math.Min(confidence, 1.0)
}

// calculateRelationshipRelevance calculates relevance score for a relationship
func (bqs *BasicQualityScorer) calculateRelationshipRelevance(text string, relationship schema.Edge, entityMap map[string]schema.Node) float64 {
	relevance := 0.3 // Base relevance

	// Check if relationship is contextually relevant
	fromEntity, fromExists := entityMap[relationship.From]
	toEntity, toExists := entityMap[relationship.To]

	if !fromExists || !toExists {
		return 0.0
	}

	// Check entity relevance
	fromRelevance := bqs.calculateEntityRelevance(text, fromEntity)
	toRelevance := bqs.calculateEntityRelevance(text, toEntity)

	relevance += (fromRelevance + toRelevance) * 0.3

	// Check relationship type relevance
	relationshipType := strings.ToLower(string(relationship.Type))
	textLower := strings.ToLower(text)

	// Simple keyword matching for relationship types
	relationshipKeywords := map[string][]string{
		"related_to":     {"related", "connected", "associated"},
		"similar_to":     {"similar", "like", "comparable"},
		"part_of":        {"part", "component", "element"},
		"struggles_with": {"struggle", "difficult", "problem"},
		"synonym":        {"synonym", "same", "equivalent"},
	}

	if keywords, exists := relationshipKeywords[relationshipType]; exists {
		for _, keyword := range keywords {
			if strings.Contains(textLower, keyword) {
				relevance += 0.1
				break
			}
		}
	}

	return math.Min(relevance, 1.0)
}

// calculateRelationshipStrength calculates strength score for a relationship
func (bqs *BasicQualityScorer) calculateRelationshipStrength(relationship schema.Edge, entityMap map[string]schema.Node) float64 {
	strength := relationship.Weight

	// Normalize weight to 0-1 range if needed
	if strength > 1.0 {
		strength = 1.0
	}
	if strength < 0.0 {
		strength = 0.0
	}

	// Adjust based on entity quality
	fromEntity, fromExists := entityMap[relationship.From]
	toEntity, toExists := entityMap[relationship.To]

	if fromExists && toExists {
		// Consider entity weights if available
		fromWeight := fromEntity.Weight
		toWeight := toEntity.Weight

		if fromWeight > 0 && toWeight > 0 {
			entityQuality := (fromWeight + toWeight) / 2.0
			strength = (strength + entityQuality) / 2.0
		}
	}

	return strength
}

// calculateBidirectionalityConsistency calculates bidirectionality consistency
func (bqs *BasicQualityScorer) calculateBidirectionalityConsistency(relationship schema.Edge, allRelationships []schema.Edge) float64 {
	consistency := 1.0 // Assume consistent by default

	// Check for reverse relationship
	hasReverse := false
	for _, other := range allRelationships {
		if other.ID == relationship.ID {
			continue
		}

		if other.From == relationship.To && other.To == relationship.From {
			hasReverse = true

			// Check if weights are consistent
			weightDiff := math.Abs(relationship.Weight - other.Weight)
			if weightDiff > 0.3 {
				consistency -= 0.2 // Penalize inconsistent weights
			}
			break
		}
	}

	// Some relationship types should be bidirectional
	bidirectionalTypes := map[string]bool{
		"SIMILAR_TO": true,
		"SYNONYM":    true,
		"RELATED_TO": true,
	}

	if bidirectionalTypes[string(relationship.Type)] && !hasReverse {
		consistency -= 0.3 // Penalize missing reverse relationship
	}

	return math.Max(consistency, 0.0)
}

// calculateRelationshipCompleteness calculates completeness of relationship extraction
func (bqs *BasicQualityScorer) calculateRelationshipCompleteness(text string, entities []schema.Node, relationships []schema.Edge) float64 {
	if len(entities) < 2 {
		return 1.0 // Can't have relationships with fewer than 2 entities
	}

	// Estimate expected number of relationships based on entity count
	maxPossibleRelationships := len(entities) * (len(entities) - 1) / 2                            // n*(n-1)/2 for undirected graph
	expectedRelationships := math.Min(float64(maxPossibleRelationships), float64(len(entities)*2)) // Reasonable expectation

	actualRelationships := float64(len(relationships))

	if expectedRelationships == 0 {
		return 1.0
	}

	completeness := actualRelationships / expectedRelationships
	return math.Min(completeness, 1.0)
}

// isExpectedRelationshipType checks if relationship type is expected for the domain
func (bqs *BasicQualityScorer) isExpectedRelationshipType(relationshipType string) bool {
	for _, expected := range bqs.config.ExpectedRelationshipTypes {
		if expected == relationshipType {
			return true
		}
	}
	return false
}

// generateRelationshipRecommendations generates recommendations for relationship extraction improvement
func (bqs *BasicQualityScorer) generateRelationshipRecommendations(score *RelationshipQualityScore) []string {
	recommendations := make([]string, 0)

	if score.Confidence < 0.6 {
		recommendations = append(recommendations, "Consider improving relationship extraction prompts to increase confidence")
	}

	if score.Completeness < 0.5 {
		recommendations = append(recommendations, "Enhance relationship detection to improve completeness")
	}

	if score.Consistency < 0.6 {
		recommendations = append(recommendations, "Review relationship consistency and bidirectionality checks")
	}

	orphanedCount := 0
	for _, issue := range score.Issues {
		if issue.Type == IssueTypeRelationshipOrphaned {
			orphanedCount++
		}
	}

	if orphanedCount > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Fix %d orphaned relationships by ensuring entity extraction completeness", orphanedCount))
	}

	return recommendations
}
