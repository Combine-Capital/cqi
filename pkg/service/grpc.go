package service

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// GRPCService implements the Service interface for gRPC servers.
// It manages the lifecycle of a gRPC server with graceful shutdown.
type GRPCService struct {
	name             string
	addr             string
	server           *grpc.Server
	registerFunc     func(*grpc.Server)
	shutdownTimeout  time.Duration
	enableReflection bool
	mu               sync.Mutex
	started          bool
	listener         net.Listener
}

// GRPCServiceOption is a functional option for configuring a GRPCService.
type GRPCServiceOption func(*GRPCService)

// WithGRPCShutdownTimeout sets the graceful shutdown timeout for the gRPC server.
func WithGRPCShutdownTimeout(timeout time.Duration) GRPCServiceOption {
	return func(s *GRPCService) {
		s.shutdownTimeout = timeout
	}
}

// WithReflection enables gRPC reflection for the server.
// This allows tools like grpcurl to introspect the service.
func WithReflection(enable bool) GRPCServiceOption {
	return func(s *GRPCService) {
		s.enableReflection = enable
	}
}

// NewGRPCService creates a new gRPC service with the provided configuration.
// The registerFunc is called with the gRPC server to register service implementations.
//
// Example:
//
//	svc := service.NewGRPCService("grpc-server", ":9090",
//	    func(s *grpc.Server) {
//	        pb.RegisterMyServiceServer(s, &myServiceImpl{})
//	    },
//	    service.WithReflection(true),
//	    service.WithGRPCShutdownTimeout(30*time.Second),
//	)
func NewGRPCService(name, addr string, registerFunc func(*grpc.Server), opts ...GRPCServiceOption) *GRPCService {
	s := &GRPCService{
		name:             name,
		addr:             addr,
		registerFunc:     registerFunc,
		shutdownTimeout:  30 * time.Second,
		enableReflection: false,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// NewGRPCServiceWithServer creates a new gRPC service using an existing gRPC server.
// This is useful when you need to configure the gRPC server with specific interceptors
// or other options before creating the service.
//
// Example:
//
//	grpcServer := grpc.NewServer(
//	    grpc.UnaryInterceptor(myInterceptor),
//	)
//	svc := service.NewGRPCServiceWithServer("grpc-server", ":9090", grpcServer,
//	    service.WithGRPCShutdownTimeout(30*time.Second),
//	)
func NewGRPCServiceWithServer(name, addr string, server *grpc.Server, opts ...GRPCServiceOption) *GRPCService {
	s := &GRPCService{
		name:            name,
		addr:            addr,
		server:          server,
		shutdownTimeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start starts the gRPC server and blocks until the server is listening.
// It returns an error if the server fails to start.
func (s *GRPCService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("service %s already started", s.name)
	}

	// Create listener
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}
	s.listener = listener

	// Create server if not provided
	if s.server == nil {
		s.server = grpc.NewServer()
	}

	// Register services if registerFunc provided
	if s.registerFunc != nil {
		s.registerFunc(s.server)
	}

	// Enable reflection if requested
	if s.enableReflection {
		reflection.Register(s.server)
	}

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.Serve(listener); err != nil {
			errChan <- err
		}
	}()

	// Wait a moment to check for immediate startup errors
	select {
	case err := <-errChan:
		return fmt.Errorf("failed to start gRPC service %s: %w", s.name, err)
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
		s.started = true
		return nil
	case <-ctx.Done():
		s.server.Stop()
		return ctx.Err()
	}
}

// Stop gracefully stops the gRPC server, waiting for in-flight RPCs to complete.
// It respects the context deadline for shutdown timeout.
func (s *GRPCService) Stop(ctx context.Context) error {
	s.mu.Lock()
	server := s.server
	started := s.started
	s.mu.Unlock()

	if !started || server == nil {
		return nil
	}

	// Use configured shutdown timeout if context has no deadline
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.shutdownTimeout)
		defer cancel()
	}

	// Attempt graceful stop with timeout
	stopped := make(chan struct{})
	go func() {
		server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		// Graceful stop completed
	case <-ctx.Done():
		// Timeout exceeded, force stop
		server.Stop()
		return fmt.Errorf("failed to gracefully stop gRPC service %s: %w", s.name, ctx.Err())
	}

	s.mu.Lock()
	s.started = false
	s.mu.Unlock()

	return nil
}

// Name returns the service name.
func (s *GRPCService) Name() string {
	return s.name
}

// Health checks if the gRPC server is running.
// Returns nil if healthy, error if not running.
func (s *GRPCService) Health() error {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()

	if !started {
		return fmt.Errorf("service %s not running", s.name)
	}

	return nil
}

// Server returns the underlying gRPC server.
// This is useful for testing or advanced configuration.
func (s *GRPCService) Server() *grpc.Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.server
}
