package logging

import (
	"io"
	"os"
	"strings"

	"github.com/Combine-Capital/cqi/pkg/config"
	"github.com/rs/zerolog"
)

// Logger provides structured logging with trace context support.
// It wraps zerolog.Logger to provide a consistent interface across the CQI infrastructure.
type Logger struct {
	zlog zerolog.Logger
	cfg  config.LogConfig
}

// New creates a new Logger instance from the provided configuration.
// It configures the log level, output format (JSON/console), and output destination.
func New(cfg config.LogConfig) *Logger {
	// Determine output writer
	var w io.Writer
	switch strings.ToLower(cfg.Output) {
	case "stderr":
		w = os.Stderr
	case "stdout", "":
		w = os.Stdout
	default:
		// If it's a file path, we would open it here
		// For now, default to stdout
		w = os.Stdout
	}

	// Configure output format
	var logger zerolog.Logger
	if strings.ToLower(cfg.Format) == "console" {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: w, TimeFormat: "2006-01-02 15:04:05"})
	} else {
		// Default to JSON
		logger = zerolog.New(w)
	}

	// Add timestamp
	logger = logger.With().Timestamp().Logger()

	// Set log level
	level := parseLogLevel(cfg.Level)
	logger = logger.Level(level)

	return &Logger{
		zlog: logger,
		cfg:  cfg,
	}
}

// parseLogLevel converts a string log level to zerolog.Level.
func parseLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

// Debug returns a debug level event.
func (l *Logger) Debug() *zerolog.Event {
	return l.zlog.Debug()
}

// Info returns an info level event.
func (l *Logger) Info() *zerolog.Event {
	return l.zlog.Info()
}

// Warn returns a warning level event.
func (l *Logger) Warn() *zerolog.Event {
	return l.zlog.Warn()
}

// Error returns an error level event.
func (l *Logger) Error() *zerolog.Event {
	return l.zlog.Error()
}

// Fatal returns a fatal level event.
// The application will exit with status 1 after logging the event.
func (l *Logger) Fatal() *zerolog.Event {
	return l.zlog.Fatal()
}

// Panic returns a panic level event.
// The application will panic after logging the event.
func (l *Logger) Panic() *zerolog.Event {
	return l.zlog.Panic()
}

// With returns a logger with additional context fields.
func (l *Logger) With() zerolog.Context {
	return l.zlog.With()
}

// WithComponent returns a new logger with a component field set.
// This is useful for identifying which package/component generated the log.
func (l *Logger) WithComponent(component string) *Logger {
	newLogger := l.zlog.With().Str(Component, component).Logger()
	return &Logger{
		zlog: newLogger,
		cfg:  l.cfg,
	}
}

// WithServiceName returns a new logger with the service name field set.
func (l *Logger) WithServiceName(serviceName string) *Logger {
	newLogger := l.zlog.With().Str(ServiceName, serviceName).Logger()
	return &Logger{
		zlog: newLogger,
		cfg:  l.cfg,
	}
}

// WithFields returns a new logger with multiple fields set.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	ctx := l.zlog.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}
	newLogger := ctx.Logger()
	return &Logger{
		zlog: newLogger,
		cfg:  l.cfg,
	}
}

// GetZerolog returns the underlying zerolog.Logger for advanced use cases.
func (l *Logger) GetZerolog() *zerolog.Logger {
	return &l.zlog
}

// Level returns the current log level.
func (l *Logger) Level() zerolog.Level {
	return l.zlog.GetLevel()
}

// SetLevel changes the log level.
func (l *Logger) SetLevel(level zerolog.Level) {
	l.zlog = l.zlog.Level(level)
}
