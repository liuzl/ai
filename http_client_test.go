package ai

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHTTPClientSuccessNoRetry tests a successful request without retries
func TestHTTPClientSuccessNoRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"success"}`))
	}))
	defer server.Close()

	client := newBaseClient("test", server.URL, "", 5*time.Second, nil, 3)
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

// TestHTTPClientRetryOn500 tests retry behavior on 500 errors
func TestHTTPClientRetryOn500(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"server error"}`))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"result":"success"}`))
		}
	}))
	defer server.Close()

	client := newBaseClient("test", server.URL, "", 5*time.Second, nil, 3)
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestHTTPClientRetryOn503 tests retry behavior on 503 Service Unavailable
func TestHTTPClientRetryOn503(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"service unavailable"}`))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"result":"success"}`))
		}
	}))
	defer server.Close()

	client := newBaseClient("test", server.URL, "", 5*time.Second, nil, 3)
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

// TestHTTPClientNoRetryOn400 tests that 400 errors are not retried
func TestHTTPClientNoRetryOn400(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client := newBaseClient("test", server.URL, "", 5*time.Second, nil, 3)
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var invalidReqErr *InvalidRequestError
	if !errors.As(err, &invalidReqErr) {
		t.Errorf("Expected InvalidRequestError, got %T", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retry), got %d", attempts)
	}
}

// TestHTTPClientNoRetryOn401 tests that 401 errors are not retried
func TestHTTPClientNoRetryOn401(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"unauthorized"}}`))
	}))
	defer server.Close()

	client := newBaseClient("test", server.URL, "", 5*time.Second, nil, 3)
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Errorf("Expected AuthenticationError, got %T", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retry), got %d", attempts)
	}
}

// TestHTTPClientMaxRetriesExceeded tests behavior when max retries are exceeded
func TestHTTPClientMaxRetriesExceeded(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"always fails"}`))
	}))
	defer server.Close()

	maxRetries := 3
	client := newBaseClient("test", server.URL, "", 5*time.Second, nil, maxRetries)
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if err == nil {
		t.Fatal("Expected error after max retries, got nil")
	}

	var serverErr *ServerError
	if !errors.As(err, &serverErr) {
		t.Errorf("Expected ServerError, got %T", err)
	}

	if attempts != maxRetries {
		t.Errorf("Expected %d attempts, got %d", maxRetries, attempts)
	}
}

// TestHTTPClientExponentialBackoff tests that backoff increases exponentially
func TestHTTPClientExponentialBackoff(t *testing.T) {
	attempts := 0
	requestTimes := []time.Time{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		requestTimes = append(requestTimes, time.Now())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	client := newBaseClient("test", server.URL, "", 10*time.Second, nil, 3)
	client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if len(requestTimes) < 2 {
		t.Fatal("Not enough requests to test backoff")
	}

	// Check that delays are increasing (with some tolerance for jitter)
	// First retry should be ~1s, second retry should be ~2s
	firstDelay := requestTimes[1].Sub(requestTimes[0])
	if firstDelay < 900*time.Millisecond || firstDelay > 2*time.Second {
		t.Errorf("First retry delay out of expected range (0.9-2s): %v", firstDelay)
	}

	if len(requestTimes) >= 3 {
		secondDelay := requestTimes[2].Sub(requestTimes[1])
		// Second delay should be roughly 2x the base (with jitter), so 2s-3s range
		if secondDelay < 1800*time.Millisecond || secondDelay > 4*time.Second {
			t.Errorf("Second retry delay out of expected range (1.8-4s): %v", secondDelay)
		}
	}
}

// TestHTTPClientContextCancellation tests context cancellation during retry
func TestHTTPClientContextCancellation(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		// Always return 500 to trigger retries
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client := newBaseClient("test", server.URL, "", 30*time.Second, nil, 10)
	_, err := client.doRequestRaw(ctx, "POST", "/test", map[string]string{"key": "value"})

	if err == nil {
		t.Fatal("Expected error from context cancellation, got nil")
	}

	// Should have made at least 1 attempt, but not all 10 due to context cancellation
	if attempts == 0 {
		t.Error("Expected at least 1 attempt")
	}
	if attempts >= 10 {
		t.Error("Expected fewer attempts due to context cancellation")
	}
}

// TestHTTPClientTimeout tests request timeout
func TestHTTPClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"success"}`))
	}))
	defer server.Close()

	// Set very short timeout
	client := newBaseClient("test", server.URL, "", 100*time.Millisecond, nil, 1)
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Errorf("Expected TimeoutError, got %T: %v", err, err)
	}
}

// TestHTTPClientRateLimitError tests 429 rate limit error handling
func TestHTTPClientRateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limit exceeded"}}`))
	}))
	defer server.Close()

	client := newBaseClient("test", server.URL, "", 5*time.Second, nil, 3)
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var rateLimitErr *RateLimitError
	if !errors.As(err, &rateLimitErr) {
		t.Errorf("Expected RateLimitError, got %T", err)
	}

	if rateLimitErr.RetryAfter != 60*time.Second {
		t.Errorf("Expected RetryAfter = 60s, got %v", rateLimitErr.RetryAfter)
	}
}

// TestParseRetryAfterSeconds tests parsing Retry-After header with seconds
func TestParseRetryAfterSeconds(t *testing.T) {
	tests := []struct {
		header   string
		expected time.Duration
	}{
		{"60", 60 * time.Second},
		{"120", 120 * time.Second},
		{"0", 0},
		{"", 0},
		{"invalid", 0},
		{"-10", 0}, // Negative values should return 0
	}

	for _, tt := range tests {
		result := parseRetryAfter(tt.header)
		if result != tt.expected {
			t.Errorf("parseRetryAfter(%q) = %v, expected %v", tt.header, result, tt.expected)
		}
	}
}

// TestParseRetryAfterHTTPDate tests parsing Retry-After header with HTTP date
func TestParseRetryAfterHTTPDate(t *testing.T) {
	// Create a time 30 seconds in the future (use UTC for HTTP date format)
	futureTime := time.Now().UTC().Add(30 * time.Second)
	httpDate := futureTime.Format(http.TimeFormat)

	result := parseRetryAfter(httpDate)

	// Allow some tolerance (25-35 seconds) due to processing time
	if result < 25*time.Second || result > 35*time.Second {
		t.Errorf("parseRetryAfter(%q) = %v, expected ~30s", httpDate, result)
	}

	// Test past date (should return 0)
	pastTime := time.Now().UTC().Add(-30 * time.Second)
	pastHTTPDate := pastTime.Format(http.TimeFormat)
	result = parseRetryAfter(pastHTTPDate)
	if result != 0 {
		t.Errorf("parseRetryAfter(past date) = %v, expected 0", result)
	}
}

// TestHTTPClientInvalidJSON tests handling of invalid JSON in request
func TestHTTPClientInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"success"}`))
	}))
	defer server.Close()

	client := newBaseClient("test", server.URL, "", 5*time.Second, nil, 3)

	// Create a value that cannot be marshaled to JSON
	invalidValue := make(chan int) // channels cannot be marshaled
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]interface{}{"invalid": invalidValue})

	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}

	if !errors.Is(err, nil) && err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

// TestHTTPClientProviderInError tests that provider name is included in errors
func TestHTTPClientProviderInError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	providerName := "test-provider"
	client := newBaseClient(providerName, server.URL, "", 5*time.Second, nil, 3)
	_, err := client.doRequestRaw(context.Background(), "POST", "/test", map[string]string{"key": "value"})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var authErr *AuthenticationError
	if errors.As(err, &authErr) {
		if authErr.Provider() != providerName {
			t.Errorf("Expected provider %q in error, got %q", providerName, authErr.Provider())
		}
	} else {
		t.Errorf("Expected AuthenticationError, got %T", err)
	}
}
