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

// openaiClient implements the Client interface for the OpenAI provider.
type openaiClient struct {
	apiKey     string
	baseURL    string
	apiVersion string
	httpClient *http.Client
	maxRetries int
}

// newOpenAIClient is the internal constructor for the OpenAI client.
func newOpenAIClient(cfg *Config) Client {
	baseURL := "https://api.openai.com"
	if cfg.baseURL != "" {
		baseURL = cfg.baseURL
	}
	return &openaiClient{
		apiKey:     cfg.apiKey,
		baseURL:    baseURL,
		apiVersion: "v1",
		httpClient: &http.Client{Timeout: cfg.timeout},
		maxRetries: 3,
	}
}

// Generate implements the Client interface for OpenAI.
func (c *openaiClient) Generate(ctx context.Context, req *Request) (*Response, error) {
	// 1. Build the provider-specific request from the universal request.
	openaiReq, err := c.newOpenAIRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build openai request: %w", err)
	}

	// 2. Call the provider-specific method to get the response.
	resp, err := c.createChatCompletion(ctx, openaiReq)
	if err != nil {
		return nil, err
	}

	// 3. Convert the provider-specific response to the universal response.
	return c.toContentResponse(resp)
}

// newOpenAIRequest translates our universal request to an OpenAI-specific one.
func (c *openaiClient) newOpenAIRequest(req *Request) (*openaiChatCompletionRequest, error) {
	openaiReq := &openaiChatCompletionRequest{
		Model:    req.Model,
		Messages: make([]openaiMessage, len(req.Messages)),
	}
	if req.Model == "" {
		openaiReq.Model = "gpt-4o-mini" // Default model
	}

	for i, msg := range req.Messages {
		// Direct mapping for roles and content
		openaiReq.Messages[i] = openaiMessage{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		// Translate our universal ToolCall to OpenAI's format for assistant messages
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

	// Translate our universal Tool definition to OpenAI's format
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

	// Prepend system prompt if provided
	if req.SystemPrompt != "" {
		openaiReq.Messages = append([]openaiMessage{
			{Role: "system", Content: req.SystemPrompt},
		}, openaiReq.Messages...)
	}

	return openaiReq, nil
}

// toContentResponse translates an OpenAI-specific response to our universal one.
func (c *openaiClient) toContentResponse(resp *openaiChatCompletionResponse) (*Response, error) {
	if len(resp.Choices) == 0 {
		return &Response{}, nil
	}

	choice := resp.Choices[0]
	universalResp := &Response{
		Text: choice.Message.Content,
	}

	// If the model wants to call tools, translate them.
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

// createChatCompletion is the internal method that calls the OpenAI API.
func (c *openaiClient) createChatCompletion(ctx context.Context, req *openaiChatCompletionRequest) (*openaiChatCompletionResponse, error) {
	var resp openaiChatCompletionResponse
	err := c.doJSONRequest(ctx, "POST", "/chat/completions", req, &resp)
	return &resp, err
}

// doJSONRequest is the core HTTP request helper for the OpenAI client.
func (c *openaiClient) doJSONRequest(ctx context.Context, method, path string, reqBody, respBody any) error {
	// This implementation remains largely the same.
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
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	var httpResp *http.Response
	for attempt := range c.maxRetries {
		httpResp, err = c.httpClient.Do(httpReq)
		if err == nil && httpResp.StatusCode < 500 {
			break // Success or non-retriable error
		}
		if attempt < c.maxRetries {
			time.Sleep(1 * time.Second) // Wait before retrying
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
		var apiError openaiErrorResponse
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

// --- Private OpenAI Specific Types ---

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

type openaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
