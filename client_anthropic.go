package ai

import (
	"net/http"
)

// newAnthropicClient is the internal constructor for the Anthropic client.
func newAnthropicClient(cfg *Config) Client {
	baseURL := "https://api.anthropic.com"
	if cfg.baseURL != "" {
		baseURL = cfg.baseURL
	}
	headers := make(http.Header)
	headers.Set("x-api-key", cfg.apiKey)
	headers.Set("anthropic-version", "2023-06-01") // Required header

	return &genericClient{
		b:       newBaseClient(baseURL, "v1", cfg.timeout, headers, 3),
		adapter: &anthropicAdapter{},
	}
}
