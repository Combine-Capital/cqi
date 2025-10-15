package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/rs/zerolog"
)

func TestClient_WithAuthToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer global-token" {
			t.Errorf("Expected Bearer global-token, got: %s", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.HTTPClientConfig{
		BaseURL:    server.URL,
		Timeout:    1 * time.Second,
		RetryCount: 0,
	}

	client, _ := New(context.Background(), cfg)
	defer client.Close()

	client.WithAuthToken("global-token")

	_, err := client.Get(context.Background(), "/test").Do()
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
}

func TestClient_WithDefaultHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "value" {
			t.Error("Expected X-Custom header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.HTTPClientConfig{
		BaseURL:    server.URL,
		Timeout:    1 * time.Second,
		RetryCount: 0,
	}

	client, _ := New(context.Background(), cfg)
	defer client.Close()

	client.WithDefaultHeader("X-Custom", "value")

	_, err := client.Get(context.Background(), "/test").Do()
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
}

func TestClient_WithLogging(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.HTTPClientConfig{
		BaseURL:    server.URL,
		Timeout:    1 * time.Second,
		RetryCount: 0,
	}

	client, _ := New(context.Background(), cfg)
	defer client.Close()

	logger := zerolog.Nop()
	client.WithLogging(&logger)

	// Test that logging middleware doesn't break requests
	_, err := client.Get(context.Background(), "/test").Do()
	if err != nil {
		t.Fatalf("Request with logging failed: %v", err)
	}
}
