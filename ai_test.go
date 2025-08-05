package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/liuzl/ai"
)

// TestSimpleChat tests the basic text generation functionality.
func TestSimpleChat(t *testing.T) {
	client, model := setupTestClient(t)
	if client == nil {
		return // Skips test if client setup fails
	}

	req := &ai.Request{
		Model: model,
		Messages: []ai.Message{
			{Role: "user", Content: "Tell me a one-sentence joke about programming."},
		},
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if resp.Text == "" {
		t.Fatal("Expected a non-empty text response, but got an empty string.")
	}
	if len(resp.ToolCalls) > 0 {
		t.Fatalf("Expected no tool calls, but got %d", len(resp.ToolCalls))
	}

	fmt.Printf("\n--- Simple Chat Test ---\n")
	fmt.Printf("Provider: %s\n", os.Getenv("AI_PROVIDER"))
	fmt.Printf("Response: %s\n", resp.Text)
	fmt.Printf("------------------------\n")
}

// TestFunctionCalling tests the tool calling functionality.
func TestFunctionCalling(t *testing.T) {
	client, model := setupTestClient(t)
	if client == nil {
		return
	}

	// 1. Define the tool
	getCurrentWeatherTool := ai.Tool{
		Type: "function",
		Function: ai.FunctionDefinition{
			Name:        "get_current_weather",
			Description: "Get the current weather for a location",
			Parameters:  json.RawMessage(`{"type": "object", "properties": {"location": {"type": "string", "description": "The city and state, e.g. San Francisco, CA"}}, "required": ["location"]}`),
		},
	}

	// 2. Initial request asking the model to use the tool
	messages := []ai.Message{{Role: "user", Content: "What is the weather like in Boston, MA?"}}
	req := &ai.Request{
		Model:    model,
		Messages: messages,
		Tools:    []ai.Tool{getCurrentWeatherTool},
	}

	// 3. First call to the model
	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	if len(resp.ToolCalls) == 0 {
		t.Fatalf("Expected tool calls, but got none. Response text: %s", resp.Text)
	}

	// 4. Append the assistant's request to the message history
	messages = append(messages, ai.Message{Role: "assistant", ToolCalls: resp.ToolCalls})

	// 5. Execute the tool and append the result
	toolCall := resp.ToolCalls[0]
	if toolCall.Function != "get_current_weather" {
		t.Fatalf("Expected function call to 'get_current_weather', but got '%s'", toolCall.Function)
	}
	
	// Mock the function execution
	weatherData := `{"temperature": "22", "unit": "celsius"}`
	messages = append(messages, ai.Message{
		Role:       "tool",
		ToolCallID: toolCall.ID,
		Content:    weatherData,
	})

	// 6. Second call to the model with the tool result
	finalReq := &ai.Request{
		Model:    model,
		Messages: messages,
	}
	finalResp, err := client.Generate(context.Background(), finalReq)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	if finalResp.Text == "" {
		t.Fatal("Expected a final text response, but got an empty string.")
	}
	if !strings.Contains(finalResp.Text, "22") {
		t.Errorf("Expected final response to contain the weather data '22', but it didn't. Got: %s", finalResp.Text)
	}

	fmt.Printf("\n--- Function Calling Test ---\n")
	fmt.Printf("Provider: %s\n", os.Getenv("AI_PROVIDER"))
	fmt.Printf("Final Response: %s\n", finalResp.Text)
	fmt.Printf("---------------------------\n")
}

// setupTestClient is a helper function to initialize the client for tests.
func setupTestClient(t *testing.T) (ai.Client, string) {
	t.Helper()
	if err := godotenv.Load(); err != nil {
		t.Log("No .env file found, reading from environment variables")
	}

	provider := os.Getenv("AI_PROVIDER")
	if provider == "" {
		provider = "openai" // Default to openai
	}

	var apiKey, model, baseURL string
	switch provider {
	case "openai":
		apiKey = os.Getenv("OPENAI_API_KEY")
		model = os.Getenv("OPENAI_MODEL")
		baseURL = os.Getenv("OPENAI_BASE_URL")
	case "gemini":
		apiKey = os.Getenv("GEMINI_API_KEY")
		model = os.Getenv("GEMINI_MODEL")
		baseURL = os.Getenv("GEMINI_BASE_URL")
	default:
		t.Fatalf("Unsupported AI_PROVIDER: %s", provider)
	}

	if apiKey == "" {
		t.Skipf("API key for %s not set, skipping test", provider)
		return nil, ""
	}

	client, err := ai.NewClient(
		ai.WithProvider(provider),
		ai.WithAPIKey(apiKey),
		ai.WithBaseURL(baseURL),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	return client, model
}
