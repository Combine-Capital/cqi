# Project Brief: CQI - Crypto Quant Infrastructure

## Vision
A comprehensive infrastructure package that provides all common functionality needed by every service in the Crypto Quant platform including event publishing/subscribing, database connections, caching, authentication, metrics collection, distributed tracing, and configuration management, using the type definitions and contracts from CQC (github.com/Combine-Capital/cqc) to ensure consistent communication patterns while allowing service developers to focus on business logic rather than reimplementing infrastructure patterns.

## User Personas
### Primary User: Service Developer
- **Role:** Developer building any CQ platform service who needs standard infrastructure capabilities
- **Needs:** Event bus client for CQC events, database connection pooling, Redis cache client, authentication middleware, metrics/tracing setup, configuration loader
- **Pain Points:** Reimplementing the same infrastructure code in every service, inconsistent logging formats, different metric naming, managing connections
- **Success:** Imports cqi packages and immediately has production-ready infrastructure that works with CQC types with a few lines of code

### Secondary User: Platform Operator
- **Role:** DevOps engineer responsible for running and monitoring the CQ platform
- **Needs:** Consistent metrics across all services, unified log formats, health check endpoints, distributed tracing, centralized configuration
- **Pain Points:** Each service logs differently, metrics are inconsistent, no standardized health checks, hard to trace requests across services
- **Success:** Can monitor, debug, and operate all services using consistent patterns and tools

## Core Requirements
- [MVP] The system should add github.com/Combine-Capital/cqc/go as a Go module dependency in go.mod (CQC is the contracts repository containing protobuf definitions)
- [MVP] The system should import event message types from github.com/Combine-Capital/cqc/go/events/v1 (such as OrderPlaced, PositionChanged, PriceUpdated, RiskAlert)
- [MVP] The system should provide an event bus client with Publish(topic string, event proto.Message) and Subscribe(topic string) methods that work with CQC's protobuf events
- [MVP] The system should provide event bus implementations for both in-memory (testing) and Kafka (production) backends
- [MVP] The system should import health check types from github.com/Combine-Capital/cqc/go/infrastructure/v1 (HealthCheckResponse, ServiceStatus) for standardized health endpoints
- [MVP] The system should provide database connection pooling with PostgreSQL support including health checks, automatic reconnection, and connection limits
- [MVP] The system should provide a Redis cache client with Get/Set/Delete methods that can serialize and deserialize CQC protobuf messages
- [MVP] The system should import log field definitions from github.com/Combine-Capital/cqc/go/observability/v1 and provide structured JSON logging with trace_id, service_name, and timestamp
- [MVP] The system should import metric types from github.com/Combine-Capital/cqc/go/observability/v1 and provide Prometheus metrics with Counter, Gauge, and Histogram types
- [MVP] The system should provide OpenTelemetry distributed tracing that propagates trace context using CQC's standard trace headers
- [MVP] The system should import config schemas from github.com/Combine-Capital/cqc/go/config/v1 and provide configuration loading from environment variables and YAML files
- [MVP] The system should import auth types from github.com/Combine-Capital/cqc/go/auth/v1 and provide middleware for API key and JWT validation
- [MVP] The system should provide a retry package with exponential backoff for transient failures
- [MVP] The system should organize code in pkg/ directory with subpackages: bus/, database/, cache/, logging/, metrics/, tracing/, config/, auth/, retry/
- [Post MVP] The system should provide circuit breaker pattern implementation for fault tolerance
- [Post MVP] The system should provide rate limiting middleware with configurable limits per client
- [Post MVP] The system should provide secret management integration with HashiCorp Vault or AWS Secrets Manager

## Success Metrics
1. Every CQ service uses cqi packages exclusively for infrastructure concerns without custom implementations
2. All events published through CQI are valid CQC protobuf messages that can be consumed by any service
3. All services produce consistent metrics and logs that conform to CQC's observability standards