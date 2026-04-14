package extractor

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultProviderConfig_Branches(t *testing.T) {
	cases := []ProviderType{
		ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderOllama,
		ProviderDeepSeek, ProviderMistral, ProviderBedrock,
	}
	for _, pt := range cases {
		cfg := DefaultProviderConfig(pt)
		require.Equal(t, pt, cfg.Type)
		require.NotEmpty(t, cfg.Model)
		require.True(t, cfg.HealthCheck.Enabled)
		require.Greater(t, cfg.RateLimit.RequestsPerMinute, 0)
	}
}

func TestValidateProviderConfig_BranchesAndNormalization(t *testing.T) {
	ollama := &ProviderConfig{Type: ProviderOllama, Model: "llama2"}
	require.Error(t, ValidateProviderConfig(ollama))
	ollama.Endpoint = "http://localhost:11434"
	require.NoError(t, ValidateProviderConfig(ollama))

	bedrock := &ProviderConfig{Type: ProviderBedrock, Model: "m"}
	require.Error(t, ValidateProviderConfig(bedrock))
	bedrock.Region = "us-east-1"
	require.NoError(t, ValidateProviderConfig(bedrock))

	openai := &ProviderConfig{
		Type: ProviderOpenAI, Model: "gpt-4", APIKey: "k",
		Timeout: 0,
	}
	openai.RateLimit.RetryAttempts = -5
	require.NoError(t, ValidateProviderConfig(openai))
	require.Equal(t, 120*time.Second, openai.Timeout)
	require.Equal(t, 0, openai.RateLimit.RetryAttempts)

	openai.RateLimit.RetryAttempts = 999
	require.NoError(t, ValidateProviderConfig(openai))
	require.Equal(t, 10, openai.RateLimit.RetryAttempts)
}

func TestDefaultEmbeddingConfig_ExtraProviders(t *testing.T) {
	for _, pt := range []EmbeddingProviderType{
		EmbeddingProviderGemini, EmbeddingProviderLMStudio, EmbeddingProviderOpenRouter, EmbeddingProviderONNX,
	} {
		cfg := DefaultEmbeddingProviderConfig(pt)
		require.Equal(t, pt, cfg.Type)
		require.NotEmpty(t, cfg.Model)
		require.Greater(t, cfg.Dimensions, 0)
		require.True(t, cfg.HealthCheck.Enabled)
	}
}

func TestValidateEmbeddingProviderConfig_ExtraBranches(t *testing.T) {
	local := &EmbeddingProviderConfig{
		Type: EmbeddingProviderLocal, Model: "m", Dimensions: 8,
		CustomOptions: map[string]interface{}{"model_path": ""},
	}
	require.Error(t, ValidateEmbeddingProviderConfig(local))

	onnx := &EmbeddingProviderConfig{
		Type: EmbeddingProviderONNX, Model: "m", Dimensions: 640,
		CustomOptions: map[string]interface{}{"model_path": ""},
	}
	require.Error(t, ValidateEmbeddingProviderConfig(onnx))

	valid := &EmbeddingProviderConfig{
		Type: EmbeddingProviderOllama, Model: "m", Endpoint: "http://localhost:11434",
		Dimensions: 8, MaxBatchSize: 0, Timeout: 0,
	}
	valid.RateLimit.RetryAttempts = -1
	require.NoError(t, ValidateEmbeddingProviderConfig(valid))
	require.Equal(t, 1, valid.MaxBatchSize)
	require.Equal(t, 60*time.Second, valid.Timeout)
	require.Equal(t, 0, valid.RateLimit.RetryAttempts)

	valid.RateLimit.RetryAttempts = 999
	require.NoError(t, ValidateEmbeddingProviderConfig(valid))
	require.Equal(t, 10, valid.RateLimit.RetryAttempts)
}

func TestExtractorErrorMarshalAndDefaultRetry(t *testing.T) {
	e := NewExtractorError("x", "msg", 499)
	b, err := e.MarshalJSON()
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	require.Equal(t, "x", m["type"])
	require.Equal(t, "msg", m["message"])

	rc := DefaultRetryConfig()
	require.Equal(t, 3, rc.MaxAttempts)
	require.True(t, rc.Jitter)
	require.Contains(t, rc.RetryableErrors, "timeout")
}

