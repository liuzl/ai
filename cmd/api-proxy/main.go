package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/liuzl/ai"
)

// Config holds the proxy server configuration.
type Config struct {
	ListenAddr     string
	APIFormat      ai.Provider // The API format to accept (openai, gemini, anthropic)
	TargetProvider ai.Provider // The provider to call (openai, gemini, anthropic)
	TargetAPIKey   string
	TargetBaseURL  string
	TargetModel    string
	Timeout        time.Duration
	VerboseLogging bool
}

// ProxyServer wraps the AI client and provides format-specific HTTP endpoints.
type ProxyServer struct {
	config           *Config
	client           ai.Client
	formatConverter  ai.FormatConverter
	converterFactory *ai.FormatConverterFactory
}

// NewProxyServer creates a new universal API proxy server.
func NewProxyServer(config *Config) (*ProxyServer, error) {
	// Create AI client for the target provider
	opts := []ai.Option{
		ai.WithProvider(config.TargetProvider),
		ai.WithAPIKey(config.TargetAPIKey),
		ai.WithTimeout(config.Timeout),
	}
	if config.TargetBaseURL != "" {
		opts = append(opts, ai.WithBaseURL(config.TargetBaseURL))
	}
	if config.TargetModel != "" {
		opts = append(opts, ai.WithModel(config.TargetModel))
	}

	client, err := ai.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	// Create format converter factory
	factory := ai.NewFormatConverterFactory()

	// Get the converter for the API format
	converter, err := factory.GetConverter(config.APIFormat)
	if err != nil {
		return nil, fmt.Errorf("failed to create format converter: %w", err)
	}

	return &ProxyServer{
		config:           config,
		client:           client,
		formatConverter:  converter,
		converterFactory: factory,
	}, nil
}

// handleRequest is a generic handler that works for any provider format.
func (s *ProxyServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if s.config.VerboseLogging {
		log.Printf("Received request: %s", string(body))
	}

	// Parse request based on API format
	var providerReq any
	switch s.config.APIFormat {
	case ai.ProviderOpenAI:
		var req ai.OpenAIChatCompletionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse OpenAI request: %v", err), http.StatusBadRequest)
			return
		}
		// If stream requested, handle SSE streaming directly.
		if req.Stream {
			s.handleOpenAIStream(w, r, &req)
			return
		}
		providerReq = &req
	case ai.ProviderGemini:
		var req ai.GeminiGenerateContentRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse Gemini request: %v", err), http.StatusBadRequest)
			return
		}
		providerReq = &req
	case ai.ProviderAnthropic:
		var req ai.AnthropicMessagesRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse Anthropic request: %v", err), http.StatusBadRequest)
			return
		}
		providerReq = &req
	default:
		http.Error(w, "Unsupported API format", http.StatusInternalServerError)
		return
	}

	// Convert to universal request
	universalReq, err := s.formatConverter.ConvertRequestFromFormat(providerReq)
	if err != nil {
		log.Printf("Error converting request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to convert request: %v", err), http.StatusBadRequest)
		return
	}

	// Override model if configured
	if s.config.TargetModel != "" && universalReq.Model == "" {
		universalReq.Model = s.config.TargetModel
	}

	// Preserve original model for response
	originalModel := universalReq.Model

	if s.config.VerboseLogging {
		reqJSON, _ := json.MarshalIndent(universalReq, "", "  ")
		log.Printf("Universal request: %s", string(reqJSON))
	}

	// Call the target provider
	ctx, cancel := context.WithTimeout(r.Context(), s.config.Timeout)
	defer cancel()

	universalResp, err := s.client.Generate(ctx, universalReq)
	if err != nil {
		log.Printf("Error calling provider: %v", err)
		http.Error(w, fmt.Sprintf("Provider error: %v", err), http.StatusInternalServerError)
		return
	}

	if s.config.VerboseLogging {
		respJSON, _ := json.MarshalIndent(universalResp, "", "  ")
		log.Printf("Universal response: %s", string(respJSON))
	}

	// Convert response back to the API format
	providerResp, err := s.formatConverter.ConvertResponseToFormat(universalResp, originalModel)
	if err != nil {
		log.Printf("Error converting response: %v", err)
		http.Error(w, fmt.Sprintf("Failed to convert response: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(providerResp); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleOpenAIStream streams responses in OpenAI SSE format.
func (s *ProxyServer) handleOpenAIStream(w http.ResponseWriter, r *http.Request, openaiReq *ai.OpenAIChatCompletionRequest) {
	streamingClient, ok := s.client.(ai.StreamingClient)
	if !ok {
		http.Error(w, "Streaming not supported by target provider", http.StatusNotImplemented)
		return
	}

	// Convert to universal request
	universalReq, err := s.formatConverter.ConvertRequestFromFormat(openaiReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to convert request: %v", err), http.StatusBadRequest)
		return
	}

	// Override model if configured
	if s.config.TargetModel != "" && universalReq.Model == "" {
		universalReq.Model = s.config.TargetModel
	}
	originalModel := universalReq.Model

	ctx, cancel := context.WithTimeout(r.Context(), s.config.Timeout)
	defer cancel()

	reader, err := streamingClient.Stream(ctx, universalReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Provider stream error: %v", err), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported by server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	streamID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())

	for {
		chunk, err := reader.Recv()
		if err == io.EOF {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		if err != nil {
			errPayload := map[string]string{"error": err.Error()}
			if b, marshalErr := json.Marshal(errPayload); marshalErr == nil {
				fmt.Fprintf(w, "data: %s\n\n", b)
			}
			flusher.Flush()
			return
		}

		payload := buildOpenAIStreamChunk(streamID, originalModel, chunk)
		data, err := json.Marshal(payload)
		if err != nil {
			errPayload := map[string]string{"error": err.Error()}
			if b, marshalErr := json.Marshal(errPayload); marshalErr == nil {
				fmt.Fprintf(w, "data: %s\n\n", b)
			}
			flusher.Flush()
			return
		}

		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		if chunk.Done {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
	}
}

// buildOpenAIStreamChunk converts a universal StreamChunk to OpenAI SSE chunk.
func buildOpenAIStreamChunk(id, model string, chunk *ai.StreamChunk) *openAIStreamChunk {
	choice := openAIStreamChoice{
		Index: 0,
		Delta: openAIStreamDelta{},
	}
	if chunk.TextDelta != "" {
		choice.Delta.Content = append(choice.Delta.Content, openAIContentPart{
			Type: "text",
			Text: chunk.TextDelta,
		})
	}
	for _, tc := range chunk.ToolCallDeltas {
		choice.Delta.ToolCalls = append(choice.Delta.ToolCalls, openAIToolCallDelta{
			ID:   tc.ID,
			Type: tc.Type,
			Function: openAIFunctionCallDelta{
				Name:      tc.Function,
				Arguments: tc.ArgumentsDelta,
			},
		})
	}

	if chunk.Done {
		if len(choice.Delta.ToolCalls) > 0 {
			choice.FinishReason = "tool_calls"
		} else {
			choice.FinishReason = "stop"
		}
	}

	return &openAIStreamChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []openAIStreamChoice{choice},
	}
}

// Minimal OpenAI streaming chunk structs for SSE responses.
type openAIStreamChunk struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIStreamChoice `json:"choices"`
}

type openAIStreamChoice struct {
	Index        int               `json:"index"`
	Delta        openAIStreamDelta `json:"delta"`
	FinishReason string            `json:"finish_reason,omitempty"`
}

type openAIStreamDelta struct {
	Role      string                `json:"role,omitempty"`
	Content   []openAIContentPart   `json:"content,omitempty"`
	ToolCalls []openAIToolCallDelta `json:"tool_calls,omitempty"`
}

type openAIContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type openAIToolCallDelta struct {
	ID       string                  `json:"id,omitempty"`
	Type     string                  `json:"type,omitempty"`
	Function openAIFunctionCallDelta `json:"function"`
}

type openAIFunctionCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// Start starts the HTTP server.
func (s *ProxyServer) Start() error {
	mux := http.NewServeMux()

	// Register the endpoint based on API format
	endpoint := s.formatConverter.GetEndpoint()

	// For Gemini, we need to handle the dynamic model path
	if s.config.APIFormat == ai.ProviderGemini {
		// Match any model path
		mux.HandleFunc("/v1beta/models/", s.handleRequest)
		mux.HandleFunc("/v1/models/", s.handleRequest)
	} else {
		mux.HandleFunc(endpoint, s.handleRequest)
	}

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting API proxy server on %s", s.config.ListenAddr)
	log.Printf("API Format: %s (endpoint: %s)", s.config.APIFormat, endpoint)
	log.Printf("Target Provider: %s", s.config.TargetProvider)
	if s.config.VerboseLogging {
		log.Printf("Verbose logging enabled")
	}

	return http.ListenAndServe(s.config.ListenAddr, mux)
}

func main() {
	// Parse command-line flags
	listenAddr := flag.String("listen", ":8080", "Server listen address")
	apiFormat := flag.String("format", "", "API format to accept (openai, gemini, anthropic)")
	targetProvider := flag.String("provider", "", "Target AI provider to call (openai, gemini, anthropic)")
	apiKey := flag.String("api-key", "", "Target provider API key")
	baseURL := flag.String("base-url", "", "Target provider base URL (optional)")
	model := flag.String("model", "", "Target provider model (optional)")
	timeout := flag.Duration("timeout", 5*time.Minute, "Request timeout")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Validate and default API format
	if *apiFormat == "" {
		*apiFormat = strings.ToLower(os.Getenv("API_FORMAT"))
		if *apiFormat == "" {
			*apiFormat = "openai" // Default to OpenAI format
		}
	}
	*apiFormat = strings.ToLower(*apiFormat)

	// Validate and default target provider
	if *targetProvider == "" {
		*targetProvider = strings.ToLower(os.Getenv("AI_PROVIDER"))
		if *targetProvider == "" {
			log.Fatal("Error: -provider flag or AI_PROVIDER env var is required")
		}
	}
	*targetProvider = strings.ToLower(*targetProvider)

	// Get API key from environment if not provided
	if *apiKey == "" {
		switch ai.Provider(*targetProvider) {
		case ai.ProviderOpenAI:
			*apiKey = os.Getenv("OPENAI_API_KEY")
		case ai.ProviderGemini:
			*apiKey = os.Getenv("GEMINI_API_KEY")
		case ai.ProviderAnthropic:
			*apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if *apiKey == "" {
			log.Fatal("Error: -api-key flag or provider-specific API key env var is required")
		}
	}

	// Create configuration
	config := &Config{
		ListenAddr:     *listenAddr,
		APIFormat:      ai.Provider(*apiFormat),
		TargetProvider: ai.Provider(*targetProvider),
		TargetAPIKey:   *apiKey,
		TargetBaseURL:  *baseURL,
		TargetModel:    *model,
		Timeout:        *timeout,
		VerboseLogging: *verbose,
	}

	// Create and start proxy server
	server, err := NewProxyServer(config)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
