package cli

import (
	"context"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze-history",
	Short: "Analyze recent chat history to extract deeper knowledge",
	Long:  `AnalyzeHistory processes the conversation log to find new entities and relationships that might have been missed during real-time interaction.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		eng, err := InitEngine(ctx)
		if err != nil {
			color.Red("Failed to initialize engine: %v", err)
			return
		}
		defer eng.Close()

		sessionID := GetSessionID()
		color.Cyan("Analyzing history for session: %s...", sessionID)

		err = eng.AnalyzeHistory(ctx, sessionID)
		if err != nil {
			color.Red("Analysis failed: %v", err)
			return
		}

		color.Green("History analysis complete. Knowledge graph updated.")
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}
