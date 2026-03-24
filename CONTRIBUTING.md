# Contributing to AI Memory Integration

Thank you for your interest in contributing to the AI Memory Integration library!

## Development Setup

### Prerequisites

- Go 1.22 or 1.23
- Git
- golangci-lint (for local linting)
- gosec (for security scanning)

### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/ai-memory-go.git
   cd ai-memory-go
   ```

3. Install dependencies:
   ```bash
   go mod download
   ```

4. Create a new branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Workflow

### Running Tests

Before submitting a pull request, ensure all tests pass:

```bash
# Run all tests
go test -v ./...

# Run tests with race detection
go test -v -race ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...
```

### Code Quality

#### Linting

We use golangci-lint for code quality checks:

```bash
# Run linter
golangci-lint run --timeout=5m

# Auto-fix issues where possible
golangci-lint run --fix
```

#### Formatting

Ensure your code is properly formatted:

```bash
# Format code
go fmt ./...

# Organize imports
goimports -w .
```

#### Security Scanning

Run security checks before committing:

```bash
# Run gosec
gosec ./...
```

### Building

Verify your changes build successfully:

```bash
# Build all packages
go build -v ./...

# Build main binary
go build -o ai-memory-go -v .
```

## Pull Request Process

1. **Update Documentation**: If you're adding new features, update relevant documentation
2. **Add Tests**: Ensure new code has appropriate test coverage
3. **Run CI Checks Locally**: Run tests, linting, and builds before pushing
4. **Commit Messages**: Use clear, descriptive commit messages
5. **Pull Request Description**: Clearly describe what your PR does and why

### PR Checklist

- [ ] Tests pass locally (`go test -v -race ./...`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] Security scan passes (`gosec ./...`)
- [ ] Documentation updated (if applicable)
- [ ] Tests added for new functionality
- [ ] Commit messages are clear and descriptive

## CI/CD Pipeline

All pull requests automatically run through our CI/CD pipeline, which includes:

- **Test Job**: Runs tests on Go 1.22 and 1.23 with race detection
- **Lint Job**: Runs golangci-lint with comprehensive checks
- **Build Job**: Verifies the code builds on multiple Go versions
- **Security Job**: Runs Gosec security scanner

Your PR must pass all CI checks before it can be merged.

## Code Style Guidelines

### General Principles

- Follow standard Go conventions and idioms
- Write clear, self-documenting code
- Add comments for complex logic
- Keep functions focused and small
- Use meaningful variable and function names

### Package Organization

- Each package should have a clear, single responsibility
- Use interfaces for abstraction and testability
- Keep internal implementation details private
- Export only what's necessary for public API

### Error Handling

- Always check and handle errors
- Provide context in error messages
- Use `fmt.Errorf` with `%w` for error wrapping
- Don't panic in library code (except for truly unrecoverable errors)

### Testing

- Write table-driven tests where appropriate
- Test both success and error cases
- Use meaningful test names that describe what's being tested
- Aim for high test coverage (>80%)
- Use subtests for better organization

### Concurrency

- Use channels and goroutines idiomatically
- Always handle context cancellation
- Avoid data races (test with `-race` flag)
- Document goroutine lifecycle and ownership

## Project Structure

```
ai-memory-go/
├── .github/
│   └── workflows/
│       ├── ci.yml           # CI/CD pipeline
│       └── README.md        # Workflow documentation
├── extractor/               # LLM extraction logic
├── graph/                   # Graph database interfaces
├── parser/                  # Document parsing
├── schema/                  # Data schemas
├── storage/                 # Storage interfaces
├── vector/                  # Vector database interfaces
├── .golangci.yml           # Linter configuration
├── go.mod                  # Go module definition
├── main.go                 # Main entry point
└── README.md               # Project documentation
```

## Getting Help

- Check existing issues and pull requests
- Read the design document in `.kiro/specs/ai-memory-integration/design.md`
- Review the requirements in `.kiro/specs/ai-memory-integration/requirements.md`
- Ask questions in pull request comments

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.

## Code of Conduct

- Be respectful and inclusive
- Provide constructive feedback
- Focus on the code, not the person
- Help create a welcoming environment for all contributors
