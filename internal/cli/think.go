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
	thinkLimit       int
	thinkMaxSteps    int
	thinkIncludeRsn  bool
	thinkLearnRels   bool
)

var thinkCmd = &cobra.Command{
	Use:   "think [query]",
	Short: "Iteratively think and answer using the memory engine",
	Long:  `Think allows the engine to autonomously navigate the knowledge graph if it needs more context.`,
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

		query := &schema.ThinkQuery{
			Text:               queryText,
			SessionID:          sessionID,
			Limit:              thinkLimit,
			EnableThinking:     true,
			MaxThinkingSteps:   thinkMaxSteps,
			IncludeReasoning:   thinkIncludeRsn,
			LearnRelationships: thinkLearnRels,
		}

		resp, err := eng.Think(ctx, query)
		if err != nil {
			color.Red("Think failed: %v", err)
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

		color.Cyan("Thinking Process & Answer for: %q", queryText)
		fmt.Println(strings.Repeat("-", 40))

		if resp.Reasoning != "" {
			color.Magenta("Reasoning:")
			fmt.Println(resp.Reasoning)
			fmt.Println(strings.Repeat("-", 40))
		}

		color.Yellow("Final Answer:")
		fmt.Println(resp.Answer)
		fmt.Println(strings.Repeat("-", 40))
	},
}

func init() {
	rootCmd.AddCommand(thinkCmd)
	thinkCmd.Flags().IntVarP(&thinkLimit, "limit", "l", 5, "maximum number of initial context results")
	thinkCmd.Flags().IntVarP(&thinkMaxSteps, "steps", "t", 3, "maximum number of thinking hops in the graph")
	thinkCmd.Flags().BoolVarP(&thinkIncludeRsn, "reasoning", "r", true, "output reasoning trace")
	thinkCmd.Flags().BoolVar(&thinkLearnRels, "learn", true, "learn new bridging relationships automatically")
}
