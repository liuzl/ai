package ai

import (
	"context"
	"fmt"
)

// StreamingClient exposes streaming generation without changing the existing Client API.
// genericClient implements this interface; users can call Stream via the helper function.
type StreamingClient interface {
	Stream(ctx context.Context, req *Request) (StreamReader, error)
}

// StreamReader allows incremental consumption of a streamed response.
// Implementations must be safe for sequential Recv calls from a single goroutine.
type StreamReader interface {
	// Recv blocks until the next chunk is available or the stream ends.
	// It returns io.EOF when the stream is finished.
	Recv() (*StreamChunk, error)
	// Close should release any underlying resources (e.g., HTTP body).
	Close() error
}

// StreamChunk represents an incremental update from the provider.
type StreamChunk struct {
	// TextDelta is the incremental text returned in this chunk.
	TextDelta string
	// ToolCallDeltas contains incremental tool/function call updates.
	ToolCallDeltas []ToolCallDelta
	// Snapshot is the accumulated response after applying this chunk.
	Snapshot *Response
	// Done indicates the provider signaled completion in this chunk.
	Done bool
}

// ToolCallDelta represents incremental tool call data.
type ToolCallDelta struct {
	ID               string
	Type             string
	Function         string
	ArgumentsDelta   string
	ThoughtSignature string
	Done             bool
}

// Stream invokes streaming generation when supported by the client.
// It returns an error if the provided client does not implement streaming.
func Stream(ctx context.Context, client Client, req *Request) (StreamReader, error) {
	if sc, ok := client.(StreamingClient); ok {
		return sc.Stream(ctx, req)
	}
	return nil, fmt.Errorf("streaming not supported by this client")
}
