package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/liuzl/ai"
)

// ClientPool manages a pool of AI clients with thread-safe access
type ClientPool struct {
	mu      sync.RWMutex
	clients map[string]ai.Client // key: provider name
}

// NewClientPool creates a new empty client pool
func NewClientPool() *ClientPool {
	return &ClientPool{
		clients: make(map[string]ai.Client),
	}
}

// GetClient retrieves or creates a client for the specified provider
// This method is thread-safe and uses double-checked locking for efficiency
func (p *ClientPool) GetClient(provider ai.Provider) (ai.Client, error) {
	key := string(provider)

	// Fast path: read lock
	p.mu.RLock()
	client, exists := p.clients[key]
	p.mu.RUnlock()

	if exists {
		return client, nil
	}

	// Slow path: create client
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check: another goroutine might have created it
	if client, exists := p.clients[key]; exists {
		return client, nil
	}

	// Create new client from environment variables
	client, err := createClientFromEnv(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for %s: %w", provider, err)
	}

	// Cache the client
	p.clients[key] = client
	return client, nil
}

// GetStreamingClient retrieves a streaming-capable client
func (p *ClientPool) GetStreamingClient(provider ai.Provider) (ai.StreamingClient, error) {
	client, err := p.GetClient(provider)
	if err != nil {
		return nil, err
	}

	streamingClient, ok := client.(ai.StreamingClient)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support streaming", provider)
	}

	return streamingClient, nil
}

// createClientFromEnv creates an AI client from environment variables
func createClientFromEnv(provider ai.Provider) (ai.Client, error) {
	var apiKey, baseURL string

	// Get provider-specific environment variables
	switch provider {
	case ai.ProviderOpenAI:
		apiKey = os.Getenv("OPENAI_API_KEY")
		baseURL = os.Getenv("OPENAI_BASE_URL")
	case ai.ProviderGemini:
		apiKey = os.Getenv("GEMINI_API_KEY")
		baseURL = os.Getenv("GEMINI_BASE_URL")
	case ai.ProviderAnthropic:
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		baseURL = os.Getenv("ANTHROPIC_BASE_URL")
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	// Check if API key is set
	if apiKey == "" {
		return nil, fmt.Errorf("API key not set for provider %s (set %s_API_KEY environment variable)",
			provider, provider)
	}

	// Build client options
	opts := []ai.Option{
		ai.WithProvider(provider),
		ai.WithAPIKey(apiKey),
		ai.WithTimeout(5 * time.Minute),
	}

	// Add base URL if specified
	if baseURL != "" {
		opts = append(opts, ai.WithBaseURL(baseURL))
	}

	// Create and return the client
	return ai.NewClient(opts...)
}
