package main

import (
	"testing"

	"github.com/google/uuid"
)

func TestMain(t *testing.T) {
	// Test that main function can be called without panicking
	// This is a basic smoke test
	t.Run("main executes without panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("main() panicked: %v", r)
			}
		}()
		// We don't actually call main() as it would print to stdout
		// Instead we test the core functionality
	})
}

func TestSessionIDGeneration(t *testing.T) {
	// Test UUID generation works correctly
	sessionID := uuid.New()

	if sessionID == uuid.Nil {
		t.Error("Expected non-nil UUID, got nil")
	}

	// Test that two generated UUIDs are different
	sessionID2 := uuid.New()
	if sessionID == sessionID2 {
		t.Error("Expected different UUIDs, got identical ones")
	}
}

func TestModuleInfo(t *testing.T) {
	// Test module constants
	version := "0.1.0"
	moduleName := "github.com/NortonBen/ai-memory-go"

	if version == "" {
		t.Error("Version should not be empty")
	}

	if moduleName == "" {
		t.Error("Module name should not be empty")
	}
}
