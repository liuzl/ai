package ai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// GeminiFormatConverter provides conversion between Google Gemini API format and Universal format.
// It implements the FormatConverter interface.
type GeminiFormatConverter struct{}

// NewGeminiFormatConverter creates a new Gemini format converter.
func NewGeminiFormatConverter() *GeminiFormatConverter {
	return &GeminiFormatConverter{}
}

// GetEndpoint returns the Gemini API endpoint path (note: model is typically part of the path).
func (c *GeminiFormatConverter) GetEndpoint() string {
	return "/v1beta/models/{model}:generateContent"
}

// GetProviderName returns the provider name.
func (c *GeminiFormatConverter) GetProviderName() string {
	return string(ProviderGemini)
}

// ConvertRequestFromFormat converts a Gemini request to Universal format.
func (c *GeminiFormatConverter) ConvertRequestFromFormat(providerReq any) (*Request, error) {
	geminiReq, ok := providerReq.(*GeminiGenerateContentRequest)
	if !ok {
		return nil, NewInvalidRequestError(string(ProviderGemini), "expected *GeminiGenerateContentRequest", "", nil)
	}
	return c.ConvertRequestToUniversal(geminiReq)
}

// ConvertResponseToFormat converts a Universal Response to Gemini format.
func (c *GeminiFormatConverter) ConvertResponseToFormat(universalResp *Response, originalModel string) (any, error) {
	return c.ConvertResponseToGemini(universalResp)
}

// ConvertRequestToUniversal converts a Gemini request to Universal format.
func (c *GeminiFormatConverter) ConvertRequestToUniversal(geminiReq *GeminiGenerateContentRequest) (*Request, error) {
	if geminiReq == nil {
		return nil, fmt.Errorf("gemini request cannot be nil")
	}

	universalReq := &Request{
		Messages: make([]Message, 0, len(geminiReq.Contents)),
	}

	// Extract system instruction if present
	if geminiReq.SystemInstruction != nil && len(geminiReq.SystemInstruction.Parts) > 0 {
		for _, part := range geminiReq.SystemInstruction.Parts {
			if part.Text != nil {
				universalReq.SystemPrompt += *part.Text
			}
		}
	}

	// Convert messages
	for _, content := range geminiReq.Contents {
		msg := Message{}

		// Map Gemini role to Universal role
		switch content.Role {
		case "user":
			msg.Role = RoleUser
		case "model":
			msg.Role = RoleAssistant
		default:
			msg.Role = RoleUser // fallback
		}

		// Process parts
		if len(content.Parts) > 0 {
			// Check if it's multimodal or has special content
			hasMultimodal := false
			for _, part := range content.Parts {
				if part.InlineData != nil || part.FunctionCall != nil || part.FunctionResponse != nil {
					hasMultimodal = true
					break
				}
			}

			if hasMultimodal {
				// Use ContentParts for multimodal or special content
				msg.ContentParts = make([]ContentPart, 0, len(content.Parts))
				for _, part := range content.Parts {
					if part.Text != nil {
						msg.ContentParts = append(msg.ContentParts, ContentPart{
							Type: ContentTypeText,
							Text: *part.Text,
						})
					} else if part.InlineData != nil {
						// Extract format from MIME type
						format := strings.TrimPrefix(part.InlineData.MimeType, "image/")
						msg.ContentParts = append(msg.ContentParts, ContentPart{
							Type: ContentTypeImage,
							ImageSource: &ImageSource{
								Type:   ImageSourceTypeBase64,
								Data:   part.InlineData.Data,
								Format: format,
							},
						})
					}
				}
			} else {
				// Simple text content
				for _, part := range content.Parts {
					if part.Text != nil {
						msg.Content += *part.Text
					}
				}
			}

			// Handle function calls (tool calls from assistant)
			for _, part := range content.Parts {
				if part.FunctionCall != nil {
					args, err := json.Marshal(part.FunctionCall.Args)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal function call args: %w", err)
					}
					// Generate ID for the tool call
					randBytes := make([]byte, 8)
					rand.Read(randBytes)
					msg.ToolCalls = append(msg.ToolCalls, ToolCall{
						ID:        "gemini-" + hex.EncodeToString(randBytes),
						Type:      "function",
						Function:  part.FunctionCall.Name,
						Arguments: string(args),
					})
				}
			}

			// Handle function responses (tool results from user)
			for _, part := range content.Parts {
				if part.FunctionResponse != nil {
					// This is a tool result message
					msg.Role = RoleTool
					respBytes, err := json.Marshal(part.FunctionResponse.Response)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal function response: %w", err)
					}
					msg.Content = string(respBytes)
					// Note: Gemini doesn't provide tool_call_id, so we can't set it
					break
				}
			}
		}

		universalReq.Messages = append(universalReq.Messages, msg)
	}

	// Convert tools
	if len(geminiReq.Tools) > 0 {
		universalReq.Tools = make([]Tool, 0)
		for _, tool := range geminiReq.Tools {
			for _, fnDecl := range tool.FunctionDeclarations {
				universalReq.Tools = append(universalReq.Tools, Tool{
					Type:     "function",
					Function: FunctionDefinition(fnDecl),
				})
			}
		}
	}

	return universalReq, nil
}

// ConvertResponseToGemini converts a Universal Response to Gemini format.
func (c *GeminiFormatConverter) ConvertResponseToGemini(universalResp *Response) (*GeminiGenerateContentResponse, error) {
	if universalResp == nil {
		return nil, fmt.Errorf("universal response cannot be nil")
	}

	geminiResp := &GeminiGenerateContentResponse{
		Candidates: []geminiCandidate{
			{
				Content: geminiContent{
					Parts: make([]geminiPart, 0),
					Role:  "model",
				},
			},
		},
	}

	// Add text content if present
	if universalResp.Text != "" {
		text := universalResp.Text
		geminiResp.Candidates[0].Content.Parts = append(
			geminiResp.Candidates[0].Content.Parts,
			geminiPart{Text: &text},
		)
	}

	// Add tool calls if present
	for _, tc := range universalResp.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
		}
		geminiResp.Candidates[0].Content.Parts = append(
			geminiResp.Candidates[0].Content.Parts,
			geminiPart{
				FunctionCall: &geminiFunctionCall{
					Name: tc.Function,
					Args: args,
				},
			},
		)
	}

	return geminiResp, nil
}

// --- Gemini Specific Types (Exported for format conversion) ---

// GeminiGenerateContentRequest represents a Gemini generateContent request.
type GeminiGenerateContentRequest struct {
	Contents          []geminiContent `json:"contents"`
	Tools             []geminiTool    `json:"tools,omitempty"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
}

// GeminiGenerateContentResponse represents a Gemini generateContent response.
type GeminiGenerateContentResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}
