package ai

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGeminiAdapter_BuildRequestPayloadIncludesThoughtSignature(t *testing.T) {
	adapter := &geminiAdapter{}
	sig := "sig-123"

	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "hi"},
			{
				Role: RoleAssistant,
				ToolCalls: []ToolCall{
					{
						Function:         "check_flight",
						Arguments:        `{"flight":"AA100"}`,
						ThoughtSignature: sig,
					},
				},
			},
		},
	}

	payload, err := adapter.buildRequestPayload(context.Background(), req)
	if err != nil {
		t.Fatalf("buildRequestPayload returned error: %v", err)
	}

	greq, ok := payload.(*geminiGenerateContentRequest)
	if !ok {
		t.Fatalf("payload type = %T, want *geminiGenerateContentRequest", payload)
	}

	if len(greq.Contents) < 2 || len(greq.Contents[1].Parts) == 0 {
		t.Fatalf("unexpected contents: %+v", greq.Contents)
	}

	if got := greq.Contents[1].Parts[0].ThoughtSignature; got != sig {
		t.Fatalf("thoughtSignature not propagated, got %q want %q", got, sig)
	}
}

func TestGeminiAdapter_ParseResponsePreservesThoughtSignature(t *testing.T) {
	adapter := &geminiAdapter{}
	sig := "sig-A"

	providerResp := geminiGenerateContentResponse{
		Candidates: []geminiCandidate{
			{
				Content: geminiContent{
					Parts: []geminiPart{
						{
							FunctionCall: &geminiFunctionCall{
								Name: "check_flight",
								Args: map[string]any{"flight": "AA100"},
							},
							ThoughtSignature: sig,
						},
					},
				},
			},
		},
	}

	raw, err := json.Marshal(providerResp)
	if err != nil {
		t.Fatalf("failed to marshal provider response: %v", err)
	}

	resp, err := adapter.parseResponse(raw)
	if err != nil {
		t.Fatalf("parseResponse returned error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ThoughtSignature != sig {
		t.Fatalf("thoughtSignature lost, got %q want %q", resp.ToolCalls[0].ThoughtSignature, sig)
	}
}

func TestGeminiAdapter_StreamThoughtSignature(t *testing.T) {
	adapter := &geminiAdapter{}
	acc := newStreamAccumulator()
	sig := "sig-stream"

	event := &sseEvent{Data: []byte(`{
		"candidates": [{
			"content": {
				"parts": [{
					"functionCall": {"name": "check_flight", "args": {"flight": "AA100"}},
					"thoughtSignature": "` + sig + `"
				}]
			}
		}]
	}`)}

	chunk, done, err := adapter.parseStreamEvent(event, acc)
	if err != nil {
		t.Fatalf("parseStreamEvent returned error: %v", err)
	}
	if done {
		t.Fatalf("expected stream not done")
	}
	if len(chunk.ToolCallDeltas) != 1 {
		t.Fatalf("expected 1 tool call delta, got %d", len(chunk.ToolCallDeltas))
	}
	if chunk.ToolCallDeltas[0].ThoughtSignature != sig {
		t.Fatalf("delta missing thoughtSignature, got %q want %q", chunk.ToolCallDeltas[0].ThoughtSignature, sig)
	}

	acc.applyChunk(chunk)
	if len(acc.response.ToolCalls) != 1 {
		t.Fatalf("expected 1 accumulated tool call, got %d", len(acc.response.ToolCalls))
	}
	if acc.response.ToolCalls[0].ThoughtSignature != sig {
		t.Fatalf("accumulated tool call missing thoughtSignature, got %q want %q", acc.response.ToolCalls[0].ThoughtSignature, sig)
	}
}
