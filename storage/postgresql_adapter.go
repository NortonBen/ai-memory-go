package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	_ "github.com/lib/pq"
)

// PostgresAdapter implements RelationalStore using PostgreSQL
type PostgresAdapter struct {
	db     *sql.DB
	config *RelationalConfig
}

// NewPostgresAdapter creates a new PostgreSQL adapter
func NewPostgresAdapter(config *RelationalConfig) (*PostgresAdapter, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.Username, config.Password, config.Database, config.SSLMode)

	if config.SSLMode == "" {
		connStr += " sslmode=disable"
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Apply connection pool settings
	if config.MaxConnections > 0 {
		db.SetMaxOpenConns(config.MaxConnections)
	}
	if config.MinConnections > 0 {
		db.SetMaxIdleConns(config.MinConnections) // sql.DB doesn't have SetMinIdleConns, use MaxIdleConns as proxy
	}
	if config.MaxLifetime > 0 {
		db.SetConnMaxLifetime(config.MaxLifetime)
	}
	if config.IdleTimeout > 0 {
		db.SetConnMaxIdleTime(config.IdleTimeout)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	adapter := &PostgresAdapter{
		db:     db,
		config: config,
	}

	if err := adapter.setupTables(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to setup tables: %w", err)
	}

	return adapter, nil
}

func (pa *PostgresAdapter) setupTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS memory_sessions (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255),
			context JSONB,
			created_at TIMESTAMP WITH TIME ZONE,
			last_access TIMESTAMP WITH TIME ZONE,
			is_active BOOLEAN,
			expires_at TIMESTAMP WITH TIME ZONE
		)`,
		`CREATE TABLE IF NOT EXISTS datapoints (
			id VARCHAR(255) PRIMARY KEY,
			content TEXT,
			content_type VARCHAR(255),
			metadata JSONB,
			session_id VARCHAR(255),
			user_id VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE,
			updated_at TIMESTAMP WITH TIME ZONE,
			processing_status VARCHAR(50),
			error_message TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_datapoints_session_id ON datapoints(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_datapoints_user_id ON datapoints(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_sessions_user_id ON memory_sessions(user_id)`,
	}

	if pa.config.EnableJSONIndexes {
		queries = append(queries, `CREATE INDEX IF NOT EXISTS idx_datapoints_metadata ON datapoints USING GIN (metadata)`)
		queries = append(queries, `CREATE INDEX IF NOT EXISTS idx_sessions_context ON memory_sessions USING GIN (context)`)
	}

	if pa.config.EnableFullText {
		// PostgreSQL uses specialized type tsvector for full text search
		queries = append(queries, `ALTER TABLE datapoints ADD COLUMN IF NOT EXISTS content_fts tsvector GENERATED ALWAYS AS (to_tsvector('english', content)) STORED`)
		queries = append(queries, `CREATE INDEX IF NOT EXISTS idx_datapoints_content_fts ON datapoints USING GIN (content_fts)`)
	}

	for _, query := range queries {
		if _, err := pa.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute setup query %q: %w", query, err)
		}
	}

	return nil
}

// StoreDataPoint implements RelationalStore
func (pa *PostgresAdapter) StoreDataPoint(ctx context.Context, dp *schema.DataPoint) error {
	metadataBytes, err := json.Marshal(dp.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO datapoints (id, content, content_type, metadata, session_id, user_id, 
			created_at, updated_at, processing_status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			content_type = EXCLUDED.content_type,
			metadata = EXCLUDED.metadata,
			session_id = EXCLUDED.session_id,
			user_id = EXCLUDED.user_id,
			updated_at = EXCLUDED.updated_at,
			processing_status = EXCLUDED.processing_status,
			error_message = EXCLUDED.error_message
	`

	_, err = pa.db.ExecContext(ctx, query,
		dp.ID, dp.Content, dp.ContentType, metadataBytes, dp.SessionID, dp.UserID,
		dp.CreatedAt, dp.UpdatedAt, dp.ProcessingStatus, dp.ErrorMessage)

	return err
}

// GetDataPoint implements RelationalStore
func (pa *PostgresAdapter) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) {
	query := `
		SELECT id, content, content_type, metadata, session_id, user_id,
			created_at, updated_at, processing_status, error_message
		FROM datapoints WHERE id = $1
	`
	row := pa.db.QueryRowContext(ctx, query, id)

	var dp schema.DataPoint
	var metadataBytes []byte

	err := row.Scan(
		&dp.ID, &dp.Content, &dp.ContentType, &metadataBytes, &dp.SessionID, &dp.UserID,
		&dp.CreatedAt, &dp.UpdatedAt, &dp.ProcessingStatus, &dp.ErrorMessage)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("datapoint not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &dp.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	} else {
		dp.Metadata = make(map[string]interface{})
	}

	return &dp, nil
}

// UpdateDataPoint implements RelationalStore
func (pa *PostgresAdapter) UpdateDataPoint(ctx context.Context, dp *schema.DataPoint) error {
	metadataBytes, err := json.Marshal(dp.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE datapoints SET
			content = $1, content_type = $2, metadata = $3, session_id = $4, user_id = $5,
			updated_at = $6, processing_status = $7, error_message = $8
		WHERE id = $9
	`
	result, err := pa.db.ExecContext(ctx, query,
		dp.Content, dp.ContentType, metadataBytes, dp.SessionID, dp.UserID,
		time.Now(), dp.ProcessingStatus, dp.ErrorMessage, dp.ID)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("datapoint not found: %s", dp.ID)
	}

	return nil
}

// DeleteDataPoint implements RelationalStore
func (pa *PostgresAdapter) DeleteDataPoint(ctx context.Context, id string) error {
	query := `DELETE FROM datapoints WHERE id = $1`
	result, err := pa.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("datapoint not found: %s", id)
	}

	return nil
}

// DeleteDataPointsBySession implements RelationalStore
func (pa *PostgresAdapter) DeleteDataPointsBySession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM datapoints WHERE session_id = $1`
	_, err := pa.db.ExecContext(ctx, query, sessionID)
	return err
}

// QueryDataPoints implements RelationalStore
func (pa *PostgresAdapter) QueryDataPoints(ctx context.Context, q *DataPointQuery) ([]*schema.DataPoint, error) {
	if q == nil {
		q = DefaultDataPointQuery()
	}

	queryStr := `
		SELECT id, content, content_type, metadata, session_id, user_id,
			created_at, updated_at, processing_status, error_message
		FROM datapoints WHERE 1=1
	`
	args := []interface{}{}
	argId := 1

	if q.SessionID != "" {
		queryStr += fmt.Sprintf(" AND session_id = $%d", argId)
		args = append(args, q.SessionID)
		argId++
	}
	if q.UserID != "" {
		queryStr += fmt.Sprintf(" AND user_id = $%d", argId)
		args = append(args, q.UserID)
		argId++
	}
	if q.ContentType != "" {
		queryStr += fmt.Sprintf(" AND content_type = $%d", argId)
		args = append(args, q.ContentType)
		argId++
	}
	if q.SearchText != "" {
		if pa.config.EnableFullText && (q.SearchMode == "fulltext" || q.SearchMode == "") {
			queryStr += fmt.Sprintf(" AND content_fts @@ plainto_tsquery('english', $%d)", argId)
			args = append(args, q.SearchText)
			argId++
		} else {
			queryStr += fmt.Sprintf(" AND content ILIKE $%d", argId)
			args = append(args, "%"+q.SearchText+"%")
			argId++
		}
	}
	if q.CreatedAfter != nil {
		queryStr += fmt.Sprintf(" AND created_at > $%d", argId)
		args = append(args, *q.CreatedAfter)
		argId++
	}

	if q.SortBy != "" {
		order := "ASC"
		if strings.ToLower(q.SortOrder) == "desc" {
			order = "DESC"
		}
		// Basic prevention against sql injection for order by
		validSortCol := map[string]bool{"created_at": true, "updated_at": true, "id": true}
		if validSortCol[q.SortBy] {
			queryStr += fmt.Sprintf(" ORDER BY %s %s", q.SortBy, order)
		} else {
			queryStr += fmt.Sprintf(" ORDER BY created_at %s", order)
		}
	} else {
		queryStr += " ORDER BY created_at DESC"
	}

	if q.Limit > 0 {
		queryStr += fmt.Sprintf(" LIMIT $%d", argId)
		args = append(args, q.Limit)
		argId++
	}
	if q.Offset > 0 {
		queryStr += fmt.Sprintf(" OFFSET $%d", argId)
		args = append(args, q.Offset)
		argId++
	}

	rows, err := pa.db.QueryContext(ctx, queryStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*schema.DataPoint
	for rows.Next() {
		dp := &schema.DataPoint{}
		var metadataBytes []byte
		err := rows.Scan(
			&dp.ID, &dp.Content, &dp.ContentType, &metadataBytes, &dp.SessionID, &dp.UserID,
			&dp.CreatedAt, &dp.UpdatedAt, &dp.ProcessingStatus, &dp.ErrorMessage)
		if err != nil {
			return nil, err
		}
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &dp.Metadata); err != nil {
				return nil, err
			}
		} else {
			dp.Metadata = make(map[string]interface{})
		}
		results = append(results, dp)
	}

	return results, nil
}

// SearchDataPoints implements RelationalStore
func (pa *PostgresAdapter) SearchDataPoints(ctx context.Context, searchQuery string, filters map[string]interface{}) ([]*schema.DataPoint, error) {
	q := DefaultDataPointQuery()
	q.SearchText = searchQuery
	q.Filters = filters
	return pa.QueryDataPoints(ctx, q)
}

// StoreSession implements RelationalStore
func (pa *PostgresAdapter) StoreSession(ctx context.Context, session *schema.MemorySession) error {
	contextBytes, err := json.Marshal(session.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal session context: %w", err)
	}

	query := `
		INSERT INTO memory_sessions (id, user_id, context, created_at, last_access, is_active, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			context = EXCLUDED.context,
			last_access = EXCLUDED.last_access,
			is_active = EXCLUDED.is_active,
			expires_at = EXCLUDED.expires_at
	`
	_, err = pa.db.ExecContext(ctx, query,
		session.ID, session.UserID, contextBytes, session.CreatedAt, 
		session.LastAccess, session.IsActive, session.ExpiresAt)

	return err
}

// GetSession implements RelationalStore
func (pa *PostgresAdapter) GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error) {
	query := `
		SELECT id, user_id, context, created_at, last_access, is_active, expires_at
		FROM memory_sessions WHERE id = $1
	`
	row := pa.db.QueryRowContext(ctx, query, sessionID)

	var ms schema.MemorySession
	var contextBytes []byte

	err := row.Scan(
		&ms.ID, &ms.UserID, &contextBytes, &ms.CreatedAt, 
		&ms.LastAccess, &ms.IsActive, &ms.ExpiresAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, err
	}

	if len(contextBytes) > 0 {
		if err := json.Unmarshal(contextBytes, &ms.Context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}
	} else {
		ms.Context = make(map[string]interface{})
	}

	return &ms, nil
}

// UpdateSession implements RelationalStore
func (pa *PostgresAdapter) UpdateSession(ctx context.Context, session *schema.MemorySession) error {
	contextBytes, err := json.Marshal(session.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal session context: %w", err)
	}

	query := `
		UPDATE memory_sessions SET
			user_id = $1, context = $2, last_access = $3, is_active = $4, expires_at = $5
		WHERE id = $6
	`
	result, err := pa.db.ExecContext(ctx, query,
		session.UserID, contextBytes, time.Now(), session.IsActive, session.ExpiresAt, session.ID)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	return nil
}

// DeleteSession implements RelationalStore
func (pa *PostgresAdapter) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM memory_sessions WHERE id = $1`
	result, err := pa.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// ListSessions implements RelationalStore
func (pa *PostgresAdapter) ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error) {
	query := `
		SELECT id, user_id, context, created_at, last_access, is_active, expires_at
		FROM memory_sessions WHERE user_id = $1 ORDER BY last_access DESC
	`
	rows, err := pa.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*schema.MemorySession
	for rows.Next() {
		ms := &schema.MemorySession{}
		var contextBytes []byte
		err := rows.Scan(
			&ms.ID, &ms.UserID, &contextBytes, &ms.CreatedAt, 
			&ms.LastAccess, &ms.IsActive, &ms.ExpiresAt)
		if err != nil {
			return nil, err
		}
		if len(contextBytes) > 0 {
			if err := json.Unmarshal(contextBytes, &ms.Context); err != nil {
				return nil, err
			}
		} else {
			ms.Context = make(map[string]interface{})
		}
		sessions = append(sessions, ms)
	}

	return sessions, nil
}

// StoreBatch implements RelationalStore
func (pa *PostgresAdapter) StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error {
	tx, err := pa.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	query := `
		INSERT INTO datapoints (id, content, content_type, metadata, session_id, user_id, 
			created_at, updated_at, processing_status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			content_type = EXCLUDED.content_type,
			metadata = EXCLUDED.metadata,
			session_id = EXCLUDED.session_id,
			user_id = EXCLUDED.user_id,
			updated_at = EXCLUDED.updated_at,
			processing_status = EXCLUDED.processing_status,
			error_message = EXCLUDED.error_message
	`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, dp := range dataPoints {
		metadataBytes, err := json.Marshal(dp.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata for dp %s: %w", dp.ID, err)
		}
		_, err = stmt.ExecContext(ctx,
			dp.ID, dp.Content, dp.ContentType, metadataBytes, dp.SessionID, dp.UserID,
			dp.CreatedAt, dp.UpdatedAt, dp.ProcessingStatus, dp.ErrorMessage)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteBatch implements RelationalStore
func (pa *PostgresAdapter) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// For efficiency, use ANY in postgres
	query := `DELETE FROM datapoints WHERE id = ANY($1)`
	
	// Format ids into postgres array literal {id1,id2}
	arrayLiteral := "{" + strings.Join(ids, ",") + "}"

	_, err := pa.db.ExecContext(ctx, query, arrayLiteral)
	return err
}

// GetDataPointCount implements RelationalStore
func (pa *PostgresAdapter) GetDataPointCount(ctx context.Context) (int64, error) {
	var count int64
	err := pa.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM datapoints`).Scan(&count)
	return count, err
}

// GetSessionCount implements RelationalStore
func (pa *PostgresAdapter) GetSessionCount(ctx context.Context) (int64, error) {
	var count int64
	err := pa.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_sessions`).Scan(&count)
	return count, err
}

// Health implements RelationalStore
func (pa *PostgresAdapter) Health(ctx context.Context) error {
	return pa.db.PingContext(ctx)
}

// Close implements RelationalStore
func (pa *PostgresAdapter) Close() error {
	return pa.db.Close()
}
