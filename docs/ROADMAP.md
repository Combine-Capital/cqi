# Implementation Roadmap

## Progress Checklist
- [x] **Commit 1**: Project Foundation & Module Setup
- [x] **Commit 2**: Foundation Packages (Errors & Config)
- [x] **Commit 3**: Logging Package
- [x] **Commit 4**: Retry Package
- [x] **Commit 5**: Metrics Package
- [x] **Commit 6**: Tracing Package
- [x] **Commit 7**: Database Package
- [x] **Commit 8**: Cache Package
- [ ] **Commit 9**: Event Bus Package
- [ ] **Commit 10**: Authentication Package
- [ ] **Commit 11**: Health Check Framework
- [ ] **Commit 12**: Examples & Integration Tests
- [ ] **Commit 13**: Documentation & Release Preparation

## Implementation Sequence

### Commit 1: Project Foundation & Module Setup

**Goal**: Establish Go module structure and project scaffolding with CQC dependency
**Depends**: none

**Deliverables**:
- [x] Initialize Go module at `go.mod` with path `github.com/Combine-Capital/cqi`
- [x] Add CQC dependency: `github.com/Combine-Capital/cqc` (for event type imports)
- [x] Create directory structure: `pkg/`, `examples/`, `test/`, `docs/`, `internal/`
- [x] Create `README.md` with library overview, installation instructions, and quick start guide
- [x] Create `LICENSE` file
- [x] Copy `docs/BRIEF.md` and `docs/SPEC.md` to repository

**Success**:
- `go mod init github.com/Combine-Capital/cqi` completes successfully (exits with code 0)
- `go get github.com/Combine-Capital/cqc@latest` resolves and adds to go.mod
- All directories exist: `pkg/`, `examples/`, `test/integration/`, `test/testdata/`, `docs/`
- `go mod tidy` runs without errors

---

### Commit 2: Foundation Packages (Errors & Config)

**Goal**: Implement error types and configuration loading as foundation for all other packages
**Depends**: Commit 1

**Deliverables**:
- [x] Create `pkg/errors/errors.go` with error type definitions: Permanent, Temporary, NotFound, InvalidInput, Unauthorized
- [x] Create `pkg/errors/wrapping.go` with error wrapping functions preserving type information
- [x] Create `pkg/errors/types.go` with type checking functions: IsPermanent, IsTemporary, IsNotFound, etc.
- [x] Create `pkg/errors/http.go` with HTTP status code mapping (404, 400, 401, 503, 500)
- [x] Create `pkg/errors/grpc.go` with gRPC status code mapping using google.golang.org/grpc/status
- [x] Create `pkg/errors/middleware.go` with RecoveryMiddleware for panic catching
- [x] Create `pkg/errors/errors_test.go` with unit tests for all error types and wrapping
- [x] Create `pkg/config/config.go` with config structs: DatabaseConfig, CacheConfig, EventBusConfig, ServerConfig, LogConfig, MetricsConfig, TracingConfig
- [x] Create `pkg/config/loader.go` with Load() and MustLoad() using viper for env vars and YAML/JSON
- [x] Create `pkg/config/validator.go` with validation for required fields and default values
- [x] Create `pkg/config/config_test.go` with unit tests for config loading and validation

**Success**:
- All error constructors return correct types: NewPermanent(), NewTemporary(), NewNotFound(), etc.
- Error type checking functions correctly identify error types using errors.As
- HTTP mapping returns correct status codes: NotFound→404, InvalidInput→400, Unauthorized→401, Temporary→503, Permanent→500
- Config loads from YAML file and environment variables with prefix support (e.g., CQI_DATABASE_HOST)
- Config validation catches missing required fields and applies default values
- `go test ./pkg/errors/...` and `go test ./pkg/config/...` pass with >90% coverage

---

### Commit 3: Logging Package

**Goal**: Implement structured logging with zerolog for trace context propagation
**Depends**: Commit 2 (config, errors)

**Deliverables**:
- [x] Add zerolog dependency: `github.com/rs/zerolog`
- [x] Create `pkg/logging/fields.go` with standard field constants: TraceID, SpanID, ServiceName, Timestamp, Level, Message, Error
- [x] Create `pkg/logging/logger.go` with Logger interface and zerolog implementation accepting LogConfig
- [x] Create `pkg/logging/context.go` with logger context propagation helpers for context.Context
- [x] Create `pkg/logging/middleware.go` with HTTP/gRPC middleware for request/response logging with duration and status
- [x] Create `pkg/logging/logging_test.go` with unit tests for logger creation, field injection, and middleware

**Success**:
- Logger instance created from LogConfig with configurable log level (debug, info, warn, error)
- Logger extracts trace/span IDs from context.Context and includes in JSON output
- Log level control works via LOG_LEVEL environment variable
- HTTP middleware logs request start/end with method, path, status, duration
- `go test ./pkg/logging/...` passes with >90% coverage
- Zero allocations in hot path confirmed by benchmark tests

---

### Commit 4: Retry Package ✅

**Goal**: Implement retry logic with exponential backoff independent of other infrastructure
**Depends**: Commit 2 (errors for retry policies)

**Deliverables**:
- [x] Add `github.com/cenkalti/backoff/v5` for battle-tested exponential backoff with jitter
- [x] Create `pkg/retry/config.go` with retry policies: PolicyTemporary (only Temporary errors), PolicyAll, PolicyNone, PolicyFunc (custom)
- [x] Create `pkg/retry/retry.go` with Do(ctx, config, fn) and DoWithData[T](ctx, config, fn) functions respecting context cancellation
- [x] Create `pkg/retry/retry_test.go` with comprehensive unit tests for retry execution, policies, backoff, and context cancellation

**Success**:
- ✅ Retry() executes function with exponential backoff up to configured max attempts
- ✅ Backoff delay includes random jitter (±25%) to prevent thundering herd
- ✅ Context cancellation stops retry immediately during backoff sleep
- ✅ Retry policies correctly classify errors: only Temporary errors retried with PolicyTemporary
- ✅ `go test ./pkg/retry/...` passes with 91.7% coverage (>90%)

---

### Commit 5: Metrics Package ✅

**Goal**: Implement Prometheus metrics collection with standardized naming
**Depends**: Commit 2 (config)

**Deliverables**:
- [x] Add Prometheus dependency: `github.com/prometheus/client_golang/prometheus`
- [x] Create `pkg/metrics/metrics.go` with Init() function, registry, and HTTP handler on configured port/path
- [x] Create `pkg/metrics/collectors.go` with type-safe constructors: NewCounter, NewGauge, NewHistogram with label validation
- [x] Create `pkg/metrics/standard.go` with standard HTTP metrics (http_request_duration_seconds, http_request_count_total, http_request_size_bytes, http_response_size_bytes) and gRPC metrics (grpc_call_duration_seconds, grpc_call_count_total)
- [x] Create `pkg/metrics/middleware.go` with HTTP/gRPC middleware that automatically records metrics with labels (method, path, status_code)
- [x] Create `pkg/metrics/metrics_test.go` with unit tests for metric registration, collection, and middleware

**Success**:
- ✅ Metrics endpoint `/metrics` returns Prometheus text exposition format
- ✅ Metric naming follows convention: {service_name}_{subsystem}_{metric_name}
- ✅ HTTP middleware records request_duration, request_count with labels for method, path, status_code
- ✅ Duplicate metric registration is prevented with validation error
- ✅ `go test ./pkg/metrics/...` passes with 91.5% coverage (>90%)

---

### Commit 6: Tracing Package ✅

**Goal**: Implement OpenTelemetry distributed tracing with W3C trace context
**Depends**: Commit 2 (config)

**Deliverables**:
- [x] Add OpenTelemetry dependencies: `go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/exporters/otlp/otlptrace`
- [x] Create `pkg/tracing/tracer.go` with tracer provider initialization accepting TracingConfig with OTLP endpoint and sample rate
- [x] Create `pkg/tracing/span.go` with StartSpan(ctx, name) helper that creates child spans with automatic parent linking
- [x] Create `pkg/tracing/propagation.go` with trace context injection/extraction for traceparent/tracestate headers (W3C standard)
- [x] Create `pkg/tracing/middleware.go` with HTTP/gRPC middleware for automatic span creation with attributes (method, path, status_code)
- [x] Create `pkg/tracing/tracing_test.go` with unit tests for span creation, context propagation, and middleware

**Success**:
- ✅ Tracer provider initializes with configured OTLP endpoint and sample rate
- ✅ StartSpan creates spans that are automatically linked to parent span from context
- ✅ Trace context is extracted from incoming traceparent header and injected into outbound requests
- ✅ HTTP/gRPC middleware creates root spans for incoming requests with method, path, status attributes
- ✅ Graceful shutdown flushes pending spans before service termination
- ✅ `go test ./pkg/tracing/...` passes with 93.0% coverage (>90%)

---

### Commit 7: Database Package ✅

**Goal**: Implement PostgreSQL connection pooling with transaction helpers
**Depends**: Commit 2 (config), Commit 3 (logging)

**Deliverables**:
- [x] Add pgx dependencies: `github.com/jackc/pgx/v5`, `github.com/jackc/pgx/v5/pgxpool`, `github.com/pashagolub/pgxmock/v4`
- [x] Create `pkg/database/database.go` with Database interface: Query(ctx, sql, args), QueryRow(ctx, sql, args), Exec(ctx, sql, args)
- [x] Create `pkg/database/pool.go` with NewPool(ctx, DatabaseConfig) creating pgxpool.Pool with min/max connections and timeouts
- [x] Create `pkg/database/transaction.go` with transaction helpers: Begin(ctx), Commit(), Rollback(), WithTransaction(ctx, func) with automatic rollback on error
- [x] Create `pkg/database/health.go` with health checker executing SELECT 1 with timeout
- [x] Create `pkg/database/pool_test.go` with unit tests for pool operations, Query, QueryRow, Exec, Begin
- [x] Create `pkg/database/transaction_test.go` with unit tests for transactions, commit, rollback, WithTransaction
- [x] Create `pkg/database/health_test.go` with unit tests for CheckHealth and PingWithTimeout

**Success**:
- ✅ Connection pool created with configured min/max connections from DatabaseConfig
- ✅ All query methods respect context cancellation and timeout
- ✅ WithTransaction automatically rolls back on error and panic, commits on success
- ✅ Auto-reconnect handled by pgxpool automatically with exponential backoff
- ✅ Health check returns error if database unreachable or timeout exceeded
- ✅ `go test ./pkg/database/...` passes with 62.7% coverage (unit tests cover all testable logic; NewPool requires integration tests)

---

### Commit 8: Cache Package ✅

**Goal**: Implement Redis caching with protobuf serialization
**Depends**: Commit 1 (CQC dependency), Commit 2 (config), Commit 3 (logging)

**Deliverables**:
- [x] Add redis dependency: `github.com/redis/go-redis/v9`
- [x] Add protobuf dependency: `google.golang.org/protobuf/proto`
- [x] Create `pkg/cache/cache.go` with Cache interface: Get(ctx, key, dest proto.Message), Set(ctx, key, value proto.Message, ttl), Delete(ctx, key), Exists(ctx, key)
- [x] Create `pkg/cache/redis.go` with NewRedis(CacheConfig) creating redis.Client with connection settings
- [x] Create `pkg/cache/key.go` with Key(prefix, parts...) builder for consistent cache key formatting
- [x] Create `pkg/cache/aside.go` with GetOrLoad(ctx, key, loader func, ttl) cache-aside helper that populates cache on miss
- [x] Create `pkg/cache/health.go` with health checker executing Redis PING command
- [x] Create `pkg/cache/cache_test.go` with unit tests using redis mock

**Success**:
- ✅ Cache serializes protobuf messages to wire format using proto.Marshal before storing in Redis
- ✅ Cache deserializes wire format to protobuf messages using proto.Unmarshal on retrieval
- ✅ Cache key builder creates consistent keys with prefix and parts joined by ":"
- ✅ GetOrLoad executes loader function on cache miss and stores result with configured TTL
- ✅ Cache operations handle Redis connection failures gracefully (return error, no panic)
- ✅ Health check verifies Redis connectivity with PING command
- ✅ `go test ./pkg/cache/...` passes with 93.0% coverage (>90%)

---

### Commit 9: Event Bus Package

**Goal**: Implement event bus with Kafka and in-memory backends
**Depends**: Commit 1 (CQC dependency), Commit 2 (config), Commit 3 (logging), Commit 4 (retry), Commit 5 (metrics)

**Deliverables**:
- [ ] Add Kafka dependency: `github.com/segmentio/kafka-go`
- [ ] Import CQC event types: `github.com/Combine-Capital/cqc/gen/go/cqc/events/v1`
- [ ] Create `pkg/bus/bus.go` with EventBus interface: Publish(ctx, topic, proto.Message) error, Subscribe(ctx, topic, handler) error
- [ ] Create `pkg/bus/kafka.go` with KafkaEventBus implementation using segmentio/kafka-go with broker list and consumer group from EventBusConfig
- [ ] Create `pkg/bus/memory.go` with MemoryEventBus using Go channels for testing/development
- [ ] Create `pkg/bus/topics.go` with topic naming convention helpers: TopicName(eventType) returns "cqc.events.v1.{event_type}"
- [ ] Create `pkg/bus/middleware.go` with subscriber middleware: WithRetry, WithLogging, WithMetrics options for handler chaining
- [ ] Create `pkg/bus/bus_test.go` with unit tests for both backends and middleware

**Success**:
- Publish serializes CQC protobuf events to wire format before sending to Kafka topic
- Subscribe deserializes wire format to protobuf events before invoking handler
- Topic names follow convention: "cqc.events.v1.asset_created", "cqc.events.v1.position_changed"
- Subscriber middleware chains: logging logs event type/size, metrics records handler duration, retry retries on Temporary errors
- KafkaEventBus handles consumer group rebalancing gracefully
- MemoryEventBus delivers messages via channels for fast testing
- Graceful shutdown flushes pending messages and closes Kafka connections
- `go test ./pkg/bus/...` passes with >90% coverage

---

### Commit 10: Authentication Package

**Goal**: Implement authentication middleware for API keys and JWT tokens
**Depends**: Commit 2 (config, errors)

**Deliverables**:
- [ ] Add JWT dependency: `github.com/golang-jwt/jwt/v5`
- [ ] Create `pkg/auth/auth.go` with AuthContext type containing UserID, ServiceID, AuthType (API_KEY, JWT), Claims map
- [ ] Create `pkg/auth/context.go` with GetAuthContext(ctx) utility to extract auth from context.Context
- [ ] Create `pkg/auth/apikey.go` with APIKeyMiddleware(validKeys []string) for HTTP and APIKeyInterceptor for gRPC checking Authorization: Bearer {key}
- [ ] Create `pkg/auth/jwt.go` with JWTMiddleware(publicKey, issuer, audience) for HTTP and JWTInterceptor for gRPC validating JWT signatures and claims (exp, iss, aud)
- [ ] Create `pkg/auth/auth_test.go` with unit tests for both API key and JWT validation

**Success**:
- API key middleware validates Authorization header against configured list of valid keys
- JWT middleware parses JWT token, verifies signature with public key, validates exp/iss/aud claims
- Auth context is injected into context.Context and retrievable via GetAuthContext
- Invalid auth returns 401 Unauthorized (HTTP) or Unauthenticated (gRPC) status
- Both HTTP middleware and gRPC interceptor supported for each auth type
- `go test ./pkg/auth/...` passes with >90% coverage

---

### Commit 11: Health Check Framework

**Goal**: Implement health check framework with liveness and readiness endpoints
**Depends**: Commit 7 (database), Commit 8 (cache), Commit 9 (bus)

**Deliverables**:
- [ ] Create `pkg/health/checker.go` with Checker interface: Check(ctx) error implemented by all infrastructure components
- [ ] Create `pkg/health/health.go` with Health struct and RegisterChecker(name, checker) method
- [ ] Create `pkg/health/handlers.go` with HTTP handlers: LivenessHandler() returns 200 if service running (no dependency checks), ReadinessHandler() returns 200 if all checkers pass, 503 otherwise
- [ ] Implement database health checker in `pkg/database/health.go` executing SELECT 1 with 5 second timeout
- [ ] Implement cache health checker in `pkg/cache/health.go` executing Redis PING with 5 second timeout
- [ ] Implement event bus health checker in `pkg/bus/kafka.go` verifying broker connectivity with 5 second timeout
- [ ] Create `pkg/health/health_test.go` with unit tests for health check registration and execution

**Success**:
- /health/live endpoint returns 200 OK immediately (no dependency checks)
- /health/ready endpoint executes all registered checkers with 5 second timeout per checker
- /health/ready returns JSON: {"status": "healthy|unhealthy", "checks": {"database": "ok", "cache": "error: connection refused"}}
- Failed health checks log error details but don't crash service
- Health check results cached for 1 second to prevent checker stampede under load
- `go test ./pkg/health/...` passes with >90% coverage

---

### Commit 12: Examples & Integration Tests

**Goal**: Provide working examples and integration tests with real infrastructure
**Depends**: All previous commits (1-11)

**Deliverables**:
- [ ] Create `examples/simple/main.go` demonstrating minimal usage: config, logging, database
- [ ] Create `examples/simple/config.yaml` with example configuration for simple service
- [ ] Create `examples/full/main.go` demonstrating all packages: config, logging, metrics, tracing, database, cache, bus, auth, health
- [ ] Create `examples/full/config.yaml` with complete configuration example including all infrastructure components
- [ ] Create `examples/full/README.md` with step-by-step setup and running instructions
- [ ] Create `test/integration/docker-compose.yml` with Postgres 14, Redis 7, Kafka 3.0 services
- [ ] Create `test/integration/database_test.go` with integration tests connecting to real Postgres (Query, Exec, transactions)
- [ ] Create `test/integration/cache_test.go` with integration tests connecting to real Redis (Get, Set, Delete, protobuf serialization)
- [ ] Create `test/integration/bus_test.go` with integration tests connecting to real Kafka (Publish, Subscribe, middleware)
- [ ] Create `test/testdata/configs/test_config.yaml` with test configuration
- [ ] Create `internal/common/testutil/containers.go` with Docker container setup helpers for tests

**Success**:
- `cd examples/simple && go run main.go` starts service successfully with config loaded and database connected
- `cd examples/full && go run main.go` starts service with all infrastructure initialized (DB, cache, Kafka, metrics, tracing, health endpoints)
- `docker-compose -f test/integration/docker-compose.yml up -d` starts Postgres, Redis, Kafka containers
- `go test ./test/integration/...` passes with real Postgres, Redis, Kafka connections (all integration tests green)
- Integration tests verify actual database queries, cache operations, event publishing/subscribing
- Examples serve as documentation showing real usage patterns with CQC event types

---

### Commit 13: Documentation & Release Preparation

**Goal**: Complete package documentation and prepare for initial release
**Depends**: Commit 12

**Deliverables**:
- [ ] Add package-level godoc to all 11 packages in `pkg/` with overview and usage example
- [ ] Update `README.md` with installation instructions, quick start guide (15 line example), architecture overview, and links to examples
- [ ] Create `CONTRIBUTING.md` with development setup, testing guidelines, and PR process
- [ ] Create `CHANGELOG.md` with v0.1.0 release notes listing all MVP features
- [ ] Verify all unit tests pass: `go test ./pkg/...` >90% coverage
- [ ] Verify all integration tests pass: `go test ./test/integration/...` with Docker infrastructure
- [ ] Run `go mod tidy` to clean up dependencies
- [ ] Tag release: `git tag v0.1.0`

**Success**:
- All packages have godoc that shows up in pkg.go.dev after publish
- README demonstrates complete service setup in 15-20 lines matching BRIEF success criteria
- `go test ./...` passes with >90% total coverage
- `go test -race ./...` passes with no race conditions detected
- Module is ready for consumption: `go get github.com/Combine-Capital/cqi@v0.1.0` works
- Documentation enables new service setup in <30 minutes (BRIEF success metric)
