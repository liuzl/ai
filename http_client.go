package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
}

// newBaseClient creates and configures a new baseClient.
func newBaseClient(baseURL, apiVersion string, timeout time.Duration, headers http.Header, maxRetries int) *baseClient {
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
	}
}

// doJSONRequest performs an HTTP request with JSON marshalling for the request
// body and JSON unmarshalling for the response body. It handles retries on
// 5xx server errors.
func (c *baseClient) doJSONRequest(ctx context.Context, method, path string, reqBody, respBody any) error {
	var body io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonBody)
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path, err = url.JoinPath(u.Path, c.apiVersion, path)
	if err != nil {
		return fmt.Errorf("failed to join URL path: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header = c.headers

	var httpResp *http.Response
	// Use a range loop over a number for a concise retry mechanism.
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		httpResp, err = c.httpClient.Do(httpReq)
		if err == nil && httpResp.StatusCode < 500 {
			break // Success or non-retriable error
		}
		if attempt < c.maxRetries-1 {
			time.Sleep(1 * time.Second) // Wait before retrying
		}
	}
	if err != nil {
		return fmt.Errorf("request failed after %d attempts: %w", c.maxRetries, err)
	}
	defer httpResp.Body.Close()

	respBodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode >= 400 {
		// Attempt to unmarshal into a generic error structure to provide more context.
		// This is a best-effort approach, as error structures vary by provider.
		var apiError struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBodyBytes, &apiError) == nil && apiError.Error.Message != "" {
			return fmt.Errorf("API error %d: %s", httpResp.StatusCode, apiError.Error.Message)
		}
		// Fallback for unexpected error formats
		return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(respBodyBytes))
	}

	if respBody != nil {
		if err := json.Unmarshal(respBodyBytes, respBody); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// doRequestRaw performs an HTTP request and returns the raw response body bytes.
// It handles retries on 5xx server errors.
func (c *baseClient) doRequestRaw(ctx context.Context, method, path string, reqBody any) ([]byte, error) {
	var body io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonBody)
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path, err = url.JoinPath(u.Path, c.apiVersion, path)
	if err != nil {
		return nil, fmt.Errorf("failed to join URL path: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header = c.headers

	var httpResp *http.Response
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		httpResp, err = c.httpClient.Do(httpReq)
		if err == nil && httpResp.StatusCode < 500 {
			break // Success or non-retriable error
		}
		if attempt < c.maxRetries-1 {
			time.Sleep(1 * time.Second) // Wait before retrying
		}
	}
	if err != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.maxRetries, err)
	}
	defer httpResp.Body.Close()

	respBodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode >= 400 {
		var apiError struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBodyBytes, &apiError) == nil && apiError.Error.Message != "" {
			return nil, fmt.Errorf("API error %d: %s", httpResp.StatusCode, apiError.Error.Message)
		}
		return nil, fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(respBodyBytes))
	}

	return respBodyBytes, nil
}
