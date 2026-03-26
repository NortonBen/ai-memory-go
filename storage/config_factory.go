// Package storage provides configuration factory and environment loading for storage backends
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/vector"
	"gopkg.in/yaml.v3"
)

// ConfigFactory creates and manages storage configurations
type ConfigFactory struct {
	profiles map[string]*StorageConfig
	current  string
}

// NewConfigFactory creates a new configuration factory
func NewConfigFactory() *ConfigFactory {
	factory := &ConfigFactory{
		profiles: make(map[string]*StorageConfig),
		current:  "default",
	}

	// Add default profile
	factory.profiles["default"] = DefaultStorageConfig()

	// Add common profiles
	factory.addCommonProfiles()

	return factory
}

// LoadFromFile loads configuration from a JSON or YAML file
func (cf *ConfigFactory) LoadFromFile(filename string) (*StorageConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config StorageConfig

	// Determine file format by extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	return &config, nil
}

// LoadFromEnvironment creates configuration from environment variables
func (cf *ConfigFactory) LoadFromEnvironment() (*StorageConfig, error) {
	config := DefaultStorageConfig()

	// Load global settings
	if err := cf.loadGlobalSettings(config); err != nil {
		return nil, fmt.Errorf("failed to load global settings: %w", err)
	}

	// Load relational database settings
	if err := cf.loadRelationalSettings(config); err != nil {
		return nil, fmt.Errorf("failed to load relational settings: %w", err)
	}

	// Load graph database settings
	if err := cf.loadGraphSettings(config); err != nil {
		return nil, fmt.Errorf("failed to load graph settings: %w", err)
	}

	// Load vector database settings
	if err := cf.loadVectorSettings(config); err != nil {
		return nil, fmt.Errorf("failed to load vector settings: %w", err)
	}

	return config, nil
}

// CreateFileBasedConfig creates a configuration optimized for file-based storage
func (cf *ConfigFactory) CreateFileBasedConfig(dataDir string) *StorageConfig {
	config := DefaultStorageConfig()
	config.Environment = "embedded"

	// SQLite for relational storage
	config.Relational.Type = StorageTypeSQLite
	config.Relational.Database = filepath.Join(dataDir, "memory.db")
	config.Relational.MaxConnections = 5

	// SQLite for graph storage (using recursive CTEs)
	config.Graph.Type = graph.StoreTypeSQLite
	config.Graph.Database = filepath.Join(dataDir, "graph.db")
	config.Graph.BatchSize = 100
	config.Graph.ConnTimeout = 30 * time.Second

	// SQLite for vector storage
	config.Vector.Type = vector.StoreTypeSQLite
	config.Vector.Database = filepath.Join(dataDir, "vectors.db")
	config.Vector.Collection = "embeddings"
	config.Vector.Dimension = 768

	return config
}

// CreateCloudConfig creates a configuration optimized for cloud deployment
func (cf *ConfigFactory) CreateCloudConfig() *StorageConfig {
	config := DefaultStorageConfig()
	config.Environment = "production"

	// PostgreSQL for relational storage
	config.Relational.Type = StorageTypePostgreSQL
	config.Relational.MaxConnections = 50
	config.Relational.MinConnections = 5
	config.Relational.EnableFullText = true
	config.Relational.EnableJSONIndexes = true

	// Neo4j for graph storage
	config.Graph.Type = graph.StoreTypeNeo4j
	config.Graph.MaxConnections = 20
	config.Graph.EnableIndexing = true
	config.Graph.EnableCaching = true
	config.Graph.Neo4j = &graph.Neo4jConfig{
		MaxConnectionLife: 30 * time.Minute,
		MaxConnectionPool: 20,
		ConnectionTimeout: 30 * time.Second,
		SocketTimeout:     60 * time.Second,
		EncryptionLevel:   "required",
		TrustStrategy:     "trust_system_ca_signed_certificates",
		EnableBookmarks:   true,
		EnableRouting:     true,
		EnableMetrics:     true,
	}

	// Qdrant for vector storage
	config.Vector.Type = vector.StoreTypeQdrant
	config.Vector.MaxConnections = 20
	config.Vector.EnableCaching = true
	config.Vector.Qdrant = &vector.QdrantConfig{
		Timeout:           30 * time.Second,
		MaxConnections:    20,
		EnableTLS:         true,
		EnableGRPC:        true,
		CompressionLevel:  "gzip",
		DefaultVectorSize: 768,
		DefaultDistance:   "cosine",
		OnDiskPayload:     true,
		MaxSegmentSize:    4 * 1024 * 1024, // 4MB
		MemmapThreshold:   1024 * 1024,     // 1MB
		IndexingThreshold: 20000,
	}

	return config
}

// CreateDevelopmentConfig creates a configuration optimized for development
func (cf *ConfigFactory) CreateDevelopmentConfig() *StorageConfig {
	config := DefaultStorageConfig()
	config.Environment = "development"

	// In-memory or lightweight options for development
	config.Relational.Type = StorageTypeSQLite
	config.Relational.Database = ":memory:"
	config.Relational.MaxConnections = 5

	config.Graph.Type = graph.StoreTypeInMemory
	config.Vector.Type = vector.StoreTypeInMemory

	return config
}

// GetProfile returns a named configuration profile
func (cf *ConfigFactory) GetProfile(name string) (*StorageConfig, error) {
	profile, exists := cf.profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile not found: %s", name)
	}

	// Return a copy to prevent modification
	configCopy := *profile
	return &configCopy, nil
}

// AddProfile adds a new configuration profile
func (cf *ConfigFactory) AddProfile(name string, config *StorageConfig) {
	cf.profiles[name] = config
}

// ListProfiles returns all available profile names
func (cf *ConfigFactory) ListProfiles() []string {
	profiles := make([]string, 0, len(cf.profiles))
	for name := range cf.profiles {
		profiles = append(profiles, name)
	}
	return profiles
}

// SaveToFile saves configuration to a file
func (cf *ConfigFactory) SaveToFile(config *StorageConfig, filename string) error {
	var data []byte
	var err error

	// Determine file format by extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(config, "", "  ")
	case ".yaml", ".yml":
		data, err = yaml.Marshal(config)
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ValidateConfig validates a storage configuration
func (cf *ConfigFactory) ValidateConfig(config *StorageConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate relational config
	if err := cf.validateRelationalConfig(config.Relational); err != nil {
		return fmt.Errorf("invalid relational config: %w", err)
	}

	// Validate graph config
	if err := cf.validateGraphConfig(config.Graph); err != nil {
		return fmt.Errorf("invalid graph config: %w", err)
	}

	// Validate vector config
	if err := cf.validateVectorConfig(config.Vector); err != nil {
		return fmt.Errorf("invalid vector config: %w", err)
	}

	return nil
}

// Private helper methods

func (cf *ConfigFactory) addCommonProfiles() {
	// File-based profile (SQLite + Kuzu + LanceDB)
	cf.profiles["file-based"] = cf.CreateFileBasedConfig("./data")

	// Cloud profile (PostgreSQL + Neo4j + Qdrant)
	cf.profiles["cloud"] = cf.CreateCloudConfig()

	// Development profile (in-memory)
	cf.profiles["development"] = cf.CreateDevelopmentConfig()

	// Hybrid profile (PostgreSQL + SurrealDB + pgvector)
	hybridConfig := DefaultStorageConfig()
	hybridConfig.Environment = "production"
	hybridConfig.Relational.Type = StorageTypePostgreSQL
	hybridConfig.Graph.Type = graph.StoreTypeSurrealDB
	hybridConfig.Vector.Type = vector.StoreTypePgVector
	cf.profiles["hybrid"] = hybridConfig
}

func (cf *ConfigFactory) loadGlobalSettings(config *StorageConfig) error {
	if env := os.Getenv("AI_MEMORY_STORAGE_ENVIRONMENT"); env != "" {
		config.Environment = env
	}

	if val := os.Getenv("AI_MEMORY_STORAGE_ENABLE_TRANSACTIONS"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			config.EnableTransactions = enabled
		}
	}

	if val := os.Getenv("AI_MEMORY_STORAGE_CONNECTION_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.ConnectionTimeout = duration
		}
	}

	if val := os.Getenv("AI_MEMORY_STORAGE_QUERY_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.QueryTimeout = duration
		}
	}

	if val := os.Getenv("AI_MEMORY_STORAGE_MAX_RETRIES"); val != "" {
		if retries, err := strconv.Atoi(val); err == nil {
			config.MaxRetries = retries
		}
	}

	return nil
}

func (cf *ConfigFactory) loadRelationalSettings(config *StorageConfig) error {
	if dbType := os.Getenv("AI_MEMORY_RELATIONAL_TYPE"); dbType != "" {
		config.Relational.Type = StorageType(dbType)
	}

	if host := os.Getenv("AI_MEMORY_RELATIONAL_HOST"); host != "" {
		config.Relational.Host = host
	}

	if port := os.Getenv("AI_MEMORY_RELATIONAL_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Relational.Port = p
		}
	}

	if database := os.Getenv("AI_MEMORY_RELATIONAL_DATABASE"); database != "" {
		config.Relational.Database = database
	}

	if username := os.Getenv("AI_MEMORY_RELATIONAL_USERNAME"); username != "" {
		config.Relational.Username = username
	}

	if password := os.Getenv("AI_MEMORY_RELATIONAL_PASSWORD"); password != "" {
		config.Relational.Password = password
	}

	if sslMode := os.Getenv("AI_MEMORY_RELATIONAL_SSL_MODE"); sslMode != "" {
		config.Relational.SSLMode = sslMode
	}

	if maxConns := os.Getenv("AI_MEMORY_RELATIONAL_MAX_CONNECTIONS"); maxConns != "" {
		if mc, err := strconv.Atoi(maxConns); err == nil {
			config.Relational.MaxConnections = mc
		}
	}

	return nil
}

func (cf *ConfigFactory) loadGraphSettings(config *StorageConfig) error {
	if graphType := os.Getenv("AI_MEMORY_GRAPH_TYPE"); graphType != "" {
		config.Graph.Type = graph.GraphStoreType(graphType)
	}

	if host := os.Getenv("AI_MEMORY_GRAPH_HOST"); host != "" {
		config.Graph.Host = host
	}

	if port := os.Getenv("AI_MEMORY_GRAPH_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Graph.Port = p
		}
	}

	if database := os.Getenv("AI_MEMORY_GRAPH_DATABASE"); database != "" {
		config.Graph.Database = database
	}

	if username := os.Getenv("AI_MEMORY_GRAPH_USERNAME"); username != "" {
		config.Graph.Username = username
	}

	if password := os.Getenv("AI_MEMORY_GRAPH_PASSWORD"); password != "" {
		config.Graph.Password = password
	}

	// Load provider-specific settings
	cf.loadNeo4jSettings(config)
	cf.loadSurrealDBSettings(config)
	cf.loadKuzuSettings(config)

	return nil
}

func (cf *ConfigFactory) loadVectorSettings(config *StorageConfig) error {
	if vectorType := os.Getenv("AI_MEMORY_VECTOR_TYPE"); vectorType != "" {
		config.Vector.Type = vector.VectorStoreType(vectorType)
	}

	if host := os.Getenv("AI_MEMORY_VECTOR_HOST"); host != "" {
		config.Vector.Host = host
	}

	if port := os.Getenv("AI_MEMORY_VECTOR_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.Vector.Port = p
		}
	}

	if database := os.Getenv("AI_MEMORY_VECTOR_DATABASE"); database != "" {
		config.Vector.Database = database
	}

	if collection := os.Getenv("AI_MEMORY_VECTOR_COLLECTION"); collection != "" {
		config.Vector.Collection = collection
	}

	if dimension := os.Getenv("AI_MEMORY_VECTOR_DIMENSION"); dimension != "" {
		if d, err := strconv.Atoi(dimension); err == nil {
			config.Vector.Dimension = d
		}
	}

	if metric := os.Getenv("AI_MEMORY_VECTOR_DISTANCE_METRIC"); metric != "" {
		config.Vector.DistanceMetric = metric
	}

	if indexType := os.Getenv("AI_MEMORY_VECTOR_INDEX_TYPE"); indexType != "" {
		config.Vector.IndexType = indexType
	}

	// Load provider-specific settings
	cf.loadQdrantSettings(config)
	cf.loadLanceDBSettings(config)
	cf.loadPgVectorSettings(config)
	cf.loadChromaDBSettings(config)
	cf.loadRedisSettings(config)

	return nil
}

func (cf *ConfigFactory) loadNeo4jSettings(config *StorageConfig) {
	if uri := os.Getenv("AI_MEMORY_NEO4J_URI"); uri != "" {
		if config.Graph.Neo4j == nil {
			config.Graph.Neo4j = &graph.Neo4jConfig{}
		}
		config.Graph.Neo4j.URI = uri
	}

	if realm := os.Getenv("AI_MEMORY_NEO4J_REALM"); realm != "" {
		if config.Graph.Neo4j == nil {
			config.Graph.Neo4j = &graph.Neo4jConfig{}
		}
		config.Graph.Neo4j.Realm = realm
	}

	if encryption := os.Getenv("AI_MEMORY_NEO4J_ENCRYPTION_LEVEL"); encryption != "" {
		if config.Graph.Neo4j == nil {
			config.Graph.Neo4j = &graph.Neo4jConfig{}
		}
		config.Graph.Neo4j.EncryptionLevel = encryption
	}

	if trust := os.Getenv("AI_MEMORY_NEO4J_TRUST_STRATEGY"); trust != "" {
		if config.Graph.Neo4j == nil {
			config.Graph.Neo4j = &graph.Neo4jConfig{}
		}
		config.Graph.Neo4j.TrustStrategy = trust
	}
}

func (cf *ConfigFactory) loadSurrealDBSettings(config *StorageConfig) {
	if endpoint := os.Getenv("AI_MEMORY_SURREALDB_ENDPOINT"); endpoint != "" {
		if config.Graph.SurrealDB == nil {
			config.Graph.SurrealDB = &graph.SurrealDBConfig{}
		}
		config.Graph.SurrealDB.Endpoint = endpoint
	}

	if namespace := os.Getenv("AI_MEMORY_SURREALDB_NAMESPACE"); namespace != "" {
		if config.Graph.SurrealDB == nil {
			config.Graph.SurrealDB = &graph.SurrealDBConfig{}
		}
		config.Graph.SurrealDB.Namespace = namespace
	}

	if scope := os.Getenv("AI_MEMORY_SURREALDB_SCOPE"); scope != "" {
		if config.Graph.SurrealDB == nil {
			config.Graph.SurrealDB = &graph.SurrealDBConfig{}
		}
		config.Graph.SurrealDB.Scope = scope
	}

	if token := os.Getenv("AI_MEMORY_SURREALDB_TOKEN"); token != "" {
		if config.Graph.SurrealDB == nil {
			config.Graph.SurrealDB = &graph.SurrealDBConfig{}
		}
		config.Graph.SurrealDB.Token = token
	}
}

func (cf *ConfigFactory) loadKuzuSettings(config *StorageConfig) {
	if dbPath := os.Getenv("AI_MEMORY_KUZU_DATABASE_PATH"); dbPath != "" {
		if config.Graph.Kuzu == nil {
			config.Graph.Kuzu = &graph.KuzuConfig{}
		}
		config.Graph.Kuzu.DatabasePath = dbPath
	}

	if bufferSize := os.Getenv("AI_MEMORY_KUZU_BUFFER_POOL_SIZE"); bufferSize != "" {
		if bs, err := strconv.ParseInt(bufferSize, 10, 64); err == nil {
			if config.Graph.Kuzu == nil {
				config.Graph.Kuzu = &graph.KuzuConfig{}
			}
			config.Graph.Kuzu.BufferPoolSize = bs
		}
	}

	if maxThreads := os.Getenv("AI_MEMORY_KUZU_MAX_NUM_THREADS"); maxThreads != "" {
		if mt, err := strconv.Atoi(maxThreads); err == nil {
			if config.Graph.Kuzu == nil {
				config.Graph.Kuzu = &graph.KuzuConfig{}
			}
			config.Graph.Kuzu.MaxNumThreads = mt
		}
	}
}

func (cf *ConfigFactory) loadQdrantSettings(config *StorageConfig) {
	if apiKey := os.Getenv("AI_MEMORY_QDRANT_API_KEY"); apiKey != "" {
		if config.Vector.Qdrant == nil {
			config.Vector.Qdrant = &vector.QdrantConfig{}
		}
		config.Vector.Qdrant.APIKey = apiKey
	}

	if grpc := os.Getenv("AI_MEMORY_QDRANT_ENABLE_GRPC"); grpc != "" {
		if enabled, err := strconv.ParseBool(grpc); err == nil {
			if config.Vector.Qdrant == nil {
				config.Vector.Qdrant = &vector.QdrantConfig{}
			}
			config.Vector.Qdrant.EnableGRPC = enabled
		}
	}

	if grpcPort := os.Getenv("AI_MEMORY_QDRANT_GRPC_PORT"); grpcPort != "" {
		if port, err := strconv.Atoi(grpcPort); err == nil {
			if config.Vector.Qdrant == nil {
				config.Vector.Qdrant = &vector.QdrantConfig{}
			}
			config.Vector.Qdrant.GRPCPort = port
		}
	}
}

func (cf *ConfigFactory) loadLanceDBSettings(config *StorageConfig) {
	if uri := os.Getenv("AI_MEMORY_LANCEDB_URI"); uri != "" {
		if config.Vector.LanceDB == nil {
			config.Vector.LanceDB = &vector.LanceDBConfig{}
		}
		config.Vector.LanceDB.URI = uri
	}

	if storageType := os.Getenv("AI_MEMORY_LANCEDB_STORAGE_TYPE"); storageType != "" {
		if config.Vector.LanceDB == nil {
			config.Vector.LanceDB = &vector.LanceDBConfig{}
		}
		config.Vector.LanceDB.StorageType = storageType
	}

	if dataDir := os.Getenv("AI_MEMORY_LANCEDB_DATA_DIR"); dataDir != "" {
		if config.Vector.LanceDB == nil {
			config.Vector.LanceDB = &vector.LanceDBConfig{}
		}
		config.Vector.LanceDB.DataDir = dataDir
	}

	if tableName := os.Getenv("AI_MEMORY_LANCEDB_TABLE_NAME"); tableName != "" {
		if config.Vector.LanceDB == nil {
			config.Vector.LanceDB = &vector.LanceDBConfig{}
		}
		config.Vector.LanceDB.TableName = tableName
	}
}

func (cf *ConfigFactory) loadPgVectorSettings(config *StorageConfig) {
	if dimensions := os.Getenv("AI_MEMORY_PGVECTOR_VECTOR_DIMENSIONS"); dimensions != "" {
		if d, err := strconv.Atoi(dimensions); err == nil {
			if config.Vector.PgVector == nil {
				config.Vector.PgVector = &vector.PgVectorConfig{}
			}
			config.Vector.PgVector.VectorDimensions = d
		}
	}

	if indexType := os.Getenv("AI_MEMORY_PGVECTOR_INDEX_TYPE"); indexType != "" {
		if config.Vector.PgVector == nil {
			config.Vector.PgVector = &vector.PgVectorConfig{}
		}
		config.Vector.PgVector.IndexType = indexType
	}

	if tableName := os.Getenv("AI_MEMORY_PGVECTOR_TABLE_NAME"); tableName != "" {
		if config.Vector.PgVector == nil {
			config.Vector.PgVector = &vector.PgVectorConfig{}
		}
		config.Vector.PgVector.TableName = tableName
	}
}

func (cf *ConfigFactory) loadChromaDBSettings(config *StorageConfig) {
	if apiKey := os.Getenv("AI_MEMORY_CHROMADB_API_KEY"); apiKey != "" {
		if config.Vector.ChromaDB == nil {
			config.Vector.ChromaDB = &vector.ChromaDBConfig{}
		}
		config.Vector.ChromaDB.APIKey = apiKey
	}

	if tenant := os.Getenv("AI_MEMORY_CHROMADB_TENANT"); tenant != "" {
		if config.Vector.ChromaDB == nil {
			config.Vector.ChromaDB = &vector.ChromaDBConfig{}
		}
		config.Vector.ChromaDB.Tenant = tenant
	}

	if collection := os.Getenv("AI_MEMORY_CHROMADB_DEFAULT_COLLECTION"); collection != "" {
		if config.Vector.ChromaDB == nil {
			config.Vector.ChromaDB = &vector.ChromaDBConfig{}
		}
		config.Vector.ChromaDB.DefaultCollectionName = collection
	}
}

func (cf *ConfigFactory) loadRedisSettings(config *StorageConfig) {
	if indexName := os.Getenv("AI_MEMORY_REDIS_INDEX_NAME"); indexName != "" {
		if config.Vector.Redis == nil {
			config.Vector.Redis = &vector.RedisConfig{}
		}
		config.Vector.Redis.IndexName = indexName
	}

	if vectorField := os.Getenv("AI_MEMORY_REDIS_VECTOR_FIELD"); vectorField != "" {
		if config.Vector.Redis == nil {
			config.Vector.Redis = &vector.RedisConfig{}
		}
		config.Vector.Redis.VectorField = vectorField
	}

	if algorithm := os.Getenv("AI_MEMORY_REDIS_VECTOR_ALGORITHM"); algorithm != "" {
		if config.Vector.Redis == nil {
			config.Vector.Redis = &vector.RedisConfig{}
		}
		config.Vector.Redis.VectorAlgorithm = algorithm
	}
}

func (cf *ConfigFactory) validateRelationalConfig(config *RelationalConfig) error {
	if config == nil {
		return fmt.Errorf("relational config cannot be nil")
	}

	if config.Type == "" {
		return fmt.Errorf("relational storage type is required")
	}

	if config.Database == "" {
		return fmt.Errorf("database name is required")
	}

	if config.MaxConnections <= 0 {
		return fmt.Errorf("max connections must be positive")
	}

	return nil
}

func (cf *ConfigFactory) validateGraphConfig(config *graph.GraphConfig) error {
	if config == nil {
		return fmt.Errorf("graph config cannot be nil")
	}

	if config.Type == "" {
		return fmt.Errorf("graph storage type is required")
	}

	if config.MaxConnections <= 0 {
		return fmt.Errorf("max connections must be positive")
	}

	return nil
}

func (cf *ConfigFactory) validateVectorConfig(config *vector.VectorConfig) error {
	if config == nil {
		return fmt.Errorf("vector config cannot be nil")
	}

	if config.Type == "" {
		return fmt.Errorf("vector storage type is required")
	}

	if config.Dimension <= 0 {
		return fmt.Errorf("vector dimension must be positive")
	}

	if config.MaxConnections <= 0 {
		return fmt.Errorf("max connections must be positive")
	}

	return nil
}
