// Package httpclient provides an HTTP/REST client with retry, circuit breaker, rate limiting,
// and comprehensive middleware support for CQI infrastructure. It wraps the resty library
// with CQI patterns for observability and error handling.
//
// Example usage:
//
//	cfg := config.HTTPClientConfig{
//	    BaseURL:    "https://api.example.com",
//	    Timeout:    30 * time.Second,
//	    RetryCount: 3,
//	}
//
//	client, err := httpclient.New(ctx, cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Make a GET request
//	resp, err := client.Get(ctx, "/users/123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Make a POST request with JSON body
//	user := map[string]string{"name": "John"}
//	resp, err = client.Post(ctx, "/users").WithJSON(user).Do()
//
//	// Use with protobuf
//	var userProto pb.User
//	resp, err = client.Get(ctx, "/users/123").IntoProto(&userProto).Do()
package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"golang.org/x/time/rate"
	"resty.dev/v3"
)

// Client provides HTTP/REST client functionality with retry, circuit breaker,
// rate limiting, and middleware support.
type Client struct {
	resty   *resty.Client
	config  config.HTTPClientConfig
	limiter *rate.Limiter
	ctx     context.Context
	cancel  context.CancelFunc
}

// New creates a new HTTP client with the provided configuration.
// It initializes connection pooling, retry logic, circuit breaker (if enabled),
// and rate limiting (if configured).
func New(ctx context.Context, cfg config.HTTPClientConfig) (*Client, error) {
	// Apply defaults
	cfg = applyDefaults(cfg)

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, errors.Wrap(err, "invalid http client config")
	}

	// Create resty client
	restyClient := resty.New()

	// Set base URL if provided
	if cfg.BaseURL != "" {
		restyClient.SetBaseURL(cfg.BaseURL)
	}

	// Set timeout
	restyClient.SetTimeout(cfg.Timeout)

	// Configure retry
	if cfg.RetryCount > 0 {
		restyClient.
			SetRetryCount(cfg.RetryCount).
			SetRetryWaitTime(cfg.RetryWaitTime).
			SetRetryMaxWaitTime(cfg.RetryMaxWaitTime)

		// Add retry condition for temporary errors and 5xx status codes
		restyClient.AddRetryConditions(func(res *resty.Response, err error) bool {
			// Retry on error
			if err != nil {
				// Check if it's a temporary error using CQI error types
				return errors.IsTemporary(err)
			}

			// Retry on 5xx status codes (except 501 Not Implemented)
			statusCode := res.StatusCode()
			return statusCode >= 500 && statusCode != 501
		})
	}

	// Configure transport (connection pooling)
	transport := &http.Transport{
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ExpectContinueTimeout: cfg.ExpectContinueTimeout,
	}
	restyClient.SetTransport(transport)

	// Configure circuit breaker if enabled
	if cfg.CircuitBreakerEnabled {
		cb := resty.NewCircuitBreaker().
			SetTimeout(cfg.CircuitBreakerTimeout).
			SetFailureThreshold(uint32(cfg.CircuitBreakerFailureThreshold)).
			SetSuccessThreshold(uint32(cfg.CircuitBreakerSuccessThreshold))
		restyClient.SetCircuitBreaker(cb)
	}

	// Create rate limiter if configured
	var limiter *rate.Limiter
	if cfg.RateLimitPerSecond > 0 {
		limiter = rate.NewLimiter(rate.Limit(cfg.RateLimitPerSecond), cfg.RateLimitBurst)
	}

	// Create client context
	clientCtx, cancel := context.WithCancel(ctx)

	return &Client{
		resty:   restyClient,
		config:  cfg,
		limiter: limiter,
		ctx:     clientCtx,
		cancel:  cancel,
	}, nil
}

// Get creates a new GET request for the specified URL.
// The URL can be relative (appended to BaseURL) or absolute.
func (c *Client) Get(ctx context.Context, url string) *Request {
	return c.NewRequest(ctx).SetMethod(http.MethodGet).SetURL(url)
}

// Post creates a new POST request for the specified URL.
func (c *Client) Post(ctx context.Context, url string) *Request {
	return c.NewRequest(ctx).SetMethod(http.MethodPost).SetURL(url)
}

// Put creates a new PUT request for the specified URL.
func (c *Client) Put(ctx context.Context, url string) *Request {
	return c.NewRequest(ctx).SetMethod(http.MethodPut).SetURL(url)
}

// Patch creates a new PATCH request for the specified URL.
func (c *Client) Patch(ctx context.Context, url string) *Request {
	return c.NewRequest(ctx).SetMethod(http.MethodPatch).SetURL(url)
}

// Delete creates a new DELETE request for the specified URL.
func (c *Client) Delete(ctx context.Context, url string) *Request {
	return c.NewRequest(ctx).SetMethod(http.MethodDelete).SetURL(url)
}

// Head creates a new HEAD request for the specified URL.
func (c *Client) Head(ctx context.Context, url string) *Request {
	return c.NewRequest(ctx).SetMethod(http.MethodHead).SetURL(url)
}

// Options creates a new OPTIONS request for the specified URL.
func (c *Client) Options(ctx context.Context, url string) *Request {
	return c.NewRequest(ctx).SetMethod(http.MethodOptions).SetURL(url)
}

// NewRequest creates a new request with the client's configuration.
func (c *Client) NewRequest(ctx context.Context) *Request {
	return &Request{
		client: c,
		resty:  c.resty.R(),
		ctx:    ctx,
	}
}

// Close releases all resources associated with the client.
// It cancels the client context and closes the circuit breaker.
func (c *Client) Close() error {
	c.cancel()
	c.resty.Close()
	return nil
}

// checkRateLimit enforces rate limiting before making a request.
// It blocks until a token is available or the context is canceled.
func (c *Client) checkRateLimit(ctx context.Context) error {
	if c.limiter == nil {
		return nil
	}

	if err := c.limiter.Wait(ctx); err != nil {
		return errors.Wrap(err, "rate limit wait failed")
	}

	return nil
}

// applyDefaults applies default values to unset configuration fields.
func applyDefaults(cfg config.HTTPClientConfig) config.HTTPClientConfig {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.RetryCount == 0 {
		cfg.RetryCount = 3
	}
	if cfg.RetryWaitTime == 0 {
		cfg.RetryWaitTime = 1 * time.Second
	}
	if cfg.RetryMaxWaitTime == 0 {
		cfg.RetryMaxWaitTime = 10 * time.Second
	}
	if cfg.RateLimitBurst == 0 && cfg.RateLimitPerSecond > 0 {
		cfg.RateLimitBurst = 1
	}
	if cfg.CircuitBreakerTimeout == 0 {
		cfg.CircuitBreakerTimeout = 60 * time.Second
	}
	if cfg.CircuitBreakerFailureThreshold == 0 {
		cfg.CircuitBreakerFailureThreshold = 5
	}
	if cfg.CircuitBreakerSuccessThreshold == 0 {
		cfg.CircuitBreakerSuccessThreshold = 2
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 100
	}
	if cfg.MaxIdleConnsPerHost == 0 {
		cfg.MaxIdleConnsPerHost = 10
	}
	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = 90 * time.Second
	}
	if cfg.TLSHandshakeTimeout == 0 {
		cfg.TLSHandshakeTimeout = 10 * time.Second
	}
	if cfg.ExpectContinueTimeout == 0 {
		cfg.ExpectContinueTimeout = 1 * time.Second
	}
	return cfg
}

// validateConfig validates the HTTP client configuration.
func validateConfig(cfg config.HTTPClientConfig) error {
	if cfg.Timeout < 0 {
		return fmt.Errorf("timeout must be positive, got: %v", cfg.Timeout)
	}
	if cfg.RetryCount < 0 {
		return fmt.Errorf("retry_count must be non-negative, got: %d", cfg.RetryCount)
	}
	if cfg.RateLimitPerSecond < 0 {
		return fmt.Errorf("rate_limit_per_second must be non-negative, got: %f", cfg.RateLimitPerSecond)
	}
	if cfg.RateLimitBurst < 0 {
		return fmt.Errorf("rate_limit_burst must be non-negative, got: %d", cfg.RateLimitBurst)
	}
	if cfg.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns must be non-negative, got: %d", cfg.MaxIdleConns)
	}
	return nil
}
