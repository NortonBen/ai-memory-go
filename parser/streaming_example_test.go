package parser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExampleStreamingParser demonstrates basic usage of the streaming parser
func ExampleStreamingParser() {
	// Create a streaming parser with default configuration
	parser := NewStreamingParser(nil, nil)

	// Example content
	content := `This is the first paragraph of a large document.
It contains multiple sentences and demonstrates streaming parsing.

This is the second paragraph. The streaming parser processes
content in chunks to maintain constant memory usage.

This is the third paragraph. It shows how the parser
handles different content types efficiently.`

	// Parse content from a string reader
	reader := strings.NewReader(content)
	result, err := parser.ParseReaderStream(context.Background(), reader, "example_document")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display results
	fmt.Printf("Processed %d chunks in %v\n", result.ChunksCreated, result.ProcessingTime)
	for i, chunk := range result.Chunks {
		fmt.Printf("Chunk %d: %s...\n", i+1, chunk.Content[:min(50, len(chunk.Content))])
	}

	// Output:
	// Processed 3 chunks in [duration]
	// Chunk 1: This is the first paragraph of a large document...
	// Chunk 2: This is the second paragraph. The streaming parse...
	// Chunk 3: This is the third paragraph. It shows how the par...
}

// ExampleStreamingParser_withProgress demonstrates progress tracking
func ExampleStreamingParser_withProgress() {
	// Configure streaming parser with progress tracking
	config := &StreamingConfig{
		BufferSize:             32 * 1024, // 32KB buffer
		MaxChunkSize:           2 * 1024,  // 2KB max chunk
		MinChunkSize:           100,       // 100B min chunk
		EnableProgressTracking: true,
		FlushInterval:          50 * time.Millisecond,
		ProgressCallback: func(bytesProcessed, totalBytes int64, chunksCreated int) {
			if totalBytes > 0 {
				progress := float64(bytesProcessed) / float64(totalBytes) * 100
				fmt.Printf("Progress: %.1f%% (%d chunks)\n", progress, chunksCreated)
			} else {
				fmt.Printf("Processed: %d bytes (%d chunks)\n", bytesProcessed, chunksCreated)
			}
		},
	}

	parser := NewStreamingParser(config, nil)

	// Create large content
	var contentBuilder strings.Builder
	for i := 0; i < 100; i++ {
		contentBuilder.WriteString(fmt.Sprintf("This is paragraph %d with substantial content. ", i+1))
		contentBuilder.WriteString("It contains multiple sentences to demonstrate progress tracking. ")
		contentBuilder.WriteString("The streaming parser will show progress as it processes this content.\n\n")
	}

	reader := strings.NewReader(contentBuilder.String())
	result, err := parser.ParseReaderStream(context.Background(), reader, "progress_example")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Final result: %d chunks processed\n", result.ChunksCreated)
}

// ExampleStreamingParser_largeFile demonstrates parsing a large file
func ExampleStreamingParser_largeFile() {
	// Create a temporary large file
	tmpDir := os.TempDir()
	largeFile := filepath.Join(tmpDir, "large_example.txt")

	// Generate large content
	file, err := os.Create(largeFile)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}

	// Write 1000 paragraphs
	for i := 0; i < 1000; i++ {
		paragraph := fmt.Sprintf("This is paragraph %d of a very large document. "+
			"It demonstrates how the streaming parser handles large files efficiently. "+
			"The parser maintains constant memory usage regardless of file size. "+
			"This is important for processing files that exceed available RAM.\n\n", i+1)
		file.WriteString(paragraph)
	}
	file.Close()

	// Clean up file when done
	defer os.Remove(largeFile)

	// Parse the large file using streaming
	parser := NewStreamingParser(nil, nil)
	result, err := parser.ParseFileStream(context.Background(), largeFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Display memory efficiency
	memUsage := parser.GetMemoryUsage()
	fmt.Printf("File size: %d bytes\n", result.TotalBytes)
	fmt.Printf("Memory usage: %d bytes (%.2f%% of file size)\n",
		memUsage, float64(memUsage)/float64(result.TotalBytes)*100)
	fmt.Printf("Chunks created: %d\n", result.ChunksCreated)
	fmt.Printf("Processing time: %v\n", result.ProcessingTime)
}

// ExampleUnifiedParser_autoStreaming demonstrates automatic streaming selection
func ExampleUnifiedParser_autoStreaming() {
	parser := NewUnifiedParser(nil)
	defer parser.Close()

	tmpDir := os.TempDir()

	// Create small file
	smallFile := filepath.Join(tmpDir, "small_example.txt")
	smallContent := "This is a small file that will use regular parsing."
	os.WriteFile(smallFile, []byte(smallContent), 0644)
	defer os.Remove(smallFile)

	// Create large file (>10MB to trigger streaming)
	largeFile := filepath.Join(tmpDir, "large_example.txt")
	largeContent := strings.Repeat("This is repeated content to create a large file. ", 250000)
	os.WriteFile(largeFile, []byte(largeContent), 0644)
	defer os.Remove(largeFile)

	// Parse small file (will use regular parsing)
	shouldStream, _ := parser.ShouldUseStreaming(smallFile)
	fmt.Printf("Small file should use streaming: %v\n", shouldStream)

	chunks1, err := parser.ParseFileAuto(context.Background(), smallFile)
	if err != nil {
		fmt.Printf("Error parsing small file: %v\n", err)
		return
	}
	fmt.Printf("Small file: %d chunks\n", len(chunks1))

	// Parse large file (will use streaming)
	shouldStream, _ = parser.ShouldUseStreaming(largeFile)
	fmt.Printf("Large file should use streaming: %v\n", shouldStream)

	chunks2, err := parser.ParseFileAuto(context.Background(), largeFile)
	if err != nil {
		fmt.Printf("Error parsing large file: %v\n", err)
		return
	}
	fmt.Printf("Large file: %d chunks\n", len(chunks2))

	// Output:
	// Small file should use streaming: false
	// Small file: 1 chunks
	// Large file should use streaming: true
	// Large file: [many] chunks
}

// ExampleStreamingParser_customConfig demonstrates custom configuration
func ExampleStreamingParser_customConfig() {
	// Custom streaming configuration
	streamConfig := &StreamingConfig{
		BufferSize:   16 * 1024, // 16KB buffer
		ChunkOverlap: 512,       // 512B overlap
		MaxChunkSize: 2 * 1024,  // 2KB max chunk
		MinChunkSize: 128,       // 128B min chunk
	}

	// Custom chunking configuration
	chunkConfig := &ChunkingConfig{
		Strategy: StrategySentence, // Chunk by sentences
		MaxSize:  500,              // 500 chars max
		MinSize:  50,               // 50 chars min
		Overlap:  25,               // 25 chars overlap
	}

	parser := NewStreamingParser(streamConfig, chunkConfig)

	content := `First sentence of the document. Second sentence follows immediately.
Third sentence starts a new thought. Fourth sentence concludes the paragraph.

New paragraph begins here. Another sentence in the second paragraph.
Final sentence of the document ends here.`

	reader := strings.NewReader(content)
	result, err := parser.ParseReaderStream(context.Background(), reader, "custom_config")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Custom configuration results:\n")
	fmt.Printf("Chunks created: %d\n", result.ChunksCreated)
	fmt.Printf("Memory usage: %d bytes\n", parser.GetMemoryUsage())

	for i, chunk := range result.Chunks {
		fmt.Printf("Chunk %d (%d chars): %s\n", i+1, len(chunk.Content), chunk.Content)
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
