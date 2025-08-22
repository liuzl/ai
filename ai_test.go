package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liuzl/ai"
)

// TestSimpleChat tests the basic text generation functionality using a mock server.
func TestSimpleChat(t *testing.T) {
	testCases := []struct {
		name           string
		provider       ai.Provider
		mockResponse   string
		expectedText   string
		expectingError bool
	}{
		{
			name:         "OpenAI",
			provider:     ai.ProviderOpenAI,
			mockResponse: `{"choices": [{"message": {"role": "assistant", "content": "Why did the scarecrow win an award? Because he was outstanding in his field!"}}]}`,
			expectedText: "Why did the scarecrow win an award? Because he was outstanding in his field!",
		},
		{
			name:         "Gemini",
			provider:     ai.ProviderGemini,
			mockResponse: `{"candidates": [{"content": {"role": "model", "parts": [{"text": "It's hard to explain puns to kleptomaniacs because they always take things literally."}]}}]}`,
			expectedText: "It's hard to explain puns to kleptomaniacs because they always take things literally.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.mockResponse)
			}))
			defer server.Close()

			client, err := ai.NewClient(
				ai.WithProvider(tc.provider),
				ai.WithAPIKey("test-key"),
				ai.WithBaseURL(server.URL),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			req := &ai.Request{
				Messages: []ai.Message{{Role: ai.RoleUser, Content: "Tell me a joke."}},
			}

			resp, err := client.Generate(context.Background(), req)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			if resp.Text != tc.expectedText {
				t.Errorf("Expected response text '%s', but got '%s'", tc.expectedText, resp.Text)
			}
			if len(resp.ToolCalls) > 0 {
				t.Errorf("Expected no tool calls, but got %d", len(resp.ToolCalls))
			}
		})
	}
}

// TestFunctionCalling tests the tool calling functionality using a mock server.
func TestFunctionCalling(t *testing.T) {
	testCases := []struct {
		name                    string
		provider                ai.Provider
		mockToolCallResponse    string
		mockFinalResponse       string
		expectedFunctionName    string
		expectedFunctionArgs    string
		expectedFinalTextSubstr string
	}{
		{
			name:                    "OpenAI",
			provider:                ai.ProviderOpenAI,
			mockToolCallResponse:    `{"choices": [{"message": {"role": "assistant", "tool_calls": [{"id": "call_123", "type": "function", "function": {"name": "get_current_weather", "arguments": "{\"location\": \"Boston, MA\"}"}}]}}]}`,
			mockFinalResponse:       `{"choices": [{"message": {"role": "assistant", "content": "The weather in Boston is 22 degrees Celsius."}}]}`,
			expectedFunctionName:    "get_current_weather",
			expectedFunctionArgs:    `{"location": "Boston, MA"}`,
			expectedFinalTextSubstr: "22 degrees",
		},
		{
			name:                    "Gemini",
			provider:                ai.ProviderGemini,
			mockToolCallResponse:    `{"candidates": [{"content": {"role": "model", "parts": [{"functionCall": {"name": "get_current_weather", "args": {"location": "Boston, MA"}}}]}}]}`,
			mockFinalResponse:       `{"candidates": [{"content": {"role": "model", "parts": [{"text": "In Boston, it is currently 22 Celsius."}]}}]}`,
			expectedFunctionName:    "get_current_weather",
			expectedFunctionArgs:    `{"location":"Boston, MA"}`, // Note: JSON marshaling removes spaces
			expectedFinalTextSubstr: "22 Celsius",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if callCount == 0 {
					fmt.Fprint(w, tc.mockToolCallResponse)
				} else {
					fmt.Fprint(w, tc.mockFinalResponse)
				}
				callCount++
			}))
			defer server.Close()

			client, err := ai.NewClient(
				ai.WithProvider(tc.provider),
				ai.WithAPIKey("test-key"),
				ai.WithBaseURL(server.URL),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// 1. Initial request asking the model to use the tool
			messages := []ai.Message{{Role: ai.RoleUser, Content: "What is the weather like in Boston, MA?"}}
			tool := ai.Tool{
				Type: "function",
				Function: ai.FunctionDefinition{
					Name:       "get_current_weather",
					Parameters: json.RawMessage(`{"type": "object", "properties": {}}`),
				},
			}
			req := &ai.Request{Messages: messages, Tools: []ai.Tool{tool}}

			// 2. First call to the model (should return a tool call)
			resp, err := client.Generate(context.Background(), req)
			if err != nil {
				t.Fatalf("First call to Generate failed: %v", err)
			}
			if len(resp.ToolCalls) != 1 {
				t.Fatalf("Expected 1 tool call, but got %d. Response text: %s", len(resp.ToolCalls), resp.Text)
			}

			// 3. Verify the tool call
			toolCall := resp.ToolCalls[0]
			if toolCall.Function != tc.expectedFunctionName {
				t.Errorf("Expected function name '%s', got '%s'", tc.expectedFunctionName, toolCall.Function)
			}
			if toolCall.Arguments != tc.expectedFunctionArgs {
				t.Errorf("Expected function arguments '%s', got '%s'", tc.expectedFunctionArgs, toolCall.Arguments)
			}

			// 4. Second call to the model with the (mocked) tool result
			messages = append(messages, ai.Message{Role: ai.RoleAssistant, ToolCalls: resp.ToolCalls})
			messages = append(messages, ai.Message{
				Role:       ai.RoleTool,
				ToolCallID: toolCall.ID,
				Content:    `{"temperature": "22", "unit": "celsius"}`,
			})
			finalReq := &ai.Request{Messages: messages}

			finalResp, err := client.Generate(context.Background(), finalReq)
			if err != nil {
				t.Fatalf("Second call to Generate failed: %v", err)
			}

			// 5. Verify the final response
			if !strings.Contains(finalResp.Text, tc.expectedFinalTextSubstr) {
				t.Errorf("Expected final response to contain '%s', but got: %s", tc.expectedFinalTextSubstr, finalResp.Text)
			}
		})
	}
}

// TestSystemPrompt verifies that the system prompt is sent correctly for each provider.
func TestSystemPrompt(t *testing.T) {
	systemPrompt := "You are a helpful assistant."
	testCases := []struct {
		name     string
		provider ai.Provider
		verifier func(t *testing.T, r *http.Request)
	}{
		{
			name:     "OpenAI",
			provider: ai.ProviderOpenAI,
			verifier: func(t *testing.T, r *http.Request) {
				var reqBody map[string]interface{}
				body, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(body, &reqBody); err != nil {
					t.Fatalf("Failed to unmarshal request body: %v", err)
				}
				messages := reqBody["messages"].([]interface{})
				if len(messages) < 2 { // System + User
					t.Fatalf("Expected at least 2 messages, but got %d", len(messages))
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
			provider: ai.ProviderGemini,
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.verifier(t, r)
				w.Header().Set("Content-Type", "application/json")
				switch tc.provider {
				case ai.ProviderOpenAI:
					fmt.Fprint(w, `{"choices": [{"message": {"role": "assistant", "content": "OK"}}]}`)
				case ai.ProviderGemini:
					fmt.Fprint(w, `{"candidates": [{"content": {"role": "model", "parts": [{"text": "OK"}]}}]}`)
				}
			}))
			defer server.Close()

			client, err := ai.NewClient(
				ai.WithProvider(tc.provider),
				ai.WithAPIKey("test-key"),
				ai.WithBaseURL(server.URL),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			req := &ai.Request{
				SystemPrompt: systemPrompt,
				Messages:     []ai.Message{{Role: ai.RoleUser, Content: "Hello"}},
			}
			_, err = client.Generate(context.Background(), req)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}
		})
	}
}

// TestMultiToolFunctionCalling verifies that the Gemini client can handle a response
// containing multiple tool calls in a single turn.
func TestMultiToolFunctionCalling(t *testing.T) {
	mockResponse := `{
		"candidates": [{
			"content": {
				"role": "model",
				"parts": [
					{"functionCall": {"name": "get_weather", "args": {"location": "Boston, MA"}}},
					{"functionCall": {"name": "get_weather", "args": {"location": "New York, NY"}}}
				]
			}
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, mockResponse)
	}))
	defer server.Close()

	client, err := ai.NewClient(
		ai.WithProvider(ai.ProviderGemini),
		ai.WithAPIKey("test-key"),
		ai.WithBaseURL(server.URL),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &ai.Request{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "What's the weather in Boston and New York?"}},
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(resp.ToolCalls) != 2 {
		t.Fatalf("Expected 2 tool calls, but got %d", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].Function != "get_weather" {
		t.Errorf("Expected first tool call to be 'get_weather', got '%s'", resp.ToolCalls[0].Function)
	}
	if !strings.Contains(resp.ToolCalls[0].Arguments, "Boston, MA") {
		t.Errorf("Expected first tool call args to contain 'Boston, MA', got '%s'", resp.ToolCalls[0].Arguments)
	}

	if resp.ToolCalls[1].Function != "get_weather" {
		t.Errorf("Expected second tool call to be 'get_weather', got '%s'", resp.ToolCalls[1].Function)
	}
	if !strings.Contains(resp.ToolCalls[1].Arguments, "New York, NY") {
		t.Errorf("Expected second tool call args to contain 'New York, NY', got '%s'", resp.ToolCalls[1].Arguments)
	}

	if resp.ToolCalls[0].ID == resp.ToolCalls[1].ID {
		t.Errorf("Expected tool call IDs to be unique, but both were '%s'", resp.ToolCalls[0].ID)
	}
	if !strings.HasSuffix(resp.ToolCalls[0].ID, "-0") {
		t.Errorf("Expected first ID to end with '-0', got '%s'", resp.ToolCalls[0].ID)
	}
	if !strings.HasSuffix(resp.ToolCalls[1].ID, "-1") {
		t.Errorf("Expected second ID to end with '-1', got '%s'", resp.ToolCalls[1].ID)
	}
}

// TestAnthropicImplementation tests the Anthropic provider against a mock server,
// covering both simple chat and tool calling.
func TestAnthropicImplementation(t *testing.T) {
	mockSimpleResponse := `{
		"content": [{"type": "text", "text": "Hello! How can I help you today?"}],
		"stop_reason": "end_turn"
	}`
	mockToolCallResponse := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_01A09q90qw90lq917835lq9",
				"name": "get_stock_price",
				"input": {"ticker": "GOOG"}
			}
		],
		"stop_reason": "tool_use"
	}`

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("Expected x-api-key header was not found")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("Expected anthropic-version header was not found")
		}

		w.Header().Set("Content-Type", "application/json")
		if callCount == 0 {
			fmt.Fprint(w, mockSimpleResponse)
		} else {
			fmt.Fprint(w, mockToolCallResponse)
		}
		callCount++
	}))
	defer server.Close()

	client, err := ai.NewClient(
		ai.WithProvider(ai.ProviderAnthropic),
		ai.WithAPIKey("test-key"),
		ai.WithBaseURL(server.URL),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// --- Test 1: Simple Chat ---
	t.Run("Simple Chat", func(t *testing.T) {
		req := &ai.Request{
			Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hello"}},
		}
		resp, err := client.Generate(context.Background(), req)
		if err != nil {
			t.Fatalf("Simple chat generate failed: %v", err)
		}
		expectedText := "Hello! How can I help you today?"
		if resp.Text != expectedText {
			t.Errorf("Expected text '%s', got '%s'", expectedText, resp.Text)
		}
		if len(resp.ToolCalls) > 0 {
			t.Errorf("Expected no tool calls, but got %d", len(resp.ToolCalls))
		}
	})

	// --- Test 2: Tool Calling ---
	t.Run("Tool Calling", func(t *testing.T) {
		tool := ai.Tool{
			Type: "function",
			Function: ai.FunctionDefinition{
				Name:       "get_stock_price",
				Parameters: json.RawMessage(`{"type": "object", "properties": {"ticker": {"type": "string"}}}`),
			},
		}
		req := &ai.Request{
			Messages: []ai.Message{{Role: ai.RoleUser, Content: "What's the price of GOOG?"}},
			Tools:    []ai.Tool{tool},
		}
		resp, err := client.Generate(context.Background(), req)
		if err != nil {
			t.Fatalf("Tool call generate failed: %v", err)
		}
		if len(resp.ToolCalls) != 1 {
			t.Fatalf("Expected 1 tool call, got %d", len(resp.ToolCalls))
		}
		toolCall := resp.ToolCalls[0]
		if toolCall.Function != "get_stock_price" {
			t.Errorf("Expected function name 'get_stock_price', got '%s'", toolCall.Function)
		}
		expectedArgs := `{"ticker":"GOOG"}`
		if toolCall.Arguments != expectedArgs {
			t.Errorf("Expected args '%s', got '%s'", expectedArgs, toolCall.Arguments)
		}
		if toolCall.ID == "" {
			t.Error("Expected a non-empty tool call ID")
		}
	})
}
