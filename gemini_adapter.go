package ai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
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

func (a *geminiAdapter) newStreamDecoder(r io.Reader) streamDecoder {
	// Gemini uses JSON array format: [{obj1},{obj2},{obj3}]
	return newJSONArrayDecoder(r)
}

// buildRequestPayload converts the universal Request into the provider-specific
// request body struct. It handles parallel downloading of external media resources.
func (a *geminiAdapter) buildRequestPayload(ctx context.Context, req *Request) (any, error) {
	// 1. Prepare skeleton contents and identify download tasks
	contents, tasks, err := a.prepareContents(req)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare contents: %w", err)
	}

	// 2. Execute downloads in parallel
	if len(tasks) > 0 {
		if err := a.executeDownloads(ctx, tasks); err != nil {
			return nil, fmt.Errorf("failed to download media: %w", err)
		}
	}

	// 3. Assemble final request
	geminiReq := &geminiGenerateContentRequest{
		Contents: contents,
	}

	// Tools
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

	// System Prompt
	if req.SystemPrompt != "" {
		geminiReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{
				{Text: &req.SystemPrompt},
			},
		}
	}

	// Configuration
	geminiReq.GenerationConfig = &geminiGenConfig{
		MaxOutputTokens: 8192,
	}

	return geminiReq, nil
}

// downloadTask represents a pending media download operation.
type downloadTask struct {
	URL        string
	Type       ContentType
	TargetPart *geminiPart // Pointer to the part to populate upon success
}

func (a *geminiAdapter) prepareContents(req *Request) ([]geminiContent, []*downloadTask, error) {
	contents := make([]geminiContent, len(req.Messages))
	var allTasks []*downloadTask

	for i, msg := range req.Messages {
		role := a.mapRole(msg.Role)
		parts, tasks, err := a.processMessageParts(msg, req.Messages, i)
		if err != nil {
			return nil, nil, fmt.Errorf("message[%d]: %w", i, err)
		}

		contents[i] = geminiContent{
			Role:  role,
			Parts: parts,
		}
		allTasks = append(allTasks, tasks...)
	}

	return contents, allTasks, nil
}

func (a *geminiAdapter) mapRole(role Role) string {
	switch role {
	case RoleUser, RoleTool:
		return "user"
	case RoleAssistant:
		return "model"
	default:
		return "user"
	}
}

func (a *geminiAdapter) processMessageParts(msg Message, allMsgs []Message, msgIdx int) ([]geminiPart, []*downloadTask, error) {
	var parts []geminiPart
	var tasks []*downloadTask

	// 1. Handle ContentParts (Multimodal)
	if len(msg.ContentParts) > 0 {
		for _, part := range msg.ContentParts {
			p, t, err := a.processSinglePart(part)
			if err != nil {
				return nil, nil, err
			}
			parts = append(parts, p)
			if t != nil {
				tasks = append(tasks, t)
			}
		}
	} else if msg.Content != "" && msg.Role != RoleTool {
		// 2. Handle Simple Text (Backward Compatibility)
		// Tool messages handle content differently below
		parts = append(parts, geminiPart{Text: &msg.Content})
	}

	// 3. Handle Tool Calls (Assistant -> Model)
	if msg.Role == RoleAssistant && len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
				return nil, nil, fmt.Errorf("invalid tool call arguments: %w", err)
			}
			parts = append(parts, geminiPart{
				FunctionCall: &geminiFunctionCall{
					Name: tc.Function,
					Args: args,
				},
			})
		}
	}

	// 4. Handle Tool Responses (Tool -> User)
	if msg.Role == RoleTool {
		// Find matching tool call in previous messages
		// Note: This relies on the convention that tool response follows tool call.
		// In complex histories, we might need a better lookup, but this matches original logic.
		var matchingToolCall *ToolCall
		if msgIdx > 0 {
			prevMsg := allMsgs[msgIdx-1]
			for _, tc := range prevMsg.ToolCalls {
				if tc.ID == msg.ToolCallID {
					matchingToolCall = &tc
					break
				}
			}
		}

		if matchingToolCall != nil {
			var responseData map[string]any
			if err := json.Unmarshal([]byte(msg.Content), &responseData); err != nil {
				// Wrap raw content if not JSON
				responseData = map[string]any{"content": msg.Content}
			}
			parts = append(parts, geminiPart{
				FunctionResponse: &geminiFunctionResponse{
					Name:     matchingToolCall.Function,
					Response: responseData,
				},
			})
		} else {
			// Fallback if no matching tool call found (should generally be validated against)
			// Treat as simple text
			if msg.Content != "" {
				parts = append(parts, geminiPart{Text: &msg.Content})
			}
		}
	}

	return parts, tasks, nil
}

func (a *geminiAdapter) processSinglePart(part ContentPart) (geminiPart, *downloadTask, error) {
	switch part.Type {
	case ContentTypeText:
		return geminiPart{Text: &part.Text}, nil, nil

	case ContentTypeImage:
		if part.ImageSource == nil {
			return geminiPart{}, nil, nil
		}
		if part.ImageSource.Type == ImageSourceTypeURL {
			// Create placeholder part to be filled by download task
			p := geminiPart{InlineData: &geminiInlineData{}}
			return p, &downloadTask{
				URL:        part.ImageSource.URL,
				Type:       ContentTypeImage,
				TargetPart: &p,
			}, nil
		} else {
			// Handle Base64 immediately
			data := cleanBase64(part.ImageSource.Data)
			mimeType := "image/png"
			if part.ImageSource.Format != "" {
				mimeType = "image/" + part.ImageSource.Format
				if part.ImageSource.Format == "jpg" {
					mimeType = "image/jpeg"
				}
			}
			return geminiPart{InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     data,
			}}, nil, nil
		}

	case ContentTypeAudio:
		if part.AudioSource == nil {
			return geminiPart{}, nil, nil
		}
		if part.AudioSource.Type == MediaSourceTypeURL {
			p := geminiPart{InlineData: &geminiInlineData{}}
			// Store format in MimeType temporarily or deduce later?
			// The download task needs to know the expected format to set MimeType correctly
			// We can set a temporary MimeType based on format and fix it if needed
			mimeType := "audio/" + part.AudioSource.Format
			if part.AudioSource.Format == "mp3" {
				mimeType = "audio/mpeg"
			}
			p.InlineData.MimeType = mimeType

			return p, &downloadTask{
				URL:        part.AudioSource.URL,
				Type:       ContentTypeAudio,
				TargetPart: &p,
			}, nil
		} else {
			mimeType := "audio/" + part.AudioSource.Format
			if part.AudioSource.Format == "mp3" {
				mimeType = "audio/mpeg"
			}
			return geminiPart{InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     part.AudioSource.Data,
			}}, nil, nil
		}

	case ContentTypeVideo:
		if part.VideoSource == nil {
			return geminiPart{}, nil, nil
		}
		if part.VideoSource.Type == MediaSourceTypeURL {
			p := geminiPart{InlineData: &geminiInlineData{}}
			mimeType := "video/" + part.VideoSource.Format
			if part.VideoSource.Format == "3gpp" {
				mimeType = "video/3gpp"
			}
			p.InlineData.MimeType = mimeType

			return p, &downloadTask{
				URL:        part.VideoSource.URL,
				Type:       ContentTypeVideo,
				TargetPart: &p,
			}, nil
		} else {
			mimeType := "video/" + part.VideoSource.Format
			if part.VideoSource.Format == "3gpp" {
				mimeType = "video/3gpp"
			}
			return geminiPart{InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     part.VideoSource.Data,
			}}, nil, nil
		}

	case ContentTypeDocument:
		if part.DocumentSource == nil {
			return geminiPart{}, nil, nil
		}
		if part.DocumentSource.Type == MediaSourceTypeURL {
			p := geminiPart{InlineData: &geminiInlineData{MimeType: part.DocumentSource.MimeType}}
			return p, &downloadTask{
				URL:        part.DocumentSource.URL,
				Type:       ContentTypeDocument,
				TargetPart: &p,
			}, nil
		} else {
			return geminiPart{InlineData: &geminiInlineData{
				MimeType: part.DocumentSource.MimeType,
				Data:     part.DocumentSource.Data,
			}}, nil, nil
		}

	default:
		return geminiPart{}, nil, fmt.Errorf("unsupported content type: %s", part.Type)
	}
}

func cleanBase64(data string) string {
	if strings.HasPrefix(data, "data:") {
		if idx := strings.Index(data, ","); idx != -1 {
			return data[idx+1:]
		}
	}
	return data
}

func (a *geminiAdapter) executeDownloads(ctx context.Context, tasks []*downloadTask) error {
	var wg sync.WaitGroup
	// Buffered channel to collect first error
	errChan := make(chan error, len(tasks))
	// Semaphore to limit concurrency (prevent fd exhaustion)
	sem := make(chan struct{}, 5)

	for _, task := range tasks {
		wg.Add(1)
		go func(t *downloadTask) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return // Context cancelled, skip
			}
			defer func() { <-sem }()

			// Check context again
			if ctx.Err() != nil {
				return
			}

			var data, format string
			var err error

			// Use appropriate downloader based on type
			// Note: We use a shorter timeout for individual downloads if needed,
			// but relying on parent context is usually better.
			// We'll trust the parent context to handle overall timeout.
			switch t.Type {
			case ContentTypeImage:
				data, format, err = downloadImageToBase64(ctx, t.URL)
				if err == nil && t.TargetPart.InlineData.MimeType == "" {
					// Detect mimetype if not already set (for images)
					t.TargetPart.InlineData.MimeType = "image/" + format
					if format == "jpg" {
						t.TargetPart.InlineData.MimeType = "image/jpeg"
					}
				}
			default:
				// Audio, Video, Document use generic downloader
				data, err = downloadMediaToBase64(ctx, t.URL)
			}

			if err != nil {
				// Non-blocking send to error channel
				select {
				case errChan <- fmt.Errorf("download failed for %s: %w", t.URL, err):
				default:
				}
				return
			}

			// Assign result
			t.TargetPart.InlineData.Data = data
		}(task)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if err := <-errChan; err != nil {
		return err
	}
	// Check context one last time
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return nil
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
