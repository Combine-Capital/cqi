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
	"google.golang.org/protobuf/proto"
)

func TestRequest_Headers(t *testing.T) {
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

	_, err := client.Get(context.Background(), "/test").
		WithHeader("X-Custom", "value").
		Do()

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
}

func TestRequest_QueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "value" {
			t.Error("Expected query param key=value")
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

	_, err := client.Get(context.Background(), "/test").
		WithQuery("key", "value").
		Do()

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
}

func TestRequest_JSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)

		if body["name"] != "test" {
			t.Errorf("Expected name=test, got: %s", body["name"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
	}))
	defer server.Close()

	cfg := config.HTTPClientConfig{
		BaseURL:    server.URL,
		Timeout:    1 * time.Second,
		RetryCount: 0,
	}

	client, _ := New(context.Background(), cfg)
	defer client.Close()

	resp, err := client.Post(context.Background(), "/test").
		WithJSON(map[string]string{"name": "test"}).
		Do()

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Error("Expected successful response")
	}
}

func TestRequest_ProtobufBody(t *testing.T) {
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

	msg := &testproto.TestMessage{
		Id:    "req-123",
		Name:  "Request",
		Value: 100,
	}

	resp, err := client.Post(context.Background(), "/test").
		WithProto(msg).
		Do()

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

func TestRequest_AuthToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected Bearer token, got: %s", auth)
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

	_, err := client.Get(context.Background(), "/test").
		WithAuthToken("test-token").
		Do()

	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
}
