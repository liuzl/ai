package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/liuzl/ai"
)

var streamCounter uint64

// Config holds the proxy server configuration.
type Config struct {
	ListenAddr     string
	APIFormat      ai.Provider
	TargetProvider ai.Provider
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

	factory := ai.NewFormatConverterFactory()
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

// handleRequest handles incoming requests.
func (s *ProxyServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If verbose logging is enabled, use a TeeReader to capture the body
	// Note: We need to wrap r.Body, and since DecodeRequest takes *http.Request,
	// we need to temporarily replace r.Body if we want to tee it.
	if s.config.VerboseLogging {
		var buf bytes.Buffer
		tee := io.TeeReader(r.Body, &buf)
		// We can't easily replace r.Body with a TeeReader because we need it to be a ReadCloser
		// So we use NopCloser.
		r.Body = io.NopCloser(tee)
		defer func() {
			log.Printf("Received request: %s", buf.String())
		}()
	}
	defer r.Body.Close()

	// Decode request using the converter
	// Pass the full request so headers/URL can be inspected if needed
	providerReq, err := s.formatConverter.DecodeRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode request: %v", err), http.StatusBadRequest)
		return
	}

	// Check for streaming
	if s.formatConverter.IsStreaming(providerReq) {
		s.handleStream(w, r, providerReq)
		return
	}

	// Normal request handling
	universalReq, err := s.formatConverter.ConvertRequestFromFormat(providerReq)
	if err != nil {
		log.Printf("Error converting request: %v", err)
		http.Error(w, fmt.Sprintf("Failed to convert request: %v", err), http.StatusBadRequest)
		return
	}

	if s.config.TargetModel != "" {
		universalReq.Model = s.config.TargetModel
	}
	originalModel := universalReq.Model

	if s.config.VerboseLogging {
		reqJSON, _ := json.MarshalIndent(universalReq, "", "  ")
		log.Printf("Universal request: %s", string(reqJSON))
	}

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

	providerResp, err := s.formatConverter.ConvertResponseToFormat(universalResp, originalModel)
	if err != nil {
		log.Printf("Error converting response: %v", err)
		http.Error(w, fmt.Sprintf("Failed to convert response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(providerResp); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// handleStream manages the streaming response lifecycle.
func (s *ProxyServer) handleStream(w http.ResponseWriter, r *http.Request, providerReq any) {
	streamingClient, ok := s.client.(ai.StreamingClient)
	if !ok {
		http.Error(w, "Streaming not supported by target provider", http.StatusNotImplemented)
		return
	}

	universalReq, err := s.formatConverter.ConvertRequestFromFormat(providerReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to convert request: %v", err), http.StatusBadRequest)
		return
	}

	if s.config.TargetModel != "" {
		universalReq.Model = s.config.TargetModel
	}
	originalModel := universalReq.Model

	ctx, cancel := context.WithTimeout(r.Context(), s.config.Timeout)
	defer cancel()

	stream, err := streamingClient.Stream(ctx, universalReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Provider stream error: %v", err), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported by server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	streamID := fmt.Sprintf("chatcmpl-%d-%d", time.Now().Unix(), atomic.AddUint64(&streamCounter, 1))

	// Get specific handler from converter
	handler := s.formatConverter.NewStreamHandler(streamID, originalModel)

	handler.OnStart(w, flusher)

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			handler.OnEnd(w, flusher)
			return
		}
		if err != nil {
			handler.OnError(w, flusher, err)
			return
		}

		if err := handler.OnChunk(w, flusher, chunk); err != nil {
			log.Printf("Error formatting chunk: %v", err)
			return
		}
	}
}

// Start starts the HTTP server.
func (s *ProxyServer) Start() error {
	mux := http.NewServeMux()
	endpoint := s.formatConverter.GetEndpoint()

	if s.config.APIFormat == ai.ProviderGemini {
		mux.HandleFunc("/v1beta/models/", s.handleRequest)
		mux.HandleFunc("/v1/models/", s.handleRequest)
	} else {
		mux.HandleFunc(endpoint, s.handleRequest)
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting API proxy server on %s", s.config.ListenAddr)
	log.Printf("API Format: %s (endpoint: %s)", s.config.APIFormat, endpoint)
	log.Printf("Target Provider: %s", s.config.TargetProvider)

	return http.ListenAndServe(s.config.ListenAddr, mux)
}

func main() {
	config := loadConfig()
	server, err := NewProxyServer(config)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func loadConfig() *Config {
	listenAddr := flag.String("listen", ":8080", "Server listen address")
	apiFormat := flag.String("format", "", "API format to accept (openai, gemini, anthropic)")
	targetProvider := flag.String("provider", "", "Target AI provider to call (openai, gemini, anthropic)")
	apiKey := flag.String("api-key", "", "Target provider API key")
	baseURL := flag.String("base-url", "", "Target provider base URL (optional)")
	model := flag.String("model", "", "Target provider model (optional)")
	timeout := flag.Duration("timeout", 5*time.Minute, "Request timeout")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	formatStr := strings.ToLower(*apiFormat)
	if formatStr == "" {
		formatStr = strings.ToLower(os.Getenv("API_FORMAT"))
		if formatStr == "" {
			formatStr = "openai"
		}
	}

	providerStr := strings.ToLower(*targetProvider)
	if providerStr == "" {
		providerStr = strings.ToLower(os.Getenv("AI_PROVIDER"))
		if providerStr == "" {
			log.Fatal("Error: -provider flag or AI_PROVIDER env var is required")
		}
	}

	key := *apiKey
	if key == "" {
		switch ai.Provider(providerStr) {
		case ai.ProviderOpenAI:
			key = os.Getenv("OPENAI_API_KEY")
		case ai.ProviderGemini:
			key = os.Getenv("GEMINI_API_KEY")
		case ai.ProviderAnthropic:
			key = os.Getenv("ANTHROPIC_API_KEY")
		}
		if key == "" {
			log.Fatal("Error: -api-key flag or provider-specific API key env var is required")
		}
	}

	return &Config{
		ListenAddr:     *listenAddr,
		APIFormat:      ai.Provider(formatStr),
		TargetProvider: ai.Provider(providerStr),
		TargetAPIKey:   key,
		TargetBaseURL:  *baseURL,
		TargetModel:    *model,
		Timeout:        *timeout,
		VerboseLogging: *verbose,
	}
}
