# Parser Performance Testing Guide

This document describes the comprehensive performance testing suite for the AI Memory Integration parser package, implementing Task 3.3.4: Create benchmarks and performance tests.

## Overview

The performance testing suite validates that the parser meets the performance requirements specified in the design document:

- **Memory_Engine SHALL respond to search queries within 200ms for datasets up to 100,000 entities**
- **Cognify_Pipeline SHALL process documents with configurable batch sizes and parallel processing**
- **Memory_Engine SHALL provide metrics and monitoring capabilities for production deployment**

## Test Files Structure

```
parser/
├── comprehensive_performance_benchmark_test.go  # Comprehensive benchmarks
├── performance_baseline_test.go                 # Performance requirement validation
├── performance_profiling_test.go               # Profiling integration tests
├── performance_test_runner.go                  # Test runner and reporting
├── performance_comparison_test.go              # Existing comparison tests
├── performance_monitoring_test.go              # Existing monitoring tests
├── performance_regression_test.go              # Existing regression tests
├── cache_benchmark_test.go                     # Existing cache benchmarks
├── streaming_benchmark_test.go                 # Existing streaming benchmarks
└── PERFORMANCE_TESTING.md                     # This documentation
```

## Running Performance Tests

### Quick Performance Check
```bash
# Run consolidated performance test suite
go test -v ./parser -run "TestRunPerformanceTestSuite"

# Run optimized performance baselines
go test -v ./parser -run "TestOptimizedPerformanceBaselines"

# Run memory engine response time test (optimized)
go test -v ./parser -run "TestMemoryEngineResponseTimeOptimized"

# Run comprehensive performance test suite programmatically
go run parser/run_performance_tests.go -output=results -verbose
```

### Comprehensive Benchmarks
```bash
# Run all benchmarks
go test -bench=. -benchmem ./parser

# Run specific benchmark categories
go test -bench="BenchmarkUnifiedParser" -benchmem ./parser
go test -bench="BenchmarkChunkingStrategies" -benchmem ./parser
go test -bench="BenchmarkWorkerPool" -benchmem ./parser
go test -bench="BenchmarkMemoryEfficiency" -benchmem ./parser

# Run benchmarks with CPU profiling
go test -bench=. -cpuprofile=cpu.prof ./parser

# Run benchmarks with memory profiling
go test -bench=. -memprofile=mem.prof ./parser
```

### Performance Profiling
```bash
# Run CPU profiling tests
go test -v ./parser -run "TestCPUProfiling"

# Run memory profiling tests
go test -v ./parser -run "TestMemoryProfiling"

# Run goroutine profiling tests
go test -v ./parser -run "TestGoroutineProfiling"

# Run blocking profiling tests
go test -v ./parser -run "TestBlockingProfiling"

# Run mutex profiling tests
go test -v ./parser -run "TestMutexProfiling"
```

### Performance Test Suite
```bash
# Run complete performance test suite with reporting
go run parser/run_performance_tests.go

# Run with specific options
go run parser/run_performance_tests.go -output=custom_results -verbose -format=html

# Run benchmarks only
go run parser/run_performance_tests.go -bench-only

# Run tests only (no benchmarks)
go run parser/run_performance_tests.go -tests-only

# Quick mode (faster execution)
go run parser/run_performance_tests.go -quick

# Or use the test runner programmatically
go test -v ./parser -run "TestRunPerformanceTestSuite"
```

## Performance Baselines

The performance testing suite validates against these baselines:

### File Size Baselines
| File Size | Max Latency | Min Throughput | Max Memory | Max Allocs/Op |
|-----------|-------------|----------------|------------|---------------|
| 1KB       | 50ms        | 20 MB/s        | 2 MB       | 500           |
| 10KB      | 100ms       | 100 MB/s       | 5 MB       | 2,000         |
| 100KB     | 500ms       | 200 MB/s       | 20 MB      | 10,000        |
| 1MB       | 2s          | 500 MB/s       | 50 MB      | 50,000        |

### Batch Processing Baselines
| Scenario              | Max Latency | Min Throughput | Max Memory | Max Allocs/Op |
|-----------------------|-------------|----------------|------------|---------------|
| 100 Files Batch      | 5s          | 50 MB/s        | 100 MB     | 100,000       |
| 8 Concurrent Goroutines | 200ms     | 100 MB/s       | 30 MB      | 5,000         |

## Benchmark Categories

### 1. Unified Parser Benchmarks
- **BenchmarkUnifiedParserAllFormats**: Tests parsing performance across all supported formats (TXT, MD, CSV, JSON, PDF)
- **BenchmarkParserScalability**: Tests performance with increasing data sizes (1KB to 10MB)
- **BenchmarkParserThroughputMeasurement**: Measures parsing throughput in MB/s

### 2. Chunking Strategy Benchmarks
- **BenchmarkChunkingStrategiesPerformance**: Compares paragraph, sentence, fixed-size, and semantic chunking
- **BenchmarkContentTypeDetection**: Tests content type detection performance
- **BenchmarkChunkValidation**: Tests chunk validation performance

### 3. Worker Pool Benchmarks
- **BenchmarkWorkerPoolScalabilityComprehensive**: Tests worker pool performance with different configurations
- **BenchmarkConcurrentParsingLoad**: Tests concurrent parsing performance

### 4. Memory Efficiency Benchmarks
- **BenchmarkMemoryEfficiencyComprehensive**: Tests memory usage patterns across different parser types
- **BenchmarkStreamingVsRegularParsing**: Compares streaming vs regular parsing memory efficiency

### 5. Format Detection Benchmarks
- **BenchmarkFormatDetectionPerformance**: Tests file format detection speed

## Performance Profiling

### CPU Profiling
The CPU profiling tests generate CPU profiles for:
- Sequential parsing operations
- Parallel parsing with worker pools
- Cached parsing operations

Profiles are saved to `testdata/performance_monitoring/profiles/cpu_*.prof`

### Memory Profiling
Memory profiling tests analyze:
- Memory allocation patterns
- Memory usage with different file sizes
- Memory efficiency of different parser configurations

Profiles are saved to `testdata/performance_monitoring/profiles/mem_*.prof`

### Goroutine Profiling
Goroutine profiling tests examine:
- Goroutine creation and management
- Worker pool goroutine usage
- Concurrent parsing goroutine patterns

### Blocking and Mutex Profiling
These tests identify:
- Synchronization bottlenecks
- Lock contention issues
- Channel blocking patterns

## Performance Monitoring

### Automated Monitoring
The performance monitoring tests provide:
- Continuous performance tracking
- Performance degradation detection
- Automated baseline validation

### Metrics Collection
Key metrics collected include:
- **Latency**: Response time for parsing operations
- **Throughput**: Data processing rate in MB/s
- **Memory Usage**: Peak and average memory consumption
- **Allocations**: Memory allocations per operation
- **Concurrency**: Goroutine and worker pool efficiency

### Performance Reports
The test runner generates comprehensive reports:
- **JSON Report**: Machine-readable performance data
- **HTML Report**: Human-readable performance dashboard
- **CSV Summary**: Spreadsheet-compatible results

## Performance Requirements Validation

### Memory Engine Response Time
Tests validate the requirement that the Memory_Engine SHALL respond to search queries within 200ms for datasets up to 100,000 entities.

### Cognify Pipeline Performance
Tests validate configurable batch sizes and parallel processing capabilities with different worker pool configurations.

### Production Readiness
Tests ensure the parser can handle production workloads with:
- High throughput processing
- Efficient memory usage
- Concurrent operation safety
- Scalable performance characteristics

## Interpreting Results

### Successful Performance Test
```
✓ SmallFile_1KB baseline passed:
  Latency: 25ms (limit: 50ms)
  Throughput: 45.2 MB/s (min: 20.0 MB/s)
  Memory: 1.2 MB (max: 2.0 MB)
  Allocs: 234/op (max: 500)
```

### Performance Regression
```
✗ MediumFile_10KB baseline failed:
  Latency: 150ms > 100ms (max allowed)
  Throughput: 85.3 MB/s (min: 100.0 MB/s)
```

### Benchmark Results
```
BenchmarkUnifiedParserAllFormats/Format_txt-8    1000    1234567 ns/op    45.2 MB/s    1024 B/op    15 allocs/op
```

## Optimization Recommendations

Based on performance test results, the system provides automated recommendations:

### High Priority
- **Throughput Issues**: Enable worker pool parallelization
- **Memory Issues**: Use streaming parser for large files
- **Reliability Issues**: Review failed test details

### Medium Priority
- **Concurrency Issues**: Increase worker pool size
- **Cache Issues**: Optimize cache configuration
- **Allocation Issues**: Implement memory pooling

### Low Priority
- **Minor Optimizations**: Fine-tune chunking parameters
- **Monitoring**: Add additional performance metrics

## Continuous Integration

### Performance Gates
The performance tests can be integrated into CI/CD pipelines to:
- Block deployments that fail performance baselines
- Track performance trends over time
- Alert on performance regressions

### Automated Reporting
Performance reports can be automatically generated and published to:
- Build artifacts
- Performance dashboards
- Monitoring systems

## Troubleshooting

### Common Issues

1. **High Memory Usage**
   - Use streaming parser for large files
   - Reduce chunk sizes
   - Enable garbage collection tuning

2. **Low Throughput**
   - Enable worker pool parallelization
   - Optimize chunking strategy
   - Check for I/O bottlenecks

3. **High Latency**
   - Use caching for repeated operations
   - Optimize content type detection
   - Reduce allocation overhead

### Performance Analysis Tools

1. **Go pprof**: Analyze CPU and memory profiles
   ```bash
   go tool pprof cpu.prof
   go tool pprof mem.prof
   ```

2. **Benchstat**: Compare benchmark results
   ```bash
   go install golang.org/x/perf/cmd/benchstat@latest
   benchstat old.txt new.txt
   ```

3. **Trace Analysis**: Analyze execution traces
   ```bash
   go test -trace=trace.out ./parser
   go tool trace trace.out
   ```

## Best Practices

1. **Run tests in consistent environment**
2. **Use multiple iterations for stable results**
3. **Monitor system resources during tests**
4. **Compare results against baselines**
5. **Profile before and after optimizations**
6. **Document performance characteristics**
7. **Automate performance regression detection**

## Contributing

When adding new performance tests:

1. Follow the existing naming conventions
2. Include appropriate baselines and thresholds
3. Add documentation for new test categories
4. Update this guide with new test descriptions
5. Ensure tests are deterministic and reproducible