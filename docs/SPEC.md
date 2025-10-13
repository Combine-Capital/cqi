# MVP Technical Specification: CQI - Crypto Quant Infrastructure

**Project Type:** Shared Infrastructure Library (Go package providing event bus, database, cache, observability, auth)

## Core Requirements (from Brief)

### MVP Scope

#### Event Bus & Messaging
- Import CQC event types from github.com/Combine-Capital/cqc/gen/go/cqc/events/v1
- Provide event bus interface: Publish(ctx, topic, proto.Message) error and Subscribe(ctx, topic, handler) error
- Support in-memory backend (testing) and Kafka backend (production) with automatic protobuf serialization
- Provide subscriber middleware for logging, metrics, error handling, and retry
- Define topic naming convention: "cqc.events.v1.{event_type}"

#### Data Storage & Caching
- Provide PostgreSQL connection pool with health checks, auto-reconnection, configurable limits, graceful shutdown
- Provide database interface: Query, QueryRow, Exec with context support for cancellation/timeout
- Provide transaction helpers: Begin, Commit, Rollback, WithTransaction (auto-rollback on error)
- Provide Redis client: Get/Set/Delete/Exists with native CQC protobuf serialization
- Provide cache key builder with consistent naming and TTL support
- Provide cache-aside helpers for automatic populate-on-miss

#### Observability & Monitoring
- Define standard log fields: trace_id, span_id, service_name, timestamp, level, message, error
- Provide structured logger (zerolog/zap) with context propagation and env-based log level control
- Provide Prometheus client: Counter, Gauge, Histogram with naming convention {service}_{subsystem}_{metric}
- Provide standard HTTP metrics (request_duration, request_count, request/response_size) and gRPC metrics (call_duration, call_count)
- Provide OpenTelemetry tracing with span creation, context propagation, traceparent/tracestate header injection/extraction
- Provide HTTP/gRPC middleware: auto-span creation, trace context injection, request/response logging
- Provide health check framework: /health/live and /health/ready endpoints with component checkers (database, cache, bus)

#### Configuration & Secrets
- Define config structs: DatabaseConfig, CacheConfig, EventBusConfig, ServerConfig, LogConfig, MetricsConfig, TracingConfig
- Load config from environment variables with prefix support (e.g., CQI_DATABASE_HOST) using viper
- Load config from YAML/JSON with env var override
- Validate config: required field checks, default values
- Load secrets from env vars (document external secret manager for production)

#### Security & Authentication
- Define auth context types: API key and JWT with user/service identity fields
- Provide HTTP/gRPC middleware for API key validation against configured keys
- Provide HTTP/gRPC middleware for JWT validation with public key verification and claims extraction
- Provide utilities to extract auth from request metadata and propagate via context.Context

#### Reliability & Error Handling
- Provide retry package with exponential backoff, jitter, configurable max attempts, per-error-type policies
- Define error types: Permanent, Temporary, NotFound, InvalidInput, Unauthorized with wrapping support
- Provide error middleware: translate errors to HTTP status codes and gRPC status codes
- Provide panic recovery middleware for HTTP/gRPC with logging and metrics

#### Project Organization
- Organize code in pkg/ with subpackages: bus/, database/, cache/, logging/, metrics/, tracing/, config/, auth/, retry/, errors/, health/
- Document each package with usage examples
- Provide example configs and integration tests with CQC types

### Post-MVP Scope
- Contribute infrastructure protobufs to CQC for platform standardization
- Circuit breaker with configurable thresholds, timeout, half-open testing
- Rate limiting (token bucket/sliding window) per client/endpoint
- Secret management integration (Vault/AWS Secrets Manager) with rotation
- Service discovery (Consul/Kubernetes)
- Distributed locks (Redis/database)
- Graceful degradation for optional dependencies

## Technology Stack

### Core Technologies
- **Go 1.21+** - Primary language for all CQ platform services, excellent concurrency, strong typing
- **Protocol Buffers v3** - Message serialization format (via CQC dependency), type-safe event contracts
- **PostgreSQL 14+** - Relational database, ACID compliance, connection pooling via pgx
- **Redis 7+** - In-memory cache, low-latency data access, native protobuf serialization
- **Kafka 3.0+** - Distributed event streaming, at-least-once delivery, horizontal scalability

### Observability Stack
- **zerolog** - Zero-allocation structured logging, 10x faster than standard library, JSON output
- **Prometheus client_golang** - Pull-based metrics, industry standard, minimal overhead
- **OpenTelemetry Go SDK** - Distributed tracing, vendor-neutral, W3C trace context standard

### Configuration & Libraries
- **viper** - Configuration management, env/file/flag support, widely adopted
- **pgx/v5** - PostgreSQL driver, prepared statements, connection pooling, best-in-class performance
- **go-redis/v9** - Redis client, cluster support, pipelining, context support
- **segmentio/kafka-go** - Kafka client, zero-dependencies, context support, balanced consumer groups

### Justification for Choices
- **zerolog over zap**: Zero allocation in hot paths, simpler API, better performance (though both are excellent)
- **pgx over database/sql**: Native PostgreSQL features, better performance, prepared statement cache
- **segmentio/kafka-go over confluent**: Pure Go, no CGO dependencies, easier deployment, sufficient features for MVP
- **viper**: De facto standard for Go config management, supports all required formats

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         CQI Infrastructure Library                        │
│                    (Go Module: github.com/Combine-Capital/cqi)           │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ imports
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    CQC Contracts (Event Types)                            │
│              github.com/Combine-Capital/cqc/gen/go/cqc/*                 │
└──────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────┐
│                              CQI Package Structure                        │
│                                                                           │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────┐  ┌─────────────┐    │
│  │ pkg/config/ │  │ pkg/logging/ │  │ pkg/auth/ │  │ pkg/errors/ │    │
│  │             │  │              │  │           │  │             │    │
│  │ Load Config │  │  Logger      │  │ Middleware│  │  Error Types│    │
│  │ Validate    │  │  Log Fields  │  │ Context   │  │  Wrapping   │    │
│  └──────┬──────┘  └──────┬───────┘  └─────┬─────┘  └──────┬──────┘    │
│         │                │                  │                │           │
│         └────────────────┼──────────────────┼────────────────┘           │
│                          │                  │                            │
│  ┌──────────────────────┼──────────────────┼────────────────────────┐  │
│  │                      ▼                  ▼                        │  │
│  │    Core Infrastructure Components (depend on config/logging)    │  │
│  │                                                                  │  │
│  │  ┌────────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐  │  │
│  │  │ pkg/bus/   │  │ pkg/db/  │  │ pkg/cache│  │ pkg/metrics/│  │  │
│  │  │            │  │          │  │          │  │             │  │  │
│  │  │ EventBus   │  │ ConnPool │  │ Redis    │  │ Prometheus  │  │  │
│  │  │ - Kafka    │  │ Query    │  │ Proto    │  │ Counters    │  │  │
│  │  │ - Memory   │  │ Exec     │  │ Serialize│  │ Gauges      │  │  │
│  │  │ Middleware │  │ Tx Mgmt  │  │ CacheKey │  │ Histograms  │  │  │
│  │  └────────────┘  └──────────┘  └──────────┘  └─────────────┘  │  │
│  │                                                                  │  │
│  │  ┌────────────┐  ┌──────────┐  ┌──────────┐                   │  │
│  │  │pkg/tracing/│  │pkg/health│  │pkg/retry/│                   │  │
│  │  │            │  │          │  │          │                   │  │
│  │  │ OTEL       │  │ Liveness │  │ Backoff  │                   │  │
│  │  │ Span       │  │ Readiness│  │ Jitter   │                   │  │
│  │  │ Middleware │  │ Checkers │  │ Policies │                   │  │
│  │  └────────────┘  └──────────┘  └──────────┘                   │  │
│  └──────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ imported by
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                       Consuming Services                                  │
│  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐            │
│  │  cqar  │  │  cqvx  │  │  cqmd  │  │  cqex  │  │  cqpm  │  ...       │
│  └────────┘  └────────┘  └────────┘  └────────┘  └────────┘            │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ connects to
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                       External Infrastructure                             │
│  ┌────────────┐  ┌────────────┐  ┌───────────┐  ┌──────────────┐       │
│  │ PostgreSQL │  │   Redis    │  │   Kafka   │  │  Prometheus  │       │
│  │  Database  │  │   Cache    │  │  Events   │  │   Metrics    │       │
│  └────────────┘  └────────────┘  └───────────┘  └──────────────┘       │
└──────────────────────────────────────────────────────────────────────────┘
```

## Data Flow

### Primary User Flow: Service Developer Using CQI

**Scenario:** Developer builds new service (e.g., cqpm - Portfolio Management) that needs to publish PositionChanged events and query a database.

#### 1. Service Initialization (main.go)

```go
import (
    "github.com/Combine-Capital/cqi/pkg/config"
    "github.com/Combine-Capital/cqi/pkg/logging"
    "github.com/Combine-Capital/cqi/pkg/database"
    "github.com/Combine-Capital/cqi/pkg/bus"
    "github.com/Combine-Capital/cqi/pkg/cache"
    "github.com/Combine-Capital/cqi/pkg/metrics"
    "github.com/Combine-Capital/cqi/pkg/tracing"
    "github.com/Combine-Capital/cqi/pkg/health"
)

func main() {
    // Load configuration (env vars + YAML)
    cfg := config.MustLoad("config.yaml", "CQPM")
    
    // Initialize logger with trace context
    logger := logging.New(cfg.Log)
    
    // Initialize tracer
    tracer := tracing.New(cfg.Tracing, "cqpm")
    defer tracer.Shutdown()
    
    // Initialize metrics
    metrics.Init(cfg.Metrics, "cqpm")
    
    // Initialize database pool
    db := database.NewPool(cfg.Database)
    defer db.Close()
    
    // Initialize cache
    cache := cache.NewRedis(cfg.Cache)
    defer cache.Close()
    
    // Initialize event bus
    eventBus := bus.NewKafka(cfg.EventBus)
    defer eventBus.Close()
    
    // Initialize health checks
    health := health.New(
        health.WithDatabaseCheck(db),
        health.WithCacheCheck(cache),
        health.WithEventBusCheck(eventBus),
    )
    
    // Start HTTP server with health endpoints
    // Service is now fully instrumented - 15 lines of code
}
```

#### 2. Publishing Events

```go
import "github.com/Combine-Capital/cqc/gen/go/cqc/events/v1"

func (s *Service) UpdatePosition(ctx context.Context, position *Position) error {
    // Business logic: update position in database
    if err := s.db.Exec(ctx, "UPDATE positions SET ..."); err != nil {
        return err
    }
    
    // Publish event (automatic logging, metrics, serialization)
    event := &events.PositionChanged{
        PortfolioId: position.PortfolioId,
        AssetId: position.AssetId,
        NewQuantity: position.Quantity,
        Timestamp: timestamppb.Now(),
    }
    
    return s.eventBus.Publish(ctx, "cqc.events.v1.position_changed", event)
}
```

#### 3. Subscribing to Events

```go
func (s *Service) StartEventHandlers(ctx context.Context) error {
    // Subscribe with automatic retry, logging, metrics
    return s.eventBus.Subscribe(ctx, "cqc.events.v1.price_updated", 
        bus.HandlerFunc(func(ctx context.Context, msg proto.Message) error {
            priceEvent := msg.(*events.PriceUpdated)
            return s.handlePriceUpdate(ctx, priceEvent)
        }),
        bus.WithRetry(3, time.Second),
        bus.WithLogging(s.logger),
        bus.WithMetrics(),
    )
}
```

#### 4. Database Operations with Caching

```go
func (s *Service) GetAsset(ctx context.Context, assetId string) (*Asset, error) {
    // Try cache first
    cacheKey := cache.Key("asset", assetId)
    
    var asset Asset
    if err := s.cache.Get(ctx, cacheKey, &asset); err == nil {
        return &asset, nil // Cache hit
    }
    
    // Cache miss - query database
    if err := s.db.QueryRow(ctx, 
        "SELECT * FROM assets WHERE id = $1", assetId).Scan(&asset); err != nil {
        return nil, err
    }
    
    // Populate cache
    s.cache.Set(ctx, cacheKey, &asset, 5*time.Minute)
    
    return &asset, nil
}
```

#### 5. HTTP Endpoint with Full Observability

```go
import (
    "github.com/Combine-Capital/cqi/pkg/tracing"
    "github.com/Combine-Capital/cqi/pkg/auth"
)

func (s *Service) SetupRouter() *http.ServeMux {
    mux := http.NewServeMux()
    
    // Apply middleware (tracing, auth, metrics, logging, panic recovery)
    handler := tracing.HTTPMiddleware(
        auth.JWTMiddleware(s.jwtValidator)(
            metrics.HTTPMiddleware(
                logging.HTTPMiddleware(s.logger)(
                    errors.RecoveryMiddleware()(mux)))))
    
    mux.HandleFunc("/api/positions", s.handleGetPositions)
    
    return handler
}
```

## System Components

### Configuration Package (pkg/config/)
**Purpose:** Centralized configuration loading and validation for all infrastructure components

**Inputs:** 
- YAML/JSON configuration files
- Environment variables (with prefix, e.g., CQI_DATABASE_HOST)
- Default values hardcoded in structs

**Outputs:** 
- Validated configuration structs (DatabaseConfig, CacheConfig, EventBusConfig, etc.)
- Error if required fields missing or validation fails

**Dependencies:** 
- viper (config loading)
- validator/v10 (struct validation)

**Key Responsibilities:**
- Define configuration structs for all infrastructure components with struct tags
- Load config from YAML/JSON files using viper
- Override file config with environment variables using configurable prefix
- Validate required fields and value constraints (e.g., port ranges, positive timeouts)
- Populate default values for optional fields
- Provide MustLoad() helper that panics on error (for main.go initialization)
- Document configuration fields with examples in package godoc

**Post-MVP:**
- Hot reload configuration changes without service restart
- Configuration schema export for IDE autocomplete
- Remote configuration backend (Consul, etcd)

---

### Logging Package (pkg/logging/)
**Purpose:** Structured JSON logging with trace context propagation and consistent field names

**Inputs:** 
- LogConfig (level, format, output destination)
- Context with trace IDs from OpenTelemetry
- Log messages with structured fields

**Outputs:** 
- JSON log lines with standard fields to stdout/file
- Trace/span IDs automatically included from context

**Dependencies:** 
- zerolog (structured logging)
- OpenTelemetry for trace context extraction

**Key Responsibilities:**
- Create logger instance with configured log level (debug, info, warn, error)
- Define standard log field constants: TraceID, SpanID, ServiceName, Timestamp, Level, Message, Error
- Extract trace/span IDs from context.Context and automatically include in logs
- Provide logger methods: Debug, Info, Warn, Error with structured field support
- Provide HTTP/gRPC middleware that logs request start/end with duration, status, path
- Support log level control via environment variable (LOG_LEVEL)
- Zero-allocation logging for hot paths using zerolog's API

**Post-MVP:**
- Log sampling for high-volume debug logs
- Log aggregation helpers (ELK, Loki integration)
- Sensitive data masking for PII/credentials

---

### Metrics Package (pkg/metrics/)
**Purpose:** Prometheus metrics collection with standardized naming and type-safe collectors

**Inputs:** 
- MetricsConfig (port, path, enabled subsystems)
- Metric observations (counter increments, gauge sets, histogram observations)

**Outputs:** 
- Prometheus /metrics endpoint in text exposition format
- Standard metrics for HTTP/gRPC servers

**Dependencies:** 
- prometheus/client_golang (metrics collection and HTTP handler)

**Key Responsibilities:**
- Initialize Prometheus registry and HTTP handler on configured port/path
- Enforce naming convention: {service_name}_{subsystem}_{metric_name}
- Provide type-safe metric constructors: NewCounter, NewGauge, NewHistogram with label validation
- Define standard HTTP metrics: http_request_duration_seconds, http_request_count_total, http_request_size_bytes, http_response_size_bytes
- Define standard gRPC metrics: grpc_call_duration_seconds, grpc_call_count_total
- Provide HTTP/gRPC middleware that automatically records metrics
- Support metric labels for method, path, status_code, error_type
- Validate metric names at registration time (prevent duplicates)

**Post-MVP:**
- Metric cardinality limits to prevent label explosion
- Custom metric aggregation (summary percentiles)
- Push gateway support for batch jobs

---

### Tracing Package (pkg/tracing/)
**Purpose:** OpenTelemetry distributed tracing with W3C trace context propagation

**Inputs:** 
- TracingConfig (endpoint, service name, sample rate)
- HTTP/gRPC requests with traceparent headers
- Application code creating spans

**Outputs:** 
- Trace spans exported to OTLP endpoint (Jaeger, Tempo, etc.)
- Injected traceparent/tracestate headers in outbound requests

**Dependencies:** 
- go.opentelemetry.io/otel (tracing SDK)
- go.opentelemetry.io/otel/exporters/otlp (OTLP exporter)

**Key Responsibilities:**
- Initialize OpenTelemetry tracer provider with configured exporter
- Create root spans for incoming HTTP/gRPC requests
- Extract trace context from traceparent/tracestate headers (W3C standard)
- Inject trace context into outbound HTTP/gRPC requests
- Provide span creation helpers: StartSpan(ctx, name) with automatic parent linking
- Provide HTTP/gRPC middleware for automatic span creation with attributes (method, path, status)
- Record span events for key operations (database queries, cache hits/misses)
- Support trace sampling (always, never, probabilistic)
- Graceful shutdown with span flushing on service termination

**Post-MVP:**
- Automatic database query span creation with SQL text
- Custom span attributes for business operations
- Trace context propagation to Kafka messages

---

### Event Bus Package (pkg/bus/)
**Purpose:** Publish/subscribe event bus with CQC protobuf message support and multiple backends

**Inputs:** 
- EventBusConfig (backend type, Kafka brokers/topics, consumer group)
- CQC protobuf events to publish
- Event handlers to subscribe

**Outputs:** 
- Events published to Kafka topics (production) or in-memory channels (testing)
- Subscribed handlers invoked with deserialized protobuf messages

**Dependencies:** 
- github.com/Combine-Capital/cqc (event type definitions)
- segmentio/kafka-go (Kafka client)
- google.golang.org/protobuf (protobuf serialization)

**Key Responsibilities:**
- Define EventBus interface: Publish(ctx, topic, proto.Message) error and Subscribe(ctx, topic, handler) error
- Implement KafkaEventBus with segmentio/kafka-go for production
- Implement MemoryEventBus with Go channels for testing/development
- Serialize protobuf messages to wire format before publishing
- Deserialize wire format to protobuf messages before handler invocation
- Enforce topic naming convention: "cqc.events.v1.{event_type}"
- Provide subscriber middleware interface: logging, metrics, error handling, retry
- Support middleware chaining: WithRetry, WithLogging, WithMetrics options
- Handle Kafka consumer group rebalancing gracefully
- Provide graceful shutdown: flush pending messages, close connections

**Post-MVP:**
- Dead letter queue for failed messages after retries
- Message compression (gzip, snappy)
- Schema registry integration for version enforcement
- Event replay from offset

---

### Database Package (pkg/database/)
**Purpose:** PostgreSQL connection pooling with transaction management and health checks

**Inputs:** 
- DatabaseConfig (host, port, database, user, password, pool settings)
- SQL queries with parameters
- Transaction functions

**Outputs:** 
- Query results (rows, single row, exec result)
- Transaction commit/rollback
- Health check status

**Dependencies:** 
- jackc/pgx/v5 (PostgreSQL driver)
- jackc/pgx/v5/pgxpool (connection pooling)

**Key Responsibilities:**
- Create connection pool with configured min/max connections, timeouts
- Provide database interface: Query(ctx, sql, args), QueryRow(ctx, sql, args), Exec(ctx, sql, args)
- Respect context cancellation and timeout for all operations
- Provide transaction helpers: Begin(ctx), Commit(), Rollback()
- Provide WithTransaction(ctx, func(tx) error) helper with automatic rollback on error
- Implement health check: simple SELECT 1 query with timeout
- Auto-reconnect on connection loss with exponential backoff
- Graceful shutdown: wait for active queries, close connections
- Log slow queries with configurable threshold

**Post-MVP:**
- Query builder helpers to reduce SQL string concatenation
- Read replica support with automatic routing
- Prepared statement caching
- Query result streaming for large datasets

---

### Cache Package (pkg/cache/)
**Purpose:** Redis caching with native CQC protobuf serialization and cache patterns

**Inputs:** 
- CacheConfig (host, port, password, database, timeouts)
- Cache keys and protobuf messages
- TTL durations

**Outputs:** 
- Cached values (deserialized protobuf messages)
- Cache operation results (hit, miss, error)

**Dependencies:** 
- go-redis/v9 (Redis client)
- google.golang.org/protobuf (protobuf serialization)

**Key Responsibilities:**
- Create Redis client with configured connection settings
- Provide cache interface: Get(ctx, key, dest proto.Message) error, Set(ctx, key, value proto.Message, ttl) error, Delete(ctx, key) error, Exists(ctx, key) bool
- Serialize protobuf messages to wire format for storage
- Deserialize wire format to protobuf messages on retrieval
- Provide cache key builder: Key(prefix, parts...) string with consistent formatting
- Provide cache-aside helper: GetOrLoad(ctx, key, loader func() (proto.Message, error), ttl) that populates cache on miss
- Implement health check: Redis PING command
- Handle cache misses gracefully (return specific error, not panic)
- Support TTL per key (no global default)

**Post-MVP:**
- Cache invalidation patterns (tags, dependencies)
- Distributed cache locks for cache stampede prevention
- Cache warming utilities
- Redis cluster support

---

### Authentication Package (pkg/auth/)
**Purpose:** HTTP/gRPC authentication middleware with API key and JWT validation

**Inputs:** 
- AuthConfig (API keys, JWT public key, issuer, audience)
- HTTP/gRPC requests with Authorization headers
- JWT tokens to validate

**Outputs:** 
- Authenticated context with user/service identity
- HTTP 401/gRPC Unauthenticated errors for invalid auth

**Dependencies:** 
- golang-jwt/jwt/v5 (JWT parsing and validation)
- standard library crypto for public key loading

**Key Responsibilities:**
- Define AuthContext type with fields: UserID, ServiceID, AuthType (API_KEY, JWT), Claims map
- Provide HTTP middleware: APIKeyMiddleware(validKeys []string) that checks Authorization: Bearer {key}
- Provide HTTP middleware: JWTMiddleware(publicKey, issuer, audience) that validates JWT signatures and claims
- Provide gRPC interceptor: APIKeyInterceptor and JWTInterceptor with same validation logic
- Extract auth context from request metadata and inject into context.Context
- Provide utility: GetAuthContext(ctx) (*AuthContext, error) to retrieve auth from context
- Return 401 Unauthorized (HTTP) or Unauthenticated (gRPC) on validation failure
- Support multiple API keys (check against configured list)
- Validate JWT exp (expiration), iss (issuer), aud (audience) claims

**Post-MVP:**
- OIDC integration for external identity providers
- Permission/role checking middleware
- Auth token refresh helpers
- mTLS support for service-to-service auth

---

### Retry Package (pkg/retry/)
**Purpose:** Exponential backoff retry logic with jitter and per-error-type policies

**Inputs:** 
- RetryConfig (max attempts, initial backoff, max backoff, multiplier)
- Function to retry: func() error
- Retry policy: which errors are retryable

**Outputs:** 
- Success after retry or final error after exhausting attempts

**Dependencies:** 
- pkg/errors (error type classification)
- standard library time for backoff timing

**Key Responsibilities:**
- Provide Retry(ctx, config, fn func() error) error that executes fn with retries
- Implement exponential backoff: delay = initial * multiplier^attempt (capped at max)
- Add random jitter to backoff (±25%) to prevent thundering herd
- Respect context cancellation during backoff sleep
- Check error type against policy: only retry Temporary errors, never retry Permanent errors
- Count retry attempts and expose via metrics
- Log each retry attempt with error and next delay
- Provide default policies: RetryTemporary (only Temporary errors), RetryAll (all errors), RetryNone (no retries)
- Support custom retry policies: func(error) bool

**Post-MVP:**
- Circuit breaker integration (stop retrying if downstream is down)
- Retry budget (limit total retry percentage to prevent cascading failures)
- Configurable backoff strategies (linear, constant, custom)

---

### Errors Package (pkg/errors/)
**Purpose:** Standard error types with wrapping and HTTP/gRPC status code mapping

**Inputs:** 
- Base errors from application or dependencies
- Error types and messages

**Outputs:** 
- Typed errors (Permanent, Temporary, NotFound, etc.)
- HTTP status codes (500, 404, 400, 401)
- gRPC status codes (Internal, NotFound, InvalidArgument, Unauthenticated)

**Dependencies:** 
- standard library errors (wrapping)
- google.golang.org/grpc/status (gRPC status codes)

**Key Responsibilities:**
- Define error types: Permanent, Temporary, NotFound, InvalidInput, Unauthorized
- Provide constructors: NewPermanent(msg), NewTemporary(msg), NewNotFound(resource, id), etc.
- Implement error wrapping: errors.Wrap(err, "context") preserves type
- Provide type checking: IsPermanent(err), IsTemporary(err), IsNotFound(err) using errors.As
- Provide HTTP middleware: RecoveryMiddleware() that catches panics and maps errors to status codes
- Map error types to HTTP status codes: NotFound→404, InvalidInput→400, Unauthorized→401, Temporary→503, Permanent→500
- Map error types to gRPC status codes: NotFound→NotFound, InvalidInput→InvalidArgument, Unauthorized→Unauthenticated, Temporary→Unavailable, Permanent→Internal
- Include error cause chain in logs but not in HTTP response body (security)
- Provide standardized error response format: {"error": "message", "code": "NOT_FOUND"}

**Post-MVP:**
- Error aggregation for multiple validation errors
- Error codes for client error handling
- Localized error messages

---

### Health Check Package (pkg/health/)
**Purpose:** Liveness and readiness health checks for Kubernetes and load balancers

**Inputs:** 
- Component health checkers (database, cache, event bus)
- Health check requests to /health/live and /health/ready

**Outputs:** 
- HTTP 200 OK if healthy, 503 Service Unavailable if unhealthy
- JSON response with component statuses

**Dependencies:** 
- pkg/database (database health check)
- pkg/cache (cache health check)
- pkg/bus (event bus health check)

**Key Responsibilities:**
- Define Checker interface: Check(ctx) error implemented by all components
- Provide Health struct with RegisterChecker(name, checker) method
- Provide /health/live endpoint: returns 200 if service is running (no dependency checks)
- Provide /health/ready endpoint: returns 200 if all registered checkers pass, 503 otherwise
- Execute health checks with timeout (default 5 seconds)
- Return JSON response: {"status": "healthy|unhealthy", "checks": {"database": "ok", "cache": "error: connection refused"}}
- Database checker: execute SELECT 1 with timeout
- Cache checker: execute PING with timeout
- Event bus checker: verify connection to brokers
- Log health check failures with error details
- Cache health check results for 1 second to prevent stampede

**Post-MVP:**
- Startup probe endpoint for slow-starting services
- Detailed health check metrics (check duration, failure rate)
- Health check dependencies (service A requires service B)

## File Structure

```
cqi/
├── go.mod                          # Go module definition: github.com/Combine-Capital/cqi
├── go.sum                          # Dependency checksums
├── README.md                       # Library overview, installation, quick start
├── LICENSE                         # License file
│
├── docs/
│   ├── BRIEF.md                    # Project brief (from requirements)
│   └── SPEC.md                     # This technical specification
│
├── pkg/                            # Public API - all packages importable by consuming services
│   │
│   ├── config/                     # Configuration loading and validation
│   │   ├── config.go               # Config structs: DatabaseConfig, CacheConfig, EventBusConfig, etc.
│   │   ├── loader.go               # Load() and MustLoad() functions using viper
│   │   ├── validator.go            # Validation logic for required fields and constraints
│   │   └── config_test.go          # Unit tests for config loading and validation
│   │
│   ├── logging/                    # Structured logging with trace context
│   │   ├── logger.go               # Logger interface and zerolog implementation
│   │   ├── fields.go               # Standard field constants (TraceID, SpanID, etc.)
│   │   ├── middleware.go           # HTTP/gRPC logging middleware
│   │   ├── context.go              # Logger context propagation helpers
│   │   └── logging_test.go         # Unit tests for logger and middleware
│   │
│   ├── metrics/                    # Prometheus metrics collection
│   │   ├── metrics.go              # Init(), registry, HTTP handler
│   │   ├── collectors.go           # NewCounter, NewGauge, NewHistogram constructors
│   │   ├── standard.go             # Standard HTTP/gRPC metrics definitions
│   │   ├── middleware.go           # HTTP/gRPC metrics middleware
│   │   └── metrics_test.go         # Unit tests for metric collection
│   │
│   ├── tracing/                    # OpenTelemetry distributed tracing
│   │   ├── tracer.go               # Tracer initialization and configuration
│   │   ├── span.go                 # StartSpan helpers and span utilities
│   │   ├── middleware.go           # HTTP/gRPC tracing middleware
│   │   ├── propagation.go          # Trace context injection/extraction
│   │   └── tracing_test.go         # Unit tests for tracing
│   │
│   ├── bus/                        # Event bus with Kafka and in-memory backends
│   │   ├── bus.go                  # EventBus interface: Publish, Subscribe
│   │   ├── kafka.go                # KafkaEventBus implementation
│   │   ├── memory.go               # MemoryEventBus implementation (testing)
│   │   ├── middleware.go           # Subscriber middleware: retry, logging, metrics
│   │   ├── topics.go               # Topic naming convention helpers
│   │   └── bus_test.go             # Unit tests for event bus and middleware
│   │
│   ├── database/                   # PostgreSQL connection pooling and transactions
│   │   ├── database.go             # Database interface: Query, QueryRow, Exec
│   │   ├── pool.go                 # Connection pool creation and management (pgxpool)
│   │   ├── transaction.go          # Transaction helpers: Begin, Commit, Rollback, WithTransaction
│   │   ├── health.go               # Database health checker
│   │   └── database_test.go        # Unit tests for database operations
│   │
│   ├── cache/                      # Redis caching with protobuf serialization
│   │   ├── cache.go                # Cache interface: Get, Set, Delete, Exists
│   │   ├── redis.go                # Redis client implementation
│   │   ├── key.go                  # Cache key builder with consistent naming
│   │   ├── aside.go                # Cache-aside pattern helpers (GetOrLoad)
│   │   ├── health.go               # Redis health checker
│   │   └── cache_test.go           # Unit tests for cache operations
│   │
│   ├── auth/                       # Authentication middleware and context
│   │   ├── auth.go                 # AuthContext type and extraction utilities
│   │   ├── apikey.go               # API key validation middleware (HTTP/gRPC)
│   │   ├── jwt.go                  # JWT validation middleware (HTTP/gRPC)
│   │   ├── context.go              # Auth context propagation via context.Context
│   │   └── auth_test.go            # Unit tests for auth middleware
│   │
│   ├── retry/                      # Retry logic with exponential backoff
│   │   ├── retry.go                # Retry() function with configurable policy
│   │   ├── backoff.go              # Exponential backoff with jitter calculation
│   │   ├── policy.go               # Retry policies: RetryTemporary, RetryAll, RetryNone
│   │   └── retry_test.go           # Unit tests for retry logic
│   │
│   ├── errors/                     # Standard error types and mapping
│   │   ├── errors.go               # Error type definitions: Permanent, Temporary, etc.
│   │   ├── wrapping.go             # Error wrapping helpers preserving type
│   │   ├── types.go                # Type checking: IsPermanent, IsTemporary, etc.
│   │   ├── middleware.go           # Recovery middleware with error mapping
│   │   ├── http.go                 # HTTP status code mapping
│   │   ├── grpc.go                 # gRPC status code mapping
│   │   └── errors_test.go          # Unit tests for error types and mapping
│   │
│   └── health/                     # Health check framework
│       ├── health.go               # Health struct with checker registration
│       ├── checker.go              # Checker interface and component implementations
│       ├── handlers.go             # HTTP handlers for /health/live and /health/ready
│       └── health_test.go          # Unit tests for health checks
│
├── examples/                       # Example service demonstrating CQI usage
│   ├── simple/                     # Minimal example
│   │   ├── main.go                 # Simple service using config, logging, database
│   │   └── config.yaml             # Example configuration file
│   └── full/                       # Full-featured example
│       ├── main.go                 # Service using all CQI packages
│       ├── config.yaml             # Full configuration example
│       └── README.md               # Example documentation
│
├── test/                           # Integration tests
│   ├── integration/
│   │   ├── database_test.go        # Database integration tests (real Postgres)
│   │   ├── cache_test.go           # Cache integration tests (real Redis)
│   │   ├── bus_test.go             # Event bus integration tests (real Kafka)
│   │   └── docker-compose.yml      # Test infrastructure (Postgres, Redis, Kafka)
│   └── testdata/
│       ├── configs/                # Test configuration files
│       └── fixtures/               # Test data fixtures
│
└── internal/                       # Private implementation details (optional)
    └── common/                     # Shared utilities not part of public API
        └── testutil/               # Test helpers
            └── containers.go       # Docker container setup for integration tests
```

## Integration Patterns

### MVP Usage Pattern: Complete Service Setup

**Goal:** Service developer imports cqi and has fully instrumented service in <30 minutes.

#### Step 1: Add CQI Dependency (2 minutes)

```bash
# In service repository (e.g., cqpm)
go get github.com/Combine-Capital/cqi@latest
go get github.com/Combine-Capital/cqc@latest
```

#### Step 2: Create Configuration File (5 minutes)

```yaml
# config.yaml
service_name: "cqpm"

database:
  host: "${CQI_DATABASE_HOST:localhost}"
  port: 5432
  database: "cqpm"
  user: "${CQI_DATABASE_USER:postgres}"
  password: "${CQI_DATABASE_PASSWORD}"
  max_connections: 25
  connection_timeout: 5s

cache:
  host: "${CQI_CACHE_HOST:localhost}"
  port: 6379
  password: "${CQI_CACHE_PASSWORD}"
  database: 0

event_bus:
  backend: "kafka"  # or "memory" for testing
  brokers:
    - "${CQI_KAFKA_BROKER:localhost:9092}"
  consumer_group: "cqpm"

logging:
  level: "${LOG_LEVEL:info}"
  format: "json"

metrics:
  port: 9090
  path: "/metrics"

tracing:
  endpoint: "${CQI_TRACING_ENDPOINT:localhost:4317}"
  sample_rate: 0.1
```

#### Step 3: Initialize Infrastructure (10 minutes)

```go
// cmd/server/main.go
package main

import (
    "context"
    "log"
    "net/http"
    
    "github.com/Combine-Capital/cqi/pkg/config"
    "github.com/Combine-Capital/cqi/pkg/logging"
    "github.com/Combine-Capital/cqi/pkg/database"
    "github.com/Combine-Capital/cqi/pkg/cache"
    "github.com/Combine-Capital/cqi/pkg/bus"
    "github.com/Combine-Capital/cqi/pkg/metrics"
    "github.com/Combine-Capital/cqi/pkg/tracing"
    "github.com/Combine-Capital/cqi/pkg/health"
)

func main() {
    // Load config (10-20 lines total for full infrastructure)
    cfg := config.MustLoad("config.yaml", "CQI")
    
    // Initialize logger
    logger := logging.New(cfg.Log)
    logger.Info("Starting cqpm service")
    
    // Initialize tracer
    tracer, err := tracing.New(cfg.Tracing, cfg.ServiceName)
    if err != nil {
        log.Fatal(err)
    }
    defer tracer.Shutdown(context.Background())
    
    // Initialize metrics
    metrics.Init(cfg.Metrics, cfg.ServiceName)
    
    // Initialize database
    db, err := database.NewPool(context.Background(), cfg.Database)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Initialize cache
    cache := cache.NewRedis(cfg.Cache)
    defer cache.Close()
    
    // Initialize event bus
    eventBus, err := bus.New(cfg.EventBus)
    if err != nil {
        log.Fatal(err)
    }
    defer eventBus.Close()
    
    // Setup health checks
    health := health.New(
        health.WithDatabaseCheck(db),
        health.WithCacheCheck(cache),
        health.WithEventBusCheck(eventBus),
    )
    
    // Create HTTP server with health endpoints
    mux := http.NewServeMux()
    mux.Handle("/health/live", health.LivenessHandler())
    mux.Handle("/health/ready", health.ReadinessHandler())
    
    // Add business endpoints here...
    
    logger.Info("Server listening on :8080")
    if err := http.ListenAndServe(":8080", mux); err != nil {
        log.Fatal(err)
    }
}
```

#### Step 4: Use Infrastructure in Business Logic (13 minutes)

```go
// internal/service/position.go
package service

import (
    "context"
    "github.com/Combine-Capital/cqi/pkg/database"
    "github.com/Combine-Capital/cqi/pkg/cache"
    "github.com/Combine-Capital/cqi/pkg/bus"
    "github.com/Combine-Capital/cqi/pkg/logging"
    "github.com/Combine-Capital/cqc/gen/go/cqc/events/v1"
)

type PositionService struct {
    db     *database.Pool
    cache  cache.Cache
    bus    bus.EventBus
    logger logging.Logger
}

func (s *PositionService) GetPosition(ctx context.Context, id string) (*Position, error) {
    // Automatic tracing, logging, metrics from context
    
    // Try cache first
    var pos Position
    key := cache.Key("position", id)
    if err := s.cache.Get(ctx, key, &pos); err == nil {
        return &pos, nil
    }
    
    // Query database
    err := s.db.QueryRow(ctx, 
        "SELECT * FROM positions WHERE id = $1", id).Scan(&pos)
    if err != nil {
        return nil, err
    }
    
    // Populate cache
    s.cache.Set(ctx, key, &pos, 5*time.Minute)
    
    return &pos, nil
}

func (s *PositionService) UpdatePosition(ctx context.Context, pos *Position) error {
    // Update in transaction
    err := s.db.WithTransaction(ctx, func(ctx context.Context, tx database.Tx) error {
        // Update position
        _, err := tx.Exec(ctx, "UPDATE positions SET quantity = $1 WHERE id = $2", 
            pos.Quantity, pos.ID)
        if err != nil {
            return err
        }
        
        // Invalidate cache
        s.cache.Delete(ctx, cache.Key("position", pos.ID))
        
        return nil
    })
    if err != nil {
        return err
    }
    
    // Publish event
    event := &events.PositionChanged{
        PortfolioId: pos.PortfolioID,
        AssetId: pos.AssetID,
        NewQuantity: pos.Quantity,
    }
    
    return s.bus.Publish(ctx, "cqc.events.v1.position_changed", event)
}
```

### Post-MVP Extensions

**Circuit Breaker (Post-MVP):**
```go
import "github.com/Combine-Capital/cqi/pkg/circuitbreaker"

breaker := circuitbreaker.New(circuitbreaker.Config{
    FailureThreshold: 5,
    Timeout: 30 * time.Second,
})

err := breaker.Execute(func() error {
    return externalAPI.Call()
})
```

**Rate Limiting (Post-MVP):**
```go
import "github.com/Combine-Capital/cqi/pkg/ratelimit"

limiter := ratelimit.New(ratelimit.Config{
    RequestsPerSecond: 100,
    Burst: 10,
})

handler := limiter.HTTPMiddleware(businessHandler)
```

**Secret Management (Post-MVP):**
```go
import "github.com/Combine-Capital/cqi/pkg/secrets"

secretMgr := secrets.NewVault(secrets.VaultConfig{
    Address: "https://vault.example.com",
    Token: os.Getenv("VAULT_TOKEN"),
})

dbPassword, err := secretMgr.Get(ctx, "database/cqpm/password")
```

**Service Discovery (Post-MVP):**
```go
import "github.com/Combine-Capital/cqi/pkg/discovery"

disco := discovery.NewConsul(discovery.ConsulConfig{
    Address: "localhost:8500",
})

endpoints, err := disco.Resolve(ctx, "cqmd")
```

## Testing Strategy

### Unit Tests (pkg/*_test.go)
- Each package has comprehensive unit tests
- Mock external dependencies (database, Redis, Kafka)
- Test error handling, edge cases, configuration validation
- Target: >90% code coverage

### Integration Tests (test/integration/)
- Real infrastructure via Docker Compose (Postgres, Redis, Kafka)
- Test actual database queries, cache operations, event publishing
- Verify health checks work with real components
- Run in CI pipeline before merge

### Example Tests (examples/)
- Working examples serve as integration tests
- Verify library can be imported and used correctly
- Document best practices and common patterns

### Performance Tests (optional for MVP, recommended Post-MVP)
- Benchmark logging, metrics, tracing overhead
- Verify <1ms overhead for observability
- Load test event bus throughput
- Cache operation latency testing
