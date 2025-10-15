package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/cache/testproto"
	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/errors"
	"google.golang.org/protobuf/proto"
)

func TestResponse_StatusAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "value")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"key": "value"})
	}))
	defer server.Close()

	cfg := config.HTTPClientConfig{
		BaseURL:    server.URL,
		Timeout:    1 * time.Second,
		RetryCount: 0,
	}

	client, _ := New(context.Background(), cfg)
	defer client.Close()

	resp, err := client.Get(context.Background(), "/test").Do()
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode())
	}

	if resp.Header("X-Test") != "value" {
		t.Error("Expected X-Test header")
	}

	if !resp.IsSuccess() {
		t.Error("Expected IsSuccess() to return true")
	}

	if resp.IsError() {
		t.Error("Expected IsError() to return false")
	}
}

func TestResponse_BodyAsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"key": "value"})
	}))
	defer server.Close()

	cfg := config.HTTPClientConfig{
		BaseURL:    server.URL,
		Timeout:    1 * time.Second,
		RetryCount: 0,
	}

	client, _ := New(context.Background(), cfg)
	defer client.Close()

	resp, err := client.Get(context.Background(), "/test").Do()
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	var result map[string]string
	if err := resp.BodyAsJSON(&result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected key=value, got: %s", result["key"])
	}
}

func TestResponse_BodyAsProto(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		msg := &testproto.TestMessage{
			Id:    "test-123",
			Name:  "Test",
			Value: 42,
		}
		data, _ := proto.Marshal(msg)
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}))
	defer server.Close()

	cfg := config.HTTPClientConfig{
		BaseURL:    server.URL,
		Timeout:    1 * time.Second,
		RetryCount: 0,
	}

	client, _ := New(context.Background(), cfg)
	defer client.Close()

	resp, err := client.Get(context.Background(), "/test").Do()
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	var result testproto.TestMessage
	if err := resp.BodyAsProto(&result); err != nil {
		t.Fatalf("Failed to parse proto: %v", err)
	}

	if result.Id != "test-123" {
		t.Errorf("Expected id=test-123, got: %s", result.Id)
	}
}

func TestResponse_ErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		checkErr   func(error) bool
	}{
		{"400 -> InvalidInput", http.StatusBadRequest, errors.IsInvalidInput},
		{"401 -> Unauthorized", http.StatusUnauthorized, errors.IsUnauthorized},
		{"404 -> NotFound", http.StatusNotFound, errors.IsNotFound},
		{"429 -> Temporary", http.StatusTooManyRequests, errors.IsTemporary},
		{"500 -> Temporary", http.StatusInternalServerError, errors.IsTemporary},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			cfg := config.HTTPClientConfig{
				BaseURL:          server.URL,
				Timeout:          1 * time.Second,
				RetryCount:       1, // Minimum to avoid long waits
				RetryWaitTime:    1 * time.Millisecond,
				RetryMaxWaitTime: 1 * time.Millisecond,
			}

			client, _ := New(context.Background(), cfg)
			defer client.Close()

			_, err := client.Get(context.Background(), "/test").Do()
			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			if !tt.checkErr(err) {
				t.Errorf("Expected specific error type, got: %v", err)
			}
		})
	}
}
