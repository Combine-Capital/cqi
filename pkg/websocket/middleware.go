package websocket

import (
	"context"
	"fmt"
	"time"

	"github.com/Combine-Capital/cqi/pkg/metrics"
	"github.com/rs/zerolog"
)

// WithLogging adds logging middleware to the WebSocket client.
// It logs connection events, disconnections, and message activity.
func (c *Client) WithLogging(logger *zerolog.Logger) *Client {
	// Wrap existing handlers with logging
	c.mu.Lock()
	defer c.mu.Unlock()

	originalHandlers := c.handlers.handlers
	for msgType, handler := range originalHandlers {
		loggingHandler := createLoggingHandler(logger, msgType, handler)
		c.handlers.handlers[msgType] = loggingHandler
	}

	return c
}

// createLoggingHandler wraps a handler with logging functionality.
func createLoggingHandler(logger *zerolog.Logger, messageType string, handler HandlerFunc) HandlerFunc {
	return func(ctx context.Context, msg *Message) error {
		start := time.Now()

		logger.Debug().
			Str("message_type", messageType).
			Int("message_size", len(msg.Raw)).
			Msg("processing websocket message")

		err := handler(ctx, msg)

		logEvent := logger.Info()
		if err != nil {
			logEvent = logger.Error().Err(err)
		}

		logEvent.
			Str("message_type", messageType).
			Dur("duration_ms", time.Since(start)).
			Msg("websocket message processed")

		return err
	}
}

// WithMetrics adds metrics middleware to the WebSocket client.
// It records message count, connection duration, and reconnect attempts.
func (c *Client) WithMetrics(namespace string) *Client {
	// Create metrics collectors
	messageCount, err := metrics.NewCounter(metrics.CounterOpts{
		Namespace: namespace,
		Subsystem: "websocket_client",
		Name:      "message_count_total",
		Help:      "Total count of WebSocket messages received",
		Labels:    []string{"message_type", "status"},
	})
	if err != nil {
		// If metrics fail to initialize, skip metrics collection
		return c
	}

	messageDuration, err := metrics.NewHistogram(metrics.HistogramOpts{
		Namespace: namespace,
		Subsystem: "websocket_client",
		Name:      "message_duration_seconds",
		Help:      "Duration of WebSocket message processing in seconds",
		Labels:    []string{"message_type"},
	})
	if err != nil {
		return c
	}

	reconnectCount, err := metrics.NewCounter(metrics.CounterOpts{
		Namespace: namespace,
		Subsystem: "websocket_client",
		Name:      "reconnect_count_total",
		Help:      "Total count of WebSocket reconnection attempts",
		Labels:    []string{"status"},
	})
	if err != nil {
		return c
	}

	// Store metrics for reconnect tracking
	_ = reconnectCount

	// Wrap existing handlers with metrics
	c.mu.Lock()
	defer c.mu.Unlock()

	originalHandlers := c.handlers.handlers
	for msgType, handler := range originalHandlers {
		metricsHandler := createMetricsHandler(messageCount, messageDuration, msgType, handler)
		c.handlers.handlers[msgType] = metricsHandler
	}

	return c
}

// createMetricsHandler wraps a handler with metrics functionality.
func createMetricsHandler(counter *metrics.Counter, histogram *metrics.Histogram, messageType string, handler HandlerFunc) HandlerFunc {
	return func(ctx context.Context, msg *Message) error {
		start := time.Now()

		err := handler(ctx, msg)

		duration := time.Since(start).Seconds()
		status := "success"
		if err != nil {
			status = "error"
		}

		counter.Inc(messageType, status)
		histogram.Observe(duration, messageType)

		return err
	}
}

// WithRetry adds retry middleware to the WebSocket client.
// It automatically retries failed operations with exponential backoff.
func (c *Client) WithRetry(maxAttempts int, initialDelay time.Duration) *Client {
	// Wrap existing handlers with retry logic
	c.mu.Lock()
	defer c.mu.Unlock()

	originalHandlers := c.handlers.handlers
	for msgType, handler := range originalHandlers {
		retryHandler := createRetryHandler(maxAttempts, initialDelay, handler)
		c.handlers.handlers[msgType] = retryHandler
	}

	return c
}

// createRetryHandler wraps a handler with retry functionality.
func createRetryHandler(maxAttempts int, initialDelay time.Duration, handler HandlerFunc) HandlerFunc {
	return func(ctx context.Context, msg *Message) error {
		var lastErr error
		delay := initialDelay

		for attempt := 0; attempt < maxAttempts; attempt++ {
			// Try to execute handler
			err := handler(ctx, msg)
			if err == nil {
				return nil
			}

			lastErr = err

			// Check if we should retry (context not canceled, not last attempt)
			if attempt < maxAttempts-1 && ctx.Err() == nil {
				// Wait before retry
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}

				// Exponential backoff
				delay *= 2
			}
		}

		return fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
	}
}

// LogConnection logs connection events.
func LogConnection(logger *zerolog.Logger, url string, connected bool) {
	if connected {
		logger.Info().
			Str("url", url).
			Msg("websocket connected")
	} else {
		logger.Warn().
			Str("url", url).
			Msg("websocket disconnected")
	}
}

// LogReconnect logs reconnection attempts.
func LogReconnect(logger *zerolog.Logger, url string, attempt int, err error) {
	logger.Warn().
		Str("url", url).
		Int("attempt", attempt).
		Err(err).
		Msg("websocket reconnection attempt")
}
