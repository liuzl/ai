package gemini

import (
	"context"
	"fmt"
	"os"
	"testing"

	_ "github.com/joho/godotenv/autoload"
)

func TestGenerateContentRequest(t *testing.T) {
	client := NewClient(os.Getenv("GEMINI_API_KEY"), WithBaseURL(os.Getenv("GEMINI_BASE_URL")))

	req := &GenerateContentRequest{Contents: []Content{{Parts: []Part{{Text: StringPtr("Hello, world!")}}}}}

	resp, err := client.GenerateContent(context.Background(), "gemini-2.5-flash", req)
	if err != nil {
		t.Errorf("Error generating content: %v", err)
	}
	if len(resp.Candidates) == 0 {
		t.Errorf("Expected at least one candidate, got %d", len(resp.Candidates))
	}
	fmt.Println(*resp.Candidates[0].Content.Parts[0].Text)
}
