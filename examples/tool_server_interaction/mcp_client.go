package main

import (
	"context"
	"log"

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

	// --- 1. Setup Tool Server Manager and Register the Remote Server ---
	log.Println("Initializing Tool Server Manager...")
	manager := ai.NewToolServerManager()
	if err := manager.AddRemoteServer(mcpServerName, mcpServerURL); err != nil {
		log.Fatalf("Failed to add remote tool server: %v", err)
	}
	log.Printf("Successfully registered remote server '%s' at %s", mcpServerName, mcpServerURL)

	// --- 2. Fetch Tools from the Registered Server ---
	// Get the client for our server.
	client, ok := manager.GetClient(mcpServerName)
	if !ok {
		log.Fatalf("Logic error: could not retrieve client after registering it.")
	}
	// Defer Close to ensure resources are cleaned up at the end.
	defer client.Close()

	// FetchTools will automatically connect on the first call.
	log.Println("Fetching available tools from server...")
	aiTools, err := client.FetchTools(ctx)
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
	log.Println("Initializing AI client from environment variables...")
	aiClient, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}
	log.Println("AI client initialized successfully.")

	// --- 4. Orchestrate the AI Interaction ---
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Please list all files in the current directory using the shell and return the output."},
	}
	req := &ai.Request{Messages: messages, Tools: aiTools}

	log.Println("Sending initial request to the model...")
	authResp, err := aiClient.Generate(ctx, req)
	if err != nil {
		log.Fatalf("Initial model call failed: %v", err)
	}

	if len(authResp.ToolCalls) == 0 {
		log.Fatalf("Expected a tool call, but got a text response: %s", authResp.Text)
	}
	authToolCall := authResp.ToolCalls[0]
	log.Printf("Model wants to call the '%s' function with arguments: %s\n", authToolCall.Function, authToolCall.Arguments)
	messages = append(messages, ai.Message{Role: ai.RoleAssistant, ToolCalls: authResp.ToolCalls})

	// --- 5. Execute the Tool via the Same Client ---
	// No need to reconnect; the client maintains the session.
	authToolResult, err := client.ExecuteTool(ctx, authToolCall)
	if err != nil {
		log.Fatalf("Tool call failed: %v", err)
	}
	log.Printf("Received result from tool server: %s\n", authToolResult)

	// --- 6. Send the Result Back to the Model ---
	messages = append(messages, ai.Message{Role: ai.RoleTool, ToolCallID: authToolCall.ID, Content: authToolResult})
	finalReq := &ai.Request{Messages: messages}

	log.Println("Sending tool result back to the model for a final answer...")
	finalResp, err := aiClient.Generate(ctx, finalReq)
	if err != nil {
		log.Fatalf("Final model call failed: %v", err)
	}

	// --- 7. Print the Final Response ---
	log.Println("--- Final Model Response ---")
	log.Println(finalResp.Text)
	log.Println("--------------------------")
}
