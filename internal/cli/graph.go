package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	graphDepth    int
	graphLimit    int
	graphNodeType string
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Query knowledge graph directly",
}

var graphQueryCmd = &cobra.Command{
	Use:   "query [entity_name]",
	Short: "Expand a subgraph from an entity name",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		runtime, err := initRuntime(ctx)
		if err != nil {
			color.Red("Failed to initialize runtime: %v", err)
			return
		}
		defer runtime.Engine.Close()

		entityName := strings.TrimSpace(args[0])
		if entityName == "" {
			color.Red("entity_name cannot be empty")
			return
		}

		seeds, err := findSeedNodes(ctx, runtime.GraphStore, entityName, graphNodeType)
		if err != nil {
			color.Red("Graph query failed: %v", err)
			return
		}
		seeds = filterNodesBySession(seeds, GetSessionID())
		if len(seeds) == 0 {
			color.Yellow("No graph seed nodes found for %q", entityName)
			return
		}

		nodesByID := make(map[string]*schema.Node)
		for _, seed := range seeds {
			nodesByID[seed.ID] = seed
			neighbors, trErr := runtime.GraphStore.TraverseGraph(ctx, seed.ID, graphDepth, nil)
			if trErr != nil {
				color.Red("Failed graph traversal for seed %s: %v", seed.ID, trErr)
				continue
			}
			for _, n := range filterNodesBySession(neighbors, GetSessionID()) {
				nodesByID[n.ID] = n
			}
		}

		nodes := mapValues(nodesByID)
		if graphLimit > 0 && len(nodes) > graphLimit {
			nodes = nodes[:graphLimit]
		}

		out := map[string]any{
			"query":      entityName,
			"session_id": GetSessionID(),
			"depth":      graphDepth,
			"seed_count": len(seeds),
			"node_count": len(nodes),
			"seeds":      seeds,
			"nodes":      nodes,
		}

		switch output {
		case "json":
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
			return
		case "yaml":
			b, _ := yaml.Marshal(out)
			fmt.Println(string(b))
			return
		}

		color.Cyan("Graph query: %q", entityName)
		fmt.Printf("Session: %s | Depth: %d\n", GetSessionID(), graphDepth)
		fmt.Printf("Seed nodes: %d | Total nodes: %d\n\n", len(seeds), len(nodes))
		for i, n := range nodes {
			name, _ := n.Properties["name"].(string)
			if strings.TrimSpace(name) == "" {
				name = n.ID
			}
			color.Green("%d. %s (%s)", i+1, name, n.Type)
		}
	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.AddCommand(graphQueryCmd)
	graphQueryCmd.Flags().IntVar(&graphDepth, "depth", 2, "graph traversal depth")
	graphQueryCmd.Flags().IntVar(&graphLimit, "limit", 50, "maximum number of nodes returned")
	graphQueryCmd.Flags().StringVar(&graphNodeType, "node-type", "", "optional seed node type (Person, Org, Project, ...)")
}

func findSeedNodes(ctx context.Context, store interface {
	FindNodesByType(ctx context.Context, nodeType schema.NodeType) ([]*schema.Node, error)
	FindNodesByEntity(ctx context.Context, entityName string, entityType schema.NodeType) ([]*schema.Node, error)
}, entityName string, nodeType string) ([]*schema.Node, error) {
	typed := strings.TrimSpace(nodeType)
	if typed != "" {
		return store.FindNodesByEntity(ctx, entityName, schema.NodeType(typed))
	}

	var out []*schema.Node
	target := strings.ToLower(entityName)
	for _, nt := range allNodeTypesForQuery() {
		nodes, err := store.FindNodesByType(ctx, nt)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			name, _ := n.Properties["name"].(string)
			if strings.Contains(strings.ToLower(name), target) {
				out = append(out, n)
			}
		}
	}
	return out, nil
}

func allNodeTypesForQuery() []schema.NodeType {
	return []schema.NodeType{
		schema.NodeTypeEntity,
		schema.NodeTypePerson,
		schema.NodeTypeOrg,
		schema.NodeTypeProject,
		schema.NodeTypeTask,
		schema.NodeTypeEvent,
		schema.NodeTypeDocument,
		schema.NodeTypeSession,
		schema.NodeTypeUser,
		schema.NodeTypeConcept,
		schema.NodeTypeWord,
		schema.NodeTypeUserPreference,
		schema.NodeTypeGrammarRule,
	}
}

func filterNodesBySession(nodes []*schema.Node, sessionID string) []*schema.Node {
	out := make([]*schema.Node, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if sessionID == "" || n.SessionID == "" || n.SessionID == sessionID {
			out = append(out, n)
		}
	}
	return out
}

func mapValues(in map[string]*schema.Node) []*schema.Node {
	out := make([]*schema.Node, 0, len(in))
	for _, n := range in {
		out = append(out, n)
	}
	return out
}
