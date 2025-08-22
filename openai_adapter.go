package ai

import (
	"encoding/json"
	"fmt"
)

// openaiAdapter implements the providerAdapter interface for OpenAI.
type openaiAdapter struct{}

func (a *openaiAdapter) getModel(req *Request) string {
	if req.Model == "" {
		return "gpt-4o-mini"
	}
	return req.Model
}

func (a *openaiAdapter) getEndpoint(model string) string {
	return "/chat/completions"
}

func (a *openaiAdapter) buildRequestPayload(req *Request) (any, error) {
	openaiReq := &openaiChatCompletionRequest{
		Model:    a.getModel(req),
		Messages: make([]openaiMessage, len(req.Messages)),
	}

	for i, msg := range req.Messages {
		openaiReq.Messages[i] = openaiMessage{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			openaiReq.Messages[i].ToolCalls = make([]openaiToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				openaiReq.Messages[i].ToolCalls[j] = openaiToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: openaiFunctionCall{
						Name:      tc.Function,
						Arguments: tc.Arguments,
					},
				}
			}
		}
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
	universalResp := &Response{
		Text: choice.Message.Content,
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

// --- Private OpenAI Specific Types ---
// (These are the same structs from the original client_openai.go)

type openaiChatCompletionRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
	Tools    []openaiTool    `json:"tools,omitempty"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
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
	Choices []openaiChoice `json:"choices"`
}

type openaiChoice struct {
	Message openaiMessage `json:"message"`
}
