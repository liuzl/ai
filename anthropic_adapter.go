package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// anthropicAdapter implements the providerAdapter interface for Anthropic.
type anthropicAdapter struct{}

func (a *anthropicAdapter) getModel(req *Request) string {
	if req.Model == "" {
		// A reasonable default, user can override.
		return "claude-haiku-4-5"
	}
	return req.Model
}

func (a *anthropicAdapter) getEndpoint(model string) string {
	return "/messages"
}

func (a *anthropicAdapter) buildRequestPayload(ctx context.Context, req *Request) (any, error) {
	anthropicReq := &anthropicMessagesRequest{
		Model:     a.getModel(req),
		System:    req.SystemPrompt,
		Messages:  make([]anthropicMessage, 0, len(req.Messages)),
		MaxTokens: 4096, // A required parameter for Anthropic.
	}

	for _, msg := range req.Messages {
		var role string
		var contentBlocks []anthropicContentBlock

		switch msg.Role {
		case RoleUser:
			role = "user"
			// A user message can be a simple text message, multimodal content, or a tool result.
			if msg.ToolCallID != "" {
				// Tool result message
				contentBlocks = append(contentBlocks, anthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				})
			} else if len(msg.ContentParts) > 0 {
				// Multimodal content
				for _, part := range msg.ContentParts {
					switch part.Type {
					case ContentTypeText:
						contentBlocks = append(contentBlocks, anthropicContentBlock{
							Type: "text",
							Text: part.Text,
						})
					case ContentTypeImage:
						if part.ImageSource != nil {
							// Determine media type
							mediaType := "image/png" // default
							if part.ImageSource.Format != "" {
								mediaType = "image/" + part.ImageSource.Format
								if part.ImageSource.Format == "jpg" {
									mediaType = "image/jpeg"
								}
							}

							source := &anthropicImageSource{MediaType: mediaType}
							switch part.ImageSource.Type {
							case ImageSourceTypeURL:
								source.Type = "url"
								source.URL = part.ImageSource.URL
							case ImageSourceTypeBase64:
								source.Type = "base64"
								data := part.ImageSource.Data
								// Remove data URI prefix if present
								if strings.HasPrefix(data, "data:") {
									if idx := strings.Index(data, ","); idx != -1 {
										data = data[idx+1:]
									}
								}
								source.Data = data
							}

							contentBlocks = append(contentBlocks, anthropicContentBlock{
								Type:   "image",
								Source: source,
							})
						}
					case ContentTypeDocument:
						if part.DocumentSource != nil {
							// Anthropic supports PDF documents
							mediaType := part.DocumentSource.MimeType
							if mediaType == "" {
								mediaType = "application/pdf"
							}

							source := &anthropicImageSource{MediaType: mediaType}
							switch part.DocumentSource.Type {
							case MediaSourceTypeURL:
								source.Type = "url"
								source.URL = part.DocumentSource.URL
							case MediaSourceTypeBase64:
								source.Type = "base64"
								data := part.DocumentSource.Data
								// Remove data URI prefix if present
								if strings.HasPrefix(data, "data:") {
									if idx := strings.Index(data, ","); idx != -1 {
										data = data[idx+1:]
									}
								}
								source.Data = data
							}

							// Anthropic uses "document" type for PDFs
							contentBlocks = append(contentBlocks, anthropicContentBlock{
								Type:   "document",
								Source: source,
							})
						}
					case ContentTypeAudio:
						return nil, fmt.Errorf("anthropic provider does not support audio input (content type: audio). Supported providers: Gemini")
					case ContentTypeVideo:
						return nil, fmt.Errorf("anthropic provider does not support video input (content type: video). Supported providers: Gemini")
					default:
						return nil, fmt.Errorf("anthropic provider does not support content type: %s", part.Type)
					}
				}
			} else if msg.Content != "" {
				// Backward compatibility: simple text content
				contentBlocks = append(contentBlocks, anthropicContentBlock{Type: "text", Text: msg.Content})
			}
		case RoleAssistant:
			role = "assistant"
			// An assistant message can have text content and tool calls.
			if len(msg.ContentParts) > 0 {
				// Handle multimodal content (typically just text for assistant)
				for _, part := range msg.ContentParts {
					if part.Type == ContentTypeText {
						contentBlocks = append(contentBlocks, anthropicContentBlock{
							Type: "text",
							Text: part.Text,
						})
					}
				}
			} else if msg.Content != "" {
				// Backward compatibility: simple text content
				contentBlocks = append(contentBlocks, anthropicContentBlock{Type: "text", Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
					return nil, fmt.Errorf("failed to unmarshal tool call arguments for anthropic: %w", err)
				}
				contentBlocks = append(contentBlocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function,
					Input: args,
				})
			}
		default:
			// Anthropic only supports 'user' and 'assistant' roles. Skip others.
			continue
		}

		if len(contentBlocks) > 0 {
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
				Role:    role,
				Content: contentBlocks,
			})
		}
	}

	// Translate our universal Tool definition to Anthropic's format.
	if len(req.Tools) > 0 {
		anthropicReq.Tools = make([]anthropicTool, len(req.Tools))
		for i, t := range req.Tools {
			anthropicReq.Tools[i] = anthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			}
		}
	}

	return anthropicReq, nil
}

func (a *anthropicAdapter) parseResponse(providerResp []byte) (*Response, error) {
	var anthropicResp anthropicMessagesResponse
	if err := json.Unmarshal(providerResp, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal anthropic response: %w", err)
	}

	universalResp := &Response{}

	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			universalResp.Text += block.Text
		case "tool_use":
			args, err := json.Marshal(block.Input)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal anthropic tool input: %w", err)
			}
			toolCall := ToolCall{
				ID:        block.ID,
				Type:      "function",
				Function:  block.Name,
				Arguments: string(args),
			}
			universalResp.ToolCalls = append(universalResp.ToolCalls, toolCall)
		}
	}

	return universalResp, nil
}

func (a *anthropicAdapter) enableStreaming(payload any) {
	if req, ok := payload.(*anthropicMessagesRequest); ok {
		req.Stream = true
	}
}

func (a *anthropicAdapter) parseStreamEvent(event *sseEvent, acc *streamAccumulator) (*StreamChunk, bool, error) {
	if len(event.Data) == 0 {
		return nil, false, nil
	}

	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(event.Data, &envelope); err != nil {
		return nil, false, fmt.Errorf("failed to parse anthropic stream envelope: %w", err)
	}

	eventType := envelope.Type
	if eventType == "" && event.Event != "" {
		eventType = event.Event
	}

	switch eventType {
	case "message_stop":
		return &StreamChunk{Done: true}, true, nil
	case "content_block_start":
		var payload struct {
			Index        int                   `json:"index"`
			ContentBlock anthropicContentBlock `json:"content_block"`
		}
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return nil, false, err
		}
		if payload.ContentBlock.Type == "text" {
			acc.anthropicBlocks[payload.Index] = &anthropicBlockState{kind: "text"}
		} else if payload.ContentBlock.Type == "tool_use" {
			acc.anthropicBlocks[payload.Index] = &anthropicBlockState{
				kind:     "tool",
				toolID:   payload.ContentBlock.ID,
				toolName: payload.ContentBlock.Name,
			}
		}
		return nil, false, nil
	case "content_block_delta":
		var payload struct {
			Index int `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text,omitempty"`
				PartialJSON string `json:"partial_json,omitempty"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return nil, false, err
		}
		block := acc.anthropicBlocks[payload.Index]
		if block == nil {
			return nil, false, nil
		}
		chunk := &StreamChunk{}
		if block.kind == "text" && payload.Delta.Text != "" {
			chunk.TextDelta = payload.Delta.Text
		}
		if block.kind == "tool" && (payload.Delta.PartialJSON != "" || payload.Delta.Text != "") {
			argDelta := payload.Delta.PartialJSON
			if argDelta == "" {
				argDelta = payload.Delta.Text
			}
			chunk.ToolCallDeltas = append(chunk.ToolCallDeltas, ToolCallDelta{
				ID:             block.toolID,
				Type:           "function",
				Function:       block.toolName,
				ArgumentsDelta: argDelta,
			})
		}
		if chunk.TextDelta == "" && len(chunk.ToolCallDeltas) == 0 {
			return nil, false, nil
		}
		return chunk, false, nil
	case "content_block_stop":
		var payload struct {
			Index int `json:"index"`
		}
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return nil, false, err
		}
		block := acc.anthropicBlocks[payload.Index]
		if block == nil {
			return nil, false, nil
		}
		if block.kind == "tool" {
			return &StreamChunk{
				ToolCallDeltas: []ToolCallDelta{
					{
						ID:   block.toolID,
						Type: "function",
						// ArgumentsDelta empty; Done marks completion.
						Function: block.toolName,
						Done:     true,
					},
				},
			}, false, nil
		}
		return nil, false, nil
	case "message_delta":
		var payload struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return nil, false, err
		}
		if payload.Delta.StopReason != "" {
			return &StreamChunk{Done: true}, true, nil
		}
		return nil, false, nil
	default:
		// Ignore other event types
		return nil, false, nil
	}
}

func (a *anthropicAdapter) getStreamEndpoint(model string) string {
	return a.getEndpoint(model)
}

// --- Private Anthropic Specific Types ---

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicMessagesRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicMessagesResponse struct {
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// For image content
	Source *anthropicImageSource `json:"source,omitempty"`
	// For tool use request from model
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
	// For tool result response from user
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`       // "base64" or "url"
	MediaType string `json:"media_type"` // "image/jpeg", "image/png", "image/gif", "image/webp"
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}
