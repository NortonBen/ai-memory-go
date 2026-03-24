# Storage Configuration Management

This document describes the comprehensive configuration management system for AI Memory Integration storage backends.

## Overview

The storage configuration system provides unified configuration management for multiple storage backends:

- **Graph Storage**: Neo4j, SurrealDB, Kuzu, FalkorDB, In-Memory
- **Vector Storage**: Qdrant, LanceDB, pgvector, ChromaDB, Redis, In-Memory  
- **Relational Storage**: PostgreSQL, SQLite, In-Memory

## Configuration Sources

The system supports multiple configuration sources with the following precedence (highest to lowest):

1. **Environment Variables** - Runtime overrides
2. **Configuration Files** - JSON/YAML files
3. **Go Structs** - Programmatic configuration
4. **Default Values** - Built-in sensible defaults

## Configuration Profiles

Pre-defined profiles for common deployment scenarios:

### File-Based Profile (`file-based`)
Optimized for embedded deployment with zero infrastructure setup:
- **Relational**: SQLite (local file)
- **Graph**: Kuzu (local file)
- **Vector**: LanceDB (local file)

```go
config := factory.CreateFileBasedConfig("./data")
```

### Cloud Profile (`cloud`)
Optimized for production cloud deployment:
- **Relational**: PostgreSQL (with connection pooling)
- **Graph**: Neo4j (with clustering support)
- **Vector**: Qdrant (with TLS and GRPC)

```go
config := factory.CreateCloudConfig()
```

### Development Profile (`development`)
Optimized for local development:
- **Relational**: SQLite (in-memory)
- **Graph**: In-Memory
- **Vector**: In-Memory

```go
config := factory.CreateDevelopmentConfig()
```

### Hybrid Profile (`hybrid`)
Uses SurrealDB for both graph and vector operations:
- **Relational**: PostgreSQL
- **Graph**: SurrealDB
- **Vector**: pgvector (PostgreSQL extension)

## Environment Variables

### Global Settings
```bash
AI_MEMORY_STORAGE_ENVIRONMENT=production
AI_MEMORY_STORAGE_ENABLE_TRANSACTIONS=true
AI_MEMORY_STORAGE_CONNECTION_TIMEOUT=30s
AI_MEMORY_STORAGE_QUERY_TIMEOUT=30s
AI_MEMORY_STORAGE_MAX_RETRIES=3
```

### Relational Database
```bash
AI_MEMORY_RELATIONAL_TYPE=postgresql
AI_MEMORY_RELATIONAL_HOST=localhost
AI_MEMORY_RELATIONAL_PORT=5432
AI_MEMORY_RELATIONAL_DATABASE=ai_memory
AI_MEMORY_RELATIONAL_USERNAME=memory_user
AI_MEMORY_RELATIONAL_PASSWORD=secret
AI_MEMORY_RELATIONAL_SSL_MODE=require
AI_MEMORY_RELATIONAL_MAX_CONNECTIONS=50
```

### Graph Database
```bash
AI_MEMORY_GRAPH_TYPE=neo4j
AI_MEMORY_GRAPH_HOST=localhost
AI_MEMORY_GRAPH_PORT=7687
AI_MEMORY_GRAPH_DATABASE=neo4j
AI_MEMORY_GRAPH_USERNAME=neo4j
AI_MEMORY_GRAPH_PASSWORD=secret
```

#### Neo4j-Specific
```bash
AI_MEMORY_NEO4J_URI=bolt://localhost:7687
AI_MEMORY_NEO4J_REALM=
AI_MEMORY_NEO4J_ENCRYPTION_LEVEL=required
AI_MEMORY_NEO4J_TRUST_STRATEGY=trust_system_ca_signed_certificates
```

#### SurrealDB-Specific
```bash
AI_MEMORY_SURREALDB_ENDPOINT=ws://localhost:8000/rpc
AI_MEMORY_SURREALDB_NAMESPACE=ai_memory
AI_MEMORY_SURREALDB_SCOPE=
AI_MEMORY_SURREALDB_TOKEN=jwt_token_here
```

#### Kuzu-Specific
```bash
AI_MEMORY_KUZU_DATABASE_PATH=./data/graph.kuzu
AI_MEMORY_KUZU_BUFFER_POOL_SIZE=268435456  # 256MB
AI_MEMORY_KUZU_MAX_NUM_THREADS=4
```

### Vector Database
```bash
AI_MEMORY_VECTOR_TYPE=qdrant
AI_MEMORY_VECTOR_HOST=localhost
AI_MEMORY_VECTOR_PORT=6333
AI_MEMORY_VECTOR_DATABASE=
AI_MEMORY_VECTOR_COLLECTION=memory_vectors
AI_MEMORY_VECTOR_DIMENSION=768
AI_MEMORY_VECTOR_DISTANCE_METRIC=cosine
AI_MEMORY_VECTOR_INDEX_TYPE=hnsw
```

#### Qdrant-Specific
```bash
AI_MEMORY_QDRANT_API_KEY=your_api_key
AI_MEMORY_QDRANT_ENABLE_GRPC=true
AI_MEMORY_QDRANT_GRPC_PORT=6334
```

#### LanceDB-Specific
```bash
AI_MEMORY_LANCEDB_URI=./data/vectors.lance
AI_MEMORY_LANCEDB_STORAGE_TYPE=local
AI_MEMORY_LANCEDB_DATA_DIR=./data
AI_MEMORY_LANCEDB_TABLE_NAME=embeddings
```

#### pgvector-Specific
```bash
AI_MEMORY_PGVECTOR_VECTOR_DIMENSIONS=768
AI_MEMORY_PGVECTOR_INDEX_TYPE=hnsw
AI_MEMORY_PGVECTOR_TABLE_NAME=embeddings
```

#### ChromaDB-Specific
```bash
AI_MEMORY_CHROMADB_API_KEY=your_api_key
AI_MEMORY_CHROMADB_TENANT=default_tenant
AI_MEMORY_CHROMADB_DEFAULT_COLLECTION=memory_vectors
```

#### Redis-Specific
```bash
AI_MEMORY_REDIS_INDEX_NAME=vector_index
AI_MEMORY_REDIS_VECTOR_FIELD=embedding
AI_MEMORY_REDIS_VECTOR_ALGORITHM=HNSW
```

## Configuration Files

### JSON Configuration
```json
{
  "environment": "production",
  "enable_transactions": true,
  "connection_timeout": "30s",
  "query_timeout": "30s",
  "max_retries": 3,
  "relational": {
    "type": "postgresql",
    "host": "localhost",
    "port": 5432,
    "database": "ai_memory",
    "username": "memory_user",
    "password": "${AI_MEMORY_DB_PASSWORD}",
    "ssl_mode": "require",
    "max_connections": 50,
    "min_connections": 5,
    "batch_size": 1000,
    "enable_full_text": true,
    "enable_json_indexes": true
  },
  "graph": {
    "type": "neo4j",
    "host": "localhost",
    "port": 7687,
    "database": "neo4j",
    "username": "neo4j",
    "password": "${AI_MEMORY_NEO4J_PASSWORD}",
    "max_connections": 20,
    "batch_size": 500,
    "enable_indexing": true,
    "enable_caching": true,
    "neo4j": {
      "uri": "bolt://localhost:7687",
      "encryption_level": "required",
      "trust_strategy": "trust_system_ca_signed_certificates",
      "enable_bookmarks": true,
      "enable_routing": true,
      "enable_metrics": true
    }
  },
  "vector": {
    "type": "qdrant",
    "host": "localhost",
    "port": 6333,
    "collection": "memory_vectors",
    "dimension": 768,
    "distance_metric": "cosine",
    "index_type": "hnsw",
    "max_connections": 20,
    "batch_size": 100,
    "enable_caching": true,
    "qdrant": {
      "api_key": "${AI_MEMORY_QDRANT_API_KEY}",
      "enable_tls": true,
      "enable_grpc": true,
      "compression_level": "gzip",
      "default_vector_size": 768,
      "default_distance": "cosine",
      "on_disk_payload": true
    }
  }
}
```

### YAML Configuration
See `example_config.yaml` for comprehensive YAML examples.

## Programmatic Configuration

### Using ConfigFactory
```go
package main

import (
    "github.com/NortonBen/ai-memory-go/storage"
)

func main() {
    // Create factory
    factory := storage.NewConfigFactory()
    
    // Load from environment
    config, err := factory.LoadFromEnvironment()
    if err != nil {
        panic(err)
    }
    
    // Or load from file
    config, err = factory.LoadFromFile("config.yaml")
    if err != nil {
        panic(err)
    }
    
    // Or use predefined profile
    config, err = factory.GetProfile("cloud")
    if err != nil {
        panic(err)
    }
    
    // Validate configuration
    if err := factory.ValidateConfig(config); err != nil {
        panic(err)
    }
    
    // Use configuration to create storage
    // ... storage initialization code
}
```

### Custom Configuration
```go
// Create custom configuration
config := &storage.StorageConfig{
    Environment:        "custom",
    EnableTransactions: true,
    ConnectionTimeout:  30 * time.Second,
    QueryTimeout:       45 * time.Second,
    MaxRetries:         5,
    
    Relational: &storage.RelationalConfig{
        Type:           storage.StorageTypePostgreSQL,
        Host:           "db.example.com",
        Port:           5432,
        Database:       "ai_memory",
        Username:       "user",
        Password:       "password",
        SSLMode:        "require",
        MaxConnections: 100,
        MinConnections: 10,
        BatchSize:      1000,
    },
    
    Graph: &storage.GraphConfig{
        Type:           storage.GraphStoreTypeNeo4j,
        Host:           "graph.example.com",
        Port:           7687,
        Database:       "neo4j",
        Username:       "neo4j",
        Password:       "password",
        MaxConnections: 50,
        BatchSize:      500,
        EnableIndexing: true,
        EnableCaching:  true,
        Neo4j: &storage.Neo4jConfig{
            URI:             "bolt://graph.example.com:7687",
            EncryptionLevel: "required",
            TrustStrategy:   "trust_system_ca_signed_certificates",
            EnableBookmarks: true,
            EnableRouting:   true,
            EnableMetrics:   true,
        },
    },
    
    Vector: &storage.VectorConfig{
        Type:           storage.VectorStoreTypeQdrant,
        Host:           "vector.example.com",
        Port:           6333,
        Collection:     "memory_vectors",
        Dimension:      768,
        DistanceMetric: "cosine",
        IndexType:      "hnsw",
        MaxConnections: 30,
        BatchSize:      100,
        EnableCaching:  true,
        Qdrant: &storage.QdrantConfig{
            APIKey:           "your_api_key",
            EnableTLS:        true,
            EnableGRPC:       true,
            CompressionLevel: "gzip",
            DefaultDistance:  "cosine",
            OnDiskPayload:    true,
        },
    },
}
```

## Provider-Specific Configurations

### Neo4j Configuration
```go
neo4jConfig := &storage.Neo4jConfig{
    URI:                   "bolt://localhost:7687",
    Realm:                 "",
    UserAgent:             "ai-memory-go/1.0",
    MaxConnectionLife:     30 * time.Minute,
    MaxConnectionPool:     20,
    ConnectionTimeout:     30 * time.Second,
    SocketTimeout:         60 * time.Second,
    EncryptionLevel:       "required",
    TrustStrategy:         "trust_system_ca_signed_certificates",
    EnableBookmarks:       true,
    EnableRouting:         true,
    EnableMetrics:         true,
    DefaultDatabase:       "neo4j",
    ImpersonatedUser:      "",
}
```

### SurrealDB Configuration
```go
surrealConfig := &storage.SurrealDBConfig{
    Endpoint:           "ws://localhost:8000/rpc",
    Namespace:          "ai_memory",
    Database:           "memory",
    Scope:              "",
    Username:           "root",
    Password:           "password",
    Token:              "",
    Timeout:            30 * time.Second,
    MaxConnections:     15,
    EnableTLS:          false,
    EnableLivequeries:  true,
    EnableTransactions: true,
    StrictMode:         false,
}
```

### Kuzu Configuration
```go
kuzuConfig := &storage.KuzuConfig{
    DatabasePath:          "./data/graph.kuzu",
    BufferPoolSize:        256 * 1024 * 1024, // 256MB
    MaxNumThreads:         4,
    EnableCompression:     true,
    CheckpointWaitTimeout: 30 * time.Second,
    EnableCheckpoint:      true,
    LogLevel:              "info",
    HashJoinSizeRatio:     1.5,
    EnableSemiMask:        true,
    EnableZoneMap:         true,
    EnableProgressBar:     false,
}
```

### Qdrant Configuration
```go
qdrantConfig := &storage.QdrantConfig{
    Host:              "localhost",
    Port:              6333,
    APIKey:            "your_api_key",
    Timeout:           30 * time.Second,
    MaxConnections:    20,
    EnableTLS:         true,
    EnableGRPC:        true,
    GRPCPort:          6334,
    CompressionLevel:  "gzip",
    DefaultVectorSize: 768,
    DefaultDistance:   "cosine",
    OnDiskPayload:     true,
    MaxSegmentSize:    4 * 1024 * 1024, // 4MB
    MemmapThreshold:   1024 * 1024,     // 1MB
    IndexingThreshold: 20000,
}
```

### LanceDB Configuration
```go
lanceConfig := &storage.LanceDBConfig{
    URI:                "./data/vectors.lance",
    StorageType:        "local",
    DataDir:            "./data",
    TableName:          "embeddings",
    BlockSize:          8192,
    MaxRowsPerFile:     1000000,
    MaxRowsPerGroup:    10000,
    MaxBytesPerFile:    1024 * 1024 * 1024, // 1GB
    IndexCacheSize:     100,
    MetricType:         "cosine",
    EnableStatistics:   true,
    EnableBloomFilter:  true,
}
```

### pgvector Configuration
```go
pgvectorConfig := &storage.PgVectorConfig{
    Host:               "localhost",
    Port:               5432,
    Database:           "vectors",
    Username:           "vector_user",
    Password:           "password",
    SSLMode:            "require",
    MaxConnections:     15,
    MinConnections:     2,
    MaxConnLifetime:    1 * time.Hour,
    MaxConnIdleTime:    10 * time.Minute,
    HealthCheckPeriod:  1 * time.Minute,
    VectorDimensions:   768,
    IndexType:          "hnsw",
    TableName:          "embeddings",
    VectorColumn:       "embedding",
    MetadataColumn:     "metadata",
    MaintenanceWorkMem: "256MB",
    MaxParallelWorkers: 4,
    EffectiveCacheSize: "1GB",
}
```

## TLS Configuration

For secure connections, configure TLS settings:

```go
tlsConfig := &storage.TLSConfig{
    Enabled:            true,
    CertFile:           "/path/to/cert.pem",
    KeyFile:            "/path/to/key.pem",
    CAFile:             "/path/to/ca.pem",
    ServerName:         "example.com",
    InsecureSkipVerify: false,
    MinVersion:         "1.2",
    MaxVersion:         "1.3",
    CipherSuites:       []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
}
```

## Configuration Validation

The system provides comprehensive validation:

```go
factory := storage.NewConfigFactory()
config := storage.DefaultStorageConfig()

// Validate configuration
if err := factory.ValidateConfig(config); err != nil {
    log.Fatalf("Invalid configuration: %v", err)
}
```

Validation checks include:
- Required fields are present
- Connection parameters are valid
- Resource limits are positive
- Provider-specific settings are consistent
- TLS configuration is valid

## Best Practices

### Security
1. **Use environment variables** for sensitive data (passwords, API keys)
2. **Enable TLS** for production deployments
3. **Use strong authentication** methods
4. **Rotate credentials** regularly
5. **Limit connection permissions** to minimum required

### Performance
1. **Configure connection pooling** appropriately for your workload
2. **Set reasonable timeouts** to prevent hanging connections
3. **Enable caching** where appropriate
4. **Use batch operations** for bulk data operations
5. **Monitor resource usage** and adjust limits accordingly

### Reliability
1. **Enable transactions** for data consistency
2. **Configure retries** with exponential backoff
3. **Set up health checks** for all storage backends
4. **Use clustering** for high availability in production
5. **Implement proper backup strategies**

### Development
1. **Use file-based profile** for local development
2. **Use in-memory stores** for testing
3. **Validate configurations** before deployment
4. **Use configuration profiles** for different environments
5. **Document custom configurations** thoroughly

## Migration Guide

### From Single Backend to Multi-Backend
```go
// Old single backend configuration
oldConfig := map[string]interface{}{
    "database_url": "postgresql://user:pass@localhost/db",
}

// New multi-backend configuration
newConfig := &storage.StorageConfig{
    Relational: &storage.RelationalConfig{
        Type:     storage.StorageTypePostgreSQL,
        Host:     "localhost",
        Database: "db",
        Username: "user",
        Password: "pass",
    },
    Graph: storage.DefaultGraphConfig(),
    Vector: storage.DefaultVectorConfig(),
}
```

### Environment Variable Migration
```bash
# Old environment variables
DATABASE_URL=postgresql://user:pass@localhost/db

# New environment variables
AI_MEMORY_RELATIONAL_TYPE=postgresql
AI_MEMORY_RELATIONAL_HOST=localhost
AI_MEMORY_RELATIONAL_DATABASE=db
AI_MEMORY_RELATIONAL_USERNAME=user
AI_MEMORY_RELATIONAL_PASSWORD=pass
```

## Troubleshooting

### Common Issues

1. **Connection Failures**
   - Check network connectivity
   - Verify credentials
   - Ensure services are running
   - Check firewall settings

2. **Performance Issues**
   - Increase connection pool size
   - Adjust timeout values
   - Enable caching
   - Optimize batch sizes

3. **Configuration Errors**
   - Validate configuration syntax
   - Check required fields
   - Verify environment variables
   - Test with minimal configuration

### Debug Mode
Enable debug logging to troubleshoot configuration issues:

```go
config := storage.DefaultStorageConfig()
config.Environment = "development"
// Debug logging will be enabled automatically
```

### Health Checks
Implement health checks for all storage backends:

```go
// Check storage health
if err := storage.Health(ctx); err != nil {
    log.Printf("Storage health check failed: %v", err)
}
```