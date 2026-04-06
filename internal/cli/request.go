package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	requestHopDepth   int
	requestIncludeRsn bool
	requestLearnRels  bool
	requestFourTier   bool
)

var requestCmd = &cobra.Command{
	Use:   "request [message]",
	Short: "Send a conversational message to the memory engine",
	Long:  `Request processes a user message, extracts intent, updates memory, and generates an answer using context from history and graph.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		eng, err := InitEngine(ctx)
		if err != nil {
			color.Red("Failed to initialize engine: %v", err)
			return
		}
		defer eng.Close()

		message := args[0]
		sessionID := GetSessionID()

		opts := []engine.RequestOption{
			engine.WithHopDepth(requestHopDepth),
			engine.WithEnableThinking(true),
			engine.WithIncludeReasoning(requestIncludeRsn),
			engine.WithLearnRelationships(requestLearnRels),
		}
		if requestFourTier {
			en := true
			opts = append(opts, engine.WithRequestFourTier(&schema.FourTierSearchOptions{Enabled: &en}))
		}

		resp, err := eng.Request(ctx, sessionID, message, opts...)
		if err != nil {
			color.Red("Request failed: %v", err)
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

		color.Cyan("Engine Thinking Process & Result")
		fmt.Println(strings.Repeat("-", 40))

		if resp.Intent != nil {
			color.Blue("Detected Intent: [Query: %v, Delete: %v]", resp.Intent.IsQuery, resp.Intent.IsDelete)
			if verbose {
				fmt.Printf("Reasoning: %s\n", resp.Intent.Reasoning)
			}
			fmt.Println(strings.Repeat("-", 40))
		}

		if resp.Reasoning != "" && requestIncludeRsn {
			color.Magenta("Memory Retrieval Reasoning:")
			fmt.Println(resp.Reasoning)
			fmt.Println(strings.Repeat("-", 40))
		}

		color.Yellow("Final Answer:")
		fmt.Println(resp.Answer)
		fmt.Println(strings.Repeat("-", 40))
	},
}

func init() {
	rootCmd.AddCommand(requestCmd)
	requestCmd.Flags().IntVarP(&requestHopDepth, "hop-depth", "d", 2, "number of graph hops to explore during retrieval")
	requestCmd.Flags().BoolVarP(&requestIncludeRsn, "reasoning", "r", true, "display reasoning steps")
	requestCmd.Flags().BoolVar(&requestLearnRels, "learn", true, "enable automatic learning of new bridging relationships")
	requestCmd.Flags().BoolVar(&requestFourTier, "four-tier", false, "enable four-tier retrieval for query intent (overrides engine default off)")
}
