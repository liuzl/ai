# AI Gateway

A production-ready AI gateway that accepts multiple AI provider formats (OpenAI, Gemini, Anthropic) and routes requests to backend providers based on YAML-configured model mappings.

## Features

- **Multiple Format Support**: Accept requests in OpenAI, Gemini, or Anthropic format
- **Flexible Routing**: Route requests to any backend provider based on model configuration
- **YAML Configuration**: Simple model-to-provider mapping
- **Environment Variable Support**: Load credentials from `.env` files
- **Production-Ready Observability**:
  - Prometheus metrics at `/metrics`
  - Structured JSON logging
  - Request ID tracing
  - Health check endpoint at `/health`
- **Streaming Support**: Full support for streaming responses
- **Thread-Safe**: Efficient client pooling with concurrent access
- **Graceful Shutdown**: 30-second timeout for in-flight requests

## Quick Start

### 1. Install via go install

```bash
go install github.com/liuzl/ai/cmd/ai-gateway@latest
```

Or build from source:

```bash
cd cmd/ai-gateway
go build -o ai-gateway
```

### 2. Configure Environment Variables

Copy the example environment file and fill in your credentials:

```bash
cp .env.example .env
```

Edit `.env`:

```bash
OPENAI_API_KEY=sk-your-openai-key
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key
GEMINI_API_KEY=AIza-your-gemini-key
```

### 3. Configure Model Routing

The gateway uses `config/proxy-config.yaml` to map models to providers. Example:

```yaml
version: "1.0"

models:
  - name: "gpt-4"
    provider: "openai"

  - name: "claude-3-5-sonnet-20241022"
    provider: "anthropic"

  - name: "gemini-2.0-flash-exp"
    provider: "gemini"
```

### 4. Start the Gateway

```bash
./ai-gateway
```

The server will start on `:8080` by default.

## Usage Examples

### OpenAI Format → Gemini Backend

```bash
curl -X POST http://localhost:8080/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Anthropic Format → Anthropic Backend

```bash
curl -X POST http://localhost:8080/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "max_tokens": 1024
  }'
```

### Gemini Format → Gemini Backend

```bash
curl -X POST http://localhost:8080/gemini/v1/models/gemini-1.5-pro:generateContent \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [{"text": "Hello!"}]
      }
    ]
  }'
```

### Streaming Requests

Add `"stream": true` to any request:

```bash
curl -X POST http://localhost:8080/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Tell me a story"}],
    "stream": true
  }'
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/openai/v1/chat/completions` | OpenAI format requests |
| `/anthropic/v1/messages` | Anthropic format requests |
| `/gemini/v1/models/{model}:generateContent` | Gemini format requests |
| `/gemini/v1beta/models/{model}:generateContent` | Gemini beta format requests |
| `/health` | Health check endpoint |
| `/metrics` | Prometheus metrics |

## Configuration

### Command-Line Flags

```bash
./ai-gateway [options]

Options:
  -listen string
        Server listen address (default ":8080")
  -config string
        Path to YAML configuration file (default "config/proxy-config.yaml")
  -env-file string
        Path to .env file (default ".env")
  -verbose
        Enable verbose logging
```

### YAML Configuration

```yaml
version: "1.0"

models:
  - name: "model-name"
    provider: "openai|gemini|anthropic"
    description: "Optional description"

# Optional: fallback provider for unknown models
default_provider: "openai"

# Optional: custom timeout
timeout: "5m"
```

### Environment Variables

**Provider Credentials:**
- `OPENAI_API_KEY` - OpenAI API key (required if using OpenAI)
- `GEMINI_API_KEY` - Gemini API key (required if using Gemini)
- `ANTHROPIC_API_KEY` - Anthropic API key (required if using Anthropic)

**Optional Base URLs:**
- `OPENAI_BASE_URL` - Custom OpenAI endpoint
- `GEMINI_BASE_URL` - Custom Gemini endpoint
- `ANTHROPIC_BASE_URL` - Custom Anthropic endpoint

## Observability

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "models": 9,
  "timestamp": "2025-11-27T10:30:45Z"
}
```

### Prometheus Metrics

Available at `/metrics`:

- `proxy_requests_total{format, model, provider, status}` - Total requests
- `proxy_request_duration_seconds{format, model, provider}` - Request duration histogram
- `proxy_errors_total{format, model, provider, error_type}` - Total errors
- `proxy_active_requests{format, provider}` - Active requests gauge

Example queries:

```promql
# Request rate by provider
sum(rate(proxy_requests_total[5m])) by (provider)

# P95 latency by model
histogram_quantile(0.95, proxy_request_duration_seconds_bucket) by (model)

# Error rate
sum(rate(proxy_errors_total[5m])) / sum(rate(proxy_requests_total[5m]))
```

### Structured Logging

The gateway outputs JSON-formatted logs to stdout:

```json
{
  "timestamp": "2025-11-27T10:30:45.123Z",
  "level": "info",
  "request_id": "req-1732704645-a3f2e1",
  "message": "request completed successfully",
  "duration_ms": 245.67,
  "status_code": 200,
  "format": "openai",
  "model": "gpt-4",
  "provider": "openai",
  "streaming": false
}
```

### Request Tracing

All requests are assigned a unique request ID. You can:
- Provide your own: `X-Request-ID: my-custom-id`
- Let the gateway generate one automatically
- The request ID is returned in response headers and logs

## Error Handling

The gateway returns errors in the following format:

```json
{
  "error": {
    "message": "Error description",
    "type": "error_type",
    "request_id": "req-1732704645-a3f2e1"
  }
}
```

**Error Types:**
- `unknown_model` - Model not found in configuration
- `auth` - Authentication failed
- `rate_limit` - Rate limit exceeded
- `timeout` - Request timeout
- `invalid_request` - Malformed request
- `server_error` - Backend provider error
- `network` - Network connectivity issue

## Deployment

### Docker

```dockerfile
FROM golang:1.24 AS builder
WORKDIR /app
COPY . .
RUN cd cmd/ai-gateway && go build -o ai-gateway

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/cmd/ai-gateway/ai-gateway .
COPY --from=builder /app/cmd/ai-gateway/config/ config/
EXPOSE 8080
CMD ["./ai-gateway"]
```

Build and run:

```bash
docker build -t ai-gateway -f cmd/ai-gateway/Dockerfile .
docker run -p 8080:8080 --env-file cmd/ai-gateway/.env ai-gateway
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-gateway
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: gateway
        image: ai-gateway:latest
        ports:
        - containerPort: 8080
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: ai-credentials
              key: openai-key
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
```

## Development

### Project Structure

```
cmd/ai-gateway/
├── main.go              # Entry point
├── config.go            # YAML configuration
├── server.go            # HTTP server
├── handlers.go          # Request handlers
├── client_pool.go       # Client caching
├── middleware.go        # HTTP middleware
├── logger.go            # Structured logging
├── metrics.go           # Prometheus metrics
├── request_context.go   # Context utilities
├── config/
│   └── proxy-config.yaml
├── .env.example
└── README.md
```

### Running Tests

```bash
go test -v ./...
go test -cover ./...
```

### Adding a New Model

1. Edit `config/proxy-config.yaml`:
   ```yaml
   models:
     - name: "new-model-name"
       provider: "openai"  # or gemini, anthropic
   ```

2. Restart the gateway

3. The new model is immediately available

## Architecture

```
Client Request (Format A/B/C)
    ↓
[Path Router] → Format-specific endpoints
    ↓
[Middleware] → Request ID, Logging, Metrics, Recovery
    ↓
[Format Decoder] → Use FormatConverter.DecodeRequest()
    ↓
[Model Extractor] → Parse model from request
    ↓
[Config Lookup] → YAML: model → provider mapping
    ↓
[Client Pool] → Cached ai.Client per provider
    ↓
[Backend API] → client.Generate() or Stream()
    ↓
[Response Convert] → FormatConverter.ConvertResponseToFormat()
    ↓
[Response] → Original format + headers + metrics
```

## License

See the main repository LICENSE file.

## Support

For issues and questions, please open an issue on the GitHub repository: https://github.com/liuzl/ai
