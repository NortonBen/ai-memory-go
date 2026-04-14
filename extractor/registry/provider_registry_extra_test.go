package registry

import (
	"os"
	"path/filepath"
	"testing"

	ext "github.com/NortonBen/ai-memory-go/extractor"
	"github.com/stretchr/testify/require"
)

type failingLLMFactory struct{}

func (f *failingLLMFactory) CreateProvider(config *ext.ProviderConfig) (ext.LLMProvider, error) {
	return nil, ext.NewExtractorError("forced", "create failed", 500)
}
func (f *failingLLMFactory) CreateProviderWithDefaults(providerType ext.ProviderType, apiKey, model string) (ext.LLMProvider, error) {
	return nil, nil
}
func (f *failingLLMFactory) ListSupportedProviders() []ext.ProviderType { return nil }
func (f *failingLLMFactory) GetProviderCapabilities(providerType ext.ProviderType) (*ext.ProviderCapabilities, error) {
	return nil, nil
}
func (f *failingLLMFactory) ValidateConfig(config *ext.ProviderConfig) error { return nil }
func (f *failingLLMFactory) GetDefaultConfig(providerType ext.ProviderType) (*ext.ProviderConfig, error) {
	return nil, nil
}
func (f *failingLLMFactory) RegisterCustomProvider(providerType ext.ProviderType, createFunc func(*ext.ProviderConfig) (ext.LLMProvider, error)) error {
	return nil
}

type failingEmbeddingFactory struct{}

func (f *failingEmbeddingFactory) CreateProvider(config *ext.EmbeddingProviderConfig) (ext.EmbeddingProvider, error) {
	return nil, ext.NewExtractorError("forced", "create failed", 500)
}
func (f *failingEmbeddingFactory) CreateProviderWithDefaults(providerType ext.EmbeddingProviderType, apiKey, model string) (ext.EmbeddingProvider, error) {
	return nil, nil
}
func (f *failingEmbeddingFactory) ListSupportedProviders() []ext.EmbeddingProviderType { return nil }
func (f *failingEmbeddingFactory) GetProviderCapabilities(providerType ext.EmbeddingProviderType) (*ext.EmbeddingProviderCapabilities, error) {
	return nil, nil
}
func (f *failingEmbeddingFactory) ValidateConfig(config *ext.EmbeddingProviderConfig) error { return nil }
func (f *failingEmbeddingFactory) GetDefaultConfig(providerType ext.EmbeddingProviderType) (*ext.EmbeddingProviderConfig, error) {
	return nil, nil
}
func (f *failingEmbeddingFactory) RegisterCustomProvider(providerType ext.EmbeddingProviderType, createFunc func(*ext.EmbeddingProviderConfig) (ext.EmbeddingProvider, error)) error {
	return nil
}
func (f *failingEmbeddingFactory) GetSupportedModels(providerType ext.EmbeddingProviderType) ([]string, error) {
	return nil, nil
}
func (f *failingEmbeddingFactory) EstimateProviderCost(providerType ext.EmbeddingProviderType, tokenCount int) (float64, error) {
	return 0, nil
}

type llmCloseErrProvider struct{ *ConfiguredMockLLMProvider }

func (p *llmCloseErrProvider) Close() error { return ext.NewExtractorError("close", "llm close fail", 500) }

type embCloseErrProvider struct{ *ConfiguredMockEmbeddingProvider }

func (p *embCloseErrProvider) Close() error { return ext.NewExtractorError("close", "emb close fail", 500) }

func TestProviderRegistry_ConfigAndGettersAndFileIO(t *testing.T) {
	cfgm := ext.NewConfigManager()
	r := NewProviderRegistryWithConfig(cfgm)
	require.Equal(t, cfgm, r.GetConfigManager())
	require.NotNil(t, r.GetLLMFactory())
	require.NotNil(t, r.GetEmbeddingFactory())
	require.NotNil(t, r.GetLLMManager())
	require.NotNil(t, r.GetEmbeddingManager())

	llmCfg := ext.DefaultProviderConfig(ext.ProviderMistral)
	llmCfg.APIKey = "k"
	require.NoError(t, cfgm.SetLLMConfig("m1", llmCfg))
	embCfg := ext.DefaultEmbeddingProviderConfig(ext.EmbeddingProviderOllama)
	embCfg.APIKey = "k"
	embCfg.Dimensions = 8
	require.NoError(t, cfgm.SetEmbeddingConfig("e1", embCfg))

	require.NoError(t, r.registerProvidersFromConfig())
	require.Contains(t, r.ListLLMProviders(), "m1")
	require.Contains(t, r.ListEmbeddingProviders(), "e1")

	tmp := t.TempDir()
	out := filepath.Join(tmp, "providers.yaml")
	require.NoError(t, r.SaveToFile(out))
	_, err := os.Stat(out)
	require.NoError(t, err)

	r2 := NewProviderRegistry()
	require.NoError(t, r2.LoadFromFile(out))
	require.Contains(t, r2.ListLLMProviders(), "m1")
	require.Contains(t, r2.ListEmbeddingProviders(), "e1")
}

func TestProviderRegistry_LoadFromFile_Error(t *testing.T) {
	r := NewProviderRegistry()
	err := r.LoadFromFile(filepath.Join(t.TempDir(), "missing.yaml"))
	require.Error(t, err)
}

func TestProviderRegistry_RegisterProvidersFromConfig_ErrorPaths(t *testing.T) {
	cfgm := ext.NewConfigManager()
	r := NewProviderRegistryWithConfig(cfgm)

	llmCfg := ext.DefaultProviderConfig(ext.ProviderMistral)
	llmCfg.APIKey = "k"
	require.NoError(t, cfgm.SetLLMConfig("m1", llmCfg))
	r.llmFactory = &failingLLMFactory{}
	require.Error(t, r.registerProvidersFromConfig())

	cfgm2 := ext.NewConfigManager()
	r2 := NewProviderRegistryWithConfig(cfgm2)
	embCfg := ext.DefaultEmbeddingProviderConfig(ext.EmbeddingProviderOllama)
	embCfg.APIKey = "k"
	embCfg.Dimensions = 8
	require.NoError(t, cfgm2.SetEmbeddingConfig("e1", embCfg))
	r2.embeddingFactory = &failingEmbeddingFactory{}
	require.Error(t, r2.registerProvidersFromConfig())
}

func TestProviderRegistry_Close_ErrorAggregation(t *testing.T) {
	r := NewProviderRegistry()
	llmBase := NewConfiguredMockLLMProvider(ext.ProviderMistral, &ext.ProviderConfig{
		Type: ext.ProviderMistral, Model: "m", APIKey: "k",
	}).(*ConfiguredMockLLMProvider)
	embBase := NewConfiguredMockEmbeddingProvider(&ext.EmbeddingProviderConfig{
		Type: ext.EmbeddingProviderOllama, Model: "m", APIKey: "k", Dimensions: 8,
	}).(*ConfiguredMockEmbeddingProvider)

	r.registeredLLMs["x"] = &RegisteredLLMProvider{Name: "x", Provider: &llmCloseErrProvider{llmBase}}
	r.registeredEmbeddings["y"] = &RegisteredEmbeddingProvider{Name: "y", Provider: &embCloseErrProvider{embBase}}

	err := r.Close()
	require.Error(t, err)
}

