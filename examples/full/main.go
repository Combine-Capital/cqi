// Full example demonstrating comprehensive CQI usage with all infrastructure packages.
//
// This example shows:
//   - Configuration management with YAML and environment variables
//   - Structured logging with zerolog
//   - Prometheus metrics collection
//   - OpenTelemetry distributed tracing
//   - PostgreSQL database operations with transactions
//   - Redis caching with protobuf serialization
//   - NATS JetStream event bus for pub/sub messaging
//   - Health check endpoints (liveness and readiness)
//
// Prerequisites:
//  1. Start infrastructure (from test/integration directory):
//     docker-compose up -d
//  2. Set environment variables:
//     export FULL_DATABASE_PASSWORD=postgres
//     export FULL_CACHE_PASSWORD=""
//  3. Run the service:
//     go run main.go
//  4. Access endpoints:
//     - Metrics: http://localhost:9090/metrics
//     - Health: http://localhost:8080/health/live
//     - Health: http://localhost:8080/health/ready
//     - API: http://localhost:8080/api/example
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Combine-Capital/cqi/pkg/bus"
	"github.com/Combine-Capital/cqi/pkg/cache"
	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/database"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/Combine-Capital/cqi/pkg/health"
	"github.com/Combine-Capital/cqi/pkg/logging"
	"github.com/Combine-Capital/cqi/pkg/metrics"
	"github.com/Combine-Capital/cqi/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	testproto "github.com/Combine-Capital/cqi/pkg/cache/testproto"
)

// Service encapsulates all infrastructure components
type Service struct {
	config  *config.Config
	logger  *logging.Logger
	db      *database.Pool
	cache   *cache.RedisCache
	bus     bus.EventBus
	health  *health.Health
	tracer  trace.Tracer
	cleanup func()
}

func main() {
	ctx := context.Background()

	// Load configuration from config.yaml and environment variables with FULL_ prefix
	cfg := config.MustLoad("config.yaml", "FULL")

	// Initialize structured logger
	logger := logging.New(cfg.Log)
	logger.Info().Msg("Full service starting")

	// Initialize OpenTelemetry tracer
	tracerProvider, shutdownTracer, err := tracing.NewTracerProvider(ctx, cfg.Tracing, "cqi-full-example")
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize tracer")
		log.Fatal(err)
	}
	defer func() {
		if err := shutdownTracer(ctx); err != nil {
			logger.Error().Err(err).Msg("Failed to shutdown tracer")
		}
	}()
	tracer := tracerProvider.Tracer("cqi-full-example")

	// Initialize Prometheus metrics
	metricsConfig := metrics.MetricsConfig{
		Enabled:   true,
		Port:      cfg.Metrics.Port,
		Path:      cfg.Metrics.Path,
		Namespace: "cqi_full_example",
	}
	if err := metrics.Init(metricsConfig); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize metrics")
		log.Fatal(err)
	}
	logger.Info().Int("port", cfg.Metrics.Port).Msg("Metrics server started")

	// Initialize database connection pool
	db, err := database.NewPool(ctx, cfg.Database)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize database")
		log.Fatal(err)
	}
	defer db.Close()
	logger.Info().Msg("Database connected")

	// Initialize Redis cache
	redisCache, err := cache.NewRedis(ctx, cfg.Cache)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize cache")
		log.Fatal(err)
	}
	defer redisCache.Close()
	logger.Info().Msg("Cache connected")

	// Initialize event bus (use in-memory for demo, JetStream for production)
	var eventBus bus.EventBus
	if cfg.EventBus.Backend == "jetstream" {
		eventBus, err = bus.NewJetStream(ctx, cfg.EventBus)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to initialize JetStream event bus")
			log.Fatal(err)
		}
		logger.Info().Msg("JetStream event bus connected")
	} else {
		eventBus = bus.NewMemory()
		logger.Info().Msg("In-memory event bus initialized")
	}
	defer eventBus.Close()

	// Initialize health checks
	healthChecker := health.New()
	healthChecker.RegisterChecker("database", health.CheckerFunc(func(ctx context.Context) error {
		return database.CheckHealth(ctx, db)
	}))
	healthChecker.RegisterChecker("cache", health.CheckerFunc(func(ctx context.Context) error {
		return redisCache.CheckHealth(ctx)
	}))
	logger.Info().Msg("Health checks registered")

	// Create service instance
	svc := &Service{
		config: cfg,
		logger: logger,
		db:     db,
		cache:  redisCache,
		bus:    eventBus,
		health: healthChecker,
		tracer: tracer,
	}

	// Initialize database schema
	if err := svc.initializeSchema(ctx); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize schema")
		log.Fatal(err)
	}

	// Start event subscribers
	if err := svc.startEventHandlers(ctx); err != nil {
		logger.Error().Err(err).Msg("Failed to start event handlers")
		log.Fatal(err)
	}

	// Setup HTTP server with middleware
	mux := svc.setupRouter()

	// Start HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info().Int("port", cfg.Server.HTTPPort).Msg("HTTP server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP server error")
			log.Fatal(err)
		}
	}()

	// Demonstrate infrastructure usage
	if err := svc.demonstrateUsage(ctx); err != nil {
		logger.Error().Err(err).Msg("Demo failed")
	}

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down gracefully...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}

	logger.Info().Msg("Service stopped")
}

// initializeSchema creates necessary database tables
func (s *Service) initializeSchema(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS example_records (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			value INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		return errors.Wrap(err, "failed to create example_records table")
	}

	s.logger.Info().Msg("Database schema initialized")
	return nil
}

// startEventHandlers subscribes to event topics
func (s *Service) startEventHandlers(ctx context.Context) error {
	// Subscribe to test events with middleware
	err := s.bus.Subscribe(ctx, "cqi.example.test_event",
		bus.HandlerFunc(func(ctx context.Context, msg proto.Message) error {
			event := msg.(*testproto.TestMessage)
			s.logger.Info().
				Str("id", event.Id).
				Str("name", event.Name).
				Msg("Received test event")
			return nil
		}),
		bus.WithLogging(s.logger),
		bus.WithMetrics(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to test events")
	}

	s.logger.Info().Msg("Event handlers started")
	return nil
}

// setupRouter configures HTTP routes with middleware
func (s *Service) setupRouter() *http.ServeMux {
	mux := http.NewServeMux()

	// Health endpoints (no auth required)
	mux.HandleFunc("/health/live", s.health.LivenessHandler())
	mux.HandleFunc("/health/ready", s.health.ReadinessHandler())

	// API endpoints
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/example", s.handleExample)

	// Apply middleware chain: recovery -> tracing -> metrics -> logging
	handler := errors.RecoveryMiddleware(func(v interface{}) error {
		s.logger.Error().Interface("panic", v).Msg("Panic recovered")
		return nil
	})(
		tracing.HTTPMiddleware("cqi_full_example")(
			metrics.HTTPMiddleware("cqi_full_example")(
				logging.HTTPMiddleware(s.logger)(apiMux),
			),
		),
	)

	// Mount API handler
	mux.Handle("/api/", handler)

	return mux
}

// handleExample is an example HTTP handler demonstrating trace context and logging
func (s *Service) handleExample(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Start a span for this operation
	ctx, span := s.tracer.Start(ctx, "handleExample")
	defer span.End()

	// Extract logger with trace context
	logger := logging.FromContext(ctx)
	if logger == nil {
		logger = s.logger
	}
	logger.Info().Msg("Processing example request")

	// Example: Query database
	var count int
	err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM example_records").Scan(&count)
	if err != nil {
		logger.Error().Err(err).Msg("Database query failed")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Example: Cache operation
	cacheKey := cache.Key("example", "count")
	testMsg := &testproto.TestMessage{
		Id:    "cache-test",
		Name:  fmt.Sprintf("Count: %d", count),
		Value: int64(count),
	}
	if err := s.cache.Set(ctx, cacheKey, testMsg, 5*time.Minute); err != nil {
		logger.Warn().Err(err).Msg("Cache set failed")
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","record_count":%d}`, count)
}

// demonstrateUsage shows examples of using each infrastructure component
func (s *Service) demonstrateUsage(ctx context.Context) error {
	s.logger.Info().Msg("Starting infrastructure demonstration")

	// 1. Database: Insert and query
	s.logger.Info().Msg("Demo 1: Database operations")
	var recordID int
	err := s.db.QueryRow(ctx,
		"INSERT INTO example_records (name, value) VALUES ($1, $2) RETURNING id",
		"demo-record",
		42,
	).Scan(&recordID)
	if err != nil {
		return errors.Wrap(err, "database insert failed")
	}
	s.logger.Info().Int("record_id", recordID).Msg("Inserted database record")

	// 2. Database: Transaction
	s.logger.Info().Msg("Demo 2: Database transaction")
	err = database.WithTransaction(ctx, s.db, func(tx database.Transaction) error {
		_, err := tx.Exec(ctx,
			"INSERT INTO example_records (name, value) VALUES ($1, $2), ($3, $4)",
			"tx-record-1", 100,
			"tx-record-2", 200,
		)
		return err
	})
	if err != nil {
		return errors.Wrap(err, "transaction failed")
	}
	s.logger.Info().Msg("Transaction completed")

	// 3. Cache: Set and Get
	s.logger.Info().Msg("Demo 3: Cache operations")
	testMsg := &testproto.TestMessage{
		Id:    "cache-demo",
		Name:  "Demo Message",
		Value: 123,
	}
	cacheKey := cache.Key("demo", testMsg.Id)
	if err := s.cache.Set(ctx, cacheKey, testMsg, 10*time.Minute); err != nil {
		return errors.Wrap(err, "cache set failed")
	}

	var retrieved testproto.TestMessage
	if err := s.cache.Get(ctx, cacheKey, &retrieved); err != nil {
		return errors.Wrap(err, "cache get failed")
	}
	s.logger.Info().
		Str("id", retrieved.Id).
		Str("name", retrieved.Name).
		Int64("value", retrieved.Value).
		Msg("Retrieved from cache")

	// 4. Event Bus: Publish
	s.logger.Info().Msg("Demo 4: Event bus publish")
	eventMsg := &testproto.TestMessage{
		Id:    "event-demo",
		Name:  "Event Message",
		Value: 456,
	}
	if err := s.bus.Publish(ctx, "cqi.example.test_event", eventMsg); err != nil {
		return errors.Wrap(err, "event publish failed")
	}
	s.logger.Info().Msg("Published event")

	// Give subscriber time to process
	time.Sleep(100 * time.Millisecond)

	s.logger.Info().Msg("Infrastructure demonstration completed")
	return nil
}
