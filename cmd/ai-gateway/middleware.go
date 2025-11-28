package main

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"zliu.org/goutil/rest"
)

// RequestIDMiddleware adds a request ID to each request
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract or generate request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = GenerateRequestID()
		}

		// Store in context
		ctx := WithRequestID(r.Context(), requestID)
		r = r.WithContext(ctx)

		// Add to response headers
		w.Header().Set("X-Request-ID", requestID)

		// Call next handler
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests and responses
func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			requestID := GetRequestID(r.Context())

			// Create a response writer wrapper to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Log request
			rest.Log().Info().
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Msg("request started")

			// Call next handler
			next.ServeHTTP(rw, r)

			// Log response
			duration := time.Since(startTime)
			rest.Log().Info().
				Str("request_id", requestID).
				Dur("duration", duration).
				Int("status_code", rw.statusCode).
				Msg("request completed")
		})
	}
}

// RecoveryMiddleware recovers from panics and logs them
func RecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID := GetRequestID(r.Context())

					// Log the panic
					rest.Log().Error().
						Str("request_id", requestID).
						Str("error", fmt.Sprintf("%v", err)).
						Str("stack", string(debug.Stack())).
						Msg("panic recovered")

					// Return 500 error
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Request-ID", requestID)
					w.WriteHeader(http.StatusInternalServerError)
					w.Write(fmt.Appendf(nil, `{"error":{"message":"Internal server error","request_id":"%s"}}`, requestID))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter is a wrapper around http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher interface for streaming support
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
