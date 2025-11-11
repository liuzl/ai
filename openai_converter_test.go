package ai

import (
	"encoding/json"
	"testing"
)

func TestConvertRequestToUniversal(t *testing.T) {
	converter := NewOpenAIFormatConverter()

	tests := []struct {
		name        string
		openaiReq   *OpenAIChatCompletionRequest
		wantErr     bool
		checkResult func(*testing.T, *Request)
	}{
		{
			name: "simple text message",
			openaiReq: &OpenAIChatCompletionRequest{
				Model: "gpt-4",
				Messages: []openaiMessage{
					{
						Role:    "system",
						Content: "You are a helpful assistant.",
					},
					{
						Role:    "user",
						Content: "Hello!",
					},
				},
			},
			wantErr: false,
			checkResult: func(t *testing.T, req *Request) {
				if req.Model != "gpt-4" {
					t.Errorf("Expected model 'gpt-4', got '%s'", req.Model)
				}
				if req.SystemPrompt != "You are a helpful assistant." {
					t.Errorf("Expected system prompt to be set, got '%s'", req.SystemPrompt)
				}
				if len(req.Messages) != 1 {
					t.Errorf("Expected 1 message (system removed), got %d", len(req.Messages))
				}
				if req.Messages[0].Role != RoleUser {
					t.Errorf("Expected user role, got %s", req.Messages[0].Role)
				}
				if req.Messages[0].Content != "Hello!" {
					t.Errorf("Expected 'Hello!', got '%s'", req.Messages[0].Content)
				}
			},
		},
		{
			name: "multimodal message with image",
			openaiReq: &OpenAIChatCompletionRequest{
				Model: "gpt-4o",
				Messages: []openaiMessage{
					{
						Role: "user",
						Content: []any{
							map[string]any{
								"type": "text",
								"text": "What's in this image?",
							},
							map[string]any{
								"type": "image_url",
								"image_url": map[string]any{
									"url": "https://example.com/image.png",
								},
							},
						},
					},
				},
			},
			wantErr: false,
			checkResult: func(t *testing.T, req *Request) {
				if len(req.Messages) != 1 {
					t.Fatalf("Expected 1 message, got %d", len(req.Messages))
				}
				if len(req.Messages[0].ContentParts) != 2 {
					t.Fatalf("Expected 2 content parts, got %d", len(req.Messages[0].ContentParts))
				}
				if req.Messages[0].ContentParts[0].Type != ContentTypeText {
					t.Errorf("Expected text type, got %s", req.Messages[0].ContentParts[0].Type)
				}
				if req.Messages[0].ContentParts[0].Text != "What's in this image?" {
					t.Errorf("Unexpected text: %s", req.Messages[0].ContentParts[0].Text)
				}
				if req.Messages[0].ContentParts[1].Type != ContentTypeImage {
					t.Errorf("Expected image type, got %s", req.Messages[0].ContentParts[1].Type)
				}
				if req.Messages[0].ContentParts[1].ImageSource == nil {
					t.Fatal("Expected image source to be set")
				}
				if req.Messages[0].ContentParts[1].ImageSource.Type != ImageSourceTypeURL {
					t.Errorf("Expected URL type, got %s", req.Messages[0].ContentParts[1].ImageSource.Type)
				}
				if req.Messages[0].ContentParts[1].ImageSource.URL != "https://example.com/image.png" {
					t.Errorf("Unexpected URL: %s", req.Messages[0].ContentParts[1].ImageSource.URL)
				}
			},
		},
		{
			name: "tool call message",
			openaiReq: &OpenAIChatCompletionRequest{
				Model: "gpt-4",
				Messages: []openaiMessage{
					{
						Role:    "user",
						Content: "What's the weather?",
					},
					{
						Role:    "assistant",
						Content: "",
						ToolCalls: []openaiToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: openaiFunctionCall{
									Name:      "get_weather",
									Arguments: `{"location":"San Francisco"}`,
								},
							},
						},
					},
				},
				Tools: []openaiTool{
					{
						Type: "function",
						Function: openaiFunctionDefinition{
							Name:        "get_weather",
							Description: "Get weather for a location",
							Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`),
						},
					},
				},
			},
			wantErr: false,
			checkResult: func(t *testing.T, req *Request) {
				if len(req.Messages) != 2 {
					t.Fatalf("Expected 2 messages, got %d", len(req.Messages))
				}
				if len(req.Messages[1].ToolCalls) != 1 {
					t.Fatalf("Expected 1 tool call, got %d", len(req.Messages[1].ToolCalls))
				}
				tc := req.Messages[1].ToolCalls[0]
				if tc.ID != "call_123" {
					t.Errorf("Expected ID 'call_123', got '%s'", tc.ID)
				}
				if tc.Function != "get_weather" {
					t.Errorf("Expected function 'get_weather', got '%s'", tc.Function)
				}
				if len(req.Tools) != 1 {
					t.Fatalf("Expected 1 tool, got %d", len(req.Tools))
				}
				if req.Tools[0].Function.Name != "get_weather" {
					t.Errorf("Expected tool name 'get_weather', got '%s'", req.Tools[0].Function.Name)
				}
			},
		},
		{
			name:      "nil request",
			openaiReq: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertRequestToUniversal(tt.openaiReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertRequestToUniversal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestConvertResponseToOpenAI(t *testing.T) {
	converter := NewOpenAIFormatConverter()

	tests := []struct {
		name           string
		universalResp  *Response
		model          string
		promptTokens   int
		completionToks int
		wantErr        bool
		checkResult    func(*testing.T, *openaiChatCompletionResponse)
	}{
		{
			name: "simple text response",
			universalResp: &Response{
				Text: "Hello! How can I help you?",
			},
			model:          "gpt-4",
			promptTokens:   10,
			completionToks: 8,
			wantErr:        false,
			checkResult: func(t *testing.T, resp *openaiChatCompletionResponse) {
				if resp.Model != "gpt-4" {
					t.Errorf("Expected model 'gpt-4', got '%s'", resp.Model)
				}
				if resp.Object != "chat.completion" {
					t.Errorf("Expected object 'chat.completion', got '%s'", resp.Object)
				}
				if len(resp.Choices) != 1 {
					t.Fatalf("Expected 1 choice, got %d", len(resp.Choices))
				}
				choice := resp.Choices[0]
				if choice.Message.Role != "assistant" {
					t.Errorf("Expected role 'assistant', got '%s'", choice.Message.Role)
				}
				content, ok := choice.Message.Content.(string)
				if !ok {
					t.Fatalf("Expected string content, got %T", choice.Message.Content)
				}
				if content != "Hello! How can I help you?" {
					t.Errorf("Unexpected content: %s", content)
				}
				if choice.FinishReason != "stop" {
					t.Errorf("Expected finish reason 'stop', got '%s'", choice.FinishReason)
				}
				if resp.Usage == nil {
					t.Fatal("Expected usage to be set")
				}
				if resp.Usage.PromptTokens != 10 {
					t.Errorf("Expected 10 prompt tokens, got %d", resp.Usage.PromptTokens)
				}
				if resp.Usage.CompletionTokens != 8 {
					t.Errorf("Expected 8 completion tokens, got %d", resp.Usage.CompletionTokens)
				}
				if resp.Usage.TotalTokens != 18 {
					t.Errorf("Expected 18 total tokens, got %d", resp.Usage.TotalTokens)
				}
			},
		},
		{
			name: "tool call response",
			universalResp: &Response{
				ToolCalls: []ToolCall{
					{
						ID:        "call_456",
						Type:      "function",
						Function:  "get_weather",
						Arguments: `{"location":"New York"}`,
					},
				},
			},
			model:          "gpt-4",
			promptTokens:   15,
			completionToks: 12,
			wantErr:        false,
			checkResult: func(t *testing.T, resp *openaiChatCompletionResponse) {
				if len(resp.Choices) != 1 {
					t.Fatalf("Expected 1 choice, got %d", len(resp.Choices))
				}
				choice := resp.Choices[0]
				if choice.FinishReason != "tool_calls" {
					t.Errorf("Expected finish reason 'tool_calls', got '%s'", choice.FinishReason)
				}
				if len(choice.Message.ToolCalls) != 1 {
					t.Fatalf("Expected 1 tool call, got %d", len(choice.Message.ToolCalls))
				}
				tc := choice.Message.ToolCalls[0]
				if tc.ID != "call_456" {
					t.Errorf("Expected ID 'call_456', got '%s'", tc.ID)
				}
				if tc.Function.Name != "get_weather" {
					t.Errorf("Expected function 'get_weather', got '%s'", tc.Function.Name)
				}
				if tc.Function.Arguments != `{"location":"New York"}` {
					t.Errorf("Unexpected arguments: %s", tc.Function.Arguments)
				}
			},
		},
		{
			name:          "nil response",
			universalResp: nil,
			model:         "gpt-4",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertResponseToOpenAI(tt.universalResp, tt.model, tt.promptTokens, tt.completionToks)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertResponseToOpenAI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}
