package ai

import (
	"encoding/json"
	"testing"
)

func TestAnthropicConverter_ConvertRequestFromFormat_ToolUse(t *testing.T) {
	converter := NewAnthropicFormatConverter()

	// content with single tool_use block
	requestJSON := `{
		"model": "claude-3-opus-20240229",
		"messages": [
			{
				"role": "user",
				"content": "What's the weather in SF?"
			},
			{
				"role": "assistant",
				"content": [
					{
						"type": "tool_use",
						"id": "toolu_01XyZ",
						"name": "get_weather",
						"input": {"location": "San Francisco, CA"}
					}
				]
			}
		],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get weather info",
				"input_schema": {
					"type": "object",
					"properties": {
						"location": {"type": "string"}
					}
				}
			}
		]
	}`

	var providerReq AnthropicIncomingRequest
	if err := json.Unmarshal([]byte(requestJSON), &providerReq); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	req, err := converter.ConvertRequestFromFormat(&providerReq)
	if err != nil {
		t.Fatalf("ConvertRequestFromFormat failed: %v", err)
	}

	// Validate the converted request
	if len(req.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(req.Messages))
	}

	msg1 := req.Messages[1]
	if msg1.Role != RoleAssistant {
		t.Errorf("Expected role assistant, got %s", msg1.Role)
	}

	if len(msg1.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(msg1.ToolCalls))
	} else {
		tc := msg1.ToolCalls[0]
		if tc.ID != "toolu_01XyZ" {
			t.Errorf("Expected tool call ID toolu_01XyZ, got %s", tc.ID)
		}
		if tc.Function != "get_weather" {
			t.Errorf("Expected function get_weather, got %s", tc.Function)
		}
	}
	
	// This validation should pass if the conversion was correct
	if err := req.Validate(); err != nil {
		t.Errorf("Request validation failed: %v", err)
	}
}
