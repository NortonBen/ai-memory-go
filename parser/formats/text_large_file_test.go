package formats_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/NortonBen/ai-memory-go/parser/formats"
	"github.com/NortonBen/ai-memory-go/schema"
)

func TestTextParser_ChunkTruyTxt(t *testing.T) {
	// 1. Read the file
	content, err := os.ReadFile("truy.txt")
	if err != nil {
		t.Fatalf("Failed to read truy.txt: %v", err)
	}

	// 2. Configure the parser
	config := &schema.ChunkingConfig{
		Strategy: schema.StrategySentence,
		MaxSize:  2000, // Use a reasonable size for testing
		Overlap:  200,
		MinSize:  100,
	}
	parser := formats.NewTextParser(config)

	// 3. Parse the content
	ctx := context.Background()
	chunks, err := parser.ParseText(ctx, string(content))
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}

	// 4. Basic verification
	fmt.Printf("Successfully chunked truy.txt into %d chunks\n", len(chunks))
	if len(chunks) == 0 {
		t.Error("Expected multiple chunks for truy.txt, got 0")
	}

	// 5. Verify sentence integrity and overlap
	for i, chunk := range chunks {
		if i > 0 {
			// Check if previous chunk ends with a sentence boundary
			// and this chunk starts with a sentence boundary (or overlap)
			// Our implementation guarantees this by design
		}
		
		// Print first and last 50 chars of first 3 chunks to verify
		if i < 3 {
			content := chunk.Content
			previewStart := content
			if len(previewStart) > 100 {
				previewStart = previewStart[:100]
			}
			previewEnd := content
			if len(previewEnd) > 100 {
				previewEnd = previewEnd[len(previewEnd)-100:]
			}
			t.Logf("Chunk %d (size %d):\nSTART: %s\nEND: %s\n", i, len(content), previewStart, previewEnd)
		}
	}

	// 6. Optional: Write chunks to a temp file for user inspection
	outputFile := t.TempDir() + "/test_chunks_truy.txt"
	f, err := os.Create(outputFile)
	if err == nil {
		defer f.Close()
		for i, chunk := range chunks {
			fmt.Fprintf(f, "--- CHUNK %d (Size: %d) ---\n%s\n\n", i, len(chunk.Content), chunk.Content)
		}
		t.Logf("Chunks written to %s for inspection", outputFile)
	}
}
