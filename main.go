package main

import (
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AI Memory Integration - Go-native AI Memory library inspired by Cognee's architecture
// This is the main entry point for the AI Memory library
func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Generate a unique ID for this session
	sessionID := uuid.New()

	fmt.Println("AI Memory Integration - Go Library")
	fmt.Println("Version: 0.1.0")
	fmt.Println("Module: github.com/NortonBen/ai-memory-go")
	fmt.Printf("Session ID: %s\n", sessionID.String())

	logger.Info("AI Memory library initialized successfully",
		zap.String("session_id", sessionID.String()),
		zap.String("version", "0.1.0"),
	)
}
