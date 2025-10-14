package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// HTTPService implements the Service interface for HTTP servers.
// It manages the lifecycle of an HTTP server with graceful shutdown.
type HTTPService struct {
	name            string
	addr            string
	handler         http.Handler
	server          *http.Server
	readTimeout     time.Duration
	writeTimeout    time.Duration
	shutdownTimeout time.Duration
	maxHeaderBytes  int
	mu              sync.Mutex
	started         bool
}

// HTTPServiceOption is a functional option for configuring an HTTPService.
type HTTPServiceOption func(*HTTPService)

// WithReadTimeout sets the HTTP server read timeout.
func WithReadTimeout(timeout time.Duration) HTTPServiceOption {
	return func(s *HTTPService) {
		s.readTimeout = timeout
	}
}

// WithWriteTimeout sets the HTTP server write timeout.
func WithWriteTimeout(timeout time.Duration) HTTPServiceOption {
	return func(s *HTTPService) {
		s.writeTimeout = timeout
	}
}

// WithShutdownTimeout sets the graceful shutdown timeout.
func WithShutdownTimeout(timeout time.Duration) HTTPServiceOption {
	return func(s *HTTPService) {
		s.shutdownTimeout = timeout
	}
}

// WithMaxHeaderBytes sets the maximum header bytes for the HTTP server.
func WithMaxHeaderBytes(bytes int) HTTPServiceOption {
	return func(s *HTTPService) {
		s.maxHeaderBytes = bytes
	}
}

// NewHTTPService creates a new HTTP service with the provided configuration.
// The handler will be invoked for all incoming HTTP requests.
//
// Example:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/health", healthHandler)
//	svc := service.NewHTTPService("api-server", ":8080", mux,
//	    service.WithShutdownTimeout(30*time.Second),
//	)
func NewHTTPService(name, addr string, handler http.Handler, opts ...HTTPServiceOption) *HTTPService {
	s := &HTTPService{
		name:            name,
		addr:            addr,
		handler:         handler,
		readTimeout:     10 * time.Second,
		writeTimeout:    10 * time.Second,
		shutdownTimeout: 30 * time.Second,
		maxHeaderBytes:  1 << 20, // 1 MB
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start starts the HTTP server and blocks until the server is listening.
// It returns an error if the server fails to start.
func (s *HTTPService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("service %s already started", s.name)
	}

	s.server = &http.Server{
		Addr:           s.addr,
		Handler:        s.handler,
		ReadTimeout:    s.readTimeout,
		WriteTimeout:   s.writeTimeout,
		MaxHeaderBytes: s.maxHeaderBytes,
		BaseContext:    func(net.Listener) context.Context { return ctx },
	}

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait a moment to check for immediate startup errors
	select {
	case err := <-errChan:
		return fmt.Errorf("failed to start HTTP service %s: %w", s.name, err)
	case <-time.After(100 * time.Millisecond):
		// Server started successfully
		s.started = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop gracefully stops the HTTP server, waiting for in-flight requests to complete.
// It respects the context deadline for shutdown timeout.
func (s *HTTPService) Stop(ctx context.Context) error {
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

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP service %s: %w", s.name, err)
	}

	s.mu.Lock()
	s.started = false
	s.mu.Unlock()

	return nil
}

// Name returns the service name.
func (s *HTTPService) Name() string {
	return s.name
}

// Health checks if the HTTP server is running.
// Returns nil if healthy, error if not running.
func (s *HTTPService) Health() error {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()

	if !started {
		return fmt.Errorf("service %s not running", s.name)
	}

	return nil
}
