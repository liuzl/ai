package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// --- Public API ---

// Client is the unified interface for different AI providers.
// It abstracts the underlying implementation of each provider.
type Client interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
}

// Request is a universal request structure for content generation.
type Request struct {
	Model    string
	Messages []Message
	Tools    []Tool // Defines tools available to the model
}

// Response is a universal response structure.
// A response can have EITHER text content OR tool calls, but not both.
type Response struct {
	Text      string
	ToolCalls []ToolCall // The list of tools the model wants to call
}

// Role defines the originator of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleModel     Role = "model" // For Gemini compatibility
)

// Message represents a universal message structure.
type Message struct {
	Role       Role
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

// Config holds all possible Configuration options for any client.
type Config struct {
	provider   string
	apiKey     string
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

// Option is the function signature for Configuration options.
type Option func(*Config)

// WithProvider sets the AI provider (e.g., "openai", "gemini"). This is required.
func WithProvider(provider string) Option {
	return func(c *Config) {
		c.provider = provider
	}
}

// WithAPIKey sets the API key for authentication. This is required.
func WithAPIKey(apiKey string) Option {
	return func(c *Config) {
		c.apiKey = apiKey
	}
}

// WithBaseURL sets a custom base URL for the API endpoint.
func WithBaseURL(baseURL string) Option {
	return func(c *Config) {
		c.baseURL = baseURL
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.timeout = timeout
	}
}

// NewClient is the single, unified factory function to create an AI client.
func NewClient(opts ...Option) (Client, error) {
	cfg := &Config{
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
