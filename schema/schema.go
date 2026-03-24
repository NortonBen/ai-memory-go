// Package schema defines the core data structures for the AI Memory system.
// It provides Node and Edge definitions for the knowledge graph, along with
// DataPoint structures for memory storage and processing.
package schema

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// NodeType defines the type of nodes in the knowledge graph
type NodeType string

const (
	NodeTypeConcept        NodeType = "Concept"
	NodeTypeWord           NodeType = "Word"
	NodeTypeUserPreference NodeType = "UserPreference"
	NodeTypeGrammarRule    NodeType = "GrammarRule"
	NodeTypeEntity         NodeType = "Entity"
	NodeTypeDocument       NodeType = "Document"
	NodeTypeSession        NodeType = "Session"
	NodeTypeUser           NodeType = "User"
)

// EdgeType defines the type of relationships between nodes
type EdgeType string

const (
	EdgeTypeRelatedTo     EdgeType = "RELATED_TO"
	EdgeTypeFailedAt      EdgeType = "FAILED_AT"
	EdgeTypeSynonym       EdgeType = "SYNONYM"
	EdgeTypeStrugglesWIth EdgeType = "STRUGGLES_WITH"
	EdgeTypeContains      EdgeType = "CONTAINS"
	EdgeTypeReferencedBy  EdgeType = "REFERENCED_BY"
	EdgeTypeSimilarTo     EdgeType = "SIMILAR_TO"
	EdgeTypePartOf        EdgeType = "PART_OF"
	EdgeTypeCreatedBy     EdgeType = "CREATED_BY"
	EdgeTypeUsedIn        EdgeType = "USED_IN"
)

// Node represents a node in the knowledge graph
type Node struct {
	ID         string                 `json:"id"`
	Type       NodeType               `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`

	// Metadata for processing
	SessionID string  `json:"session_id,omitempty"`
	UserID    string  `json:"user_id,omitempty"`
	Weight    float64 `json:"weight,omitempty"`
}

// Edge represents a relationship between two nodes
type Edge struct {
	ID         string                 `json:"id"`
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Type       EdgeType               `json:"type"`
	Weight     float64                `json:"weight"`
	Properties map[string]interface{} `json:"properties"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`

	// Metadata for processing
	SessionID string `json:"session_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

// DataPoint represents the fundamental memory unit in the system
type DataPoint struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	ContentType string                 `json:"content_type"`
	Metadata    map[string]interface{} `json:"metadata"`
	Embedding   []float32              `json:"embedding,omitempty"`
	SessionID   string                 `json:"session_id"`
	UserID      string                 `json:"user_id,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`

	// Graph relationships
	Relationships []Relationship `json:"relationships,omitempty"`

	// Processing status
	ProcessingStatus ProcessingStatus `json:"processing_status"`
	ErrorMessage     string           `json:"error_message,omitempty"`
}

// Relationship defines connections between DataPoints
type Relationship struct {
	Type     EdgeType               `json:"type"`
	Target   string                 `json:"target"`
	Weight   float64                `json:"weight"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ProcessingStatus tracks the processing state of a DataPoint
type ProcessingStatus string

const (
	StatusPending    ProcessingStatus = "pending"
	StatusProcessing ProcessingStatus = "processing"
	StatusCompleted  ProcessingStatus = "completed"
	StatusFailed     ProcessingStatus = "failed"
	StatusRetrying   ProcessingStatus = "retrying"
)

// MemorySession manages isolated memory contexts
type MemorySession struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"user_id,omitempty"`
	Context    map[string]interface{} `json:"context"`
	CreatedAt  time.Time              `json:"created_at"`
	LastAccess time.Time              `json:"last_access"`
	IsActive   bool                   `json:"is_active"`
	ExpiresAt  *time.Time             `json:"expires_at,omitempty"`
}

// Clone creates a deep copy of a Node
func (n *Node) Clone() *Node {
	clone := &Node{
		ID:        n.ID,
		Type:      n.Type,
		CreatedAt: n.CreatedAt,
		UpdatedAt: time.Now(),
		SessionID: n.SessionID,
		UserID:    n.UserID,
		Weight:    n.Weight,
	}

	// Deep copy properties
	if n.Properties != nil {
		clone.Properties = make(map[string]interface{})
		for k, v := range n.Properties {
			clone.Properties[k] = v
		}
	}

	return clone
}

// ToDataPoint converts a Node to a DataPoint representation
func (n *Node) ToDataPoint() *DataPoint {
	metadata := make(map[string]interface{})
	metadata["node_type"] = string(n.Type)
	metadata["node_weight"] = n.Weight
	for k, v := range n.Properties {
		metadata[k] = v
	}

	content := ""
	if name, ok := n.Properties["name"].(string); ok {
		content = name
	} else if label, ok := n.Properties["label"].(string); ok {
		content = label
	}

	return &DataPoint{
		ID:               n.ID,
		Content:          content,
		ContentType:      "node",
		Metadata:         metadata,
		SessionID:        n.SessionID,
		UserID:           n.UserID,
		CreatedAt:        n.CreatedAt,
		UpdatedAt:        n.UpdatedAt,
		ProcessingStatus: StatusCompleted,
	}
}

// Clone creates a deep copy of an Edge
func (e *Edge) Clone() *Edge {
	clone := &Edge{
		ID:        e.ID,
		From:      e.From,
		To:        e.To,
		Type:      e.Type,
		Weight:    e.Weight,
		CreatedAt: e.CreatedAt,
		UpdatedAt: time.Now(),
		SessionID: e.SessionID,
		UserID:    e.UserID,
	}

	// Deep copy properties
	if e.Properties != nil {
		clone.Properties = make(map[string]interface{})
		for k, v := range e.Properties {
			clone.Properties[k] = v
		}
	}

	return clone
}

// ToRelationship converts an Edge to a Relationship
func (e *Edge) ToRelationship() Relationship {
	return Relationship{
		Type:     e.Type,
		Target:   e.To,
		Weight:   e.Weight,
		Metadata: e.Properties,
	}
}

// Clone creates a deep copy of a MemorySession
func (ms *MemorySession) Clone() *MemorySession {
	clone := &MemorySession{
		ID:         ms.ID,
		UserID:     ms.UserID,
		CreatedAt:  ms.CreatedAt,
		LastAccess: time.Now(),
		IsActive:   ms.IsActive,
	}

	// Deep copy context
	if ms.Context != nil {
		clone.Context = make(map[string]interface{})
		for k, v := range ms.Context {
			clone.Context[k] = v
		}
	}

	// Copy ExpiresAt if set
	if ms.ExpiresAt != nil {
		expires := *ms.ExpiresAt
		clone.ExpiresAt = &expires
	}

	return clone
}

// Clone creates a deep copy of a DataPoint
func (dp *DataPoint) Clone() *DataPoint {
	clone := &DataPoint{
		ID:               dp.ID,
		Content:          dp.Content,
		ContentType:      dp.ContentType,
		SessionID:        dp.SessionID,
		UserID:           dp.UserID,
		CreatedAt:        dp.CreatedAt,
		UpdatedAt:        time.Now(),
		ProcessingStatus: dp.ProcessingStatus,
		ErrorMessage:     dp.ErrorMessage,
	}

	// Deep copy metadata
	if dp.Metadata != nil {
		clone.Metadata = make(map[string]interface{})
		for k, v := range dp.Metadata {
			clone.Metadata[k] = v
		}
	}

	// Deep copy embedding
	if dp.Embedding != nil {
		clone.Embedding = make([]float32, len(dp.Embedding))
		copy(clone.Embedding, dp.Embedding)
	}

	// Deep copy relationships
	if dp.Relationships != nil {
		clone.Relationships = make([]Relationship, len(dp.Relationships))
		copy(clone.Relationships, dp.Relationships)
	}

	return clone
}

// ToJSON converts a DataPoint to JSON string
func (dp *DataPoint) ToJSON() (string, error) {
	data, err := json.Marshal(dp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON creates a DataPoint from JSON string
func DataPointFromJSON(jsonStr string) (*DataPoint, error) {
	var dp DataPoint
	err := json.Unmarshal([]byte(jsonStr), &dp)
	if err != nil {
		return nil, err
	}
	return &dp, nil
}

// Validate checks if the Node has required fields
func (n *Node) Validate() error {
	if n.ID == "" {
		return fmt.Errorf("Node ID is required")
	}
	if n.Type == "" {
		return fmt.Errorf("Node Type is required")
	}
	if n.Properties == nil {
		n.Properties = make(map[string]interface{})
	}
	return nil
}

// Validate checks if the Edge has required fields
func (e *Edge) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("Edge ID is required")
	}
	if e.From == "" {
		return fmt.Errorf("Edge From node is required")
	}
	if e.To == "" {
		return fmt.Errorf("Edge To node is required")
	}
	if e.Type == "" {
		return fmt.Errorf("Edge Type is required")
	}
	if e.From == e.To {
		return fmt.Errorf("Edge cannot connect a node to itself")
	}
	if e.Properties == nil {
		e.Properties = make(map[string]interface{})
	}
	return nil
}

// Validate checks if the MemorySession has required fields
func (ms *MemorySession) Validate() error {
	if ms.ID == "" {
		return fmt.Errorf("MemorySession ID is required")
	}
	if ms.Context == nil {
		ms.Context = make(map[string]interface{})
	}
	return nil
}

// Validate checks if the ProcessedQuery has required fields
func (pq *ProcessedQuery) Validate() error {
	if pq.OriginalText == "" {
		return fmt.Errorf("ProcessedQuery OriginalText is required")
	}
	if len(pq.Vector) == 0 {
		return fmt.Errorf("ProcessedQuery Vector is required")
	}
	if pq.Metadata == nil {
		pq.Metadata = make(map[string]interface{})
	}
	return nil
}

// Validate checks if the SearchResult has required fields
func (sr *SearchResult) Validate() error {
	if sr.DataPoint == nil {
		return fmt.Errorf("SearchResult DataPoint is required")
	}
	if err := sr.DataPoint.Validate(); err != nil {
		return fmt.Errorf("SearchResult DataPoint validation failed: %w", err)
	}
	if sr.Score < 0 || sr.Score > 1 {
		return fmt.Errorf("SearchResult Score must be between 0 and 1")
	}
	if sr.Mode == "" {
		return fmt.Errorf("SearchResult Mode is required")
	}
	if sr.Metadata == nil {
		sr.Metadata = make(map[string]interface{})
	}
	// Validate score components if present
	if sr.VectorScore < 0 || sr.VectorScore > 1 {
		return fmt.Errorf("SearchResult VectorScore must be between 0 and 1")
	}
	if sr.GraphScore < 0 || sr.GraphScore > 1 {
		return fmt.Errorf("SearchResult GraphScore must be between 0 and 1")
	}
	if sr.TemporalScore < 0 || sr.TemporalScore > 1 {
		return fmt.Errorf("SearchResult TemporalScore must be between 0 and 1")
	}
	if sr.UserScore < 0 || sr.UserScore > 1 {
		return fmt.Errorf("SearchResult UserScore must be between 0 and 1")
	}
	return nil
}

// Validate checks if the DataPoint has required fields
func (dp *DataPoint) Validate() error {
	if dp.ID == "" {
		return fmt.Errorf("DataPoint ID is required")
	}
	if dp.Content == "" {
		return fmt.Errorf("DataPoint Content is required")
	}
	if dp.SessionID == "" {
		return fmt.Errorf("DataPoint SessionID is required")
	}
	return nil
}

// NewNode creates a new Node with proper initialization
func NewNode(nodeType NodeType, properties map[string]interface{}) *Node {
	return &Node{
		ID:         generateNodeID(),
		Type:       nodeType,
		Properties: properties,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Weight:     1.0,
	}
}

// NewEdge creates a new Edge with proper initialization
func NewEdge(from, to string, edgeType EdgeType, weight float64) *Edge {
	return &Edge{
		ID:         generateEdgeID(),
		From:       from,
		To:         to,
		Type:       edgeType,
		Weight:     weight,
		Properties: make(map[string]interface{}),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// ProcessedQuery represents a search query after processing and vectorization
type ProcessedQuery struct {
	OriginalText string                 `json:"original_text"`
	Vector       []float32              `json:"vector"`
	Entities     []*Node                `json:"entities"`
	Keywords     []string               `json:"keywords"`
	Language     string                 `json:"language,omitempty"`
	Intent       string                 `json:"intent,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ProcessedAt  time.Time              `json:"processed_at"`
}

// AnchorNode represents a node found during hybrid search with relevance score
type AnchorNode struct {
	Node   *Node   `json:"node"`
	Score  float64 `json:"score"`
	Source string  `json:"source"` // "vector", "entity", "keyword"
	Rank   int     `json:"rank"`
}

// EnrichedNode represents a node with its neighborhood context for graph traversal
type EnrichedNode struct {
	Core              *Node   `json:"core"`
	DirectNeighbors   []*Node `json:"direct_neighbors"`
	IndirectNeighbors []*Node `json:"indirect_neighbors"`
	RelevanceScore    float64 `json:"relevance_score"`
	ContextSummary    string  `json:"context_summary,omitempty"`
	TraversalDepth    int     `json:"traversal_depth"`
}

// TimeRange defines a start and end time for temporal searches
type TimeRange struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// SearchQuery defines search parameters and modes
type SearchQuery struct {
	Text                string                 `json:"text"`
	SessionID           string                 `json:"session_id"`
	Mode                RetrievalMode          `json:"mode"`
	Limit               int                    `json:"limit"`
	SimilarityThreshold float64                `json:"similarity_threshold"`
	Filters             map[string]interface{} `json:"filters"`
	HopDepth            int                    `json:"hop_depth"` // Optional graph traversal depth
	TimeRange           *TimeRange             `json:"time_range,omitempty"`
}

// SearchResults contains the results of a search operation
type SearchResults struct {
	Results       []*SearchResult `json:"results"`
	Total         int             `json:"total"`
	QueryTime     time.Duration   `json:"query_time"`
	ContextSize   int             `json:"context_size"`
	Answer        string          `json:"answer,omitempty"`
	ParsedContext string          `json:"parsed_context,omitempty"`
}

// ThinkQuery defines parameters for the reasoning and answer generation
type ThinkQuery struct {
	Text             string `json:"text"`
	SessionID        string `json:"session_id"`
	Limit            int    `json:"limit"`             // Limit for vector search
	HopDepth         int    `json:"hop_depth"`         // Depth of graph traversal (1 or 2)
	IncludeReasoning bool   `json:"include_reasoning"` // If false, skip reasoning for faster answer

	// Iterative Agentic Features
	EnableThinking     bool `json:"enable_thinking"`      // Triggers iterative reasoning loop
	MaxThinkingSteps   int  `json:"max_thinking_steps"`   // Max depth of iterations allowed
	LearnRelationships bool `json:"learn_relationships"`  // Triggers relationship extraction & persistence upon success
}

// ThinkResult represents the structured output of a MemoryEngine Think operation
type ThinkResult struct {
	Reasoning   string         `json:"reasoning"`
	Answer      string         `json:"answer"`
	ContextUsed *SearchResults `json:"context_used,omitempty"`
}


// SearchResult represents a final search result with rich context for LLM consumption
type SearchResult struct {
	DataPoint     *DataPoint             `json:"datapoint"`
	Score         float64                `json:"score"`
	Mode          RetrievalMode          `json:"mode"`
	Relationships []RelationshipContext  `json:"relationships"`
	Neighborhood  *NeighborhoodSummary   `json:"neighborhood,omitempty"`
	Explanation   string                 `json:"explanation,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`

	// Multi-factor scoring breakdown
	VectorScore   float64 `json:"vector_score,omitempty"`
	GraphScore    float64 `json:"graph_score,omitempty"`
	TemporalScore float64 `json:"temporal_score,omitempty"`
	UserScore     float64 `json:"user_score,omitempty"`

	// Search context
	QueryTime     time.Duration `json:"query_time,omitempty"`
	TraversalPath []string      `json:"traversal_path,omitempty"`
	Rank          int           `json:"rank"`
}

// RetrievalMode defines different search strategies
type RetrievalMode string

const (
	ModeSemanticSearch RetrievalMode = "semantic_search"
	ModeGraphTraversal RetrievalMode = "graph_traversal"
	ModeHybridSearch   RetrievalMode = "hybrid_search"
	ModeTemporalSearch RetrievalMode = "temporal_search"
	ModeContextualRAG  RetrievalMode = "contextual_rag"
)

// RelationshipContext provides context about relationships in search results
type RelationshipContext struct {
	Type        EdgeType `json:"type"`
	Target      string   `json:"target"`
	TargetLabel string   `json:"target_label,omitempty"`
	Weight      float64  `json:"weight"`
	Description string   `json:"description,omitempty"`
}

// NeighborhoodSummary provides a summary of a node's neighborhood
type NeighborhoodSummary struct {
	DirectCount   int                    `json:"direct_count"`
	IndirectCount int                    `json:"indirect_count"`
	TopConcepts   []string               `json:"top_concepts"`
	SummaryText   string                 `json:"summary_text,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Helper functions for ID generation with uniqueness guarantee
var idCounter uint64
var idMutex sync.Mutex

func generateNodeID() string {
	idMutex.Lock()
	defer idMutex.Unlock()
	idCounter++

	// Combine timestamp, counter, and random bytes for uniqueness
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)

	return fmt.Sprintf("node_%d_%d_%s", timestamp, idCounter, hex.EncodeToString(randomBytes))
}

func generateEdgeID() string {
	idMutex.Lock()
	defer idMutex.Unlock()
	idCounter++

	// Combine timestamp, counter, and random bytes for uniqueness
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)

	return fmt.Sprintf("edge_%d_%d_%s", timestamp, idCounter, hex.EncodeToString(randomBytes))
}

// NewSearchResult creates a new SearchResult with proper initialization
func NewSearchResult(dataPoint *DataPoint, score float64, mode RetrievalMode) *SearchResult {
	return &SearchResult{
		DataPoint:     dataPoint,
		Score:         score,
		Mode:          mode,
		Relationships: make([]RelationshipContext, 0),
		Metadata:      make(map[string]interface{}),
		Rank:          0,
	}
}

// Clone creates a deep copy of a SearchResult
func (sr *SearchResult) Clone() *SearchResult {
	clone := &SearchResult{
		DataPoint:     sr.DataPoint.Clone(),
		Score:         sr.Score,
		Mode:          sr.Mode,
		Explanation:   sr.Explanation,
		VectorScore:   sr.VectorScore,
		GraphScore:    sr.GraphScore,
		TemporalScore: sr.TemporalScore,
		UserScore:     sr.UserScore,
		QueryTime:     sr.QueryTime,
		Rank:          sr.Rank,
	}

	// Deep copy relationships
	if sr.Relationships != nil {
		clone.Relationships = make([]RelationshipContext, len(sr.Relationships))
		copy(clone.Relationships, sr.Relationships)
	}

	// Deep copy neighborhood
	if sr.Neighborhood != nil {
		clone.Neighborhood = &NeighborhoodSummary{
			DirectCount:   sr.Neighborhood.DirectCount,
			IndirectCount: sr.Neighborhood.IndirectCount,
			SummaryText:   sr.Neighborhood.SummaryText,
		}
		if sr.Neighborhood.TopConcepts != nil {
			clone.Neighborhood.TopConcepts = make([]string, len(sr.Neighborhood.TopConcepts))
			copy(clone.Neighborhood.TopConcepts, sr.Neighborhood.TopConcepts)
		}
		if sr.Neighborhood.Metadata != nil {
			clone.Neighborhood.Metadata = make(map[string]interface{})
			for k, v := range sr.Neighborhood.Metadata {
				clone.Neighborhood.Metadata[k] = v
			}
		}
	}

	// Deep copy metadata
	if sr.Metadata != nil {
		clone.Metadata = make(map[string]interface{})
		for k, v := range sr.Metadata {
			clone.Metadata[k] = v
		}
	}

	// Deep copy traversal path
	if sr.TraversalPath != nil {
		clone.TraversalPath = make([]string, len(sr.TraversalPath))
		copy(clone.TraversalPath, sr.TraversalPath)
	}

	return clone
}

// ToJSON converts a SearchResult to JSON string
func (sr *SearchResult) ToJSON() (string, error) {
	data, err := json.Marshal(sr)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON creates a SearchResult from JSON string
func SearchResultFromJSON(jsonStr string) (*SearchResult, error) {
	var sr SearchResult
	err := json.Unmarshal([]byte(jsonStr), &sr)
	if err != nil {
		return nil, err
	}
	return &sr, nil
}

// AddRelationship adds a relationship context to the search result
func (sr *SearchResult) AddRelationship(relType EdgeType, target, targetLabel string, weight float64, description string) {
	rel := RelationshipContext{
		Type:        relType,
		Target:      target,
		TargetLabel: targetLabel,
		Weight:      weight,
		Description: description,
	}
	sr.Relationships = append(sr.Relationships, rel)
}

// SetNeighborhood sets the neighborhood summary for the search result
func (sr *SearchResult) SetNeighborhood(directCount, indirectCount int, topConcepts []string, summaryText string) {
	sr.Neighborhood = &NeighborhoodSummary{
		DirectCount:   directCount,
		IndirectCount: indirectCount,
		TopConcepts:   topConcepts,
		SummaryText:   summaryText,
		Metadata:      make(map[string]interface{}),
	}
}

// SetScoreBreakdown sets the multi-factor scoring breakdown
func (sr *SearchResult) SetScoreBreakdown(vectorScore, graphScore, temporalScore, userScore float64) {
	sr.VectorScore = vectorScore
	sr.GraphScore = graphScore
	sr.TemporalScore = temporalScore
	sr.UserScore = userScore
}

// CalculateFinalScore computes the final score from component scores using default weights
// Default weights: Vector(40%), Graph(30%), Temporal(20%), User(10%)
func (sr *SearchResult) CalculateFinalScore() float64 {
	return sr.VectorScore*0.4 + sr.GraphScore*0.3 + sr.TemporalScore*0.2 + sr.UserScore*0.1
}

// CalculateFinalScoreWithWeights computes the final score with custom weights
func (sr *SearchResult) CalculateFinalScoreWithWeights(vectorWeight, graphWeight, temporalWeight, userWeight float64) float64 {
	return sr.VectorScore*vectorWeight + sr.GraphScore*graphWeight + sr.TemporalScore*temporalWeight + sr.UserScore*userWeight
}

// GetRelationshipsByType returns all relationships of a specific type
func (sr *SearchResult) GetRelationshipsByType(relType EdgeType) []RelationshipContext {
	var results []RelationshipContext
	for _, rel := range sr.Relationships {
		if rel.Type == relType {
			results = append(results, rel)
		}
	}
	return results
}

// HasRelationshipTo checks if the search result has a relationship to a specific target
func (sr *SearchResult) HasRelationshipTo(targetID string) bool {
	for _, rel := range sr.Relationships {
		if rel.Target == targetID {
			return true
		}
	}
	return false
}

// GetContextSummary generates a human-readable summary of the search result context
func (sr *SearchResult) GetContextSummary() string {
	summary := fmt.Sprintf("Result (Score: %.2f, Mode: %s)\n", sr.Score, sr.Mode)
	summary += fmt.Sprintf("Content: %s\n", sr.DataPoint.Content)

	if len(sr.Relationships) > 0 {
		summary += fmt.Sprintf("Relationships: %d\n", len(sr.Relationships))
		for i, rel := range sr.Relationships {
			if i < 3 { // Show first 3 relationships
				summary += fmt.Sprintf("  - %s -> %s (%.2f)\n", rel.Type, rel.TargetLabel, rel.Weight)
			}
		}
		if len(sr.Relationships) > 3 {
			summary += fmt.Sprintf("  ... and %d more\n", len(sr.Relationships)-3)
		}
	}

	if sr.Neighborhood != nil {
		summary += fmt.Sprintf("Neighborhood: %d direct, %d indirect neighbors\n",
			sr.Neighborhood.DirectCount, sr.Neighborhood.IndirectCount)
		if len(sr.Neighborhood.TopConcepts) > 0 {
			summary += fmt.Sprintf("Top Concepts: %v\n", sr.Neighborhood.TopConcepts)
		}
	}

	return summary
}

// IsRelevant checks if the search result meets a minimum relevance threshold
func (sr *SearchResult) IsRelevant(threshold float64) bool {
	return sr.Score >= threshold
}

// GetTraversalDepth returns the depth of graph traversal used to find this result
func (sr *SearchResult) GetTraversalDepth() int {
	return len(sr.TraversalPath)
}
