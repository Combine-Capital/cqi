# CQI - Crypto Quant Infrastructure

Part of the **Crypto Quant Platform** - Professional-grade crypto trading infrastructure.

## Overview

CQI is a shared infrastructure library for Go services in the Crypto Quant platform. It provides production-ready components for event bus, database, cache, observability (logging, metrics, tracing), configuration management, authentication, and error handling with native [CQC](https://github.com/Combine-Capital/cqc) protobuf integration.

**Key Features:**
- **Service Lifecycle**: HTTP/gRPC server abstractions with graceful shutdown and signal handling
- **Service Orchestration**: Multi-service runner with dependency management and automatic restart
- **Service Discovery**: Registry for dynamic service registration with local and Redis backends
- **Event Bus**: NATS JetStream & in-memory backends with automatic protobuf serialization
- **Data Storage**: PostgreSQL connection pooling with transaction helpers
- **Caching**: Redis client with native protobuf serialization
- **Observability**: Structured logging (zerolog), Prometheus metrics, OpenTelemetry tracing
- **Configuration**: Environment variables + YAML/JSON with validation
- **Authentication**: API key & JWT middleware for HTTP/gRPC
- **Reliability**: Exponential backoff retry, typed errors, panic recovery, health checks

## Installation

```bash
go get github.com/Combine-Capital/cqi@latest
```

## Quick Start

```go
package main

import (
    "context"
    "net/http"
    
    "github.com/Combine-Capital/cqi/pkg/config"
    "github.com/Combine-Capital/cqi/pkg/service"
    "github.com/Combine-Capital/cqi/pkg/runner"
)

func main() {
    ctx := context.Background()
    
    // Load configuration
    cfg := config.MustLoad("config.yaml", "MYSERVICE")
    
    // Create HTTP service with handler
    httpSvc := service.NewHTTPService("api", ":8080", http.HandlerFunc(handler))
    
    // Create runner and add services with dependencies
    r := runner.New("my-app")
    r.Add(httpSvc, runner.WithRestartPolicy(runner.RestartOnFailure))
    
    // Start all services
    if err := r.Start(ctx); err != nil {
        panic(err)
    }
    
    // Wait for shutdown signal
    service.WaitForShutdown(ctx, r)
}

func handler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Hello from CQI!"))
}
```

See [examples/](./examples/) for complete working examples with database, cache, and event bus integration.

## Repository Structure

```
cqi/
├── cmd/           # Application entrypoints
├── internal/      # Private application code
├── pkg/           # Public libraries
│   ├── auth/      # API key & JWT authentication
│   ├── bus/       # Event bus (NATS JetStream, in-memory)
│   ├── cache/     # Redis cache client
│   ├── config/    # Configuration loading
│   ├── database/  # PostgreSQL connection pool
│   ├── errors/    # Error types and handling
│   ├── health/    # Health check framework
│   ├── logging/   # Structured logging
│   ├── metrics/   # Prometheus metrics
│   ├── registry/  # Service discovery and registration
│   ├── retry/     # Retry with backoff
│   ├── runner/    # Multi-service orchestration
│   ├── service/   # Service lifecycle management
│   └── tracing/   # OpenTelemetry tracing
├── examples/      # Working code examples
│   ├── simple/    # Minimal usage example
│   └── full/      # Complete integration example
├── test/          # Integration tests
│   ├── integration/  # Integration test suites
│   └── testdata/     # Test fixtures
└── docs/          # Documentation
```

## Architecture

CQI provides a layered architecture for building microservices:

### Service Layer
- **service**: HTTP/gRPC server lifecycle with graceful shutdown
- **runner**: Orchestrate multiple services with dependency resolution
- **registry**: Dynamic service discovery (local/Redis backends)

### Infrastructure Layer
- **database**: PostgreSQL connection pooling with transactions
- **cache**: Redis client with protobuf serialization
- **bus**: Event bus (NATS JetStream) for async messaging

### Observability Layer
- **logging**: Structured logging with trace context
- **metrics**: Prometheus metrics collection
- **tracing**: OpenTelemetry distributed tracing
- **health**: Liveness and readiness health checks

### Foundation Layer
- **config**: Configuration management (env + files)
- **auth**: Authentication middleware (API key + JWT)
- **errors**: Typed errors with classification
- **retry**: Exponential backoff retry logic

## Configuration

CQI uses environment variables with prefix support and YAML/JSON configuration files:

```yaml
# config.yaml
database:
  host: localhost
  port: 5432
  database: mydb
  user: user
  password: ${DB_PASSWORD}  # From environment
  max_connections: 25

cache:
  address: localhost:6379
  password: ${REDIS_PASSWORD}
  db: 0

log:
  level: info
  format: json

metrics:
  enabled: true
  port: 9090
  path: /metrics
```

Environment variables override config file values using the prefix:
```bash
export MYSERVICE_DATABASE_HOST=prod-db.example.com
export MYSERVICE_LOG_LEVEL=debug
```

## Documentation

- [Project Brief](./docs/BRIEF.md) - Vision and requirements
- [Technical Specification](./docs/SPEC.md) - Architecture and design
- [Implementation Roadmap](./docs/ROADMAP.md) - Development plan

## Related Services

- [CQ Hub](https://github.com/Combine-Capital/cqhub) - Platform Documentation
- [CQC](https://github.com/Combine-Capital/cqc) - Platform Contracts

## License

MIT License - see [LICENSE](./LICENSE) for details
