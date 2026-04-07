package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
)

func init() {
	graph.RegisterStore(graph.StoreTypeSQLite, func(config *graph.GraphConfig) (graph.GraphStore, error) {
		return NewSQLiteAdapter(config)
	})
}

// SQLiteGraphStore implements GraphStore using a dedicated SQLite DB file.
// It uses recursive CTEs for graph traversal.
// For vector storage use vector.SQLiteVectorStore with a separate DB file.
type SQLiteGraphStore struct {
	db   *sql.DB
	path string
}

// NewSQLiteAdapter opens (or creates) a SQLite database at path and
// initialises the graph schema (nodes + edges tables).
func NewSQLiteAdapter(config *graph.GraphConfig) (*SQLiteGraphStore, error) {
	path := config.Database
	if path == "" {
		path = "memory_graph.db"
	}
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("sqlite graph open: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &SQLiteGraphStore{db: db, path: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite graph migrate: %w", err)
	}
	return s, nil
}

func (s *SQLiteGraphStore) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS graph_nodes (
			id          TEXT PRIMARY KEY,
			type        TEXT NOT NULL,
			entity_name TEXT DEFAULT '',
			properties  TEXT DEFAULT '{}',
			session_id  TEXT DEFAULT '',
			user_id     TEXT DEFAULT '',
			weight      REAL DEFAULT 1.0,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_type ON graph_nodes(type)`,
		`CREATE INDEX IF NOT EXISTS idx_nodes_entity ON graph_nodes(entity_name)`,
		`CREATE TABLE IF NOT EXISTS graph_edges (
			id         TEXT PRIMARY KEY,
			from_id    TEXT NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
			to_id      TEXT NOT NULL REFERENCES graph_nodes(id) ON DELETE CASCADE,
			type       TEXT NOT NULL,
			weight     REAL DEFAULT 1.0,
			properties TEXT DEFAULT '{}',
			session_id TEXT DEFAULT '',
			user_id    TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_from ON graph_edges(from_id)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_to ON graph_edges(to_id)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_type ON graph_edges(type)`,
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

// Health implements GraphStore.
func (s *SQLiteGraphStore) Health(_ context.Context) error { return s.db.Ping() }

// Close implements GraphStore.
func (s *SQLiteGraphStore) Close() error { return s.db.Close() }

// ─── Node Operations ──────────────────────────────────────────────────────────

func (s *SQLiteGraphStore) StoreNode(_ context.Context, node *schema.Node) error {
	props, _ := json.Marshal(node.Properties)
	entityName := ""
	if name, ok := node.Properties["name"].(string); ok {
		entityName = name
	}
	_, err := s.db.Exec(
		`INSERT INTO graph_nodes(id, type, entity_name, properties, session_id, user_id, weight, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   type=excluded.type, entity_name=excluded.entity_name, properties=excluded.properties,
		   session_id=excluded.session_id, user_id=excluded.user_id, weight=excluded.weight,
		   updated_at=excluded.updated_at`,
		node.ID, string(node.Type), entityName, string(props),
		node.SessionID, node.UserID, node.Weight, node.CreatedAt, node.UpdatedAt,
	)
	return err
}

func (s *SQLiteGraphStore) GetNode(_ context.Context, nodeID string) (*schema.Node, error) {
	row := s.db.QueryRow(
		`SELECT id, type, entity_name, properties, session_id, user_id, weight, created_at, updated_at
		 FROM graph_nodes WHERE id=?`, nodeID)
	return scanGNode(row)
}

func (s *SQLiteGraphStore) UpdateNode(ctx context.Context, node *schema.Node) error {
	node.UpdatedAt = time.Now()
	return s.StoreNode(ctx, node)
}

func (s *SQLiteGraphStore) DeleteNode(_ context.Context, nodeID string) error {
	_, err := s.db.Exec(`DELETE FROM graph_nodes WHERE id=?`, nodeID)
	return err
}

func (s *SQLiteGraphStore) FindNodesByType(_ context.Context, nodeType schema.NodeType) ([]*schema.Node, error) {
	rows, err := s.db.Query(
		`SELECT id, type, entity_name, properties, session_id, user_id, weight, created_at, updated_at
		 FROM graph_nodes WHERE type=?`, string(nodeType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGNodes(rows)
}

func (s *SQLiteGraphStore) FindNodesByProperty(_ context.Context, property string, value interface{}) ([]*schema.Node, error) {
	rows, err := s.db.Query(
		`SELECT id, type, entity_name, properties, session_id, user_id, weight, created_at, updated_at
		 FROM graph_nodes WHERE JSON_EXTRACT(properties, ?)=?`,
		"$."+property, fmt.Sprintf("%v", value))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGNodes(rows)
}

func (s *SQLiteGraphStore) FindNodesByEntity(_ context.Context, entityName string, entityType schema.NodeType) ([]*schema.Node, error) {
	rows, err := s.db.Query(
		`SELECT id, type, entity_name, properties, session_id, user_id, weight, created_at, updated_at
		 FROM graph_nodes WHERE entity_name=? AND type=?`,
		entityName, string(entityType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGNodes(rows)
}

// ─── Edge Operations ──────────────────────────────────────────────────────────

func (s *SQLiteGraphStore) CreateRelationship(_ context.Context, edge *schema.Edge) error {
	props, _ := json.Marshal(edge.Properties)
	_, err := s.db.Exec(
		`INSERT INTO graph_edges(id, from_id, to_id, type, weight, properties, session_id, user_id, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   type=excluded.type, weight=excluded.weight, properties=excluded.properties,
		   session_id=excluded.session_id, user_id=excluded.user_id, updated_at=excluded.updated_at`,
		edge.ID, edge.From, edge.To, string(edge.Type), edge.Weight, string(props),
		edge.SessionID, edge.UserID, edge.CreatedAt, edge.UpdatedAt,
	)
	return err
}

func (s *SQLiteGraphStore) GetRelationship(_ context.Context, edgeID string) (*schema.Edge, error) {
	row := s.db.QueryRow(
		`SELECT id, from_id, to_id, type, weight, properties, session_id, user_id, created_at, updated_at
		 FROM graph_edges WHERE id=?`, edgeID)
	return scanGEdge(row)
}

func (s *SQLiteGraphStore) UpdateRelationship(ctx context.Context, edge *schema.Edge) error {
	edge.UpdatedAt = time.Now()
	return s.CreateRelationship(ctx, edge)
}

func (s *SQLiteGraphStore) DeleteRelationship(_ context.Context, edgeID string) error {
	_, err := s.db.Exec(`DELETE FROM graph_edges WHERE id=?`, edgeID)
	return err
}

// ─── Graph Traversal ─────────────────────────────────────────────────────────

func (s *SQLiteGraphStore) TraverseGraph(_ context.Context, startNodeID string, depth int, _ map[string]interface{}) ([]*schema.Node, error) {
	if depth <= 0 {
		depth = 2
	}
	rows, err := s.db.Query(`
		WITH RECURSIVE traverse(id, depth) AS (
			SELECT ?, 0
			UNION ALL
			SELECT e.to_id, t.depth + 1
			FROM graph_edges e JOIN traverse t ON e.from_id = t.id
			WHERE t.depth < ?
		)
		SELECT DISTINCT n.id, n.type, n.entity_name, n.properties, n.session_id, n.user_id, n.weight, n.created_at, n.updated_at
		FROM graph_nodes n JOIN traverse t ON n.id = t.id
		WHERE n.id != ?`, startNodeID, depth, startNodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGNodes(rows)
}

func (s *SQLiteGraphStore) FindConnected(_ context.Context, nodeID string, edgeTypes []schema.EdgeType) ([]*schema.Node, error) {
	baseQ := `SELECT n.id, n.type, n.entity_name, n.properties, n.session_id, n.user_id, n.weight, n.created_at, n.updated_at
	          FROM graph_nodes n JOIN graph_edges e ON e.to_id = n.id WHERE e.from_id = ?`
	if len(edgeTypes) == 0 {
		rows, err := s.db.Query(baseQ, nodeID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return collectGNodes(rows)
	}
	ph := strings.Repeat("?,", len(edgeTypes))
	ph = ph[:len(ph)-1]
	args := []interface{}{nodeID}
	for _, et := range edgeTypes {
		args = append(args, string(et))
	}
	rows, err := s.db.Query(fmt.Sprintf("%s AND e.type IN (%s)", baseQ, ph), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGNodes(rows)
}

func (s *SQLiteGraphStore) FindPath(_ context.Context, fromNodeID, toNodeID string, maxDepth int) ([]*schema.Node, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	rows, err := s.db.Query(`
		WITH RECURSIVE path(id, path_ids, depth) AS (
			SELECT ?, ?, 0
			UNION ALL
			SELECT e.to_id, p.path_ids || ',' || e.to_id, p.depth + 1
			FROM graph_edges e JOIN path p ON e.from_id = p.id
			WHERE p.depth < ? AND p.path_ids NOT LIKE '%' || e.to_id || '%'
		)
		SELECT n.id, n.type, n.entity_name, n.properties, n.session_id, n.user_id, n.weight, n.created_at, n.updated_at
		FROM graph_nodes n
		JOIN (SELECT path_ids FROM path WHERE id = ? LIMIT 1) p ON p.path_ids LIKE '%' || n.id || '%'`,
		fromNodeID, fromNodeID, maxDepth, toNodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectGNodes(rows)
}

// ─── Batch ───────────────────────────────────────────────────────────────────

func (s *SQLiteGraphStore) StoreBatch(ctx context.Context, nodes []*schema.Node, edges []*schema.Edge) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	nStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO graph_nodes(id, type, entity_name, properties, session_id, user_id, weight, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET type=excluded.type, entity_name=excluded.entity_name,
		   properties=excluded.properties, session_id=excluded.session_id, user_id=excluded.user_id,
		   weight=excluded.weight, updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer nStmt.Close()

	for _, n := range nodes {
		props, _ := json.Marshal(n.Properties)
		entityName := ""
		if name, ok := n.Properties["name"].(string); ok {
			entityName = name
		}
		if _, err := nStmt.ExecContext(ctx, n.ID, string(n.Type), entityName, string(props),
			n.SessionID, n.UserID, n.Weight, n.CreatedAt, n.UpdatedAt); err != nil {
			return err
		}
	}

	eStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO graph_edges(id, from_id, to_id, type, weight, properties, session_id, user_id, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET type=excluded.type, weight=excluded.weight,
		   properties=excluded.properties, updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer eStmt.Close()

	for _, e := range edges {
		props, _ := json.Marshal(e.Properties)
		if _, err := eStmt.ExecContext(ctx, e.ID, e.From, e.To, string(e.Type), e.Weight, string(props),
			e.SessionID, e.UserID, e.CreatedAt, e.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteGraphStore) DeleteBatch(ctx context.Context, nodeIDs []string, edgeIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range edgeIDs {
		tx.ExecContext(ctx, `DELETE FROM graph_edges WHERE id=?`, id)
	}
	for _, id := range nodeIDs {
		tx.ExecContext(ctx, `DELETE FROM graph_nodes WHERE id=?`, id)
	}
	return tx.Commit()
}

// DeleteGraphBySessionID implements GraphStore.
func (s *SQLiteGraphStore) DeleteGraphBySessionID(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM graph_edges WHERE session_id IS NULL OR session_id = ''`); err != nil {
			return err
		}
		_, err := s.db.ExecContext(ctx, `DELETE FROM graph_nodes WHERE session_id IS NULL OR session_id = ''`)
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM graph_edges WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM graph_nodes WHERE session_id = ?`, sessionID)
	return err
}

// ─── Analytics ───────────────────────────────────────────────────────────────

func (s *SQLiteGraphStore) GetNodeCount(_ context.Context) (int64, error) {
	var c int64
	return c, s.db.QueryRow(`SELECT COUNT(*) FROM graph_nodes`).Scan(&c)
}

func (s *SQLiteGraphStore) GetEdgeCount(_ context.Context) (int64, error) {
	var c int64
	return c, s.db.QueryRow(`SELECT COUNT(*) FROM graph_edges`).Scan(&c)
}

func (s *SQLiteGraphStore) GetConnectedComponents(_ context.Context) ([][]string, error) {
	rows, err := s.db.Query(`SELECT id FROM graph_nodes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var comps [][]string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		comps = append(comps, []string{id})
	}
	return comps, rows.Err()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

type gRowScanner interface{ Scan(dest ...interface{}) error }

func scanGNode(row gRowScanner) (*schema.Node, error) {
	var id, typ, entityName, propsStr, sessionID, userID string
	var weight float64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &typ, &entityName, &propsStr, &sessionID, &userID, &weight, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var props map[string]interface{}
	if err := json.Unmarshal([]byte(propsStr), &props); err != nil {
		props = make(map[string]interface{})
	}
	if entityName != "" {
		if _, ok := props["name"]; !ok {
			props["name"] = entityName
		}
	}
	return &schema.Node{
		ID:        id,
		Type:      schema.NodeType(typ),
		Properties: props,
		SessionID: sessionID,
		UserID:    userID,
		Weight:    weight,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func scanGEdge(row gRowScanner) (*schema.Edge, error) {
	var id, fromID, toID, typ, propsStr, sessionID, userID string
	var weight float64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &fromID, &toID, &typ, &weight, &propsStr, &sessionID, &userID, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var props map[string]interface{}
	if err := json.Unmarshal([]byte(propsStr), &props); err != nil {
		props = make(map[string]interface{})
	}
	return &schema.Edge{
		ID:        id,
		From:      fromID,
		To:        toID,
		Type:      schema.EdgeType(typ),
		Weight:    weight,
		Properties: props,
		SessionID: sessionID,
		UserID:    userID,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func collectGNodes(rows *sql.Rows) ([]*schema.Node, error) {
	var nodes []*schema.Node
	for rows.Next() {
		n, err := scanGNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
