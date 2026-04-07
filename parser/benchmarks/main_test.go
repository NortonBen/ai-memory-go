package benchmarks

import (
	"fmt"
	"os"
	"testing"
)

// Benchmarks in this package are opt-in to keep default CI/test runs stable and fast.
func TestMain(m *testing.M) {
	if os.Getenv("RUN_PARSER_BENCHMARK_TESTS") == "1" {
		os.Exit(m.Run())
	}

	fmt.Println("Skipping parser/benchmarks tests by default; set RUN_PARSER_BENCHMARK_TESTS=1 to enable")
	os.Exit(0)
}
