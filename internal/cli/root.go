package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/NortonBen/ai-memory-go/internal/sessionid"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	session string
	verbose bool
	output  string
)

var rootCmd = &cobra.Command{
	Use:   "ai-memory-cli",
	Short: "CLI tool for ai-memory-go",
	Long:  `A command-line interface for the AI Memory Integration system, providing operations to add, cognify, search, and delete memory nodes.`,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ai-memory.yaml)")
	rootCmd.PersistentFlags().StringVarP(&session, "session", "s", "default", `active memory session (named), or keyword "global"/"shared"/"_" to add shared rows with empty session_id; see session help`)
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "display detailed operation information")
	rootCmd.PersistentFlags().StringVarP(&output, "format", "f", "text", "output format (text, json, yaml)")
}

// getSessionRaw returns the persistent session string from -s or ~/.ai-memory/session.txt (trimmed).
func getSessionRaw() string {
	flag := strings.TrimSpace(session)
	if flag != "" && flag != sessionid.DefaultName {
		return flag
	}
	home, err := os.UserHomeDir()
	if err == nil {
		sessionFile := filepath.Join(home, ".ai-memory", "session.txt")
		if data, err := os.ReadFile(sessionFile); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return sessionid.DefaultName
}

// GetSessionID returns the engine session for Search, Think, Request (chat history, scoping).
// Global keywords map to the named default workspace so history is not keyed under the word "global".
func GetSessionID() string {
	return sessionid.ForEngineContext(getSessionRaw())
}

// GetSessionRaw returns the configured session string before engine normalization (for add → global vs named).
func GetSessionRaw() string {
	return getSessionRaw()
}
