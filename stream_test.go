package ai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIStreamingText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":[{\"type\":\"text\",\"text\":\"Hello\"}]}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":[{\"type\":\"text\",\"text\":\" world\"}]},\"finish_reason\":\"stop\"}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := NewClient(
		WithProvider(ProviderOpenAI),
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "hi"},
		},
	}

	reader, err := Stream(context.Background(), client, req)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	defer reader.Close()

	var got string
	for {
		chunk, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv error: %v", err)
		}
		got += chunk.TextDelta
	}

	if got != "Hello world" {
		t.Fatalf("unexpected stream text: %q", got)
	}
}

func TestAnthropicStreamingText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		fmt.Fprintf(w, "event: content_block_start\ndata: {\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: content_block_delta\ndata: {\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: content_block_delta\ndata: {\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" there\"}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: message_delta\ndata: {\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: message_stop\ndata: {}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := NewClient(
		WithProvider(ProviderAnthropic),
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "hi"},
		},
	}

	reader, err := Stream(context.Background(), client, req)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	defer reader.Close()

	var got string
	for {
		chunk, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv error: %v", err)
		}
		got += chunk.TextDelta
	}

	if got != "Hi there" {
		t.Fatalf("unexpected stream text: %q", got)
	}
}

func TestGeminiStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-2.5-flash:streamGenerateContent" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"do\",\"args\":{\"x\":1}}}]}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"!\"}]},\"finishReason\":\"STOP\"}]}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client, err := NewClient(
		WithProvider(ProviderGemini),
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "hi"},
		},
	}

	reader, err := Stream(context.Background(), client, req)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	defer reader.Close()

	var got string
	var finalSnap *Response
	for {
		chunk, err := reader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv error: %v", err)
		}
		got += chunk.TextDelta
		finalSnap = chunk.Snapshot
	}

	if got != "Hello!" {
		t.Fatalf("unexpected stream text: %q", got)
	}
	if finalSnap == nil || len(finalSnap.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call in snapshot, got %+v", finalSnap)
	}
	if finalSnap.ToolCalls[0].Function != "do" || finalSnap.ToolCalls[0].Arguments != `{"x":1}` {
		t.Fatalf("unexpected tool call: %+v", finalSnap.ToolCalls[0])
	}
}
