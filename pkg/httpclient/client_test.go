package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
)

func TestNew(t *testing.T) {
	t.Run("creates client with defaults", func(t *testing.T) {
		cfg := config.HTTPClientConfig{
			BaseURL: "https://api.example.com",
		}

		client, err := New(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Close()

		if client.config.Timeout == 0 {
			t.Error("Expected default timeout to be applied")
		}
		if client.config.RetryCount == 0 {
			t.Error("Expected default retry count to be applied")
		}
	})

	t.Run("validates negative timeout", func(t *testing.T) {
		cfg := config.HTTPClientConfig{
			BaseURL: "https://api.example.com",
			Timeout: -1 * time.Second,
		}

		_, err := New(context.Background(), cfg)
		if err == nil {
			t.Fatal("Expected validation error for negative timeout")
		}
	})
}

func TestClient_HTTPMethods(t *testing.T) {
	methods := []struct {
		name   string
		method string
		fn     func(*Client, context.Context, string) *Request
	}{
		{"GET", http.MethodGet, func(c *Client, ctx context.Context, url string) *Request { return c.Get(ctx, url) }},
		{"POST", http.MethodPost, func(c *Client, ctx context.Context, url string) *Request { return c.Post(ctx, url) }},
		{"PUT", http.MethodPut, func(c *Client, ctx context.Context, url string) *Request { return c.Put(ctx, url) }},
		{"PATCH", http.MethodPatch, func(c *Client, ctx context.Context, url string) *Request { return c.Patch(ctx, url) }},
		{"DELETE", http.MethodDelete, func(c *Client, ctx context.Context, url string) *Request { return c.Delete(ctx, url) }},
	}

	for _, tt := range methods {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("Expected method %s, got: %s", tt.method, r.Method)
				}
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			}))
			defer server.Close()

			cfg := config.HTTPClientConfig{
				BaseURL:    server.URL,
				Timeout:    1 * time.Second,
				RetryCount: 0,
			}

			client, err := New(context.Background(), cfg)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}
			defer client.Close()

			ctx := context.Background()
			resp, err := tt.fn(client, ctx, "/test").Do()
			if err != nil {
				t.Fatalf("%s request failed: %v", tt.method, err)
			}

			if !resp.IsSuccess() {
				t.Errorf("Expected success, got status: %d", resp.StatusCode())
			}
		})
	}
}

func TestClient_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.HTTPClientConfig{
		BaseURL:            server.URL,
		Timeout:            1 * time.Second,
		RateLimitPerSecond: 10, // 10 requests per second
		RateLimitBurst:     1,
		RetryCount:         0,
	}

	client, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	start := time.Now()

	// Make 3 requests - should be rate limited
	for i := 0; i < 3; i++ {
		_, err := client.Get(ctx, "/test").Do()
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
	}

	elapsed := time.Since(start)

	// With 10/sec and burst of 1, 3 requests should take at least 100ms
	if elapsed < 80*time.Millisecond {
		t.Errorf("Rate limiting may not be working, took: %v", elapsed)
	}
}

func TestClient_CircuitBreaker(t *testing.T) {
	cfg := config.HTTPClientConfig{
		BaseURL:                        "https://api.example.com",
		Timeout:                        1 * time.Second,
		CircuitBreakerEnabled:          true,
		CircuitBreakerTimeout:          100 * time.Millisecond,
		CircuitBreakerFailureThreshold: 2,
		CircuitBreakerSuccessThreshold: 1,
		RetryCount:                     0,
	}

	client, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	if !client.config.CircuitBreakerEnabled {
		t.Error("Expected circuit breaker to be enabled")
	}
}
