package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/liuzl/ai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// --- 1. Setup AI Client ---
	// This part is from the original ai_test.go to set up the AI client.
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set the GEMINI_API_KEY environment variable.")
	}
	client, err := ai.NewClient(ai.WithProvider("gemini"), ai.WithAPIKey(apiKey), ai.WithBaseURL(os.Getenv("GEMINI_BASE_URL")))
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	// --- 2. Define the Tool for the AI Model ---
	// We describe the 'execute_shell' tool so the model knows how to use it.
	executeShellTool := ai.Tool{
		Type: "function",
		Function: ai.FunctionDefinition{
			Name:        "execute_shell",
			Description: "Run a shell command in the global working directory.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"cmd": {
						"type": "string",
						"description": "The shell command to execute."
					}
				},
				"required": ["cmd"]
			}`),
		},
	}

	// --- 3. Initial Request to the Model ---
	// We ask the model a question that should trigger the use of our tool.
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Please list all files in the current directory using the shell and return the output."},
	}
	req := &ai.Request{
		Messages: messages,
		Tools:    []ai.Tool{executeShellTool},
	}

	log.Println("Sending initial request to the model...")
	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Fatalf("Initial model call failed: %v", err)
	}

	// --- 4. Handle the Model's Tool Call ---
	if len(resp.ToolCalls) == 0 {
		log.Fatalf("Expected a tool call, but got a text response: %s", resp.Text)
	}

toolCall := resp.ToolCalls[0]
	log.Printf("Model wants to call the '%s' function with arguments: %s\n", toolCall.Function, toolCall.Arguments)

	// Append the model's response to our message history for context.
	messages = append(messages, ai.Message{Role: ai.RoleAssistant, ToolCalls: resp.ToolCalls})

	// --- 5. Execute the Tool via MCP ---
	// This is where we use the MCP Go SDK to call the actual tool on the server.
	toolResult, err := callMCPExecuteShell(toolCall.Arguments)
	if err != nil {
		log.Fatalf("MCP tool call failed: %v", err)
	}
	log.Printf("Received result from MCP server: %s\n", toolResult)

	// --- 6. Send the Tool's Result Back to the Model ---
	messages = append(messages, ai.Message{
		Role:       ai.RoleTool,
		ToolCallID: toolCall.ID,
		Content:    toolResult,
	})

	finalReq := &ai.Request{Messages: messages}
	log.Println("Sending tool result back to the model for a final answer...")
	finalResp, err := client.Generate(context.Background(), finalReq)
	if err != nil {
		log.Fatalf("Final model call failed: %v", err)
	}

	// --- 7. Print the Final, User-Facing Response ---
	log.Println("--- Final Model Response ---")
	log.Println(finalResp.Text)
	log.Println("--------------------------")
}

// callMCPExecuteShell connects to the MCP server and calls the 'execute_shell' tool.
func callMCPExecuteShell(jsonArgs string) (string, error) {
	// Unmarshal the arguments from the model (e.g., `{"cmd":"ls -l"}`)
	var args map[string]any
	if err := json.Unmarshal([]byte(jsonArgs), &args); err != nil {
		return "", fmt.Errorf("failed to unmarshal tool arguments: %w", err)
	}

	// Connect to the MCP server
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-go-sdk-example", Version: "v0.1.0"}, nil)
	transport := mcp.NewStreamableClientTransport("http://localhost:8080/mcp", nil)
	session, err := client.Connect(ctx, transport)
	if err != nil {
		return "", fmt.Errorf("failed to connect to MCP server: %v", err)
	}
	defer session.Close()

	// Call the tool with the arguments provided by the AI model
	params := &mcp.CallToolParams{
		Name:      "execute_shell",
		Arguments: args,
	}
	res, err := session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to call 'execute_shell' tool: %v", err)
	}

	// Process the response from the tool
	var toolOutput string
	if res.IsError {
		toolOutput = "Error from tool: "
	}
	for _, contentItem := range res.Content {
		if textContent, ok := contentItem.(*mcp.TextContent); ok {
			toolOutput += textContent.Text
		}
	}
	return toolOutput, nil
}
