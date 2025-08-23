# Go AI

A Go library providing a unified, provider-agnostic interface for interacting with multiple AI models, including Google Gemini, OpenAI, and Anthropic. This library simplifies content generation and tool integration, allowing you to switch between AI providers with minimal code changes.

It also features built-in support for the [Model-Context Protocol (MCP)](https://github.com/modelcontextprotocol), enabling seamless integration with external tool servers.

## Features

- **Unified Client Interface**: A single `ai.Client` interface for Google Gemini, OpenAI, and Anthropic.
- **Provider-Agnostic API**: Universal `Request`, `Response`, and `Message` structs for consistent interaction.
- **Simplified Configuration**: Easily configure clients using environment variables or functional options.
- **First-Class Tool Support**: Abstracted support for function calling (tools) that works across providers.
- **MCP Integration**: Discover, connect to, and execute tools on MCP-compliant servers.

## Installation

To add the library to your project, run:

```sh
go get github.com/liuzl/ai
```

## Configuration

The easiest way to configure the client is by setting environment variables. The library's `NewClientFromEnv()` function will automatically detect and use them.

- `AI_PROVIDER`: The provider to use. Can be `openai` (default), `gemini`, or `anthropic`.

### OpenAI

- `OPENAI_API_KEY`: Your OpenAI API key.
- `OPENAI_MODEL`: (Optional) The model name, e.g., `gpt-4o-mini`.
- `OPENAI_BASE_URL`: (Optional) For using a custom or proxy endpoint.

### Google Gemini

- `GEMINI_API_KEY`: Your Gemini API key.
- `GEMINI_MODEL`: (Optional) The model name, e.g., `gemini-2.5-flash`.
- `GEMINI_BASE_URL`: (Optional) For using a custom endpoint.

### Anthropic

- `ANTHROPIC_API_KEY`: Your Anthropic API key.
- `ANTHROPIC_MODEL`: (Optional) The model name, e.g., `claude-3-haiku-20240307`.
- `ANTHROPIC_BASE_URL`: (Optional) For using a custom endpoint.

## Usage

### Basic Example: Simple Text Generation

This example shows how to create a client from environment variables and generate a simple text response.

```go
// main.go
package main

import (
	"context"
	"fmt"
	"log"

	// Use godotenv to load .env file for local development
	_ "github.com/joho/godotenv/autoload"
	"github.com/liuzl/ai"
)

func main() {
	// Create a new client using the recommended NewClientFromEnv function.
	// This automatically reads the AI_PROVIDER and corresponding API keys.
	client, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	// Create a request for the model.
	req := &ai.Request{
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Tell me a one-sentence joke about programming."},
		},
	}

	// Call the Generate function.
	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Fatalf("Generate failed: %v", err)
	}

	// Print the result.
	fmt.Println(resp.Text)
}
```

### Running the Examples

The `examples` directory contains runnable code. To run the simple chat example, execute the following command from the root of the project:

```sh
go run ./examples/simple_chat
```

### Advanced Example: Using Tools with an MCP Server

This library can orchestrate interactions between an AI model and an external tool server that implements the Model-Context Protocol (MCP).

The following example demonstrates the full loop:
1.  Connect to an MCP server to discover available tools.
2.  Ask the AI model a question, providing the list of tools it can use.
3.  Receive a `ToolCall` from the model.
4.  Execute the `ToolCall` on the MCP server.
5.  Send the tool's result back to the model for a final, synthesized answer.

```go
package main

import (
	"context"
	"log"

	_ "github.com/joho/godotenv/autoload" // for loading .env file
	"github.com/liuzl/ai"
)

const (
	mcpServerName = "remote-shell"
	mcpServerURL  = "http://localhost:8080/mcp" // URL of a running MCP server
)

func main() {
	ctx := context.Background()

	// 1. Setup ToolServerManager and register the remote server.
	manager := ai.NewToolServerManager()
	if err := manager.AddRemoteServer(mcpServerName, mcpServerURL); err != nil {
		log.Fatalf("Failed to add remote tool server: %v", err)
	}

	// 2. Get the client for the server and defer its closing.
	toolClient, _ := manager.GetClient(mcpServerName)
	defer toolClient.Close()

	// 3. Fetch available tools. The client will connect automatically.
	aiTools, err := toolClient.FetchTools(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch tools: %v", err)
	}
	log.Printf("Found %d tools on server '%s'.\n", len(aiTools), mcpServerName)

	// 4. Create an AI client
	aiClient, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}

	// 5. Ask the model a question, making it aware of the tools
	messages := []ai.Message{
		{Role: ai.RoleUser, Content: "List all files in the current directory using the shell."},
	}
	req := &ai.Request{Messages: messages, Tools: aiTools}

	resp, err := aiClient.Generate(ctx, req)
	if err != nil {
		log.Fatalf("Initial model call failed: %v", err)
	}

	// 6. Check for a tool call and execute it
	if len(resp.ToolCalls) == 0 {
		log.Fatalf("Expected a tool call, but got text: %s", resp.Text)
	}
	toolCall := resp.ToolCalls[0]
	log.Printf("Model wants to call function '%s'.\n", toolCall.Function)
	messages = append(messages, ai.Message{Role: ai.RoleAssistant, ToolCalls: resp.ToolCalls})

	toolResult, err := toolClient.ExecuteTool(ctx, toolCall)
	if err != nil {
		log.Fatalf("Tool call failed: %v", err)
	}
	log.Printf("Tool executed successfully.\n")

	// 7. Send the result back to the model for a final answer
	messages = append(messages, ai.Message{Role: ai.RoleTool, ToolCallID: toolCall.ID, Content: toolResult})
	finalReq := &ai.Request{Messages: messages}

	finalResp, err := aiClient.Generate(ctx, finalReq)
	if err != nil {
		log.Fatalf("Final model call failed: %v", err)
	}

	// 8. Print the final, synthesized response
	log.Println("--- Final Model Response ---")
	log.Println(finalResp.Text)
	log.Println("--------------------------")
}
