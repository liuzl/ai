package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRequestValidation_EmptyMessages tests that empty messages fail validation
func TestRequestValidation_EmptyMessages(t *testing.T) {
	req := &Request{
		Messages: []Message{},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for empty messages, got nil")
	}

	if !strings.Contains(err.Error(), "at least one message") {
		t.Errorf("Expected error about empty messages, got: %v", err)
	}
}

// TestRequestValidation_InvalidRole tests invalid message roles
func TestRequestValidation_InvalidRole(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{Role: Role("invalid"), Content: "test"},
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid role, got nil")
	}

	if !strings.Contains(err.Error(), "invalid role") {
		t.Errorf("Expected error about invalid role, got: %v", err)
	}
}

// TestRequestValidation_MessageNoContent tests message without content or tool calls
func TestRequestValidation_MessageNoContent(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{Role: RoleUser}, // No content or tool calls
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for message without content, got nil")
	}

	if !strings.Contains(err.Error(), "must have either content or tool calls") {
		t.Errorf("Expected error about missing content, got: %v", err)
	}
}

// TestRequestValidation_ToolRoleWithoutID tests tool role without tool_call_id
func TestRequestValidation_ToolRoleWithoutID(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{Role: RoleTool, Content: "result"},
			// Missing ToolCallID
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for tool role without ID, got nil")
	}

	if !strings.Contains(err.Error(), "must have tool_call_id") {
		t.Errorf("Expected error about missing tool_call_id, got: %v", err)
	}
}

// TestRequestValidation_EmptyTextContentPart tests empty text in content parts
func TestRequestValidation_EmptyTextContentPart(t *testing.T) {
	req := &Request{
		Messages: []Message{
			NewMultimodalMessage(RoleUser, []ContentPart{
				{Type: ContentTypeText, Text: "   "}, // Whitespace only
			}),
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for empty text content part, got nil")
	}

	if !strings.Contains(err.Error(), "text content cannot be empty") {
		t.Errorf("Expected error about empty text, got: %v", err)
	}
}

// TestRequestValidation_ImagePartWithoutSource tests image part without source
func TestRequestValidation_ImagePartWithoutSource(t *testing.T) {
	req := &Request{
		Messages: []Message{
			NewMultimodalMessage(RoleUser, []ContentPart{
				{Type: ContentTypeImage}, // No ImageSource
			}),
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for image part without source, got nil")
	}

	if !strings.Contains(err.Error(), "must have image source") {
		t.Errorf("Expected error about missing image source, got: %v", err)
	}
}

// TestRequestValidation_InvalidImageSourceType tests invalid image source type
func TestRequestValidation_InvalidImageSourceType(t *testing.T) {
	req := &Request{
		Messages: []Message{
			NewMultimodalMessage(RoleUser, []ContentPart{
				{
					Type: ContentTypeImage,
					ImageSource: &ImageSource{
						Type: ImageSourceType("invalid"),
					},
				},
			}),
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid image source type, got nil")
	}

	if !strings.Contains(err.Error(), "invalid image source type") {
		t.Errorf("Expected error about invalid image source type, got: %v", err)
	}
}

// TestRequestValidation_EmptyImageURL tests empty image URL
func TestRequestValidation_EmptyImageURL(t *testing.T) {
	req := &Request{
		Messages: []Message{
			NewMultimodalMessage(RoleUser, []ContentPart{
				{
					Type: ContentTypeImage,
					ImageSource: &ImageSource{
						Type: ImageSourceTypeURL,
						URL:  "   ", // Whitespace only
					},
				},
			}),
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for empty image URL, got nil")
	}

	if !strings.Contains(err.Error(), "image URL cannot be empty") {
		t.Errorf("Expected error about empty URL, got: %v", err)
	}
}

// TestRequestValidation_EmptyImageData tests empty base64 image data
func TestRequestValidation_EmptyImageData(t *testing.T) {
	req := &Request{
		Messages: []Message{
			NewMultimodalMessage(RoleUser, []ContentPart{
				{
					Type: ContentTypeImage,
					ImageSource: &ImageSource{
						Type: ImageSourceTypeBase64,
						Data: "",
					},
				},
			}),
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for empty image data, got nil")
	}

	if !strings.Contains(err.Error(), "image data cannot be empty") {
		t.Errorf("Expected error about empty data, got: %v", err)
	}
}

// TestRequestValidation_InvalidContentType tests invalid content type
func TestRequestValidation_InvalidContentType(t *testing.T) {
	req := &Request{
		Messages: []Message{
			NewMultimodalMessage(RoleUser, []ContentPart{
				{Type: ContentType("invalid"), Text: "test"},
			}),
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid content type, got nil")
	}

	if !strings.Contains(err.Error(), "invalid content type") {
		t.Errorf("Expected error about invalid content type, got: %v", err)
	}
}

// TestRequestValidation_EmptyToolCallFunction tests tool call without function name
func TestRequestValidation_EmptyToolCallFunction(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{Function: "", Arguments: `{}`},
				},
			},
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for empty function name, got nil")
	}

	if !strings.Contains(err.Error(), "function name cannot be empty") {
		t.Errorf("Expected error about empty function name, got: %v", err)
	}
}

// TestRequestValidation_EmptyToolCallArguments tests tool call without arguments
func TestRequestValidation_EmptyToolCallArguments(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{Function: "test", Arguments: ""},
				},
			},
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for empty arguments, got nil")
	}

	if !strings.Contains(err.Error(), "arguments cannot be empty") {
		t.Errorf("Expected error about empty arguments, got: %v", err)
	}
}

// TestRequestValidation_InvalidToolCallJSON tests tool call with invalid JSON arguments
func TestRequestValidation_InvalidToolCallJSON(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{Function: "test", Arguments: "not valid json"},
				},
			},
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid JSON arguments, got nil")
	}

	if !strings.Contains(err.Error(), "invalid JSON arguments") {
		t.Errorf("Expected error about invalid JSON, got: %v", err)
	}
}

// TestRequestValidation_EmptyToolType tests tool without type
func TestRequestValidation_EmptyToolType(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "test"},
		},
		Tools: []Tool{
			{
				Type: "",
				Function: FunctionDefinition{
					Name:       "test",
					Parameters: json.RawMessage(`{}`),
				},
			},
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for empty tool type, got nil")
	}

	if !strings.Contains(err.Error(), "type cannot be empty") {
		t.Errorf("Expected error about empty type, got: %v", err)
	}
}

// TestRequestValidation_EmptyToolFunctionName tests tool without function name
func TestRequestValidation_EmptyToolFunctionName(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "test"},
		},
		Tools: []Tool{
			{
				Type: "function",
				Function: FunctionDefinition{
					Name:       "",
					Parameters: json.RawMessage(`{}`),
				},
			},
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for empty function name, got nil")
	}

	if !strings.Contains(err.Error(), "function name cannot be empty") {
		t.Errorf("Expected error about empty function name, got: %v", err)
	}
}

// TestRequestValidation_EmptyToolParameters tests tool without parameters
func TestRequestValidation_EmptyToolParameters(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "test"},
		},
		Tools: []Tool{
			{
				Type: "function",
				Function: FunctionDefinition{
					Name:       "test",
					Parameters: json.RawMessage{},
				},
			},
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for empty parameters, got nil")
	}

	if !strings.Contains(err.Error(), "parameters cannot be empty") {
		t.Errorf("Expected error about empty parameters, got: %v", err)
	}
}

// TestRequestValidation_InvalidToolParametersJSON tests tool with invalid JSON parameters
func TestRequestValidation_InvalidToolParametersJSON(t *testing.T) {
	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "test"},
		},
		Tools: []Tool{
			{
				Type: "function",
				Function: FunctionDefinition{
					Name:       "test",
					Parameters: json.RawMessage(`not valid json`),
				},
			},
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for invalid JSON parameters, got nil")
	}

	if !strings.Contains(err.Error(), "invalid JSON parameters") {
		t.Errorf("Expected error about invalid JSON, got: %v", err)
	}
}

// TestRequestValidation_WhitespaceOnlyModel tests model with whitespace only
func TestRequestValidation_WhitespaceOnlyModel(t *testing.T) {
	req := &Request{
		Model: "   ",
		Messages: []Message{
			{Role: RoleUser, Content: "test"},
		},
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("Expected error for whitespace-only model, got nil")
	}

	if !strings.Contains(err.Error(), "model cannot be whitespace only") {
		t.Errorf("Expected error about whitespace model, got: %v", err)
	}
}

// TestRequestValidation_ValidRequests tests various valid request configurations
func TestRequestValidation_ValidRequests(t *testing.T) {
	testCases := []struct {
		name string
		req  *Request
	}{
		{
			name: "Simple text message",
			req: &Request{
				Messages: []Message{
					{Role: RoleUser, Content: "Hello"},
				},
			},
		},
		{
			name: "Multimodal message with text and image",
			req: &Request{
				Messages: []Message{
					NewMultimodalMessage(RoleUser, []ContentPart{
						NewTextPart("What's in this image?"),
						NewImagePartFromURL("https://example.com/image.jpg"),
					}),
				},
			},
		},
		{
			name: "Message with tool calls",
			req: &Request{
				Messages: []Message{
					{
						Role: RoleAssistant,
						ToolCalls: []ToolCall{
							{
								ID:        "call_123",
								Type:      "function",
								Function:  "get_weather",
								Arguments: `{"city":"London"}`,
							},
						},
					},
				},
			},
		},
		{
			name: "Tool result message",
			req: &Request{
				Messages: []Message{
					{
						Role:       RoleTool,
						Content:    `{"temperature":20}`,
						ToolCallID: "call_123",
					},
				},
			},
		},
		{
			name: "Request with tools",
			req: &Request{
				Messages: []Message{
					{Role: RoleUser, Content: "What's the weather?"},
				},
				Tools: []Tool{
					{
						Type: "function",
						Function: FunctionDefinition{
							Name:        "get_weather",
							Description: "Get weather for a city",
							Parameters:  json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
						},
					},
				},
			},
		},
		{
			name: "Request with model specified",
			req: &Request{
				Model: "gpt-4o",
				Messages: []Message{
					{Role: RoleUser, Content: "Hello"},
				},
			},
		},
		{
			name: "Request with system prompt",
			req: &Request{
				SystemPrompt: "You are a helpful assistant",
				Messages: []Message{
					{Role: RoleUser, Content: "Hello"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if err != nil {
				t.Errorf("Expected valid request to pass, got error: %v", err)
			}
		})
	}
}

// TestRequestValidation_Integration tests validation is called in Generate
func TestRequestValidation_Integration(t *testing.T) {
	// Create a mock client
	client, err := NewClient(
		WithProvider(ProviderOpenAI),
		WithAPIKey("test-key"),
		WithBaseURL("http://invalid"),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with invalid request (empty messages)
	req := &Request{Messages: []Message{}}
	_, err = client.Generate(nil, req)

	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	if !strings.Contains(err.Error(), "invalid request") {
		t.Errorf("Expected 'invalid request' error, got: %v", err)
	}

	if !strings.Contains(err.Error(), "at least one message") {
		t.Errorf("Expected error about empty messages, got: %v", err)
	}
}
