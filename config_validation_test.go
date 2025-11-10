package ai_test

import (
	"strings"
	"testing"
	"time"

	"github.com/liuzl/ai"
)

// TestConfigValidation_Provider tests provider validation.
func TestConfigValidation_Provider(t *testing.T) {
	t.Run("missing provider", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithAPIKey("test-key"),
		)
		if err == nil {
			t.Fatal("Expected error for missing provider")
		}
		if !strings.Contains(err.Error(), "provider is required") {
			t.Errorf("Expected 'provider is required' error, got: %v", err)
		}
	})

	t.Run("unsupported provider", func(t *testing.T) {
		// This test uses a workaround since Provider type is constrained
		// In real usage, invalid providers would be caught at compile time
		_, err := ai.NewClient(
			ai.WithProvider(ai.Provider("invalid-provider")),
			ai.WithAPIKey("test-key"),
		)
		if err == nil {
			t.Fatal("Expected error for unsupported provider")
		}
		if !strings.Contains(err.Error(), "unsupported provider") {
			t.Errorf("Expected 'unsupported provider' error, got: %v", err)
		}
	})

	t.Run("valid providers", func(t *testing.T) {
		providers := []ai.Provider{
			ai.ProviderOpenAI,
			ai.ProviderGemini,
			ai.ProviderAnthropic,
		}
		for _, provider := range providers {
			_, err := ai.NewClient(
				ai.WithProvider(provider),
				ai.WithAPIKey("test-key"),
			)
			if err != nil {
				t.Errorf("Expected no error for provider %q, got: %v", provider, err)
			}
		}
	})
}

// TestConfigValidation_APIKey tests API key validation.
func TestConfigValidation_APIKey(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
		)
		if err == nil {
			t.Fatal("Expected error for missing API key")
		}
		if !strings.Contains(err.Error(), "API key is required") {
			t.Errorf("Expected 'API key is required' error, got: %v", err)
		}
	})

	t.Run("empty API key", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey(""),
		)
		if err == nil {
			t.Fatal("Expected error for empty API key")
		}
		if !strings.Contains(err.Error(), "API key is required") {
			t.Errorf("Expected 'API key is required' error, got: %v", err)
		}
	})

	t.Run("whitespace-only API key", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("   "),
		)
		if err == nil {
			t.Fatal("Expected error for whitespace-only API key")
		}
		if !strings.Contains(err.Error(), "cannot be empty or whitespace only") {
			t.Errorf("Expected 'whitespace only' error, got: %v", err)
		}
	})

	t.Run("valid API key", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("valid-key-123"),
		)
		if err != nil {
			t.Errorf("Expected no error for valid API key, got: %v", err)
		}
	})
}

// TestConfigValidation_Timeout tests timeout validation.
func TestConfigValidation_Timeout(t *testing.T) {
	t.Run("negative timeout", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithTimeout(-1*time.Second),
		)
		if err == nil {
			t.Fatal("Expected error for negative timeout")
		}
		if !strings.Contains(err.Error(), "timeout must be positive") {
			t.Errorf("Expected 'timeout must be positive' error, got: %v", err)
		}
	})

	t.Run("zero timeout", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithTimeout(0),
		)
		if err == nil {
			t.Fatal("Expected error for zero timeout")
		}
		if !strings.Contains(err.Error(), "timeout must be positive") {
			t.Errorf("Expected 'timeout must be positive' error, got: %v", err)
		}
	})

	t.Run("valid timeout", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithTimeout(10*time.Second),
		)
		if err != nil {
			t.Errorf("Expected no error for valid timeout, got: %v", err)
		}
	})

	t.Run("default timeout", func(t *testing.T) {
		// Default timeout should be valid
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
		)
		if err != nil {
			t.Errorf("Expected no error with default timeout, got: %v", err)
		}
	})
}

// TestConfigValidation_BaseURL tests baseURL validation.
func TestConfigValidation_BaseURL(t *testing.T) {
	t.Run("missing scheme", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithBaseURL("api.example.com"),
		)
		if err == nil {
			t.Fatal("Expected error for baseURL without scheme")
		}
		if !strings.Contains(err.Error(), "must include scheme") {
			t.Errorf("Expected 'must include scheme' error, got: %v", err)
		}
	})

	t.Run("invalid scheme", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithBaseURL("ftp://api.example.com"),
		)
		if err == nil {
			t.Fatal("Expected error for invalid scheme")
		}
		if !strings.Contains(err.Error(), "must be http or https") {
			t.Errorf("Expected 'must be http or https' error, got: %v", err)
		}
	})

	t.Run("missing host", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithBaseURL("https://"),
		)
		if err == nil {
			t.Fatal("Expected error for baseURL without host")
		}
		if !strings.Contains(err.Error(), "must include host") {
			t.Errorf("Expected 'must include host' error, got: %v", err)
		}
	})

	t.Run("whitespace-only baseURL", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithBaseURL("   "),
		)
		if err == nil {
			t.Fatal("Expected error for whitespace-only baseURL")
		}
		if !strings.Contains(err.Error(), "cannot be empty or whitespace only") {
			t.Errorf("Expected 'whitespace only' error, got: %v", err)
		}
	})

	t.Run("valid http URL", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithBaseURL("http://localhost:8080"),
		)
		if err != nil {
			t.Errorf("Expected no error for valid http URL, got: %v", err)
		}
	})

	t.Run("valid https URL", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithBaseURL("https://api.example.com"),
		)
		if err != nil {
			t.Errorf("Expected no error for valid https URL, got: %v", err)
		}
	})

	t.Run("valid URL with path", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithBaseURL("https://api.example.com/v1"),
		)
		if err != nil {
			t.Errorf("Expected no error for valid URL with path, got: %v", err)
		}
	})

	t.Run("no baseURL (optional)", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
		)
		if err != nil {
			t.Errorf("Expected no error when baseURL not provided, got: %v", err)
		}
	})
}

// TestConfigValidation_Model tests model validation.
func TestConfigValidation_Model(t *testing.T) {
	t.Run("whitespace-only model", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithModel("   "),
		)
		if err == nil {
			t.Fatal("Expected error for whitespace-only model")
		}
		if !strings.Contains(err.Error(), "model cannot be empty or whitespace only") {
			t.Errorf("Expected 'model whitespace only' error, got: %v", err)
		}
	})

	t.Run("valid model", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
			ai.WithModel("gpt-4"),
		)
		if err != nil {
			t.Errorf("Expected no error for valid model, got: %v", err)
		}
	})

	t.Run("no model (optional)", func(t *testing.T) {
		_, err := ai.NewClient(
			ai.WithProvider(ai.ProviderOpenAI),
			ai.WithAPIKey("test-key"),
		)
		if err != nil {
			t.Errorf("Expected no error when model not provided, got: %v", err)
		}
	})
}

// TestConfigValidation_Complete tests a complete valid configuration.
func TestConfigValidation_Complete(t *testing.T) {
	_, err := ai.NewClient(
		ai.WithProvider(ai.ProviderOpenAI),
		ai.WithAPIKey("sk-test-key-123"),
		ai.WithBaseURL("https://api.openai.com"),
		ai.WithModel("gpt-4"),
		ai.WithTimeout(60*time.Second),
	)
	if err != nil {
		t.Errorf("Expected no error for complete valid configuration, got: %v", err)
	}
}

// TestConfigValidation_MultipleErrors tests that the first error is returned.
func TestConfigValidation_MultipleErrors(t *testing.T) {
	// Missing provider and API key - should return provider error first
	_, err := ai.NewClient()
	if err == nil {
		t.Fatal("Expected error for invalid configuration")
	}
	// Should get provider error since it's checked first
	if !strings.Contains(err.Error(), "provider") {
		t.Errorf("Expected provider error, got: %v", err)
	}
}
