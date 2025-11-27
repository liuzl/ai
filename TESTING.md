# Streaming & Format Matrix Test Report (Sanitized)

This document summarizes end-to-end tests covering all request-format and backend combinations (streaming and non-streaming). All API keys and secrets are intentionally omitted.

## Environments
- **OpenAI-compatible gateway**
  - Base URL: (redacted; set via `OPENAI_BASE_URL`)
  - Model: `google/gemini-2.5-flash-lite`
  - API key: set via `OPENAI_API_KEY` env
- **Anthropic-compatible endpoint**
  - Base URL: (redacted; set via `ANTHROPIC_BASE_URL`)
  - Model: `claude-haiku-4-5`
  - API key: set via `ANTHROPIC_API_KEY` env
- **Gemini native**
  - Model: `gemini-2.5-flash`

## Coverage Matrix (✔ = tested)
| Request Format | Target Backend            | Non-Stream | Stream |
|----------------|---------------------------|------------|--------|
| OpenAI         | OpenAI-compatible gateway | ✔          | ✔      |
| OpenAI         | Gemini                    | ✔          | ✔      |
| OpenAI         | Anthropic-compatible      | ✔          | ✔      |
| Anthropic      | OpenAI-compatible gateway | ✔          | ✔      |
| Anthropic      | Gemini                    | ✔          | ✔      |
| Anthropic      | Anthropic-compatible      | ✔          | ✔      |
| Gemini         | OpenAI-compatible gateway | ✔          | ✔      |
| Gemini         | Gemini                    | ✔          | ✔      |
| Gemini         | Anthropic-compatible      | ✔          | ✔      |

## Notes & Observations
- Streaming responses return SSE; non-streaming returns JSON.
- OpenAI streaming parser accepts `delta.content` as string or array.
- Anthropic/Gemini request structs support `stream` flag; proxy also detects streaming via path (e.g., `:streamGenerateContent`).
- If a port is in use, switch to a free port when running the proxy examples.
- Ensure all API keys/base URLs/models are provided via environment variables before running commands.

## Example Commands
Replace `<PORT>`, `<target-model>`, `<base-url>`, `<backend>`, and ensure corresponding env vars (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.) are set.

### OpenAI Format → Any Backend (Streaming)
```bash
body='{"model":"<target-model>","stream":true,"messages":[{"role":"user","content":"Stream a greeting"}]}'
go run ./cmd/api-proxy -format openai -provider <backend> -model <target-model> -base-url <base-url> -listen :<PORT> &
pid=$!; sleep 3
curl -N -s -X POST http://localhost:<PORT>/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d "$body"
kill $pid || true
```

### Anthropic Format → Any Backend (Streaming)
```bash
body='{"model":"<target-model>","stream":true,"max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"Stream a greeting"}]}]}'
go run ./cmd/api-proxy -format anthropic -provider <backend> -model <target-model> -base-url <base-url> -listen :<PORT> &
pid=$!; sleep 3
curl -N -s -X POST http://localhost:<PORT>/v1/messages \
  -H "Content-Type: application/json" \
  -d "$body"
kill $pid || true
```

### Gemini Format → Any Backend (Streaming)
```bash
body='{"stream":true,"contents":[{"role":"user","parts":[{"text":"Stream a greeting"}]}]}'
go run ./cmd/api-proxy -format gemini -provider <backend> -model <target-model> -base-url <base-url> -listen :<PORT> &
pid=$!; sleep 3
curl -N -s -X POST http://localhost:<PORT>/v1beta/models/<target-model>:streamGenerateContent \
  -H "Content-Type: application/json" \
  -d "$body"
kill $pid || true
```

### Non-Streaming Examples
- OpenAI format: remove `"stream":true` and POST to `/v1/chat/completions`.
- Anthropic format: remove `"stream":true` and POST to `/v1/messages`.
- Gemini format: remove `"stream":true` and POST to `/v1beta/models/<model>:generateContent`.
