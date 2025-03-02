// package logging - structured logging for titan
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config configures the logger.
type Config struct {
	// Level is the minimum log level (debug, info, warn, error).
	Level string

	// Format is the log format (json, text).
	Format string

	// Service is the service name (api, worker).
	Service string

	// Output is the output writer (default: os.Stdout).
	Output io.Writer
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(service string) Config {
	return Config{
		Level:   "info",
		Format:  "json",
		Service: service,
		Output:  os.Stdout,
	}
}

// New creates a new structured logger.
func New(cfg Config) *slog.Logger {
	level := parseLevel(cfg.Level)
	
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	if strings.ToLower(cfg.Format) == "text" {
		handler = slog.NewTextHandler(output, opts)
	} else {
		handler = slog.NewJSONHandler(output, opts)
	}

	// Add default attributes
	logger := slog.New(handler).With(
		slog.String("service", cfg.Service),
	)

	return logger
}

// parseLevel converts a string log level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// SetDefault sets the default logger.
func SetDefault(logger *slog.Logger) {
	slog.SetDefault(logger)
}
