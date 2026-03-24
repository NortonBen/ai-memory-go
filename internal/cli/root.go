package cli

import (
	"os"
	"path/filepath"

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
	rootCmd.PersistentFlags().StringVarP(&session, "session", "s", "default", "active memory session")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "display detailed operation information")
	rootCmd.PersistentFlags().StringVarP(&output, "format", "f", "text", "output format (text, json, yaml)")
}

func GetSessionID() string {
	if session != "default" && session != "" {
		return session
	}
	home, err := os.UserHomeDir()
	if err == nil {
		sessionFile := filepath.Join(home, ".ai-memory", "session.txt")
		if data, err := os.ReadFile(sessionFile); err == nil {
			return string(data)
		}
	}
	return "default"
}
