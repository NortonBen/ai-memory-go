package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	_ "github.com/lib/pq"
)

func init() {
	storage.RegisterRelationalStore(storage.StorageTypePostgreSQL, func(config *storage.RelationalConfig) (storage.RelationalStore, error) {
		return NewPostgresAdapter(config)
	})
}

// PostgresAdapter implements RelationalStore using PostgreSQL
type PostgresAdapter struct {
	db     *sql.DB
	config *storage.RelationalConfig
}

// NewPostgresAdapter creates a new PostgreSQL adapter
func NewPostgresAdapter(config *storage.RelationalConfig) (*PostgresAdapter, error) {
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
			error_message TEXT,
			nodes JSONB,
			edges JSONB
		)`,
		`CREATE TABLE IF NOT EXISTS input_datapoints (
			id VARCHAR(255) PRIMARY KEY,
			content TEXT,
			content_type VARCHAR(255),
			metadata JSONB,
			session_id VARCHAR(255),
			user_id VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE,
			updated_at TIMESTAMP WITH TIME ZONE,
			processing_status VARCHAR(50),
			error_message TEXT,
			nodes JSONB,
			edges JSONB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_datapoints_session_id ON datapoints(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_datapoints_user_id ON datapoints(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_input_datapoints_session_id ON input_datapoints(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_input_datapoints_user_id ON input_datapoints(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_sessions_user_id ON memory_sessions(user_id)`,
		`CREATE TABLE IF NOT EXISTS session_messages (
			id VARCHAR(255) PRIMARY KEY,
			session_id VARCHAR(255) NOT NULL,
			role VARCHAR(50) NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_session_messages_session_id ON session_messages(session_id)`,
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

func isInputDataPoint(dp *schema.DataPoint) bool {
	if dp == nil || dp.Metadata == nil {
		return false
	}
	raw, ok := dp.Metadata["is_input"]
	if !ok {
		return false
	}
	v, ok := raw.(bool)
	return ok && v
}

func tableForDataPoint(dp *schema.DataPoint) string {
	if isInputDataPoint(dp) {
		return "input_datapoints"
	}
	return "datapoints"
}

// StoreDataPoint implements RelationalStore
func (pa *PostgresAdapter) StoreDataPoint(ctx context.Context, dp *schema.DataPoint) error {
	metadataBytes, err := json.Marshal(dp.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	nodesBytes, _ := json.Marshal(dp.Nodes)
	edgesBytes, _ := json.Marshal(dp.Edges)

	table := tableForDataPoint(dp)
	query := fmt.Sprintf(`
		INSERT INTO %s (id, content, content_type, metadata, session_id, user_id, 
			created_at, updated_at, processing_status, error_message, nodes, edges)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			content_type = EXCLUDED.content_type,
			metadata = EXCLUDED.metadata,
			session_id = EXCLUDED.session_id,
			user_id = EXCLUDED.user_id,
			updated_at = EXCLUDED.updated_at,
			processing_status = EXCLUDED.processing_status,
			error_message = EXCLUDED.error_message,
			nodes = EXCLUDED.nodes,
			edges = EXCLUDED.edges
	`, table)

	_, err = pa.db.ExecContext(ctx, query,
		dp.ID, dp.Content, dp.ContentType, metadataBytes, dp.SessionID, dp.UserID,
		dp.CreatedAt, dp.UpdatedAt, dp.ProcessingStatus, dp.ErrorMessage, nodesBytes, edgesBytes)

	return err
}

// GetDataPoint implements RelationalStore
func (pa *PostgresAdapter) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) {
	for _, table := range []string{"datapoints", "input_datapoints"} {
		query := fmt.Sprintf(`
			SELECT id, content, content_type, metadata, session_id, user_id,
				created_at, updated_at, processing_status, error_message, nodes, edges
			FROM %s WHERE id = $1
		`, table)
		row := pa.db.QueryRowContext(ctx, query, id)

		var dp schema.DataPoint
		var metadataBytes, nodesBytes, edgesBytes []byte
		err := row.Scan(
			&dp.ID, &dp.Content, &dp.ContentType, &metadataBytes, &dp.SessionID, &dp.UserID,
			&dp.CreatedAt, &dp.UpdatedAt, &dp.ProcessingStatus, &dp.ErrorMessage, &nodesBytes, &edgesBytes)
		if err == sql.ErrNoRows {
			continue
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
		if len(nodesBytes) > 0 {
			_ = json.Unmarshal(nodesBytes, &dp.Nodes)
		}
		if len(edgesBytes) > 0 {
			_ = json.Unmarshal(edgesBytes, &dp.Edges)
		}
		return &dp, nil
	}
	return nil, fmt.Errorf("datapoint not found: %s", id)
}

// UpdateDataPoint implements RelationalStore
func (pa *PostgresAdapter) UpdateDataPoint(ctx context.Context, dp *schema.DataPoint) error {
	metadataBytes, err := json.Marshal(dp.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	nodesBytes, _ := json.Marshal(dp.Nodes)
	edgesBytes, _ := json.Marshal(dp.Edges)

	preferred := tableForDataPoint(dp)
	tables := []string{preferred}
	if preferred == "datapoints" {
		tables = append(tables, "input_datapoints")
	} else {
		tables = append(tables, "datapoints")
	}
	for _, table := range tables {
		query := fmt.Sprintf(`
			UPDATE %s SET
				content = $1, content_type = $2, metadata = $3, session_id = $4, user_id = $5,
				updated_at = $6, processing_status = $7, error_message = $8, nodes = $9, edges = $10
			WHERE id = $11
		`, table)
		result, err := pa.db.ExecContext(ctx, query,
			dp.Content, dp.ContentType, metadataBytes, dp.SessionID, dp.UserID,
			time.Now(), dp.ProcessingStatus, dp.ErrorMessage, nodesBytes, edgesBytes, dp.ID)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows > 0 {
			return nil
		}
	}
	return fmt.Errorf("datapoint not found: %s", dp.ID)
}

// DeleteDataPoint implements RelationalStore
func (pa *PostgresAdapter) DeleteDataPoint(ctx context.Context, id string) error {
	var total int64
	for _, table := range []string{"datapoints", "input_datapoints"} {
		query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", table)
		result, err := pa.db.ExecContext(ctx, query, id)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		total += rows
	}
	if total == 0 {
		return fmt.Errorf("datapoint not found: %s", id)
	}
	return nil
}

// DeleteDataPointsBySession implements RelationalStore
func (pa *PostgresAdapter) DeleteDataPointsBySession(ctx context.Context, sessionID string) error {
	for _, table := range []string{"datapoints", "input_datapoints"} {
		query := fmt.Sprintf("DELETE FROM %s WHERE session_id = $1", table)
		if _, err := pa.db.ExecContext(ctx, query, sessionID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteDataPointsUnscoped implements RelationalStore.
func (pa *PostgresAdapter) DeleteDataPointsUnscoped(ctx context.Context) error {
	for _, table := range []string{"datapoints", "input_datapoints"} {
		q := fmt.Sprintf("DELETE FROM %s WHERE session_id IS NULL OR session_id = ''", table)
		if _, err := pa.db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// QueryDataPoints implements RelationalStore
func (pa *PostgresAdapter) QueryDataPoints(ctx context.Context, q *storage.DataPointQuery) ([]*schema.DataPoint, error) {
	if q == nil {
		q = storage.DefaultDataPointQuery()
	}
	primary, err := pa.queryDataPointsFromTable(ctx, q, "datapoints")
	if err != nil {
		return nil, err
	}
	inputs, err := pa.queryDataPointsFromTable(ctx, q, "input_datapoints")
	if err != nil {
		return nil, err
	}
	results := append(primary, inputs...)

	sortBy := q.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	desc := strings.ToLower(q.SortOrder) != "asc"
	sort.Slice(results, func(i, j int) bool {
		switch sortBy {
		case "updated_at":
			if desc {
				return results[i].UpdatedAt.After(results[j].UpdatedAt)
			}
			return results[i].UpdatedAt.Before(results[j].UpdatedAt)
		case "id":
			if desc {
				return results[i].ID > results[j].ID
			}
			return results[i].ID < results[j].ID
		default:
			if desc {
				return results[i].CreatedAt.After(results[j].CreatedAt)
			}
			return results[i].CreatedAt.Before(results[j].CreatedAt)
		}
	})

	start := q.Offset
	if start < 0 {
		start = 0
	}
	if start >= len(results) {
		return []*schema.DataPoint{}, nil
	}
	end := len(results)
	if q.Limit > 0 && start+q.Limit < end {
		end = start + q.Limit
	}
	return results[start:end], nil
}

func (pa *PostgresAdapter) queryDataPointsFromTable(ctx context.Context, q *storage.DataPointQuery, table string) ([]*schema.DataPoint, error) {
	queryStr := fmt.Sprintf(`
		SELECT id, content, content_type, metadata, session_id, user_id,
			created_at, updated_at, processing_status, error_message, nodes, edges
		FROM %s WHERE 1=1
	`, table)
	args := []interface{}{}
	argID := 1

	if q.UnscopedSessionOnly {
		queryStr += " AND (session_id IS NULL OR session_id = '')"
	} else if q.SessionID != "" && q.IncludeGlobalSession {
		queryStr += fmt.Sprintf(" AND (session_id = $%d OR session_id IS NULL OR session_id = '')", argID)
		args = append(args, q.SessionID)
		argID++
	} else if q.SessionID != "" {
		queryStr += fmt.Sprintf(" AND session_id = $%d", argID)
		args = append(args, q.SessionID)
		argID++
	}
	if q.UserID != "" {
		queryStr += fmt.Sprintf(" AND user_id = $%d", argID)
		args = append(args, q.UserID)
		argID++
	}
	if q.ContentType != "" {
		queryStr += fmt.Sprintf(" AND content_type = $%d", argID)
		args = append(args, q.ContentType)
		argID++
	}
	if q.SearchText != "" {
		if q.SearchMode == "exact" {
			queryStr += fmt.Sprintf(" AND content = $%d", argID)
			args = append(args, q.SearchText)
		} else {
			queryStr += fmt.Sprintf(" AND content ILIKE $%d", argID)
			args = append(args, "%"+q.SearchText+"%")
		}
		argID++
	}
	if q.CreatedAfter != nil {
		queryStr += fmt.Sprintf(" AND created_at > $%d", argID)
		args = append(args, *q.CreatedAfter)
	}

	rows, err := pa.db.QueryContext(ctx, queryStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*schema.DataPoint
	for rows.Next() {
		dp := &schema.DataPoint{}
		var metadataBytes, nodesBytes, edgesBytes []byte
		err := rows.Scan(
			&dp.ID, &dp.Content, &dp.ContentType, &metadataBytes, &dp.SessionID, &dp.UserID,
			&dp.CreatedAt, &dp.UpdatedAt, &dp.ProcessingStatus, &dp.ErrorMessage, &nodesBytes, &edgesBytes)
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
		if len(nodesBytes) > 0 {
			_ = json.Unmarshal(nodesBytes, &dp.Nodes)
		}
		if len(edgesBytes) > 0 {
			_ = json.Unmarshal(edgesBytes, &dp.Edges)
		}
		results = append(results, dp)
	}
	return results, nil
}

// SearchDataPoints implements RelationalStore
func (pa *PostgresAdapter) SearchDataPoints(ctx context.Context, searchQuery string, filters map[string]interface{}) ([]*schema.DataPoint, error) {
	q := storage.DefaultDataPointQuery()
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

// AddMessageToSession implements RelationalStore
func (pa *PostgresAdapter) AddMessageToSession(ctx context.Context, sessionID string, msg schema.Message) error {
	query := `
		INSERT INTO session_messages (id, session_id, role, content, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := pa.db.ExecContext(ctx, query,
		msg.ID, sessionID, msg.Role, msg.Content, msg.Timestamp)
	return err
}

// GetSessionMessages implements RelationalStore
func (pa *PostgresAdapter) GetSessionMessages(ctx context.Context, sessionID string) ([]schema.Message, error) {
	queryStr := `
		SELECT id, role, content, created_at
		FROM session_messages
		WHERE session_id = $1
		ORDER BY created_at ASC
	`
	rows, err := pa.db.QueryContext(ctx, queryStr, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []schema.Message
	for rows.Next() {
		var msg schema.Message
		if err := rows.Scan(&msg.ID, &msg.Role, &msg.Content, &msg.Timestamp); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// DeleteSessionMessages implements RelationalStore.
func (pa *PostgresAdapter) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	_, err := pa.db.ExecContext(ctx, `DELETE FROM session_messages WHERE session_id = $1`, sessionID)
	return err
}

// StoreBatch implements RelationalStore
func (pa *PostgresAdapter) StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error {
	for _, dp := range dataPoints {
		if err := pa.StoreDataPoint(ctx, dp); err != nil {
			return err
		}
	}
	return nil
}

// DeleteBatch implements RelationalStore
func (pa *PostgresAdapter) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// For efficiency, use ANY in postgres
	arrayLiteral := "{" + strings.Join(ids, ",") + "}"
	for _, table := range []string{"datapoints", "input_datapoints"} {
		query := fmt.Sprintf("DELETE FROM %s WHERE id = ANY($1)", table)
		if _, err := pa.db.ExecContext(ctx, query, arrayLiteral); err != nil {
			return err
		}
	}
	return nil
}

// GetDataPointCount implements RelationalStore
func (pa *PostgresAdapter) GetDataPointCount(ctx context.Context) (int64, error) {
	var a, b int64
	if err := pa.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM datapoints`).Scan(&a); err != nil {
		return 0, err
	}
	if err := pa.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM input_datapoints`).Scan(&b); err != nil {
		return 0, err
	}
	return a + b, nil
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
