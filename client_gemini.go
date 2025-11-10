package ai

import (
	"net/http"
)

// newGeminiClient is the internal constructor for the Gemini client.
// It now sets up the generic client with the Gemini-specific adapter.
func newGeminiClient(cfg *Config) Client {
	baseURL := "https://generativelanguage.googleapis.com"
	if cfg.baseURL != "" {
		baseURL = cfg.baseURL
	}
	headers := make(http.Header)
	headers.Set("x-goog-api-key", cfg.apiKey)

	return &genericClient{
		b:       newBaseClient(string(ProviderGemini), baseURL, "v1beta", cfg.timeout, headers, 3),
		adapter: &geminiAdapter{},
	}
}
