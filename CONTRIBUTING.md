# Contributing to CQI

Thank you for your interest in contributing to the Crypto Quant Infrastructure (CQI) library!

## Development Setup

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose (for integration tests)
- Git

### Getting Started

1. **Clone the repository**
   ```bash
   git clone https://github.com/Combine-Capital/cqi.git
   cd cqi
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Run unit tests**
   ```bash
   go test ./pkg/...
   ```

4. **Run integration tests** (requires Docker)
   ```bash
   docker compose -f test/integration/docker-compose.yml up -d
   go test ./test/integration/...
   docker compose -f test/integration/docker-compose.yml down
   ```

## Project Structure

```
pkg/           # Public packages (stable API)
  ├── auth/    # Authentication middleware
  ├── bus/     # Event bus implementations
  ├── cache/   # Cache client
  ├── config/  # Configuration management
  ├── database/# Database connection pool
  ├── errors/  # Error types
  ├── health/  # Health check framework
  ├── logging/ # Structured logging
  ├── metrics/ # Prometheus metrics
  ├── registry/# Service discovery
  ├── retry/   # Retry logic
  ├── runner/  # Service orchestration
  ├── service/ # Service lifecycle
  └── tracing/ # Distributed tracing

internal/      # Private implementation details
examples/      # Working code examples
test/          # Integration tests
docs/          # Documentation
```

## Testing Guidelines

### Unit Tests

- All public packages must have unit tests
- Target >90% code coverage per package
- Use table-driven tests where appropriate
- Mock external dependencies (database, Redis, NATS)

**Example:**
```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "test", false},
        {"empty input", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Feature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Feature() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Feature() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

- Use real infrastructure (Postgres, Redis, NATS) via Docker Compose
- Located in `test/integration/`
- Run with `go test ./test/integration/...`
- Clean up resources after tests

### Race Detection

Always run tests with race detection before submitting:
```bash
go test -race ./...
```

## Code Style

### Go Conventions

- Follow standard Go formatting: `gofmt -s`
- Use meaningful variable names (no single-letter vars except loops)
- Keep functions focused and small
- Document all exported types and functions

### Documentation

- All packages must have package-level documentation
- Exported functions/types need godoc comments
- Include usage examples in package documentation

**Example:**
```go
// Package example provides example functionality.
//
// Example usage:
//
//	client := example.New()
//	result, err := client.DoSomething(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
package example
```

### Error Handling

- Use typed errors from `pkg/errors`
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Don't panic in library code (use errors)

### Context

- Always pass `context.Context` as the first parameter
- Respect context cancellation in long-running operations
- Use `context.WithTimeout` for operations with deadlines

## Pull Request Process

1. **Create a feature branch**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes**
   - Write tests first (TDD approach)
   - Implement the feature
   - Ensure all tests pass
   - Run race detection

3. **Commit your changes**
   ```bash
   git add .
   git commit -m "feat: add new feature"
   ```

   Follow [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat:` - New feature
   - `fix:` - Bug fix
   - `docs:` - Documentation only
   - `test:` - Adding tests
   - `refactor:` - Code refactoring
   - `perf:` - Performance improvement
   - `chore:` - Maintenance tasks

4. **Push and create PR**
   ```bash
   git push origin feature/my-feature
   ```

5. **PR Requirements**
   - All tests must pass (CI will verify)
   - Code coverage should not decrease
   - No race conditions detected
   - Documentation updated if needed
   - CHANGELOG.md updated for user-facing changes

## Code Review

- All PRs require at least one approval
- Address all review comments
- Keep PRs focused and reasonably sized
- Be responsive to feedback

## Versioning

We use [Semantic Versioning](https://semver.org/):
- MAJOR: Incompatible API changes
- MINOR: Backward-compatible functionality
- PATCH: Backward-compatible bug fixes

## Release Process

1. Update CHANGELOG.md with release notes
2. Update version in relevant files
3. Create a git tag: `git tag v0.x.0`
4. Push tag: `git push origin v0.x.0`
5. GitHub Actions will create the release

## Getting Help

- Open an issue for bugs or feature requests
- Check existing issues before creating new ones
- Provide minimal reproducible examples for bugs
- Tag issues appropriately (bug, enhancement, question)

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Assume good intentions

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

Thank you for contributing to CQI! 🚀
