package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/liuzl/ai"
)

// This example demonstrates how to initialize a client from environment variables
// and perform a simple text generation request.
//
// Before running, make sure you have a .env file in the root of the project with
// your provider and API key, for example:
//
// AI_PROVIDER=openai
// OPENAI_API_KEY="your-api-key-here"
// OPENAI_MODEL="gpt-4o-mini"
//
// Or for Gemini:
//
// AI_PROVIDER=gemini
// GEMINI_API_KEY="your-api-key-here"
// GEMINI_MODEL="gemini-1.5-flash"
//
func main() {
	// Load .env file from the root directory.
	// It's okay if it fails, we'll just use the environment variables that are set.
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("Warning: .env file not found, reading from environment")
	}

	// Create a new client using the recommended NewClientFromEnv function.
	// This automatically reads the AI_PROVIDER and corresponding variables.
	log.Println("Initializing AI client from environment...")
	client, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}
	log.Println("AI client initialized successfully.")

	// Create a request for the model.
	req := &ai.Request{
		// The model is optional here because NewClientFromEnv can also set it.
		// If set here, it will override the one from the environment for this specific call.
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Tell me a one-sentence joke about programming."},
		},
	}

	// Call the Generate function.
	log.Println("Sending request to the AI provider...")
	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}

	// Print the result.
	fmt.Println("\n--- AI Response ---")
	fmt.Println(resp.Text)
	fmt.Println("-------------------")

	// Also print the provider for clarity.
	fmt.Printf("(Used provider: %s)\n", os.Getenv("AI_PROVIDER"))
}
