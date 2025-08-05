package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// geminiClient implements the AIClient interface for the Google Gemini provider.
type geminiClient struct {
	apiKey     string
	baseURL    string
	apiVersion string
	httpClient *http.Client
	maxRetries int
}

// newGeminiClient is the internal constructor for the Gemini client.
func newGeminiClient(cfg *Config) Client {
	baseURL := "https://generativelanguage.googleapis.com"
	if cfg.baseURL != "" {
		baseURL = cfg.baseURL
	}
	return &geminiClient{
		apiKey:     cfg.apiKey,
		baseURL:    baseURL,
		apiVersion: "v1beta",
		httpClient: &http.Client{Timeout: cfg.timeout},
		maxRetries: 3,
	}
}

// GenerateUniversalContent implements the AIClient interface for Gemini.
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
		case "user":
			role = "user"
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: &msg.Content})
			}
		case "assistant", "model":
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
		case "tool":
			role = "function"
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

	return geminiReq, nil
}

// toContentResponse translates a Gemini-specific response to our universal one.
func (c *geminiClient) toContentResponse(resp *geminiGenerateContentResponse) (*Response, error) {
	if len(resp.Candidates) == 0 {
		return &Response{}, nil
	}

	candidate := resp.Candidates[0]
	universalResp := &Response{}

	if len(candidate.Content.Parts) > 0 {
		part := candidate.Content.Parts[0]
		if part.Text != nil {
			universalResp.Text = *part.Text
		}
		if part.FunctionCall != nil {
			args, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal gemini function call args: %w", err)
			}
			universalResp.ToolCalls = []ToolCall{
				{
					ID:        fmt.Sprintf("gemini-tool-call-%d", time.Now().UnixNano()),
					Type:      "function",
					Function:  part.FunctionCall.Name,
					Arguments: string(args),
				},
			}
		}
	}

	return universalResp, nil
}

// callGeminiAPI is the internal method that calls the Gemini API.
func (c *geminiClient) callGeminiAPI(ctx context.Context, model string, req *geminiGenerateContentRequest) (*geminiGenerateContentResponse, error) {
	path := fmt.Sprintf("/models/%s:generateContent", model)
	var resp geminiGenerateContentResponse
	err := c.doJSONRequest(ctx, "POST", path, req, &resp)
	return &resp, err
}

// doJSONRequest is the core HTTP request helper for the Gemini client.
func (c *geminiClient) doJSONRequest(ctx context.Context, method, path string, reqBody, respBody any) error {
	var body io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonBody)
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path, err = url.JoinPath(u.Path, c.apiVersion, path)
	if err != nil {
		return fmt.Errorf("failed to join URL path: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.apiKey)
	var httpResp *http.Response
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		httpResp, err = c.httpClient.Do(httpReq)
		if err == nil && httpResp.StatusCode < 500 {
			break
		}
		if attempt < c.maxRetries {
			time.Sleep(1 * time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, err)
	}
	defer httpResp.Body.Close()
	respBodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	if httpResp.StatusCode >= 400 {
		var apiError geminiErrorResponse
		if err := json.Unmarshal(respBodyBytes, &apiError); err != nil {
			return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(respBodyBytes))
		}
		return fmt.Errorf("API error %d: %s", httpResp.StatusCode, apiError.Error.Message)
	}
	if respBody != nil {
		if err := json.Unmarshal(respBodyBytes, respBody); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}
	return nil
}

// --- Private Gemini Specific Types ---

type geminiGenerateContentRequest struct {
	Contents []geminiContent `json:"contents"`
	Tools    []geminiTool    `json:"tools,omitempty"`
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

type geminiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
