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
	delCurrent bool
	force      bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete memory data",
	Long: `Delete by datapoint id, or wipe an entire session (relational + vectors + graph + chat history).

Session: use --session NAME, or --current (uses -s / ~/.ai-memory/session.txt).
To delete only the global pool (empty session_id), use --session global (or shared / _).`,
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

		sessionArg := resolveDeleteSessionArg(delSession, delCurrent)

		if delID != "" || sessionArg != "" {
			err = eng.DeleteMemory(ctx, delID, sessionArg)
			if err != nil {
				color.Red("Failed to delete memory: %v", err)
				return
			}
			color.Green("Deleted successfully.")
		} else if delAll {
			color.Yellow("Global delete-all not fully supported without session ID by Engine.")
		} else {
			color.Yellow("Please specify --id, --session, --current, or --all")
			_ = cmd.Help()
		}
	},
}

func resolveDeleteSessionArg(sessionFlag string, useCurrent bool) string {
	sessionArg := strings.TrimSpace(sessionFlag)
	if sessionArg == "" && useCurrent {
		sessionArg = GetSessionRaw()
	}
	return sessionArg
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&delID, "id", "", "Delete specific memory by ID")
	deleteCmd.Flags().BoolVar(&delAll, "all", false, "Delete all memory globally")
	deleteCmd.Flags().StringVar(&delSession, "session", "", "Delete all data for this session (or global/shared/_ for unscoped pool)")
	deleteCmd.Flags().BoolVar(&delCurrent, "current", false, "Same as --session with active session from -s or ~/.ai-memory/session.txt")
	deleteCmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
}
