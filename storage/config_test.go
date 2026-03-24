package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultStorageConfig(t *testing.T) {
	config := DefaultStorageConfig()

	// Test global settings
	if config.Environment != "development" {
		t.Errorf("Expected environment 'development', got %s", config.Environment)
	}

	if !config.EnableTransactions {
		t.Error("Expected transactions to be enabled by default")
	}

	if config.ConnectionTimeout != 30*time.Second {
		t.Errorf("Expected connection timeout 30s, got %v", config.ConnectionTimeout)
	}

	// Test relational config
	if config.Relational.Type != StorageTypeSQLite {
		t.Errorf("Expected SQLite by default, got %s", config.Relational.Type)
	}

	if config.Relational.Database != "memory.db" {
		t.Errorf("Expected database 'memory.db', got %s", config.Relational.Database)
	}

	// Test graph config
	if config.Graph.Type != GraphStoreTypeInMemory {
		t.Errorf("Expected in-memory graph store by default, got %s", config.Graph.Type)
	}

	// Test vector config
	if config.Vector.Type != VectorStoreTypeInMemory {
		t.Errorf("Expected in-memory vector store by default, got %s", config.Vector.Type)
	}

	if config.Vector.Dimension != 768 {
		t.Errorf("Expected vector dimension 768, got %d", config.Vector.Dimension)
	}
}

func TestConfigFactory(t *testing.T) {
	factory := NewConfigFactory()

	// Test default profile
	defaultConfig, err := factory.GetProfile("default")
	if err != nil {
		t.Fatalf("Failed to get default profile: %v", err)
	}

	if defaultConfig.Environment != "development" {
		t.Errorf("Expected development environment, got %s", defaultConfig.Environment)
	}

	// Test file-based profile
	fileConfig, err := factory.GetProfile("file-based")
	if err != nil {
		t.Fatalf("Failed to get file-based profile: %v", err)
	}

	if fileConfig.Environment != "embedded" {
		t.Errorf("Expected embedded environment, got %s", fileConfig.Environment)
	}

	if fileConfig.Relational.Type != StorageTypeSQLite {
		t.Errorf("Expected SQLite for file-based profile, got %s", fileConfig.Relational.Type)
	}

	if fileConfig.Graph.Type != GraphStoreTypeKuzu {
		t.Errorf("Expected Kuzu for file-based profile, got %s", fileConfig.Graph.Type)
	}

	if fileConfig.Vector.Type != VectorStoreTypeLanceDB {
		t.Errorf("Expected LanceDB for file-based profile, got %s", fileConfig.Vector.Type)
	}

	// Test cloud profile
	cloudConfig, err := factory.GetProfile("cloud")
	if err != nil {
		t.Fatalf("Failed to get cloud profile: %v", err)
	}

	if cloudConfig.Environment != "production" {
		t.Errorf("Expected production environment, got %s", cloudConfig.Environment)
	}

	if cloudConfig.Relational.Type != StorageTypePostgreSQL {
		t.Errorf("Expected PostgreSQL for cloud profile, got %s", cloudConfig.Relational.Type)
	}

	if cloudConfig.Graph.Type != GraphStoreTypeNeo4j {
		t.Errorf("Expected Neo4j for cloud profile, got %s", cloudConfig.Graph.Type)
	}

	if cloudConfig.Vector.Type != VectorStoreTypeQdrant {
		t.Errorf("Expected Qdrant for cloud profile, got %s", cloudConfig.Vector.Type)
	}
}

func TestCreateFileBasedConfig(t *testing.T) {
	factory := NewConfigFactory()
	dataDir := "/tmp/test-data"

	config := factory.CreateFileBasedConfig(dataDir)

	// Test paths are correctly set
	expectedDBPath := filepath.Join(dataDir, "memory.db")
	if config.Relational.Database != expectedDBPath {
		t.Errorf("Expected database path %s, got %s", expectedDBPath, config.Relational.Database)
	}

	expectedGraphPath := filepath.Join(dataDir, "graph.kuzu")
	if config.Graph.Kuzu.DatabasePath != expectedGraphPath {
		t.Errorf("Expected graph path %s, got %s", expectedGraphPath, config.Graph.Kuzu.DatabasePath)
	}

	expectedVectorPath := filepath.Join(dataDir, "vectors.lance")
	if config.Vector.LanceDB.URI != expectedVectorPath {
		t.Errorf("Expected vector path %s, got %s", expectedVectorPath, config.Vector.LanceDB.URI)
	}

	// Test Kuzu-specific settings
	if config.Graph.Kuzu.BufferPoolSize != 256*1024*1024 {
		t.Errorf("Expected buffer pool size 256MB, got %d", config.Graph.Kuzu.BufferPoolSize)
	}

	if !config.Graph.Kuzu.EnableCompression {
		t.Error("Expected compression to be enabled")
	}

	// Test LanceDB-specific settings
	if config.Vector.LanceDB.StorageType != "local" {
		t.Errorf("Expected local storage type, got %s", config.Vector.LanceDB.StorageType)
	}

	if config.Vector.LanceDB.MetricType != "cosine" {
		t.Errorf("Expected cosine metric, got %s", config.Vector.LanceDB.MetricType)
	}
}

func TestCreateCloudConfig(t *testing.T) {
	factory := NewConfigFactory()
	config := factory.CreateCloudConfig()

	// Test production settings
	if config.Environment != "production" {
		t.Errorf("Expected production environment, got %s", config.Environment)
	}

	// Test PostgreSQL settings
	if config.Relational.MaxConnections != 50 {
		t.Errorf("Expected 50 max connections, got %d", config.Relational.MaxConnections)
	}

	if !config.Relational.EnableFullText {
		t.Error("Expected full-text search to be enabled")
	}

	// Test Neo4j settings
	if config.Graph.Neo4j == nil {
		t.Fatal("Expected Neo4j config to be set")
	}

	if config.Graph.Neo4j.EncryptionLevel != "required" {
		t.Errorf("Expected required encryption, got %s", config.Graph.Neo4j.EncryptionLevel)
	}

	if !config.Graph.Neo4j.EnableRouting {
		t.Error("Expected routing to be enabled")
	}

	// Test Qdrant settings
	if config.Vector.Qdrant == nil {
		t.Fatal("Expected Qdrant config to be set")
	}

	if !config.Vector.Qdrant.EnableTLS {
		t.Error("Expected TLS to be enabled")
	}

	if config.Vector.Qdrant.CompressionLevel != "gzip" {
		t.Errorf("Expected gzip compression, got %s", config.Vector.Qdrant.CompressionLevel)
	}
}

func TestLoadFromEnvironment(t *testing.T) {
	factory := NewConfigFactory()

	// Set environment variables
	os.Setenv("AI_MEMORY_STORAGE_ENVIRONMENT", "test")
	os.Setenv("AI_MEMORY_STORAGE_ENABLE_TRANSACTIONS", "false")
	os.Setenv("AI_MEMORY_RELATIONAL_TYPE", "postgresql")
	os.Setenv("AI_MEMORY_RELATIONAL_HOST", "localhost")
	os.Setenv("AI_MEMORY_RELATIONAL_PORT", "5432")
	os.Setenv("AI_MEMORY_RELATIONAL_DATABASE", "testdb")
	os.Setenv("AI_MEMORY_GRAPH_TYPE", "neo4j")
	os.Setenv("AI_MEMORY_VECTOR_TYPE", "qdrant")
	os.Setenv("AI_MEMORY_VECTOR_DIMENSION", "1536")

	defer func() {
		// Clean up environment variables
		os.Unsetenv("AI_MEMORY_STORAGE_ENVIRONMENT")
		os.Unsetenv("AI_MEMORY_STORAGE_ENABLE_TRANSACTIONS")
		os.Unsetenv("AI_MEMORY_RELATIONAL_TYPE")
		os.Unsetenv("AI_MEMORY_RELATIONAL_HOST")
		os.Unsetenv("AI_MEMORY_RELATIONAL_PORT")
		os.Unsetenv("AI_MEMORY_RELATIONAL_DATABASE")
		os.Unsetenv("AI_MEMORY_GRAPH_TYPE")
		os.Unsetenv("AI_MEMORY_VECTOR_TYPE")
		os.Unsetenv("AI_MEMORY_VECTOR_DIMENSION")
	}()

	config, err := factory.LoadFromEnvironment()
	if err != nil {
		t.Fatalf("Failed to load from environment: %v", err)
	}

	// Test global settings
	if config.Environment != "test" {
		t.Errorf("Expected environment 'test', got %s", config.Environment)
	}

	if config.EnableTransactions {
		t.Error("Expected transactions to be disabled")
	}

	// Test relational settings
	if config.Relational.Type != StorageTypePostgreSQL {
		t.Errorf("Expected PostgreSQL, got %s", config.Relational.Type)
	}

	if config.Relational.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got %s", config.Relational.Host)
	}

	if config.Relational.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", config.Relational.Port)
	}

	if config.Relational.Database != "testdb" {
		t.Errorf("Expected database 'testdb', got %s", config.Relational.Database)
	}

	// Test graph settings
	if config.Graph.Type != GraphStoreTypeNeo4j {
		t.Errorf("Expected Neo4j, got %s", config.Graph.Type)
	}

	// Test vector settings
	if config.Vector.Type != VectorStoreTypeQdrant {
		t.Errorf("Expected Qdrant, got %s", config.Vector.Type)
	}

	if config.Vector.Dimension != 1536 {
		t.Errorf("Expected dimension 1536, got %d", config.Vector.Dimension)
	}
}

func TestValidateConfig(t *testing.T) {
	factory := NewConfigFactory()

	// Test valid config
	validConfig := DefaultStorageConfig()
	if err := factory.ValidateConfig(validConfig); err != nil {
		t.Errorf("Valid config should not return error: %v", err)
	}

	// Test nil config
	if err := factory.ValidateConfig(nil); err == nil {
		t.Error("Nil config should return error")
	}

	// Test invalid relational config
	invalidConfig := DefaultStorageConfig()
	invalidConfig.Relational.Type = ""
	if err := factory.ValidateConfig(invalidConfig); err == nil {
		t.Error("Config with empty relational type should return error")
	}

	// Test invalid graph config
	invalidConfig2 := DefaultStorageConfig()
	invalidConfig2.Graph.MaxConnections = 0
	if err := factory.ValidateConfig(invalidConfig2); err == nil {
		t.Error("Config with zero max connections should return error")
	}

	// Test invalid vector config
	invalidConfig3 := DefaultStorageConfig()
	invalidConfig3.Vector.Dimension = 0
	if err := factory.ValidateConfig(invalidConfig3); err == nil {
		t.Error("Config with zero vector dimension should return error")
	}
}

func TestProviderSpecificConfigs(t *testing.T) {
	// Test Neo4j config
	neo4jConfig := &Neo4jConfig{
		URI:               "bolt://localhost:7687",
		MaxConnectionLife: 30 * time.Minute,
		EncryptionLevel:   "required",
		EnableBookmarks:   true,
	}

	if neo4jConfig.URI != "bolt://localhost:7687" {
		t.Errorf("Expected Neo4j URI, got %s", neo4jConfig.URI)
	}

	// Test SurrealDB config
	surrealConfig := &SurrealDBConfig{
		Endpoint:          "ws://localhost:8000/rpc",
		Namespace:         "test",
		Database:          "memory",
		EnableLivequeries: true,
	}

	if surrealConfig.Endpoint != "ws://localhost:8000/rpc" {
		t.Errorf("Expected SurrealDB endpoint, got %s", surrealConfig.Endpoint)
	}

	// Test Kuzu config
	kuzuConfig := &KuzuConfig{
		DatabasePath:      "./test.kuzu",
		BufferPoolSize:    512 * 1024 * 1024,
		MaxNumThreads:     8,
		EnableCompression: true,
	}

	if kuzuConfig.BufferPoolSize != 512*1024*1024 {
		t.Errorf("Expected buffer pool size 512MB, got %d", kuzuConfig.BufferPoolSize)
	}

	// Test Qdrant config
	qdrantConfig := &QdrantConfig{
		Host:            "localhost",
		Port:            6333,
		EnableGRPC:      true,
		DefaultDistance: "cosine",
		OnDiskPayload:   true,
	}

	if !qdrantConfig.EnableGRPC {
		t.Error("Expected GRPC to be enabled")
	}

	// Test LanceDB config
	lanceConfig := &LanceDBConfig{
		URI:         "./vectors.lance",
		StorageType: "local",
		TableName:   "embeddings",
		MetricType:  "cosine",
	}

	if lanceConfig.MetricType != "cosine" {
		t.Errorf("Expected cosine metric, got %s", lanceConfig.MetricType)
	}

	// Test pgvector config
	pgvectorConfig := &PgVectorConfig{
		Host:             "localhost",
		Port:             5432,
		Database:         "vectors",
		VectorDimensions: 768,
		IndexType:        "hnsw",
		TableName:        "embeddings",
	}

	if pgvectorConfig.IndexType != "hnsw" {
		t.Errorf("Expected HNSW index, got %s", pgvectorConfig.IndexType)
	}

	// Test ChromaDB config
	chromaConfig := &ChromaDBConfig{
		Host:                  "localhost",
		Port:                  8000,
		DefaultCollectionName: "memory",
		EnablePersistence:     true,
	}

	if !chromaConfig.EnablePersistence {
		t.Error("Expected persistence to be enabled")
	}

	// Test Redis config
	redisConfig := &RedisConfig{
		Host:            "localhost",
		Port:            6379,
		Database:        0,
		IndexName:       "vector_index",
		VectorAlgorithm: "HNSW",
		VectorType:      "FLOAT32",
	}

	if redisConfig.VectorAlgorithm != "HNSW" {
		t.Errorf("Expected HNSW algorithm, got %s", redisConfig.VectorAlgorithm)
	}
}

func TestTLSConfig(t *testing.T) {
	tlsConfig := &TLSConfig{
		Enabled:            true,
		CertFile:           "/path/to/cert.pem",
		KeyFile:            "/path/to/key.pem",
		CAFile:             "/path/to/ca.pem",
		ServerName:         "example.com",
		InsecureSkipVerify: false,
		MinVersion:         "1.2",
		MaxVersion:         "1.3",
	}

	if !tlsConfig.Enabled {
		t.Error("Expected TLS to be enabled")
	}

	if tlsConfig.InsecureSkipVerify {
		t.Error("Expected secure verification")
	}

	if tlsConfig.MinVersion != "1.2" {
		t.Errorf("Expected min version 1.2, got %s", tlsConfig.MinVersion)
	}
}

func TestEnvironmentVariableNames(t *testing.T) {
	envVars := GetEnvironmentVariableNames()

	// Check that we have a reasonable number of environment variables
	if len(envVars) < 20 {
		t.Errorf("Expected at least 20 environment variables, got %d", len(envVars))
	}

	// Check for some key environment variables
	expectedVars := []string{
		"AI_MEMORY_STORAGE_ENVIRONMENT",
		"AI_MEMORY_RELATIONAL_TYPE",
		"AI_MEMORY_GRAPH_TYPE",
		"AI_MEMORY_VECTOR_TYPE",
		"AI_MEMORY_NEO4J_URI",
		"AI_MEMORY_QDRANT_API_KEY",
	}

	for _, expected := range expectedVars {
		found := false
		for _, actual := range envVars {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected environment variable %s not found", expected)
		}
	}
}

func TestStorageConfigManager(t *testing.T) {
	manager := NewStorageConfigManager()

	// Test default config
	config := manager.GetConfig()
	if config == nil {
		t.Fatal("Expected default config to be set")
	}

	// Test adding profile
	testConfig := DefaultStorageConfig()
	testConfig.Environment = "test"
	manager.AddProfile("test", testConfig)

	// Test switching profile
	err := manager.SetProfile("test")
	if err != nil {
		t.Fatalf("Failed to set profile: %v", err)
	}

	currentConfig := manager.GetConfig()
	if currentConfig.Environment != "test" {
		t.Errorf("Expected test environment, got %s", currentConfig.Environment)
	}

	// Test invalid profile
	err = manager.SetProfile("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent profile")
	}
}
