package extractor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigManager_DefaultsAndSetters(t *testing.T) {
	cm := NewConfigManager()
	require.NotNil(t, cm)
	require.NotNil(t, cm.GetGlobalConfig())

	g := cm.GetGlobalConfig()
	g.LogLevel = "debug"
	cm.SetGlobalConfig(g)
	got := cm.GetGlobalConfig()
	require.Equal(t, "debug", got.LogLevel)

	llm := DefaultProviderConfig(ProviderOpenAI)
	llm.APIKey = "k"
	require.NoError(t, cm.SetLLMConfig("openai", llm))
	outLLM, err := cm.GetLLMConfig("openai")
	require.NoError(t, err)
	require.Equal(t, llm.Model, outLLM.Model)

	emb := DefaultEmbeddingProviderConfig(EmbeddingProviderOpenAI)
	emb.APIKey = "k"
	require.NoError(t, cm.SetEmbeddingConfig("openai", emb))
	outEmb, err := cm.GetEmbeddingConfig("openai")
	require.NoError(t, err)
	require.Equal(t, emb.Model, outEmb.Model)

	require.Len(t, cm.ListLLMConfigs(), 1)
	require.Len(t, cm.ListEmbeddingConfigs(), 1)
	require.NoError(t, cm.RemoveLLMConfig("openai"))
	require.NoError(t, cm.RemoveEmbeddingConfig("openai"))
	require.Error(t, cm.RemoveLLMConfig("missing"))
	require.Error(t, cm.RemoveEmbeddingConfig("missing"))
}

func TestConfigManager_LoadFromFile_JSON_YAML_AndErrors(t *testing.T) {
	cm := NewConfigManager()
	tmp := t.TempDir()

	jsonPath := filepath.Join(tmp, "cfg.json")
	jsonData := `{
		"global":{"default_llm_provider":"gemini","log_level":"warn"},
		"llm":{"openai":{"type":"openai","model":"gpt-4","api_key":"k"}},
		"embedding":{"openai":{"type":"openai","model":"text-embedding-3-small","dimensions":1536}}
	}`
	require.NoError(t, os.WriteFile(jsonPath, []byte(jsonData), 0o644))
	require.NoError(t, cm.LoadFromFile(jsonPath))
	g := cm.GetGlobalConfig()
	require.Equal(t, "gemini", g.DefaultLLMProvider)

	yamlPath := filepath.Join(tmp, "cfg.yaml")
	yamlData := `
global:
  default_llm_provider: anthropic
llm:
  anthropic:
    type: anthropic
    model: claude-3-5-sonnet
    api_key: k2
`
	require.NoError(t, os.WriteFile(yamlPath, []byte(yamlData), 0o644))
	require.NoError(t, cm.LoadFromFile(yamlPath))

	badExt := filepath.Join(tmp, "cfg.txt")
	require.NoError(t, os.WriteFile(badExt, []byte("x"), 0o644))
	require.Error(t, cm.LoadFromFile(badExt))

	badJSON := filepath.Join(tmp, "bad.json")
	require.NoError(t, os.WriteFile(badJSON, []byte("{"), 0o644))
	require.Error(t, cm.LoadFromFile(badJSON))
}

func TestConfigManager_SaveReloadEnvAndToggles(t *testing.T) {
	cm := NewConfigManager()
	tmp := t.TempDir()

	// SaveToFile json/yaml and unsupported extension
	require.NoError(t, cm.SaveToFile(filepath.Join(tmp, "out.json")))
	require.NoError(t, cm.SaveToFile(filepath.Join(tmp, "out.yaml")))
	require.Error(t, cm.SaveToFile(filepath.Join(tmp, "out.ini")))

	// Environment load paths
	t.Setenv("AI_MEMORY_DEFAULT_LLM_PROVIDER", "deepseek")
	t.Setenv("AI_MEMORY_MAX_CONCURRENT_REQUESTS", "200")
	t.Setenv("AI_MEMORY_ENABLE_METRICS", "false")
	t.Setenv("OPENAI_API_KEY", "k-openai")
	t.Setenv("OPENAI_MODEL", "gpt-4o-mini")
	t.Setenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
	t.Setenv("OLLAMA_ENDPOINT", "http://localhost:11434")
	t.Setenv("OLLAMA_MODEL", "llama3")
	t.Setenv("OLLAMA_EMBEDDING_MODEL", "nomic-embed")
	require.NoError(t, cm.LoadFromEnvironment())
	g := cm.GetGlobalConfig()
	require.Equal(t, "deepseek", g.DefaultLLMProvider)
	require.Equal(t, 200, g.MaxConcurrentRequests)
	require.False(t, g.EnableMetrics)

	_, err := cm.GetLLMConfig("openai")
	require.NoError(t, err)
	_, err = cm.GetEmbeddingConfig("openai")
	require.NoError(t, err)
	_, err = cm.GetLLMConfig("ollama")
	require.NoError(t, err)
	_, err = cm.GetEmbeddingConfig("ollama")
	require.NoError(t, err)

	// reload from current source
	require.NoError(t, cm.Reload())

	// auto reload toggles (cover worker start path without asserting goroutine side effects)
	cm.EnableAutoReload(10 * time.Millisecond)
	cm.DisableAutoReload()
	cm.SetEnvironmentOverrides(true)
	cm.SetEnvironmentOverrides(false)
}

func TestConfigManager_Constructors(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "c.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"global":{"default_llm_provider":"openai"}}`), 0o644))

	cm, err := NewConfigManagerFromFile(p)
	require.NoError(t, err)
	require.NotNil(t, cm)

	cm2, err := NewConfigManagerFromEnv()
	require.NoError(t, err)
	require.NotNil(t, cm2)
}

