package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/extractor/registry"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/storage"
	_ "github.com/NortonBen/ai-memory-go/storage/adapters/postgresql"
	_ "github.com/NortonBen/ai-memory-go/storage/adapters/sqlite"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.Flags().Bool("init", false, "Initialize a default configuration file")
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long:  `View current configuration or initialize a default config file using --init.`,
	Run: func(cmd *cobra.Command, args []string) {
		initFlag, _ := cmd.Flags().GetBool("init")
		if initFlag {
			createDefaultConfig()
			return
		}

		fmt.Println("Current Configuration:")
		settings := viper.AllSettings()
		for k, v := range settings {
			fmt.Printf("  %s: %v\n", k, v)
		}
	},
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".ai-memory")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}

func createDefaultConfig() {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	configPath := filepath.Join(home, ".ai-memory.yaml")

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config file already exists at %s\n", configPath)
		return
	}

	viper.Set("db.datadir", filepath.Join(home, ".ai-memory", "data"))
	viper.Set("db.vector.provider", "sqlite")
	viper.Set("db.graph.provider", "sqlite")
	viper.Set("db.redis.endpoint", "localhost:6379")
	viper.Set("db.redis.password", "")

	viper.Set("db.postgres.host", "localhost")
	viper.Set("db.postgres.port", 5432)
	viper.Set("db.postgres.username", "postgres")
	viper.Set("db.postgres.password", "postgres")
	viper.Set("db.postgres.database", "ai_memory")
	viper.Set("db.postgres.collection", "vector_embeddings")

	viper.Set("db.qdrant.host", "localhost")
	viper.Set("db.qdrant.port", 6334)
	viper.Set("db.qdrant.collection", "ai_memory")

	viper.Set("db.neo4j.host", "localhost")
	viper.Set("db.neo4j.port", 7687)
	viper.Set("db.neo4j.username", "neo4j")
	viper.Set("db.neo4j.password", "password")
	viper.Set("db.neo4j.database", "neo4j")

	viper.Set("llm.provider", "lmstudio")
	viper.Set("llm.endpoint", "http://localhost:1234/v1")
	viper.Set("llm.model", "qwen/qwen3-4b-2507")
	viper.Set("llm.api_key", "")
	viper.Set("embedder.provider", "lmstudio")
	viper.Set("embedder.endpoint", "http://localhost:1234/v1")
	viper.Set("embedder.model", "text-embedding-nomic-embed-text-v1.5")
	viper.Set("embedder.dimensions", 768)
	viper.Set("embedder.api_key", "")

	err = viper.SafeWriteConfigAs(configPath)
	cobra.CheckErr(err)

	fmt.Printf("Created default configuration file at %s\n", configPath)
}

// InitEngine initializes a MemoryEngine based on viper config
func InitEngine(ctx context.Context) (engine.MemoryEngine, error) {
	dataDir := viper.GetString("db.datadir")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".ai-memory", "data")
	}
	_ = os.MkdirAll(dataDir, 0o750)

	// Embedder Factory
	embProviderStr := viper.GetString("embedder.provider")
	embEndpoint := viper.GetString("embedder.endpoint")
	embModel := viper.GetString("embedder.model")
	embApiKey := viper.GetString("embedder.api_key")
	embDim := viper.GetInt("embedder.dimensions")
	if embDim == 0 {
		embDim = 768
	}

	embFactory := registry.NewEmbeddingProviderFactory()
	embConfig := &extractor.EmbeddingProviderConfig{
		Type:       extractor.EmbeddingProviderType(embProviderStr),
		Endpoint:   embEndpoint,
		Model:      embModel,
		APIKey:     embApiKey,
		Dimensions: embDim,
	}
	baseEmbedder, err := embFactory.CreateProvider(embConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to init base embedder: %w", err)
	}

	cache := vector.NewInMemoryEmbeddingCache()
	embedder := vector.NewAutoEmbedder(embProviderStr, cache)
	embedder.AddProvider(embProviderStr, baseEmbedder)

	// Stores
	var graphStore graph.GraphStore
	graphProvider := viper.GetString("db.graph.provider")
	if graphProvider == "redis" {
		redisEndpoint := viper.GetString("db.redis.endpoint")
		redisPassword := viper.GetString("db.redis.password")
		graphStore, err = graph.NewRedisGraphStore(redisEndpoint, redisPassword)
	} else if graphProvider == "neo4j" {
		cfg := &graph.GraphConfig{
			Host:     viper.GetString("db.neo4j.host"),
			Port:     viper.GetInt("db.neo4j.port"),
			Username: viper.GetString("db.neo4j.username"),
			Password: viper.GetString("db.neo4j.password"),
			Database: viper.GetString("db.neo4j.database"),
		}
		graphStore, err = graph.NewNeo4jStore(cfg.Host, cfg.Username, cfg.Password)
	} else {
		graphStore, err = graph.NewSQLiteGraphStore(filepath.Join(dataDir, "graph.db"))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to init graph store: %w", err)
	}

	var vecStore vector.VectorStore
	vecProvider := viper.GetString("db.vector.provider")
	if vecProvider == "redis" {
		redisEndpoint := viper.GetString("db.redis.endpoint")
		redisPassword := viper.GetString("db.redis.password")
		vecStore, err = vector.NewRedisVectorStore(redisEndpoint, redisPassword, embDim)
	} else if vecProvider == "postgres" {
		cfg := &vector.VectorConfig{
			Host:       viper.GetString("db.postgres.host"),
			Port:       viper.GetInt("db.postgres.port"),
			Username:   viper.GetString("db.postgres.username"),
			Password:   viper.GetString("db.postgres.password"),
			Database:   viper.GetString("db.postgres.database"),
			Collection: viper.GetString("db.postgres.collection"),
			Dimension:  embDim,
		}
		vecStore, err = vector.NewPgVectorStore(cfg)
	} else if vecProvider == "qdrant" {
		cfg := &vector.VectorConfig{
			Host:       viper.GetString("db.qdrant.host"),
			Port:       viper.GetInt("db.qdrant.port"),
			Collection: viper.GetString("db.qdrant.collection"),
			Dimension:  embDim,
		}
		vecStore, err = vector.NewQdrantStore(cfg)
	} else {
		vecStore, err = vector.NewSQLiteVectorStore(filepath.Join(dataDir, "vectors.db"), embDim)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to init vector store: %w", err)
	}

	var relStore storage.RelationalStore
	relStore, err = storage.NewSQLiteAdapter(&storage.RelationalConfig{
		Type:        storage.StorageTypeSQLite,
		Database:    filepath.Join(dataDir, "rel.db"),
		ConnTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init relational store: %w", err)
	}

	// LLM Factory
	llmProviderStr := viper.GetString("llm.provider")
	llmEndpoint := viper.GetString("llm.endpoint")
	llmModel := viper.GetString("llm.model")
	llmApiKey := viper.GetString("llm.api_key")

	llmFactory := registry.NewProviderFactory()
	llmConfig := &extractor.ProviderConfig{
		Type:     extractor.ProviderType(llmProviderStr),
		Endpoint: llmEndpoint,
		Model:    llmModel,
		APIKey:   llmApiKey,
	}
	var llmProv extractor.LLMProvider
	llmProv, err = llmFactory.CreateProvider(llmConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to init llm provider: %w", err)
	}
	llmExt := extractor.NewBasicExtractor(llmProv, nil)

	eng := engine.NewMemoryEngineWithStores(llmExt, embedder, relStore, graphStore, vecStore, engine.EngineConfig{MaxWorkers: 4})
	return eng, nil
}
