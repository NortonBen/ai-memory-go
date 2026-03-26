package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
		`CREATE INDEX IF NOT EXISTS idx_datapoints_session_id ON datapoints(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_datapoints_user_id ON datapoints(user_id)`,
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

// StoreDataPoint implements RelationalStore
func (sa *SQLiteAdapter) StoreDataPoint(ctx context.Context, dp *schema.DataPoint) error {
	metadataBytes, err := json.Marshal(dp.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	nodesBytes, _ := json.Marshal(dp.Nodes)
	edgesBytes, _ := json.Marshal(dp.Edges)

	query := `
		INSERT INTO datapoints (id, content, content_type, metadata, session_id, user_id, 
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
	`

	_, err = sa.db.ExecContext(ctx, query,
		dp.ID, dp.Content, dp.ContentType, string(metadataBytes), dp.SessionID, dp.UserID,
		dp.CreatedAt, dp.UpdatedAt, dp.ProcessingStatus, dp.ErrorMessage, string(nodesBytes), string(edgesBytes))

	return err
}

// GetDataPoint implements RelationalStore
func (sa *SQLiteAdapter) GetDataPoint(ctx context.Context, id string) (*schema.DataPoint, error) {
	query := `
		SELECT id, content, content_type, metadata, session_id, user_id,
			created_at, updated_at, processing_status, error_message, nodes, edges
		FROM datapoints WHERE id = ?
	`
	row := sa.db.QueryRowContext(ctx, query, id)

	var dp schema.DataPoint
	var metadataStr, nodesStr, edgesStr sql.NullString

	err := row.Scan(
		&dp.ID, &dp.Content, &dp.ContentType, &metadataStr, &dp.SessionID, &dp.UserID,
		&dp.CreatedAt, &dp.UpdatedAt, &dp.ProcessingStatus, &dp.ErrorMessage, &nodesStr, &edgesStr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("datapoint not found: %s", id)
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

// UpdateDataPoint implements RelationalStore
func (sa *SQLiteAdapter) UpdateDataPoint(ctx context.Context, dp *schema.DataPoint) error {
	metadataBytes, err := json.Marshal(dp.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	nodesBytes, _ := json.Marshal(dp.Nodes)
	edgesBytes, _ := json.Marshal(dp.Edges)

	query := `
		UPDATE datapoints SET
			content = ?, content_type = ?, metadata = ?, session_id = ?, user_id = ?,
			updated_at = ?, processing_status = ?, error_message = ?, nodes = ?, edges = ?
		WHERE id = ?
	`
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
	if rows == 0 {
		return fmt.Errorf("datapoint not found: %s", dp.ID)
	}

	return nil
}

// DeleteDataPoint implements RelationalStore
func (sa *SQLiteAdapter) DeleteDataPoint(ctx context.Context, id string) error {
	query := `DELETE FROM datapoints WHERE id = ?`
	result, err := sa.db.ExecContext(ctx, query, id)
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
func (sa *SQLiteAdapter) DeleteDataPointsBySession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM datapoints WHERE session_id = ?`
	_, err := sa.db.ExecContext(ctx, query, sessionID)
	return err
}

// QueryDataPoints implements RelationalStore
func (sa *SQLiteAdapter) QueryDataPoints(ctx context.Context, q *storage.DataPointQuery) ([]*schema.DataPoint, error) {
	if q == nil {
		q = storage.DefaultDataPointQuery()
	}

	queryStr := `
		SELECT id, content, content_type, metadata, session_id, user_id,
			created_at, updated_at, processing_status, error_message, nodes, edges
		FROM datapoints WHERE 1=1
	`
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
		} else if sa.config.EnableFullText && (q.SearchMode == "fulltext" || q.SearchMode == "") {
			queryStr = `
			SELECT d.id, d.content, d.content_type, d.metadata, d.session_id, d.user_id,
				d.created_at, d.updated_at, d.processing_status, d.error_message, d.nodes, d.edges
			FROM datapoints d
			JOIN datapoints_fts fts ON d.rowid = fts.rowid
			WHERE fts.content MATCH ?
			`
			args = []interface{}{q.SearchText} // Reset args, replace WHERE clause logic below if mixed
			
			// Re-apply other filters
			if q.SessionID != "" {
				queryStr += " AND d.session_id = ?"
				args = append(args, q.SessionID)
			}
			if q.UserID != "" {
				queryStr += " AND d.user_id = ?"
				args = append(args, q.UserID)
			}
			if q.ContentType != "" {
				queryStr += " AND d.content_type = ?"
				args = append(args, q.ContentType)
			}
		} else {
			queryStr += " AND content LIKE ?"
			args = append(args, "%"+q.SearchText+"%")
		}
	}
	if q.CreatedAfter != nil {
		queryStr += " AND created_at > ?"
		args = append(args, *q.CreatedAfter)
	}

	if q.SortBy != "" {
		order := "ASC"
		if strings.ToLower(q.SortOrder) == "desc" {
			order = "DESC"
		}
		// Basic prevention against sql injection for order by
		validSortCol := map[string]bool{"created_at": true, "updated_at": true, "id": true}
		if validSortCol[q.SortBy] {
			if sa.config.EnableFullText && q.SearchText != "" && (q.SearchMode == "fulltext" || q.SearchMode == "") {
				queryStr += fmt.Sprintf(" ORDER BY d.%s %s", q.SortBy, order)
			} else {
				queryStr += fmt.Sprintf(" ORDER BY %s %s", q.SortBy, order)
			}
		} else {
			if sa.config.EnableFullText && q.SearchText != "" && (q.SearchMode == "fulltext" || q.SearchMode == "") {
				queryStr += fmt.Sprintf(" ORDER BY d.created_at %s", order)
			} else {
				queryStr += fmt.Sprintf(" ORDER BY created_at %s", order)
			}
		}
	} else {
		if sa.config.EnableFullText && q.SearchText != "" && (q.SearchMode == "fulltext" || q.SearchMode == "") {
			queryStr += " ORDER BY d.created_at DESC"
		} else {
			queryStr += " ORDER BY created_at DESC"
		}
	}

	if q.Limit > 0 {
		queryStr += " LIMIT ?"
		args = append(args, q.Limit)
	}
	if q.Offset > 0 {
		queryStr += " OFFSET ?"
		args = append(args, q.Offset)
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
		err := rows.Scan(
			&dp.ID, &dp.Content, &dp.ContentType, &metadataStr, &dp.SessionID, &dp.UserID,
			&dp.CreatedAt, &dp.UpdatedAt, &dp.ProcessingStatus, &dp.ErrorMessage, &nodesStr, &edgesStr)
		if err != nil {
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
	tx, err := sa.db.BeginTx(ctx, nil)
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
		nodesBytes, _ := json.Marshal(dp.Nodes)
		edgesBytes, _ := json.Marshal(dp.Edges)
		_, err = stmt.ExecContext(ctx,
			dp.ID, dp.Content, dp.ContentType, string(metadataBytes), dp.SessionID, dp.UserID,
			dp.CreatedAt, dp.UpdatedAt, dp.ProcessingStatus, dp.ErrorMessage, string(nodesBytes), string(edgesBytes))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
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
		_, err := sa.db.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetDataPointCount implements RelationalStore
func (sa *SQLiteAdapter) GetDataPointCount(ctx context.Context) (int64, error) {
	var count int64
	err := sa.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM datapoints`).Scan(&count)
	return count, err
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
