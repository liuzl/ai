package ai

import (
	"encoding/json"
	"fmt"
)

// ConvertRequest converts a provider-specific request payload into the universal Request format.
func ConvertRequest(sourceFormat string, payload []byte) (*Request, error) {
	switch sourceFormat {
	case string(ProviderOpenAI):
		return fromOpenAIFormat(payload)
	case string(ProviderGemini):
		return fromGeminiFormat(payload)
	case string(ProviderAnthropic):
		return fromAnthropicFormat(payload)
	default:
		return nil, fmt.Errorf("unsupported provider format: %s", sourceFormat)
	}
}

// ConvertResponse converts a universal Response into a provider-specific response payload.
func ConvertResponse(targetFormat string, resp *Response) ([]byte, error) {
	switch targetFormat {
	case string(ProviderOpenAI):
		return toOpenAIFormat(resp)
	case string(ProviderGemini):
		return toGeminiFormat(resp)
	case string(ProviderAnthropic):
		return toAnthropicFormat(resp)
	default:
		return nil, fmt.Errorf("unsupported provider format: %s", targetFormat)
	}
}

// --- OpenAI Conversion ---

type openAIRequestFormat struct {
	Model    string                `json:"model"`
	Messages []openAIMessageFormat `json:"messages"`
	Tools    []openAIToolFormat    `json:"tools,omitempty"`
}

type openAIMessageFormat struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content"`
	ToolCalls  []openAIToolCallFormat `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
}

type openAIToolCallFormat struct {
	ID       string                   `json:"id"`
	Type     string                   `json:"type"`
	Function openAIFunctionCallFormat `json:"function"`
}

type openAIFunctionCallFormat struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIToolFormat struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type openAIResponseFormat struct {
	Choices []struct {
		Message struct {
			Role      string                 `json:"role"`
			Content   string                 `json:"content,omitempty"`
			ToolCalls []openAIToolCallFormat `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
}

func fromOpenAIFormat(payload []byte) (*Request, error) {
	var openaiReq openAIRequestFormat
	if err := json.Unmarshal(payload, &openaiReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OpenAI request: %w", err)
	}

	req := &Request{
		Model: openaiReq.Model,
	}

	for _, msg := range openaiReq.Messages {
		aiMsg := Message{
			Role:       Role(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}

		for _, tc := range msg.ToolCalls {
			aiMsg.ToolCalls = append(aiMsg.ToolCalls, ToolCall{
				ID:        tc.ID,
				Type:      tc.Type,
				Function:  tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
		req.Messages = append(req.Messages, aiMsg)
	}

	for _, tool := range openaiReq.Tools {
		req.Tools = append(req.Tools, Tool(tool))
	}

	return req, nil
}

func toOpenAIFormat(resp *Response) ([]byte, error) {
	openaiResp := openAIResponseFormat{
		Choices: []struct {
			Message struct {
				Role      string                 `json:"role"`
				Content   string                 `json:"content,omitempty"`
				ToolCalls []openAIToolCallFormat `json:"tool_calls,omitempty"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Role      string                 `json:"role"`
					Content   string                 `json:"content,omitempty"`
					ToolCalls []openAIToolCallFormat `json:"tool_calls,omitempty"`
				}{
					Role:    "assistant",
					Content: resp.Text,
				},
			},
		},
	}

	if len(resp.ToolCalls) > 0 {
		toolCalls := make([]openAIToolCallFormat, len(resp.ToolCalls))
		for i, tc := range resp.ToolCalls {
			toolCalls[i] = openAIToolCallFormat{
				ID:   tc.ID,
				Type: tc.Type,
				Function: openAIFunctionCallFormat{
					Name:      tc.Function,
					Arguments: tc.Arguments,
				},
			}
		}
		openaiResp.Choices[0].Message.ToolCalls = toolCalls
		// If there are tool calls, content can be empty
		openaiResp.Choices[0].Message.Content = ""
	}

	return json.Marshal(openaiResp)
}

// --- Gemini Conversion ---

type geminiRequestFormat struct {
	Contents []geminiContentFormat `json:"contents"`
}

type geminiContentFormat struct {
	Role  string             `json:"role"`
	Parts []geminiPartFormat `json:"parts"`
}

type geminiPartFormat struct {
	Text         *string                   `json:"text,omitempty"`
	FunctionCall *geminiFunctionCallFormat `json:"functionCall,omitempty"`
}

type geminiFunctionCallFormat struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type geminiResponseFormat struct {
	Candidates []struct {
		Content struct {
			Role  string             `json:"role"`
			Parts []geminiPartFormat `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func fromGeminiFormat(payload []byte) (*Request, error) {
	var geminiReq geminiRequestFormat
	if err := json.Unmarshal(payload, &geminiReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Gemini request: %w", err)
	}

	req := &Request{}

	for _, content := range geminiReq.Contents {
		msg := Message{
			Role: Role(content.Role),
		}

		for _, part := range content.Parts {
			if part.Text != nil {
				msg.Content = *part.Text
			}
			if part.FunctionCall != nil {
				argsBytes, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal function call args: %w", err)
				}

				msg.ToolCalls = append(msg.ToolCalls, ToolCall{
					Function:  part.FunctionCall.Name,
					Arguments: string(argsBytes),
				})
			}
		}
		req.Messages = append(req.Messages, msg)
	}

	return req, nil
}

func toGeminiFormat(resp *Response) ([]byte, error) {
	parts := []geminiPartFormat{}
	if resp.Text != "" {
		parts = append(parts, geminiPartFormat{Text: &resp.Text})
	}

	for _, tc := range resp.ToolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
		}
		parts = append(parts, geminiPartFormat{
			FunctionCall: &geminiFunctionCallFormat{
				Name: tc.Function,
				Args: args,
			},
		})
	}

	geminiResp := geminiResponseFormat{
		Candidates: []struct {
			Content struct {
				Role  string             `json:"role"`
				Parts []geminiPartFormat `json:"parts"`
			} `json:"content"`
		}{
			{
				Content: struct {
					Role  string             `json:"role"`
					Parts []geminiPartFormat `json:"parts"`
				}{
					Role:  "model",
					Parts: parts,
				},
			},
		},
	}

	return json.Marshal(geminiResp)
}

// --- Anthropic Conversion ---

type anthropicRequestFormat struct {
	Model     string                   `json:"model"`
	Messages  []anthropicMessageFormat `json:"messages"`
	MaxTokens int                      `json:"max_tokens"`
}

type anthropicMessageFormat struct {
	Role    string                        `json:"role"`
	Content []anthropicContentBlockFormat `json:"content"`
}

type anthropicContentBlockFormat struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
}

type anthropicResponseFormat struct {
	Content    []anthropicContentBlockFormat `json:"content"`
	StopReason string                        `json:"stop_reason"`
}

func fromAnthropicFormat(payload []byte) (*Request, error) {
	var anthropicReq anthropicRequestFormat
	if err := json.Unmarshal(payload, &anthropicReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Anthropic request: %w", err)
	}

	req := &Request{
		Model: anthropicReq.Model,
	}

	for _, msg := range anthropicReq.Messages {
		aiMsg := Message{
			Role: Role(msg.Role),
		}

		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				aiMsg.Content = block.Text
			case "tool_use":
				argsBytes, err := json.Marshal(block.Input)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool use input: %w", err)
				}

				toolCallID := block.ID
				if toolCallID == "" {
					toolCallID = block.ToolUseID
				}
				aiMsg.ToolCalls = append(aiMsg.ToolCalls, ToolCall{
					ID:        toolCallID,
					Function:  block.Name,
					Arguments: string(argsBytes),
				})
			case "tool_result":
				toolCallID := block.ID
				if toolCallID == "" {
					toolCallID = block.ToolUseID
				}
				aiMsg.ToolCallID = toolCallID
				aiMsg.Content = block.Text
			}
		}
		req.Messages = append(req.Messages, aiMsg)
	}

	return req, nil
}

func toAnthropicFormat(resp *Response) ([]byte, error) {
	content := []anthropicContentBlockFormat{}
	if resp.Text != "" {
		content = append(content, anthropicContentBlockFormat{
			Type: "text",
			Text: resp.Text,
		})
	}

	for _, tc := range resp.ToolCalls {
		var input map[string]any
		if err := json.Unmarshal([]byte(tc.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
		}
		content = append(content, anthropicContentBlockFormat{
			Type:      "tool_use",
			ToolUseID: tc.ID,
			Name:      tc.Function,
			Input:     input,
		})
	}

	anthropicResp := anthropicResponseFormat{
		Content:    content,
		StopReason: "end_turn",
	}

	return json.Marshal(anthropicResp)
}
