package gemini

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

const (
	// DefaultBaseURL is the default Gemini API base URL
	DefaultBaseURL = "https://generativelanguage.googleapis.com"
	// DefaultAPIVersion is the default API version
	DefaultAPIVersion = "v1beta"
	// DefaultTimeout is the default HTTP client timeout
	DefaultTimeout = 30 * time.Second
	// DefaultMaxRetries is the default number of retries
	DefaultMaxRetries = 3
)

// Client represents a Gemini API client
type Client struct {
	apiKey     string
	baseURL    string
	apiVersion string
	httpClient *http.Client
	maxRetries int
}

// ClientOption is a function that configures a Client
type ClientOption func(*Client)

// WithBaseURL sets the base URL for the client
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithAPIVersion sets the API version for the client
func WithAPIVersion(apiVersion string) ClientOption {
	return func(c *Client) {
		c.apiVersion = apiVersion
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithMaxRetries sets the maximum number of retries
func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *Client) {
		c.maxRetries = maxRetries
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new Gemini API client
func NewClient(apiKey string, opts ...ClientOption) *Client {
	client := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		apiVersion: DefaultAPIVersion,
		httpClient: &http.Client{Timeout: DefaultTimeout},
		maxRetries: DefaultMaxRetries,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// request represents an HTTP request
type request struct {
	method  string
	path    string
	body    any
	headers map[string]string
	query   map[string]string
}

// response represents an HTTP response
type response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// doRequest performs an HTTP request with retry logic
func (c *Client) doRequest(ctx context.Context, req *request) (*response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.performRequest(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Don't retry on client errors (4xx)
		if resp != nil && resp.StatusCode >= 400 && resp.StatusCode < 500 {
			break
		}

		// Wait before retrying (exponential backoff)
		if attempt < c.maxRetries {
			waitTime := time.Duration(1<<attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitTime):
				continue
			}
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// performRequest performs a single HTTP request
func (c *Client) performRequest(ctx context.Context, req *request) (*response, error) {
	// Build URL
	u, err := url.Parse(c.baseURL + "/" + c.apiVersion + req.path)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Add query parameters
	if req.query != nil {
		q := u.Query()
		for k, v := range req.query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}

	// Prepare request body
	var body io.Reader
	if req.body != nil {
		jsonBody, err := json.Marshal(req.body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonBody)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.apiKey)

	for k, v := range req.headers {
		httpReq.Header.Set(k, v)
	}

	// Perform request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		var apiError ErrorResponse
		if err := json.Unmarshal(respBody, &apiError); err != nil {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			APIError:   apiError.Error,
		}
	}

	return &response{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    resp.Header,
	}, nil
}

// APIError represents an API error
type APIError struct {
	StatusCode int
	APIError   Error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.APIError.Message)
}

// GenerateContent generates content using the Gemini API
func (c *Client) GenerateContent(ctx context.Context, model string, req *GenerateContentRequest) (*GenerateContentResponse, error) {
	path := fmt.Sprintf("/models/%s:generateContent", model)

	httpReq := &request{
		method: "POST",
		path:   path,
		body:   req,
	}

	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result GenerateContentResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// EmbedContent generates embeddings using the Gemini API
func (c *Client) EmbedContent(ctx context.Context, req *EmbedContentRequest) (*EmbedContentResponse, error) {
	path := fmt.Sprintf("/models/%s:embedContent", req.Model)

	httpReq := &request{
		method: "POST",
		path:   path,
		body:   req,
	}

	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result EmbedContentResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// BatchEmbedContents generates embeddings for multiple contents
func (c *Client) BatchEmbedContents(ctx context.Context, req *BatchEmbedContentsRequest) (*BatchEmbedContentsResponse, error) {
	path := "/models/embedding-001:batchEmbedContents"

	httpReq := &request{
		method: "POST",
		path:   path,
		body:   req,
	}

	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result BatchEmbedContentsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// CountTokens counts tokens in the provided content
func (c *Client) CountTokens(ctx context.Context, model string, req *CountTokensRequest) (*CountTokensResponse, error) {
	path := fmt.Sprintf("/models/%s:countTokens", model)

	httpReq := &request{
		method: "POST",
		path:   path,
		body:   req,
	}

	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result CountTokensResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// ListModels lists available models
func (c *Client) ListModels(ctx context.Context) (*ListModelsResponse, error) {
	httpReq := &request{
		method: "GET",
		path:   "/models",
	}

	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result ListModelsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// GetModel gets information about a specific model
func (c *Client) GetModel(ctx context.Context, model string) (*Model, error) {
	path := fmt.Sprintf("/models/%s", model)

	httpReq := &request{
		method: "GET",
		path:   path,
	}

	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result Model
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// UploadFile uploads a file to the Gemini API
func (c *Client) UploadFile(ctx context.Context, req *UploadFileRequest) (*UploadFileResponse, error) {
	httpReq := &request{
		method: "POST",
		path:   "/files",
		body:   req,
	}

	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result UploadFileResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// ListFiles lists uploaded files
func (c *Client) ListFiles(ctx context.Context) (*ListFilesResponse, error) {
	httpReq := &request{
		method: "GET",
		path:   "/files",
	}

	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result ListFilesResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// GetFile gets information about a specific file
func (c *Client) GetFile(ctx context.Context, name string) (*File, error) {
	path := fmt.Sprintf("/files/%s", name)

	httpReq := &request{
		method: "GET",
		path:   path,
	}

	resp, err := c.doRequest(ctx, httpReq)
	if err != nil {
		return nil, err
	}

	var result File
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// DeleteFile deletes a file
func (c *Client) DeleteFile(ctx context.Context, name string) error {
	path := fmt.Sprintf("/files/%s", name)

	httpReq := &request{
		method: "DELETE",
		path:   path,
	}

	_, err := c.doRequest(ctx, httpReq)
	return err
}
