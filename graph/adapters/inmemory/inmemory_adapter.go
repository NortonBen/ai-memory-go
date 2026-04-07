package inmemory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
)

func init() {
	graph.RegisterStore(graph.StoreTypeInMemory, func(config *graph.GraphConfig) (graph.GraphStore, error) {
		return NewInMemoryGraphStore(), nil
	})
}

// InMemoryGraphStore implements GraphStore using in-memory adjacency lists
type InMemoryGraphStore struct {
	nodes         map[string]*schema.Node
	edges         map[string]*schema.Edge
	adjacencyList map[string][]string // nodeID -> []edgeIDs
	reverseList   map[string][]string // nodeID -> []edgeIDs (incoming)
	mu            sync.RWMutex
}

// NewInMemoryGraphStore creates a new in-memory graph store
func NewInMemoryGraphStore() *InMemoryGraphStore {
	return &InMemoryGraphStore{
		nodes:         make(map[string]*schema.Node),
		edges:         make(map[string]*schema.Edge),
		adjacencyList: make(map[string][]string),
		reverseList:   make(map[string][]string),
	}
}

// StoreNode stores a node in the graph
func (img *InMemoryGraphStore) StoreNode(ctx context.Context, node *schema.Node) error {
	if err := node.Validate(); err != nil {
		return fmt.Errorf("invalid node: %w", err)
	}

	img.mu.Lock()
	defer img.mu.Unlock()

	img.nodes[node.ID] = node
	
	// Initialize adjacency lists if not exists
	if _, exists := img.adjacencyList[node.ID]; !exists {
		img.adjacencyList[node.ID] = make([]string, 0)
	}
	if _, exists := img.reverseList[node.ID]; !exists {
		img.reverseList[node.ID] = make([]string, 0)
	}

	return nil
}

// GetNode retrieves a node by ID
func (img *InMemoryGraphStore) GetNode(ctx context.Context, nodeID string) (*schema.Node, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	node, exists := img.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	return node, nil
}

// UpdateNode updates an existing node
func (img *InMemoryGraphStore) UpdateNode(ctx context.Context, node *schema.Node) error {
	if err := node.Validate(); err != nil {
		return fmt.Errorf("invalid node: %w", err)
	}

	img.mu.Lock()
	defer img.mu.Unlock()

	if _, exists := img.nodes[node.ID]; !exists {
		return fmt.Errorf("node %s not found", node.ID)
	}

	img.nodes[node.ID] = node
	return nil
}

// DeleteNode deletes a node and its associated edges
func (img *InMemoryGraphStore) DeleteNode(ctx context.Context, nodeID string) error {
	img.mu.Lock()
	defer img.mu.Unlock()

	if _, exists := img.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	// Delete all outgoing edges
	if edgeIDs, exists := img.adjacencyList[nodeID]; exists {
		for _, edgeID := range edgeIDs {
			delete(img.edges, edgeID)
		}
		delete(img.adjacencyList, nodeID)
	}

	// Delete all incoming edges
	if edgeIDs, exists := img.reverseList[nodeID]; exists {
		for _, edgeID := range edgeIDs {
			delete(img.edges, edgeID)
		}
		delete(img.reverseList, nodeID)
	}

	// Delete the node
	delete(img.nodes, nodeID)

	return nil
}

// CreateRelationship creates an edge between two nodes
func (img *InMemoryGraphStore) CreateRelationship(ctx context.Context, edge *schema.Edge) error {
	if err := edge.Validate(); err != nil {
		return fmt.Errorf("invalid edge: %w", err)
	}

	img.mu.Lock()
	defer img.mu.Unlock()

	// Check if nodes exist
	if _, exists := img.nodes[edge.From]; !exists {
		return fmt.Errorf("from node %s not found", edge.From)
	}
	if _, exists := img.nodes[edge.To]; !exists {
		return fmt.Errorf("to node %s not found", edge.To)
	}

	// Store edge
	img.edges[edge.ID] = edge

	// Update adjacency lists
	img.adjacencyList[edge.From] = append(img.adjacencyList[edge.From], edge.ID)
	img.reverseList[edge.To] = append(img.reverseList[edge.To], edge.ID)

	return nil
}

// GetRelationship retrieves an edge by ID
func (img *InMemoryGraphStore) GetRelationship(ctx context.Context, edgeID string) (*schema.Edge, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	edge, exists := img.edges[edgeID]
	if !exists {
		return nil, fmt.Errorf("edge %s not found", edgeID)
	}

	return edge, nil
}

// UpdateRelationship updates an existing edge
func (img *InMemoryGraphStore) UpdateRelationship(ctx context.Context, edge *schema.Edge) error {
	if err := edge.Validate(); err != nil {
		return fmt.Errorf("invalid edge: %w", err)
	}

	img.mu.Lock()
	defer img.mu.Unlock()

	if _, exists := img.edges[edge.ID]; !exists {
		return fmt.Errorf("edge %s not found", edge.ID)
	}

	img.edges[edge.ID] = edge
	return nil
}

// DeleteRelationship deletes an edge
func (img *InMemoryGraphStore) DeleteRelationship(ctx context.Context, edgeID string) error {
	img.mu.Lock()
	defer img.mu.Unlock()

	edge, exists := img.edges[edgeID]
	if !exists {
		return fmt.Errorf("edge %s not found", edgeID)
	}

	// Remove from adjacency lists
	img.removeEdgeFromList(img.adjacencyList[edge.From], edgeID)
	img.removeEdgeFromList(img.reverseList[edge.To], edgeID)

	// Delete edge
	delete(img.edges, edgeID)

	return nil
}

// removeEdgeFromList removes an edge ID from a list
func (img *InMemoryGraphStore) removeEdgeFromList(list []string, edgeID string) []string {
	for i, id := range list {
		if id == edgeID {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

// TraverseGraph traverses the graph from a starting node
func (img *InMemoryGraphStore) TraverseGraph(ctx context.Context, startNodeID string, depth int, filters map[string]interface{}) ([]*schema.Node, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	if _, exists := img.nodes[startNodeID]; !exists {
		return nil, fmt.Errorf("start node %s not found", startNodeID)
	}

	visited := make(map[string]bool)
	result := make([]*schema.Node, 0)

	img.traverseRecursive(startNodeID, depth, 0, visited, &result, filters)

	return result, nil
}

// traverseRecursive performs recursive graph traversal
func (img *InMemoryGraphStore) traverseRecursive(nodeID string, maxDepth, currentDepth int, visited map[string]bool, result *[]*schema.Node, filters map[string]interface{}) {
	if currentDepth > maxDepth || visited[nodeID] {
		return
	}

	visited[nodeID] = true
	node := img.nodes[nodeID]

	// Apply filters
	if img.matchesFilters(node, filters) {
		*result = append(*result, node)
	}

	// Traverse neighbors
	if edgeIDs, exists := img.adjacencyList[nodeID]; exists {
		for _, edgeID := range edgeIDs {
			edge := img.edges[edgeID]
			img.traverseRecursive(edge.To, maxDepth, currentDepth+1, visited, result, filters)
		}
	}
}

// FindConnected finds nodes connected to a given node by specific edge types
func (img *InMemoryGraphStore) FindConnected(ctx context.Context, nodeID string, edgeTypes []schema.EdgeType) ([]*schema.Node, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	if _, exists := img.nodes[nodeID]; !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	result := make([]*schema.Node, 0)
	edgeTypeMap := make(map[schema.EdgeType]bool)
	for _, et := range edgeTypes {
		edgeTypeMap[et] = true
	}

	// Find outgoing connections
	if edgeIDs, exists := img.adjacencyList[nodeID]; exists {
		for _, edgeID := range edgeIDs {
			edge := img.edges[edgeID]
			if len(edgeTypes) == 0 || edgeTypeMap[edge.Type] {
				if targetNode, exists := img.nodes[edge.To]; exists {
					result = append(result, targetNode)
				}
			}
		}
	}

	return result, nil
}

// FindPath finds a path between two nodes
func (img *InMemoryGraphStore) FindPath(ctx context.Context, fromNodeID, toNodeID string, maxDepth int) ([]*schema.Node, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	if _, exists := img.nodes[fromNodeID]; !exists {
		return nil, fmt.Errorf("from node %s not found", fromNodeID)
	}
	if _, exists := img.nodes[toNodeID]; !exists {
		return nil, fmt.Errorf("to node %s not found", toNodeID)
	}

	// BFS to find shortest path
	queue := [][]string{{fromNodeID}}
	visited := make(map[string]bool)

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		currentNodeID := path[len(path)-1]

		if currentNodeID == toNodeID {
			// Found path, convert to nodes
			result := make([]*schema.Node, len(path))
			for i, nodeID := range path {
				result[i] = img.nodes[nodeID]
			}
			return result, nil
		}

		if len(path) >= maxDepth {
			continue
		}

		if visited[currentNodeID] {
			continue
		}
		visited[currentNodeID] = true

		// Explore neighbors
		if edgeIDs, exists := img.adjacencyList[currentNodeID]; exists {
			for _, edgeID := range edgeIDs {
				edge := img.edges[edgeID]
				newPath := make([]string, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = edge.To
				queue = append(queue, newPath)
			}
		}
	}

	return nil, fmt.Errorf("no path found between %s and %s", fromNodeID, toNodeID)
}

// FindNodesByType finds all nodes of a specific type
func (img *InMemoryGraphStore) FindNodesByType(ctx context.Context, nodeType schema.NodeType) ([]*schema.Node, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	result := make([]*schema.Node, 0)
	for _, node := range img.nodes {
		if node.Type == nodeType {
			result = append(result, node)
		}
	}

	return result, nil
}

// FindNodesByProperty finds nodes with a specific property value
func (img *InMemoryGraphStore) FindNodesByProperty(ctx context.Context, property string, value interface{}) ([]*schema.Node, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	result := make([]*schema.Node, 0)
	for _, node := range img.nodes {
		if propValue, exists := node.Properties[property]; exists && propValue == value {
			result = append(result, node)
		}
	}

	return result, nil
}

// FindNodesByEntity finds nodes by entity name and type. If entityType is empty, matches any type.
func (img *InMemoryGraphStore) FindNodesByEntity(ctx context.Context, entityName string, entityType schema.NodeType) ([]*schema.Node, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	result := make([]*schema.Node, 0)
	for _, node := range img.nodes {
		if entityType == "" || node.Type == entityType {
			if name, ok := node.Properties["name"].(string); ok && name == entityName {
				result = append(result, node)
			}
		}
	}

	return result, nil
}

// StoreBatch stores multiple nodes and edges in a batch
func (img *InMemoryGraphStore) StoreBatch(ctx context.Context, nodes []*schema.Node, edges []*schema.Edge) error {
	img.mu.Lock()
	defer img.mu.Unlock()

	// Store all nodes first
	for _, node := range nodes {
		if err := node.Validate(); err != nil {
			return fmt.Errorf("invalid node %s: %w", node.ID, err)
		}
		img.nodes[node.ID] = node
		if _, exists := img.adjacencyList[node.ID]; !exists {
			img.adjacencyList[node.ID] = make([]string, 0)
		}
		if _, exists := img.reverseList[node.ID]; !exists {
			img.reverseList[node.ID] = make([]string, 0)
		}
	}

	// Store all edges
	for _, edge := range edges {
		if err := edge.Validate(); err != nil {
			return fmt.Errorf("invalid edge %s: %w", edge.ID, err)
		}
		img.edges[edge.ID] = edge
		img.adjacencyList[edge.From] = append(img.adjacencyList[edge.From], edge.ID)
		img.reverseList[edge.To] = append(img.reverseList[edge.To], edge.ID)
	}

	return nil
}

// DeleteBatch deletes multiple nodes and edges
func (img *InMemoryGraphStore) DeleteBatch(ctx context.Context, nodeIDs []string, edgeIDs []string) error {
	img.mu.Lock()
	defer img.mu.Unlock()

	// Delete edges first
	for _, edgeID := range edgeIDs {
		if edge, exists := img.edges[edgeID]; exists {
			img.removeEdgeFromList(img.adjacencyList[edge.From], edgeID)
			img.removeEdgeFromList(img.reverseList[edge.To], edgeID)
			delete(img.edges, edgeID)
		}
	}

	// Delete nodes
	for _, nodeID := range nodeIDs {
		delete(img.nodes, nodeID)
		delete(img.adjacencyList, nodeID)
		delete(img.reverseList, nodeID)
	}

	return nil
}

// DeleteGraphBySessionID implements GraphStore.
func (img *InMemoryGraphStore) DeleteGraphBySessionID(ctx context.Context, sessionID string) error {
	match := func(s string) bool {
		if strings.TrimSpace(sessionID) == "" {
			return strings.TrimSpace(s) == ""
		}
		return s == sessionID
	}

	img.mu.Lock()
	defer img.mu.Unlock()

	var edgeIDs []string
	for id, e := range img.edges {
		if match(e.SessionID) {
			edgeIDs = append(edgeIDs, id)
		}
	}
	for _, edgeID := range edgeIDs {
		if edge, exists := img.edges[edgeID]; exists {
			img.removeEdgeFromList(img.adjacencyList[edge.From], edgeID)
			img.removeEdgeFromList(img.reverseList[edge.To], edgeID)
			delete(img.edges, edgeID)
		}
	}

	var nodeIDs []string
	for id, n := range img.nodes {
		if match(n.SessionID) {
			nodeIDs = append(nodeIDs, id)
		}
	}
	for _, nodeID := range nodeIDs {
		delete(img.nodes, nodeID)
		delete(img.adjacencyList, nodeID)
		delete(img.reverseList, nodeID)
	}
	return nil
}

// GetNodeCount returns the total number of nodes
func (img *InMemoryGraphStore) GetNodeCount(ctx context.Context) (int64, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	return int64(len(img.nodes)), nil
}

// ListNodes returns all nodes in the graph
func (img *InMemoryGraphStore) ListNodes(ctx context.Context) ([]*schema.Node, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	nodes := make([]*schema.Node, 0, len(img.nodes))
	for _, node := range img.nodes {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// GetEdgeCount returns the total number of edges
func (img *InMemoryGraphStore) GetEdgeCount(ctx context.Context) (int64, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	return int64(len(img.edges)), nil
}

// ListEdges returns all edges in the graph
func (img *InMemoryGraphStore) ListEdges(ctx context.Context) ([]*schema.Edge, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	edges := make([]*schema.Edge, 0, len(img.edges))
	for _, edge := range img.edges {
		edges = append(edges, edge)
	}
	return edges, nil
}

// GetConnectedComponents finds all connected components in the graph
func (img *InMemoryGraphStore) GetConnectedComponents(ctx context.Context) ([][]string, error) {
	img.mu.RLock()
	defer img.mu.RUnlock()

	visited := make(map[string]bool)
	components := make([][]string, 0)

	for nodeID := range img.nodes {
		if !visited[nodeID] {
			component := make([]string, 0)
			img.dfsComponent(nodeID, visited, &component)
			components = append(components, component)
		}
	}

	return components, nil
}

// dfsComponent performs DFS to find a connected component
func (img *InMemoryGraphStore) dfsComponent(nodeID string, visited map[string]bool, component *[]string) {
	visited[nodeID] = true
	*component = append(*component, nodeID)

	// Visit neighbors
	if edgeIDs, exists := img.adjacencyList[nodeID]; exists {
		for _, edgeID := range edgeIDs {
			edge := img.edges[edgeID]
			if !visited[edge.To] {
				img.dfsComponent(edge.To, visited, component)
			}
		}
	}
}

// Health checks if the graph store is healthy
func (img *InMemoryGraphStore) Health(ctx context.Context) error {
	return nil // In-memory store is always healthy
}

// Close closes the graph store
func (img *InMemoryGraphStore) Close() error {
	img.mu.Lock()
	defer img.mu.Unlock()

	img.nodes = make(map[string]*schema.Node)
	img.edges = make(map[string]*schema.Edge)
	img.adjacencyList = make(map[string][]string)
	img.reverseList = make(map[string][]string)

	return nil
}

// matchesFilters checks if a node matches the given filters
func (img *InMemoryGraphStore) matchesFilters(node *schema.Node, filters map[string]interface{}) bool {
	if len(filters) == 0 {
		return true
	}

	for key, value := range filters {
		if key == "type" {
			if node.Type != value.(schema.NodeType) {
				return false
			}
		} else if propValue, exists := node.Properties[key]; !exists || propValue != value {
			return false
		}
	}

	return true
}
