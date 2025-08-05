package openai

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
	DefaultBaseURL    = "https://api.openai.com"
	DefaultAPIVersion = "v1"
	DefaultTimeout    = 30 * time.Second
)

// Client represents an OpenAI API client.
type Client struct {
	apiKey     string
	baseURL    string
	apiVersion string
	httpClient *http.Client
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL sets the base URL for the client.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithAPIVersion sets the API version for the client.
func WithAPIVersion(apiVersion string) ClientOption {
	return func(c *Client) {
		c.apiVersion = apiVersion
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new OpenAI API client.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	client := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		apiVersion: DefaultAPIVersion,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// APIError represents an API error.
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

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiError ErrorResponse
		if err := json.Unmarshal(respBodyBytes, &apiError); err != nil {
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

// CreateChatCompletion creates a chat completion.
func (c *Client) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	var resp ChatCompletionResponse
	err := c.doJSONRequest(ctx, "POST", "/chat/completions", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
