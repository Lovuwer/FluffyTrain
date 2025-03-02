// Package middleware provides HTTP middleware for the Titan API.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/joek3softwares-boop/titan/internal/logging"
)

// Logger returns a middleware that logs HTTP requests.
func Logger(logger *slog.Logger) func(next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Get request ID from context
			requestID := logging.GetRequestID(r.Context())

			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds() * 1000 // Convert to milliseconds

			logging.LogRequest(logger, r.Method, r.URL.Path, ww.statusCode, duration, requestID)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
