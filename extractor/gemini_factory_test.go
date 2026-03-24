package extractor

import (
	"testing"
)

// TestGeminiProviderFactoryCreation tests that Gemini providers can be created through the factory
func TestGeminiProviderFactoryCreation(t *testing.T) {
	t.Run("LLMProvider", func(t *testing.T) {
		factory := NewProviderFactory()

		// Test creating Gemini LLM provider through factory
		config := &ProviderConfig{
			Type:   ProviderGemini,
			APIKey: "test-api-key",
			Model:  "gemini-1.5-flash",
		}

		provider, err := factory.CreateProvider(config)
		if err != nil {
			t.Fatalf("Failed to create Gemini provider through factory: %v", err)
		}

		if provider == nil {
			t.Fatal("Factory returned nil Gemini provider")
		}

		if provider.GetProviderType() != ProviderGemini {
			t.Errorf("Expected provider type %v, got %v", ProviderGemini, provider.GetProviderType())
		}

		if provider.GetModel() != "gemini-1.5-flash" {
			t.Errorf("Expected model gemini-1.5-flash, got %s", provider.GetModel())
		}

		// Test capabilities
		caps := provider.GetCapabilities()
		if !caps.SupportsCompletion {
			t.Error("Expected Gemini to support completion")
		}
		if !caps.SupportsJSONSchema {
			t.Error("Expected Gemini to support JSON schema")
		}
		if caps.MaxContextLength != 1000000 {
			t.Errorf("Expected max context length 1000000, got %d", caps.MaxContextLength)
		}
	})

	t.Run("EmbeddingProvider", func(t *testing.T) {
		factory := NewEmbeddingProviderFactory()

		// Test creating Gemini embedding provider through factory
		config := &EmbeddingProviderConfig{
			Type:       EmbeddingProviderGemini,
			APIKey:     "test-api-key",
			Model:      "text-embedding-004",
			Dimensions: 768,
		}

		provider, err := factory.CreateProvider(config)
		if err != nil {
			t.Fatalf("Failed to create Gemini embedding provider through factory: %v", err)
		}

		if provider == nil {
			t.Fatal("Factory returned nil Gemini embedding provider")
		}

		if provider.GetProviderType() != EmbeddingProviderGemini {
			t.Errorf("Expected provider type %v, got %v", EmbeddingProviderGemini, provider.GetProviderType())
		}

		if provider.GetModel() != "text-embedding-004" {
			t.Errorf("Expected model text-embedding-004, got %s", provider.GetModel())
		}

		if provider.GetDimensions() != 768 {
			t.Errorf("Expected dimensions 768, got %d", provider.GetDimensions())
		}

		// Test capabilities
		caps := provider.GetCapabilities()
		if !caps.SupportsBatching {
			t.Error("Expected Gemini embedding to support batching")
		}
		if !caps.SupportsCustomDims {
			t.Error("Expected Gemini embedding to support custom dimensions")
		}
		if caps.MaxTokensPerText != 2048 {
			t.Errorf("Expected max tokens per text 2048, got %d", caps.MaxTokensPerText)
		}
	})
}

// TestGeminiProviderFactoryDefaults tests creating Gemini providers with defaults
func TestGeminiProviderFactoryDefaults(t *testing.T) {
	t.Run("LLMProvider", func(t *testing.T) {
		factory := NewProviderFactory()

		provider, err := factory.CreateProviderWithDefaults(ProviderGemini, "test-api-key", "")
		if err != nil {
			t.Fatalf("Failed to create Gemini provider with defaults: %v", err)
		}

		// Should default to gemini-1.5-flash
		if provider.GetModel() != "gemini-1.5-flash" {
			t.Errorf("Expected default model gemini-1.5-flash, got %s", provider.GetModel())
		}
	})

	t.Run("EmbeddingProvider", func(t *testing.T) {
		factory := NewEmbeddingProviderFactory()

		provider, err := factory.CreateProviderWithDefaults(EmbeddingProviderGemini, "test-api-key", "")
		if err != nil {
			t.Fatalf("Failed to create Gemini embedding provider with defaults: %v", err)
		}

		// Should default to text-embedding-004
		if provider.GetModel() != "text-embedding-004" {
			t.Errorf("Expected default model text-embedding-004, got %s", provider.GetModel())
		}

		// Should default to 768 dimensions
		if provider.GetDimensions() != 768 {
			t.Errorf("Expected default dimensions 768, got %d", provider.GetDimensions())
		}
	})
}

// TestGeminiProviderFactoryCapabilities tests getting capabilities through factory
func TestGeminiProviderFactoryCapabilities(t *testing.T) {
	t.Run("LLMProvider", func(t *testing.T) {
		factory := NewProviderFactory()

		caps, err := factory.GetProviderCapabilities(ProviderGemini)
		if err != nil {
			t.Fatalf("Failed to get Gemini capabilities: %v", err)
		}

		if caps == nil {
			t.Fatal("Gemini capabilities are nil")
		}

		if !caps.SupportsCompletion {
			t.Error("Expected Gemini to support completion")
		}
		if !caps.SupportsJSONSchema {
			t.Error("Expected Gemini to support JSON schema")
		}
		if caps.MaxContextLength != 1000000 {
			t.Errorf("Expected max context length 1000000, got %d", caps.MaxContextLength)
		}
	})

	t.Run("EmbeddingProvider", func(t *testing.T) {
		factory := NewEmbeddingProviderFactory()

		caps, err := factory.GetProviderCapabilities(EmbeddingProviderGemini)
		if err != nil {
			t.Fatalf("Failed to get Gemini embedding capabilities: %v", err)
		}

		if caps == nil {
			t.Fatal("Gemini embedding capabilities are nil")
		}

		if !caps.SupportsBatching {
			t.Error("Expected Gemini embedding to support batching")
		}
		if !caps.SupportsCustomDims {
			t.Error("Expected Gemini embedding to support custom dimensions")
		}
		if caps.DefaultDimension != 768 {
			t.Errorf("Expected default dimension 768, got %d", caps.DefaultDimension)
		}
	})
}

// TestGeminiProviderFactorySupported tests that Gemini is in supported providers list
func TestGeminiProviderFactorySupported(t *testing.T) {
	t.Run("LLMProvider", func(t *testing.T) {
		factory := NewProviderFactory()

		supportedProviders := factory.ListSupportedProviders()
		found := false
		for _, provider := range supportedProviders {
			if provider == ProviderGemini {
				found = true
				break
			}
		}
		if !found {
			t.Error("Gemini provider not found in supported providers list")
		}
	})

	t.Run("EmbeddingProvider", func(t *testing.T) {
		factory := NewEmbeddingProviderFactory()

		supportedProviders := factory.ListSupportedProviders()
		found := false
		for _, provider := range supportedProviders {
			if provider == EmbeddingProviderGemini {
				found = true
				break
			}
		}
		if !found {
			t.Error("Gemini embedding provider not found in supported providers list")
		}
	})
}
