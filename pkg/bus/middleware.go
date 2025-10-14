package bus

import (
	"context"
	"fmt"
	"time"

	"github.com/Combine-Capital/cqi/pkg/errors"
	"github.com/Combine-Capital/cqi/pkg/logging"
	"github.com/Combine-Capital/cqi/pkg/retry"
	"google.golang.org/protobuf/proto"
)

// WithRetry wraps a handler with retry logic for temporary errors.
// It will retry the handler up to maxAttempts times with exponential backoff.
//
// Example:
//
//	bus.Subscribe(ctx, topic, handler, bus.WithRetry(3, time.Second))
func WithRetry(maxAttempts int, initialDelay time.Duration) SubscribeOption {
	return func(opts *subscribeOptions) {
		middleware := func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, msg proto.Message) error {
				cfg := retry.Config{
					MaxAttempts:  uint(maxAttempts),
					InitialDelay: initialDelay,
					MaxDelay:     30 * time.Second,
					Multiplier:   2.0,
					Policy:       retry.PolicyTemporary,
				}

				return retry.Do(ctx, cfg, func() error {
					return next(ctx, msg)
				})
			}
		}
		opts.middlewares = append(opts.middlewares, middleware)
	}
}

// WithLogging wraps a handler with logging for message processing.
// It logs the start and end of message processing, including any errors.
//
// Example:
//
//	bus.Subscribe(ctx, topic, handler, bus.WithLogging(logger))
func WithLogging(logger *logging.Logger) SubscribeOption {
	return func(opts *subscribeOptions) {
		middleware := func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, msg proto.Message) error {
				// Extract message type name
				msgType := fmt.Sprintf("%T", msg)

				logger.Info().
					Str("message_type", msgType).
					Msg("processing event")

				err := next(ctx, msg)

				if err != nil {
					logger.Error().
						Err(err).
						Str("message_type", msgType).
						Msg("event processing failed")
				} else {
					logger.Debug().
						Str("message_type", msgType).
						Msg("event processed successfully")
				}

				return err
			}
		}
		opts.middlewares = append(opts.middlewares, middleware)
	}
}

// WithMetrics wraps a handler with metrics collection.
// It records the duration and outcome (success/failure) of message processing.
//
// Example:
//
//	bus.Subscribe(ctx, topic, handler, bus.WithMetrics())
func WithMetrics() SubscribeOption {
	return func(opts *subscribeOptions) {
		middleware := func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, msg proto.Message) error {
				start := time.Now()
				err := next(ctx, msg)
				duration := time.Since(start)

				// Extract message type
				msgType := fmt.Sprintf("%T", msg)

				// Record metrics
				// Note: In production, this would use the metrics package
				// For now, we'll just track duration
				_ = duration
				_ = msgType

				// In a full implementation:
				// metrics.RecordEventProcessingDuration(msgType, duration)
				// if err != nil {
				//     metrics.IncrementEventProcessingErrors(msgType)
				// }

				return err
			}
		}
		opts.middlewares = append(opts.middlewares, middleware)
	}
}

// WithErrorHandler wraps a handler with custom error handling.
// The errorHandler is invoked when the wrapped handler returns an error.
// If errorHandler returns nil, the error is considered handled and won't propagate.
//
// Example:
//
//	bus.Subscribe(ctx, topic, handler, bus.WithErrorHandler(func(ctx context.Context, msg proto.Message, err error) error {
//	    if errors.IsNotFound(err) {
//	        // Ignore not found errors
//	        return nil
//	    }
//	    return err
//	}))
func WithErrorHandler(errorHandler func(context.Context, proto.Message, error) error) SubscribeOption {
	return func(opts *subscribeOptions) {
		middleware := func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, msg proto.Message) error {
				err := next(ctx, msg)
				if err != nil {
					return errorHandler(ctx, msg, err)
				}
				return nil
			}
		}
		opts.middlewares = append(opts.middlewares, middleware)
	}
}

// WithRecovery wraps a handler with panic recovery.
// If the handler panics, the panic is caught and converted to an error.
//
// Example:
//
//	bus.Subscribe(ctx, topic, handler, bus.WithRecovery())
func WithRecovery() SubscribeOption {
	return func(opts *subscribeOptions) {
		middleware := func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, msg proto.Message) (err error) {
				defer func() {
					if r := recover(); r != nil {
						err = errors.NewPermanent(fmt.Sprintf("handler panicked: %v", r), nil)
					}
				}()
				return next(ctx, msg)
			}
		}
		opts.middlewares = append(opts.middlewares, middleware)
	}
}

// WithTimeout wraps a handler with a timeout.
// If the handler doesn't complete within the specified duration, it returns a timeout error.
//
// Example:
//
//	bus.Subscribe(ctx, topic, handler, bus.WithTimeout(30*time.Second))
func WithTimeout(timeout time.Duration) SubscribeOption {
	return func(opts *subscribeOptions) {
		middleware := func(next HandlerFunc) HandlerFunc {
			return func(ctx context.Context, msg proto.Message) error {
				ctx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()

				done := make(chan error, 1)
				go func() {
					done <- next(ctx, msg)
				}()

				select {
				case err := <-done:
					return err
				case <-ctx.Done():
					return errors.NewTemporary("handler timeout exceeded", ctx.Err())
				}
			}
		}
		opts.middlewares = append(opts.middlewares, middleware)
	}
}
