package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NortonBen/ai-memory-go/internal/sessionid"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session [list|create|switch]",
	Short: "Manage CLI sessions",
	Long: `Sessions group memory by context or project locally.

` + sessionid.DefaultName + `: tên session mặc định trong DB (không phải “global”).
global / shared / _: khi đặt trong session.txt hoặc dùng -s, lệnh add sẽ ghi bộ nhớ dùng chung (session_id rỗng);
  Search/Think/Request vẫn dùng ngữ cảnh có tên "` + sessionid.DefaultName + `" cho lịch sử hội thoại.
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			raw := getSessionRaw()
			color.Cyan("Raw (-s / ~/.ai-memory/session.txt): %s", raw)
			color.Cyan("Engine context (search/think/request): %s", GetSessionID())
			if sessionid.IsGlobalKeyword(raw) {
				color.Yellow("Global mode: adds are shared (empty session_id); chat/search context → %q", sessionid.DefaultName)
			}
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
			name := strings.TrimSpace(args[1])
			home, _ := os.UserHomeDir()
			sessionDir := filepath.Join(home, ".ai-memory")
			_ = os.MkdirAll(sessionDir, 0o750)
			sessionFile := filepath.Join(sessionDir, "session.txt")
			_ = os.WriteFile(sessionFile, []byte(name), 0644)
			color.Green("Switched to session: %s", name)
			if sessionid.IsGlobalKeyword(name) {
				color.Yellow("Keyword global: add → shared memory; engine context for chat/search → %q", sessionid.DefaultName)
			}
		default:
			color.Red("Unknown session command")
		}
	},
}

func init() {
	rootCmd.AddCommand(sessionCmd)
}
