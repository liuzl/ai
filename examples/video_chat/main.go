package main

import (
	"context"
	"fmt"
	"log"

	"github.com/liuzl/ai"
)

func main() {
	// This example demonstrates video input capabilities.
	// Video support is primarily available with Gemini models.
	//
	// Supported formats: MP4, MPEG, MOV, AVI, FLV, MPG, WEBM, WMV, 3GPP
	// Maximum duration: ~1 hour
	// Maximum file size: 2GB
	//
	// NOTE: Set AI_PROVIDER=gemini and GEMINI_API_KEY environment variables
	// Video is NOT supported by OpenAI or Anthropic APIs

	fmt.Println("=== Video Analysis Example ===")
	fmt.Println("This example requires Gemini API")
	fmt.Println()

	// Create a Gemini client
	client, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Example: Analyze a video file from a URL
	// Using a sample video (Big Buck Bunny trailer - open source film)
	videoURL := "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4"

	req := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Watch this video and describe what happens. What is the story? Describe the characters and the setting."),
				ai.NewVideoPartFromURL(videoURL, "mp4"),
			}),
		},
	}

	fmt.Println("Analyzing video...")
	fmt.Printf("Video URL: %s\n", videoURL)
	fmt.Println("(This may take a moment as the video is being processed...)")
	fmt.Println()

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("AI Response:")
	fmt.Println(resp.Text)
	fmt.Println()

	// Example 2: Extract specific information from video
	fmt.Println("=== Scene-by-Scene Analysis ===")

	req2 := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Analyze this video and provide:\n1. A timeline of key scenes\n2. Description of the main character\n3. The overall mood and tone\n4. Any text or captions visible in the video"),
				ai.NewVideoPartFromURL(videoURL, "mp4"),
			}),
		},
	}

	resp2, err := client.Generate(context.Background(), req2)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Detailed Analysis:")
	fmt.Println(resp2.Text)
}
