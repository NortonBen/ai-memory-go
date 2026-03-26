// Package openai - OpenAI provider tests
package openai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAIProvider(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		model   string
		wantErr bool
	}{
		{
			name:    "valid provider with default model",
			apiKey:  "test-api-key",
			model:   "",
			wantErr: false,
		},
		{
			name:    "valid provider with custom model",
			apiKey:  "test-api-key",
			model:   "gpt-4",
			wantErr: false,
		},
		{
			name:    "missing API key",
			apiKey:  "",
			model:   "gpt-4",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIProvider(tt.apiKey, tt.model)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				if tt.model != "" {
					assert.Equal(t, tt.model, provider.GetModel())
				}
			}
		})
	}
}

func TestOpenAIProvider_GetCapabilities(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	caps := provider.GetCapabilities()

	assert.True(t, caps.SupportsCompletion)
	assert.True(t, caps.SupportsChat)
	assert.True(t, caps.SupportsStreaming)
	assert.True(t, caps.SupportsJSONMode)
	assert.True(t, caps.SupportsJSONSchema)
	assert.True(t, caps.SupportsFunctionCalling)
	assert.True(t, caps.SupportsSystemPrompts)
	assert.True(t, caps.SupportsConversation)
	assert.Greater(t, caps.MaxContextLength, 0)
	assert.NotEmpty(t, caps.AvailableModels)
}

func TestOpenAIProvider_GetModel(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	assert.Equal(t, "gpt-4", provider.GetModel())
}

func TestOpenAIProvider_SetModel(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	err = provider.SetModel("gpt-3.5-turbo")
	assert.NoError(t, err)
	assert.Equal(t, "gpt-3.5-turbo", provider.GetModel())
}

func TestOpenAIProvider_GetProviderType(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	assert.Equal(t, extractor.ProviderOpenAI, provider.GetProviderType())
}

func TestOpenAIProvider_GetTokenCount(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	text := "This is a test text for token counting"
	count, err := provider.GetTokenCount(text)
	assert.NoError(t, err)
	assert.Greater(t, count, 0)
}

func TestOpenAIProvider_GetMaxTokens(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		{
			name:     "GPT-4 Turbo",
			model:    "gpt-4-turbo-preview",
			expected: 128000,
		},
		{
			name:     "GPT-4",
			model:    "gpt-4",
			expected: 8192,
		},
		{
			name:     "GPT-3.5 Turbo",
			model:    "gpt-3.5-turbo",
			expected: 16385,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIProvider("test-api-key", tt.model)
			require.NoError(t, err)

			maxTokens := provider.GetMaxTokens()
			assert.Equal(t, tt.expected, maxTokens)
		})
	}
}

func TestOpenAIProvider_Configure(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	newConfig := extractor.DefaultProviderConfig(extractor.ProviderOpenAI)
	newConfig.APIKey = "new-api-key"
	newConfig.Model = "gpt-3.5-turbo"

	err = provider.Configure(newConfig)
	assert.NoError(t, err)
	assert.Equal(t, "gpt-3.5-turbo", provider.GetModel())
}

func TestOpenAIProvider_GetConfiguration(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	config := provider.GetConfiguration()
	assert.NotNil(t, config)
	assert.Equal(t, extractor.ProviderOpenAI, config.Type)
	assert.Equal(t, "gpt-4", config.Model)
}

func TestOpenAIProvider_Close(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	err = provider.Close()
	assert.NoError(t, err)
}

func TestOpenAIProvider_GetUsage(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	ctx := context.Background()
	usage, err := provider.GetUsage(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, usage)
	assert.GreaterOrEqual(t, usage.TotalRequests, int64(0))
}

func TestOpenAIProvider_GetRateLimit(t *testing.T) {
	provider, err := NewOpenAIProvider("test-api-key", "gpt-4")
	require.NoError(t, err)

	ctx := context.Background()
	rateLimit, err := provider.GetRateLimit(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, rateLimit)
	assert.Greater(t, rateLimit.RequestsPerMinute, 0)
}

func TestNewSystemMessage(t *testing.T) {
	msg := extractor.NewSystemMessage("You are a helpful assistant")
	assert.Equal(t, extractor.RoleSystem, msg.Role)
	assert.Equal(t, "You are a helpful assistant", msg.Content)
}

func TestNewUserMessage(t *testing.T) {
	msg := extractor.NewUserMessage("Hello, how are you?")
	assert.Equal(t, extractor.RoleUser, msg.Role)
	assert.Equal(t, "Hello, how are you?", msg.Content)
}

func TestNewAssistantMessage(t *testing.T) {
	msg := extractor.NewAssistantMessage("I'm doing well, thank you!")
	assert.Equal(t, extractor.RoleAssistant, msg.Role)
	assert.Equal(t, "I'm doing well, thank you!", msg.Content)
}

// Integration tests (require valid API key)

func TestOpenAIProvider_Integration_GenerateCompletion(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-3.5-turbo")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := provider.GenerateCompletion(ctx, "Say 'Hello, World!' and nothing else.")
	assert.NoError(t, err)
	assert.NotEmpty(t, response)
	t.Logf("Response: %s", response)
}

func TestOpenAIProvider_Integration_GenerateStructuredOutput(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-3.5-turbo")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var result struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	_, err = provider.GenerateStructuredOutput(ctx, "Create a JSON object with name 'John' and age 30", &result)
	assert.NoError(t, err)
	assert.Equal(t, "John", result.Name)
	assert.Equal(t, 30, result.Age)
}

func TestOpenAIProvider_Integration_ExtractEntities(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-3.5-turbo")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text := "John works at Google in California."
	entities, err := provider.ExtractEntities(ctx, text)
	assert.NoError(t, err)
	assert.NotEmpty(t, entities)
	t.Logf("Extracted %d entities", len(entities))
}

func TestOpenAIProvider_Integration_GenerateWithContext(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-3.5-turbo")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages := []extractor.Message{
		extractor.NewSystemMessage("You are a helpful assistant."),
		extractor.NewUserMessage("What is 2+2?"),
	}

	response, err := provider.GenerateWithContext(ctx, messages, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, response)
	t.Logf("Response: %s", response)
}

func TestOpenAIProvider_Integration_Health(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-3.5-turbo")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = provider.Health(ctx)
	assert.NoError(t, err)
}

func BenchmarkOpenAIProvider_GenerateCompletion(b *testing.B) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		b.Skip("OPENAI_API_KEY not set, skipping benchmark")
	}

	provider, err := NewOpenAIProvider(apiKey, "gpt-3.5-turbo")
	require.NoError(b, err)

	ctx := context.Background()
	prompt := "Say hello"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := provider.GenerateCompletion(ctx, prompt)
		if err != nil {
			b.Fatal(err)
		}
	}
}
