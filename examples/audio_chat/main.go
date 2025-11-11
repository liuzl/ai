package main

import (
	"context"
	"fmt"
	"log"

	"github.com/liuzl/ai"
)

func main() {
	// This example demonstrates audio input capabilities.
	// Audio support is primarily available with Gemini models.
	//
	// Supported formats: MP3, WAV, AIFF, AAC, OGG, FLAC
	// Maximum duration: ~9.5 hours
	//
	// NOTE: Set AI_PROVIDER=gemini and GEMINI_API_KEY environment variables
	// Audio is NOT supported by OpenAI chat completions or Anthropic APIs

	fmt.Println("=== Audio Analysis Example ===")
	fmt.Println("This example requires Gemini API")
	fmt.Println()

	// Create a Gemini client
	client, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Example: Analyze an audio file from a URL
	// Using a sample audio file (royalty-free music)
	audioURL := "https://www2.cs.uic.edu/~i101/SoundFiles/BabyElephantWalk60.wav"

	req := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Listen to this audio and describe what you hear. What instruments can you identify? What is the mood of the music?"),
				ai.NewAudioPartFromURL(audioURL, "wav"),
			}),
		},
	}

	fmt.Println("Analyzing audio file...")
	fmt.Printf("Audio URL: %s\n\n", audioURL)

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("AI Response:")
	fmt.Println(resp.Text)
	fmt.Println()

	// Example 2: Analyze multiple aspects of audio
	fmt.Println("=== Detailed Audio Analysis ===")

	req2 := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Analyze this audio in detail:\n1. Identify the instruments\n2. Describe the tempo and rhythm\n3. What emotions does it evoke?\n4. Can you estimate the duration?"),
				ai.NewAudioPartFromURL(audioURL, "wav"),
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
