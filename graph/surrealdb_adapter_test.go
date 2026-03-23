package graph

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
)

func TestSurrealDBAdapter_Integration(t *testing.T) {
	uri := os.Getenv("SURREALDB_URI")
	if uri == "" {
		t.Skip("SURREALDB_URI not set, skipping SurrealDB integration test")
	}
	user := os.Getenv("SURREALDB_USER")
	if user == "" {
		user = "root"
	}
	pass := os.Getenv("SURREALDB_PASS")
	if pass == "" {
		pass = "root"
	}
	ns := os.Getenv("SURREALDB_NS")
	if ns == "" {
		ns = "test"
	}
	db := os.Getenv("SURREALDB_DB")
	if db == "" {
		db = "test"
	}

	config := &GraphConfig{
		Type:     StoreTypeSurrealDB,
		Host:     uri,
		Username: user,
		Password: pass,
		Database: db,
		Options: map[string]interface{}{
			"namespace": ns,
		},
	}

	store, err := NewSurrealDBStore(config)
	if err != nil {
		t.Fatalf("Failed to create SurrealDB store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test 1: Store Node
	node1 := &schema.Node{
		ID:   "test_node_1",
		Type: schema.NodeTypeConcept,
		Properties: map[string]interface{}{
			"name":  "Test Concept 1",
			"score": float64(0.9),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = store.StoreNode(ctx, node1)
	if err != nil {
		t.Fatalf("Failed to store node 1: %v", err)
	}

	// Test 2: Get Node
	retrievedNode, err := store.GetNode(ctx, "test_node_1")
	if err != nil {
		t.Fatalf("Failed to get node 1: %v", err)
	}
	if retrievedNode.ID != "test_node_1" {
		t.Errorf("Expected node ID test_node_1, got %s", retrievedNode.ID)
	}

	// Test 3: Delete Node
	err = store.DeleteNode(ctx, "test_node_1")
	if err != nil {
		t.Fatalf("Failed to delete node 1: %v", err)
	}
}
