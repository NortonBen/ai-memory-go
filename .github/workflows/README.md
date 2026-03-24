# GitHub Actions CI/CD Pipeline

This directory contains the GitHub Actions workflows for the AI Memory Integration library.

## Workflows

### CI Workflow (`ci.yml`)

The main continuous integration workflow runs on every push and pull request to `main` and `develop` branches.

#### Jobs

1. **Test Job**
   - Runs on: Ubuntu Latest
   - Go Versions: 1.22, 1.23
   - Steps:
     - Checkout code
     - Set up Go environment
     - Cache Go modules for faster builds
     - Download and verify dependencies
     - Run tests with race detection
     - Generate coverage report
     - Upload coverage to Codecov

2. **Lint Job**
   - Runs on: Ubuntu Latest
   - Go Version: 1.23
   - Steps:
     - Checkout code
     - Set up Go environment
     - Run golangci-lint with comprehensive linters

3. **Build Job**
   - Runs on: Ubuntu Latest
   - Go Versions: 1.22, 1.23
   - Steps:
     - Checkout code
     - Set up Go environment
     - Build all packages
     - Build main binary

4. **Security Job**
   - Runs on: Ubuntu Latest
   - Steps:
     - Checkout code
     - Run Gosec security scanner
     - Upload SARIF results to GitHub Security

## Configuration Files

### `.golangci.yml`

Linting configuration with the following enabled linters:
- errcheck: Check for unchecked errors
- gosimple: Simplify code suggestions
- govet: Go vet analysis
- ineffassign: Detect ineffectual assignments
- staticcheck: Static analysis
- unused: Check for unused code
- gofmt: Format checking
- goimports: Import organization
- misspell: Spell checking
- revive: Fast, configurable linter
- gosec: Security-focused linter
- gocritic: Comprehensive diagnostics
- unconvert: Remove unnecessary conversions
- unparam: Detect unused parameters
- prealloc: Find slice declarations that could be preallocated
- exportloopref: Check for loop variable capture issues

## Required Secrets

To enable all features, configure these secrets in your GitHub repository:

- `CODECOV_TOKEN`: Token for uploading coverage reports to Codecov (optional)

## Badge URLs

Add these badges to your README.md:

```markdown
[![CI](https://github.com/NortonBen/ai-memory-go/actions/workflows/ci.yml/badge.svg)](https://github.com/NortonBen/ai-memory-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/NortonBen/ai-memory-go)](https://goreportcard.com/report/github.com/NortonBen/ai-memory-go)
[![codecov](https://codecov.io/gh/NortonBen/ai-memory-go/branch/main/graph/badge.svg)](https://codecov.io/gh/NortonBen/ai-memory-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/NortonBen/ai-memory-go.svg)](https://pkg.go.dev/github.com/NortonBen/ai-memory-go)
```

## Local Testing

Before pushing, you can run these commands locally:

```bash
# Run tests
go test -v -race -coverprofile=coverage.out ./...

# Run linter
golangci-lint run --timeout=5m

# Build
go build -v ./...

# Security scan
gosec ./...
```

## Troubleshooting

### Tests Failing

If tests fail in CI but pass locally:
1. Check Go version compatibility (CI tests on 1.22 and 1.23)
2. Ensure all dependencies are in go.mod
3. Check for race conditions (CI runs with `-race` flag)

### Linter Errors

If linter fails:
1. Run `golangci-lint run` locally
2. Fix reported issues
3. Consider adjusting `.golangci.yml` if rules are too strict

### Build Failures

If build fails:
1. Ensure `go mod tidy` has been run
2. Check for syntax errors
3. Verify all imports are available

## Future Enhancements

Potential additions to the CI/CD pipeline:
- Release automation with goreleaser
- Docker image building and publishing
- Integration tests with real databases
- Performance benchmarking
- Dependency vulnerability scanning
- Multi-platform builds (Linux, macOS, Windows)
