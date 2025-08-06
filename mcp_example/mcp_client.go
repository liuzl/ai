package main

import (
	"context"
	"log"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/liuzl/ai"
)

const (
	// The name we'll use to register and retrieve our remote server.
	mcpServerName = "remote-shell"
	// The URL of the running MCP server.
	mcpServerURL = "http://localhost:8080/mcp"
)

func main() {
	ctx := context.Background()

	// --- 1. Setup MCP Manager and Register the Remote Server ---
	log.Println("Initializing MCP Manager...")
	manager := ai.NewMCPManager()
	if err := manager.AddRemoteServer(mcpServerName, mcpServerURL); err != nil {
		log.Fatalf("Failed to add remote MCP server: %v", err)
	}
	log.Printf("Successfully registered remote server '%s' at %s", mcpServerName, mcpServerURL)

	// --- 2. Fetch Tools from the Registered Server ---
	// Get the executor for our server.
	executor, ok := manager.GetExecutor(mcpServerName)
	if !ok {
		log.Fatalf("Logic error: could not retrieve executor after registering it.")
	}

	// Connect the executor to establish a session.
	if err := executor.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to MCP server '%s': %v", mcpServerName, err)
	}
	defer executor.Close() // Ensure the session is closed.

	log.Println("Fetching available tools from server...")
	aiTools, err := executor.FetchTools(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch tools: %v", err)
	}
	if len(aiTools) == 0 {
		log.Fatal("No tools found on the MCP server.")
	}
	log.Println("--- Available Tools ---")
	for _, tool := range aiTools {
		log.Printf("- %s: %s", tool.Function.Name, tool.Function.Description)
	}
	log.Println("-----------------------")

	// --- 3. Setup AI Client ---
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set the GEMINI_API_KEY environment variable.")
	}
	client, err := ai.NewClient(ai.WithProvider("gemini"), ai.WithAPIKey(apiKey), ai.WithBaseURL(os.Getenv("GEMINI_BASE_URL")))
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	// --- 4. Orchestrate the AI Interaction ---
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Please list all files in the current directory using the shell and return the output."},
	}
	req := &ai.Request{Messages: messages, Tools: aiTools}

	log.Println("Sending initial request to the model...")
	resp, err := client.Generate(ctx, req)
	if err != nil {
		log.Fatalf("Initial model call failed: %v", err)
	}

	if len(resp.ToolCalls) == 0 {
		log.Fatalf("Expected a tool call, but got a text response: %s", resp.Text)
	}
	toolCall := resp.ToolCalls[0]
	log.Printf("Model wants to call the '%s' function with arguments: %s\n", toolCall.Function, toolCall.Arguments)
	messages = append(messages, ai.Message{Role: ai.RoleAssistant, ToolCalls: resp.ToolCalls})

	// --- 5. Execute the Tool via the Same Executor ---
	// No need to reconnect; the executor maintains the session.
	toolResult, err := executor.ExecuteTool(ctx, toolCall)
	if err != nil {
		log.Fatalf("MCP tool call failed: %v", err)
	}
	log.Printf("Received result from MCP server: %s\n", toolResult)

	// --- 6. Send the Result Back to the Model ---
	messages = append(messages, ai.Message{Role: ai.RoleTool, ToolCallID: toolCall.ID, Content: toolResult})
	finalReq := &ai.Request{Messages: messages}

	log.Println("Sending tool result back to the model for a final answer...")
	finalResp, err := client.Generate(ctx, finalReq)
	if err != nil {
		log.Fatalf("Final model call failed: %v", err)
	}

	// --- 7. Print the Final Response ---
	log.Println("--- Final Model Response ---")
	log.Println(finalResp.Text)
	log.Println("--------------------------")
}
