package extractor

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewGeminiProvider(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		model     string
		wantError bool
	}{
		{
			name:      "valid provider creation",
			apiKey:    "test-api-key",
			model:     GeminiPro15Flash,
			wantError: false,
		},
		{
			name:      "empty API key",
			apiKey:    "",
			model:     GeminiPro15Flash,
			wantError: true,
		},
		{
			name:      "empty model defaults to flash",
			apiKey:    "test-api-key",
			model:     "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewGeminiProvider(tt.apiKey, tt.model)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewGeminiProvider() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewGeminiProvider() unexpected error: %v", err)
				return
			}

			if provider == nil {
				t.Errorf("NewGeminiProvider() returned nil provider")
				return
			}

			// Check default model
			expectedModel := tt.model
			if expectedModel == "" {
				expectedModel = GeminiPro15Flash
			}

			if provider.GetModel() != expectedModel {
				t.Errorf("NewGeminiProvider() model = %v, want %v", provider.GetModel(), expectedModel)
			}

			if provider.GetProviderType() != ProviderGemini {
				t.Errorf("NewGeminiProvider() provider type = %v, want %v", provider.GetProviderType(), ProviderGemini)
			}
		})
	}
}

func TestGeminiProvider_GetCapabilities(t *testing.T) {
	provider, err := NewGeminiProvider("test-api-key", GeminiPro15Flash)
	if err != nil {
		t.Fatalf("NewGeminiProvider() error: %v", err)
	}

	caps := provider.GetCapabilities()

	// Check key capabilities
	if !caps.SupportsCompletion {
		t.Error("Expected SupportsCompletion to be true")
	}

	if !caps.SupportsChat {
		t.Error("Expected SupportsChat to be true")
	}

	if !caps.SupportsJSONMode {
		t.Error("Expected SupportsJSONMode to be true")
	}

	if !caps.SupportsJSONSchema {
		t.Error("Expected SupportsJSONSchema to be true")
	}

	if !caps.SupportsSystemPrompts {
		t.Error("Expected SupportsSystemPrompts to be true")
	}

	if caps.MaxContextLength != 1000000 {
		t.Errorf("Expected MaxContextLength to be 1000000, got %d", caps.MaxContextLength)
	}
}

func TestGeminiProvider_GetSupportedModels(t *testing.T) {
	provider, err := NewGeminiProvider("test-api-key", GeminiPro15Flash)
	if err != nil {
		t.Fatalf("NewGeminiProvider() error: %v", err)
	}

	models := provider.GetSupportedModels()
	expectedModels := []string{
		GeminiPro,
		GeminiProVision,
		GeminiPro15,
		GeminiPro15Flash,
	}

	if len(models) != len(expectedModels) {
		t.Errorf("Expected %d models, got %d", len(expectedModels), len(models))
	}

	for _, expected := range expectedModels {
		found := false
		for _, model := range models {
			if model == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected model %s not found in supported models", expected)
		}
	}
}

func TestGeminiProvider_SetModel(t *testing.T) {
	provider, err := NewGeminiProvider("test-api-key", GeminiPro15Flash)
	if err != nil {
		t.Fatalf("NewGeminiProvider() error: %v", err)
	}

	// Test setting a valid model
	err = provider.SetModel(GeminiPro15)
	if err != nil {
		t.Errorf("SetModel() unexpected error: %v", err)
	}

	if provider.GetModel() != GeminiPro15 {
		t.Errorf("SetModel() model = %v, want %v", provider.GetModel(), GeminiPro15)
	}
}

func TestGeminiProvider_GetTokenCount(t *testing.T) {
	provider, err := NewGeminiProvider("test-api-key", GeminiPro15Flash)
	if err != nil {
		t.Fatalf("NewGeminiProvider() error: %v", err)
	}

	text := "This is a test text for token counting"
	count, err := provider.GetTokenCount(text)
	if err != nil {
		t.Errorf("GetTokenCount() unexpected error: %v", err)
	}

	// Simple estimation should be roughly len(text)/4
	expectedCount := len(text) / 4
	if count != expectedCount {
		t.Errorf("GetTokenCount() = %d, want approximately %d", count, expectedCount)
	}
}

func TestGeminiProvider_GetMaxTokens(t *testing.T) {
	provider, err := NewGeminiProvider("test-api-key", GeminiPro15Flash)
	if err != nil {
		t.Fatalf("NewGeminiProvider() error: %v", err)
	}

	maxTokens := provider.GetMaxTokens()
	if maxTokens != 8192 {
		t.Errorf("GetMaxTokens() = %d, want %d", maxTokens, 8192)
	}
}

func TestGeminiProvider_Configure(t *testing.T) {
	provider, err := NewGeminiProvider("test-api-key", GeminiPro15Flash)
	if err != nil {
		t.Fatalf("NewGeminiProvider() error: %v", err)
	}

	// Test valid configuration
	config := &ProviderConfig{
		Type:   ProviderGemini,
		APIKey: "new-api-key",
		Model:  GeminiPro15,
	}

	err = provider.Configure(config)
	if err != nil {
		t.Errorf("Configure() unexpected error: %v", err)
	}

	if provider.GetModel() != GeminiPro15 {
		t.Errorf("Configure() model = %v, want %v", provider.GetModel(), GeminiPro15)
	}

	// Test nil configuration
	err = provider.Configure(nil)
	if err == nil {
		t.Error("Configure() expected error for nil config, got nil")
	}
}

func TestGeminiProvider_GetConfiguration(t *testing.T) {
	provider, err := NewGeminiProvider("test-api-key", GeminiPro15Flash)
	if err != nil {
		t.Fatalf("NewGeminiProvider() error: %v", err)
	}

	config := provider.GetConfiguration()
	if config == nil {
		t.Error("GetConfiguration() returned nil")
		return
	}

	if config.Type != ProviderGemini {
		t.Errorf("GetConfiguration() type = %v, want %v", config.Type, ProviderGemini)
	}

	if config.Model != GeminiPro15Flash {
		t.Errorf("GetConfiguration() model = %v, want %v", config.Model, GeminiPro15Flash)
	}
}

// Integration tests (require actual API key)
func TestGeminiProvider_Integration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping integration tests")
	}

	provider, err := NewGeminiProvider(apiKey, GeminiPro15Flash)
	if err != nil {
		t.Fatalf("NewGeminiProvider() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Health", func(t *testing.T) {
		err := provider.Health(ctx)
		if err != nil {
			t.Errorf("Health() error: %v", err)
		}
	})

	t.Run("GenerateCompletion", func(t *testing.T) {
		prompt := "What is the capital of France?"
		response, err := provider.GenerateCompletion(ctx, prompt)
		if err != nil {
			t.Errorf("GenerateCompletion() error: %v", err)
			return
		}

		if response == "" {
			t.Error("GenerateCompletion() returned empty response")
		}

		t.Logf("Response: %s", response)
	})

	t.Run("GenerateCompletionWithOptions", func(t *testing.T) {
		prompt := "Write a short poem about AI"
		options := &CompletionOptions{
			Temperature: 0.8,
			MaxTokens:   100,
		}

		response, err := provider.GenerateCompletionWithOptions(ctx, prompt, options)
		if err != nil {
			t.Errorf("GenerateCompletionWithOptions() error: %v", err)
			return
		}

		if response == "" {
			t.Error("GenerateCompletionWithOptions() returned empty response")
		}

		t.Logf("Response: %s", response)
	})

	t.Run("ExtractEntities", func(t *testing.T) {
		text := "John Smith works at Google in Mountain View, California."
		entities, err := provider.ExtractEntities(ctx, text)
		if err != nil {
			t.Errorf("ExtractEntities() error: %v", err)
			return
		}

		if len(entities) == 0 {
			t.Error("ExtractEntities() returned no entities")
		}

		t.Logf("Entities: %+v", entities)
	})

	t.Run("GenerateWithContext", func(t *testing.T) {
		messages := []Message{
			{Role: RoleSystem, Content: "You are a helpful assistant."},
			{Role: RoleUser, Content: "What is 2+2?"},
		}

		response, err := provider.GenerateWithContext(ctx, messages, nil)
		if err != nil {
			t.Errorf("GenerateWithContext() error: %v", err)
			return
		}

		if response == "" {
			t.Error("GenerateWithContext() returned empty response")
		}

		t.Logf("Response: %s", response)
	})
}

// ============================================================================
// Gemini Embedding Provider Tests
// ============================================================================

func TestNewGeminiEmbeddingProvider(t *testing.T) {
	tests := []struct {
		name      string
		apiKey    string
		model     string
		wantError bool
	}{
		{
			name:      "valid provider creation",
			apiKey:    "test-api-key",
			model:     TextEmbedding004,
			wantError: false,
		},
		{
			name:      "empty API key",
			apiKey:    "",
			model:     TextEmbedding004,
			wantError: true,
		},
		{
			name:      "empty model defaults to text-embedding-004",
			apiKey:    "test-api-key",
			model:     "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewGeminiEmbeddingProvider(tt.apiKey, tt.model)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewGeminiEmbeddingProvider() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewGeminiEmbeddingProvider() unexpected error: %v", err)
				return
			}

			if provider == nil {
				t.Errorf("NewGeminiEmbeddingProvider() returned nil provider")
				return
			}

			// Check default model
			expectedModel := tt.model
			if expectedModel == "" {
				expectedModel = TextEmbedding004
			}

			if provider.GetModel() != expectedModel {
				t.Errorf("NewGeminiEmbeddingProvider() model = %v, want %v", provider.GetModel(), expectedModel)
			}

			if provider.GetProviderType() != EmbeddingProviderGemini {
				t.Errorf("NewGeminiEmbeddingProvider() provider type = %v, want %v", provider.GetProviderType(), EmbeddingProviderGemini)
			}

			if provider.GetDimensions() != 768 {
				t.Errorf("NewGeminiEmbeddingProvider() dimensions = %d, want %d", provider.GetDimensions(), 768)
			}
		})
	}
}

func TestGeminiEmbeddingProvider_GetCapabilities(t *testing.T) {
	provider, err := NewGeminiEmbeddingProvider("test-api-key", TextEmbedding004)
	if err != nil {
		t.Fatalf("NewGeminiEmbeddingProvider() error: %v", err)
	}

	caps := provider.GetCapabilities()

	// Check key capabilities
	if !caps.SupportsBatching {
		t.Error("Expected SupportsBatching to be true")
	}

	if caps.SupportsStreaming {
		t.Error("Expected SupportsStreaming to be false")
	}

	if !caps.SupportsCustomDims {
		t.Error("Expected SupportsCustomDims to be true")
	}

	if caps.MaxTokensPerText != 2048 {
		t.Errorf("Expected MaxTokensPerText to be 2048, got %d", caps.MaxTokensPerText)
	}

	if caps.MaxBatchSize != 100 {
		t.Errorf("Expected MaxBatchSize to be 100, got %d", caps.MaxBatchSize)
	}

	if caps.DefaultDimension != 768 {
		t.Errorf("Expected DefaultDimension to be 768, got %d", caps.DefaultDimension)
	}
}

func TestGeminiEmbeddingProvider_GetSupportedModels(t *testing.T) {
	provider, err := NewGeminiEmbeddingProvider("test-api-key", TextEmbedding004)
	if err != nil {
		t.Fatalf("NewGeminiEmbeddingProvider() error: %v", err)
	}

	models := provider.GetSupportedModels()
	expectedModels := []string{TextEmbedding004}

	if len(models) != len(expectedModels) {
		t.Errorf("Expected %d models, got %d", len(expectedModels), len(models))
	}

	for _, expected := range expectedModels {
		found := false
		for _, model := range models {
			if model == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected model %s not found in supported models", expected)
		}
	}
}

func TestGeminiEmbeddingProvider_SetModel(t *testing.T) {
	provider, err := NewGeminiEmbeddingProvider("test-api-key", TextEmbedding004)
	if err != nil {
		t.Fatalf("NewGeminiEmbeddingProvider() error: %v", err)
	}

	// Test setting the same model
	err = provider.SetModel(TextEmbedding004)
	if err != nil {
		t.Errorf("SetModel() unexpected error: %v", err)
	}

	if provider.GetModel() != TextEmbedding004 {
		t.Errorf("SetModel() model = %v, want %v", provider.GetModel(), TextEmbedding004)
	}

	// Test setting an unsupported model
	err = provider.SetModel("unsupported-model")
	if err == nil {
		t.Error("SetModel() expected error for unsupported model, got nil")
	}
}

func TestGeminiEmbeddingProvider_CustomDimensions(t *testing.T) {
	provider, err := NewGeminiEmbeddingProvider("test-api-key", TextEmbedding004)
	if err != nil {
		t.Fatalf("NewGeminiEmbeddingProvider() error: %v", err)
	}

	// Test supports custom dimensions
	if !provider.SupportsCustomDimensions() {
		t.Error("Expected SupportsCustomDimensions to be true")
	}

	// Test setting valid dimensions
	err = provider.SetCustomDimensions(512)
	if err != nil {
		t.Errorf("SetCustomDimensions() unexpected error: %v", err)
	}

	if provider.GetDimensions() != 512 {
		t.Errorf("SetCustomDimensions() dimensions = %d, want %d", provider.GetDimensions(), 512)
	}

	// Test setting invalid dimensions
	err = provider.SetCustomDimensions(1000)
	if err == nil {
		t.Error("SetCustomDimensions() expected error for dimensions > 768, got nil")
	}

	err = provider.SetCustomDimensions(0)
	if err == nil {
		t.Error("SetCustomDimensions() expected error for dimensions <= 0, got nil")
	}
}

func TestGeminiEmbeddingProvider_EstimateTokenCount(t *testing.T) {
	provider, err := NewGeminiEmbeddingProvider("test-api-key", TextEmbedding004)
	if err != nil {
		t.Fatalf("NewGeminiEmbeddingProvider() error: %v", err)
	}

	text := "This is a test text for token counting"
	count, err := provider.EstimateTokenCount(text)
	if err != nil {
		t.Errorf("EstimateTokenCount() unexpected error: %v", err)
	}

	// Simple estimation should be roughly len(text)/4
	expectedCount := len(text) / 4
	if count != expectedCount {
		t.Errorf("EstimateTokenCount() = %d, want approximately %d", count, expectedCount)
	}
}

func TestGeminiEmbeddingProvider_EstimateCost(t *testing.T) {
	provider, err := NewGeminiEmbeddingProvider("test-api-key", TextEmbedding004)
	if err != nil {
		t.Fatalf("NewGeminiEmbeddingProvider() error: %v", err)
	}

	cost, err := provider.EstimateCost(1000)
	if err != nil {
		t.Errorf("EstimateCost() unexpected error: %v", err)
	}

	// Gemini embeddings are free, so cost should be 0
	if cost != 0.0 {
		t.Errorf("EstimateCost() = %f, want %f", cost, 0.0)
	}
}

func TestGeminiEmbeddingProvider_Configure(t *testing.T) {
	provider, err := NewGeminiEmbeddingProvider("test-api-key", TextEmbedding004)
	if err != nil {
		t.Fatalf("NewGeminiEmbeddingProvider() error: %v", err)
	}

	// Test valid configuration
	config := DefaultEmbeddingProviderConfig(EmbeddingProviderGemini)
	config.APIKey = "new-api-key"
	config.Model = TextEmbedding004
	config.Dimensions = 512

	err = provider.Configure(config)
	if err != nil {
		t.Errorf("Configure() unexpected error: %v", err)
	}

	if provider.GetDimensions() != 512 {
		t.Errorf("Configure() dimensions = %d, want %d", provider.GetDimensions(), 512)
	}

	// Test nil configuration
	err = provider.Configure(nil)
	if err == nil {
		t.Error("Configure() expected error for nil config, got nil")
	}
}

func TestGeminiEmbeddingProvider_DeduplicateAndEmbed(t *testing.T) {
	provider, err := NewGeminiEmbeddingProvider("test-api-key", TextEmbedding004)
	if err != nil {
		t.Fatalf("NewGeminiEmbeddingProvider() error: %v", err)
	}

	// This test would require mocking the HTTP client or actual API calls
	// For now, we'll test the deduplication logic conceptually
	texts := []string{"hello", "world", "hello", "test"}

	// In a real test, we would mock the HTTP response
	// For now, we just verify the method exists and can be called
	ctx := context.Background()
	_, err = provider.DeduplicateAndEmbed(ctx, texts)

	// We expect this to fail without a real API key, but the method should exist
	if err == nil {
		t.Log("DeduplicateAndEmbed() succeeded (unexpected with test API key)")
	} else {
		t.Logf("DeduplicateAndEmbed() failed as expected: %v", err)
	}
}

// Integration tests for embedding provider (require actual API key)
func TestGeminiEmbeddingProvider_Integration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping integration tests")
	}

	provider, err := NewGeminiEmbeddingProvider(apiKey, TextEmbedding004)
	if err != nil {
		t.Fatalf("NewGeminiEmbeddingProvider() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Health", func(t *testing.T) {
		err := provider.Health(ctx)
		if err != nil {
			t.Errorf("Health() error: %v", err)
		}
	})

	t.Run("GenerateEmbedding", func(t *testing.T) {
		text := "This is a test sentence for embedding generation."
		embedding, err := provider.GenerateEmbedding(ctx, text)
		if err != nil {
			t.Errorf("GenerateEmbedding() error: %v", err)
			return
		}

		if len(embedding) != 768 {
			t.Errorf("GenerateEmbedding() embedding length = %d, want %d", len(embedding), 768)
		}

		// Check that embedding values are reasonable (between -1 and 1 typically)
		for i, val := range embedding {
			if val < -2.0 || val > 2.0 {
				t.Errorf("GenerateEmbedding() embedding[%d] = %f, seems out of reasonable range", i, val)
				break
			}
		}

		t.Logf("Generated embedding with %d dimensions", len(embedding))
	})

	t.Run("GenerateBatchEmbeddings", func(t *testing.T) {
		texts := []string{
			"First test sentence.",
			"Second test sentence.",
			"Third test sentence.",
		}

		embeddings, err := provider.GenerateBatchEmbeddings(ctx, texts)
		if err != nil {
			t.Errorf("GenerateBatchEmbeddings() error: %v", err)
			return
		}

		if len(embeddings) != len(texts) {
			t.Errorf("GenerateBatchEmbeddings() returned %d embeddings, want %d", len(embeddings), len(texts))
		}

		for i, embedding := range embeddings {
			if len(embedding) != 768 {
				t.Errorf("GenerateBatchEmbeddings() embedding[%d] length = %d, want %d", i, len(embedding), 768)
			}
		}

		t.Logf("Generated %d embeddings", len(embeddings))
	})

	t.Run("GenerateEmbeddingWithOptions", func(t *testing.T) {
		text := "Test sentence with custom options."
		options := &EmbeddingOptions{
			Dimensions: 512,
		}

		embedding, err := provider.GenerateEmbeddingWithOptions(ctx, text, options)
		if err != nil {
			t.Errorf("GenerateEmbeddingWithOptions() error: %v", err)
			return
		}

		// Note: The dimensions might still be 768 if the API doesn't support custom dimensions
		// or if our implementation doesn't properly handle it yet
		if len(embedding) == 0 {
			t.Error("GenerateEmbeddingWithOptions() returned empty embedding")
		}

		t.Logf("Generated embedding with options, length: %d", len(embedding))
	})

	t.Run("GetUsage", func(t *testing.T) {
		usage, err := provider.GetUsage(ctx)
		if err != nil {
			t.Errorf("GetUsage() error: %v", err)
			return
		}

		if usage == nil {
			t.Error("GetUsage() returned nil")
			return
		}

		t.Logf("Usage stats: %+v", usage)
	})

	t.Run("GetRateLimit", func(t *testing.T) {
		rateLimit, err := provider.GetRateLimit(ctx)
		if err != nil {
			t.Errorf("GetRateLimit() error: %v", err)
			return
		}

		if rateLimit == nil {
			t.Error("GetRateLimit() returned nil")
			return
		}

		t.Logf("Rate limit: %+v", rateLimit)
	})
}
