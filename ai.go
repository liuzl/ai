package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// --- Public API ---

// Provider defines the supported AI providers.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderGemini    Provider = "gemini"
	ProviderAnthropic Provider = "anthropic"
)

// Client is the unified interface for different AI providers.
type Client interface {
	Generate(ctx context.Context, req *Request) (*Response, error)
}

// Request is a universal request structure for content generation.
type Request struct {
	Model        string
	SystemPrompt string
	Messages     []Message
	Tools        []Tool
}

// Response is a universal response structure.
type Response struct {
	Text      string
	ToolCalls []ToolCall
}

// Role defines the originator of a message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message represents a universal message structure.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

// Tool defines a tool the model can use.
type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition is a universal, provider-agnostic function definition.
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a request from the model to call a specific tool.
type ToolCall struct {
	ID        string
	Type      string
	Function  string
	Arguments string
}

// Config holds all possible Configuration options for any client.
type Config struct {
	provider Provider
	apiKey   string
	baseURL  string
	model    string // Added model to the config
	timeout  time.Duration
}

// Option is the function signature for Configuration options.
type Option func(*Config)

// WithProvider sets the AI provider.
func WithProvider(provider Provider) Option {
	return func(c *Config) { c.provider = provider }
}

// WithAPIKey sets the API key for authentication.
func WithAPIKey(apiKey string) Option {
	return func(c *Config) { c.apiKey = apiKey }
}

// WithBaseURL sets a custom base URL for the API endpoint.
func WithBaseURL(baseURL string) Option {
	return func(c *Config) { c.baseURL = baseURL }
}

// WithModel sets the model name to use for the client.
func WithModel(model string) Option {
	return func(c *Config) { c.model = model }
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) { c.timeout = timeout }
}

// validateConfig validates the client configuration and returns an error if invalid.
func validateConfig(cfg *Config) error {
	// Validate provider
	if cfg.provider == "" {
		return fmt.Errorf("provider is required, use WithProvider()")
	}

	// Validate provider is supported
	switch cfg.provider {
	case ProviderOpenAI, ProviderGemini, ProviderAnthropic:
		// Valid provider
	default:
		return fmt.Errorf("unsupported provider: %q (supported: openai, gemini, anthropic)", cfg.provider)
	}

	// Validate API key
	if cfg.apiKey == "" {
		return fmt.Errorf("API key is required for provider %q, use WithAPIKey()", cfg.provider)
	}
	if strings.TrimSpace(cfg.apiKey) == "" {
		return fmt.Errorf("API key cannot be empty or whitespace only")
	}

	// Validate timeout
	if cfg.timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", cfg.timeout)
	}

	// Validate baseURL if provided
	if cfg.baseURL != "" {
		if strings.TrimSpace(cfg.baseURL) == "" {
			return fmt.Errorf("baseURL cannot be empty or whitespace only")
		}
		parsedURL, err := url.Parse(cfg.baseURL)
		if err != nil {
			return fmt.Errorf("invalid baseURL: %w", err)
		}
		if parsedURL.Scheme == "" {
			return fmt.Errorf("baseURL must include scheme (http:// or https://), got: %q", cfg.baseURL)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("baseURL scheme must be http or https, got: %q", parsedURL.Scheme)
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("baseURL must include host, got: %q", cfg.baseURL)
		}
	}

	// Validate model if provided
	if cfg.model != "" && strings.TrimSpace(cfg.model) == "" {
		return fmt.Errorf("model cannot be empty or whitespace only")
	}

	return nil
}

// NewClient is the single, unified factory function to create an AI client.
func NewClient(opts ...Option) (Client, error) {
	cfg := &Config{timeout: 30 * time.Second}
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	switch cfg.provider {
	case ProviderOpenAI:
		return newOpenAIClient(cfg), nil
	case ProviderGemini:
		return newGeminiClient(cfg), nil
	case ProviderAnthropic:
		return newAnthropicClient(cfg), nil
	default:
		// This should never happen due to validateConfig, but keep for safety
		return nil, fmt.Errorf("unknown provider: %q", cfg.provider)
	}
}

// providerEnvConfig holds the environment variable names for a specific provider.
type providerEnvConfig struct {
	apiKey  string
	model   string
	baseURL string
}

// providerEnvs maps each provider to its corresponding environment variable configuration.
var providerEnvs = map[Provider]providerEnvConfig{
	ProviderOpenAI:    {"OPENAI_API_KEY", "OPENAI_MODEL", "OPENAI_BASE_URL"},
	ProviderGemini:    {"GEMINI_API_KEY", "GEMINI_MODEL", "GEMINI_BASE_URL"},
	ProviderAnthropic: {"ANTHROPIC_API_KEY", "ANTHROPIC_MODEL", "ANTHROPIC_BASE_URL"},
}

// NewClientFromEnv creates a new AI client by reading configuration from
// environment variables. It provides a convenient way to initialize the client
// without manual configuration.
//
// It uses the following environment variables:
//   - AI_PROVIDER: "openai" or "gemini" (defaults to "openai").
//   - OPENAI_API_KEY, OPENAI_MODEL, OPENAI_BASE_URL
//   - GEMINI_API_KEY, GEMINI_MODEL, GEMINI_BASE_URL
func NewClientFromEnv() (Client, error) {
	providerStr := os.Getenv("AI_PROVIDER")
	if providerStr == "" {
		providerStr = "openai" // Default to openai
	}
	provider := Provider(strings.ToLower(providerStr))

	env, ok := providerEnvs[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported AI_PROVIDER: %s", provider)
	}

	apiKey := os.Getenv(env.apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("API key for provider '%s' is not set in env var %s", provider, env.apiKey)
	}

	model := os.Getenv(env.model)
	baseURL := os.Getenv(env.baseURL)

	var opts []Option
	opts = append(opts, WithProvider(provider))
	opts = append(opts, WithAPIKey(apiKey))
	if model != "" {
		opts = append(opts, WithModel(model))
	}
	if baseURL != "" {
		opts = append(opts, WithBaseURL(baseURL))
	}
	// Add default 5 minute timeout
	opts = append(opts, WithTimeout(5*time.Minute))

	return NewClient(opts...)
}
