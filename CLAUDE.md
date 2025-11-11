# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Codebase Overview

This is a Go library (`github.com/liuzl/ai`) that provides a unified, provider-agnostic interface for interacting with multiple AI models including Google Gemini, OpenAI, and Anthropic. The library features built-in support for the Model-Context Protocol (MCP) for tool integration.

## Architecture

- **Core Interface**: `ai.Client` interface with `Generate()` method in `ai.go:24-26`
- **Provider Adapters**: Separate files for each provider (`openai_adapter.go`, `gemini_adapter.go`, `anthropic_adapter.go`)
- **Format Converters**: Bidirectional converters between provider formats and Universal format (`*_converter.go`)
- **HTTP Client**: Shared HTTP client implementation in `http_client.go`
- **Tool Server Integration**: MCP tool server management in `tool_server.go`
- **Universal Data Structures**: `Request`, `Response`, `Message`, `Tool`, `ToolCall` structs in `ai.go:29-79`
- **Universal API Proxy**: HTTP proxy server (`cmd/api-proxy`) that accepts any provider format and routes to any provider

## Development Commands

**Build:**
```bash
go build
```

**Run Tests:**
```bash
# Run all tests
go test -v

# Run specific test
go test -v -run TestSimpleChat

# Run tests with coverage
go test -cover
```

**Linting/Code Quality:**
```bash
# Go vet (runs automatically with go test)
go vet

# Format code
go fmt ./...
```

**Run Examples:**
```bash
# Simple chat example
go run ./examples/simple_chat

# MCP tool interaction example
go run ./examples/tool_server_interaction/mcp_client.go

# Universal proxy demo (requires proxy server running)
go run ./examples/universal_proxy_demo
```

**Run Universal API Proxy:**
```bash
# Build the proxy
go build ./cmd/api-proxy

# Example: Accept OpenAI format, route to Gemini
export GEMINI_API_KEY="your-key"
./api-proxy -format openai -provider gemini -model gemini-2.5-flash
```

## Key Files

- `ai.go`: Core interface and universal data structures
- `provider_adapter.go`: Base adapter interface
- `*_adapter.go`: Provider-specific implementations (convert Universal → Provider API)
- `format_converter.go`: Format converter interface
- `*_converter.go`: Format converters (convert Provider API ↔ Universal, for proxy server)
- `http_client.go`: Shared HTTP client with retry logic
- `tool_server.go`: MCP tool server integration
- `ai_test.go`: Comprehensive test suite with mock servers
- `cmd/api-proxy/main.go`: Universal API proxy server (any format → any provider)

## Environment Configuration

The library uses environment variables for configuration (see `ai.go:158-199`):
- `AI_PROVIDER`: Provider name (`openai`, `gemini`, `anthropic`)
- Provider-specific API keys and optional model/baseURL settings