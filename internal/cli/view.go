package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/NortonBen/ai-memory-go/internal/view"
	"github.com/spf13/cobra"
)

var (
	viewHost string
	viewPort int
)

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Start API + React UI to preview memory database",
	Long:  `Start a local web server exposing API endpoints and a React UI for datapoints, vectors, relationships, and search.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		runtime, err := initRuntime(ctx)
		if err != nil {
			fmt.Printf("Failed to initialize runtime: %v\n", err)
			return
		}
		defer runtime.Engine.Close()

		server, err := view.NewServer(view.Dependencies{
			Engine:    runtime.Engine,
			Graph:     runtime.GraphStore,
			RelStore:  runtime.RelStore,
			VecStore:  runtime.VecStore,
			AppName:   "AI Memory Viewer",
			AppPrefix: "/api",
		})
		if err != nil {
			fmt.Printf("Failed to initialize viewer server: %v\n", err)
			return
		}

		addr := fmt.Sprintf("%s:%d", viewHost, viewPort)
		fmt.Printf("Viewer running at http://%s\n", addr)
		fmt.Println("API base: /api")
		if err := http.ListenAndServe(addr, server.Handler()); err != nil {
			fmt.Printf("Viewer stopped: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(viewCmd)
	viewCmd.Flags().StringVar(&viewHost, "host", "127.0.0.1", "host to bind viewer server")
	viewCmd.Flags().IntVar(&viewPort, "port", 8088, "port to bind viewer server")
}
