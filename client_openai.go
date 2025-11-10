package ai

import (
	"net/http"
)

// newOpenAIClient is the internal constructor for the OpenAI client.
// It now sets up the generic client with the OpenAI-specific adapter.
func newOpenAIClient(cfg *Config) Client {
	baseURL := "https://api.openai.com"
	if cfg.baseURL != "" {
		baseURL = cfg.baseURL
	}
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+cfg.apiKey)

	return &genericClient{
		b:       newBaseClient(string(ProviderOpenAI), baseURL, "v1", cfg.timeout, headers, 3),
		adapter: &openaiAdapter{},
	}
}
