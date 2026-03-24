package engine

import (
	"context"

	"github.com/NortonBen/ai-memory-go/schema"
)

// EngineConfig holds configuration for MemoryEngine
type EngineConfig struct {
	MaxWorkers int
}

// AddOptions holds optional parameters for the Add operation
type AddOptions struct {
	SessionID            string
	Metadata             map[string]interface{}
	WaitUntilComplete    bool
	ConsistencyThreshold float32
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

// MemoryEngine defines the core memory operations
type MemoryEngine interface {
	// Core pipeline operations
	Add(ctx context.Context, content string, opts ...AddOption) (*schema.DataPoint, error)
	Cognify(ctx context.Context, dataPoint *schema.DataPoint, opts ...CognifyOption) (*schema.DataPoint, error)
	CognifyPending(ctx context.Context, sessionID string) error
	Memify(ctx context.Context, dataPoint *schema.DataPoint, opts ...MemifyOption) error
	Search(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error)
	Think(ctx context.Context, query *schema.ThinkQuery) (*schema.ThinkResult, error)

	// Memory operations
	DeleteMemory(ctx context.Context, id string, sessionID string) error

	// Health and lifecycle
	Health(ctx context.Context) error
	Close() error
}
