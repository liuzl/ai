package ai

// FormatConverter defines the interface for converting between provider-specific
// API formats and the Universal format. This enables creating proxy servers that
// accept one provider's API format and route to any other provider.
type FormatConverter interface {
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
