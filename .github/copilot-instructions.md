# Copilot Instructions for CQI

## Context & Documentation

Always use Context7 for current docs on frameworks/libraries/APIs; invoke automatically without being asked.

## Development Standards

### Go Best Practices
- Always pass `context.Context` as first parameter for cancellation/timeout support across all infrastructure operations.
- Use `defer` for cleanup (Close, Shutdown, Rollback) immediately after resource acquisition to prevent leaks.

### zerolog Patterns
- Never allocate in hot paths; use `logger.With()` to create child loggers with persistent fields instead of per-call field allocation.

### pgx Database Guidelines
- Always use `pgxpool.Pool` not single connections; prepared statements are automatically cached per connection.
- Wrap multi-statement operations in `WithTransaction()` helper for automatic rollback on error, never manual Begin/Commit/Rollback.

### Kafka Integration
- Consumer groups must have unique IDs per service; reusing group IDs across services causes message loss.

### Protocol Buffers
- Import CQC types directly (`github.com/Combine-Capital/cqc/gen/go/cqc/*`); never define infrastructure protobufs locally until Post-MVP.

### Code Quality Standards
- All exported functions must accept `context.Context` for timeout/cancellation; operations without context fail in production under load.
- Integration tests require real infrastructure (Docker Compose); no mocks for database/cache/Kafka in integration tests.

### Project Conventions
- Public API lives in `pkg/`; `internal/` for implementation details not meant for consuming services.
- Each package must have package-level godoc with usage example; examples are tested as part of CI.

### Agentic AI Guidelines
- Never create "summary" documents; direct action is more valuable than summarization.
