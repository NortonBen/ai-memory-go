package cli

import (
	"context"
	"testing"

	"github.com/NortonBen/ai-memory-go/schema"
)

type fakeGraphStore struct {
	nodesByID map[string]*schema.Node
}

func (f *fakeGraphStore) GetNode(ctx context.Context, nodeID string) (*schema.Node, error) {
	return f.nodesByID[nodeID], nil
}

func (f *fakeGraphStore) FindNodesByType(ctx context.Context, nodeType schema.NodeType) ([]*schema.Node, error) {
	out := make([]*schema.Node, 0)
	for _, n := range f.nodesByID {
		if n.Type == nodeType {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeGraphStore) FindNodesByEntity(ctx context.Context, entityName string, entityType schema.NodeType) ([]*schema.Node, error) {
	out := make([]*schema.Node, 0)
	for _, n := range f.nodesByID {
		name, _ := n.Properties["name"].(string)
		if n.Type == entityType && name == entityName {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeGraphStore) TraverseGraph(ctx context.Context, startNodeID string, depth int, filters map[string]interface{}) ([]*schema.Node, error) {
	return []*schema.Node{
		f.nodesByID[startNodeID],
	}, nil
}

func TestFindSeedNodes_ByTypedEntity(t *testing.T) {
	store := &fakeGraphStore{
		nodesByID: map[string]*schema.Node{
			"n1": {ID: "n1", Type: schema.NodeTypePerson, Properties: map[string]interface{}{"name": "Alice"}},
			"n2": {ID: "n2", Type: schema.NodeTypeOrg, Properties: map[string]interface{}{"name": "OpenAI"}},
		},
	}
	seeds, err := findSeedNodes(context.Background(), store, "Alice", string(schema.NodeTypePerson))
	if err != nil {
		t.Fatalf("findSeedNodes error: %v", err)
	}
	if len(seeds) != 1 || seeds[0].ID != "n1" {
		t.Fatalf("unexpected seeds: %#v", seeds)
	}
}

func TestFindSeedNodes_ByContainsName(t *testing.T) {
	store := &fakeGraphStore{
		nodesByID: map[string]*schema.Node{
			"n1": {ID: "n1", Type: schema.NodeTypePerson, Properties: map[string]interface{}{"name": "Alice Johnson"}},
			"n2": {ID: "n2", Type: schema.NodeTypeOrg, Properties: map[string]interface{}{"name": "OpenAI"}},
		},
	}
	seeds, err := findSeedNodes(context.Background(), store, "alice", "")
	if err != nil {
		t.Fatalf("findSeedNodes error: %v", err)
	}
	if len(seeds) != 1 || seeds[0].ID != "n1" {
		t.Fatalf("unexpected seeds: %#v", seeds)
	}
}

func TestResolveGraphSeedNodes_ForMCPNodeID(t *testing.T) {
	store := &fakeGraphStore{
		nodesByID: map[string]*schema.Node{
			"n1": {ID: "n1", Type: schema.NodeTypePerson, SessionID: "s1", Properties: map[string]interface{}{"name": "Alice"}},
			"n2": {ID: "n2", Type: schema.NodeTypePerson, SessionID: "s2", Properties: map[string]interface{}{"name": "Bob"}},
		},
	}
	seeds, err := resolveGraphSeedNodes(context.Background(), store, "", "n1", "", "s1")
	if err != nil {
		t.Fatalf("resolveGraphSeedNodes error: %v", err)
	}
	if len(seeds) != 1 || seeds[0].ID != "n1" {
		t.Fatalf("unexpected seeds: %#v", seeds)
	}
}

func TestResolveGraphSeedNodes_ForMCPEntity(t *testing.T) {
	store := &fakeGraphStore{
		nodesByID: map[string]*schema.Node{
			"n1": {ID: "n1", Type: schema.NodeTypeOrg, SessionID: "s1", Properties: map[string]interface{}{"name": "OpenAI"}},
			"n2": {ID: "n2", Type: schema.NodeTypeOrg, SessionID: "s2", Properties: map[string]interface{}{"name": "OpenAI"}},
		},
	}
	seeds, err := resolveGraphSeedNodes(context.Background(), store, "OpenAI", "", string(schema.NodeTypeOrg), "s1")
	if err != nil {
		t.Fatalf("resolveGraphSeedNodes error: %v", err)
	}
	if len(seeds) != 1 || seeds[0].SessionID != "s1" {
		t.Fatalf("unexpected seeds after session filter: %#v", seeds)
	}
}
