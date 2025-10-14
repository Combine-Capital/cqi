# Integration Tests

This directory contains integration tests for CQI infrastructure components that require real external services (PostgreSQL, Redis, NATS JetStream).

## Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ installed

## Running Integration Tests

### 1. Start Infrastructure Services

```bash
cd test/integration
docker compose up -d
```

This will start:
- **PostgreSQL 14** on port 5432 (user: postgres, password: postgres, database: cqi_test)
- **Redis 7** on port 6379 (no password)
- **NATS JetStream** on port 4222 (with JetStream enabled)
- **Jaeger** on port 4318 (OTLP endpoint) and 16686 (UI)

### 2. Wait for Services to be Ready

Wait a few seconds for all services to be healthy. You can check status with:

```bash
docker compose ps
```

All services should show "healthy" status.

### 3. Run Integration Tests

From the project root:

```bash
go test -v ./test/integration/...
```

Or from this directory:

```bash
cd test/integration
go test -v
```

### 4. Stop Infrastructure Services

When done testing:

```bash
docker compose down
```

To also remove volumes:

```bash
docker compose down -v
```

## Test Coverage

### Database Tests (`database_test.go`)

- **TestDatabaseConnection**: Basic connectivity and ping
- **TestDatabaseQuery**: Query operations with multiple rows
- **TestDatabaseExec**: INSERT, UPDATE, DELETE operations
- **TestDatabaseTransaction**: Transaction commit and rollback
- **TestDatabaseContextCancellation**: Context timeout handling
- **TestDatabaseHealthCheck**: Health check functionality

### Cache Tests (`cache_test.go`)

- **TestCacheConnection**: Basic Redis connectivity
- **TestCacheSetGet**: Set and Get operations with protobuf serialization
- **TestCacheDelete**: Delete operations
- **TestCacheExists**: Existence checking
- **TestCacheTTL**: TTL expiration behavior

### Event Bus Tests (`bus_test.go`)

- **TestMemoryBusPublishSubscribe**: In-memory event bus pub/sub
- **TestMemoryBusMultipleSubscribers**: Multiple subscribers to same topic
- **TestBusWithLogging**: Event bus with logging middleware
- **TestBusWithMetrics**: Event bus with metrics middleware
- **TestJetStreamBusPublishSubscribe**: NATS JetStream pub/sub (requires NATS)

## Configuration

Tests use configuration from `test/testdata/configs/test_config.yaml` which points to:
- Database: localhost:5432
- Cache: localhost:6379
- NATS: nats://localhost:4222

You can override these with environment variables:
```bash
export CQI_DATABASE_HOST=custom-host
export CQI_DATABASE_PASSWORD=custom-password
go test -v ./test/integration/...
```

## Troubleshooting

### Tests Fail with Connection Errors

- Ensure all Docker services are running: `docker compose ps`
- Check service logs: `docker compose logs <service-name>`
- Verify ports are not in use by other applications
- Try restarting services: `docker compose restart`

### Port Conflicts

If default ports are in use, edit `docker-compose.yml`:
```yaml
services:
  postgres:
    ports:
      - "15432:5432"  # Use custom port
```

And update test configuration accordingly.

### JetStream Tests Failing

JetStream tests will be skipped if NATS is not available. To run them:
1. Ensure NATS container is running
2. Check NATS logs: `docker compose logs nats`
3. Verify JetStream is enabled: `curl http://localhost:8222/jsz`

### Slow Tests

Integration tests may be slower than unit tests because they interact with real services. This is expected and normal.

To run only specific tests:
```bash
go test -v -run TestDatabaseConnection ./test/integration/
```

## CI/CD Integration

For CI/CD pipelines, use the docker-compose.yml file to start services before running tests:

```yaml
# Example GitHub Actions workflow
- name: Start infrastructure
  run: docker compose -f test/integration/docker-compose.yml up -d

- name: Wait for services
  run: sleep 10

- name: Run integration tests
  run: go test -v ./test/integration/...

- name: Stop infrastructure
  run: docker compose -f test/integration/docker-compose.yml down
```

## Development Tips

- Use `t.Skip()` to skip tests that require unavailable services
- Use `t.Helper()` in setup functions to improve error messages
- Clean up test data in `defer` statements
- Use unique table/key names to avoid conflicts between concurrent tests
- Always use `context.Context` with appropriate timeouts
