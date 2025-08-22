package ai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// geminiAdapter implements the providerAdapter interface for Google Gemini.
type geminiAdapter struct{}

func (a *geminiAdapter) getModel(req *Request) string {
	if req.Model == "" {
		return "gemini-1.5-flash"
	}
	return req.Model
}

func (a *geminiAdapter) getEndpoint(model string) string {
	return fmt.Sprintf("/models/%s:generateContent", model)
}

func (a *geminiAdapter) buildRequestPayload(req *Request) (any, error) {
	geminiReq := &geminiGenerateContentRequest{
		Contents: make([]geminiContent, len(req.Messages)),
	}

	for i, msg := range req.Messages {
		var role string
		parts := []geminiPart{}

		switch msg.Role {
		case RoleUser:
			role = "user"
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: &msg.Content})
			}
		case RoleAssistant:
			role = "model"
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: &msg.Content})
			}
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					var args map[string]any
					if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
						return nil, fmt.Errorf("failed to unmarshal tool call arguments for gemini: %w", err)
					}
					parts = append(parts, geminiPart{
						FunctionCall: &geminiFunctionCall{
							Name: tc.Function,
							Args: args,
						},
					})
				}
			}
		case RoleTool:
			role = "user" // Gemini represents tool responses as a user message
			if i > 0 {
				prevMsg := req.Messages[i-1]
				var matchingToolCall *ToolCall
				for _, tc := range prevMsg.ToolCalls {
					if tc.ID == msg.ToolCallID {
						matchingToolCall = &tc
						break
					}
				}

				if matchingToolCall != nil {
					var responseData map[string]any
					if err := json.Unmarshal([]byte(msg.Content), &responseData); err != nil {
						responseData = map[string]any{"content": msg.Content}
					}
					parts = append(parts, geminiPart{
						FunctionResponse: &geminiFunctionResponse{
							Name:     matchingToolCall.Function,
							Response: responseData,
						},
					})
				}
			}
		default: // Fallback for system or other roles
			role = "user"
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: &msg.Content})
			}
		}

		geminiReq.Contents[i] = geminiContent{
			Role:  role,
			Parts: parts,
		}
	}

	if len(req.Tools) > 0 {
		geminiReq.Tools = make([]geminiTool, len(req.Tools))
		for i, t := range req.Tools {
			geminiReq.Tools[i] = geminiTool{
				FunctionDeclarations: []geminiFunctionDeclaration{
					{
						Name:        t.Function.Name,
						Description: t.Function.Description,
						Parameters:  t.Function.Parameters,
					},
				},
			}
		}
	}

	if req.SystemPrompt != "" {
		geminiReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{
				{Text: &req.SystemPrompt},
			},
		}
	}

	return geminiReq, nil
}

func (a *geminiAdapter) parseResponse(providerResp []byte) (*Response, error) {
	var geminiResp geminiGenerateContentResponse
	if err := json.Unmarshal(providerResp, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gemini response: %w", err)
	}
	if len(geminiResp.Candidates) == 0 {
		return &Response{}, nil
	}
	candidate := geminiResp.Candidates[0]
	universalResp := &Response{}
	for _, part := range candidate.Content.Parts {
		if part.Text != nil {
			universalResp.Text += *part.Text
		}
		if part.FunctionCall != nil {
			args, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal gemini function call args: %w", err)
			}
			// Gemini API does not provide a tool_call_id, so we generate one.
			// Using crypto/rand for a secure random ID.
			randBytes := make([]byte, 8)
			if _, err := rand.Read(randBytes); err != nil {
				return nil, fmt.Errorf("failed to generate random tool call ID: %w", err)
			}
			toolCall := ToolCall{
				ID:        "gemini-tool-call-" + hex.EncodeToString(randBytes),
				Type:      "function",
				Function:  part.FunctionCall.Name,
				Arguments: string(args),
			}
			universalResp.ToolCalls = append(universalResp.ToolCalls, toolCall)
		}
	}
	return universalResp, nil
}

// --- Private Gemini Specific Types ---
// (These are the same structs from the original client_gemini.go)

type geminiGenerateContentRequest struct {
	Contents          []geminiContent `json:"contents"`
	Tools             []geminiTool    `json:"tools,omitempty"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text             *string                 `json:"text,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type geminiGenerateContentResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}
