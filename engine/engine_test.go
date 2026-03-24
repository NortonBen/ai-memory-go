package engine

import (
	"testing"
)

func TestNewMemoryEngine(t *testing.T) {
	cfg := EngineConfig{MaxWorkers: 2}
	
	// Fast test to ensure initialization and closure without panic.
	// For actual unit tests, robust mock implementations for 
	// Extractor, EmbeddingProvider, and Storage are needed.
	engine := NewMemoryEngine(nil, nil, nil, cfg)
	if engine == nil {
		t.Fatal("Expected NewMemoryEngine to return an instance")
	}
	
	if engine.workerPool == nil {
		t.Fatal("Expected workerPool to be initialized")
	}
	
	engine.Close()
}
