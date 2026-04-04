package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/vector"
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	// Register the sqlite-vec extension with the go-sqlite3 driver.
	sqlite_vec.Auto()

	vector.RegisterStore(vector.StoreTypeSQLite, func(config *vector.VectorConfig) (vector.VectorStore, error) {
		return NewSQLiteVectorStore(config)
	})
}

// SQLiteVectorStore implements VectorStore using a dedicated SQLite DB file
// with the sqlite-vec extension for KNN vector search.
// Use a separate DB file from SQLiteGraphStore for independent I/O performance.
type SQLiteVectorStore struct {
	db        *sql.DB
	path      string
	dimension int
}

type knnRow struct {
	id       string
	metadata map[string]interface{}
	distance float64
}

// NewSQLiteVectorStore opens (or creates) a SQLite+sqlite-vec database at path.
// dimension is the size of the embedding vectors.
func NewSQLiteVectorStore(config *vector.VectorConfig) (*SQLiteVectorStore, error) {
	path := config.Database // Using Database field for file path
	dimension := config.Dimension

	if dimension <= 0 {
		dimension = 768
	}
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("sqlite vec open: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &SQLiteVectorStore{db: db, path: path, dimension: dimension}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite vec migrate: %w", err)
	}
	return s, nil
}

func (s *SQLiteVectorStore) migrate() error {
	stmts := []string{
		// sqlite-vec virtual table for KNN search
		fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS vec_items
			USING vec0(embedding float[%d])`, s.dimension),
		// ID ↔ rowid mapping + JSON metadata
		`CREATE TABLE IF NOT EXISTS vec_meta (
			id         TEXT PRIMARY KEY,
			rowid      INTEGER UNIQUE,
			metadata   TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_vec_meta_rowid ON vec_meta(rowid)`,
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w\n  SQL: %s", err, stmt)
		}
	}
	return tx.Commit()
}

// Health implements VectorStore.
func (s *SQLiteVectorStore) Health(_ context.Context) error { return s.db.Ping() }

// Close implements VectorStore.
func (s *SQLiteVectorStore) Close() error { return s.db.Close() }

// ─── CRUD ─────────────────────────────────────────────────────────────────────

func (s *SQLiteVectorStore) StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error {
	if err := s.validateVectorDim(embedding); err != nil {
		return err
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	now := time.Now()

	// Check if id already exists
	var existingRowid int64
	err = s.db.QueryRowContext(ctx, `SELECT rowid FROM vec_meta WHERE id=?`, id).Scan(&existingRowid)

	tx, txErr := s.db.BeginTx(ctx, nil)
	if txErr != nil {
		return txErr
	}
	defer tx.Rollback()

	if err != nil {
		// New — insert into vec_items (auto rowid), then record mapping
		res, err := tx.ExecContext(ctx, `INSERT INTO vec_items(embedding) VALUES(?)`, f32ToBlob(embedding))
		if err != nil {
			return fmt.Errorf("vec_items insert: %w", err)
		}
		newRowid, _ := res.LastInsertId()
		_, err = tx.ExecContext(ctx,
			`INSERT INTO vec_meta(id, rowid, metadata, created_at, updated_at) VALUES(?,?,?,?,?)`,
			id, newRowid, string(metaJSON), now, now)
		if err != nil {
			return fmt.Errorf("vec_meta insert: %w", err)
		}
	} else {
		// Update existing
		if _, err := tx.ExecContext(ctx,
			`UPDATE vec_items SET embedding=? WHERE rowid=?`, f32ToBlob(embedding), existingRowid); err != nil {
			return fmt.Errorf("vec_items update: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE vec_meta SET metadata=?, updated_at=? WHERE id=?`, string(metaJSON), now, id); err != nil {
			return fmt.Errorf("vec_meta update: %w", err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteVectorStore) GetEmbedding(ctx context.Context, id string) ([]float32, map[string]interface{}, error) {
	var rowid int64
	var metaJSON string
	if err := s.db.QueryRowContext(ctx, `SELECT rowid, metadata FROM vec_meta WHERE id=?`, id).Scan(&rowid, &metaJSON); err != nil {
		return nil, nil, fmt.Errorf("embedding not found: %s", id)
	}
	var blob []byte
	if err := s.db.QueryRowContext(ctx, `SELECT embedding FROM vec_items WHERE rowid=?`, rowid).Scan(&blob); err != nil {
		return nil, nil, fmt.Errorf("vec_items read: %w", err)
	}
	var meta map[string]interface{}
	json.Unmarshal([]byte(metaJSON), &meta)
	return blobToF32(blob), meta, nil
}

func (s *SQLiteVectorStore) UpdateEmbedding(ctx context.Context, id string, embedding []float32) error {
	if err := s.validateVectorDim(embedding); err != nil {
		return err
	}
	var rowid int64
	if err := s.db.QueryRowContext(ctx, `SELECT rowid FROM vec_meta WHERE id=?`, id).Scan(&rowid); err != nil {
		return fmt.Errorf("embedding not found: %s", id)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE vec_items SET embedding=? WHERE rowid=?`, f32ToBlob(embedding), rowid)
	return err
}

func (s *SQLiteVectorStore) DeleteEmbedding(ctx context.Context, id string) error {
	var rowid int64
	if err := s.db.QueryRowContext(ctx, `SELECT rowid FROM vec_meta WHERE id=?`, id).Scan(&rowid); err != nil {
		return nil // not found = no-op
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.ExecContext(ctx, `DELETE FROM vec_items WHERE rowid=?`, rowid)
	tx.ExecContext(ctx, `DELETE FROM vec_meta WHERE id=?`, id)
	return tx.Commit()
}

func (s *SQLiteVectorStore) StoreBatchEmbeddings(ctx context.Context, embeddings []*vector.EmbeddingData) error {
	for _, emb := range embeddings {
		if err := s.StoreEmbedding(ctx, emb.ID, emb.Embedding, emb.Metadata); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteVectorStore) DeleteBatchEmbeddings(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if err := s.DeleteEmbedding(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ─── Search ───────────────────────────────────────────────────────────────────

func (s *SQLiteVectorStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*vector.SimilarityResult, error) {
	return s.SimilaritySearchWithFilter(ctx, queryEmbedding, nil, limit, threshold)
}

func (s *SQLiteVectorStore) SimilaritySearchWithFilter(ctx context.Context, queryEmbedding []float32, filters map[string]interface{}, limit int, threshold float64) ([]*vector.SimilarityResult, error) {
	if err := s.validateVectorDim(queryEmbedding); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	// When filters are applied, fetch a wider KNN window to avoid filtering away
	// near candidates and missing valid matches deeper in the ranking.
	candidateLimit := limit
	if len(filters) > 0 {
		candidateLimit = max(limit*4, limit+10)
	}
	maxCandidateLimit := 5000

	var candidates []knnRow
	for {
		rows, err := s.knnSearch(ctx, queryEmbedding, candidateLimit)
		if err != nil {
			return nil, err
		}
		candidates = rows

		if len(filters) == 0 {
			break
		}
		matched := 0
		for _, r := range candidates {
			if vecMatchesFilters(r.metadata, filters) {
				score := 1.0 / (1.0 + r.distance)
				if score >= threshold {
					matched++
				}
			}
		}
		if matched >= limit || len(candidates) < candidateLimit || candidateLimit >= maxCandidateLimit {
			break
		}
		candidateLimit = min(candidateLimit*2, maxCandidateLimit)
	}

	var results []*vector.SimilarityResult
	for _, r := range candidates {
		score := 1.0 / (1.0 + r.distance)
		if score < threshold {
			continue
		}
		if !vecMatchesFilters(r.metadata, filters) {
			continue
		}
		results = append(results, &vector.SimilarityResult{
			ID:       r.id,
			Score:    score,
			Metadata: r.metadata,
			Distance: r.distance,
		})
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

// ─── Collection Management (SQLite = single-table, lightweight) ───────────────

func (s *SQLiteVectorStore) CreateCollection(_ context.Context, _ string, _ int, _ *vector.CollectionConfig) error {
	return nil
}

func (s *SQLiteVectorStore) DeleteCollection(ctx context.Context, _ string) error {
	s.db.ExecContext(ctx, `DELETE FROM vec_items`)
	s.db.ExecContext(ctx, `DELETE FROM vec_meta`)
	return nil
}

func (s *SQLiteVectorStore) ListCollections(_ context.Context) ([]string, error) {
	return []string{"default"}, nil
}

func (s *SQLiteVectorStore) GetCollectionInfo(ctx context.Context, name string) (*vector.CollectionInfo, error) {
	var count int64
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vec_meta`).Scan(&count)
	return &vector.CollectionInfo{
		Name:        name,
		Dimension:   s.dimension,
		VectorCount: count,
		Status:      "ready",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (s *SQLiteVectorStore) GetEmbeddingCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vec_meta`).Scan(&count)
	return count, err
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func f32ToBlob(v []float32) []byte {
	b := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

func blobToF32(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func vecMatchesFilters(metadata map[string]interface{}, filters map[string]interface{}) bool {
	if len(filters) == 0 {
		return true
	}
	for k, v := range filters {
		mv, ok := metadata[k]
		if !ok || !strings.EqualFold(fmt.Sprintf("%v", mv), fmt.Sprintf("%v", v)) {
			return false
		}
	}
	return true
}

func (s *SQLiteVectorStore) validateVectorDim(v []float32) error {
	if len(v) != s.dimension {
		return fmt.Errorf("invalid embedding dimension: expected %d, got %d", s.dimension, len(v))
	}
	return nil
}

func (s *SQLiteVectorStore) knnSearch(ctx context.Context, queryEmbedding []float32, limit int) ([]knnRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.metadata, v.distance
		FROM vec_items v
		JOIN vec_meta m ON m.rowid = v.rowid
		WHERE v.embedding MATCH ? AND k = ?
		ORDER BY v.distance`,
		f32ToBlob(queryEmbedding), limit)
	if err != nil {
		return nil, fmt.Errorf("knn search: %w", err)
	}
	defer rows.Close()

	out := make([]knnRow, 0, limit)
	for rows.Next() {
		var id, metaJSON string
		var distance float64
		if err := rows.Scan(&id, &metaJSON, &distance); err != nil {
			return nil, err
		}
		var meta map[string]interface{}
		_ = json.Unmarshal([]byte(metaJSON), &meta)
		if meta == nil {
			meta = make(map[string]interface{})
		}
		out = append(out, knnRow{
			id:       id,
			metadata: meta,
			distance: distance,
		})
	}
	return out, rows.Err()
}
