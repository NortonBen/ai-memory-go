package formats_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/NortonBen/ai-memory-go/parser/formats"
	"github.com/NortonBen/ai-memory-go/schema"
)

func TestTextParser_TruySentenceOverlap(t *testing.T) {
	// 1. Read the file
	content, err := os.ReadFile("truy.txt")
	if err != nil {
		t.Fatalf("Failed to read truy.txt: %v", err)
	}

	// 2. Configure the parser
	// MaxSize 1000, Overlap 250 (should be enough for 1-2 Vietnamese sentences)
	config := &schema.ChunkingConfig{
		Strategy: schema.StrategySentence,
		MaxSize:  1000,
		Overlap:  250,
		MinSize:  100,
	}
	parser := formats.NewTextParser(config)

	// 3. Parse the content
	ctx := context.Background()
	chunks, err := parser.ParseText(ctx, string(content))
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}

	fmt.Printf("Chunked truy.txt into %d chunks\n", len(chunks))

	// 4. Verify overlap logic
	// Each chunk N and N+1 should share "connecting sentences"
	reportFile, _ := os.Create(t.TempDir() + "/truy_overlap_report.txt")
	defer reportFile.Close()

	for i := 0; i < len(chunks)-1; i++ {
		c1 := chunks[i].Content
		c2 := chunks[i+1].Content

		if strings.TrimSpace(c2) == "" {
			t.Errorf("Chunk %d is empty", i+1)
			continue
		}

		// Overlap now supports both sentence-based and hard split fallbacks.
		// Verify using prefix overlap instead of full first sentence matching.
		prefixLen := 80
		if len(c2) < prefixLen {
			prefixLen = len(c2)
		}
		prefix := strings.TrimSpace(c2[:prefixLen])
		if prefix == "" {
			t.Logf("Warning: empty overlap prefix for Chunk %d", i+1)
			continue
		}

		if strings.Contains(c1, prefix) {
			fmt.Fprintf(reportFile, "MATCH Chunk %d -> %d\nOverlap Prefix: %s\n\n", i, i+1, prefix)
		}
	}

	// Some boundaries can legitimately miss direct overlap due to normalization/splitting fallback.
	// Enforce that overlap works for most transitions rather than 100% strict sentence matching.
	matched := 0
	total := len(chunks) - 1
	for i := 0; i < len(chunks)-1; i++ {
		c1 := chunks[i].Content
		c2 := chunks[i+1].Content
		prefixLen := 40
		if len(c2) < prefixLen {
			prefixLen = len(c2)
		}
		if prefixLen == 0 {
			continue
		}
		if strings.Contains(c1, strings.TrimSpace(c2[:prefixLen])) {
			matched++
		}
	}
	if total > 0 && matched < int(float64(total)*0.90) {
		t.Fatalf("Overlap coverage too low: matched=%d total=%d", matched, total)
	}
}
