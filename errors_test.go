package ai_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/liuzl/ai"
)

// TestAuthenticationError tests AuthenticationError creation and properties.
func TestAuthenticationError(t *testing.T) {
	originalErr := fmt.Errorf("invalid API key")
	err := ai.NewAuthenticationError("openai", 401, "Invalid API key", originalErr)

	// Test error message
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("Expected 'authentication failed' in error message, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "openai") {
		t.Errorf("Expected provider 'openai' in error message, got: %s", err.Error())
	}

	// Test status code
	if err.StatusCode() != 401 {
		t.Errorf("Expected status code 401, got %d", err.StatusCode())
	}

	// Test provider
	if err.Provider() != "openai" {
		t.Errorf("Expected provider 'openai', got %s", err.Provider())
	}

	// Test unwrap
	if !errors.Is(err, originalErr) {
		t.Error("Expected error to wrap original error")
	}

	// Test type assertion
	var authErr *ai.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Error("Expected error to be AuthenticationError")
	}
}

// TestRateLimitError tests RateLimitError creation and properties.
func TestRateLimitError(t *testing.T) {
	retryAfter := 60 * time.Second
	err := ai.NewRateLimitError("gemini", "Too many requests", retryAfter, nil)

	// Test error message
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("Expected 'rate limit exceeded' in error message, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "retry after") {
		t.Errorf("Expected 'retry after' in error message, got: %s", err.Error())
	}

	// Test status code
	if err.StatusCode() != 429 {
		t.Errorf("Expected status code 429, got %d", err.StatusCode())
	}

	// Test retry after
	if err.RetryAfter != retryAfter {
		t.Errorf("Expected retry after %v, got %v", retryAfter, err.RetryAfter)
	}

	// Test type assertion
	var rateLimitErr *ai.RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Error("Expected error to be RateLimitError")
	}
}

// TestInvalidRequestError tests InvalidRequestError creation and properties.
func TestInvalidRequestError(t *testing.T) {
	err := ai.NewInvalidRequestError("anthropic", "Missing required field", "field: messages", nil)

	// Test error message
	if !strings.Contains(err.Error(), "invalid request") {
		t.Errorf("Expected 'invalid request' in error message, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "field: messages") {
		t.Errorf("Expected details in error message, got: %s", err.Error())
	}

	// Test status code
	if err.StatusCode() != 400 {
		t.Errorf("Expected status code 400, got %d", err.StatusCode())
	}

	// Test details
	if err.Details != "field: messages" {
		t.Errorf("Expected details 'field: messages', got %s", err.Details)
	}

	// Test type assertion
	var invalidReqErr *ai.InvalidRequestError
	if !errors.As(err, &invalidReqErr) {
		t.Error("Expected error to be InvalidRequestError")
	}
}

// TestServerError tests ServerError creation and properties.
func TestServerError(t *testing.T) {
	err := ai.NewServerError("openai", 503, "Service temporarily unavailable", nil)

	// Test error message
	if !strings.Contains(err.Error(), "server error") {
		t.Errorf("Expected 'server error' in error message, got: %s", err.Error())
	}

	// Test status code
	if err.StatusCode() != 503 {
		t.Errorf("Expected status code 503, got %d", err.StatusCode())
	}

	// Test type assertion
	var serverErr *ai.ServerError
	if !errors.As(err, &serverErr) {
		t.Error("Expected error to be ServerError")
	}
}

// TestNetworkError tests NetworkError creation and properties.
func TestNetworkError(t *testing.T) {
	originalErr := fmt.Errorf("connection refused")
	err := ai.NewNetworkError("gemini", "Failed to connect", originalErr)

	// Test error message
	if !strings.Contains(err.Error(), "network error") {
		t.Errorf("Expected 'network error' in error message, got: %s", err.Error())
	}

	// Test status code (should be 0 for network errors)
	if err.StatusCode() != 0 {
		t.Errorf("Expected status code 0, got %d", err.StatusCode())
	}

	// Test unwrap
	if !errors.Is(err, originalErr) {
		t.Error("Expected error to wrap original error")
	}

	// Test type assertion
	var netErr *ai.NetworkError
	if !errors.As(err, &netErr) {
		t.Error("Expected error to be NetworkError")
	}
}

// TestTimeoutError tests TimeoutError creation and properties.
func TestTimeoutError(t *testing.T) {
	duration := 30 * time.Second
	err := ai.NewTimeoutError("anthropic", duration, nil)

	// Test error message
	if !strings.Contains(err.Error(), "request timeout") {
		t.Errorf("Expected 'request timeout' in error message, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "30s") {
		t.Errorf("Expected duration in error message, got: %s", err.Error())
	}

	// Test status code (should be 0 for timeout errors)
	if err.StatusCode() != 0 {
		t.Errorf("Expected status code 0, got %d", err.StatusCode())
	}

	// Test duration
	if err.Duration != duration {
		t.Errorf("Expected duration %v, got %v", duration, err.Duration)
	}

	// Test type assertion
	var timeoutErr *ai.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Error("Expected error to be TimeoutError")
	}
}

// TestUnknownError tests UnknownError creation and properties.
func TestUnknownError(t *testing.T) {
	err := ai.NewUnknownError("openai", 418, "I'm a teapot", nil)

	// Test error message
	if !strings.Contains(err.Error(), "unknown error") {
		t.Errorf("Expected 'unknown error' in error message, got: %s", err.Error())
	}

	// Test status code
	if err.StatusCode() != 418 {
		t.Errorf("Expected status code 418, got %d", err.StatusCode())
	}

	// Test type assertion
	var unknownErr *ai.UnknownError
	if !errors.As(err, &unknownErr) {
		t.Error("Expected error to be UnknownError")
	}
}

// TestErrorWithStatusInterface tests that all errors implement ErrorWithStatus.
func TestErrorWithStatusInterface(t *testing.T) {
	testCases := []struct {
		name  string
		error ai.ErrorWithStatus
	}{
		{"AuthenticationError", ai.NewAuthenticationError("openai", 401, "test", nil)},
		{"RateLimitError", ai.NewRateLimitError("gemini", "test", 60*time.Second, nil)},
		{"InvalidRequestError", ai.NewInvalidRequestError("anthropic", "test", "details", nil)},
		{"ServerError", ai.NewServerError("openai", 500, "test", nil)},
		{"NetworkError", ai.NewNetworkError("gemini", "test", nil)},
		{"TimeoutError", ai.NewTimeoutError("anthropic", 30*time.Second, nil)},
		{"UnknownError", ai.NewUnknownError("openai", 999, "test", nil)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// All should implement error interface
			if tc.error.Error() == "" {
				t.Error("Error message should not be empty")
			}

			// All should have a provider
			if tc.error.Provider() == "" {
				t.Error("Provider should not be empty")
			}

			// All should have a status code (may be 0 for network/timeout)
			_ = tc.error.StatusCode()

			// All should support unwrapping (may be nil)
			_ = tc.error.Unwrap()
		})
	}
}

// TestErrorTypeDistinction tests that different error types can be distinguished.
func TestErrorTypeDistinction(t *testing.T) {
	authErr := ai.NewAuthenticationError("openai", 401, "test", nil)
	rateLimitErr := ai.NewRateLimitError("gemini", "test", 60*time.Second, nil)
	invalidReqErr := ai.NewInvalidRequestError("anthropic", "test", "details", nil)

	// Test that each error is only its own type
	var ae1 *ai.AuthenticationError
	var re1 *ai.RateLimitError
	var ie1 *ai.InvalidRequestError

	if !errors.As(authErr, &ae1) {
		t.Error("authErr should be AuthenticationError")
	}
	if errors.As(authErr, &re1) {
		t.Error("authErr should not be RateLimitError")
	}
	if errors.As(authErr, &ie1) {
		t.Error("authErr should not be InvalidRequestError")
	}

	var ae2 *ai.AuthenticationError
	var re2 *ai.RateLimitError
	var ie2 *ai.InvalidRequestError

	if errors.As(rateLimitErr, &ae2) {
		t.Error("rateLimitErr should not be AuthenticationError")
	}
	if !errors.As(rateLimitErr, &re2) {
		t.Error("rateLimitErr should be RateLimitError")
	}
	if errors.As(rateLimitErr, &ie2) {
		t.Error("rateLimitErr should not be InvalidRequestError")
	}

	var ae3 *ai.AuthenticationError
	var re3 *ai.RateLimitError
	var ie3 *ai.InvalidRequestError

	if errors.As(invalidReqErr, &ae3) {
		t.Error("invalidReqErr should not be AuthenticationError")
	}
	if errors.As(invalidReqErr, &re3) {
		t.Error("invalidReqErr should not be RateLimitError")
	}
	if !errors.As(invalidReqErr, &ie3) {
		t.Error("invalidReqErr should be InvalidRequestError")
	}
}
