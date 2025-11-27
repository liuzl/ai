package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	requestIDKey contextKey = "request-id"
)

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from the context
// Returns an empty string if no request ID is found
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// GenerateRequestID generates a unique request ID
// Format: req-{unix_timestamp}-{random_hex}
// Example: req-1732704645-a3f2e1d4b2c8
func GenerateRequestID() string {
	// Get current timestamp
	timestamp := time.Now().Unix()

	// Generate 6 random bytes
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use timestamp only
		return fmt.Sprintf("req-%d-fallback", timestamp)
	}

	// Format: req-{timestamp}-{hex}
	return fmt.Sprintf("req-%d-%x", timestamp, b)
}
