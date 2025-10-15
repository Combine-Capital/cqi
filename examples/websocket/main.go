// WebSocket client example demonstrating CQI WebSocket client usage with auto-reconnect,
// message handlers, and graceful shutdown.
//
// To run:
//  1. Start a WebSocket test server (e.g., wss://ws.postman-echo.com/raw)
//  2. Set environment: export WEBSOCKET_WEBSOCKET_URL=wss://ws.postman-echo.com/raw
//  3. Run: go run main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/Combine-Capital/cqi/pkg/logging"
	"github.com/Combine-Capital/cqi/pkg/websocket"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration from config.yaml and environment variables with WEBSOCKET_ prefix
	cfg := config.MustLoad("config.yaml", "WEBSOCKET")

	// Initialize structured logger
	logger := logging.New(cfg.Log)
	logger.Info().Msg("WebSocket client example starting")

	// Create WebSocket client with auto-reconnect
	client, err := websocket.New(ctx, cfg.WebSocket)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create WebSocket client")
	}
	defer client.Close()

	logger.Info().
		Str("url", cfg.WebSocket.URL).
		Int("reconnect_max_attempts", cfg.WebSocket.ReconnectMaxAttempts).
		Dur("ping_interval", cfg.WebSocket.PingInterval).
		Msg("WebSocket client initialized")

	// Register message handlers before connecting
	registerHandlers(client, logger)

	// Connect to WebSocket server
	logger.Info().Msg("Connecting to WebSocket server...")
	if err := client.Connect(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to WebSocket server")
	}

	logger.Info().
		Bool("connected", client.IsConnected()).
		Msg("WebSocket connection established")

	// Wait group for coordinating goroutines
	var wg sync.WaitGroup

	// Start message sender goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendMessages(ctx, client, logger)
	}()

	// Start connection monitor goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		monitorConnection(ctx, client, logger)
	}()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		cancel()
	case <-ctx.Done():
		logger.Info().Msg("Context canceled")
	}

	// Wait for goroutines to complete
	logger.Info().Msg("Waiting for goroutines to complete...")
	wg.Wait()

	logger.Info().Msg("WebSocket client example completed successfully")
}

// registerHandlers registers message handlers for different message types
func registerHandlers(client *websocket.Client, logger *logging.Logger) {
	// Handler for echo responses
	client.RegisterHandler("echo", func(ctx context.Context, msg *websocket.Message) error {
		logger.Info().
			Str("type", msg.Type).
			Str("payload", string(msg.Payload)).
			Msg("Received echo message")
		return nil
	})

	// Handler for ping messages
	client.RegisterHandler("ping", func(ctx context.Context, msg *websocket.Message) error {
		logger.Debug().
			Str("type", msg.Type).
			Msg("Received ping message")

		// Send pong response
		response := map[string]string{
			"type": "pong",
		}
		if err := client.Send(ctx, response); err != nil {
			logger.Error().Err(err).Msg("Failed to send pong response")
			return err
		}

		logger.Debug().Msg("Sent pong response")
		return nil
	})

	// Handler for generic messages
	client.RegisterHandler("message", func(ctx context.Context, msg *websocket.Message) error {
		logger.Info().
			Str("type", msg.Type).
			Str("payload", string(msg.Payload)).
			Msg("Received message")
		return nil
	})

	logger.Info().
		Int("handlers", 3).
		Msg("Message handlers registered")
}

// sendMessages sends test messages periodically
func sendMessages(ctx context.Context, client *websocket.Client, logger *logging.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	messageCount := 0

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Message sender stopping")
			return

		case <-ticker.C:
			if !client.IsConnected() {
				logger.Warn().Msg("Not connected, skipping message send")
				continue
			}

			messageCount++

			// Send a test message
			message := map[string]interface{}{
				"type":      "echo",
				"message":   fmt.Sprintf("Test message #%d", messageCount),
				"timestamp": time.Now().Unix(),
			}

			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := client.Send(sendCtx, message)
			cancel()

			if err != nil {
				logger.Error().
					Err(err).
					Int("message_count", messageCount).
					Msg("Failed to send message")
				continue
			}

			logger.Info().
				Int("message_count", messageCount).
				Msg("Message sent successfully")
		}
	}
}

// monitorConnection monitors the connection status
func monitorConnection(ctx context.Context, client *websocket.Client, logger *logging.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("Connection monitor stopping")
			return

		case <-ticker.C:
			connected := client.IsConnected()
			logger.Info().
				Bool("connected", connected).
				Msg("Connection status check")

			if !connected {
				logger.Warn().Msg("Connection lost, waiting for auto-reconnect...")
			}
		}
	}
}
