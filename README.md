# Go AI

A Go library providing a unified, provider-agnostic interface for interacting with multiple AI models, including Google Gemini, OpenAI, and Anthropic. This library simplifies content generation and tool integration, allowing you to switch between AI providers with minimal code changes.

It also features built-in support for the [Model-Context Protocol (MCP)](https://github.com/modelcontextprotocol), enabling seamless integration with external tool servers.

## Features

- **Unified Client Interface**: A single `ai.Client` interface for Google Gemini, OpenAI, and Anthropic.
- **Provider-Agnostic API**: Universal `Request`, `Response`, and `Message` structs for consistent interaction.
- **Simplified Configuration**: Easily configure clients using environment variables or functional options.
- **Multimodal Support**: Support for text, images, audio, video, and PDF documents with automatic format handling.
- **First-Class Tool Support**: Abstracted support for function calling (tools) that works across providers.
- **MCP Integration**: Discover, connect to, and execute tools on MCP-compliant servers.
- **AI Gateway**: Production-ready HTTP gateway (in `cmd/ai-gateway`) that accepts OpenAI, Gemini, or Anthropic request formats and routes to configured providers with streaming, metrics, and health checks.
- **Error Handling**: Clear error messages when using unsupported features with specific providers.

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
- `OPENAI_MODEL`: (Optional) The model name, e.g., `gpt-5-mini`.
- `OPENAI_BASE_URL`: (Optional) For using a custom or proxy endpoint.

### Google Gemini

- `GEMINI_API_KEY`: Your Gemini API key.
- `GEMINI_MODEL`: (Optional) The model name, e.g., `gemini-2.5-flash`.
- `GEMINI_BASE_URL`: (Optional) For using a custom endpoint.

### Anthropic

- `ANTHROPIC_API_KEY`: Your Anthropic API key.
- `ANTHROPIC_MODEL`: (Optional) The model name, e.g., `claude-haiku-4-5`.
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

### Streaming Responses

Streaming is available without changing the existing `Client` interface. Use the helper `ai.Stream` and consume incremental chunks:

```go
reader, err := ai.Stream(ctx, client, req)
if err != nil {
	log.Fatalf("Stream failed: %v", err)
}
defer reader.Close()

for {
	chunk, err := reader.Recv()
	if err == io.EOF {
		break
	}
	if err != nil {
		log.Fatalf("stream read error: %v", err)
	}
	fmt.Print(chunk.TextDelta)
}
```

Currently streaming is implemented for OpenAI and Anthropic providers.
Gemini is also supported via the `:streamGenerateContent` endpoint.

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
```

## Multimodal Support

The library provides comprehensive support for multiple content types beyond text, including images, audio, video, and PDF documents. Different providers support different modalities:

### Supported Content Types by Provider

| Content Type | OpenAI | Gemini | Anthropic | Notes |
|-------------|--------|--------|-----------|-------|
| **Text** | ✅ | ✅ | ✅ | Universal support |
| **Images** | ✅ | ✅ | ✅ | PNG, JPEG, WEBP, GIF |
| **Audio** | ❌ | ✅ | ❌ | MP3, WAV, AIFF, AAC, OGG, FLAC |
| **Video** | ❌ | ✅ | ❌ | MP4, MPEG, MOV, AVI, FLV, WEBM, etc. |
| **PDF Documents** | ❌ | ✅ | ✅ | Native PDF parsing |

### Image Analysis Example

```go
req := &ai.Request{
	Messages: []ai.Message{
		ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
			ai.NewTextPart("What's in this image?"),
			ai.NewImagePartFromURL("https://example.com/image.jpg"),
		}),
	},
}
```

### Audio Analysis Example (Gemini only)

```go
req := &ai.Request{
	Messages: []ai.Message{
		ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
			ai.NewTextPart("Transcribe and analyze this audio"),
			ai.NewAudioPartFromURL("https://example.com/audio.mp3"),
		}),
	},
}
```

### Video Analysis Example (Gemini only)

```go
req := &ai.Request{
	Messages: []ai.Message{
		ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
			ai.NewTextPart("Describe what happens in this video"),
			ai.NewVideoPartFromURL("https://example.com/video.mp4", "mp4"),
		}),
	},
}
```

### PDF Document Analysis Example (Gemini & Anthropic)

```go
req := &ai.Request{
	Messages: []ai.Message{
		ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
			ai.NewTextPart("Summarize this research paper"),
			ai.NewPDFPartFromURL("https://arxiv.org/pdf/1706.03762.pdf"),
		}),
	},
}
```

### Using Base64-Encoded Media

All media types also support base64-encoded data for local files:

```go
// Read local file
audioData, _ := os.ReadFile("audio.mp3")
base64Audio := base64.StdEncoding.EncodeToString(audioData)

req := &ai.Request{
	Messages: []ai.Message{
		ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
			ai.NewTextPart("Analyze this audio"),
			ai.NewAudioPartFromBase64(base64Audio, "mp3"),
		}),
	},
}
```

### Automatic Format Handling

- **Gemini**: Automatically downloads media from URLs and converts to base64
- **Anthropic**: Supports both URL and base64 for images and PDFs
- **OpenAI**: Supports URL and base64 for images

### Error Handling

The library provides clear error messages when attempting to use unsupported content types:

```go
// Trying to use audio with OpenAI will return:
// "OpenAI provider does not support audio input (content type: audio).
//  Supported providers: Gemini"
```

### Complete Examples

See the `examples` directory for complete working examples:

- **[examples/simple_chat](examples/simple_chat)** - Basic text generation
- **[examples/vision_chat](examples/vision_chat)** - Image analysis with all providers
- **[examples/audio_chat](examples/audio_chat)** - Audio analysis with Gemini
- **[examples/video_chat](examples/video_chat)** - Video analysis with Gemini
- **[examples/pdf_chat](examples/pdf_chat)** - PDF document Q&A with Gemini/Anthropic
- **[examples/tool_server_interaction](examples/tool_server_interaction)** - MCP tool integration

Each example includes detailed documentation on supported formats, use cases, and limitations.

## AI Gateway

`cmd/ai-gateway` is a production-ready gateway that lets you send OpenAI, Gemini, or Anthropic formatted requests and routes them to the provider you configure per model. It supports streaming responses, Prometheus metrics at `/metrics`, health checks at `/health`, structured JSON logs with request IDs, and YAML-based routing.

### Quick Start

```bash
# Install (or build from source in cmd/ai-gateway)
go install github.com/liuzl/ai/cmd/ai-gateway@latest

# Set credentials (or use an .env file)
export OPENAI_API_KEY=sk-openai
export GEMINI_API_KEY=AIza-gemini
export ANTHROPIC_API_KEY=sk-anthropic

# Configure model routing
cat > config/proxy-config.yaml <<'EOF'
version: "1.0"
models:
  - name: "gpt-4o-mini"
    provider: "openai"
  - name: "claude-3-5-sonnet-20241022"
    provider: "anthropic"
  - name: "gemini-2.0-flash-exp"
    provider: "gemini"
EOF

# Start the gateway (defaults to :8080)
ai-gateway -config config/proxy-config.yaml
```

Send requests in any supported format; the gateway forwards to the configured provider for that model:

```bash
curl -X POST http://localhost:8080/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'
```

See `cmd/ai-gateway/README.md` for full configuration, deployment, and observability details.

## License

MIT License - see LICENSE file for details.
