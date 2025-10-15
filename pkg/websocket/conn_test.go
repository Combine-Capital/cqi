package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/gorilla/websocket"
)

func TestNewConn(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL:                   getWSURL(server),
		ReconnectInitialDelay: 100 * time.Millisecond,
		ReconnectMaxDelay:     1 * time.Second,
		PingInterval:          100 * time.Millisecond,
		PongWait:              200 * time.Millisecond,
		WriteWait:             100 * time.Millisecond,
		ReadBufferSize:        1024,
		WriteBufferSize:       1024,
	}

	conn, err := NewConn(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	if !conn.IsConnected() {
		t.Error("Expected connection to be connected")
	}
}

func TestConn_SendReceive(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL:                   getWSURL(server),
		PingInterval:          1 * time.Second,
		PongWait:              2 * time.Second,
		WriteWait:             1 * time.Second,
		ReconnectInitialDelay: 100 * time.Millisecond,
		ReconnectMaxDelay:     1 * time.Second,
		ReadBufferSize:        1024,
		WriteBufferSize:       1024,
	}

	conn, err := NewConn(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	// Give connection a moment to fully establish
	time.Sleep(100 * time.Millisecond)

	// Send message
	data := []byte("test message")
	err = conn.Send(context.Background(), websocket.TextMessage, data)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Receive message (echo from server) with longer timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	messageType, received, err := conn.Receive(ctx)
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	if messageType != websocket.TextMessage {
		t.Errorf("Expected TextMessage, got: %d", messageType)
	}

	if string(received) != string(data) {
		t.Errorf("Expected %s, got: %s", data, received)
	}
}

func TestConn_Close(t *testing.T) {
	server := newTestWSServer(t)
	defer server.Close()

	cfg := config.WebSocketConfig{
		URL:          getWSURL(server),
		PingInterval: 1 * time.Second,
		WriteWait:    1 * time.Second,
	}

	conn, err := NewConn(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}

	if !conn.IsConnected() {
		t.Error("Expected connection to be connected")
	}

	// Close connection
	err = conn.Close()
	if err != nil {
		t.Fatalf("Failed to close connection: %v", err)
	}

	if conn.IsConnected() {
		t.Error("Expected connection to be disconnected after close")
	}
}

func TestConn_SendWhenDisconnected(t *testing.T) {
	server := newTestWSServer(t)
	server.Close() // Close server immediately

	cfg := config.WebSocketConfig{
		URL:                  getWSURL(server),
		ReconnectMaxAttempts: 0, // No reconnection
		PingInterval:         1 * time.Second,
		WriteWait:            100 * time.Millisecond,
	}

	conn, err := NewConn(context.Background(), cfg)
	if err == nil {
		defer conn.Close()
		t.Fatal("Expected error creating connection to closed server")
	}
}

func TestConn_Reconnect(t *testing.T) {
	t.Skip("Reconnect test requires complex server setup")
	// This test would require simulating connection loss
	// and verifying reconnection behavior
}

func TestConn_PingHandler(t *testing.T) {
	t.Skip("Ping handler test requires time-based verification")
	// This test would require waiting for ping intervals
	// and verifying ping messages are sent
}
