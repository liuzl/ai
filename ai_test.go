package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
			{Role: ai.RoleUser, Content: "Tell me a one-sentence joke about programming."},
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
	messages := []ai.Message{{Role: ai.RoleUser, Content: "What is the weather like in Boston, MA?"}}
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
	messages = append(messages, ai.Message{Role: ai.RoleAssistant, ToolCalls: resp.ToolCalls})

	// 5. Execute the tool and append the result
	toolCall := resp.ToolCalls[0]
	if toolCall.Function != "get_current_weather" {
		t.Fatalf("Expected function call to 'get_current_weather', but got '%s'", toolCall.Function)
	}

	// Mock the function execution
	weatherData := `{"temperature": "22", "unit": "celsius"}`
	messages = append(messages, ai.Message{
		Role:       ai.RoleTool,
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

// TestSystemPrompt verifies that the system prompt is sent correctly for each provider.
func TestSystemPrompt(t *testing.T) {
	systemPrompt := "You are a helpful assistant."
	testCases := []struct {
		name     string
		provider string
		verifier func(t *testing.T, r *http.Request)
	}{
		{
			name:     "OpenAI",
			provider: "openai",
			verifier: func(t *testing.T, r *http.Request) {
				var reqBody map[string]interface{}
				body, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(body, &reqBody); err != nil {
					t.Fatalf("Failed to unmarshal request body: %v", err)
				}
				messages := reqBody["messages"].([]interface{})
				if len(messages) == 0 {
					t.Fatal("Expected messages, but got none")
				}
				firstMessage := messages[0].(map[string]interface{})
				if firstMessage["role"] != "system" {
					t.Errorf("Expected first message role to be 'system', got '%s'", firstMessage["role"])
				}
				if firstMessage["content"] != systemPrompt {
					t.Errorf("Expected system prompt content to be '%s', got '%s'", systemPrompt, firstMessage["content"])
				}
			},
		},
		{
			name:     "Gemini",
			provider: "gemini",
			verifier: func(t *testing.T, r *http.Request) {
				var reqBody map[string]interface{}
				body, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(body, &reqBody); err != nil {
					t.Fatalf("Failed to unmarshal request body: %v", err)
				}
				sysInstruction, ok := reqBody["systemInstruction"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'systemInstruction' field in request body")
				}
				parts := sysInstruction["parts"].([]interface{})
				if len(parts) == 0 {
					t.Fatal("Expected parts in systemInstruction, but got none")
				}
				firstPart := parts[0].(map[string]interface{})
				if firstPart["text"] != systemPrompt {
					t.Errorf("Expected system prompt text to be '%s', got '%s'", systemPrompt, firstPart["text"])
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.verifier(t, r)
				// Send back a minimal valid response to prevent client errors
				w.Header().Set("Content-Type", "application/json")
				switch tc.provider {
				case "openai":
					fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "OK"}}]}`)
				case "gemini":
					fmt.Fprint(w, `{"candidates": [{"content": {"role": "model", "parts": [{"text": "OK"}]}}]}`)
				}
			}))
			defer server.Close()

			// Setup client to use the mock server
			client, err := ai.NewClient(
				ai.WithProvider(tc.provider),
				ai.WithAPIKey("test-key"),
				ai.WithBaseURL(server.URL),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// Make the request
			req := &ai.Request{
				SystemPrompt: systemPrompt,
				Messages: []ai.Message{
					{Role: ai.RoleUser, Content: "Hello"},
				},
			}
			_, err = client.Generate(context.Background(), req)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}
		})
	}
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
