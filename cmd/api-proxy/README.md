# Universal AI API Proxy Server

This proxy server provides a universal format conversion layer that allows you to **use any provider's API format to call any provider**. For example:

- Use OpenAI format to call Gemini or Anthropic
- Use Gemini format to call OpenAI or Anthropic
- Use Anthropic format to call OpenAI or Gemini

## Why Use This?

- **Format Flexibility**: Use the API format you're familiar with, regardless of the backend provider
- **Easy Provider Switching**: Change providers without rewriting client code
- **Tool Compatibility**: Use existing tools/SDKs designed for one provider with another
- **Cost Optimization**: Route expensive API calls to cheaper providers using the same format
- **Testing**: Test different providers using the same test suite

## Quick Start

### Installation

```bash
go install github.com/liuzl/ai/cmd/api-proxy@latest
```

Or build from source:

```bash
cd cmd/api-proxy
go build
```

### Example: Use OpenAI Format to Call Gemini

```bash
export GEMINI_API_KEY="your-gemini-api-key"

api-proxy \
  -format openai \
  -provider gemini \
  -model gemini-2.5-flash
```

Now you can use the OpenAI SDK/API to call Gemini:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Example: Use Anthropic Format to Call OpenAI

```bash
export OPENAI_API_KEY="your-openai-api-key"

api-proxy \
  -format anthropic \
  -provider openai \
  -model gpt-4o-mini
```

Now you can use the Anthropic SDK/API to call OpenAI:

```bash
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: dummy" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Example: Use Gemini Format to Call Anthropic

```bash
export ANTHROPIC_API_KEY="your-anthropic-api-key"

api-proxy \
  -format gemini \
  -provider anthropic \
  -model claude-3-5-haiku-20241022
```

## Configuration Options

```
-listen string
    Server listen address (default ":8080")

-format string
    API format to accept: openai, gemini, or anthropic
    Can also be set via API_FORMAT environment variable
    Default: openai

-provider string
    Target AI provider to call: openai, gemini, or anthropic
    Can also be set via AI_PROVIDER environment variable
    Required

-api-key string
    Target provider API key
    Can also be set via provider-specific env vars:
    - OPENAI_API_KEY
    - GEMINI_API_KEY
    - ANTHROPIC_API_KEY

-model string
    Target provider model (optional)
    If not specified, uses the model from the request

-base-url string
    Target provider base URL (optional)
    Useful for custom endpoints or proxies

-timeout duration
    Request timeout (default 5m0s)

-verbose
    Enable verbose logging for debugging
```

## Supported Format Combinations

All 9 combinations are supported:

| API Format ↓ / Provider → | OpenAI | Gemini | Anthropic |
|---------------------------|--------|--------|-----------|
| **OpenAI**                | ✅     | ✅     | ✅        |
| **Gemini**                | ✅     | ✅     | ✅        |
| **Anthropic**             | ✅     | ✅     | ✅        |

## API Endpoints by Format

- **OpenAI Format**: `POST /v1/chat/completions`
- **Gemini Format**: `POST /v1beta/models/{model}:generateContent` or `POST /v1/models/{model}:generateContent`
- **Anthropic Format**: `POST /v1/messages`

## Usage Examples

### Using OpenAI Python SDK with Gemini Backend

```python
from openai import OpenAI

# Start proxy: api-proxy -format openai -provider gemini

client = OpenAI(
    api_key="dummy",
    base_url="http://localhost:8080/v1"
)

response = client.chat.completions.create(
    model="gpt-4",  # Ignored, uses Gemini model configured in proxy
    messages=[{"role": "user", "content": "Explain quantum computing"}]
)

print(response.choices[0].message.content)
```

### Using Anthropic SDK with OpenAI Backend

```python
import anthropic

# Start proxy: api-proxy -format anthropic -provider openai -model gpt-4o

client = anthropic.Anthropic(
    api_key="dummy",
    base_url="http://localhost:8080"
)

message = client.messages.create(
    model="claude-3-5-sonnet-20241022",  # Ignored, uses OpenAI model
    max_tokens=1024,
    messages=[{"role": "user", "content": "Write a haiku about coding"}]
)

print(message.content[0].text)
```

### Using Google Generative AI SDK with Anthropic Backend

```python
import google.generativeai as genai

# Start proxy: api-proxy -format gemini -provider anthropic -model claude-3-5-haiku-20241022

# Configure to use proxy (Note: The official SDK may not support custom base URLs easily)
# This is a conceptual example - you may need to use HTTP requests directly

# Using requests directly:
import requests

url = "http://localhost:8080/v1beta/models/gemini-2.5-flash:generateContent"
data = {
    "contents": [{
        "parts": [{"text": "Explain machine learning"}]
    }]
}

response = requests.post(url, json=data)
print(response.json())
```

## Tool/Function Calling

The proxy supports tool calling across all formats and providers:

```bash
# Start proxy
api-proxy -format openai -provider anthropic -model claude-3-5-sonnet-20241022

# Call with tools (OpenAI format)
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "What is the weather in Tokyo?"}
    ],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather for a location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {"type": "string"}
          },
          "required": ["location"]
        }
      }
    }]
  }'
```

## Multimodal/Vision Support

The proxy supports image inputs across all formats:

```bash
# OpenAI format with images -> Gemini backend
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "What is in this image?"},
        {
          "type": "image_url",
          "image_url": {"url": "https://example.com/image.jpg"}
        }
      ]
    }]
  }'
```

## Health Check

```bash
curl http://localhost:8080/health
# Returns: OK
```

## Environment Variables

Instead of command-line flags, you can use environment variables:

```bash
# Set configuration via environment variables
export API_FORMAT=openai
export AI_PROVIDER=gemini
export GEMINI_API_KEY=your-key

# Start proxy
api-proxy
```

## Docker Deployment

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o api-proxy ./cmd/api-proxy

FROM alpine:latest
COPY --from=builder /app/api-proxy /usr/local/bin/
ENTRYPOINT ["api-proxy"]
```

```bash
# Build
docker build -t ai-api-proxy .

# Run with OpenAI format -> Gemini backend
docker run -p 8080:8080 \
  -e GEMINI_API_KEY=your-key \
  ai-api-proxy -format openai -provider gemini
```

## Use Cases

### 1. Cost Optimization
Route OpenAI-formatted requests to cheaper providers:
```bash
api-proxy -format openai -provider gemini -model gemini-2.5-flash
# Your OpenAI apps now use cheaper Gemini pricing
```

### 2. Provider Comparison
Test the same prompts across different providers:
```bash
# Terminal 1: OpenAI -> Gemini
api-proxy -listen :8081 -format openai -provider gemini

# Terminal 2: OpenAI -> Anthropic
api-proxy -listen :8082 -format openai -provider anthropic

# Terminal 3: OpenAI -> OpenAI (baseline)
api-proxy -listen :8083 -format openai -provider openai
```

### 3. Vendor Lock-in Mitigation
Build apps using one API format, easily switch providers:
```bash
# Development: Use cheaper provider
api-proxy -format openai -provider gemini

# Production: Switch to different provider
api-proxy -format openai -provider anthropic
```

### 4. Legacy Tool Support
Use old tools designed for one provider with new providers:
```bash
# Run old OpenAI-only tool with Gemini
api-proxy -format openai -provider gemini
./legacy-openai-tool --base-url http://localhost:8080
```

## Verbose Logging

Enable verbose logging for debugging:

```bash
api-proxy -format openai -provider gemini -verbose
```

This will log all requests and responses in Universal format.

## Limitations

- Token counts in responses may not be accurate (different providers report differently)
- Some provider-specific features may not translate perfectly
- Streaming responses are not yet supported

## Architecture

```
┌─────────────────────┐
│  Client (Any SDK)   │
│  Uses Format A      │
└──────────┬──────────┘
           │ HTTP Request (Format A)
           ▼
┌─────────────────────────────┐
│    API Proxy Server         │
│                             │
│  Format A → Universal       │
│         ↓                   │
│  Universal Client           │
│         ↓                   │
│  Universal → Provider B API │
│         ↓                   │
│  Provider B Response        │
│         ↓                   │
│  Universal → Format A       │
└──────────┬──────────────────┘
           │ HTTP Response (Format A)
           ▼
┌─────────────────────┐
│  Client receives    │
│  response in        │
│  expected format    │
└─────────────────────┘
```

## Troubleshooting

**Issue**: "unsupported provider format"
- Check that `-format` is one of: openai, gemini, anthropic

**Issue**: "Provider error: unauthorized"
- Verify API key is correct for the **target provider** (not the format)
- Check environment variable matches the provider

**Issue**: Different behavior than direct API calls
- Some provider features may not translate perfectly
- Use `-verbose` to see the Universal format conversion
- Check that your request follows the format's specification

## License

Same license as the parent project.
