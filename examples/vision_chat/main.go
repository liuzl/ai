package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/liuzl/ai"
)

func main() {
	// This example demonstrates multimodal (vision) capabilities.
	// You can send images along with text prompts to vision-enabled models.
	//
	// Supported models:
	// - OpenAI: gpt-4o, gpt-4o-mini, gpt-4-turbo
	// - Gemini: gemini-2.0-flash-exp, gemini-2.5-flash-preview-05-20, gemini-1.5-pro, gemini-1.5-flash
	// - Anthropic: claude-3-opus-20240229, claude-3-sonnet-20240229, claude-3-haiku-20240307,
	//              claude-3-5-sonnet-20241022, claude-3-5-haiku-20241022

	// Create a client using environment variables
	// Set AI_PROVIDER=openai (or gemini, anthropic) and corresponding API key
	client, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Example 1: Analyze an image from a URL
	fmt.Println("=== Example 1: Image from URL ===")
	runImageURLExample(client)

	// Example 2: Analyze an image from base64 data
	fmt.Println("\n=== Example 2: Image from Base64 ===")
	runImageBase64Example(client)

	// Example 3: Compare multiple images
	fmt.Println("\n=== Example 3: Compare Multiple Images ===")
	runMultiImageExample(client)
}

func runImageURLExample(client ai.Client) {
	// Create a multimodal message with text and an image URL
	req := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("What's in this image? Please describe it in detail."),
				ai.NewImagePartFromURL("https://upload.wikimedia.org/wikipedia/commons/thumb/d/dd/Gfp-wisconsin-madison-the-nature-boardwalk.jpg/2560px-Gfp-wisconsin-madison-the-nature-boardwalk.jpg"),
			}),
		},
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Response: %s\n", resp.Text)
}

func runImageBase64Example(client ai.Client) {
	// For this example, we'll create a simple 1x1 red pixel PNG
	// In a real application, you would read an actual image file
	redPixelPNG := createRedPixelPNG()

	req := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("What color is this image?"),
				ai.NewImagePartFromBase64(redPixelPNG, "png"),
			}),
		},
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Response: %s\n", resp.Text)
}

func runMultiImageExample(client ai.Client) {
	// Compare two images
	req := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Compare these two images and describe their differences:"),
				ai.NewImagePartFromURL("https://upload.wikimedia.org/wikipedia/commons/thumb/3/3a/Cat03.jpg/1200px-Cat03.jpg"),
				ai.NewImagePartFromURL("https://upload.wikimedia.org/wikipedia/commons/thumb/4/4d/Cat_November_2010-1a.jpg/1200px-Cat_November_2010-1a.jpg"),
			}),
		},
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("Response: %s\n", resp.Text)
}

// createRedPixelPNG creates a base64-encoded 1x1 red pixel PNG
func createRedPixelPNG() string {
	// This is a minimal valid PNG file (1x1 red pixel)
	pngBytes := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE,
		0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54, // IDAT chunk (red pixel)
		0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00, 0x00,
		0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D, 0xB4,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, // IEND chunk
		0xAE, 0x42, 0x60, 0x82,
	}
	return base64.StdEncoding.EncodeToString(pngBytes)
}
