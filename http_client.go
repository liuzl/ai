package ai

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// baseClient handles the underlying HTTP transport, including authentication,
// endpoint construction, and retry logic for different AI providers.
type baseClient struct {
	httpClient *http.Client
	baseURL    string
	apiVersion string
	headers    http.Header
	maxRetries int
	provider   string
}

// newBaseClient creates and configures a new baseClient.
func newBaseClient(provider, baseURL, apiVersion string, timeout time.Duration, headers http.Header, maxRetries int) *baseClient {
	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Content-Type", "application/json")

	return &baseClient{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    baseURL,
		apiVersion: apiVersion,
		headers:    headers,
		maxRetries: maxRetries,
		provider:   provider,
	}
}

// doRequestRaw performs an HTTP request and returns the raw response body bytes.
// It handles retries with exponential backoff and jitter on 5xx server errors.
func (c *baseClient) doRequestRaw(ctx context.Context, method, path string, reqBody any) ([]byte, error) {
	// Marshal JSON once for reuse across retries
	var jsonBody []byte
	if reqBody != nil {
		var err error
		jsonBody, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path, err = url.JoinPath(u.Path, c.apiVersion, path)
	if err != nil {
		return nil, fmt.Errorf("failed to join URL path: %w", err)
	}

	var httpResp *http.Response
	baseDelay := 1 * time.Second
	maxDelay := 30 * time.Second
	for attempt := range c.maxRetries {
		// Create a new request body for each attempt
		var body io.Reader
		if jsonBody != nil {
			body = bytes.NewReader(jsonBody)
		}

		httpReq, reqErr := http.NewRequestWithContext(ctx, method, u.String(), body)
		if reqErr != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", reqErr)
		}
		// Clone headers to prevent race conditions and request corruption
		httpReq.Header = c.headers.Clone()

		httpResp, err = c.httpClient.Do(httpReq)
		if err == nil && httpResp.StatusCode < 500 {
			break // Success or non-retriable error
		}
		// Close response body if we're going to retry (not the last attempt)
		if attempt < c.maxRetries-1 && httpResp != nil && httpResp.Body != nil {
			httpResp.Body.Close()
		}
		if attempt < c.maxRetries-1 {
			// Calculate backoff duration 2^attempt
			backoff := min(baseDelay*(1<<attempt), maxDelay)
			// Add jitter (randomness) to avoid thundering herd
			// Use crypto/rand for unpredictable jitter
			randomBytes := make([]byte, 2)
			_, _ = rand.Read(randomBytes)                             // Ignore error - worst case is 0 jitter
			jitterMs := int(randomBytes[0])<<8 | int(randomBytes[1])  // 0-65535
			jitter := time.Duration(jitterMs%1000) * time.Millisecond // 0-999ms
			sleepDuration := backoff + jitter

			// Sleep with context cancellation support
			select {
			case <-time.After(sleepDuration):
				// Continue to next retry
			case <-ctx.Done():
				// Context cancelled, return immediately
				return nil, fmt.Errorf("request canceled during retry: %w", ctx.Err())
			}
		}
	}
	if err != nil {
		// Check for timeout error
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, NewTimeoutError(c.provider, c.httpClient.Timeout, err)
		}
		// Check for context cancellation
		if errors.Is(err, context.Canceled) {
			return nil, fmt.Errorf("request canceled: %w", err)
		}
		// Network error (connection refused, DNS, etc.)
		return nil, NewNetworkError(c.provider, err.Error(), err)
	}
	if httpResp == nil {
		return nil, fmt.Errorf("received nil response without error")
	}
	defer httpResp.Body.Close()

	respBodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode >= 400 {
		// Try to parse structured API error
		var apiError struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		errorMessage := string(respBodyBytes)
		errorDetails := ""
		// Use explicit error variable for clarity
		err := json.Unmarshal(respBodyBytes, &apiError)
		if err == nil && apiError.Error.Message != "" {
			errorMessage = apiError.Error.Message
			errorDetails = apiError.Error.Type
		}

		// Return typed errors based on status code
		switch httpResp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, NewAuthenticationError(c.provider, httpResp.StatusCode, errorMessage, nil)
		case http.StatusBadRequest:
			return nil, NewInvalidRequestError(c.provider, errorMessage, errorDetails, nil)
		case http.StatusTooManyRequests:
			retryAfter := parseRetryAfter(httpResp.Header.Get("Retry-After"))
			return nil, NewRateLimitError(c.provider, errorMessage, retryAfter, nil)
		default:
			if httpResp.StatusCode >= 500 {
				return nil, NewServerError(c.provider, httpResp.StatusCode, errorMessage, nil)
			}
			// Other 4xx errors
			return nil, NewUnknownError(c.provider, httpResp.StatusCode, errorMessage, nil)
		}
	}

	return respBodyBytes, nil
}

// parseRetryAfter parses the Retry-After header and returns the duration.
// It supports both seconds (integer) and HTTP date formats.
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}

	// Try parsing as integer (seconds)
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP date
	if t, err := http.ParseTime(header); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return duration
		}
	}

	return 0
}

// doStream performs an HTTP request expecting an SSE response.
// It returns the raw *http.Response and its Body for streaming consumption.
// The caller is responsible for closing the body.
func (c *baseClient) doStream(ctx context.Context, method, path string, reqBody any) (*http.Response, io.ReadCloser, error) {
	var jsonBody []byte
	if reqBody != nil {
		var err error
		jsonBody, err = json.Marshal(reqBody)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path, err = url.JoinPath(u.Path, c.apiVersion, path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to join URL path: %w", err)
	}

	var body io.Reader
	if jsonBody != nil {
		body = bytes.NewReader(jsonBody)
	}

	httpReq, reqErr := http.NewRequestWithContext(ctx, method, u.String(), body)
	if reqErr != nil {
		return nil, nil, fmt.Errorf("failed to create HTTP request: %w", reqErr)
	}
	httpReq.Header = c.headers.Clone()
	httpReq.Header.Set("Accept", "text/event-stream")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Check for timeout error
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, nil, NewTimeoutError(c.provider, c.httpClient.Timeout, err)
		}
		if errors.Is(err, context.Canceled) {
			return nil, nil, fmt.Errorf("request canceled: %w", err)
		}
		return nil, nil, NewNetworkError(c.provider, err.Error(), err)
	}

	if httpResp.StatusCode >= 400 {
		respBodyBytes, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()

		// Try to parse structured API error
		var apiError struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		errorMessage := string(respBodyBytes)
		errorDetails := ""
		err := json.Unmarshal(respBodyBytes, &apiError)
		if err == nil && apiError.Error.Message != "" {
			errorMessage = apiError.Error.Message
			errorDetails = apiError.Error.Type
		}

		switch httpResp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, nil, NewAuthenticationError(c.provider, httpResp.StatusCode, errorMessage, nil)
		case http.StatusBadRequest:
			return nil, nil, NewInvalidRequestError(c.provider, errorMessage, errorDetails, nil)
		case http.StatusTooManyRequests:
			retryAfter := parseRetryAfter(httpResp.Header.Get("Retry-After"))
			return nil, nil, NewRateLimitError(c.provider, errorMessage, retryAfter, nil)
		default:
			if httpResp.StatusCode >= 500 {
				return nil, nil, NewServerError(c.provider, httpResp.StatusCode, errorMessage, nil)
			}
			return nil, nil, NewUnknownError(c.provider, httpResp.StatusCode, errorMessage, nil)
		}
	}

	return httpResp, httpResp.Body, nil
}
