package ai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
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

// DecodeRequest decodes the request body into the OpenAI request struct.
func (c *OpenAIFormatConverter) DecodeRequest(r *http.Request) (any, error) {
	var req OpenAIChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI request: %w", err)
	}
	return &req, nil
}

// IsStreaming checks if the decoded request indicates a streaming response.
func (c *OpenAIFormatConverter) IsStreaming(providerReq any) bool {
	if req, ok := providerReq.(*OpenAIChatCompletionRequest); ok {
		return req.Stream
	}
	return false
}

// NewStreamHandler creates a handler for formatting streaming events.
func (c *OpenAIFormatConverter) NewStreamHandler(id string, model string) StreamEventHandler {
	return &OpenAIStreamHandler{
		ID:    id,
		Model: model,
	}
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

// --- OpenAI Stream Handler ---

type OpenAIStreamHandler struct {
	ID    string
	Model string
}

func (h *OpenAIStreamHandler) OnStart(w http.ResponseWriter, flusher http.Flusher) {}

func (h *OpenAIStreamHandler) OnChunk(w http.ResponseWriter, flusher http.Flusher, chunk *StreamChunk) error {
	payload := buildOpenAIStreamChunk(h.ID, h.Model, chunk)
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
	if chunk.Done {
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
	return nil
}

func (h *OpenAIStreamHandler) OnEnd(w http.ResponseWriter, flusher http.Flusher) {
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (h *OpenAIStreamHandler) OnError(w http.ResponseWriter, flusher http.Flusher, err error) {
	errPayload := map[string]string{"error": err.Error()}
	if b, marshalErr := json.Marshal(errPayload); marshalErr == nil {
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	flusher.Flush()
}

func buildOpenAIStreamChunk(id, model string, chunk *StreamChunk) *openAIStreamChunk {
	choice := openAIStreamChoice{
		Index: 0,
		Delta: openAIStreamDelta{},
	}
	if chunk.TextDelta != "" {
		choice.Delta.Content = append(choice.Delta.Content, openAIContentPart{
			Type: "text",
			Text: chunk.TextDelta,
		})
	}
	for _, tc := range chunk.ToolCallDeltas {
		choice.Delta.ToolCalls = append(choice.Delta.ToolCalls, openAIToolCallDelta{
			ID:   tc.ID,
			Type: tc.Type,
			Function: openAIFunctionCallDelta{
				Name:      tc.Function,
				Arguments: tc.ArgumentsDelta,
			},
		})
	}

	if chunk.Done {
		if len(choice.Delta.ToolCalls) > 0 {
			choice.FinishReason = "tool_calls"
		} else {
			choice.FinishReason = "stop"
		}
	}

	return &openAIStreamChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []openAIStreamChoice{choice},
	}
}

// Minimal OpenAI streaming chunk structs
type openAIStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
}

type openAIStreamChoice struct {
	Index        int               `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason string            `json:"finish_reason,omitempty"`
}

type openAIStreamDelta struct {
	Role      string                `json:"role,omitempty"`
	Content   []openAIContentPart   `json:"content,omitempty"`
	ToolCalls []openAIToolCallDelta `json:"tool_calls,omitempty"`
}

type openAIContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type openAIToolCallDelta struct {
	ID       string                  `json:"id,omitempty"`
	Type     string                  `json:"type,omitempty"`
	Function openAIFunctionCallDelta `json:"function"`
}

type openAIFunctionCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
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
