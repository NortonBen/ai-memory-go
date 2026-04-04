package formats_test

import (
	"context"
	"strings"
	"testing"

	"github.com/NortonBen/ai-memory-go/parser/formats"
	"github.com/NortonBen/ai-memory-go/schema"
)

func TestTextParser_ParagraphSplitsOversizedParagraph(t *testing.T) {
	config := &schema.ChunkingConfig{
		Strategy: schema.StrategyParagraph,
		MaxSize:  120,
		Overlap:  20,
		MinSize:  10,
	}
	parser := formats.NewTextParser(config)
	content := strings.Repeat("Doan van ban rat dai ", 30)

	chunks, err := parser.ParseText(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("Expected oversized paragraph to be split, got %d chunk(s)", len(chunks))
	}

	for i, chunk := range chunks {
		if got := len([]rune(chunk.Content)); got > config.MaxSize {
			t.Fatalf("Chunk %d exceeds MaxSize: got=%d max=%d", i, got, config.MaxSize)
		}
	}
}

func TestTextParser_SentenceSplitsOversizedSentence(t *testing.T) {
	config := &schema.ChunkingConfig{
		Strategy: schema.StrategySentence,
		MaxSize:  100,
		Overlap:  15,
		MinSize:  1,
	}
	parser := formats.NewTextParser(config)
	content := strings.Repeat("x", 260) + "."

	chunks, err := parser.ParseText(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("Expected oversized sentence to be split, got %d chunk(s)", len(chunks))
	}

	for i, chunk := range chunks {
		if got := len([]rune(chunk.Content)); got > config.MaxSize {
			t.Fatalf("Chunk %d exceeds MaxSize: got=%d max=%d", i, got, config.MaxSize)
		}
	}
}

func TestTextParser_FixedSizeOverlapGreaterThanMaxSize(t *testing.T) {
	config := &schema.ChunkingConfig{
		Strategy: schema.StrategyFixedSize,
		MaxSize:  50,
		Overlap:  100, // Invalid window: should be normalized internally.
		MinSize:  1,
	}
	parser := formats.NewTextParser(config)
	content := strings.Repeat("a", 300)

	chunks, err := parser.ParseText(context.Background(), content)
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("Expected chunks for non-empty content")
	}

	for i, chunk := range chunks {
		if got := len([]rune(chunk.Content)); got > config.MaxSize {
			t.Fatalf("Chunk %d exceeds MaxSize: got=%d max=%d", i, got, config.MaxSize)
		}
	}
}
