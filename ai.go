package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// --- Public API ---

// AIClient is the unified interface for different AI providers.
// It abstracts the underlying implementation of each provider.
type AIClient interface {
	GenerateUniversalContent(ctx context.Context, req *ContentRequest) (*ContentResponse, error)
}

// ContentRequest is a universal request structure for content generation.
type ContentRequest struct {
	Model    string
	Messages []Message
	Tools    []Tool // Defines tools available to the model
}

// ContentResponse is a universal response structure.
// A response can have EITHER text content OR tool calls, but not both.
type ContentResponse struct {
	Text      string
	ToolCalls []ToolCall // The list of tools the model wants to call
}

// Message represents a universal message structure.
type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall // Used when an assistant message contains tool call requests
	ToolCallID string     // Used when a message is a result of a tool call
}

// Tool defines a tool the model can use.
type Tool struct {
	Type     string             `json:"type"` // e.g., "function"
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition is a universal, provider-agnostic function definition.
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"` // A JSON Schema object
}

// ToolCall represents a request from the model to call a specific tool.
type ToolCall struct {
	ID        string // A unique ID for this tool call, needed to send back the result
	Type      string // e.g., "function"
	Function  string // The name of the function to call
	Arguments string // The arguments as a JSON string
}

// config holds all possible configuration options for any client.
type config struct {
	provider   string
	apiKey     string
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// Option is the function signature for configuration options.
type Option func(*config)

// WithProvider sets the AI provider (e.g., "openai", "gemini"). This is required.
func WithProvider(provider string) Option {
	return func(c *config) {
		c.provider = provider
	}
}

// WithAPIKey sets the API key for authentication. This is required.
func WithAPIKey(apiKey string) Option {
	return func(c *config) {
		c.apiKey = apiKey
	}
}

// WithBaseURL sets a custom base URL for the API endpoint.
func WithBaseURL(baseURL string) Option {
	return func(c *config) {
		c.baseURL = baseURL
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *config) {
		c.timeout = timeout
	}
}

// NewClient is the single, unified factory function to create an AI client.
func NewClient(opts ...Option) (AIClient, error) {
	cfg := &config{
		timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.provider == "" {
		return nil, fmt.Errorf("provider is required, use WithProvider()")
	}
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("API key is required, use WithAPIKey()")
	}

	switch cfg.provider {
	case "openai":
		return newOpenAIClient(cfg), nil
	case "gemini":
		return newGeminiClient(cfg), nil
	default:
		return nil, fmt.Errorf("unknown provider: %q", cfg.provider)
	}
}
