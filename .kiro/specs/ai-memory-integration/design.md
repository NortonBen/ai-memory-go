# AI Memory Integration - Technical Design Document

## Overview

The AI Memory Integration library is a Go-native refactor of Cognee's Python architecture, designed as a high-performance memory layer for AI applications. This library implements a Data-Driven Pipeline using Go's concurrency primitives (channels, goroutines, worker pools) to process data in parallel, replacing Python's flexibility with Go's speed and efficiency.

The system provides four core operations (Add, Cognify, Memify, Search) and supports multiple LLM providers, pluggable storage backends, and seamless integration with Wails desktop applications and Go AI services.

### Key Design Principles

- **Data-Driven Pipeline**: Worker pools process data through structured stages using Go channels
- **Concurrency-First**: Leverages goroutines and channels for parallel processing
- **Multi-Provider LLM**: Supports OpenAI, Anthropic, Gemini, Ollama, DeepSeek, Mistral, Bedrock
- **Hybrid Storage**: Combines graph databases, vector stores, and relational databases
- **Go-Native**: Pure Go implementation with idiomatic interfaces and zero Python dependencies
- **Production-Ready**: Built for high-throughput AI services with proper error handling

## Architecture

### High-Level Data-Driven Pipeline Architecture

The AI Memory Integration library follows a layered architecture based on Data-Driven Pipeline principles, using Go's concurrency features for high-performance parallel processing:

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Wails Application Layer                               │
├─────────────────────────────────────────────────────────────────────────────────┤
│                              Memory API Layer                                   │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐              │
│  │    Add()    │ │  Cognify()  │ │  Memify()   │ │  Search()   │              │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘              │
├─────────────────────────────────────────────────────────────────────────────────┤
│                         Data-Driven Pipeline Core                               │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐│
│  │ Ingestion Layer │ │Orchestration    │ │ Extraction      │ │ Storage Layer   ││
│  │ (Go Interfaces) │ │Layer ("Brain")  │ │ Layer (LLM)     │ │ (Hybrid)        ││
│  │                 │ │ Task Queue      │ │ Bridge          │ │                 ││
│  │ • Markdown      │ │ • Channels      │ │ • Ollama        │ │ • Vector: pgvec ││
│  │ • PDF           │ │ • Goroutines    │ │ • DeepSeek      │ │ • Graph: Surreal││
│  │ • Text          │ │ • Worker Pools  │ │ • JSON Schema   │ │ • Relational    ││
│  └─────────────────┘ └─────────────────┘ └─────────────────┘ └─────────────────┘│
├─────────────────────────────────────────────────────────────────────────────────┤
│                            Package Structure                                    │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐│
│  │ Package parser  │ │ Package schema  │ │Package extractor│ │Package graph &  ││
│  │ • File → Chunk  │ │ • Node & Edge   │ │ • LLM Calls     │ │ vector          ││
│  │ • Multi-format  │ │ • Structs       │ │ • JSON Schema   │ │ • Interfaces    ││
│  │ • go-doc-extract│ │ • Concept/Word  │ │ • DeepSeek Mode │ │ • Storage Impl  ││
│  └─────────────────┘ └─────────────────┘ └─────────────────┘ └─────────────────┘│
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Core Package Architecture

#### A. Package `parser`
**Nhiệm vụ**: Chuyển đổi mọi loại file về `[]Chunk`
**Công cụ**: `go-document-extractor` hoặc wrap các thư viện C++

```go
type Parser interface {
    ParseFile(filePath string) ([]Chunk, error)
    ParseText(content string) ([]Chunk, error)
    ParseMarkdown(content string) ([]Chunk, error)
    ParsePDF(filePath string) ([]Chunk, error)
}

type Chunk struct {
    ID       string                 `json:"id"`
    Content  string                 `json:"content"`
    Type     ChunkType              `json:"type"`
    Metadata map[string]interface{} `json:"metadata"`
    Source   string                 `json:"source"`
}
```

#### B. Package `schema` (Cực kỳ quan trọng)
**Định nghĩa**: Các struct cho Node và Edge
**Ví dụ**: Node có thể là `Concept`, `Word`, `UserPreference`. Edge là `RELATED_TO`, `FAILED_AT`, `SYNONYM`

```go
type Node struct {
    ID         string                 `json:"id"`
    Type       NodeType               `json:"type"`
    Properties map[string]interface{} `json:"properties"`
    CreatedAt  time.Time              `json:"created_at"`
}

type Edge struct {
    ID         string                 `json:"id"`
    From       string                 `json:"from"`
    To         string                 `json:"to"`
    Type       EdgeType               `json:"type"`
    Weight     float64                `json:"weight"`
    Properties map[string]interface{} `json:"properties"`
}

type NodeType string
const (
    NodeTypeConcept        NodeType = "Concept"
    NodeTypeWord           NodeType = "Word"
    NodeTypeUserPreference NodeType = "UserPreference"
    NodeTypeGrammarRule    NodeType = "GrammarRule"
)

type EdgeType string
const (
    EdgeTypeRelatedTo     EdgeType = "RELATED_TO"
    EdgeTypeFailedAt      EdgeType = "FAILED_AT"
    EdgeTypeSynonym       EdgeType = "SYNONYM"
    EdgeTypeStrugglesWIth EdgeType = "STRUGGLES_WITH"
)
```

#### C. Package `extractor`
**Đây là nơi gọi LLM**
**Kỹ thuật**: Sử dụng JSON Schema Mode của DeepSeek để ép AI trả về đúng struct Go

```go
type LLMExtractor interface {
    ExtractEntities(ctx context.Context, text string) ([]Node, error)
    ExtractRelationships(ctx context.Context, text string, entities []Node) ([]Edge, error)
    ExtractWithSchema(ctx context.Context, text string, schema interface{}) (interface{}, error)
}

type DeepSeekExtractor struct {
    apiKey   string
    endpoint string
    model    string
}

// JSON Schema Mode implementation
func (d *DeepSeekExtractor) ExtractWithSchema(ctx context.Context, text string, schema interface{}) (interface{}, error) {
    // Generate JSON schema from Go struct
    jsonSchema := generateJSONSchema(schema)
    
    prompt := fmt.Sprintf(`
    Extract entities and relations from the following text.
    Return response in this exact JSON schema: %s
    
    Text: %s
    `, jsonSchema, text)
    
    // Call DeepSeek with JSON mode
    response, err := d.callDeepSeek(ctx, prompt, true)
    if err != nil {
        return nil, err
    }
    
    // Parse JSON response into Go struct
    return parseJSONToStruct(response, schema)
}
```

### Data Flow Examples

#### Example 1: Memory Storage (English Learning Use Case)

**Input**: "Tôi thường quên cách dùng thì Hiện tại hoàn thành (Present Perfect)."

**Storage Pipeline Flow**:
1. **Parser**: Go nhận tin nhắn → Tạo Chunk với metadata
2. **Extractor**: Gửi sang LLM với prompt "Extract entities and relations"
3. **LLM Output**: `{Entity: "Present Perfect", Type: "Grammar", Relation: "User_Struggles_With"}`
4. **Graph Update**: Go tạo Node "Present Perfect" và nối Edge "Struggles" từ Node "User"
5. **Vector Update**: Lưu đoạn text vào Vector DB để search "Present Perfect" sẽ ra ngay ngữ cảnh này

#### Example 2: Search Query (4-Step Process)

**Query**: "Cách dùng thì Hiện tại hoàn thành mà tôi đã học là gì?"

**Search Pipeline Flow**:
1. **Step 1 - Input Processing**: 
   - Vector: [0.1, 0.3, -0.2, ...] (768-dim embedding)
   - Entities: ["Present Perfect", "User"]
   - Keywords: ["cách dùng", "hiện tại hoàn thành", "đã học"]

2. **Step 2 - Hybrid Search**:
   - Vector Search: Top-20 similar nodes (score > 0.7)
   - Entity Search: Nodes matching "Present Perfect" + "Grammar"
   - Anchor Nodes: [present_perfect_node, user_progress_node, grammar_rules_node]

3. **Step 3 - Graph Traversal**:
   - 1-hop: "Have/Has + Past Participle", "Since/For indicators", "User struggled"
   - 2-hop: "Past Simple vs Present Perfect", "Common mistakes"
   - Enriched Context: Core concept + Direct neighbors + Indirect neighbors

4. **Step 4 - Context Assembly**:
   - Reranking: Vector(40%) + Graph(30%) + Temporal(20%) + User(10%)
   - Rich Context: Grammar rule + Formula + Usage + Learning history + Mistakes
   - Final Results: Ranked list with relationship metadata

**Worker Pool Architecture**:
```go
type PipelineWorker struct {
    inputChan    chan *ProcessingTask
    outputChan   chan *ProcessingResult
    errorChan    chan error
    workerID     int
    llmExtractor LLMExtractor
    storage      Storage
}

func (w *PipelineWorker) Start(ctx context.Context) {
    for {
        select {
        case task := <-w.inputChan:
            result := w.processTask(ctx, task)
            w.outputChan <- result
        case <-ctx.Done():
            return
        }
    }
}
```

### Core Pipeline Flow

The memory processing follows two main workflows:

#### 1. Memory Storage Pipeline (Add → Cognify → Memify)
```
Raw Data → Add() → DataPoint → Cognify() → Enriched DataPoint → Memify()
                                                                    ↓
                                                    ┌─────────────────┼─────────────────┐
                                                    ↓                 ↓                 ↓
                                              Graph Store      Vector Store    Relational Store
```

#### 2. Search Pipeline (4-Step Cognee Process)
```
Query Input → Step 1: Input Processing → Step 2: Hybrid Search → Step 3: Graph Traversal → Step 4: Context Assembly
     ↓              ↓                        ↓                      ↓                        ↓
Query Text    Vector + Entities      Anchor Nodes           Enriched Nodes         Rich Context + Results
     ↓              ↓                        ↓                      ↓                        ↓
LLM Call      Embedding API          Vector DB + Graph DB    Graph Traversal        Reranking + Assembly
```

**Detailed Search Flow Architecture**:
```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           Search Pipeline Architecture                           │
├─────────────────────────────────────────────────────────────────────────────────┤
│ Step 1: Input Processing                                                        │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐                  │
│  │ Query Text      │ │ Vector          │ │ Entity          │                  │
│  │ Analysis        │ │ Generation      │ │ Extraction      │                  │
│  │ • Keywords      │ │ • Embedding API │ │ • LLM Call      │                  │
│  │ • Language      │ │ • 768/1536 dim  │ │ • JSON Schema   │                  │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│ Step 2: Hybrid Search (Parallel)                                               │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐                  │
│  │ Vector Search   │ │ Entity Search   │ │ Anchor Node     │                  │
│  │ • Similarity    │ │ • Graph Query   │ │ • Combination   │                  │
│  │ • Top-K Results │ │ • Node Matching │ │ • Deduplication │                  │
│  │ • Score Ranking │ │ • Type Filtering│ │ • Score Merge   │                  │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│ Step 3: Graph Traversal                                                         │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐                  │
│  │ 1-Hop Neighbors │ │ 2-Hop Neighbors │ │ Enriched Nodes  │                  │
│  │ • Direct Edges  │ │ • Indirect Edges│ │ • Full Context  │                  │
│  │ • Relationships │ │ • Extended Graph│ │ • Neighborhood  │                  │
│  │ • Edge Weights  │ │ • Concept Links │ │ • Relevance     │                  │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│ Step 4: Context Assembly & Reranking                                           │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐                  │
│  │ Multi-Factor    │ │ Context Builder │ │ Final Results   │                  │
│  │ Reranking       │ │ • Token Mgmt    │ │ • Rich Context  │                  │
│  │ • Vector (40%)  │ │ • Relationship  │ │ • Ranked List   │                  │
│  │ • Graph (30%)   │ │ • Summary       │ │ • Metadata      │                  │
│  │ • Temporal(20%) │ │ • Optimization  │ │ • Performance   │                  │
│  │ • User (10%)    │ │ • LLM Ready     │ │ • Metrics       │                  │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Components and Interfaces

### Core Data Structures

```go
// DataPoint represents the fundamental memory unit
type DataPoint struct {
    ID          string                 `json:"id"`
    Content     string                 `json:"content"`
    ContentType string                 `json:"content_type"`
    Metadata    map[string]interface{} `json:"metadata"`
    Embedding   []float32              `json:"embedding,omitempty"`
    SessionID   string                 `json:"session_id"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
    
    // Graph relationships
    Relationships []Relationship `json:"relationships,omitempty"`
}

// Relationship defines connections between DataPoints
type Relationship struct {
    Type     string  `json:"type"`
    Target   string  `json:"target"`
    Weight   float64 `json:"weight"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MemorySession manages isolated memory contexts
type MemorySession struct {
    ID          string            `json:"id"`
    UserID      string            `json:"user_id,omitempty"`
    Context     map[string]interface{} `json:"context"`
    CreatedAt   time.Time         `json:"created_at"`
    LastAccess  time.Time         `json:"last_access"`
    IsActive    bool              `json:"is_active"`
}
```

### Memory Engine Interface

```go
// MemoryEngine defines the core memory operations
type MemoryEngine interface {
    // Core pipeline operations
    Add(ctx context.Context, content string, metadata map[string]interface{}) (*DataPoint, error)
    Cognify(ctx context.Context, dataPoint *DataPoint) (*DataPoint, error)
    Memify(ctx context.Context, dataPoint *DataPoint) error
    Search(ctx context.Context, query *SearchQuery) (*SearchResults, error)
    
    // Session management
    CreateSession(ctx context.Context, userID string, context map[string]interface{}) (*MemorySession, error)
    GetSession(ctx context.Context, sessionID string) (*MemorySession, error)
    CloseSession(ctx context.Context, sessionID string) error
    
    // Health and lifecycle
    Health(ctx context.Context) error
    Close() error
}

// SearchQuery defines search parameters and modes
type SearchQuery struct {
    Text            string                 `json:"text"`
    SessionID       string                 `json:"session_id"`
    Mode            RetrievalMode          `json:"mode"`
    Limit           int                    `json:"limit"`
    SimilarityThreshold float64           `json:"similarity_threshold"`
    Filters         map[string]interface{} `json:"filters,omitempty"`
    TimeRange       *TimeRange             `json:"time_range,omitempty"`
}

// RetrievalMode defines different search strategies
type RetrievalMode string

const (
    ModeSemanticSearch   RetrievalMode = "semantic_search"
    ModeGraphTraversal   RetrievalMode = "graph_traversal"
    ModeHybridSearch     RetrievalMode = "hybrid_search"
    ModeTemporalSearch   RetrievalMode = "temporal_search"
    ModeContextualRAG    RetrievalMode = "contextual_rag"
)
```

### Storage Layer Interfaces

```go
// GraphStore handles relationship and graph operations
type GraphStore interface {
    StoreNode(ctx context.Context, dataPoint *DataPoint) error
    CreateRelationship(ctx context.Context, from, to string, relType string, weight float64) error
    TraverseGraph(ctx context.Context, startNode string, depth int, filters map[string]interface{}) ([]*DataPoint, error)
    FindConnected(ctx context.Context, nodeID string, relTypes []string) ([]*DataPoint, error)
    DeleteNode(ctx context.Context, nodeID string) error
}

// VectorStore handles embedding and similarity operations
type VectorStore interface {
    StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error
    SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*SimilarityResult, error)
    UpdateEmbedding(ctx context.Context, id string, embedding []float32) error
    DeleteEmbedding(ctx context.Context, id string) error
}

// RelationalStore handles structured data and metadata
type RelationalStore interface {
    StoreDataPoint(ctx context.Context, dataPoint *DataPoint) error
    GetDataPoint(ctx context.Context, id string) (*DataPoint, error)
    QueryDataPoints(ctx context.Context, filters map[string]interface{}, limit int) ([]*DataPoint, error)
    UpdateDataPoint(ctx context.Context, dataPoint *DataPoint) error
    DeleteDataPoint(ctx context.Context, id string) error
    
    // Session operations
    StoreSession(ctx context.Context, session *MemorySession) error
    GetSession(ctx context.Context, sessionID string) (*MemorySession, error)
    UpdateSession(ctx context.Context, session *MemorySession) error
}
```

## Data Models

### Memory Storage Schema

The system uses a three-layer storage approach, each optimized for specific access patterns:

#### Graph Store Schema
```
Nodes: DataPoint entities with properties
Edges: Relationships with types and weights
Indexes: ID, SessionID, ContentType, CreatedAt
```

#### Vector Store Schema  
```
Vectors: High-dimensional embeddings (typically 768 or 1536 dimensions)
Metadata: ID, SessionID, ContentType for filtering
Indexes: HNSW or IVF for fast similarity search
```

#### Relational Store Schema
```sql
-- DataPoints table
CREATE TABLE datapoints (
    id VARCHAR(255) PRIMARY KEY,
    content TEXT NOT NULL,
    content_type VARCHAR(100),
    metadata JSONB,
    session_id VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Sessions table  
CREATE TABLE sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255),
    context JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    last_access TIMESTAMP DEFAULT NOW(),
    is_active BOOLEAN DEFAULT true
);

-- Relationships table (backup for graph store)
CREATE TABLE relationships (
    id SERIAL PRIMARY KEY,
    from_node VARCHAR(255) NOT NULL,
    to_node VARCHAR(255) NOT NULL,
    rel_type VARCHAR(100) NOT NULL,
    weight FLOAT DEFAULT 1.0,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### Memory Processing Pipeline

The data transformation follows this flow:

1. **Raw Input** → DataPoint creation with basic metadata
2. **Cognify Stage** → Content analysis, entity extraction, relationship detection
3. **Embedding Generation** → Vector representation using AutoEmbedder
4. **Memify Stage** → Parallel storage across all three layers
5. **Indexing** → Update search indexes for fast retrieval

### AutoEmbedder System

Following Protocol-Lattice's approach, the AutoEmbedder provides pluggable embedding generation:

```go
// EmbeddingProvider defines the interface for embedding generation
type EmbeddingProvider interface {
    GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
    GetDimensions() int
    GetModel() string
}

// AutoEmbedder manages multiple embedding providers
type AutoEmbedder struct {
    providers map[string]EmbeddingProvider
    default   string
}

// Supported providers
type OpenAIEmbedder struct {
    apiKey string
    model  string // text-embedding-ada-002, text-embedding-3-small, etc.
}

type LocalEmbedder struct {
    modelPath string // For local models like sentence-transformers
}

type OllamaEmbedder struct {
    endpoint string
    model    string
}
```

## Implementation Details

### Memory Engine Implementation

```go
// memoryEngine implements the MemoryEngine interface
type memoryEngine struct {
    graphStore      GraphStore
    vectorStore     VectorStore
    relationalStore RelationalStore
    embedder        *AutoEmbedder
    config          *Config
    
    // Caching and performance
    cache           *cache.Cache
    sessionCache    *cache.Cache
    
    // Concurrency control
    mu              sync.RWMutex
    processingQueue chan *ProcessingTask
    workers         int
}

// ProcessingTask represents a memory operation task
type ProcessingTask struct {
    Type      TaskType
    DataPoint *DataPoint
    Query     *SearchQuery
    Result    chan TaskResult
}

// Core pipeline implementation
func (m *memoryEngine) Add(ctx context.Context, content string, metadata map[string]interface{}) (*DataPoint, error) {
    // Create DataPoint with unique ID
    dataPoint := &DataPoint{
        ID:          generateID(),
        Content:     content,
        ContentType: detectContentType(content),
        Metadata:    metadata,
        SessionID:   getSessionFromContext(ctx),
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    
    // Store in relational store immediately for durability
    if err := m.relationalStore.StoreDataPoint(ctx, dataPoint); err != nil {
        return nil, fmt.Errorf("failed to store datapoint: %w", err)
    }
    
    return dataPoint, nil
}

func (m *memoryEngine) Cognify(ctx context.Context, dataPoint *DataPoint) (*DataPoint, error) {
    // Content analysis and enrichment
    enriched := dataPoint.Clone()
    
    // Extract entities and concepts
    entities, err := m.extractEntities(ctx, dataPoint.Content)
    if err != nil {
        return nil, fmt.Errorf("entity extraction failed: %w", err)
    }
    
    // Detect relationships with existing memories
    relationships, err := m.detectRelationships(ctx, dataPoint)
    if err != nil {
        return nil, fmt.Errorf("relationship detection failed: %w", err)
    }
    
    // Generate embedding
    embedding, err := m.embedder.GenerateEmbedding(ctx, dataPoint.Content)
    if err != nil {
        return nil, fmt.Errorf("embedding generation failed: %w", err)
    }
    
    // Update DataPoint with enriched data
    enriched.Metadata["entities"] = entities
    enriched.Relationships = relationships
    enriched.Embedding = embedding
    enriched.UpdatedAt = time.Now()
    
    return enriched, nil
}

func (m *memoryEngine) Memify(ctx context.Context, dataPoint *DataPoint) error {
    // Parallel storage across all three layers
    var wg sync.WaitGroup
    errors := make(chan error, 3)
    
    // Store in graph database
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := m.graphStore.StoreNode(ctx, dataPoint); err != nil {
            errors <- fmt.Errorf("graph store failed: %w", err)
            return
        }
        
        // Create relationships
        for _, rel := range dataPoint.Relationships {
            if err := m.graphStore.CreateRelationship(ctx, dataPoint.ID, rel.Target, rel.Type, rel.Weight); err != nil {
                errors <- fmt.Errorf("relationship creation failed: %w", err)
                return
            }
        }
    }()
    
    // Store in vector database
    wg.Add(1)
    go func() {
        defer wg.Done()
        if len(dataPoint.Embedding) > 0 {
            metadata := map[string]interface{}{
                "session_id":   dataPoint.SessionID,
                "content_type": dataPoint.ContentType,
                "created_at":   dataPoint.CreatedAt,
            }
            if err := m.vectorStore.StoreEmbedding(ctx, dataPoint.ID, dataPoint.Embedding, metadata); err != nil {
                errors <- fmt.Errorf("vector store failed: %w", err)
            }
        }
    }()
    
    // Update relational store
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := m.relationalStore.UpdateDataPoint(ctx, dataPoint); err != nil {
            errors <- fmt.Errorf("relational store update failed: %w", err)
        }
    }()
    
    // Wait for all operations to complete
    go func() {
        wg.Wait()
        close(errors)
    }()
    
    // Check for errors
    for err := range errors {
        if err != nil {
            return err
        }
    }
    
    // Update cache
    m.cache.Set(dataPoint.ID, dataPoint, cache.DefaultExpiration)
    
    return nil
}
```

### Search Implementation with 4-Step Cognee Process

The Search operation implements Cognee's proven 4-step GraphRAG approach, providing superior context understanding compared to traditional RAG:

#### Step 1: Input Processing & Vectorization
```go
func (m *memoryEngine) processSearchInput(ctx context.Context, query *SearchQuery) (*ProcessedQuery, error) {
    // Generate query vector
    queryVector, err := m.embedder.GenerateEmbedding(ctx, query.Text)
    if err != nil {
        return nil, fmt.Errorf("query vectorization failed: %w", err)
    }
    
    // Extract entities/keywords from query
    entities, err := m.extractor.ExtractEntities(ctx, query.Text)
    if err != nil {
        return nil, fmt.Errorf("entity extraction failed: %w", err)
    }
    
    return &ProcessedQuery{
        OriginalText: query.Text,
        Vector:       queryVector,
        Entities:     entities,
        Keywords:     extractKeywords(query.Text),
    }, nil
}
```

#### Step 2: Hybrid Search (Vector + Graph Initial Nodes)
```go
func (m *memoryEngine) findAnchorNodes(ctx context.Context, processedQuery *ProcessedQuery) ([]*AnchorNode, error) {
    var wg sync.WaitGroup
    vectorResults := make(chan []*SimilarityResult, 1)
    entityResults := make(chan []*Node, 1)
    
    // Vector similarity search
    wg.Add(1)
    go func() {
        defer wg.Done()
        results, err := m.vectorStore.SimilaritySearch(ctx, processedQuery.Vector, 20, 0.7)
        if err == nil {
            vectorResults <- results
        }
    }()
    
    // Entity-based graph search
    wg.Add(1)
    go func() {
        defer wg.Done()
        var allNodes []*Node
        for _, entity := range processedQuery.Entities {
            nodes, err := m.graphStore.FindNodesByEntity(ctx, entity.Name, entity.Type)
            if err == nil {
                allNodes = append(allNodes, nodes...)
            }
        }
        entityResults <- allNodes
    }()
    
    wg.Wait()
    close(vectorResults)
    close(entityResults)
    
    // Combine and deduplicate anchor nodes
    return m.combineAnchorNodes(<-vectorResults, <-entityResults), nil
}
```

#### Step 3: Graph Traversal (Neighborhood Search)
```go
func (m *memoryEngine) expandGraphNeighborhood(ctx context.Context, anchorNodes []*AnchorNode) ([]*EnrichedNode, error) {
    enrichedNodes := make([]*EnrichedNode, 0)
    
    for _, anchor := range anchorNodes {
        // 1-hop traversal: Direct relationships
        directNeighbors, err := m.graphStore.FindConnected(ctx, anchor.ID, []EdgeType{
            EdgeTypeRelatedTo,
            EdgeTypeSynonym,
            EdgeTypeStrugglesWIth,
        })
        if err != nil {
            continue
        }
        
        // 2-hop traversal: Indirect relationships
        var indirectNeighbors []*Node
        for _, neighbor := range directNeighbors {
            indirect, err := m.graphStore.FindConnected(ctx, neighbor.ID, []EdgeType{
                EdgeTypeRelatedTo,
            })
            if err == nil {
                indirectNeighbors = append(indirectNeighbors, indirect...)
            }
        }
        
        // Create enriched node with neighborhood context
        enriched := &EnrichedNode{
            Core:              anchor.Node,
            DirectNeighbors:   directNeighbors,
            IndirectNeighbors: indirectNeighbors,
            RelevanceScore:    anchor.Score,
        }
        
        enrichedNodes = append(enrichedNodes, enriched)
    }
    
    return enrichedNodes, nil
}
```

#### Step 4: Context Assembly & Reranking
```go
func (m *memoryEngine) assembleSearchContext(ctx context.Context, enrichedNodes []*EnrichedNode, query *SearchQuery) (*SearchResults, error) {
    // Rerank nodes based on multiple factors
    rankedNodes := m.rerankNodes(enrichedNodes, query)
    
    // Assemble rich context for LLM
    contextBuilder := &ContextBuilder{
        MaxTokens: 4000, // Configurable context window
    }
    
    searchResults := make([]*SearchResult, 0)
    for _, node := range rankedNodes {
        if contextBuilder.CanAddNode(node) {
            result := &SearchResult{
                DataPoint:     node.Core.ToDataPoint(),
                Score:         node.RelevanceScore,
                Mode:          query.Mode,
                Relationships: m.buildRelationshipContext(node),
                Neighborhood:  m.buildNeighborhoodSummary(node),
            }
            
            searchResults = append(searchResults, result)
            contextBuilder.AddNode(node)
        }
    }
    
    return &SearchResults{
        Results:     searchResults,
        Total:       len(searchResults),
        QueryTime:   time.Since(time.Now()),
        ContextSize: contextBuilder.GetTokenCount(),
    }, nil
}

// Advanced reranking algorithm
func (m *memoryEngine) rerankNodes(nodes []*EnrichedNode, query *SearchQuery) []*EnrichedNode {
    for _, node := range nodes {
        score := 0.0
        
        // Vector similarity weight (40%)
        score += node.RelevanceScore * 0.4
        
        // Graph centrality weight (30%)
        centrality := m.calculateNodeCentrality(node)
        score += centrality * 0.3
        
        // Temporal relevance weight (20%)
        temporal := m.calculateTemporalRelevance(node, query)
        score += temporal * 0.2
        
        // User context weight (10%)
        userContext := m.calculateUserContextRelevance(node, query.SessionID)
        score += userContext * 0.1
        
        node.FinalScore = score
    }
    
    // Sort by final score
    sort.Slice(nodes, func(i, j int) bool {
        return nodes[i].FinalScore > nodes[j].FinalScore
    })
    
    return nodes
}
```

### Search Mode Comparison: Traditional RAG vs GraphRAG

| Aspect | Traditional RAG | Cognee GraphRAG |
|--------|----------------|------------------|
| **Search Method** | Top-K similar text chunks only | Top-K + Related entities in graph |
| **Context** | Disconnected, fragmented across documents | Connected through relationships (Edges) |
| **Accuracy** | Prone to hallucination with similar keywords but different meanings | Understands which entity belongs to which concept |
| **Relationship Understanding** | No relationship awareness | Explicit entity relationships and dependencies |
| **Context Window Usage** | Often wastes tokens on irrelevant similar text | Optimized context with relevant connected knowledge |
| **Query Understanding** | Keyword/semantic matching only | Entity extraction + relationship traversal |

### Example Search Flow (English Learning Use Case)

**Query**: "Cách dùng thì Hiện tại hoàn thành mà tôi đã học là gì?"

**Step 1 - Input Processing**:
```go
ProcessedQuery{
    OriginalText: "Cách dùng thì Hiện tại hoàn thành mà tôi đã học là gì?",
    Vector: [0.1, 0.3, -0.2, ...], // 768-dim embedding
    Entities: [
        {Name: "Present Perfect", Type: "Grammar"},
        {Name: "User", Type: "Person"},
    ],
    Keywords: ["cách dùng", "hiện tại hoàn thành", "đã học"],
}
```

**Step 2 - Anchor Nodes**:
```go
AnchorNodes: [
    {ID: "node_present_perfect", Score: 0.95, Type: "Grammar"},
    {ID: "node_user_progress", Score: 0.87, Type: "UserData"},
    {ID: "node_grammar_rules", Score: 0.82, Type: "Concept"},
]
```

**Step 3 - Graph Traversal**:
```go
EnrichedNodes: [
    {
        Core: "Present Perfect",
        DirectNeighbors: [
            "Have/Has + Past Participle", // Formula
            "Since/For indicators",       // Usage signals  
            "User struggled with this",   // Learning history
        ],
        IndirectNeighbors: [
            "Past Simple vs Present Perfect", // Related concepts
            "Common mistakes",                // Error patterns
        ],
    },
]
```

**Step 4 - Final Context**:
```
Rich Context for LLM:
- Present Perfect grammar rule (core concept)
- Formula: Have/Has + Past Participle  
- Usage indicators: Since, For, Already, Yet
- User's previous struggles with this topic
- Common mistakes to avoid
- Related grammar concepts for comparison
```

This approach provides significantly richer and more accurate context compared to traditional RAG, leading to better AI responses that understand both the concept and the user's learning history.
```

### Backend Adapter Implementations

```go
// Neo4j Graph Store Implementation
type neo4jGraphStore struct {
    driver neo4j.Driver
    config *Neo4jConfig
}

func (n *neo4jGraphStore) StoreNode(ctx context.Context, dataPoint *DataPoint) error {
    session := n.driver.NewSession(neo4j.SessionConfig{})
    defer session.Close()
    
    query := `
        CREATE (n:DataPoint {
            id: $id,
            content: $content,
            content_type: $content_type,
            session_id: $session_id,
            created_at: $created_at,
            metadata: $metadata
        })
    `
    
    _, err := session.WriteTransaction(func(tx neo4j.Transaction) (interface{}, error) {
        return tx.Run(query, map[string]interface{}{
            "id":           dataPoint.ID,
            "content":      dataPoint.Content,
            "content_type": dataPoint.ContentType,
            "session_id":   dataPoint.SessionID,
            "created_at":   dataPoint.CreatedAt,
            "metadata":     dataPoint.Metadata,
        })
    })
    
    return err
}

// Qdrant Vector Store Implementation
type qdrantVectorStore struct {
    client *qdrant.Client
    config *QdrantConfig
}

func (q *qdrantVectorStore) StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error {
    point := &qdrant.PointStruct{
        Id:      qdrant.NewIDNum(hash(id)),
        Vectors: qdrant.NewVectors(embedding...),
        Payload: qdrant.NewValueMap(metadata),
    }
    
    _, err := q.client.Upsert(ctx, &qdrant.UpsertPoints{
        CollectionName: q.config.CollectionName,
        Points:         []*qdrant.PointStruct{point},
    })
    
    return err
}

// PostgreSQL Relational Store Implementation
type postgresRelationalStore struct {
    db     *sql.DB
    config *PostgresConfig
}

func (p *postgresRelationalStore) StoreDataPoint(ctx context.Context, dataPoint *DataPoint) error {
    query := `
        INSERT INTO datapoints (id, content, content_type, metadata, session_id, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (id) DO UPDATE SET
            content = EXCLUDED.content,
            metadata = EXCLUDED.metadata,
            updated_at = EXCLUDED.updated_at
    `
    
    metadataJSON, _ := json.Marshal(dataPoint.Metadata)
    
    _, err := p.db.ExecContext(ctx, query,
        dataPoint.ID,
        dataPoint.Content,
        dataPoint.ContentType,
        metadataJSON,
        dataPoint.SessionID,
        dataPoint.CreatedAt,
        dataPoint.UpdatedAt,
    )
    
    return err
}
```

Now I need to use the prework tool to analyze the acceptance criteria before writing the Correctness Properties section:

<function_calls>
<invoke name="prework">
<parameter name="featureName">ai-memory-integration
## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

After analyzing the acceptance criteria, I've identified several redundancies that need to be consolidated:

**Property Reflection:**
- Properties 1.1 and 2.1 both test memory storage - can be combined into comprehensive storage property
- Properties 1.3 and 2.5 both test graph integrity - can be merged into single integrity property  
- Properties 6.1, 6.2, 6.3 all test embedding functionality - can be consolidated into comprehensive embedding property
- Properties 7.1, 7.2, 7.3 all test session isolation - can be combined into single session property
- Properties 8.1, 8.2, 8.3 all test error resilience - can be merged into comprehensive resilience property

### Property 1: Memory Storage and Retrieval

*For any* user interaction with valid content, storing it through the Add→Cognify→Memify pipeline should result in a retrievable DataPoint with proper node structure, relationships, and persistence across all storage layers.

**Validates: Requirements 1.1, 1.3, 2.1**

### Property 2: Similarity-Based Memory Retrieval

*For any* stored memory and similarity search query, the returned results should have similarity scores above the specified threshold and be ranked in descending order of relevance.

**Validates: Requirements 1.2, 6.2, 6.3**

### Property 3: Session Continuity and Context Loading

*For any* memory session that is closed and later reopened, the system should load relevant historical context from previous sessions while maintaining proper session isolation.

**Validates: Requirements 1.4, 7.1, 7.2, 7.3**

### Property 4: Graph Relationship Integrity

*For any* pair of related memories, creating a relationship should result in bidirectional connectivity that maintains referential integrity and supports traversal within 3 degrees of separation.

**Validates: Requirements 1.5, 2.2, 2.3, 2.4, 2.5**

### Property 5: Backend Interface Consistency

*For any* supported backend configuration (native Go, sidecar, external API), switching between backends should maintain identical API behavior and interface contracts.

**Validates: Requirements 4.4, 4.5**

### Property 6: Concurrent Session Safety

*For any* set of concurrent memory operations across multiple sessions, the system should maintain data integrity and proper isolation without race conditions.

**Validates: Requirements 5.3**

### Property 7: Embedding Generation and Updates

*For any* memory content, the system should generate appropriate embeddings during storage and update them when content is modified, with fallback to text-based search when embedding generation fails.

**Validates: Requirements 6.1, 6.4, 6.5**

### Property 8: Session Metadata Persistence

*For any* memory session that ends, the session metadata should be persisted for future reference while supporting cross-session memory sharing through explicit relationships.

**Validates: Requirements 7.4, 7.5**

### Property 9: Error Resilience and Recovery

*For any* storage or retrieval failure, the system should operate in degraded mode using cache, queue operations for retry with exponential backoff, and automatically synchronize when connectivity is restored.

**Validates: Requirements 8.1, 8.2, 8.3, 8.5**

### Property 10: Comprehensive Error Logging

*For any* error condition in the memory system, appropriate log entries should be created with sufficient detail for debugging and troubleshooting.

**Validates: Requirements 8.4**

### Property 11: Graceful Shutdown and Persistence

*For any* application shutdown scenario, all pending memory operations should be gracefully persisted before the system terminates.

**Validates: Requirements 3.4**

### Property 12: Offline Operation Queuing

*For any* offline scenario, memory operations should be queued locally and synchronized automatically when connectivity is restored.

**Validates: Requirements 3.5**

### Property 13: Multi-Tenant Encryption

*For any* user in a multi-tenant scenario, memories should be encrypted with user-specific keys using AES-256 encryption, ensuring data isolation and security.

**Validates: Requirements 10.1, 10.3**

### Property 14: Secure Memory Deletion

*For any* memory deletion operation, the memory should be securely removed from all storage locations (graph, vector, and relational stores) with no recoverable traces.

**Validates: Requirements 10.4**

### Property 15: Authentication and Authorization

*For any* memory access request, proper authentication and authorization should be enforced to ensure only authorized users can access their respective memories.

**Validates: Requirements 10.5**

## Error Handling

The AI Memory Integration library implements comprehensive error handling across all layers:

### Error Categories

1. **Storage Errors**: Database connectivity, write failures, consistency violations
2. **Embedding Errors**: Model unavailability, API failures, dimension mismatches  
3. **Session Errors**: Invalid session IDs, session conflicts, timeout issues
4. **Network Errors**: Connection timeouts, TLS failures, service unavailability
5. **Authentication Errors**: Invalid credentials, expired tokens, authorization failures

### Error Handling Strategies

#### Graceful Degradation
```go
type DegradedMode struct {
    CacheOnly     bool
    ReadOnly      bool
    LocalStorage  bool
    QueuedWrites  []Operation
}

func (m *memoryEngine) handleStorageFailure(ctx context.Context, err error) error {
    // Switch to degraded mode
    m.degradedMode = &DegradedMode{
        CacheOnly:    true,
        ReadOnly:     false,
        LocalStorage: true,
    }
    
    // Log the failure
    m.logger.Error("Storage failure, switching to degraded mode", 
        zap.Error(err),
        zap.String("mode", "cache_only"))
    
    return nil // Continue operation in degraded mode
}
```

#### Retry Logic with Exponential Backoff
```go
type RetryConfig struct {
    MaxRetries    int
    BaseDelay     time.Duration
    MaxDelay      time.Duration
    Multiplier    float64
}

func (m *memoryEngine) retryWithBackoff(ctx context.Context, operation func() error, config RetryConfig) error {
    var lastErr error
    delay := config.BaseDelay
    
    for i := 0; i < config.MaxRetries; i++ {
        if err := operation(); err == nil {
            return nil
        } else {
            lastErr = err
        }
        
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(delay):
            delay = time.Duration(float64(delay) * config.Multiplier)
            if delay > config.MaxDelay {
                delay = config.MaxDelay
            }
        }
    }
    
    return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}
```

#### Circuit Breaker Pattern
```go
type CircuitBreaker struct {
    maxFailures   int
    resetTimeout  time.Duration
    state         CircuitState
    failures      int
    lastFailTime  time.Time
    mu            sync.RWMutex
}

func (cb *CircuitBreaker) Execute(operation func() error) error {
    cb.mu.RLock()
    state := cb.state
    cb.mu.RUnlock()
    
    if state == StateOpen {
        if time.Since(cb.lastFailTime) > cb.resetTimeout {
            cb.setState(StateHalfOpen)
        } else {
            return ErrCircuitBreakerOpen
        }
    }
    
    err := operation()
    cb.recordResult(err)
    return err
}
```

## Testing Strategy

The AI Memory Integration library employs a dual testing approach combining unit tests and property-based tests for comprehensive coverage.

### Property-Based Testing Configuration

**Testing Library**: We'll use [gopter](https://github.com/leanovate/gopter) for property-based testing in Go, configured with:
- Minimum 100 iterations per property test
- Custom generators for DataPoints, Sessions, and Queries
- Shrinking enabled for minimal failing examples

**Test Configuration Example**:
```go
func TestMemoryStorageProperty(t *testing.T) {
    parameters := gopter.DefaultTestParameters()
    parameters.MinSuccessfulTests = 100
    parameters.MaxSize = 50
    
    properties := gopter.NewProperties(parameters)
    
    properties.Property("Memory storage and retrieval roundtrip", 
        // Feature: ai-memory-integration, Property 1: Memory Storage and Retrieval
        prop.ForAll(
            func(content string, metadata map[string]interface{}) bool {
                // Test implementation
                return verifyStorageRoundtrip(content, metadata)
            },
            genValidContent(),
            genMetadata(),
        ))
    
    properties.TestingRun(t)
}
```

### Unit Testing Strategy

**Unit Test Focus Areas**:
- Specific examples demonstrating correct behavior
- Edge cases and boundary conditions  
- Error conditions and failure scenarios
- Integration points between components
- Configuration and deployment scenarios

**Example Unit Tests**:
```go
func TestWailsIntegration(t *testing.T) {
    // Feature: ai-memory-integration, Example: Wails API compatibility
    ctx := context.Background()
    engine := setupTestEngine(t)
    
    // Test Go-native bindings
    assert.True(t, isWailsCompatible(engine))
    
    // Test context exposure
    wailsCtx := setupWailsContext(engine)
    assert.NotNil(t, wailsCtx.Add)
    assert.NotNil(t, wailsCtx.Search)
}

func TestConfigurationLoading(t *testing.T) {
    // Feature: ai-memory-integration, Example: Configuration support
    tests := []struct {
        name   string
        config Config
        valid  bool
    }{
        {"env_vars", loadFromEnv(), true},
        {"config_file", loadFromFile("test.yaml"), true},
        {"invalid_config", Config{}, false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            engine, err := NewMemoryEngine(tt.config)
            if tt.valid {
                assert.NoError(t, err)
                assert.NotNil(t, engine)
            } else {
                assert.Error(t, err)
            }
        })
    }
}
```

### Test Coverage Requirements

**Property Tests**: Each correctness property must be implemented by exactly one property-based test with the tag format:
- **Feature: ai-memory-integration, Property {number}: {property_text}**

**Unit Tests**: Focus on concrete examples, edge cases, and integration scenarios:
- API compatibility tests for Wails integration
- Configuration loading and validation tests  
- Deployment mode verification tests
- Health check endpoint tests
- Security configuration tests

**Integration Tests**: End-to-end scenarios testing the complete pipeline:
- Full Add→Cognify→Memify→Search workflows
- Multi-backend deployment scenarios
- Failure recovery and resilience testing
- Performance benchmarks for critical paths

The combination of property-based tests (verifying universal correctness) and unit tests (catching specific bugs and edge cases) provides comprehensive coverage ensuring both general correctness and concrete reliability.