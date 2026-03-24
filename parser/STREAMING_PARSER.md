# Streaming Parser Implementation

## Overview

The Streaming Parser is a memory-efficient implementation for processing large files without loading the entire content into memory. It's designed to handle files that exceed available RAM (GB+ sizes) while maintaining consistent performance and memory usage.

## Key Features

- **Memory Efficient**: Constant memory usage regardless of file size
- **Configurable Buffer Management**: Adjustable buffer sizes and streaming parameters
- **Multiple Format Support**: Text, markdown, PDF with streaming capabilities
- **Progress Tracking**: Real-time progress updates for long-running operations
- **Error Handling**: Robust error recovery and context cancellation support
- **Integration**: Seamless integration with existing parser architecture

## Architecture

### Core Components

1. **StreamingParser**: Main parser that handles streaming operations
2. **StreamingConfig**: Configuration for buffer sizes and streaming behavior
3. **StreamingResult**: Result structure with processing metrics
4. **Buffer Management**: Intelligent buffering with overlap handling

### Memory Usage

The streaming parser maintains constant memory usage through:
- Fixed-size read buffers (default: 64KB)
- Configurable chunk sizes (default: 4KB max)
- Overlap management for content continuity
- Immediate processing and disposal of processed content

## Configuration

### StreamingConfig

```go
type StreamingConfig struct {
    BufferSize             int           // Read buffer size (default: 64KB)
    ChunkOverlap           int           // Overlap between chunks (default: 1KB)
    MaxChunkSize           int           // Maximum chunk size (default: 4KB)
    MinChunkSize           int           // Minimum chunk size (default: 256B)
    EnableProgressTracking bool          // Enable progress callbacks
    ProgressCallback       func(...)     // Progress callback function
    FlushInterval          time.Duration // Progress update frequency
}
```

### Default Configuration

```go
config := DefaultStreamingConfig()
// BufferSize: 64KB
// ChunkOverlap: 1KB
// MaxChunkSize: 4KB
// MinChunkSize: 256B
// EnableProgressTracking: true
// FlushInterval: 100ms
```

## Usage Examples

### Basic Usage

```go
// Create streaming parser
parser := NewStreamingParser(nil, nil)

// Parse from file
result, err := parser.ParseFileStream(ctx, "large_file.txt")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Processed %d chunks in %v\n", result.ChunksCreated, result.ProcessingTime)
```

### With Progress Tracking

```go
config := DefaultStreamingConfig()
config.ProgressCallback = func(bytesProcessed, totalBytes int64, chunksCreated int) {
    if totalBytes > 0 {
        progress := float64(bytesProcessed) / float64(totalBytes) * 100
        fmt.Printf("Progress: %.1f%% (%d chunks)\n", progress, chunksCreated)
    }
}

parser := NewStreamingParser(config, nil)
result, err := parser.ParseFileStream(ctx, "large_file.txt")
```

### Custom Configuration

```go
streamConfig := &StreamingConfig{
    BufferSize:   32 * 1024, // 32KB buffer
    MaxChunkSize: 2 * 1024,  // 2KB chunks
    MinChunkSize: 128,       // 128B minimum
}

chunkConfig := &ChunkingConfig{
    Strategy: StrategySentence,
    MaxSize:  500,
    MinSize:  50,
}

parser := NewStreamingParser(streamConfig, chunkConfig)
```

### Unified Parser Integration

```go
parser := NewUnifiedParser(nil)

// Automatic streaming selection based on file size
chunks, err := parser.ParseFileAuto(ctx, "file.txt")

// Manual streaming
result, err := parser.ParseFileStream(ctx, "large_file.txt")

// Check if streaming should be used
shouldStream, err := parser.ShouldUseStreaming("file.txt")
```

## Performance Characteristics

### Memory Usage

- **Constant Memory**: ~70KB for default configuration (64KB buffer + 4KB chunk + 1KB overlap)
- **Scalable**: Memory usage independent of file size
- **Configurable**: Adjustable based on available memory and performance requirements

### Processing Speed

- **Throughput**: ~400MB/s on modern hardware
- **Latency**: Low latency for first chunks (streaming processing)
- **Scalability**: Linear scaling with file size

### Benchmark Results

```
BenchmarkStreamingParserThroughput-12    17937    233118 ns/op    404.59 MB/s
BenchmarkStreamingParserMemoryEfficiency/XLarge_1MB-12    2228    1614804 ns/op
BenchmarkStreamingVsRegularParsing/Streaming-12    87110    41573 ns/op
BenchmarkStreamingVsRegularParsing/Regular-12      71011    50339 ns/op
```

## Chunking Strategies

The streaming parser supports all standard chunking strategies:

### Paragraph Strategy (Default)
- Splits content by double newlines (`\n\n`)
- Preserves paragraph structure
- Good for document processing

### Sentence Strategy
- Splits content by sentence boundaries (`.`, `!`, `?`)
- Maintains sentence integrity
- Useful for NLP applications

### Fixed Size Strategy
- Splits content by fixed character count
- Configurable overlap between chunks
- Predictable chunk sizes

## Error Handling

### Context Cancellation
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := parser.ParseFileStream(ctx, "file.txt")
if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        // Handle timeout
    }
}
```

### File Errors
```go
result, err := parser.ParseFileStream(ctx, "nonexistent.txt")
if err != nil {
    if os.IsNotExist(err) {
        // Handle file not found
    }
}
```

### Recovery
- Partial results available on cancellation
- Graceful degradation on errors
- Retry logic for transient failures

## Integration with Existing Code

### Drop-in Replacement
```go
// Before: Regular parsing
chunks, err := parser.ParseFile(ctx, "file.txt")

// After: Automatic streaming
chunks, err := parser.ParseFileAuto(ctx, "file.txt")
```

### Gradual Migration
```go
// Check file size and decide
if fileSize > 10*1024*1024 { // 10MB threshold
    result, err := parser.ParseFileStream(ctx, filePath)
    chunks = result.Chunks
} else {
    chunks, err = parser.ParseFile(ctx, filePath)
}
```

## Best Practices

### Configuration Tuning

1. **Buffer Size**: 
   - Larger buffers (128KB+) for high-throughput scenarios
   - Smaller buffers (16KB-32KB) for memory-constrained environments

2. **Chunk Size**:
   - Larger chunks (8KB+) for better compression and fewer chunks
   - Smaller chunks (1KB-2KB) for more granular processing

3. **Overlap**:
   - Increase overlap for better content continuity
   - Reduce overlap to minimize memory usage

### Performance Optimization

1. **Use appropriate chunking strategy** for your content type
2. **Enable progress tracking** only when needed
3. **Tune buffer sizes** based on available memory
4. **Use context cancellation** for long-running operations

### Memory Management

1. **Monitor memory usage** with `GetMemoryUsage()`
2. **Adjust configuration** based on available resources
3. **Use streaming for files > 10MB** (automatic threshold)
4. **Process results incrementally** to avoid accumulating chunks

## Limitations

1. **Content Continuity**: Some overlap may be lost at buffer boundaries
2. **Random Access**: No support for seeking or random access
3. **Format Limitations**: Some formats may not stream well (e.g., compressed files)
4. **Memory Overhead**: Small overhead for buffer management

## Future Enhancements

1. **Compressed File Support**: Direct streaming of compressed formats
2. **Parallel Processing**: Multi-threaded streaming for very large files
3. **Adaptive Buffering**: Dynamic buffer size adjustment
4. **Format-Specific Optimizations**: Specialized streaming for different file types

## Testing

The streaming parser includes comprehensive tests:

- Unit tests for all components
- Integration tests with unified parser
- Performance benchmarks
- Memory efficiency tests
- Error handling tests
- Progress tracking tests

Run tests with:
```bash
go test ./parser -run TestStreaming -v
go test ./parser -bench BenchmarkStreaming -benchmem
```

## Conclusion

The Streaming Parser provides a robust, memory-efficient solution for processing large files. It maintains the same API as the regular parser while offering significant memory savings and consistent performance regardless of file size. The configurable nature allows optimization for different use cases and environments.