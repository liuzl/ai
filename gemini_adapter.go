package ai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// geminiAdapter implements the providerAdapter interface for Google Gemini.
type geminiAdapter struct{}

func (a *geminiAdapter) getModel(req *Request) string {
	if req.Model == "" {
		return "gemini-2.5-flash"
	}
	return req.Model
}

func (a *geminiAdapter) getEndpoint(model string) string {
	return fmt.Sprintf("/models/%s:generateContent", model)
}

func (a *geminiAdapter) getStreamEndpoint(model string) string {
	return fmt.Sprintf("/models/%s:streamGenerateContent", model)
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
			// Handle multimodal content if present
			if len(msg.ContentParts) > 0 {
				for _, part := range msg.ContentParts {
					switch part.Type {
					case ContentTypeText:
						parts = append(parts, geminiPart{Text: &part.Text})
					case ContentTypeImage:
						if part.ImageSource != nil {
							var data, format string
							var err error

							switch part.ImageSource.Type {
							case ImageSourceTypeBase64:
								// Use provided base64 data
								data = part.ImageSource.Data
								format = part.ImageSource.Format
								// Remove data URI prefix if present
								if strings.HasPrefix(data, "data:") {
									if idx := strings.Index(data, ","); idx != -1 {
										data = data[idx+1:]
									}
								}
							case ImageSourceTypeURL:
								// Gemini doesn't support URLs directly, so download and convert to base64
								// Use a background context with timeout for image download
								ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
								defer cancel()
								data, format, err = downloadImageToBase64(ctx, part.ImageSource.URL, 30*time.Second)
								if err != nil {
									return nil, fmt.Errorf("failed to download image from URL for Gemini: %w", err)
								}
							}

							if data != "" {
								// Determine MIME type
								mimeType := "image/png" // default
								if format != "" {
									mimeType = "image/" + format
									if format == "jpg" {
										mimeType = "image/jpeg"
									}
								}
								parts = append(parts, geminiPart{
									InlineData: &geminiInlineData{
										MimeType: mimeType,
										Data:     data,
									},
								})
							}
						}
					case ContentTypeAudio:
						if part.AudioSource != nil {
							var data string
							format := part.AudioSource.Format

							switch part.AudioSource.Type {
							case MediaSourceTypeBase64:
								data = part.AudioSource.Data
							case MediaSourceTypeURL:
								// Download audio from URL
								ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
								defer cancel()
								downloaded, err := downloadMediaToBase64(ctx, part.AudioSource.URL)
								if err != nil {
									return nil, fmt.Errorf("failed to download audio from URL for Gemini: %w", err)
								}
								data = downloaded
							}

							if data != "" {
								// Determine MIME type based on format
								mimeType := "audio/" + format
								if format == "mp3" {
									mimeType = "audio/mpeg"
								}
								parts = append(parts, geminiPart{
									InlineData: &geminiInlineData{
										MimeType: mimeType,
										Data:     data,
									},
								})
							}
						}
					case ContentTypeVideo:
						if part.VideoSource != nil {
							var data string
							format := part.VideoSource.Format

							switch part.VideoSource.Type {
							case MediaSourceTypeBase64:
								data = part.VideoSource.Data
							case MediaSourceTypeURL:
								// Download video from URL
								ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
								defer cancel()
								downloaded, err := downloadMediaToBase64(ctx, part.VideoSource.URL)
								if err != nil {
									return nil, fmt.Errorf("failed to download video from URL for Gemini: %w", err)
								}
								data = downloaded
							}

							if data != "" {
								// Determine MIME type based on format
								mimeType := "video/" + format
								if format == "3gpp" {
									mimeType = "video/3gpp"
								}
								parts = append(parts, geminiPart{
									InlineData: &geminiInlineData{
										MimeType: mimeType,
										Data:     data,
									},
								})
							}
						}
					case ContentTypeDocument:
						if part.DocumentSource != nil {
							var data string
							mimeType := part.DocumentSource.MimeType

							switch part.DocumentSource.Type {
							case MediaSourceTypeBase64:
								data = part.DocumentSource.Data
							case MediaSourceTypeURL:
								// Download document from URL
								ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
								defer cancel()
								downloaded, err := downloadMediaToBase64(ctx, part.DocumentSource.URL)
								if err != nil {
									return nil, fmt.Errorf("failed to download document from URL for Gemini: %w", err)
								}
								data = downloaded
							}

							if data != "" && mimeType != "" {
								parts = append(parts, geminiPart{
									InlineData: &geminiInlineData{
										MimeType: mimeType,
										Data:     data,
									},
								})
							}
						}
					default:
						return nil, fmt.Errorf("gemini provider does not support content type: %s", part.Type)
					}
				}
			} else if msg.Content != "" {
				// Backward compatibility: simple text content
				parts = append(parts, geminiPart{Text: &msg.Content})
			}
		case RoleAssistant:
			role = "model"
			// Handle multimodal content if present
			if len(msg.ContentParts) > 0 {
				for _, part := range msg.ContentParts {
					if part.Type == ContentTypeText {
						parts = append(parts, geminiPart{Text: &part.Text})
					}
				}
			} else if msg.Content != "" {
				// Backward compatibility: simple text content
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

func (a *geminiAdapter) enableStreaming(payload any) {
	// Gemini uses a dedicated streaming endpoint; no payload changes needed.
}

func (a *geminiAdapter) parseStreamEvent(event *sseEvent, acc *streamAccumulator) (*StreamChunk, bool, error) {
	if len(event.Data) == 0 {
		return nil, false, nil
	}

	// Some implementations may send "[DONE]" or empty arrays; treat as end.
	if string(event.Data) == "[DONE]" {
		return &StreamChunk{Done: true}, true, nil
	}

	var chunkResp geminiStreamResponse
	if err := json.Unmarshal(event.Data, &chunkResp); err != nil {
		// Some responses are wrapped in an array; try to decode that.
		var arr []geminiStreamResponse
		if errArr := json.Unmarshal(event.Data, &arr); errArr == nil && len(arr) > 0 {
			chunkResp = arr[0]
		} else {
			return nil, false, fmt.Errorf("failed to unmarshal gemini stream event: %w", err)
		}
	}

	if chunkResp.Done {
		return &StreamChunk{Done: true}, true, nil
	}

	if len(chunkResp.Candidates) == 0 {
		return nil, false, nil
	}

	candidate := chunkResp.Candidates[0]
	chunk := &StreamChunk{}

	for _, part := range candidate.Content.Parts {
		if part.Text != nil {
			chunk.TextDelta += *part.Text
		}
		if part.FunctionCall != nil {
			args, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, false, fmt.Errorf("failed to marshal gemini stream function call args: %w", err)
			}

			// Try to reuse an existing in-progress tool call with the same function.
			id := ""
			for existingID, tc := range acc.toolCalls {
				if tc.call.Function == part.FunctionCall.Name && !tc.completed {
					id = existingID
					break
				}
			}
			if id == "" {
				id = fmt.Sprintf("gemini-tool-call-%d", len(acc.toolCalls)+1)
			}

			chunk.ToolCallDeltas = append(chunk.ToolCallDeltas, ToolCallDelta{
				ID:             id,
				Type:           "function",
				Function:       part.FunctionCall.Name,
				ArgumentsDelta: string(args),
				Done:           true,
			})
		}
	}

	done := candidate.FinishReason != ""
	if done {
		chunk.Done = true
	}

	if chunk.TextDelta == "" && len(chunk.ToolCallDeltas) == 0 && !chunk.Done {
		return nil, false, nil
	}

	return chunk, done, nil
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
	InlineData       *geminiInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"` // e.g., "image/png", "image/jpeg"
	Data     string `json:"data"`     // Base64-encoded image data
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
	// FinishReason is only used for streaming responses.
	FinishReason string `json:"finishReason,omitempty"`
}

// geminiStreamResponse mirrors the streaming payload shape.
type geminiStreamResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Done       bool              `json:"done,omitempty"`
}
