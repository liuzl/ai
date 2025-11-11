package main

import (
	"context"
	"fmt"
	"log"

	"github.com/liuzl/ai"
)

func main() {
	// This example demonstrates PDF document input capabilities.
	// PDF support is available with both Gemini and Anthropic models.
	//
	// Supported providers:
	// - Gemini: Native PDF support
	// - Anthropic: PDF support
	// - OpenAI: Not natively supported (would need to extract text first)
	//
	// NOTE: Set AI_PROVIDER and corresponding API key environment variables

	fmt.Println("=== PDF Document Analysis Example ===")
	fmt.Println()

	// Create a client
	client, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Example: Analyze a PDF document from a URL
	// Using a sample PDF (publicly available research paper)
	pdfURL := "https://arxiv.org/pdf/1706.03762.pdf" // "Attention Is All You Need" paper

	req := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Read this PDF document and provide:\n1. The title and authors\n2. A brief summary of the main contribution\n3. The key innovation introduced in this paper"),
				ai.NewPDFPartFromURL(pdfURL),
			}),
		},
	}

	fmt.Println("Analyzing PDF document...")
	fmt.Printf("PDF URL: %s\n", pdfURL)
	fmt.Println("(This may take a moment as the PDF is being processed...)")
	fmt.Println()

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("AI Response:")
	fmt.Println(resp.Text)
	fmt.Println()

	// Example 2: Ask specific questions about the PDF
	fmt.Println("=== Question Answering ===")

	req2 := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Based on this paper, explain:\n1. What is the Transformer architecture?\n2. What are the advantages over RNNs?\n3. What are the key components (attention mechanism, etc.)?"),
				ai.NewPDFPartFromURL(pdfURL),
			}),
		},
	}

	resp2, err := client.Generate(context.Background(), req2)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Detailed Explanation:")
	fmt.Println(resp2.Text)
	fmt.Println()

	// Example 3: Extract specific information
	fmt.Println("=== Information Extraction ===")

	req3 := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Extract from this paper:\n1. The experimental results (BLEU scores)\n2. Training details (dataset size, training time)\n3. Model parameters count"),
				ai.NewPDFPartFromURL(pdfURL),
			}),
		},
	}

	resp3, err := client.Generate(context.Background(), req3)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("Extracted Information:")
	fmt.Println(resp3.Text)
}
