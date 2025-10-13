# CQI - Crypto Quant Infrastructure

Part of the **Crypto Quant Platform** - Professional-grade crypto trading infrastructure.

## Overview

CQI is a shared infrastructure library for Go services in the Crypto Quant platform. It provides production-ready components for event bus, database, cache, observability (logging, metrics, tracing), configuration management, authentication, and error handling with native [CQC](https://github.com/Combine-Capital/cqc) protobuf integration.

**Key Features:**
- **Event Bus**: Kafka & in-memory backends with automatic protobuf serialization
- **Data Storage**: PostgreSQL connection pooling with transaction helpers
- **Caching**: Redis client with native protobuf serialization
- **Observability**: Structured logging (zerolog), Prometheus metrics, OpenTelemetry tracing
- **Configuration**: Environment variables + YAML/JSON with validation
- **Authentication**: API key & JWT middleware for HTTP/gRPC
- **Reliability**: Exponential backoff retry, typed errors, panic recovery

## Installation

```bash
go get github.com/Combine-Capital/cqi@latest
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/Combine-Capital/cqi/pkg/config"
    "github.com/Combine-Capital/cqi/pkg/logging"
    "github.com/Combine-Capital/cqi/pkg/database"
    "github.com/Combine-Capital/cqi/pkg/cache"
)

func main() {
    ctx := context.Background()
    
    // Load configuration from environment variables and config file
    cfg := config.MustLoad("config.yaml", "MYSERVICE")
    
    // Initialize structured logger
    logger := logging.New(cfg.Log)
    logger.Info().Msg("Service starting")
    
    // Initialize database connection pool
    db, err := database.NewPool(ctx, cfg.Database)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Initialize cache client
    cache := cache.NewRedis(cfg.Cache)
    defer cache.Close()
    
    // Your application logic here
}
```

## Repository Structure

```
cqi/
├── cmd/           # Application entrypoints
├── internal/      # Private application code
├── pkg/           # Public libraries
│   ├── bus/       # Event bus (Kafka, in-memory)
│   ├── cache/     # Redis cache client
│   ├── config/    # Configuration loading
│   ├── database/  # PostgreSQL connection pool
│   ├── errors/    # Error types and handling
│   ├── health/    # Health check framework
│   ├── logging/   # Structured logging
│   ├── metrics/   # Prometheus metrics
│   ├── retry/     # Retry with backoff
│   └── tracing/   # OpenTelemetry tracing
├── test/          # Integration tests
│   ├── integration/  # Integration test suites
│   └── testdata/     # Test fixtures
└── docs/          # Documentation
```

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
