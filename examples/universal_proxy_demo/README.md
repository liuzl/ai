# Universal API Proxy Demo

This example demonstrates how the universal API proxy can accept different provider API formats and route them to any backend provider.

## What This Demo Shows

The same backend provider (e.g., Gemini) can respond to requests in three different API formats:
1. **OpenAI format** - `/v1/chat/completions`
2. **Gemini format** - `/v1beta/models/{model}:generateContent`
3. **Anthropic format** - `/v1/messages`

This means you can use any SDK or tool designed for one provider with a completely different backend provider!

## Prerequisites

1. Start the universal API proxy server with your chosen backend provider:

```bash
# Example: Use Gemini as the backend for all three formats
export GEMINI_API_KEY="your-gemini-api-key"

# Terminal 1: OpenAI format -> Gemini backend
api-proxy -listen :8081 -format openai -provider gemini -model gemini-2.5-flash

# Terminal 2: Gemini format -> Gemini backend
api-proxy -listen :8082 -format gemini -provider gemini -model gemini-2.5-flash

# Terminal 3: Anthropic format -> Gemini backend
api-proxy -listen :8083 -format anthropic -provider gemini -model gemini-2.5-flash
```

Or run a single instance that accepts one format:

```bash
# Single instance accepting OpenAI format, routing to Gemini
export GEMINI_API_KEY="your-gemini-api-key"
api-proxy -listen :8080 -format openai -provider gemini -model gemini-2.5-flash
```

## Running the Demo

```bash
# Make sure the proxy is running first!
go run main.go
```

## Expected Output

```
================================================================================
Universal API Proxy Demo
================================================================================

Example 1: Using OpenAI Format
--------------------------------------------------------------------------------
Request URL: http://localhost:8080/v1/chat/completions
Request body: {"model":"gpt-4","messages":[{"content":"Say 'Hello from OpenAI format!'","role":"user"}]}

Response:
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello from OpenAI format!"
      },
      "finish_reason": "stop"
    }
  ],
  ...
}

✓ Assistant's message: Hello from OpenAI format!

Example 2: Using Gemini Format
--------------------------------------------------------------------------------
Request URL: http://localhost:8080/v1beta/models/gemini-2.5-flash:generateContent
Request body: {"contents":[{"parts":[{"text":"Say 'Hello from Gemini format!'"}]}]}

Response:
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "Hello from Gemini format!"
          }
        ],
        "role": "model"
      }
    }
  ]
}

✓ Assistant's message: Hello from Gemini format!

Example 3: Using Anthropic Format
--------------------------------------------------------------------------------
Request URL: http://localhost:8080/v1/messages
Request body: {"max_tokens":1024,"messages":[{"content":"Say 'Hello from Anthropic format!'","role":"user"}],"model":"claude-3-5-sonnet-20241022"}

Response:
{
  "id": "msg_...",
  "type": "message",
  "role": "assistant",
  "model": "claude-3-5-sonnet-20241022",
  "content": [
    {
      "type": "text",
      "text": "Hello from Anthropic format!"
    }
  ],
  "stop_reason": "end_turn"
}

✓ Assistant's message: Hello from Anthropic format!

================================================================================
Demo Complete!

Note: All three requests can be routed to the SAME backend provider,
regardless of which API format they use. This is the power of the
universal proxy!
================================================================================
```

## Use Cases Demonstrated

1. **Format Flexibility**: The same backend (Gemini) responds correctly to three different API formats
2. **SDK Compatibility**: Each format can be used with its corresponding SDK (OpenAI SDK, Gemini SDK, Anthropic SDK)
3. **Migration Path**: Easily migrate between providers without changing client code
4. **Cost Optimization**: Use cheaper providers while keeping your existing API format

## Try Different Combinations

Change the proxy configuration to try different combinations:

```bash
# Use OpenAI format to call Anthropic
export ANTHROPIC_API_KEY="your-key"
api-proxy -format openai -provider anthropic -model claude-3-5-haiku-20241022

# Use Anthropic format to call OpenAI
export OPENAI_API_KEY="your-key"
api-proxy -format anthropic -provider openai -model gpt-4o-mini

# Use Gemini format to call OpenAI
export OPENAI_API_KEY="your-key"
api-proxy -format gemini -provider openai -model gpt-4o
```

All 9 combinations work!
