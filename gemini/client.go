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
	DefaultBaseURL    = "https://generativelanguage.googleapis.com"
	DefaultAPIVersion = "v1beta"
	DefaultTimeout    = 30 * time.Second
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

// APIError represents an API error
type APIError struct {
	StatusCode int
	APIError   Error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.APIError.Message)
}

// doJSONRequest is a generic helper to perform API requests and unmarshal JSON responses.
func (c *Client) doJSONRequest(ctx context.Context, method, path string, reqBody, respBody any) error {
	var body io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonBody)
	}

	// Build URL using url.JoinPath for robustness
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path = u.Path + "/" + c.apiVersion + path

	httpReq, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.apiKey)

	var resp *http.Response
	var lastErr error

	// Retry logic
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			waitTime := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
			}
		}

		resp, err = c.httpClient.Do(httpReq)
		if err == nil {
			// Don't retry on client errors (4xx)
			if resp.StatusCode < 500 && resp.StatusCode >= 400 {
				break
			}
			if resp.StatusCode < 400 {
				break
			}
		}
		lastErr = err
	}

	if lastErr != nil {
		return fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
	}
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiError ErrorResponse
		if err := json.Unmarshal(respBodyBytes, &apiError); err != nil {
			// If we can't parse the error, return a generic one
			return &APIError{
				StatusCode: resp.StatusCode,
				APIError:   Error{Message: string(respBodyBytes)},
			}
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			APIError:   apiError.Error,
		}
	}

	if respBody != nil {
		if err := json.Unmarshal(respBodyBytes, respBody); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// GenerateContent generates content using the Gemini API
func (c *Client) GenerateContent(ctx context.Context, model string, req *GenerateContentRequest) (*GenerateContentResponse, error) {
	path := fmt.Sprintf("/models/%s:generateContent", model)
	var resp GenerateContentResponse
	err := c.doJSONRequest(ctx, "POST", path, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// EmbedContent generates embeddings using the Gemini API
func (c *Client) EmbedContent(ctx context.Context, req *EmbedContentRequest) (*EmbedContentResponse, error) {
	path := fmt.Sprintf("/models/%s:embedContent", req.Model)
	var resp EmbedContentResponse
	err := c.doJSONRequest(ctx, "POST", path, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// BatchEmbedContents generates embeddings for multiple contents
func (c *Client) BatchEmbedContents(ctx context.Context, req *BatchEmbedContentsRequest) (*BatchEmbedContentsResponse, error) {
	path := "/models/embedding-001:batchEmbedContents"
	var resp BatchEmbedContentsResponse
	err := c.doJSONRequest(ctx, "POST", path, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CountTokens counts tokens in the provided content
func (c *Client) CountTokens(ctx context.Context, model string, req *CountTokensRequest) (*CountTokensResponse, error) {
	path := fmt.Sprintf("/models/%s:countTokens", model)
	var resp CountTokensResponse
	err := c.doJSONRequest(ctx, "POST", path, req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListModels lists available models
func (c *Client) ListModels(ctx context.Context) (*ListModelsResponse, error) {
	var resp ListModelsResponse
	err := c.doJSONRequest(ctx, "GET", "/models", nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetModel gets information about a specific model
func (c *Client) GetModel(ctx context.Context, model string) (*Model, error) {
	path := fmt.Sprintf("/models/%s", model)
	var resp Model
	err := c.doJSONRequest(ctx, "GET", path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// UploadFile uploads a file to the Gemini API
func (c *Client) UploadFile(ctx context.Context, req *UploadFileRequest) (*UploadFileResponse, error) {
	var resp UploadFileResponse
	err := c.doJSONRequest(ctx, "POST", "/files", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListFiles lists uploaded files
func (c *Client) ListFiles(ctx context.Context) (*ListFilesResponse, error) {
	var resp ListFilesResponse
	err := c.doJSONRequest(ctx, "GET", "/files", nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFile gets information about a specific file
func (c *Client) GetFile(ctx context.Context, name string) (*File, error) {
	path := fmt.Sprintf("/files/%s", name)
	var resp File
	err := c.doJSONRequest(ctx, "GET", path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteFile deletes a file
func (c *Client) DeleteFile(ctx context.Context, name string) error {
	path := fmt.Sprintf("/files/%s", name)
	return c.doJSONRequest(ctx, "DELETE", path, nil, nil)
}
