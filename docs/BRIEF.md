# Project Brief: CQI - Crypto Quant Infrastructure

## Vision
Shared infrastructure library providing event bus, database, cache, logging, metrics, tracing, config, and auth capabilities with native CQC protobuf integration, eliminating infrastructure reimplementation across all CQ platform services.

## User Personas
### Primary User: Service Developer
- **Role:** Developer building CQ platform services
- **Needs:** Event bus, database pooling, cache client, auth middleware, observability (logs/metrics/traces), config management
- **Pain Points:** Reimplementing infrastructure in every service, inconsistent patterns, connection management complexity
- **Success:** Import cqi package, configure with 10-20 lines, have production-ready infrastructure with CQC type integration

### Secondary User: Platform Operator
- **Role:** DevOps engineer operating CQ platform
- **Needs:** Unified observability (consistent logs/metrics/traces), standardized health checks, centralized configuration
- **Pain Points:** Service-specific monitoring patterns, fragmented logging, distributed debugging without trace correlation
- **Success:** Single dashboard for all services, trace requests end-to-end, debug issues in <5 minutes

## Core Requirements

### Event Bus & Messaging
- [MVP] Import CQC event types from github.com/Combine-Capital/cqc/gen/go/cqc/events/v1 (AssetCreated, PriceUpdated, OrderPlaced, PositionChanged, RiskAlert)
- [MVP] Provide event bus interface: Publish(ctx, topic, proto.Message) error and Subscribe(ctx, topic, handler) error
- [MVP] Support in-memory backend (testing) and NATS JetStream backend (production) with automatic protobuf serialization
- [MVP] Provide subscriber middleware for logging, metrics, error handling, and retry
- [MVP] Define topic naming convention: "cqc.events.v1.{event_type}"

### Data Storage & Caching
- [MVP] Provide PostgreSQL connection pool with health checks, auto-reconnection, configurable limits, graceful shutdown
- [MVP] Provide database interface: Query, QueryRow, Exec with context support for cancellation/timeout
- [MVP] Provide transaction helpers: Begin, Commit, Rollback, WithTransaction (auto-rollback on error)
- [MVP] Provide Redis client: Get/Set/Delete/Exists with native CQC protobuf serialization
- [MVP] Provide cache key builder with consistent naming and TTL support
- [MVP] Provide cache-aside helpers for automatic populate-on-miss

### Observability & Monitoring
- [MVP] Define standard log fields: trace_id, span_id, service_name, timestamp, level, message, error
- [MVP] Provide structured logger (zerolog/zap) with context propagation and env-based log level control
- [MVP] Provide Prometheus client: Counter, Gauge, Histogram with naming convention {service}_{subsystem}_{metric}
- [MVP] Provide standard HTTP metrics (request_duration, request_count, request/response_size) and gRPC metrics (call_duration, call_count)
- [MVP] Provide OpenTelemetry tracing with span creation, context propagation, traceparent/tracestate header injection/extraction
- [MVP] Provide HTTP/gRPC middleware: auto-span creation, trace context injection, request/response logging
- [MVP] Provide health check framework: /health/live and /health/ready endpoints with component checkers (database, cache, bus)

### Configuration & Secrets
- [MVP] Define config structs: DatabaseConfig, CacheConfig, EventBusConfig, ServerConfig, LogConfig, MetricsConfig, TracingConfig
- [MVP] Load config from environment variables with prefix support (e.g., CQI_DATABASE_HOST) using viper
- [MVP] Load config from YAML/JSON with env var override
- [MVP] Validate config: required field checks, default values
- [MVP] Load secrets from env vars (document external secret manager for production)

### Security & Authentication
- [MVP] Define auth context types: API key and JWT with user/service identity fields
- [MVP] Provide HTTP/gRPC middleware for API key validation against configured keys
- [MVP] Provide HTTP/gRPC middleware for JWT validation with public key verification and claims extraction
- [MVP] Provide utilities to extract auth from request metadata and propagate via context.Context

### Reliability & Error Handling
- [MVP] Provide retry package with exponential backoff, jitter, configurable max attempts, per-error-type policies
- [MVP] Define error types: Permanent, Temporary, NotFound, InvalidInput, Unauthorized with wrapping support
- [MVP] Provide error middleware: translate errors to HTTP status codes and gRPC status codes
- [MVP] Provide panic recovery middleware for HTTP/gRPC with logging and metrics

### Project Organization
- [MVP] Organize code in pkg/ with subpackages: bus/, database/, cache/, logging/, metrics/, tracing/, config/, auth/, retry/, errors/, health/
- [MVP] Document each package with usage examples
- [MVP] Provide example configs and integration tests with CQC types

### Post-MVP Features
- [Post MVP] Contribute infrastructure protobufs to CQC for platform standardization
- [Post MVP] Circuit breaker with configurable thresholds, timeout, half-open testing
- [Post MVP] Rate limiting (token bucket/sliding window) per client/endpoint
- [Post MVP] Secret management integration (Vault/AWS Secrets Manager) with rotation
- [Post MVP] Service discovery (Consul/Kubernetes)
- [Post MVP] Distributed locks (Redis/database)
- [Post MVP] Graceful degradation for optional dependencies

## Success Metrics
1. **Adoption**: 100% of CQ services (10 total: cqar, cqvx, cqmd, cqex, cqdefi, cqpm, cqrep, cqrisk, cqstrat, cqai) use cqi exclusively for infrastructure
2. **Type Safety**: Zero serialization errors in production over 30-day measurement period
3. **Observability**: Single monitoring dashboard supports all services, new service setup <5 minutes
4. **Development Velocity**: New service fully instrumented (DB, cache, events, logs, metrics, traces) in <30 minutes
5. **Reliability**: >90% test coverage with integration tests against Postgres, Redis, NATS JetStream