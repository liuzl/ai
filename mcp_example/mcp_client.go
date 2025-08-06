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
	// --- 1. Fetch Tools Dynamically from MCP Server ---
	log.Println("Connecting to MCP server to fetch available tools...")
	mcpTools, err := listMCPTools()
	if err != nil {
		log.Fatalf("Failed to list tools from MCP server: %v", err)
	}

	// Translate MCP tools to the AI client's format
	var aiTools []ai.Tool
	for _, mcpTool := range mcpTools.Tools {
		// The MCP SDK gives us parameters as a map. We need to convert it to a JSON RawMessage for the AI client.
		paramsJSON, err := json.Marshal(mcpTool.InputSchema)
		if err != nil {
			log.Printf("Warning: could not marshal parameters for tool %s: %v. Skipping.", mcpTool.Name, err)
			continue
		}
		aiTools = append(aiTools, ai.Tool{
			Type: "function",
			Function: ai.FunctionDefinition{
				Name:        mcpTool.Name,
				Description: mcpTool.Description,
				Parameters:  json.RawMessage(paramsJSON),
			},
		})
		log.Println(string(paramsJSON))
	}
	if len(aiTools) == 0 {
		log.Fatal("No usable tools found on the MCP server.")
	}
	log.Println("--- Dynamically Loaded Tools ---")
	for _, tool := range aiTools {
		log.Printf("- %s", tool.Function.Name)
	}
	log.Println("------------------------------")

	// --- 2. Setup AI Client ---
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("Please set the GEMINI_API_KEY environment variable.")
	}
	client, err := ai.NewClient(ai.WithProvider("gemini"), ai.WithAPIKey(apiKey), ai.WithBaseURL(os.Getenv("GEMINI_BASE_URL")))
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	// --- 3. Initial Request to the Model ---
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "Please list all files in the current directory using the shell and return the output."},
	}
	req := &ai.Request{Messages: messages, Tools: aiTools}
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
	messages = append(messages, ai.Message{Role: ai.RoleAssistant, ToolCalls: resp.ToolCalls})

	// --- 5. Execute the Tool via MCP ---
	toolResult, err := callMCPTool(toolCall.Function, toolCall.Arguments)
	if err != nil {
		log.Fatalf("MCP tool call failed: %v", err)
	}
	log.Printf("Received result from MCP server: %s\n", toolResult)

	// --- 6. Send the Tool's Result Back to the Model ---
	messages = append(messages, ai.Message{Role: ai.RoleTool, ToolCallID: toolCall.ID, Content: toolResult})
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

// listMCPTools connects to the MCP server and returns the list of available tools.
func listMCPTools() (*mcp.ListToolsResult, error) {
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-go-sdk-dynamic", Version: "v0.1.0"}, nil)
	transport := mcp.NewStreamableClientTransport("http://localhost:8080/mcp", nil)
	session, err := client.Connect(ctx, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %v", err)
	}
	defer session.Close()
	return session.ListTools(ctx, &mcp.ListToolsParams{})
}

// callMCPTool connects to the MCP server and calls the specified tool by name.
func callMCPTool(functionName string, jsonArgs string) (string, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(jsonArgs), &args); err != nil {
		return "", fmt.Errorf("failed to unmarshal tool arguments: %w", err)
	}

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-go-sdk-dynamic", Version: "v0.1.0"}, nil)
	transport := mcp.NewStreamableClientTransport("http://localhost:8080/mcp", nil)
	session, err := client.Connect(ctx, transport)
	if err != nil {
		return "", fmt.Errorf("failed to connect to MCP server: %v", err)
	}
	defer session.Close()

	params := &mcp.CallToolParams{Name: functionName, Arguments: args}
	res, err := session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to call '%s' tool: %v", functionName, err)
	}

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
