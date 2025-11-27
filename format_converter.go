package ai

import (
	"net/http"
)

// FormatConverter defines the interface for converting between provider-specific
// API formats and the Universal format. This enables creating proxy servers that
// accept one provider's API format and route to any other provider.
type FormatConverter interface {
	// DecodeRequest decodes the request body into the provider-specific request struct.
	// It reads from the HTTP request (body, headers, URL).
	DecodeRequest(r *http.Request) (any, error)

	// IsStreaming checks if the decoded request indicates a streaming response.
	IsStreaming(providerReq any) bool

	// NewStreamHandler creates a handler for formatting streaming events.
	NewStreamHandler(id string, model string) StreamEventHandler

	// ConvertRequestFromFormat converts a provider-specific request format to Universal Request.
	// The input should be the unmarshaled JSON request body from the provider's API.
	ConvertRequestFromFormat(providerReq any) (*Request, error)

	// ConvertResponseToFormat converts a Universal Response to provider-specific response format.
	// Returns the response struct that can be marshaled to JSON for the provider's API.
	ConvertResponseToFormat(universalResp *Response, originalModel string) (any, error)

	// GetEndpoint returns the API endpoint path for this format (e.g., "/v1/chat/completions", "/v1/messages").
	GetEndpoint() string

	// GetProviderName returns the provider name for this format (e.g., "openai", "gemini", "anthropic").
	GetProviderName() string
}

// StreamEventHandler defines how to format stream events for a specific provider.
type StreamEventHandler interface {
	OnStart(w http.ResponseWriter, flusher http.Flusher)
	OnChunk(w http.ResponseWriter, flusher http.Flusher, chunk *StreamChunk) error
	OnEnd(w http.ResponseWriter, flusher http.Flusher)
	OnError(w http.ResponseWriter, flusher http.Flusher, err error)
}

// FormatConverterFactory creates format converters for different providers.
type FormatConverterFactory struct{}

// NewFormatConverterFactory creates a new format converter factory.
func NewFormatConverterFactory() *FormatConverterFactory {
	return &FormatConverterFactory{}
}

// GetConverter returns the appropriate format converter for the given provider.
func (f *FormatConverterFactory) GetConverter(provider Provider) (FormatConverter, error) {
	switch provider {
	case ProviderOpenAI:
		return NewOpenAIFormatConverter(), nil
	case ProviderGemini:
		return NewGeminiFormatConverter(), nil
	case ProviderAnthropic:
		return NewAnthropicFormatConverter(), nil
	default:
		return nil, NewInvalidRequestError("", "unsupported provider format: "+string(provider), "", nil)
	}
}
