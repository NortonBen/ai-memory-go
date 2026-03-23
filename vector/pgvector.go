package vector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pgvector/pgvector-go"
)

type PgVectorStore struct {
	db        *sql.DB
	tableName string
	dimension int
}

func NewPgVectorStore(config *VectorConfig) (*PgVectorStore, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.Username, config.Password, config.Database)

	// We use the 'pgx' driver, imported via stdlib package
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(config.MaxConnections)
	db.SetConnMaxIdleTime(config.IdleTimeout)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	tableName := "vector_embeddings"
	if config.Collection != "" {
		tableName = config.Collection
	}

	return &PgVectorStore{
		db:        db,
		tableName: tableName,
		dimension: config.Dimension,
	}, nil
}

func (s *PgVectorStore) StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error {
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (id, embedding, metadata, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET 
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`, s.tableName)

	now := time.Now()
	_, err = s.db.ExecContext(ctx, query, id, pgvector.NewVector(embedding), metaJSON, now, now)
	if err != nil {
		return fmt.Errorf("failed to store embedding: %w", err)
	}
	return nil
}

func (s *PgVectorStore) GetEmbedding(ctx context.Context, id string) ([]float32, map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT embedding, metadata FROM %s WHERE id = $1", s.tableName)
	var vec pgvector.Vector
	var metaJSON []byte
	err := s.db.QueryRowContext(ctx, query, id).Scan(&vec, &metaJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, fmt.Errorf("embedding not found: %s", id)
		}
		return nil, nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	var metadata map[string]interface{}
	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &metadata); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}
	return vec.Slice(), metadata, nil
}

func (s *PgVectorStore) UpdateEmbedding(ctx context.Context, id string, embedding []float32) error {
	query := fmt.Sprintf("UPDATE %s SET embedding = $1, updated_at = $2 WHERE id = $3", s.tableName)
	res, err := s.db.ExecContext(ctx, query, pgvector.NewVector(embedding), time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update embedding: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("embedding not found: %s", id)
	}
	return nil
}

func (s *PgVectorStore) DeleteEmbedding(ctx context.Context, id string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", s.tableName)
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *PgVectorStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*SimilarityResult, error) {
	// vector distance operator: <=> for cosine distance
	query := fmt.Sprintf(`
		SELECT id, embedding, metadata, embedding <=> $1 as distance 
		FROM %s 
		WHERE (1 - (embedding <=> $1)) >= $2
		ORDER BY embedding <=> $1 
		LIMIT $3
	`, s.tableName)

	rows, err := s.db.QueryContext(ctx, query, pgvector.NewVector(queryEmbedding), threshold, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search similarity: %w", err)
	}
	defer rows.Close()

	var results []*SimilarityResult
	for rows.Next() {
		var id string
		var vec pgvector.Vector
		var metaJSON []byte
		var distance float64
		if err := rows.Scan(&id, &vec, &metaJSON, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan similarity result: %w", err)
		}

		var metadata map[string]interface{}
		if len(metaJSON) > 0 {
			if err := json.Unmarshal(metaJSON, &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		results = append(results, &SimilarityResult{
			ID:        id,
			Score:     1 - distance, // Cosine similarity is 1 - Cosine distance
			Embedding: vec.Slice(),
			Metadata:  metadata,
			Distance:  distance,
		})
	}
	return results, nil
}

func (s *PgVectorStore) SimilaritySearchWithFilter(ctx context.Context, queryEmbedding []float32, filters map[string]interface{}, limit int, threshold float64) ([]*SimilarityResult, error) {
	var filterConds []string
	var args []interface{}

	args = append(args, pgvector.NewVector(queryEmbedding), threshold, limit)
	argIdx := 4

	for k, v := range filters {
		filterJSON, err := json.Marshal(map[string]interface{}{k: v})
		if err == nil {
			filterConds = append(filterConds, fmt.Sprintf("metadata @> $%d", argIdx))
			args = append(args, filterJSON)
			argIdx++
		}
	}

	filterStr := ""
	if len(filterConds) > 0 {
		filterStr = "AND " + strings.Join(filterConds, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, embedding, metadata, embedding <=> $1 as distance 
		FROM %s 
		WHERE (1 - (embedding <=> $1)) >= $2 %s
		ORDER BY embedding <=> $1 
		LIMIT $3
	`, s.tableName, filterStr)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search similarity with filters: %w", err)
	}
	defer rows.Close()

	var results []*SimilarityResult
	for rows.Next() {
		var id string
		var vec pgvector.Vector
		var metaJSON []byte
		var distance float64
		if err := rows.Scan(&id, &vec, &metaJSON, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan target: %w", err)
		}

		var metadata map[string]interface{}
		if len(metaJSON) > 0 {
			if err := json.Unmarshal(metaJSON, &metadata); err != nil {
				return nil, fmt.Errorf("parse err: %w", err)
			}
		}

		results = append(results, &SimilarityResult{
			ID:        id,
			Score:     1 - distance,
			Embedding: vec.Slice(),
			Metadata:  metadata,
			Distance:  distance,
		})
	}
	return results, nil
}

func (s *PgVectorStore) StoreBatchEmbeddings(ctx context.Context, embeddings []*EmbeddingData) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := fmt.Sprintf(`
		INSERT INTO %s (id, embedding, metadata, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET 
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`, s.tableName)

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, emb := range embeddings {
		metaJSON, _ := json.Marshal(emb.Metadata)
		now := time.Now()
		_, err := stmt.ExecContext(ctx, emb.ID, pgvector.NewVector(emb.Embedding), metaJSON, now, now)
		if err != nil {
			return fmt.Errorf("batch store err: %w", err)
		}
	}
	return tx.Commit()
}

func (s *PgVectorStore) DeleteBatchEmbeddings(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	args := make([]interface{}, len(ids))
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", s.tableName, strings.Join(placeholders, ","))
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *PgVectorStore) CreateCollection(ctx context.Context, name string, dimension int, config *CollectionConfig) error {
	_, err := s.db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}

	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(255) PRIMARY KEY,
			embedding vector(%d),
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)
	`, name, dimension)
	_, err = s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create collection table: %w", err)
	}

	indexType := "hnsw"
	if config != nil && config.IndexType != "" {
		indexType = config.IndexType
	}

	distanceMetric := "vector_cosine_ops"
	if config != nil && config.DistanceMetric == "l2" {
		distanceMetric = "vector_l2_ops"
	} else if config != nil && config.DistanceMetric == "dot" {
		distanceMetric = "vector_ip_ops"
	}

	indexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_embedding_idx ON %s USING %s (embedding %s)
	`, name, name, indexType, distanceMetric)

	_, err = s.db.ExecContext(ctx, indexQuery)
	if err != nil {
		return fmt.Errorf("failed to create vector index: %w", err)
	}

	metaIndexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_metadata_idx ON %s USING GIN (metadata)", name, name)
	_, _ = s.db.ExecContext(ctx, metaIndexQuery)

	return nil
}

func (s *PgVectorStore) DeleteCollection(ctx context.Context, name string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", name)
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *PgVectorStore) ListCollections(ctx context.Context) ([]string, error) {
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_type = 'BASE TABLE'
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, nil
}

func (s *PgVectorStore) GetCollectionInfo(ctx context.Context, name string) (*CollectionInfo, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", name)
	err := s.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return nil, err
	}

	return &CollectionInfo{
		Name:        name,
		Dimension:   s.dimension,
		VectorCount: count,
		Status:      "ready",
	}, nil
}

func (s *PgVectorStore) GetEmbeddingCount(ctx context.Context) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.tableName)
	err := s.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

func (s *PgVectorStore) Health(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PgVectorStore) Close() error {
	return s.db.Close()
}
