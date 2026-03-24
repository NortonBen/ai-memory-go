package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	delID      string
	delAll     bool
	delSession string
	force      bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete memory data",
	Long:  `Delete a specific memory by ID, all memories in a session, or all data globally.`,
	Run: func(cmd *cobra.Command, args []string) {
		if !force {
			fmt.Print("Are you sure you want to perform this deletion? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			ans, _ := reader.ReadString('\n')
			ans = strings.TrimSpace(strings.ToLower(ans))
			if ans != "y" && ans != "yes" {
				color.Yellow("Deletion aborted.")
				return
			}
		}

		ctx := context.Background()
		eng, err := InitEngine(ctx)
		if err != nil {
			color.Red("Failed to initialize engine: %v", err)
			return
		}
		defer eng.Close()

		color.Red("Warning: Deletion is a destructive operation.")
		
		if delID != "" || delSession != "" {
			err = eng.DeleteMemory(ctx, delID, delSession)
			if err != nil {
				color.Red("Failed to delete memory: %v", err)
				return
			}
			color.Green("Deleted successfully.")
		} else if delAll {
			color.Yellow("Global delete-all not fully supported without session ID by Engine.")
		} else {
			color.Yellow("Please specify --id, --session, or --all")
			_ = cmd.Help()
		}
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&delID, "id", "", "Delete specific memory by ID")
	deleteCmd.Flags().BoolVar(&delAll, "all", false, "Delete all memory globally")
	deleteCmd.Flags().StringVar(&delSession, "session", "", "Delete all memory in session")
	deleteCmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
}
