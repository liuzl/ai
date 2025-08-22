package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// geminiClient implements the Client interface for the Google Gemini provider.
type geminiClient struct {
	b *baseClient
}

// newGeminiClient is the internal constructor for the Gemini client.
func newGeminiClient(cfg *Config) Client {
	baseURL := "https://generativelanguage.googleapis.com"
	if cfg.baseURL != "" {
		baseURL = cfg.baseURL
	}
	headers := make(http.Header)
	headers.Set("x-goog-api-key", cfg.apiKey)

	return &geminiClient{
		b: newBaseClient(baseURL, "v1beta", cfg.timeout, headers, 3),
	}
}

// Generate implements the Client interface for Gemini.
func (c *geminiClient) Generate(ctx context.Context, req *Request) (*Response, error) {
	geminiReq, err := c.newGeminiRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build gemini request: %w", err)
	}

	model := req.Model
	if model == "" {
		model = "gemini-1.5-flash"
	}

	resp, err := c.callGeminiAPI(ctx, model, geminiReq)
	if err != nil {
		return nil, err
	}

	return c.toContentResponse(resp)
}

// newGeminiRequest translates our universal request to a Gemini-specific one.
func (c *geminiClient) newGeminiRequest(req *Request) (*geminiGenerateContentRequest, error) {
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
			// Correctly translate tool calls from the assistant's message history
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
			role = "user"
			// Find the corresponding tool call to get the function name
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
		default:
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

	// Translate our universal Tool definition to Gemini's format
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

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		geminiReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{
				{Text: &req.SystemPrompt},
			},
		}
	}

	return geminiReq, nil
}

// toContentResponse translates a Gemini-specific response to our universal one.
func (c *geminiClient) toContentResponse(resp *geminiGenerateContentResponse) (*Response, error) {
	if len(resp.Candidates) == 0 {
		return &Response{}, nil
	}

	candidate := resp.Candidates[0]
	universalResp := &Response{}
	timestamp := time.Now().UnixNano() // Use a single timestamp for the whole response

	for i, part := range candidate.Content.Parts {
		if part.Text != nil {
			universalResp.Text += *part.Text
		}
		if part.FunctionCall != nil {
			args, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal gemini function call args: %w", err)
			}
			toolCall := ToolCall{
				// ID is unique within this response by using the loop index.
				ID:        fmt.Sprintf("gemini-tool-call-%d-%d", timestamp, i),
				Type:      "function",
				Function:  part.FunctionCall.Name,
				Arguments: string(args),
			}
			universalResp.ToolCalls = append(universalResp.ToolCalls, toolCall)
		}
	}

	return universalResp, nil
}

// callGeminiAPI is the internal method that calls the Gemini API.
func (c *geminiClient) callGeminiAPI(ctx context.Context, model string, req *geminiGenerateContentRequest) (*geminiGenerateContentResponse, error) {
	path := fmt.Sprintf("/models/%s:generateContent", model)
	var resp geminiGenerateContentResponse
	err := c.b.doJSONRequest(ctx, "POST", path, req, &resp)
	return &resp, err
}

// --- Private Gemini Specific Types ---

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
