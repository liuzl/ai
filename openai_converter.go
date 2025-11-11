package ai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// OpenAIFormatConverter provides conversion between OpenAI API format and Universal format.
// This enables creating an OpenAI-compatible proxy server that can route to any provider.
// It implements the FormatConverter interface.
type OpenAIFormatConverter struct{}

// NewOpenAIFormatConverter creates a new OpenAI format converter.
func NewOpenAIFormatConverter() *OpenAIFormatConverter {
	return &OpenAIFormatConverter{}
}

// GetEndpoint returns the OpenAI API endpoint path.
func (c *OpenAIFormatConverter) GetEndpoint() string {
	return "/v1/chat/completions"
}

// GetProviderName returns the provider name.
func (c *OpenAIFormatConverter) GetProviderName() string {
	return string(ProviderOpenAI)
}

// ConvertRequestFromFormat converts an OpenAI request to Universal format.
// Implements FormatConverter interface.
func (c *OpenAIFormatConverter) ConvertRequestFromFormat(providerReq any) (*Request, error) {
	openaiReq, ok := providerReq.(*OpenAIChatCompletionRequest)
	if !ok {
		return nil, NewInvalidRequestError(string(ProviderOpenAI), "expected *OpenAIChatCompletionRequest", "", nil)
	}
	return c.ConvertRequestToUniversal(openaiReq)
}

// ConvertResponseToFormat converts a Universal Response to OpenAI format.
// Implements FormatConverter interface.
func (c *OpenAIFormatConverter) ConvertResponseToFormat(universalResp *Response, originalModel string) (any, error) {
	// Token counts are set to 0 as we don't have that info from universal response
	return c.ConvertResponseToOpenAI(universalResp, originalModel, 0, 0)
}

// ConvertRequestToUniversal converts an OpenAI chat completion request to Universal Request format.
func (c *OpenAIFormatConverter) ConvertRequestToUniversal(openaiReq *OpenAIChatCompletionRequest) (*Request, error) {
	if openaiReq == nil {
		return nil, fmt.Errorf("openai request cannot be nil")
	}

	universalReq := &Request{
		Model:    openaiReq.Model,
		Messages: make([]Message, 0, len(openaiReq.Messages)),
	}

	// Convert messages
	for i, msg := range openaiReq.Messages {
		universalMsg := Message{
			Role:       Role(msg.Role),
			ToolCallID: msg.ToolCallID,
		}

		// Handle content (can be string or []openaiContentPart)
		switch content := msg.Content.(type) {
		case string:
			// Simple text content
			universalMsg.Content = content
		case []any:
			// Multimodal content - convert to ContentParts
			parts := make([]ContentPart, 0, len(content))
			for j, rawPart := range content {
				// Parse the part
				partBytes, err := json.Marshal(rawPart)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal content part[%d] in message[%d]: %w", j, i, err)
				}
				var part openaiContentPart
				if err := json.Unmarshal(partBytes, &part); err != nil {
					return nil, fmt.Errorf("failed to unmarshal content part[%d] in message[%d]: %w", j, i, err)
				}

				switch part.Type {
				case "text":
					parts = append(parts, ContentPart{
						Type: ContentTypeText,
						Text: part.Text,
					})
				case "image_url":
					if part.ImageURL != nil {
						// Determine if it's a URL or data URI
						imageSource := &ImageSource{}
						if len(part.ImageURL.URL) > 0 && part.ImageURL.URL[:5] == "data:" {
							// Base64 data URI
							imageSource.Type = ImageSourceTypeBase64
							imageSource.Data = part.ImageURL.URL
						} else {
							// Regular URL
							imageSource.Type = ImageSourceTypeURL
							imageSource.URL = part.ImageURL.URL
						}
						parts = append(parts, ContentPart{
							Type:        ContentTypeImage,
							ImageSource: imageSource,
						})
					}
				}
			}
			universalMsg.ContentParts = parts
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			universalMsg.ToolCalls = make([]ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				universalMsg.ToolCalls[j] = ToolCall{
					ID:        tc.ID,
					Type:      tc.Type,
					Function:  tc.Function.Name,
					Arguments: tc.Function.Arguments,
				}
			}
		}

		// Extract system prompt if present
		if msg.Role == string(RoleSystem) && universalReq.SystemPrompt == "" {
			if msgContent, ok := msg.Content.(string); ok {
				universalReq.SystemPrompt = msgContent
				continue // Don't add system messages to the messages array
			}
		}

		universalReq.Messages = append(universalReq.Messages, universalMsg)
	}

	// Convert tools
	if len(openaiReq.Tools) > 0 {
		universalReq.Tools = make([]Tool, len(openaiReq.Tools))
		for i, t := range openaiReq.Tools {
			universalReq.Tools[i] = Tool{
				Type: t.Type,
				Function: FunctionDefinition{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  t.Function.Parameters,
				},
			}
		}
	}

	return universalReq, nil
}

// ConvertResponseToOpenAI converts a Universal Response to OpenAI chat completion response format.
func (c *OpenAIFormatConverter) ConvertResponseToOpenAI(universalResp *Response, model string, promptTokens, completionTokens int) (*openaiChatCompletionResponse, error) {
	if universalResp == nil {
		return nil, fmt.Errorf("universal response cannot be nil")
	}

	openaiResp := &openaiChatCompletionResponse{
		ID:      generateResponseID(),
		Object:  "chat.completion",
		Created: getCurrentTimestamp(),
		Model:   model,
		Choices: []openaiChoice{
			{
				Index: 0,
				Message: openaiMessage{
					Role:    string(RoleAssistant),
					Content: universalResp.Text,
				},
				FinishReason: "stop",
			},
		},
		Usage: &openaiUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}

	// Convert tool calls if present
	if len(universalResp.ToolCalls) > 0 {
		openaiResp.Choices[0].Message.ToolCalls = make([]openaiToolCall, len(universalResp.ToolCalls))
		for i, tc := range universalResp.ToolCalls {
			openaiResp.Choices[0].Message.ToolCalls[i] = openaiToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: openaiFunctionCall{
					Name:      tc.Function,
					Arguments: tc.Arguments,
				},
			}
		}
		openaiResp.Choices[0].FinishReason = "tool_calls"
	}

	return openaiResp, nil
}

// Helper functions

func generateResponseID() string {
	// Generate a simple response ID (in production, use UUID or similar)
	return "chatcmpl-" + generateRandomID(29)
}

func getCurrentTimestamp() int64 {
	// Return current Unix timestamp
	return time.Now().Unix()
}

func generateRandomID(length int) string {
	// Generate a cryptographically secure random ID
	b := make([]byte, (length+1)/2) // Each byte becomes 2 hex chars
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	id := hex.EncodeToString(b)
	if len(id) > length {
		return id[:length]
	}
	return id
}
