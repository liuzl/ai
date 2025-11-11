package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AnthropicFormatConverter provides conversion between Anthropic API format and Universal format.
// It implements the FormatConverter interface.
type AnthropicFormatConverter struct{}

// NewAnthropicFormatConverter creates a new Anthropic format converter.
func NewAnthropicFormatConverter() *AnthropicFormatConverter {
	return &AnthropicFormatConverter{}
}

// GetEndpoint returns the Anthropic API endpoint path.
func (c *AnthropicFormatConverter) GetEndpoint() string {
	return "/v1/messages"
}

// GetProviderName returns the provider name.
func (c *AnthropicFormatConverter) GetProviderName() string {
	return string(ProviderAnthropic)
}

// ConvertRequestFromFormat converts an Anthropic request to Universal format.
func (c *AnthropicFormatConverter) ConvertRequestFromFormat(providerReq any) (*Request, error) {
	anthropicReq, ok := providerReq.(*AnthropicMessagesRequest)
	if !ok {
		return nil, NewInvalidRequestError(string(ProviderAnthropic), "expected *AnthropicMessagesRequest", "", nil)
	}
	return c.ConvertRequestToUniversal(anthropicReq)
}

// ConvertResponseToFormat converts a Universal Response to Anthropic format.
func (c *AnthropicFormatConverter) ConvertResponseToFormat(universalResp *Response, originalModel string) (any, error) {
	return c.ConvertResponseToAnthropic(universalResp, originalModel)
}

// ConvertRequestToUniversal converts an Anthropic request to Universal format.
func (c *AnthropicFormatConverter) ConvertRequestToUniversal(anthropicReq *AnthropicMessagesRequest) (*Request, error) {
	if anthropicReq == nil {
		return nil, fmt.Errorf("anthropic request cannot be nil")
	}

	universalReq := &Request{
		Model:        anthropicReq.Model,
		SystemPrompt: anthropicReq.System,
		Messages:     make([]Message, 0, len(anthropicReq.Messages)),
	}

	// Convert messages
	for _, msg := range anthropicReq.Messages {
		universalMsg := Message{}

		// Map role
		switch msg.Role {
		case "user":
			universalMsg.Role = RoleUser
		case "assistant":
			universalMsg.Role = RoleAssistant
		default:
			universalMsg.Role = RoleUser // fallback
		}

		// Process content blocks
		hasMultimodal := false
		var toolResultID string

		// Check if it's a tool result or multimodal
		for _, block := range msg.Content {
			if block.Type == "tool_result" {
				universalMsg.Role = RoleTool
				toolResultID = block.ToolUseID
				universalMsg.ToolCallID = toolResultID
				universalMsg.Content = block.Content
				break
			}
			if block.Type == "image" {
				hasMultimodal = true
			}
		}

		if universalMsg.Role != RoleTool {
			if hasMultimodal || len(msg.Content) > 1 {
				// Use ContentParts for multimodal content
				universalMsg.ContentParts = make([]ContentPart, 0, len(msg.Content))
				for _, block := range msg.Content {
					switch block.Type {
					case "text":
						universalMsg.ContentParts = append(universalMsg.ContentParts, ContentPart{
							Type: ContentTypeText,
							Text: block.Text,
						})
					case "image":
						if block.Source != nil {
							imageSource := &ImageSource{
								Format: strings.TrimPrefix(block.Source.MediaType, "image/"),
							}
							switch block.Source.Type {
							case "url":
								imageSource.Type = ImageSourceTypeURL
								imageSource.URL = block.Source.URL
							case "base64":
								imageSource.Type = ImageSourceTypeBase64
								imageSource.Data = block.Source.Data
							}
							universalMsg.ContentParts = append(universalMsg.ContentParts, ContentPart{
								Type:        ContentTypeImage,
								ImageSource: imageSource,
							})
						}
					case "tool_use":
						// Handle tool use from assistant
						args, err := json.Marshal(block.Input)
						if err != nil {
							return nil, fmt.Errorf("failed to marshal tool use input: %w", err)
						}
						universalMsg.ToolCalls = append(universalMsg.ToolCalls, ToolCall{
							ID:        block.ID,
							Type:      "function",
							Function:  block.Name,
							Arguments: string(args),
						})
					}
				}
			} else if len(msg.Content) == 1 {
				// Simple text content
				if msg.Content[0].Type == "text" {
					universalMsg.Content = msg.Content[0].Text
				}
			}
		}

		universalReq.Messages = append(universalReq.Messages, universalMsg)
	}

	// Convert tools
	if len(anthropicReq.Tools) > 0 {
		universalReq.Tools = make([]Tool, len(anthropicReq.Tools))
		for i, tool := range anthropicReq.Tools {
			universalReq.Tools[i] = Tool{
				Type: "function",
				Function: FunctionDefinition{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			}
		}
	}

	return universalReq, nil
}

// ConvertResponseToAnthropic converts a Universal Response to Anthropic format.
func (c *AnthropicFormatConverter) ConvertResponseToAnthropic(universalResp *Response, model string) (*AnthropicMessagesResponse, error) {
	if universalResp == nil {
		return nil, fmt.Errorf("universal response cannot be nil")
	}

	anthropicResp := &AnthropicMessagesResponse{
		ID:      generateAnthropicMessageID(),
		Type:    "message",
		Role:    "assistant",
		Model:   model,
		Content: make([]anthropicContentBlock, 0),
	}

	// Add text content if present
	if universalResp.Text != "" {
		anthropicResp.Content = append(anthropicResp.Content, anthropicContentBlock{
			Type: "text",
			Text: universalResp.Text,
		})
	}

	// Add tool calls if present
	for _, tc := range universalResp.ToolCalls {
		var input map[string]any
		if err := json.Unmarshal([]byte(tc.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
		}
		anthropicResp.Content = append(anthropicResp.Content, anthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function,
			Input: input,
		})
	}

	// Set stop reason
	if len(universalResp.ToolCalls) > 0 {
		anthropicResp.StopReason = "tool_use"
	} else {
		anthropicResp.StopReason = "end_turn"
	}

	return anthropicResp, nil
}

// Helper function
func generateAnthropicMessageID() string {
	return "msg_" + generateRandomID(29)
}

// --- Anthropic Specific Types (Exported for format conversion) ---

// AnthropicMessagesRequest represents an Anthropic messages request.
type AnthropicMessagesRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

// AnthropicMessagesResponse represents an Anthropic messages response.
type AnthropicMessagesResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Model      string                  `json:"model"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      *anthropicUsage         `json:"usage,omitempty"`
}

// anthropicUsage represents token usage information.
type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
