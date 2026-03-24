// Package vector - Embedding provider dependencies
package vector

import (
	_ "github.com/sashabaranov/go-openai" // OpenAI Go SDK for embeddings
)

// This file ensures embedding provider dependencies are tracked in go.mod
//
// Provider Dependencies:
// - OpenAI: Uses official Go SDK (github.com/sashabaranov/go-openai)
// - Ollama: Uses HTTP client (no external SDK required, connects to local Ollama API)
// - DeepSeek: Uses HTTP client (follows OpenAI API format, no external SDK required)
//
// Note: Ollama's official SDK requires Go 1.24+, so we use direct HTTP calls
// to maintain compatibility with Go 1.23 as specified in requirements.
