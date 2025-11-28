package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AnthropicFormatConverter provides conversion between Anthropic API format and Universal format.
// It implements the FormatConverter interface.
type AnthropicFormatConverter struct{}

// NewAnthropicFormatConverter creates a new Anthropic format converter.
func NewAnthropicFormatConverter() *AnthropicFormatConverter {
	return &AnthropicFormatConverter{}
}

// DecodeRequest decodes the request body into the Anthropic request struct.
func (c *AnthropicFormatConverter) DecodeRequest(r *http.Request) (any, error) {
	var req AnthropicIncomingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("failed to decode Anthropic request: %w", err)
	}
	return &req, nil
}

// IsStreaming checks if the decoded request indicates a streaming response.
func (c *AnthropicFormatConverter) IsStreaming(providerReq any) bool {
	if req, ok := providerReq.(*AnthropicIncomingRequest); ok {
		return req.Stream
	}
	return false
}

// NewStreamHandler creates a handler for formatting streaming events.
func (c *AnthropicFormatConverter) NewStreamHandler(id string, model string) StreamEventHandler {
	return &AnthropicStreamHandler{
		ID:        id,
		Model:     model,
		ToolIndex: make(map[string]int),
	}
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
	anthropicReq, ok := providerReq.(*AnthropicIncomingRequest)
	if !ok {
		return nil, NewInvalidRequestError(string(ProviderAnthropic), "expected *AnthropicIncomingRequest", "", nil)
	}
	return c.ConvertRequestToUniversal(anthropicReq)
}

// ConvertResponseToFormat converts a Universal Response to Anthropic format.
func (c *AnthropicFormatConverter) ConvertResponseToFormat(universalResp *Response, originalModel string) (any, error) {
	return c.ConvertResponseToAnthropic(universalResp, originalModel)
}

// ConvertRequestToUniversal converts an Anthropic request to Universal format.
func (c *AnthropicFormatConverter) ConvertRequestToUniversal(anthropicReq *AnthropicIncomingRequest) (*Request, error) {
	if anthropicReq == nil {
		return nil, fmt.Errorf("anthropic request cannot be nil")
	}

	universalReq := &Request{
		Model:        anthropicReq.Model,
		SystemPrompt: anthropicReq.System,
		Messages:     make([]Message, 0, len(anthropicReq.Messages)),
	}

	// Convert messages
	for i, msg := range anthropicReq.Messages {
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

		// Handle content (can be string or list of blocks)
		switch content := msg.Content.(type) {
		case string:
			universalMsg.Content = content
		case []any:
			// Convert []any back to []anthropicContentBlock
			contentBytes, err := json.Marshal(content)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal content array in message[%d]: %w", i, err)
			}
			var blocks []anthropicContentBlock
			if err := json.Unmarshal(contentBytes, &blocks); err != nil {
				return nil, fmt.Errorf("failed to unmarshal content blocks in message[%d]: %w", i, err)
			}

			// Process content blocks
			hasMultimodal := false
			var toolResultID string

			// Check if it's a tool result or multimodal
			for _, block := range blocks {
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
				if hasMultimodal || len(blocks) > 1 {
					// Use ContentParts for multimodal content
					universalMsg.ContentParts = make([]ContentPart, 0, len(blocks))
					for _, block := range blocks {
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
				} else if len(blocks) == 1 {
					// Simple text content
					if blocks[0].Type == "text" {
						universalMsg.Content = blocks[0].Text
					}
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

// --- Anthropic Stream Handler ---

type AnthropicStreamHandler struct {
	ID             string
	Model          string
	MessageStarted bool
	TextIndex      *int
	NextIndex      int
	ToolIndex      map[string]int
	SentStop       bool
}

func (h *AnthropicStreamHandler) OnStart(w http.ResponseWriter, flusher http.Flusher) {}

func (h *AnthropicStreamHandler) OnChunk(w http.ResponseWriter, flusher http.Flusher, chunk *StreamChunk) error {
	if !h.MessageStarted {
		sendAnthropicEvent(w, flusher, "message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    h.ID,
				"type":  "message",
				"role":  "assistant",
				"model": h.Model,
			},
		})
		h.MessageStarted = true
	}

	if chunk.TextDelta != "" {
		index := 0
		if h.TextIndex == nil {
			h.TextIndex = new(int)
			*h.TextIndex = h.NextIndex
			index = *h.TextIndex
			h.NextIndex++
			sendAnthropicEvent(w, flusher, "content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": index,
				"content_block": map[string]any{
					"type": "text",
				},
			})
		} else {
			index = *h.TextIndex
		}
		sendAnthropicEvent(w, flusher, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": index,
			"delta": map[string]any{
				"type": "text_delta",
				"text": chunk.TextDelta,
			},
		})
	}

	for _, tc := range chunk.ToolCallDeltas {
		idx, ok := h.ToolIndex[tc.ID]
		if !ok {
			idx = h.NextIndex
			h.NextIndex++
			h.ToolIndex[tc.ID] = idx
			sendAnthropicEvent(w, flusher, "content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": idx,
				"content_block": map[string]any{
					"type": "tool_use",
					"id":   tc.ID,
					"name": tc.Function,
				},
			})
		}

		if tc.ArgumentsDelta != "" {
			sendAnthropicEvent(w, flusher, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": idx,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": tc.ArgumentsDelta,
				},
			})
		}

		if tc.Done {
			sendAnthropicEvent(w, flusher, "content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": idx,
			})
		}
	}

	if chunk.Done {
		if h.TextIndex != nil {
			sendAnthropicEvent(w, flusher, "content_block_stop", map[string]any{
				"type":  "content_block_stop",
				"index": *h.TextIndex,
			})
		}
		sendAnthropicEvent(w, flusher, "message_delta", map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": "end_turn"},
		})
		sendAnthropicEvent(w, flusher, "message_stop", map[string]any{"type": "message_stop"})
		h.SentStop = true
	}
	return nil
}

func (h *AnthropicStreamHandler) OnEnd(w http.ResponseWriter, flusher http.Flusher) {
	if !h.MessageStarted || h.SentStop {
		return
	}
	if h.TextIndex != nil {
		sendAnthropicEvent(w, flusher, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": *h.TextIndex,
		})
	}
	sendAnthropicEvent(w, flusher, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": "end_turn"},
	})
	sendAnthropicEvent(w, flusher, "message_stop", map[string]any{"type": "message_stop"})
	h.SentStop = true
}

func (h *AnthropicStreamHandler) OnError(w http.ResponseWriter, flusher http.Flusher, err error) {
	sendAnthropicEvent(w, flusher, "error", map[string]string{"error": err.Error()})
}

func sendAnthropicEvent(w http.ResponseWriter, flusher http.Flusher, event string, payload any) {
	w.Write([]byte("event: " + event + "\n"))
	if b, err := json.Marshal(payload); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", b)
	} else {
		fmt.Fprintf(w, "data: {\"error\":%q}\n\n", err.Error())
	}
	flusher.Flush()
}

// Helper function
func generateAnthropicMessageID() string {
	return "msg_" + generateRandomID(29)
}

// --- Anthropic Specific Types (Exported for format conversion) ---

// AnthropicIncomingRequest represents an Anthropic messages request.
type AnthropicIncomingRequest struct {
	Model     string                     `json:"model"`
	System    string                     `json:"system,omitempty"`
	Messages  []anthropicIncomingMessage `json:"messages"`
	MaxTokens int                        `json:"max_tokens"`
	Tools     []anthropicTool            `json:"tools,omitempty"`
	Stream    bool                       `json:"stream,omitempty"`
}

type anthropicIncomingMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // Can be string or []anthropicContentBlock
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
