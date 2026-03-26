package neo4j

import (
	"context"
	"os"
	"testing"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func TestNeo4jAdapter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	uri := os.Getenv("NEO4J_URI")
	if uri == "" {
		t.Skip("NEO4J_URI not set, skipping Neo4j integration test")
	}

	username := os.Getenv("NEO4J_USER")
	password := os.Getenv("NEO4J_PASSWORD")
	if username == "" {
		username = "neo4j"
	}
	if password == "" {
		password = "password"
	}

	config := &graph.GraphConfig{
		Type:           graph.StoreTypeNeo4j,
		Username:       username,
		Password:       password,
		MaxConnections: 10,
		Options: map[string]interface{}{
			"uri": uri,
		},
	}

	store, err := NewNeo4jStore(config)
	if err != nil {
		t.Fatalf("Failed to create Neo4jStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Clean up database before tests
	// NOTE: In production or shared dev, do NOT run detach delete all
	_, err = store.executeWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
	})
	if err != nil {
		t.Fatalf("Failed to clean up database: %v", err)
	}

	// 1. Test StoreNode and GetNode
	node := schema.NewNode("TestNode", map[string]interface{}{
		"name": "IntegrationTest",
		"desc": "Testing Neo4j Adapter",
	})

	err = store.StoreNode(ctx, node)
	if err != nil {
		t.Fatalf("StoreNode failed: %v", err)
	}

	fetchedNode, err := store.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if fetchedNode.ID != node.ID {
		t.Errorf("Expected node ID %s, got %s", node.ID, fetchedNode.ID)
	}
	if fetchedNode.Properties["name"] != "IntegrationTest" {
		t.Errorf("Expected node property name to be IntegrationTest")
	}

	// 2. Test CreateRelationship
	nodeB := schema.NewNode("TestNode", map[string]interface{}{
		"name": "TargetNode",
	})
	_ = store.StoreNode(ctx, nodeB)

	edge := schema.NewEdge(node.ID, nodeB.ID, "RELATES_TO", 1.0)
	err = store.CreateRelationship(ctx, edge)
	if err != nil {
		t.Fatalf("CreateRelationship failed: %v", err)
	}

	fetchedEdge, err := store.GetRelationship(ctx, edge.ID)
	if err != nil {
		t.Fatalf("GetRelationship failed: %v", err)
	}
	if fetchedEdge.From != node.ID || fetchedEdge.To != nodeB.ID {
		t.Errorf("Edge ends mismatch")
	}

	// 3. Test Graph Traversal
	connected, err := store.FindConnected(ctx, node.ID, []schema.EdgeType{"RELATES_TO"})
	if err != nil {
		t.Fatalf("FindConnected failed: %v", err)
	}
	if len(connected) != 1 || connected[0].ID != nodeB.ID {
		t.Errorf("FindConnected did not return expected node")
	}

	// 4. Test GetNodeCount
	count, err := store.GetNodeCount(ctx)
	if err != nil {
		t.Fatalf("GetNodeCount failed: %v", err)
	}
	if count != 2 { // node A and node B
		t.Errorf("Expected 2 nodes, got %d", count)
	}

	// 5. Test Batch
	nodeC := schema.NewNode("BatchNode", map[string]interface{}{"name": "C"})
	nodeD := schema.NewNode("BatchNode", map[string]interface{}{"name": "D"})
	edgeCD := schema.NewEdge(nodeC.ID, nodeD.ID, "TEST_BATCH", 1.0)

	err = store.StoreBatch(ctx, []*schema.Node{nodeC, nodeD}, []*schema.Edge{edgeCD})
	if err != nil {
		t.Fatalf("StoreBatch failed: %v", err)
	}

	// Check total counting after batch
	totalCount, _ := store.GetNodeCount(ctx)
	if totalCount != 4 {
		t.Errorf("Expected 4 nodes after batch, got %d", totalCount)
	}

	// Clean up
	err = store.DeleteNode(ctx, node.ID)
	if err != nil {
		t.Errorf("DeleteNode failed: %v", err)
	}
}
