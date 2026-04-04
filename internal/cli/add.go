package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/NortonBen/ai-memory-go/engine"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	addFile string
	addURL  string
)

var addCmd = &cobra.Command{
	Use:   "add [content]",
	Short: "Add content to the AI Memory",
	Long:  `Add content directly via argument, from a file, or from a URL.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		eng, err := InitEngine(ctx)
		if err != nil {
			color.Red("Failed to initialize engine: %v", err)
			return
		}
		defer eng.Close()

		var content string

		if addFile != "" {
			data, err := os.ReadFile(addFile)
			if err != nil {
				color.Red("Error reading file: %v", err)
				return
			}
			content = string(data)
		} else if addURL != "" {
			resp, err := http.Get(addURL)
			if err != nil {
				color.Red("Error fetching URL: %v", err)
				return
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			content = string(body)
		} else if len(args) > 0 {
			// Convenience: if first arg looks like an existing file path, ingest file content.
			// This avoids accidental storing of literal file path string.
			candidate := strings.TrimSpace(args[0])
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				data, readErr := os.ReadFile(candidate)
				if readErr != nil {
					color.Red("Error reading file argument %q: %v", candidate, readErr)
					return
				}
				content = string(data)
			} else {
				content = args[0]
			}
		} else {
			color.Red("Please provide content, --file, or --url")
			_ = cmd.Help()
			return
		}

		sessionID := GetSessionID()
		dp, err := eng.Add(ctx, content, engine.WithSessionID(sessionID))
		if err != nil {
			color.Red("Error adding memory: %v", err)
			return
		}

		color.Green("Successfully added memory with ID: %s", dp.ID)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVar(&addFile, "file", "", "read content from file")
	addCmd.Flags().StringVar(&addURL, "url", "", "read content from URL")
}
