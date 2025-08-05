package openai

import "encoding/json"

// ChatCompletionRequest represents the request body for the chat completions API.
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	ToolChoice  any       `json:"tool_choice,omitempty"` // Can be "none", "auto", or a specific tool like {"type": "function", "function": {"name": "my_function"}}
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

// Message represents a single message in the chat history.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"` // Note: Can be empty for assistant messages with tool calls
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // Used for the "tool" role message
}

// Tool represents a tool the model can call.
type Tool struct {
	Type     string             `json:"type"` // Currently only "function"
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition defines a function that can be called by the model.
type FunctionDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Parameters is a JSON Schema object.
	Parameters json.RawMessage `json:"parameters"`
}

// ChatCompletionResponse represents the response from the chat completions API.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a single completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"` // Will be "tool_calls" when the model wants to call a tool
}

// ToolCall represents a call to a tool from the model.
type ToolCall struct {
	ID       string   `json:"id"`   // This ID must be sent back in the "tool" role message
	Type     string   `json:"type"` // "function"
	Function Function `json:"function"`
}

// Function represents the function call details from the model.
type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // A JSON string of the arguments
}

// Usage represents the token usage for a request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ErrorResponse represents an error response from the OpenAI API.
type ErrorResponse struct {
	Error Error `json:"error"`
}

// Error represents the error details.
type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    any    `json:"code,omitempty"`
}
