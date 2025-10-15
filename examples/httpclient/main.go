// HTTP client example demonstrating CQI HTTP client usage with retry, rate limiting,
// circuit breaker, and comprehensive middleware support.
//
// To run:
//  1. Start a test API server (httpbin.org or local)
//  2. Set environment: export HTTPCLIENT_HTTP_CLIENT_BASE_URL=https://httpbin.org
//  3. Run: go run main.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/httpclient"
	"github.com/Combine-Capital/cqi/pkg/logging"
)

func main() {
	ctx := context.Background()

	// Load configuration from config.yaml and environment variables with HTTPCLIENT_ prefix
	cfg := config.MustLoad("config.yaml", "HTTPCLIENT")

	// Initialize structured logger
	logger := logging.New(cfg.Log)
	logger.Info().Msg("HTTP client example starting")

	// Create HTTP client with retry, rate limiting, and circuit breaker
	client, err := httpclient.New(ctx, cfg.HTTPClient)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create HTTP client")
	}
	defer client.Close()

	logger.Info().
		Str("base_url", cfg.HTTPClient.BaseURL).
		Int("retry_count", cfg.HTTPClient.RetryCount).
		Float64("rate_limit", cfg.HTTPClient.RateLimitPerSecond).
		Msg("HTTP client initialized")

	// Demonstrate GET request
	logger.Info().Msg("Executing GET request...")
	if err := demonstrateGet(ctx, client, logger); err != nil {
		logger.Error().Err(err).Msg("GET request failed")
	}

	// Demonstrate POST request with JSON body
	logger.Info().Msg("Executing POST request...")
	if err := demonstratePost(ctx, client, logger); err != nil {
		logger.Error().Err(err).Msg("POST request failed")
	}

	// Demonstrate PUT request
	logger.Info().Msg("Executing PUT request...")
	if err := demonstratePut(ctx, client, logger); err != nil {
		logger.Error().Err(err).Msg("PUT request failed")
	}

	// Demonstrate DELETE request
	logger.Info().Msg("Executing DELETE request...")
	if err := demonstrateDelete(ctx, client, logger); err != nil {
		logger.Error().Err(err).Msg("DELETE request failed")
	}

	// Demonstrate request with headers and query parameters
	logger.Info().Msg("Executing request with headers and query params...")
	if err := demonstrateHeadersAndParams(ctx, client, logger); err != nil {
		logger.Error().Err(err).Msg("Request with headers failed")
	}

	// Demonstrate rate limiting (if enabled)
	if cfg.HTTPClient.RateLimitPerSecond > 0 {
		logger.Info().Msg("Demonstrating rate limiting...")
		demonstrateRateLimiting(ctx, client, logger)
	}

	// Demonstrate error handling
	logger.Info().Msg("Demonstrating error handling...")
	demonstrateErrorHandling(ctx, client, logger)

	logger.Info().Msg("HTTP client example completed successfully")
}

// demonstrateGet shows a simple GET request
func demonstrateGet(ctx context.Context, client *httpclient.Client, logger *logging.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "/get").Do()
	if err != nil {
		return fmt.Errorf("GET request failed: %w", err)
	}

	logger.Info().
		Int("status_code", resp.StatusCode()).
		Int("body_size", len(resp.Body())).
		Str("content_type", resp.Header("Content-Type")).
		Msg("GET request successful")

	// Parse response as JSON
	var result map[string]interface{}
	if err := resp.BodyAsJSON(&result); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	logger.Debug().Interface("response", result).Msg("GET response parsed")
	return nil
}

// demonstratePost shows a POST request with JSON body
func demonstratePost(ctx context.Context, client *httpclient.Client, logger *logging.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Prepare request body
	data := map[string]interface{}{
		"name":    "CQI Example",
		"version": "1.0.0",
		"tags":    []string{"infrastructure", "go", "microservices"},
	}

	resp, err := client.Post(ctx, "/post").
		WithJSON(data).
		Do()
	if err != nil {
		return fmt.Errorf("POST request failed: %w", err)
	}

	logger.Info().
		Int("status_code", resp.StatusCode()).
		Int("body_size", len(resp.Body())).
		Msg("POST request successful")

	// Parse response
	var result map[string]interface{}
	if err := resp.BodyAsJSON(&result); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	logger.Debug().Interface("response", result).Msg("POST response parsed")
	return nil
}

// demonstratePut shows a PUT request
func demonstratePut(ctx context.Context, client *httpclient.Client, logger *logging.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	data := map[string]string{
		"status": "updated",
	}

	resp, err := client.Put(ctx, "/put").
		WithJSON(data).
		Do()
	if err != nil {
		return fmt.Errorf("PUT request failed: %w", err)
	}

	logger.Info().
		Int("status_code", resp.StatusCode()).
		Msg("PUT request successful")

	return nil
}

// demonstrateDelete shows a DELETE request
func demonstrateDelete(ctx context.Context, client *httpclient.Client, logger *logging.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.Delete(ctx, "/delete").Do()
	if err != nil {
		return fmt.Errorf("DELETE request failed: %w", err)
	}

	logger.Info().
		Int("status_code", resp.StatusCode()).
		Msg("DELETE request successful")

	return nil
}

// demonstrateHeadersAndParams shows request with custom headers and query parameters
func demonstrateHeadersAndParams(ctx context.Context, client *httpclient.Client, logger *logging.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "/headers").
		WithHeader("X-Custom-Header", "CQI-Example").
		WithHeader("X-Request-ID", "12345").
		WithQuery("page", "1").
		WithQuery("limit", "10").
		Do()
	if err != nil {
		return fmt.Errorf("request with headers failed: %w", err)
	}

	logger.Info().
		Int("status_code", resp.StatusCode()).
		Msg("Request with headers successful")

	return nil
}

// demonstrateRateLimiting shows rate limiting in action
func demonstrateRateLimiting(ctx context.Context, client *httpclient.Client, logger *logging.Logger) {
	logger.Info().Msg("Making multiple rapid requests to demonstrate rate limiting...")

	start := time.Now()
	requestCount := 5

	for i := 0; i < requestCount; i++ {
		reqStart := time.Now()

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := client.Get(ctx, "/get").Do()
		cancel()

		duration := time.Since(reqStart)

		if err != nil {
			logger.Error().Err(err).Int("request", i+1).Msg("Rate limited request failed")
			continue
		}

		logger.Info().
			Int("request", i+1).
			Dur("duration", duration).
			Msg("Request completed")
	}

	totalDuration := time.Since(start)
	logger.Info().
		Int("total_requests", requestCount).
		Dur("total_duration", totalDuration).
		Msg("Rate limiting demonstration completed")
}

// demonstrateErrorHandling shows how errors are handled
func demonstrateErrorHandling(ctx context.Context, client *httpclient.Client, logger *logging.Logger) {
	logger.Info().Msg("Testing error handling with invalid endpoints...")

	// Test 404 Not Found
	ctx404, cancel404 := context.WithTimeout(ctx, 10*time.Second)
	defer cancel404()

	_, err := client.Get(ctx404, "/status/404").Do()
	if err != nil {
		logger.Info().Err(err).Msg("404 error handled correctly")
	}

	// Test 500 Internal Server Error (should trigger retry)
	ctx500, cancel500 := context.WithTimeout(ctx, 30*time.Second)
	defer cancel500()

	_, err = client.Get(ctx500, "/status/500").Do()
	if err != nil {
		logger.Info().Err(err).Msg("500 error handled with retry")
	}

	// Test timeout
	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancelTimeout()

	_, err = client.Get(ctxTimeout, "/delay/5").Do()
	if err != nil {
		logger.Info().Err(err).Msg("Timeout error handled correctly")
	}

	logger.Info().Msg("Error handling demonstration completed")
}
