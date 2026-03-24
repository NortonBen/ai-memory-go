package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session [list|create|switch]",
	Short: "Manage CLI sessions",
	Long:  `Sessions group memory by context or project locally.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			color.Cyan("Current active session: %s", GetSessionID())
			_ = cmd.Help()
			return
		}

		switch args[0] {
		case "list":
			fmt.Println("Sessions feature stub")
		case "switch":
			if len(args) < 2 {
				color.Red("Please provide a session name to switch to")
				return
			}
			home, _ := os.UserHomeDir()
			sessionDir := filepath.Join(home, ".ai-memory")
			_ = os.MkdirAll(sessionDir, 0o750)
			sessionFile := filepath.Join(sessionDir, "session.txt")
			_ = os.WriteFile(sessionFile, []byte(args[1]), 0644)
			color.Green("Switched to session: %s", args[1])
		default:
			color.Red("Unknown session command")
		}
	},
}

func init() {
	rootCmd.AddCommand(sessionCmd)
}
