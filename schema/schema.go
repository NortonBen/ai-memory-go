// Package schema defines the core data structures for the AI Memory system.
// It provides Node and Edge definitions for the knowledge graph, along with
// DataPoint structures for memory storage and processing.
package schema

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
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
	NodeTypePerson         NodeType = "Person"
	NodeTypeOrg            NodeType = "Org"
	NodeTypeProject        NodeType = "Project"
	NodeTypeTask           NodeType = "Task"
	NodeTypeEvent          NodeType = "Event"
	NodeTypeDocument       NodeType = "Document"
	NodeTypeSession        NodeType = "Session"
	NodeTypeUser           NodeType = "User"
)

// ChunkType defines the type of content chunk
type ChunkType string

const (
	ChunkTypeText      ChunkType = "text"
	ChunkTypeParagraph ChunkType = "paragraph"
	ChunkTypeSentence  ChunkType = "sentence"
	ChunkTypeMarkdown  ChunkType = "markdown"
	ChunkTypePDF       ChunkType = "pdf"
	ChunkTypeCode      ChunkType = "code"
)

// EdgeType defines the type of relationships between nodes
type EdgeType string

const (
	EdgeTypeRelatedTo     EdgeType = "RELATED_TO"
	EdgeTypeMentions      EdgeType = "MENTIONS"
	EdgeTypeWorksOn       EdgeType = "WORKS_ON"
	EdgeTypeDependsOn     EdgeType = "DEPENDS_ON"
	EdgeTypeDiscussedIn   EdgeType = "DISCUSSED_IN"
	EdgeTypeFailedAt      EdgeType = "FAILED_AT"
	EdgeTypeSynonym       EdgeType = "SYNONYM"
	EdgeTypeStrugglesWIth EdgeType = "STRUGGLES_WITH"
	EdgeTypeContains      EdgeType = "CONTAINS"
	EdgeTypeReferencedBy  EdgeType = "REFERENCED_BY"
	EdgeTypeSimilarTo     EdgeType = "SIMILAR_TO"
	EdgeTypePartOf        EdgeType = "PART_OF"
	EdgeTypeCreatedBy     EdgeType = "CREATED_BY"
	EdgeTypeUsedIn        EdgeType = "USED_IN"
	EdgeTypeContradicts   EdgeType = "CONTRADICTS"
	EdgeTypeUpdates       EdgeType = "UPDATES"
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

	// Extracted graph components for Memify phase
	Nodes []*Node `json:"nodes,omitempty"`
	Edges []Edge  `json:"edges,omitempty"`

	// Processing status
	ProcessingStatus ProcessingStatus `json:"processing_status"`
	ErrorMessage     string           `json:"error_message,omitempty"`
}

// Chunk represents a parsed piece of content with metadata
type Chunk struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Source    string                 `json:"source"`
	Type      ChunkType              `json:"type"`
	Offset    int64                  `json:"offset"`
	Line      int                    `json:"line"`
	Metadata  map[string]interface{} `json:"metadata"`
	Hash      string                 `json:"hash"`
	CreatedAt time.Time              `json:"created_at"`
}

// NewChunk creates a new chunk with proper initialization
func NewChunk(content, source string, chunkType ChunkType) *Chunk {
	id := GenerateChunkID(content, source)
	hash := GenerateContentHash(content)

	return &Chunk{
		ID:        id,
		Content:   content,
		Source:    source,
		Type:      chunkType,
		Hash:      hash,
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
	}
}

// ChunkingStrategy defines how content should be split into chunks
type ChunkingStrategy string

const (
	StrategyParagraph ChunkingStrategy = "paragraph"
	StrategySentence  ChunkingStrategy = "sentence"
	StrategyFixedSize ChunkingStrategy = "fixed_size"
	StrategySemantic  ChunkingStrategy = "semantic"
)

// ChunkingConfig configures how content is chunked
type ChunkingConfig struct {
	Strategy          ChunkingStrategy `json:"strategy"`
	MaxSize           int              `json:"max_size"`
	Overlap           int              `json:"overlap"`
	MinSize           int              `json:"min_size"`
	PreserveStructure bool             `json:"preserve_structure"`
}

// DefaultChunkingConfig returns a sensible default configuration
func DefaultChunkingConfig() *ChunkingConfig {
	return &ChunkingConfig{
		Strategy:          StrategyParagraph,
		MaxSize:           1000,
		Overlap:           100,
		MinSize:           50,
		PreserveStructure: true,
	}
}

// WorkerPoolConfig configures the worker pool behavior
type WorkerPoolConfig struct {
	NumWorkers    int           `json:"num_workers"`
	QueueSize     int           `json:"queue_size"`
	Timeout       time.Duration `json:"timeout"`
	RetryAttempts int           `json:"retry_attempts"`
	RetryDelay    time.Duration `json:"retry_delay"`
}

// DefaultWorkerPoolConfig returns a sensible default configuration
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		NumWorkers:    runtime.NumCPU(),
		QueueSize:     100,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    1 * time.Second,
	}
}

// WorkerPoolMetrics tracks performance metrics
type WorkerPoolMetrics struct {
	TasksSubmitted        int64         `json:"tasks_submitted"`
	TasksCompleted        int64         `json:"tasks_completed"`
	TasksFailed           int64         `json:"tasks_failed"`
	TasksRetried          int64         `json:"tasks_retried"`
	TotalProcessingTime   time.Duration `json:"total_processing_time"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	ActiveWorkers         int           `json:"active_workers"`
	QueueLength           int           `json:"queue_length"`
}

// CachePolicy defines different cache eviction policies
type CachePolicy string

const (
	PolicyLRU  CachePolicy = "lru"  // Least Recently Used
	PolicyLFU  CachePolicy = "lfu"  // Least Frequently Used
	PolicyTTL  CachePolicy = "ttl"  // Time To Live based
	PolicyFIFO CachePolicy = "fifo" // First In First Out
)

// CacheConfig configures the parser cache
type CacheConfig struct {
	Enabled                 bool          `json:"enabled"`
	MaxSize                 int           `json:"max_size"`
	MaxMemoryMB             int64         `json:"max_memory_mb"`
	TTL                     time.Duration `json:"ttl"`
	Policy                  CachePolicy   `json:"policy"`
	EnablePersistence       bool          `json:"enable_persistence"`
	PersistencePath         string        `json:"persistence_path"`
	CheckFileModTime        bool          `json:"check_file_mod_time"`
	EnableMetrics           bool          `json:"enable_metrics"`
	CleanupInterval         time.Duration `json:"cleanup_interval"`
	EnableCompression       bool          `json:"enable_compression"`
	CompressionThreshold    int           `json:"compression_threshold"`
	EnableAsyncPersistence  bool          `json:"enable_async_persistence"`
	PersistenceInterval     time.Duration `json:"persistence_interval"`
	EnableWarmup            bool          `json:"enable_warmup"`
	WarmupFiles             []string      `json:"warmup_files"`
	MaxConcurrentOperations int           `json:"max_concurrent_operations"`
	EnableDistributedCache  bool          `json:"enable_distributed_cache"`
}

// DefaultCacheConfig returns a default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:                 true,
		MaxSize:                 1000,
		MaxMemoryMB:             100,
		TTL:                     24 * time.Hour,
		Policy:                  "lru",
		EnablePersistence:       false,
		PersistencePath:         ".cache/parser_cache.json",
		CheckFileModTime:        true,
		EnableMetrics:           true,
		CleanupInterval:         5 * time.Minute,
		EnableCompression:       true,
		CompressionThreshold:    1024,
		EnableAsyncPersistence:  true,
		PersistenceInterval:     10 * time.Minute,
		EnableWarmup:            false,
		WarmupFiles:             []string{},
		MaxConcurrentOperations: 100,
		EnableDistributedCache:  false,
	}
}

// CacheMetrics holds metrics for the parser cache
type CacheMetrics struct {
	Mu                      sync.RWMutex
	Hits                    int64         `json:"hits"`
	Misses                  int64         `json:"misses"`
	HitRate                 float64       `json:"hit_rate"`
	Evictions               int64         `json:"evictions"`
	TotalEntries            int64         `json:"total_entries"`
	MemoryUsageBytes        int64         `json:"memory_usage_bytes"`
	AverageAccessTime       time.Duration `json:"average_access_time"`
	LastCleanup             time.Time     `json:"last_cleanup"`
	CompressionRatio        float64       `json:"compression_ratio"`
	PersistenceOperations   int64         `json:"persistence_operations"`
	LastPersistence         time.Time     `json:"last_persistence"`
	ConcurrentOperations    int64         `json:"concurrent_operations"`
	MaxConcurrentOperations int64         `json:"max_concurrent_operations"`
	WarmupTime              time.Duration `json:"warmup_time"`
	ErrorCount              int64         `json:"error_count"`
}

// StreamingConfig configures streaming parser behavior
type StreamingConfig struct {
	BufferSize             int `json:"buffer_size"`
	ChunkOverlap           int `json:"chunk_overlap"`
	MaxChunkSize           int `json:"max_chunk_size"`
	MinChunkSize           int `json:"min_chunk_size"`
	ProgressCallback       func(bytesProcessed, totalBytes int64, chunksCreated int)
	EnableProgressTracking bool          `json:"enable_progress_tracking"`
	FlushInterval          time.Duration `json:"flush_interval"`
}

// DefaultStreamingConfig returns sensible defaults for streaming parsing
func DefaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		BufferSize:             64 * 1024,
		ChunkOverlap:           1024,
		MaxChunkSize:           4 * 1024,
		MinChunkSize:           256,
		EnableProgressTracking: true,
		FlushInterval:          100 * time.Millisecond,
	}
}

// StreamingResult represents the result of streaming parsing
type StreamingResult struct {
	Chunks          []*Chunk               `json:"chunks"`
	TotalBytes      int64                  `json:"total_bytes"`
	ProcessingTime  time.Duration          `json:"processing_time"`
	ChunksCreated   int                    `json:"chunks_created"`
	MemoryPeakUsage int64                  `json:"memory_peak_usage"`
	Metadata        map[string]interface{} `json:"metadata"`
	IsComplete      bool                   `json:"is_complete"`
}

// GenerateChunkID creates a unique ID for a chunk based on content and metadata
func GenerateChunkID(content, source string) string {
	hash := sha256.Sum256([]byte(content + source))
	return fmt.Sprintf("chunk_%x", hash[:8])
}

// GenerateContentHash creates a hash of the content for deduplication
func GenerateContentHash(content string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return fmt.Sprintf("%x", hash)
}

// Existing Generate functions are kept above

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
	StatusCognified  ProcessingStatus = "cognified"
	StatusCompleted  ProcessingStatus = "completed"
	StatusFailed     ProcessingStatus = "failed"
	StatusRetrying   ProcessingStatus = "retrying"
)

// Role defines the sender of a message
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Message represents a single turn in a chat session
type Message struct {
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// MemorySession manages isolated memory contexts
type MemorySession struct {
	ID         string                 `json:"id"`
	UserID     string                 `json:"user_id,omitempty"`
	Context    map[string]interface{} `json:"context"`
	Messages   []Message              `json:"messages,omitempty"`
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

	// Deep copy messages
	if ms.Messages != nil {
		clone.Messages = make([]Message, len(ms.Messages))
		copy(clone.Messages, ms.Messages)
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

	// Deep copy graph drafts
	if dp.Nodes != nil {
		clone.Nodes = make([]*Node, len(dp.Nodes))
		for i, n := range dp.Nodes {
			if n != nil {
				clone.Nodes[i] = n.Clone()
			}
		}
	}

	if dp.Edges != nil {
		clone.Edges = make([]Edge, len(dp.Edges))
		for i, e := range dp.Edges {
			clone.Edges[i] = *e.Clone()
		}
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
	// Empty SessionID means global / unscoped memory (visible from every named session in search).
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
	MaxContextLength    int                    `json:"max_context_length,omitempty"` // Maximum characters to include in ParsedContext
	Analysis            *ThinkQueryAnalysis    `json:"analysis,omitempty"`           // Optional analysis for optimized retrieval
	FourTier            *FourTierSearchOptions `json:"four_tier,omitempty"`          // Ghi đè tìm kiếm bốn tầng theo request
}

// SearchResults contains the results of a search operation
type SearchResults struct {
	Results       []*SearchResult      `json:"results"`
	Total         int                  `json:"total"`
	QueryTime     time.Duration        `json:"query_time"`
	ContextSize   int                  `json:"context_size"`
	Answer        string               `json:"answer,omitempty"`
	ParsedContext string               `json:"parsed_context,omitempty"`
	FourTierStats *FourTierSearchStats `json:"four_tier_stats,omitempty"`
}

// RelationshipInfo represents a detected relationship between participants or entities
type RelationshipInfo struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Type EdgeType `json:"type"`
}

// RequestIntent determines the extracted intent of a user's request
type RequestIntent struct {
	NeedsVectorStorage bool               `json:"needs_vector_storage"`
	IsQuery            bool               `json:"is_query"`
	IsDelete           bool               `json:"is_delete"`
	DeleteTargets      []string           `json:"delete_targets,omitempty"`
	Relationships      []RelationshipInfo `json:"relationships,omitempty"`
	Reasoning          string             `json:"reasoning,omitempty"`
}

// ThinkQuery parameters for iterative multi-hop agentic retrieval
type ThinkQuery struct {
	Text      string                 `json:"text"`
	SessionID string                 `json:"session_id,omitempty"`
	Limit     int                    `json:"limit"`
	Filters   map[string]interface{} `json:"filters"`

	// Agentic Routing Options
	HopDepth           int  // Number of neighbor hops to traverse from anchors
	EnableThinking     bool // Toggle Agentic RAG / Iterative thinking loop
	MaxThinkingSteps   int  // Maximum iterations the agent can request 'missing_entities'
	LearnRelationships bool // Toggle automatic creation of 'BRIDGES_TO' relationships based on logic deduced during think
	IncludeReasoning   bool // Returns standard JSON with Thought Process included

	// Context Window Management
	MaxContextLength int  // Maximum characters per segment
	SegmentContext   bool // If true, process context in sequential segments

	// Query Analysis (Pre-Think)
	AnalyzeQuery bool                `json:"analyze_query"` // If true, perform LLM-based query analysis before retrieval
	Analysis     *ThinkQueryAnalysis `json:"analysis,omitempty"`

	// FourTier tùy chọn: truyền xuống bước retrieve (Search) trong Think.
	FourTier *FourTierSearchOptions `json:"four_tier,omitempty"`
}

// ThinkQueryAnalysis represents the LLM's pre-retrieval analysis of the query
type ThinkQueryAnalysis struct {
	QueryType      string   `json:"query_type"`      // Factual, Relational, Summarization, etc.
	Subjects       []string `json:"subjects"`        // Key entities to look for in Graph
	SearchKeywords []string `json:"search_keywords"` // Refined keywords for Vector search
	ExpectedAnswer string   `json:"expected_answer"` // Brief description of what the answer should look like
	Reasoning      string   `json:"reasoning"`       // LLM's logic for this analysis
}

// ThinkResult represents a single analytical step produced by the LLM
type ThinkResult struct {
	Intent          *RequestIntent      `json:"intent,omitempty"`
	Analysis        *ThinkQueryAnalysis `json:"analysis,omitempty"`
	Reasoning       string              `json:"reasoning,omitempty"`
	MissingEntities []string            `json:"missing_entities,omitempty"`
	Answer          string              `json:"answer,omitempty"`
	ContextUsed     *SearchResults      `json:"context_used,omitempty"`
}

// AgenticQueryResult is the final synthesized output containing reasoning flow
type AgenticQueryResult struct {
	ReasoningPath []string
	FinalAnswer   string
	ContextList   []string
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

// ResolutionAction defines how a new entity should handle a collision with an existing highly-similar entity.
type ResolutionAction string

const (
	ResolutionUpdate       ResolutionAction = "UPDATE"
	ResolutionContradict   ResolutionAction = "CONTRADICT"
	ResolutionIgnore       ResolutionAction = "IGNORE"
	ResolutionKeepSeparate ResolutionAction = "KEEP_SEPARATE"
)

// ConsistencyResult represents the outcome of an LLM evaluating an entity similarity collision.
type ConsistencyResult struct {
	Action     ResolutionAction       `json:"action"`
	Reason     string                 `json:"reason"`
	MergedData map[string]interface{} `json:"merged_data,omitempty"`
}

// ConvertToPtr converts a slice of Chunk to a slice of *Chunk
func ConvertToPtr(chunks []Chunk) []*Chunk {
	ptrChunks := make([]*Chunk, len(chunks))
	for i := range chunks {
		ptrChunks[i] = &chunks[i]
	}
	return ptrChunks
}

// ConvertToValue converts a slice of *Chunk to a slice of Chunk
func ConvertToValue(ptrChunks []*Chunk) []Chunk {
	chunks := make([]Chunk, len(ptrChunks))
	for i, ptr := range ptrChunks {
		if ptr != nil {
			chunks[i] = *ptr
		}
	}
	return chunks
}

// Connection represents a generic connection interface for any storage backend
type Connection interface {
	// Ping tests the connection
	Ping(ctx context.Context) error

	// Close closes the connection
	Close() error

	// IsValid checks if connection is still valid
	IsValid() bool

	// LastUsed returns when connection was last used
	LastUsed() time.Time

	// SetLastUsed updates the last used time
	SetLastUsed(t time.Time)
}

// ConnectionFactory creates new connections for a storage backend
type ConnectionFactory interface {
	CreateConnection(ctx context.Context) (Connection, error)
	ValidateConnection(ctx context.Context, conn Connection) error
}
