package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// openaiAdapter implements the providerAdapter interface for OpenAI.
type openaiAdapter struct{}

func (a *openaiAdapter) getModel(req *Request) string {
	if req.Model == "" {
		return "gpt-5-mini"
	}
	return req.Model
}

func (a *openaiAdapter) getEndpoint(model string) string {
	return "/chat/completions"
}

func (a *openaiAdapter) buildRequestPayload(req *Request) (any, error) {
	openaiReq := &OpenAIChatCompletionRequest{
		Model:    a.getModel(req),
		Messages: make([]openaiMessage, len(req.Messages)),
	}

	for i, msg := range req.Messages {
		openaiMsg := openaiMessage{
			Role:       string(msg.Role),
			ToolCallID: msg.ToolCallID,
		}

		// Handle multimodal content
		if len(msg.ContentParts) > 0 {
			// Convert content parts to OpenAI format
			parts := make([]openaiContentPart, 0, len(msg.ContentParts))
			for _, part := range msg.ContentParts {
				switch part.Type {
				case ContentTypeText:
					parts = append(parts, openaiContentPart{
						Type: "text",
						Text: part.Text,
					})
				case ContentTypeImage:
					if part.ImageSource != nil {
						var url string
						switch part.ImageSource.Type {
						case ImageSourceTypeURL:
							url = part.ImageSource.URL
						case ImageSourceTypeBase64:
							// Format as data URI
							url = formatBase64AsDataURI(part.ImageSource.Data, part.ImageSource.Format)
						}
						parts = append(parts, openaiContentPart{
							Type: "image_url",
							ImageURL: &openaiImageURL{
								URL: url,
							},
						})
					}
				case ContentTypeAudio:
					return nil, fmt.Errorf("OpenAI provider does not support audio input (content type: audio). Supported providers: Gemini")
				case ContentTypeVideo:
					return nil, fmt.Errorf("OpenAI provider does not support video input (content type: video). Supported providers: Gemini")
				case ContentTypeDocument:
					return nil, fmt.Errorf("OpenAI provider does not support document/PDF input (content type: document). Supported providers: Gemini, Anthropic")
				default:
					return nil, fmt.Errorf("OpenAI provider does not support content type: %s", part.Type)
				}
			}
			openaiMsg.Content = parts
		} else if msg.Content != "" {
			// Backward compatibility: simple text content
			openaiMsg.Content = msg.Content
		}

		if len(msg.ToolCalls) > 0 {
			openaiMsg.ToolCalls = make([]openaiToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				openaiMsg.ToolCalls[j] = openaiToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: openaiFunctionCall{
						Name:      tc.Function,
						Arguments: tc.Arguments,
					},
				}
			}
		}

		openaiReq.Messages[i] = openaiMsg
	}

	if len(req.Tools) > 0 {
		openaiReq.Tools = make([]openaiTool, len(req.Tools))
		for i, t := range req.Tools {
			openaiReq.Tools[i] = openaiTool{
				Type: t.Type,
				Function: openaiFunctionDefinition{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  t.Function.Parameters,
				},
			}
		}
	}

	if req.SystemPrompt != "" {
		openaiReq.Messages = append([]openaiMessage{
			{Role: string(RoleSystem), Content: req.SystemPrompt},
		}, openaiReq.Messages...)
	}

	return openaiReq, nil
}

func (a *openaiAdapter) parseResponse(providerResp []byte) (*Response, error) {
	var openaiResp openaiChatCompletionResponse
	if err := json.Unmarshal(providerResp, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal openai response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return &Response{}, nil
	}

	choice := openaiResp.Choices[0]
	universalResp := &Response{}

	// Handle Content field which can be either string (text-only) or []openaiContentPart (multimodal)
	switch content := choice.Message.Content.(type) {
	case string:
		universalResp.Text = content
	case []any:
		// Handle array of content parts (multimodal response)
		for _, part := range content {
			if partMap, ok := part.(map[string]any); ok {
				if partType, ok := partMap["type"].(string); ok && partType == "text" {
					if text, ok := partMap["text"].(string); ok {
						universalResp.Text += text
					}
				}
			}
		}
	}

	if len(choice.Message.ToolCalls) > 0 {
		universalResp.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			universalResp.ToolCalls[i] = ToolCall{
				ID:        tc.ID,
				Type:      tc.Type,
				Function:  tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	return universalResp, nil
}

// --- OpenAI Specific Types ---
// These types are exported to allow format conversion and proxy server usage.

// OpenAIChatCompletionRequest represents an OpenAI chat completion request.
// This type is exported to enable format conversion in the proxy server.
type OpenAIChatCompletionRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
	Tools    []openaiTool    `json:"tools,omitempty"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content,omitempty"` // string or []openaiContentPart
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiContentPart struct {
	Type     string          `json:"type"` // "text" or "image_url"
	Text     string          `json:"text,omitempty"`
	ImageURL *openaiImageURL `json:"image_url,omitempty"`
}

type openaiImageURL struct {
	URL    string `json:"url"`              // Can be HTTP(S) URL or data URI
	Detail string `json:"detail,omitempty"` // "auto", "low", or "high" (optional)
}

type openaiToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openaiFunctionCall `json:"function"`
}

type openaiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiTool struct {
	Type     string                   `json:"type"`
	Function openaiFunctionDefinition `json:"function"`
}

type openaiFunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openaiChatCompletionResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   *openaiUsage   `json:"usage,omitempty"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// formatBase64AsDataURI formats base64 image data as a data URI.
// If the data already starts with "data:", it returns it as-is.
// Otherwise, it prepends the appropriate data URI prefix based on the format.
func formatBase64AsDataURI(data, format string) string {
	// If already a data URI, return as-is
	if strings.HasPrefix(data, "data:") {
		return data
	}

	// Detect format from data if not specified
	if format == "" {
		format = "png" // default
	}

	// Map common formats to MIME types
	mimeType := "image/" + format
	if format == "jpg" {
		mimeType = "image/jpeg"
	}

	return fmt.Sprintf("data:%s;base64,%s", mimeType, data)
}
