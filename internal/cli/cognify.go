package cli

import (
	"context"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var cognifyID string
var cognifySession string

var cognifyCmd = &cobra.Command{
	Use:   "cognify",
	Short: "Process content to extract entities and build the knowledge graph",
	Long:  `Run background processing manually. Sweep for pending items in a session, or process a specific ID.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		eng, err := InitEngine(ctx)
		if err != nil {
			color.Red("Failed to initialize engine: %v", err)
			return
		}
		defer eng.Close()

		if cognifyID != "" {
			color.Cyan("Starting Cognify for ID: %s", cognifyID)
			dp := &schema.DataPoint{ID: cognifyID}
			_, err = eng.Cognify(ctx, dp, engine.WithWaitCognify(true))
			if err != nil {
				color.Red("Cognify process failed: %v", err)
				return
			}
			color.Green("Cognify process completed successfully.")
		} else {
			session := cognifySession
			if session == "" {
				session = "default"
			}
			color.Cyan("Sweeping for pending items in session: %s", session)
			err = eng.CognifyPending(ctx, session)
			if err != nil {
				color.Red("Cognify pending failed: %v", err)
				return
			}
			color.Green("Bulk cognify completed successfully.")
		}
	},
}

func init() {
	rootCmd.AddCommand(cognifyCmd)
	cognifyCmd.Flags().StringVar(&cognifyID, "id", "", "ID of specific memory to process")
	cognifyCmd.Flags().StringVar(&cognifySession, "session", "default", "Session ID to sweep for pending items")
}
