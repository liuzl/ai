package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/liuzl/ai"
	"gopkg.in/yaml.v3"
)

// ProxyConfig represents the YAML configuration structure
type ProxyConfig struct {
	Version         string        `yaml:"version"`
	Models          []ModelConfig `yaml:"models"`
	DefaultProvider string        `yaml:"default_provider,omitempty"`
	Timeout         string        `yaml:"timeout,omitempty"`
}

// ModelConfig represents a single model configuration
type ModelConfig struct {
	Name        string `yaml:"name"`
	Provider    string `yaml:"provider"` // "openai", "gemini", or "anthropic"
	Description string `yaml:"description,omitempty"`
}

// LoadConfig loads and parses the YAML configuration file
func LoadConfig(path string) (*ProxyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ProxyConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	if err := ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// ValidateConfig validates the configuration
func ValidateConfig(cfg *ProxyConfig) error {
	// Check version
	if cfg.Version == "" {
		return fmt.Errorf("version is required")
	}
	if cfg.Version != "1.0" {
		return fmt.Errorf("unsupported version: %s (supported: 1.0)", cfg.Version)
	}

	// Check models
	if len(cfg.Models) == 0 {
		return fmt.Errorf("at least one model must be configured")
	}

	// Validate each model and check for duplicates
	seen := make(map[string]bool)
	for i, model := range cfg.Models {
		// Check name
		if strings.TrimSpace(model.Name) == "" {
			return fmt.Errorf("models[%d]: name cannot be empty", i)
		}

		// Check for duplicate names
		if seen[model.Name] {
			return fmt.Errorf("models[%d]: duplicate model name: %s", i, model.Name)
		}
		seen[model.Name] = true

		// Check provider
		if strings.TrimSpace(model.Provider) == "" {
			return fmt.Errorf("models[%d]: provider cannot be empty for model %s", i, model.Name)
		}

		// Validate provider is supported
		provider := ai.Provider(model.Provider)
		switch provider {
		case ai.ProviderOpenAI, ai.ProviderGemini, ai.ProviderAnthropic:
			// Valid provider
		default:
			return fmt.Errorf("models[%d]: unsupported provider %q for model %s (supported: openai, gemini, anthropic)",
				i, model.Provider, model.Name)
		}
	}

	// Validate default provider if specified
	if cfg.DefaultProvider != "" {
		provider := ai.Provider(cfg.DefaultProvider)
		switch provider {
		case ai.ProviderOpenAI, ai.ProviderGemini, ai.ProviderAnthropic:
			// Valid provider
		default:
			return fmt.Errorf("unsupported default_provider: %q (supported: openai, gemini, anthropic)", cfg.DefaultProvider)
		}
	}

	return nil
}

// GetProviderForModel looks up the provider for a given model name
func (c *ProxyConfig) GetProviderForModel(model string) (ai.Provider, error) {
	for _, m := range c.Models {
		if m.Name == model {
			return ai.Provider(m.Provider), nil
		}
	}

	// Model not found - check if there's a default provider
	if c.DefaultProvider != "" {
		return ai.Provider(c.DefaultProvider), nil
	}

	return "", fmt.Errorf("unknown model: %s", model)
}

// GetModelNames returns a list of all configured model names
func (c *ProxyConfig) GetModelNames() []string {
	names := make([]string, len(c.Models))
	for i, m := range c.Models {
		names[i] = m.Name
	}
	return names
}

// GetProviders returns a set of all unique providers used in the configuration
func (c *ProxyConfig) GetProviders() []ai.Provider {
	providerSet := make(map[ai.Provider]bool)

	for _, m := range c.Models {
		providerSet[ai.Provider(m.Provider)] = true
	}

	// Add default provider if specified
	if c.DefaultProvider != "" {
		providerSet[ai.Provider(c.DefaultProvider)] = true
	}

	providers := make([]ai.Provider, 0, len(providerSet))
	for p := range providerSet {
		providers = append(providers, p)
	}

	return providers
}
