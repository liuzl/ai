package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// This example demonstrates using different provider API formats
// to call the same backend provider through the universal proxy.

func main() {
	proxyBaseURL := "http://localhost:8080"

	fmt.Println("=" + string(bytes.Repeat([]byte("="), 78)))
	fmt.Println("Universal API Proxy Demo")
	fmt.Println("=" + string(bytes.Repeat([]byte("="), 78)))
	fmt.Println()

	// Example 1: OpenAI format
	fmt.Println("Example 1: Using OpenAI Format")
	fmt.Println("-" + string(bytes.Repeat([]byte("-"), 78)))
	openaiRequest := map[string]any{
		"model": "gpt-4",
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'Hello from OpenAI format!'"},
		},
	}
	callProxy(proxyBaseURL+"/v1/chat/completions", openaiRequest, "OpenAI")
	fmt.Println()

	// Example 2: Gemini format
	fmt.Println("Example 2: Using Gemini Format")
	fmt.Println("-" + string(bytes.Repeat([]byte("-"), 78)))
	geminiRequest := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": "Say 'Hello from Gemini format!'"},
				},
			},
		},
	}
	callProxy(proxyBaseURL+"/v1beta/models/gemini-2.5-flash:generateContent", geminiRequest, "Gemini")
	fmt.Println()

	// Example 3: Anthropic format
	fmt.Println("Example 3: Using Anthropic Format")
	fmt.Println("-" + string(bytes.Repeat([]byte("-"), 78)))
	anthropicRequest := map[string]any{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'Hello from Anthropic format!'"},
		},
	}
	callProxy(proxyBaseURL+"/v1/messages", anthropicRequest, "Anthropic")
	fmt.Println()

	fmt.Println("=" + string(bytes.Repeat([]byte("="), 78)))
	fmt.Println("Demo Complete!")
	fmt.Println()
	fmt.Println("Note: All three requests can be routed to the SAME backend provider,")
	fmt.Println("regardless of which API format they use. This is the power of the")
	fmt.Println("universal proxy!")
	fmt.Println("=" + string(bytes.Repeat([]byte("="), 78)))
}

func callProxy(url string, request map[string]any, formatName string) {
	// Marshal request
	requestBody, err := json.Marshal(request)
	if err != nil {
		log.Printf("Failed to marshal %s request: %v", formatName, err)
		return
	}

	fmt.Printf("Request URL: %s\n", url)
	fmt.Printf("Request body: %s\n\n", string(requestBody))

	// Make HTTP request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Failed to make %s request: %v", formatName, err)
		log.Printf("Make sure the proxy server is running on port 8080!")
		return
	}
	defer resp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read %s response: %v", formatName, err)
		return
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		log.Printf("%s request failed with status %d: %s", formatName, resp.StatusCode, string(responseBody))
		return
	}

	// Parse and pretty print response
	var responseJSON map[string]any
	if err := json.Unmarshal(responseBody, &responseJSON); err != nil {
		log.Printf("Failed to parse %s response: %v", formatName, err)
		return
	}

	prettyResponse, _ := json.MarshalIndent(responseJSON, "", "  ")
	fmt.Printf("Response:\n%s\n", string(prettyResponse))

	// Extract the actual text response (format-specific)
	extractResponse(responseJSON, formatName)
}

func extractResponse(response map[string]any, formatName string) {
	switch formatName {
	case "OpenAI":
		if choices, ok := response["choices"].([]any); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]any); ok {
				if message, ok := choice["message"].(map[string]any); ok {
					if content, ok := message["content"].(string); ok {
						fmt.Printf("\n✓ Assistant's message: %s\n", content)
					}
				}
			}
		}
	case "Gemini":
		if candidates, ok := response["candidates"].([]any); ok && len(candidates) > 0 {
			if candidate, ok := candidates[0].(map[string]any); ok {
				if content, ok := candidate["content"].(map[string]any); ok {
					if parts, ok := content["parts"].([]any); ok && len(parts) > 0 {
						if part, ok := parts[0].(map[string]any); ok {
							if text, ok := part["text"].(string); ok {
								fmt.Printf("\n✓ Assistant's message: %s\n", text)
							}
						}
					}
				}
			}
		}
	case "Anthropic":
		if content, ok := response["content"].([]any); ok && len(content) > 0 {
			if block, ok := content[0].(map[string]any); ok {
				if text, ok := block["text"].(string); ok {
					fmt.Printf("\n✓ Assistant's message: %s\n", text)
				}
			}
		}
	}
}
