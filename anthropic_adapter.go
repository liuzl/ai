package ai

import (
	"encoding/json"
	"fmt"
)

// anthropicAdapter implements the providerAdapter interface for Anthropic.
type anthropicAdapter struct{}

func (a *anthropicAdapter) getModel(req *Request) string {
	if req.Model == "" {
		// A reasonable default, user can override.
		return "claude-3-haiku-20240307"
	}
	return req.Model
}

func (a *anthropicAdapter) getEndpoint(model string) string {
	return "/messages"
}

func (a *anthropicAdapter) buildRequestPayload(req *Request) (any, error) {
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
			// A user message can be a simple text message or a tool result.
			if msg.ToolCallID != "" {
				contentBlocks = append(contentBlocks, anthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				})
			} else {
				contentBlocks = append(contentBlocks, anthropicContentBlock{Type: "text", Text: msg.Content})
			}
		case RoleAssistant:
			role = "assistant"
			// An assistant message can have text content and tool calls.
			if msg.Content != "" {
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
	// For tool use request from model
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
	// For tool result response from user
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}
