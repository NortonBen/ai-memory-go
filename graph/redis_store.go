package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/redis/go-redis/v9"
)

type RedisGraphStore struct {
	client *redis.Client
}

func NewRedisGraphStore(endpoint, password string) (*RedisGraphStore, error) {
	if endpoint == "" {
		endpoint = "localhost:6379"
	}
	dbInt := 2
	dbRedis, ok := os.LookupEnv("REDIS_DB_GRAPH")
	if ok {
		dbInt, _ = strconv.Atoi(dbRedis)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     endpoint,
		Password: password,
		DB:       dbInt,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisGraphStore{client: rdb}, nil
}

func (s *RedisGraphStore) StoreNode(ctx context.Context, node *schema.Node) error {
	data, _ := json.Marshal(node)
	return s.client.Set(ctx, "node:"+node.ID, data, 0).Err()
}

func (s *RedisGraphStore) GetNode(ctx context.Context, nodeID string) (*schema.Node, error) {
	data, err := s.client.Get(ctx, "node:"+nodeID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("node not found: %s", nodeID)
		}
		return nil, err
	}
	var node schema.Node
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

func (s *RedisGraphStore) UpdateNode(ctx context.Context, node *schema.Node) error {
	return s.StoreNode(ctx, node)
}

func (s *RedisGraphStore) DeleteNode(ctx context.Context, nodeID string) error {
	return s.client.Del(ctx, "node:"+nodeID).Err()
}

func (s *RedisGraphStore) CreateRelationship(ctx context.Context, edge *schema.Edge) error {
	data, _ := json.Marshal(edge)
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, "edges", edge.ID, data)
	pipe.SAdd(ctx, "adj:out:"+edge.From, edge.ID)
	pipe.SAdd(ctx, "adj:in:"+edge.To, edge.ID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisGraphStore) GetRelationship(ctx context.Context, edgeID string) (*schema.Edge, error) {
	data, err := s.client.HGet(ctx, "edges", edgeID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("edge not found: %s", edgeID)
		}
		return nil, err
	}
	var edge schema.Edge
	if err := json.Unmarshal(data, &edge); err != nil {
		return nil, err
	}
	return &edge, nil
}

func (s *RedisGraphStore) UpdateRelationship(ctx context.Context, edge *schema.Edge) error {
	data, _ := json.Marshal(edge)
	return s.client.HSet(ctx, "edges", edge.ID, data).Err()
}

func (s *RedisGraphStore) DeleteRelationship(ctx context.Context, edgeID string) error {
	edge, err := s.GetRelationship(ctx, edgeID)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.SRem(ctx, "adj:out:"+edge.From, edge.ID)
	pipe.SRem(ctx, "adj:in:"+edge.To, edge.ID)
	pipe.HDel(ctx, "edges", edgeID)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisGraphStore) getAdjacentEdges(ctx context.Context, nodeID string) ([]*schema.Edge, error) {
	outEdgeIDs, _ := s.client.SMembers(ctx, "adj:out:"+nodeID).Result()
	inEdgeIDs, _ := s.client.SMembers(ctx, "adj:in:"+nodeID).Result()

	edgeIDs := append(outEdgeIDs, inEdgeIDs...)
	if len(edgeIDs) == 0 {
		return nil, nil
	}

	res, err := s.client.HMGet(ctx, "edges", edgeIDs...).Result()
	if err != nil {
		return nil, err
	}

	var edges []*schema.Edge
	for _, v := range res {
		if str, ok := v.(string); ok {
			var edge schema.Edge
			if err := json.Unmarshal([]byte(str), &edge); err == nil {
				edges = append(edges, &edge)
			}
		}
	}
	return edges, nil
}

func (s *RedisGraphStore) TraverseGraph(ctx context.Context, startNodeID string, depth int, filters map[string]interface{}) ([]*schema.Node, error) {
	visited := make(map[string]bool)
	queue := []string{startNodeID}

	var nodes []*schema.Node

	currentDepth := 0
	for len(queue) > 0 && currentDepth <= depth {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			curr := queue[0]
			queue = queue[1:]

			if visited[curr] {
				continue
			}
			visited[curr] = true

			n, err := s.GetNode(ctx, curr)
			if err == nil {
				match := true
				if filters != nil {
					for k, v := range filters {
						if k == "type" && string(n.Type) != v.(string) {
							match = false
							break
						}
					}
				}
				if match {
					nodes = append(nodes, n)
				}
			}

			if currentDepth < depth {
				edges, _ := s.getAdjacentEdges(ctx, curr)
				for _, e := range edges {
					if e.From == curr && !visited[e.To] {
						queue = append(queue, e.To)
					} else if e.To == curr && !visited[e.From] {
						queue = append(queue, e.From)
					}
				}
			}
		}
		currentDepth++
	}

	return nodes, nil
}

func (s *RedisGraphStore) FindConnected(ctx context.Context, nodeID string, edgeTypes []schema.EdgeType) ([]*schema.Node, error) {
	edges, _ := s.getAdjacentEdges(ctx, nodeID)
	typeMap := make(map[schema.EdgeType]bool)
	for _, t := range edgeTypes {
		typeMap[t] = true
	}

	visited := make(map[string]bool)
	var nodes []*schema.Node

	for _, e := range edges {
		if len(typeMap) > 0 && !typeMap[e.Type] {
			continue
		}
		var targetID string
		if e.From == nodeID {
			targetID = e.To
		} else {
			targetID = e.From
		}

		if !visited[targetID] {
			visited[targetID] = true
			n, err := s.GetNode(ctx, targetID)
			if err == nil {
				nodes = append(nodes, n)
			}
		}
	}
	return nodes, nil
}

func (s *RedisGraphStore) FindPath(ctx context.Context, fromNodeID, toNodeID string, maxDepth int) ([]*schema.Node, error) {
	// A basic BFS to find the shortest path
	visited := make(map[string]string) // node -> parent
	queue := []string{fromNodeID}
	visited[fromNodeID] = ""

	found := false
	currentDepth := 0

	for len(queue) > 0 && currentDepth <= maxDepth {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			curr := queue[0]
			queue = queue[1:]

			if curr == toNodeID {
				found = true
				break
			}

			edges, _ := s.getAdjacentEdges(ctx, curr)
			for _, e := range edges {
				next := e.To
				if e.To == curr {
					next = e.From
				}

				if _, ok := visited[next]; !ok {
					visited[next] = curr
					queue = append(queue, next)
				}
			}
		}
		if found {
			break
		}
		currentDepth++
	}

	if !found {
		return nil, fmt.Errorf("path not found")
	}

	// Reconstruct path
	var path []string
	curr := toNodeID
	for curr != "" {
		path = append([]string{curr}, path...) // Prepend
		curr = visited[curr]
	}

	var nodes []*schema.Node
	for _, id := range path {
		n, err := s.GetNode(ctx, id)
		if err == nil {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

// Simple implementations for the rest scanning keys or using naive approaches
func (s *RedisGraphStore) FindNodesByType(ctx context.Context, nodeType schema.NodeType) ([]*schema.Node, error) {
	// In production, you'd use a RediSearch index or a SET of nodes by type
	// For now, this requires SCAN
	var cursor uint64
	var nodes []*schema.Node

	for {
		var keys []string
		var err error
		keys, cursor, err = s.client.Scan(ctx, cursor, "node:*", 100).Result()
		if err != nil {
			return nil, err
		}

		if len(keys) > 0 {
			res, _ := s.client.MGet(ctx, keys...).Result()
			for _, v := range res {
				if str, ok := v.(string); ok {
					var node schema.Node
					if err := json.Unmarshal([]byte(str), &node); err == nil {
						if node.Type == nodeType {
							nodes = append(nodes, &node)
						}
					}
				}
			}
		}

		if cursor == 0 {
			break
		}
	}
	return nodes, nil
}

func (s *RedisGraphStore) FindNodesByProperty(ctx context.Context, property string, value interface{}) ([]*schema.Node, error) {
	return []*schema.Node{}, nil
}

func (s *RedisGraphStore) FindNodesByEntity(ctx context.Context, entityName string, entityType schema.NodeType) ([]*schema.Node, error) {
	// Simple scan implementation
	var cursor uint64
	var nodes []*schema.Node

	for {
		var keys []string
		var err error
		keys, cursor, err = s.client.Scan(ctx, cursor, "node:*", 100).Result()
		if err != nil {
			return nil, err
		}

		if len(keys) > 0 {
			res, _ := s.client.MGet(ctx, keys...).Result()
			for _, v := range res {
				if str, ok := v.(string); ok {
					var node schema.Node
					if err := json.Unmarshal([]byte(str), &node); err == nil {
						if node.Properties["name"] == entityName && node.Type == entityType {
							nodes = append(nodes, &node)
						}
					}
				}
			}
		}

		if cursor == 0 {
			break
		}
	}
	return nodes, nil
}

func (s *RedisGraphStore) StoreBatch(ctx context.Context, nodes []*schema.Node, edges []*schema.Edge) error {
	pipe := s.client.Pipeline()
	for _, n := range nodes {
		data, _ := json.Marshal(n)
		pipe.Set(ctx, "node:"+n.ID, data, 0)
	}
	for _, e := range edges {
		data, _ := json.Marshal(e)
		pipe.HSet(ctx, "edges", e.ID, data)
		pipe.SAdd(ctx, "adj:out:"+e.From, e.ID)
		pipe.SAdd(ctx, "adj:in:"+e.To, e.ID)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisGraphStore) DeleteBatch(ctx context.Context, nodeIDs []string, edgeIDs []string) error {
	if len(nodeIDs) == 0 && len(edgeIDs) == 0 {
		return nil
	}

	// Complex deletion inside batch is tricky due to adjacent sets.
	// For simplicity, we delete the base keys.
	keys := make([]string, len(nodeIDs))
	for i, id := range nodeIDs {
		keys[i] = "node:" + id
	}
	if len(keys) > 0 {
		s.client.Del(ctx, keys...)
	}
	if len(edgeIDs) > 0 {
		s.client.HDel(ctx, "edges", edgeIDs...)
	}
	return nil
}

func (s *RedisGraphStore) GetNodeCount(ctx context.Context) (int64, error) {
	var cursor uint64
	var count int64

	for {
		var keys []string
		var err error
		keys, cursor, err = s.client.Scan(ctx, cursor, "node:*", 1000).Result()
		if err != nil {
			return 0, err
		}

		count += int64(len(keys))

		if cursor == 0 {
			break
		}
	}
	return count, nil
}

func (s *RedisGraphStore) GetEdgeCount(ctx context.Context) (int64, error) {
	return s.client.HLen(ctx, "edges").Result()
}

func (s *RedisGraphStore) GetConnectedComponents(ctx context.Context) ([][]string, error) {
	return [][]string{}, nil
}

func (s *RedisGraphStore) Health(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisGraphStore) Close() error {
	return s.client.Close()
}
