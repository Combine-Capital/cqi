# CQI Full Example

This example demonstrates comprehensive usage of all CQI infrastructure packages in a single service.

## Features Demonstrated

- **Configuration Management**: Loading from YAML files and environment variables
- **Structured Logging**: JSON logging with zerolog
- **Metrics Collection**: Prometheus metrics with standard HTTP metrics
- **Distributed Tracing**: OpenTelemetry tracing with OTLP export
- **Database Operations**: PostgreSQL connection pooling, queries, and transactions
- **Caching**: Redis caching with protobuf serialization
- **Event Bus**: NATS JetStream (or in-memory) for pub/sub messaging
- **Health Checks**: Liveness and readiness endpoints
- **HTTP Server**: Production-ready HTTP server with middleware chain

## Prerequisites

### 1. Start Infrastructure Services

From the `test/integration` directory:

```bash
cd ../../test/integration
docker-compose up -d
```

This starts:
- PostgreSQL 14 on port 5432
- Redis 7 on port 6379
- NATS JetStream on port 4222

### 2. Set Environment Variables

```bash
export FULL_DATABASE_PASSWORD=postgres
export FULL_CACHE_PASSWORD=""
```

## Running the Example

```bash
cd examples/full
go run main.go
```

The service will:
1. Initialize all infrastructure components
2. Create a test database schema
3. Start event subscribers
4. Run demonstrations of each component
5. Start an HTTP server on port 8080
6. Serve metrics on port 9090

## Accessing Endpoints

### Health Checks

```bash
# Liveness - returns 200 if service is running
curl http://localhost:8080/health/live

# Readiness - returns 200 if all dependencies are healthy
curl http://localhost:8080/health/ready
```

### Metrics

```bash
# Prometheus metrics
curl http://localhost:9090/metrics
```

### API Endpoint

```bash
# Example API endpoint (demonstrates database + cache + tracing)
curl http://localhost:8080/api/example
```

## Configuration

The example uses `config.yaml` for base configuration and environment variables for secrets:

- `FULL_DATABASE_PASSWORD`: PostgreSQL password
- `FULL_CACHE_PASSWORD`: Redis password (empty string for no auth)

You can override any configuration value with environment variables using the `FULL_` prefix. For example:

```bash
export FULL_LOG_LEVEL=debug
export FULL_SERVER_HTTP_PORT=3000
export FULL_METRICS_PORT=9091
```

## Code Structure

### main.go

- **Service struct**: Encapsulates all infrastructure components
- **main()**: Initializes components and manages lifecycle
- **initializeSchema()**: Creates database tables
- **startEventHandlers()**: Subscribes to event topics
- **setupRouter()**: Configures HTTP routes and middleware
- **handleExample()**: Example HTTP handler
- **demonstrateUsage()**: Demonstrates each infrastructure component

### Middleware Chain

The HTTP middleware is applied in this order:
1. **Recovery**: Catches panics and logs them
2. **Tracing**: Creates spans for distributed tracing
3. **Metrics**: Records request metrics
4. **Logging**: Logs requests with trace context

## Demonstrations

When the service starts, it automatically demonstrates:

1. **Database Insert**: Inserts a record and retrieves the ID
2. **Database Transaction**: Inserts multiple records in a transaction
3. **Cache Operations**: Sets and gets a protobuf message
4. **Event Publishing**: Publishes an event that the subscriber receives

## Graceful Shutdown

Press `Ctrl+C` to trigger graceful shutdown. The service will:
1. Stop accepting new HTTP requests
2. Wait for in-flight requests to complete (30s timeout)
3. Close database connections
4. Close cache connections
5. Close event bus connections
6. Flush tracing spans

## Production Considerations

For production deployment, consider:

- Use JetStream instead of in-memory event bus (`backend: jetstream`)
- Enable HTTPS/TLS for all connections
- Use proper secret management (not environment variables)
- Configure appropriate connection pool sizes
- Set reasonable timeouts for all operations
- Enable security features (authentication, rate limiting)
- Use a proper OTLP collector for traces
- Monitor metrics and set up alerts
- Use separate databases for different environments

## Troubleshooting

### Database Connection Failed

- Ensure PostgreSQL is running: `docker ps | grep postgres`
- Check connection string in config.yaml
- Verify `FULL_DATABASE_PASSWORD` environment variable

### Cache Connection Failed

- Ensure Redis is running: `docker ps | grep redis`
- Check Redis connection settings in config.yaml
- Verify Redis is accessible: `redis-cli ping`

### Event Bus Connection Failed

- Ensure NATS is running: `docker ps | grep nats`
- Try using `backend: memory` for testing without NATS
- Check NATS connection string in config.yaml

### Port Already in Use

- Change ports in config.yaml:
  ```yaml
  server:
    http_port: 3000
  metrics:
    port: 9091
  ```
