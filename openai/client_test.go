package openai

import (
	"context"
	"fmt"
	"os"
	"testing"

	_ "github.com/joho/godotenv/autoload"
)

func TestCreateChatCompletion(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping test")
	}

	client := NewClient(apiKey, WithBaseURL(os.Getenv("OPENAI_BASE_URL")))

	req := &ChatCompletionRequest{
		Model: os.Getenv("OPENAI_MODEL"),
		Messages: []Message{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateChatCompletion failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("Expected at least one choice, but got none")
	}

	fmt.Printf("Got response: %s\n", resp.Choices[0].Message.Content)
}
