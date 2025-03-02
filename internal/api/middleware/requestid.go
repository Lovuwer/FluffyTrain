// Package middleware provides HTTP middleware for the Titan API.
package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/joek3softwares-boop/titan/internal/logging"
)

// RequestIDHeader is the header name for request ID.
const RequestIDHeader = "X-Request-ID"

// RequestID returns a middleware that adds a request ID to each request.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if request already has an ID
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add to response header
		w.Header().Set(RequestIDHeader, requestID)

		// Add to context
		ctx := logging.WithRequestID(r.Context(), requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
