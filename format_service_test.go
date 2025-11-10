package ai_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/liuzl/ai"
)

func TestFormatConversion(t *testing.T) {
	testCases := []struct {
		name              string
		provider          string
		requestPayload    []byte
		expectedRequest   *ai.Request
		responseToConvert *ai.Response
		validateResponse  func(t *testing.T, respBytes []byte)
	}{
		{
			name:     "OpenAI",
			provider: "openai",
			requestPayload: []byte(`{
				"model": "gpt-4o",
				"messages": [
					{"role": "user", "content": "Hello, how are you?"},
					{"role": "assistant", "content": "I'm doing well!", "tool_calls": [
						{"id": "call_123", "type": "function", "function": {"name": "get_weather", "arguments": "{\"location\":\"Boston\"}"}}
					]}
				],
				"tools": [
					{"type": "function", "function": {"name": "get_weather", "description": "Get weather information", "parameters": {"type": "object", "properties": {"location": {"type": "string"}}}}}
				]
			}`),
			expectedRequest: &ai.Request{
				Model: "gpt-4o",
				Messages: []ai.Message{
					{Role: "user", Content: "Hello, how are you?"},
					{Role: "assistant", Content: "I'm doing well!", ToolCalls: []ai.ToolCall{
						{ID: "call_123", Type: "function", Function: "get_weather", Arguments: "{\"location\":\"Boston\"}"},
					}},
				},
				Tools: []ai.Tool{
					{Type: "function", Function: ai.FunctionDefinition{
						Name:        "get_weather",
						Description: "Get weather information",
						Parameters:  json.RawMessage(`{"type": "object", "properties": {"location": {"type": "string"}}}`),
					}},
				},
			},
			responseToConvert: &ai.Response{
				ToolCalls: []ai.ToolCall{
					{ID: "call_123", Type: "function", Function: "get_weather", Arguments: "{\"location\":\"Boston\"}"},
				},
			},
			validateResponse: func(t *testing.T, respBytes []byte) {
				var resp struct {
					Choices []struct {
						Message struct {
							ToolCalls []struct {
								ID       string `json:"id"`
								Function struct {
									Name string `json:"name"`
								} `json:"function"`
							} `json:"tool_calls"`
						} `json:"message"`
					} `json:"choices"`
				}
				if err := json.Unmarshal(respBytes, &resp); err != nil {
					t.Fatalf("failed to unmarshal OpenAI response: %v", err)
				}
				if len(resp.Choices) != 1 {
					t.Fatal("expected 1 choice in response")
				}
				if len(resp.Choices[0].Message.ToolCalls) != 1 {
					t.Fatal("expected 1 tool_call in response")
				}
				if resp.Choices[0].Message.ToolCalls[0].ID != "call_123" {
					t.Errorf("expected tool call ID 'call_123', got '%s'", resp.Choices[0].Message.ToolCalls[0].ID)
				}
			},
		},
		{
			name:     "Gemini",
			provider: "gemini",
			requestPayload: []byte(`{
				"contents": [
					{"role": "user", "parts": [{"text": "What's the weather like?"}]},
					{"role": "model", "parts": [{"functionCall": {"name": "get_weather", "args": {"location": "New York"}}}]}
				]
			}`),
			expectedRequest: &ai.Request{
				Messages: []ai.Message{
					{Role: "user", Content: "What's the weather like?"},
					{Role: "model", ToolCalls: []ai.ToolCall{
						{Function: "get_weather", Arguments: "{\"location\":\"New York\"}"},
					}},
				},
			},
			responseToConvert: &ai.Response{
				Text: "The weather is sunny",
			},
			validateResponse: func(t *testing.T, respBytes []byte) {
				var resp struct {
					Candidates []struct {
						Content struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
						} `json:"content"`
					} `json:"candidates"`
				}
				if err := json.Unmarshal(respBytes, &resp); err != nil {
					t.Fatalf("failed to unmarshal Gemini response: %v", err)
				}
				if len(resp.Candidates) != 1 {
					t.Fatal("expected 1 candidate in response")
				}
				if len(resp.Candidates[0].Content.Parts) != 1 {
					t.Fatal("expected 1 part in response")
				}
				if resp.Candidates[0].Content.Parts[0].Text != "The weather is sunny" {
					t.Errorf("expected text 'The weather is sunny', got '%s'", resp.Candidates[0].Content.Parts[0].Text)
				}
			},
		},
		{
			name:     "Anthropic",
			provider: "anthropic",
			requestPayload: []byte(`{
				"model": "claude-3-haiku",
				"messages": [
					{"role": "user", "content": [{"type": "text", "text": "Hello Claude"}]},
					{"role": "assistant", "content": [{"type": "tool_use", "id": "toolu_01", "name": "search_web", "input": {"query": "weather"}}]}
				],
				"max_tokens": 100
			}`),
			expectedRequest: &ai.Request{
				Model: "claude-3-haiku",
				Messages: []ai.Message{
					{Role: "user", Content: "Hello Claude"},
					{Role: "assistant", ToolCalls: []ai.ToolCall{
						{ID: "toolu_01", Function: "search_web", Arguments: "{\"query\":\"weather\"}"},
					}},
				},
			},
			responseToConvert: &ai.Response{
				Text: "I found weather information",
			},
			validateResponse: func(t *testing.T, respBytes []byte) {
				var resp struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				}
				if err := json.Unmarshal(respBytes, &resp); err != nil {
					t.Fatalf("failed to unmarshal Anthropic response: %v", err)
				}
				if len(resp.Content) != 1 {
					t.Fatal("expected 1 content block in response")
				}
				if resp.Content[0].Type != "text" {
					t.Errorf("expected content type 'text', got '%s'", resp.Content[0].Type)
				}
				if resp.Content[0].Text != "I found weather information" {
					t.Errorf("expected text 'I found weather information', got '%s'", resp.Content[0].Text)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test ConvertRequest (Provider -> Universal)
			req, err := ai.ConvertRequest(tc.provider, tc.requestPayload)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}

			if !reflect.DeepEqual(req, tc.expectedRequest) {
				// A more detailed comparison for debugging
				if req.Model != tc.expectedRequest.Model {
					t.Errorf("Model mismatch: got %s, want %s", req.Model, tc.expectedRequest.Model)
				}
				if len(req.Messages) != len(tc.expectedRequest.Messages) {
					t.Fatalf("Messages length mismatch: got %d, want %d", len(req.Messages), len(tc.expectedRequest.Messages))
				}
				for i := range req.Messages {
					if !reflect.DeepEqual(req.Messages[i], tc.expectedRequest.Messages[i]) {
						t.Errorf("Message %d mismatch:\nGot:  %+v\nWant: %+v", i, req.Messages[i], tc.expectedRequest.Messages[i])
					}
				}
				if len(req.Tools) != len(tc.expectedRequest.Tools) {
					t.Fatalf("Tools length mismatch: got %d, want %d", len(req.Tools), len(tc.expectedRequest.Tools))
				}
				for i := range req.Tools {
					if !reflect.DeepEqual(req.Tools[i], tc.expectedRequest.Tools[i]) {
						t.Errorf("Tool %d mismatch:\nGot:  %+v\nWant: %+v", i, req.Tools[i], tc.expectedRequest.Tools[i])
					}
				}
				t.Fatalf("Converted request does not match expected request.")
			}

			// Test ConvertResponse (Universal -> Provider)
			respBytes, err := ai.ConvertResponse(tc.provider, tc.responseToConvert)
			if err != nil {
				t.Fatalf("ConvertResponse failed: %v", err)
			}
			if tc.validateResponse != nil {
				tc.validateResponse(t, respBytes)
			}
		})
	}
}

func TestInvalidFormat(t *testing.T) {
	t.Run("Invalid source format", func(t *testing.T) {
		_, err := ai.ConvertRequest("invalid", []byte("{}"))
		if err == nil {
			t.Error("Expected error for invalid source format, got nil")
		}
	})

	t.Run("Invalid target format", func(t *testing.T) {
		resp := &ai.Response{Text: "test"}
		_, err := ai.ConvertResponse("invalid", resp)
		if err == nil {
			t.Error("Expected error for invalid target format, got nil")
		}
	})
}
