package ai

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// providerAdapter defines the interface for provider-specific logic,
// allowing the core client to remain generic.
type providerAdapter interface {
	// buildRequestPayload converts the universal Request into the provider-specific
	// request body struct.
	buildRequestPayload(req *Request) (any, error)

	// parseResponse converts the provider-specific JSON response body
	// into the universal Response.
	parseResponse(providerResp []byte) (*Response, error)

	// getModel returns the default model for the provider if not specified in the request.
	getModel(req *Request) string

	// getEndpoint returns the API endpoint for the generation request.
	getEndpoint(model string) string
}

// streamingAdapter is implemented by providers that support streaming.
type streamingAdapter interface {
	// enableStreaming mutates the provider-specific payload to request streaming.
	enableStreaming(payload any)
	// parseStreamEvent converts a raw SSE event into a StreamChunk.
	// The accumulator can be used to manage partial tool calls and text state.
	// The boolean indicates whether the provider signaled end-of-stream.
	parseStreamEvent(event *sseEvent, acc *streamAccumulator) (*StreamChunk, bool, error)
	// getStreamEndpoint returns the endpoint for streaming (may match getEndpoint).
	getStreamEndpoint(model string) string
}

// genericClient handles the common logic for making AI requests, delegating
// provider-specific tasks to an adapter.
type genericClient struct {
	b       *baseClient
	adapter providerAdapter
}

// Generate implements the core logic for the Client interface.
func (c *genericClient) Generate(ctx context.Context, req *Request) (*Response, error) {
	// 0. Validate the request before processing
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 1. Build the provider-specific request payload using the adapter.
	payload, err := c.adapter.buildRequestPayload(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build request payload: %w", err)
	}

	// 2. Get model and endpoint from the adapter.
	model := c.adapter.getModel(req)
	endpoint := c.adapter.getEndpoint(model)

	// 3. Make the raw HTTP request.
	respBytes, err := c.b.doRequestRaw(ctx, "POST", endpoint, payload)
	if err != nil {
		return nil, err
	}

	// 4. Convert the provider-specific response to the universal response using the adapter.
	return c.adapter.parseResponse(respBytes)
}

// Stream implements the streaming generation flow when supported by the adapter.
func (c *genericClient) Stream(ctx context.Context, req *Request) (StreamReader, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	streaming, ok := c.adapter.(streamingAdapter)
	if !ok {
		return nil, fmt.Errorf("streaming not supported by provider")
	}

	// Build provider payload
	payload, err := c.adapter.buildRequestPayload(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build request payload: %w", err)
	}
	streaming.enableStreaming(payload)

	// Determine endpoint
	model := c.adapter.getModel(req)
	endpoint := streaming.getStreamEndpoint(model)

	// Execute streaming request
	_, body, err := c.b.doStream(ctx, "POST", endpoint, payload)
	if err != nil {
		return nil, err
	}

	decoder := newSSEDecoder(body)
	reader := &genericStreamReader{
		body:    body,
		decoder: decoder,
		adapter: streaming,
		acc:     newStreamAccumulator(),
	}
	return reader, nil
}

// streamAccumulator tracks state across streaming chunks to build snapshots.
type streamAccumulator struct {
	response  Response
	toolCalls map[string]*toolCallAccumulator
	order     []string
	// anthropicBlocks tracks block metadata by index for streaming tool/text assembly.
	anthropicBlocks map[int]*anthropicBlockState
}

type toolCallAccumulator struct {
	call      ToolCall
	args      strings.Builder
	completed bool
}

type anthropicBlockState struct {
	kind     string // "text" or "tool"
	toolID   string
	toolName string
}

func newStreamAccumulator() *streamAccumulator {
	return &streamAccumulator{
		toolCalls:       make(map[string]*toolCallAccumulator),
		anthropicBlocks: make(map[int]*anthropicBlockState),
	}
}

func (a *streamAccumulator) applyChunk(chunk *StreamChunk) {
	if chunk.TextDelta != "" {
		a.response.Text += chunk.TextDelta
	}

	for _, delta := range chunk.ToolCallDeltas {
		tc := a.toolCalls[delta.ID]
		if tc == nil {
			tc = &toolCallAccumulator{
				call: ToolCall{
					ID:   delta.ID,
					Type: delta.Type,
				},
			}
			a.toolCalls[delta.ID] = tc
			a.order = append(a.order, delta.ID)
		}
		if delta.Function != "" {
			tc.call.Function = delta.Function
		}
		if delta.Type != "" {
			tc.call.Type = delta.Type
		}
		if delta.ArgumentsDelta != "" {
			tc.args.WriteString(delta.ArgumentsDelta)
		}
		if delta.Done {
			tc.completed = true
		}
	}

	a.response.ToolCalls = nil
	for _, id := range a.order {
		tc := a.toolCalls[id]
		toolCall := tc.call
		toolCall.Arguments = tc.args.String()
		a.response.ToolCalls = append(a.response.ToolCalls, toolCall)
	}
}

func (a *streamAccumulator) snapshot() *Response {
	s := a.response
	if len(a.response.ToolCalls) > 0 {
		s.ToolCalls = append([]ToolCall(nil), a.response.ToolCalls...)
	}
	return &s
}

// genericStreamReader implements StreamReader over SSE events.
type genericStreamReader struct {
	body    io.Closer
	decoder *sseDecoder
	adapter streamingAdapter
	acc     *streamAccumulator
	closed  bool
}

func (r *genericStreamReader) Recv() (*StreamChunk, error) {
	if r.closed {
		return nil, io.EOF
	}
	for {
		event, err := r.decoder.Next()
		if err != nil {
			if err == io.EOF {
				_ = r.Close()
			}
			return nil, err
		}
		chunk, done, err := r.adapter.parseStreamEvent(event, r.acc)
		if err != nil {
			_ = r.Close()
			return nil, err
		}
		if chunk != nil {
			r.acc.applyChunk(chunk)
			chunk.Snapshot = r.acc.snapshot()
			if chunk.Done {
				_ = r.Close()
			}
			return chunk, nil
		}
		if done {
			_ = r.Close()
			return nil, io.EOF
		}
		// Otherwise continue to next event.
	}
}

func (r *genericStreamReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.body.Close()
}
