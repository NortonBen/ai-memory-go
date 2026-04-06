package engine

import (
	"context"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

// EngineConfig holds configuration for MemoryEngine
type EngineConfig struct {
	MaxWorkers               int           `json:"max_workers"`
	EnableBackgroundAnalysis bool          `json:"enable_background_analysis"`
	AnalysisInterval         time.Duration `json:"analysis_interval"`
	// ChunkConcurrency is the max number of chunks processed in parallel inside a single CognifyTask.
	// Defaults to 4 when unset or <= 0.
	ChunkConcurrency int `json:"chunk_concurrency"`
	// FourTier bật pipeline tìm kiếm bốn tầng (core / general / data / storage theo request).
	FourTier schema.FourTierEngineConfig `json:"four_tier"`
}

// AddOptions holds optional parameters for the Add operation
type AddOptions struct {
	SessionID            string
	Metadata             map[string]interface{}
	WaitUntilComplete    bool
	ConsistencyThreshold float32
	// MemoryTier phân vùng 4 tầng (core / general / data / storage). Ghi vào metadata memory_tier;
	// ưu tiên hơn memory_tier trong WithMetadata nếu cả hai được đặt.
	MemoryTier string
	// Labels phân loại nội dung khi lưu (rule, policy, tên truyện…). Ghi memory_labels + primary_label; rule/policy → tier core nếu không chỉ định tier. Không dùng để lọc Search.
	Labels []string
}

// AddOption configures AddOptions
type AddOption func(*AddOptions)

// WithMetadata adds custom metadata to the DataPoint
func WithMetadata(metadata map[string]interface{}) AddOption {
	return func(o *AddOptions) {
		if o.Metadata == nil {
			o.Metadata = make(map[string]interface{})
		}
		for k, v := range metadata {
			o.Metadata[k] = v
		}
	}
}

// WithWaitAdd configures whether Add should wait for background extraction to complete.
func WithWaitAdd(wait bool) AddOption {
	return func(o *AddOptions) {
		o.WaitUntilComplete = wait
	}
}

// WithConsistencyThreshold sets the vector distance threshold for invoking consistency reasoning (e.g. 0.1)
func WithConsistencyThreshold(threshold float32) AddOption {
	return func(o *AddOptions) {
		o.ConsistencyThreshold = threshold
	}
}

// WithSessionID explicitly maps a session identifier onto the DataPoint
func WithSessionID(sessionID string) AddOption {
	return func(o *AddOptions) {
		o.SessionID = sessionID
	}
}

// WithMemoryTier chọn phân vùng lưu trữ cho bản ghi (schema.MemoryTier*). Chuỗi không hợp lệ được chuẩn hóa về general.
func WithMemoryTier(tier string) AddOption {
	return func(o *AddOptions) {
		o.MemoryTier = tier
	}
}

// WithLabels gắn nhãn phân loại khi Add. Có thể kết hợp WithMemoryTier.
func WithLabels(labels ...string) AddOption {
	return func(o *AddOptions) {
		o.Labels = append(o.Labels, labels...)
	}
}

// CognifyOptions holds optional parameters for the Cognify operation
type CognifyOptions struct {
	WaitUntilComplete bool
}

// CognifyOption configures CognifyOptions
type CognifyOption func(*CognifyOptions)

// WithWaitCognify causes the process to wait for completion before returning.
func WithWaitCognify(wait bool) CognifyOption {
	return func(o *CognifyOptions) {
		o.WaitUntilComplete = wait
	}
}

// MemifyOptions holds optional parameters for the Memify operation
type MemifyOptions struct {
	WaitUntilComplete bool
}

// MemifyOption configures MemifyOptions
type MemifyOption func(*MemifyOptions)

// WithWaitMemify allows Memify to wait for any background completions (though currently synchronous).
func WithWaitMemify(wait bool) MemifyOption {
	return func(o *MemifyOptions) {
		o.WaitUntilComplete = wait
	}
}

// RequestOptions configures the Request operation
type RequestOptions struct {
	Metadata           map[string]interface{}
	HopDepth           int
	EnableThinking     bool
	MaxThinkingSteps   int
	LearnRelationships bool
	IncludeReasoning bool
	// FourTier ghi đè tìm kiếm 4 tầng cho bước Think (nil = theo cấu hình engine).
	FourTier *schema.FourTierSearchOptions
}

// RequestOption configures RequestOptions
type RequestOption func(*RequestOptions)

// WithRequestMetadata adds custom metadata to the Request
func WithRequestMetadata(metadata map[string]interface{}) RequestOption {
	return func(o *RequestOptions) {
		if o.Metadata == nil {
			o.Metadata = make(map[string]interface{})
		}
		for k, v := range metadata {
			o.Metadata[k] = v
		}
	}
}

// WithHopDepth sets the number of neighbor hops to traverse from anchors
func WithHopDepth(depth int) RequestOption {
	return func(o *RequestOptions) {
		o.HopDepth = depth
	}
}

// WithEnableThinking toggles the agentic/iterative thinking loop
func WithEnableThinking(enable bool) RequestOption {
	return func(o *RequestOptions) {
		o.EnableThinking = enable
	}
}

// WithMaxThinkingSteps sets the maximum iterations for missing entities
func WithMaxThinkingSteps(steps int) RequestOption {
	return func(o *RequestOptions) {
		o.MaxThinkingSteps = steps
	}
}

// WithLearnRelationships toggles automatic creation of bridging relationships
func WithLearnRelationships(learn bool) RequestOption {
	return func(o *RequestOptions) {
		o.LearnRelationships = learn
	}
}

// WithIncludeReasoning toggles inclusion of the thought process in the result
func WithIncludeReasoning(include bool) RequestOption {
	return func(o *RequestOptions) {
		o.IncludeReasoning = include
	}
}

// WithRequestFourTier ghi đè bốn tầng cho bước Think trong Request.
func WithRequestFourTier(ft *schema.FourTierSearchOptions) RequestOption {
	return func(o *RequestOptions) {
		o.FourTier = ft
	}
}

// MemoryEngine defines the core memory operations
type MemoryEngine interface {
	// Core pipeline operations
	Add(ctx context.Context, content string, opts ...AddOption) (*schema.DataPoint, error)
	Cognify(ctx context.Context, dataPoint *schema.DataPoint, opts ...CognifyOption) (*schema.DataPoint, error)
	CognifyPending(ctx context.Context, sessionID string) error
	Memify(ctx context.Context, dataPoint *schema.DataPoint, opts ...MemifyOption) error
	Search(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error)
	Think(ctx context.Context, query *schema.ThinkQuery) (*schema.ThinkResult, error)
	// AnalyzeHistory processes recent chat history to extract deeper relationships or update existing ones
	AnalyzeHistory(ctx context.Context, sessionID string) error
	Request(ctx context.Context, sessionID string, content string, opts ...RequestOption) (*schema.ThinkResult, error)

	// Memory operations
	DeleteMemory(ctx context.Context, id string, sessionID string) error

	// Health and lifecycle
	Health(ctx context.Context) error
	Close() error
}
