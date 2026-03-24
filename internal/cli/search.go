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
	searchLimit int
	searchMode  string
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search memory knowledge base",
	Long:  `Search the knowledge base using semantic, graph, or hybrid mode.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		eng, err := InitEngine(ctx)
		if err != nil {
			color.Red("Failed to initialize engine: %v", err)
			return
		}
		defer eng.Close()

		queryText := args[0]
		sessionID := GetSessionID()

		query := &schema.SearchQuery{
			Text:      queryText,
			SessionID: sessionID,
			Limit:     searchLimit,
		}

		resp, err := eng.Search(ctx, query)
		if err != nil {
			color.Red("Search failed: %v", err)
			return
		}

		if output == "json" {
			b, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Println(string(b))
			return
		}

		if output == "yaml" {
			b, _ := yaml.Marshal(resp)
			fmt.Println(string(b))
			return
		}

		color.Cyan("Search Results for: %q", queryText)
		fmt.Println(strings.Repeat("-", 40))
		if resp.Answer != "" {
			color.Yellow("Engine Answer:")
			fmt.Println(resp.Answer)
			fmt.Println(strings.Repeat("-", 40))
		}
		color.Yellow("Retrieved Memories:")
		for i, res := range resp.Results {
			color.Green("%d. [Score: %.2f]", i+1, res.Score)
			fmt.Println(res.DataPoint.Content)
			fmt.Println()
		}
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 5, "maximum number of results")
	searchCmd.Flags().StringVarP(&searchMode, "mode", "m", "semantic", "search mode (semantic, graph, hybrid)")
}
