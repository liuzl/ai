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

// Validate checks if the request is valid and returns an error if not.
// This method validates all request fields before sending to the API.
func (r *Request) Validate() error {
	// Check messages is not empty
	if len(r.Messages) == 0 {
		return fmt.Errorf("request must have at least one message")
	}

	// Validate each message
	for i, msg := range r.Messages {
		// Check role is valid
		switch msg.Role {
		case RoleSystem, RoleUser, RoleAssistant, RoleTool:
			// Valid role
		default:
			return fmt.Errorf("message[%d]: invalid role %q (must be system, user, assistant, or tool)", i, msg.Role)
		}

		// Check message has content or tool calls
		hasContent := msg.Content != "" || len(msg.ContentParts) > 0
		hasToolCalls := len(msg.ToolCalls) > 0

		if !hasContent && !hasToolCalls {
			return fmt.Errorf("message[%d]: must have either content or tool calls", i)
		}

		// Tool role messages must have ToolCallID
		if msg.Role == RoleTool && msg.ToolCallID == "" {
			return fmt.Errorf("message[%d]: tool role message must have tool_call_id", i)
		}

		// Validate content parts if present
		for j, part := range msg.ContentParts {
			switch part.Type {
			case ContentTypeText:
				if strings.TrimSpace(part.Text) == "" {
					return fmt.Errorf("message[%d].content_parts[%d]: text content cannot be empty", i, j)
				}
			case ContentTypeImage:
				if part.ImageSource == nil {
					return fmt.Errorf("message[%d].content_parts[%d]: image part must have image source", i, j)
				}
				if err := validateImageSource(part.ImageSource, i, j); err != nil {
					return err
				}
			default:
				return fmt.Errorf("message[%d].content_parts[%d]: invalid content type %q", i, j, part.Type)
			}
		}

		// Validate tool calls
		for j, tc := range msg.ToolCalls {
			if strings.TrimSpace(tc.Function) == "" {
				return fmt.Errorf("message[%d].tool_calls[%d]: function name cannot be empty", i, j)
			}
			if strings.TrimSpace(tc.Arguments) == "" {
				return fmt.Errorf("message[%d].tool_calls[%d]: arguments cannot be empty", i, j)
			}
			// Validate arguments is valid JSON
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
				return fmt.Errorf("message[%d].tool_calls[%d]: invalid JSON arguments: %w", i, j, err)
			}
		}
	}

	// Validate tools if present
	for i, tool := range r.Tools {
		if strings.TrimSpace(tool.Type) == "" {
			return fmt.Errorf("tools[%d]: type cannot be empty", i)
		}
		if strings.TrimSpace(tool.Function.Name) == "" {
			return fmt.Errorf("tools[%d]: function name cannot be empty", i)
		}
		if len(tool.Function.Parameters) == 0 {
			return fmt.Errorf("tools[%d]: function parameters cannot be empty", i)
		}
		// Validate parameters is valid JSON
		var params map[string]any
		if err := json.Unmarshal(tool.Function.Parameters, &params); err != nil {
			return fmt.Errorf("tools[%d]: invalid JSON parameters: %w", i, err)
		}
	}

	// Model validation (if specified)
	if r.Model != "" && strings.TrimSpace(r.Model) == "" {
		return fmt.Errorf("model cannot be whitespace only")
	}

	return nil
}

// validateImageSource validates an image source
func validateImageSource(src *ImageSource, msgIdx, partIdx int) error {
	switch src.Type {
	case ImageSourceTypeURL:
		if strings.TrimSpace(src.URL) == "" {
			return fmt.Errorf("message[%d].content_parts[%d]: image URL cannot be empty", msgIdx, partIdx)
		}
	case ImageSourceTypeBase64:
		if strings.TrimSpace(src.Data) == "" {
			return fmt.Errorf("message[%d].content_parts[%d]: image data cannot be empty", msgIdx, partIdx)
		}
	default:
		return fmt.Errorf("message[%d].content_parts[%d]: invalid image source type %q", msgIdx, partIdx, src.Type)
	}
	return nil
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

// ContentType defines the type of content in a multimodal message.
type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeImage ContentType = "image"
)

// ContentPart represents a part of multimodal content (text, image, etc.).
type ContentPart struct {
	Type        ContentType
	Text        string       // For text parts
	ImageSource *ImageSource // For image parts
}

// ImageSourceType defines how an image is provided.
type ImageSourceType string

const (
	ImageSourceTypeURL    ImageSourceType = "url"
	ImageSourceTypeBase64 ImageSourceType = "base64"
)

// ImageSource represents an image input for vision-enabled models.
type ImageSource struct {
	Type   ImageSourceType // "url" or "base64"
	URL    string          // HTTP(S) URL to the image
	Data   string          // Base64-encoded image data (with or without data URI prefix)
	Format string          // Image format: "png", "jpeg", "gif", "webp" (optional, can be auto-detected)
}

// Message represents a universal message structure.
// Supports both simple text messages (Content) and multimodal messages (ContentParts).
type Message struct {
	Role         Role
	Content      string        // Simple text content (for backward compatibility)
	ContentParts []ContentPart // Multimodal content (text + images, etc.)
	ToolCalls    []ToolCall
	ToolCallID   string
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

// --- Multimodal Helper Functions ---

// NewTextMessage creates a simple text message.
// This is a convenience function for backward compatibility.
func NewTextMessage(role Role, text string) Message {
	return Message{
		Role:    role,
		Content: text,
	}
}

// NewMultimodalMessage creates a message with multimodal content parts.
func NewMultimodalMessage(role Role, parts []ContentPart) Message {
	return Message{
		Role:         role,
		ContentParts: parts,
	}
}

// NewTextPart creates a text content part.
func NewTextPart(text string) ContentPart {
	return ContentPart{
		Type: ContentTypeText,
		Text: text,
	}
}

// NewImagePartFromURL creates an image content part from a URL.
func NewImagePartFromURL(url string) ContentPart {
	return ContentPart{
		Type: ContentTypeImage,
		ImageSource: &ImageSource{
			Type: ImageSourceTypeURL,
			URL:  url,
		},
	}
}

// NewImagePartFromBase64 creates an image content part from base64-encoded data.
// The data parameter should be the base64-encoded image data.
// The format parameter specifies the image format (e.g., "png", "jpeg", "gif", "webp").
// If format is empty, it will be auto-detected from the data URI prefix if present.
func NewImagePartFromBase64(data, format string) ContentPart {
	return ContentPart{
		Type: ContentTypeImage,
		ImageSource: &ImageSource{
			Type:   ImageSourceTypeBase64,
			Data:   data,
			Format: format,
		},
	}
}
