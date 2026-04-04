package sqlite

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
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	storage.RegisterRelationalStore(storage.StorageTypeSQLite, func(config *storage.RelationalConfig) (storage.RelationalStore, error) {
		return NewSQLiteAdapter(config)
	})
}

// SQLiteAdapter implements RelationalStore using SQLite
type SQLiteAdapter struct {
	db     *sql.DB
	config *storage.RelationalConfig
}

// NewSQLiteAdapter creates a new SQLite adapter
func NewSQLiteAdapter(config *storage.RelationalConfig) (*SQLiteAdapter, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Default to memory if no database specified
	dbPath := config.Database
	if dbPath == "" {
		dbPath = ":memory:"
	}

	// Add proper options for WAL mode and FTS5 (if supported by build)
	connStr := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON", dbPath)

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Apply connection pool settings
	// SQLite is single-write, so avoid many concurrent write connections locking the DB
	maxConns := config.MaxConnections
	if maxConns == 0 {
		maxConns = 1 // Default for SQLite to avoid locking issues in some setups
	}
	db.SetMaxOpenConns(maxConns)
	
	if config.MinConnections > 0 {
		db.SetMaxIdleConns(config.MinConnections)
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

	adapter := &SQLiteAdapter{
		db:     db,
		config: config,
	}

	if err := adapter.setupTables(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to setup tables: %w", err)
	}

	return adapter, nil
}

func (sa *SQLiteAdapter) setupTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS memory_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			context TEXT,
			created_at DATETIME,
			last_access DATETIME,
			is_active BOOLEAN,
			expires_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS datapoints (
			id TEXT PRIMARY KEY,
			content TEXT,
			content_type TEXT,
			metadata TEXT,
			session_id TEXT,
			user_id TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			processing_status TEXT,
			error_message TEXT,
			nodes TEXT,
			edges TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS input_datapoints (
			id TEXT PRIMARY KEY,
			content TEXT,
			content_type TEXT,
			metadata TEXT,
			session_id TEXT,
			user_id TEXT,
			created_at DATETIME,
			updated_at DATETIME,
			processing_status TEXT,
			error_message TEXT,
			nodes TEXT,
			edges TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_datapoints_session_id ON datapoints(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_datapoints_user_id ON datapoints(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_input_datapoints_session_id ON input_datapoints(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_input_datapoints_user_id ON input_datapoints(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_sessions_user_id ON memory_sessions(user_id)`,
	}

	if sa.config.EnableFullText {
		queries = append(queries, `CREATE VIRTUAL TABLE IF NOT EXISTS datapoints_fts USING fts5(content, content='datapoints', content_rowid='rowid')`)
		
		// Triggers to keep FTS in sync
		queries = append(queries, `
		CREATE TRIGGER IF NOT EXISTS datapoints_ai AFTER INSERT ON datapoints BEGIN
			INSERT INTO datapoints_fts(rowid, content) VALUES (new.rowid, new.content);
		END;`)
		queries = append(queries, `
		CREATE TRIGGER IF NOT EXISTS datapoints_ad AFTER DELETE ON datapoints BEGIN
			INSERT INTO datapoints_fts(datapoints_fts, rowid, content) VALUES('delete', old.rowid, old.content);
		END;`)
		queries = append(queries, `
		CREATE TRIGGER IF NOT EXISTS datapoints_au AFTER UPDATE ON datapoints BEGIN
			INSERT INTO datapoints_fts(datapoints_fts, rowid, content) VALUES('delete', old.rowid, old.content);
			INSERT INTO datapoints_fts(rowid, content) VALUES (new.rowid, new.content);
		END;`)
	}

	for _, query := range queries {
		if _, err := sa.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute setup query %q: %w", query, err)
		}
	}

	// Backwards compatibility for existing databases
	sa.db.ExecContext(ctx, "ALTER TABLE datapoints ADD COLUMN nodes TEXT")
	sa.db.ExecContext(ctx, "ALTER TABLE datapoints ADD COLUMN edges TEXT")
	sa.db.ExecContext(ctx, "ALTER TABLE input_datapoints ADD COLUMN nodes TEXT")
	sa.db.ExecContext(ctx, "ALTER TABLE input_datapoints ADD COLUMN edges TEXT")

	// Table for session messages
	sa.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS session_messages (
		id TEXT PRIMARY KEY,
		session_id TEXT,
		role TEXT,
		content TEXT,
		timestamp DATETIME
	)`)
	sa.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_session_messages_session_id ON session_messages(session_id)`)

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
func (sa *SQLiteAdapter) StoreDataPoint(ctx context.Context, dp *schema.DataPoint) error {
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			content = excluded.content,
			content_type = excluded.content_type,
			metadata = excluded.metadata,
			session_id = excluded.session_id,
			user_id = excluded.user_id,
			updated_at = excluded.updated_at,
			processing_status = excluded.processing_status,
			error_message = excluded.error_message,
			nodes = excluded.nodes,
			edges = excluded.edges
	`, table)

	_, err = sa.db.ExecContext(ctx, query,
		dp.ID, dp.Content, dp.ContentType, string(metadataBytes), dp.SessionID, dp.UserID,
		dp.CreatedAt, dp.UpdatedAt, dp.ProcessingStatus, dp.ErrorMessage, string(nodesBytes), string(edgesBytes))

	return err
}

// GetDataPoint implements RelationalStore
func (sa *SQLiteAdapter) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) {
	for _, table := range []string{"datapoints", "input_datapoints"} {
		query := fmt.Sprintf(`
			SELECT id, content, content_type, metadata, session_id, user_id,
				created_at, updated_at, processing_status, error_message, nodes, edges
			FROM %s WHERE id = ?
		`, table)
		row := sa.db.QueryRowContext(ctx, query, id)

		var dp schema.DataPoint
		var metadataStr, nodesStr, edgesStr sql.NullString
		err := row.Scan(
			&dp.ID, &dp.Content, &dp.ContentType, &metadataStr, &dp.SessionID, &dp.UserID,
			&dp.CreatedAt, &dp.UpdatedAt, &dp.ProcessingStatus, &dp.ErrorMessage, &nodesStr, &edgesStr)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}

		if metadataStr.Valid && metadataStr.String != "" {
			if err := json.Unmarshal([]byte(metadataStr.String), &dp.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		} else {
			dp.Metadata = make(map[string]interface{})
		}
		if nodesStr.Valid && nodesStr.String != "" {
			_ = json.Unmarshal([]byte(nodesStr.String), &dp.Nodes)
		}
		if edgesStr.Valid && edgesStr.String != "" {
			_ = json.Unmarshal([]byte(edgesStr.String), &dp.Edges)
		}
		return &dp, nil
	}
	return nil, fmt.Errorf("datapoint not found: %s", id)
}

// UpdateDataPoint implements RelationalStore
func (sa *SQLiteAdapter) UpdateDataPoint(ctx context.Context, dp *schema.DataPoint) error {
	metadataBytes, err := json.Marshal(dp.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	nodesBytes, _ := json.Marshal(dp.Nodes)
	edgesBytes, _ := json.Marshal(dp.Edges)

	// Try preferred table first, then fallback to the other table.
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
				content = ?, content_type = ?, metadata = ?, session_id = ?, user_id = ?,
				updated_at = ?, processing_status = ?, error_message = ?, nodes = ?, edges = ?
			WHERE id = ?
		`, table)
		result, err := sa.db.ExecContext(ctx, query,
			dp.Content, dp.ContentType, string(metadataBytes), dp.SessionID, dp.UserID,
			time.Now(), dp.ProcessingStatus, dp.ErrorMessage, string(nodesBytes), string(edgesBytes), dp.ID)
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
func (sa *SQLiteAdapter) DeleteDataPoint(ctx context.Context, id string) error {
	total := int64(0)
	for _, table := range []string{"datapoints", "input_datapoints"} {
		query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", table)
		result, err := sa.db.ExecContext(ctx, query, id)
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
func (sa *SQLiteAdapter) DeleteDataPointsBySession(ctx context.Context, sessionID string) error {
	for _, table := range []string{"datapoints", "input_datapoints"} {
		query := fmt.Sprintf("DELETE FROM %s WHERE session_id = ?", table)
		if _, err := sa.db.ExecContext(ctx, query, sessionID); err != nil {
			return err
		}
	}
	return nil
}

// QueryDataPoints implements RelationalStore
func (sa *SQLiteAdapter) QueryDataPoints(ctx context.Context, q *storage.DataPointQuery) ([]*schema.DataPoint, error) {
	if q == nil {
		q = storage.DefaultDataPointQuery()
	}
	primary, err := sa.queryDataPointsFromTable(ctx, q, "datapoints")
	if err != nil {
		return nil, err
	}
	inputs, err := sa.queryDataPointsFromTable(ctx, q, "input_datapoints")
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

func (sa *SQLiteAdapter) queryDataPointsFromTable(ctx context.Context, q *storage.DataPointQuery, table string) ([]*schema.DataPoint, error) {
	queryStr := fmt.Sprintf(`
		SELECT id, content, content_type, metadata, session_id, user_id,
			created_at, updated_at, processing_status, error_message, nodes, edges
		FROM %s WHERE 1=1
	`, table)
	args := []interface{}{}

	if q.SessionID != "" {
		queryStr += " AND session_id = ?"
		args = append(args, q.SessionID)
	}
	if q.UserID != "" {
		queryStr += " AND user_id = ?"
		args = append(args, q.UserID)
	}
	if q.ContentType != "" {
		queryStr += " AND content_type = ?"
		args = append(args, q.ContentType)
	}
	if q.SearchText != "" {
		if q.SearchMode == "exact" {
			queryStr += " AND content = ?"
			args = append(args, q.SearchText)
		} else {
			queryStr += " AND content LIKE ?"
			args = append(args, "%"+q.SearchText+"%")
		}
	}
	if q.CreatedAfter != nil {
		queryStr += " AND created_at > ?"
		args = append(args, *q.CreatedAfter)
	}

	rows, err := sa.db.QueryContext(ctx, queryStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*schema.DataPoint
	for rows.Next() {
		dp := &schema.DataPoint{}
		var metadataStr, nodesStr, edgesStr sql.NullString
		if err := rows.Scan(
			&dp.ID, &dp.Content, &dp.ContentType, &metadataStr, &dp.SessionID, &dp.UserID,
			&dp.CreatedAt, &dp.UpdatedAt, &dp.ProcessingStatus, &dp.ErrorMessage, &nodesStr, &edgesStr,
		); err != nil {
			return nil, err
		}
		if metadataStr.Valid && metadataStr.String != "" {
			if err := json.Unmarshal([]byte(metadataStr.String), &dp.Metadata); err != nil {
				return nil, err
			}
		} else {
			dp.Metadata = make(map[string]interface{})
		}
		if nodesStr.Valid && nodesStr.String != "" {
			_ = json.Unmarshal([]byte(nodesStr.String), &dp.Nodes)
		}
		if edgesStr.Valid && edgesStr.String != "" {
			_ = json.Unmarshal([]byte(edgesStr.String), &dp.Edges)
		}
		results = append(results, dp)
	}
	return results, nil
}

// SearchDataPoints implements RelationalStore
func (sa *SQLiteAdapter) SearchDataPoints(ctx context.Context, searchQuery string, filters map[string]interface{}) ([]*schema.DataPoint, error) {
	q := storage.DefaultDataPointQuery()
	q.SearchText = searchQuery
	q.Filters = filters
	return sa.QueryDataPoints(ctx, q)
}

// StoreSession implements RelationalStore
func (sa *SQLiteAdapter) StoreSession(ctx context.Context, session *schema.MemorySession) error {
	contextBytes, err := json.Marshal(session.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal session context: %w", err)
	}

	query := `
		INSERT INTO memory_sessions (id, user_id, context, created_at, last_access, is_active, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			user_id = excluded.user_id,
			context = excluded.context,
			last_access = excluded.last_access,
			is_active = excluded.is_active,
			expires_at = excluded.expires_at
	`
	_, err = sa.db.ExecContext(ctx, query,
		session.ID, session.UserID, string(contextBytes), session.CreatedAt, 
		session.LastAccess, session.IsActive, session.ExpiresAt)

	return err
}

// GetSession implements RelationalStore
func (sa *SQLiteAdapter) GetSession(ctx context.Context, sessionID string) (*schema.MemorySession, error) {
	query := `
		SELECT id, user_id, context, created_at, last_access, is_active, expires_at
		FROM memory_sessions WHERE id = ?
	`
	row := sa.db.QueryRowContext(ctx, query, sessionID)

	var ms schema.MemorySession
	var contextStr string

	err := row.Scan(
		&ms.ID, &ms.UserID, &contextStr, &ms.CreatedAt, 
		&ms.LastAccess, &ms.IsActive, &ms.ExpiresAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, err
	}

	if contextStr != "" {
		if err := json.Unmarshal([]byte(contextStr), &ms.Context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}
	} else {
		ms.Context = make(map[string]interface{})
	}

	return &ms, nil
}

// UpdateSession implements RelationalStore
func (sa *SQLiteAdapter) UpdateSession(ctx context.Context, session *schema.MemorySession) error {
	contextBytes, err := json.Marshal(session.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal session context: %w", err)
	}

	query := `
		UPDATE memory_sessions SET
			user_id = ?, context = ?, last_access = ?, is_active = ?, expires_at = ?
		WHERE id = ?
	`
	result, err := sa.db.ExecContext(ctx, query,
		session.UserID, string(contextBytes), time.Now(), session.IsActive, session.ExpiresAt, session.ID)

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
func (sa *SQLiteAdapter) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM memory_sessions WHERE id = ?`
	result, err := sa.db.ExecContext(ctx, query, sessionID)
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
func (sa *SQLiteAdapter) ListSessions(ctx context.Context, userID string) ([]*schema.MemorySession, error) {
	query := `
		SELECT id, user_id, context, created_at, last_access, is_active, expires_at
		FROM memory_sessions WHERE user_id = ? ORDER BY last_access DESC
	`
	rows, err := sa.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*schema.MemorySession
	for rows.Next() {
		ms := &schema.MemorySession{}
		var contextStr string
		err := rows.Scan(
			&ms.ID, &ms.UserID, &contextStr, &ms.CreatedAt, 
			&ms.LastAccess, &ms.IsActive, &ms.ExpiresAt)
		if err != nil {
			return nil, err
		}
		if contextStr != "" {
			if err := json.Unmarshal([]byte(contextStr), &ms.Context); err != nil {
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
func (sa *SQLiteAdapter) AddMessageToSession(ctx context.Context, sessionID string, message schema.Message) error {
	query := `
		INSERT INTO session_messages (id, session_id, role, content, timestamp)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := sa.db.ExecContext(ctx, query, message.ID, sessionID, string(message.Role), message.Content, message.Timestamp)
	return err
}

// GetSessionMessages implements RelationalStore
func (sa *SQLiteAdapter) GetSessionMessages(ctx context.Context, sessionID string) ([]schema.Message, error) {
	query := `
		SELECT id, role, content, timestamp
		FROM session_messages WHERE session_id = ? ORDER BY timestamp ASC
	`
	rows, err := sa.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []schema.Message
	for rows.Next() {
		var msg schema.Message
		var roleStr string
		err := rows.Scan(&msg.ID, &roleStr, &msg.Content, &msg.Timestamp)
		if err != nil {
			return nil, err
		}
		msg.Role = schema.Role(roleStr)
		messages = append(messages, msg)
	}

	return messages, nil
}

// StoreBatch implements RelationalStore
func (sa *SQLiteAdapter) StoreBatch(ctx context.Context, dataPoints []*schema.DataPoint) error {
	for _, dp := range dataPoints {
		if err := sa.StoreDataPoint(ctx, dp); err != nil {
			return err
		}
	}
	return nil
}

// DeleteBatch implements RelationalStore
func (sa *SQLiteAdapter) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	
	// Create batches of 999 max (SQLite hard limit for variables)
	const batchSize = 900
	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		
		batchIDs := ids[i:end]
		placeholders := make([]string, len(batchIDs))
		args := make([]interface{}, len(batchIDs))
		
		for j, id := range batchIDs {
			placeholders[j] = "?"
			args[j] = id
		}
		
		query := fmt.Sprintf("DELETE FROM datapoints WHERE id IN (%s)", strings.Join(placeholders, ","))
		if _, err := sa.db.ExecContext(ctx, query, args...); err != nil {
			return err
		}
		queryInput := fmt.Sprintf("DELETE FROM input_datapoints WHERE id IN (%s)", strings.Join(placeholders, ","))
		if _, err := sa.db.ExecContext(ctx, queryInput, args...); err != nil {
			return err
		}
	}

	return nil
}

// GetDataPointCount implements RelationalStore
func (sa *SQLiteAdapter) GetDataPointCount(ctx context.Context) (int64, error) {
	var countA, countB int64
	if err := sa.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM datapoints`).Scan(&countA); err != nil {
		return 0, err
	}
	if err := sa.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM input_datapoints`).Scan(&countB); err != nil {
		return 0, err
	}
	return countA + countB, nil
}

// GetSessionCount implements RelationalStore
func (sa *SQLiteAdapter) GetSessionCount(ctx context.Context) (int64, error) {
	var count int64
	err := sa.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_sessions`).Scan(&count)
	return count, err
}

// Health implements RelationalStore
func (sa *SQLiteAdapter) Health(ctx context.Context) error {
	return sa.db.PingContext(ctx)
}

// Close implements RelationalStore
func (sa *SQLiteAdapter) Close() error {
	return sa.db.Close()
}
