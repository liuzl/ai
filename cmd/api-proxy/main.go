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
	"zliu.org/goutil/rest"
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
	startTime := time.Now()

	rest.Log().Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("request started")

	defer func() {
		rest.Log().Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Dur("duration", time.Since(startTime)).
			Msg("request completed")
	}()

	if r.Method != http.MethodPost {
		rest.Log().Warn().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If verbose logging is enabled, use a TeeReader to capture the body
	var buf bytes.Buffer
	if s.config.VerboseLogging {
		tee := io.TeeReader(r.Body, &buf)
		r.Body = io.NopCloser(tee)
		defer func() {
			rest.Log().Debug().
				Str("body", buf.String()).
				Msg("request body")
		}()
	}
	defer r.Body.Close()

	// Decode request using the converter
	// Pass the full request so headers/URL can be inspected if needed
	providerReq, err := s.formatConverter.DecodeRequest(r)
	if err != nil {
		rest.Log().Warn().
			Err(err).
			Str("path", r.URL.Path).
			Msg("failed to decode request")
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
		rest.Log().Warn().
			Err(err).
			Str("path", r.URL.Path).
			Msg("failed to convert request from format")
		http.Error(w, fmt.Sprintf("Failed to convert request: %v", err), http.StatusBadRequest)
		return
	}

	if s.config.TargetModel != "" {
		universalReq.Model = s.config.TargetModel
	}
	originalModel := universalReq.Model

	if s.config.VerboseLogging {
		reqJSON, _ := json.MarshalIndent(universalReq, "", "  ")
		rest.Log().Debug().
			Str("model", originalModel).
			Str("request", string(reqJSON)).
			Msg("universal request")
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.Timeout)
	defer cancel()

	universalResp, err := s.client.Generate(ctx, universalReq)
	if err != nil {
		rest.Log().Error().
			Err(err).
			Str("model", originalModel).
			Str("provider", string(s.config.TargetProvider)).
			Msg("provider error")
		http.Error(w, fmt.Sprintf("Provider error: %v", err), http.StatusInternalServerError)
		return
	}

	if s.config.VerboseLogging {
		respJSON, _ := json.MarshalIndent(universalResp, "", "  ")
		rest.Log().Debug().
			Str("model", originalModel).
			Str("response", string(respJSON)).
			Msg("universal response")
	}

	providerResp, err := s.formatConverter.ConvertResponseToFormat(universalResp, originalModel)
	if err != nil {
		rest.Log().Error().
			Err(err).
			Str("model", originalModel).
			Msg("failed to convert response to format")
		http.Error(w, fmt.Sprintf("Failed to convert response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(providerResp); err != nil {
		rest.Log().Error().
			Err(err).
			Msg("failed to encode response")
	}
}

// handleStream manages the streaming response lifecycle.
func (s *ProxyServer) handleStream(w http.ResponseWriter, r *http.Request, providerReq any) {
	startTime := time.Now()

	rest.Log().Info().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Msg("streaming request started")

	streamingClient, ok := s.client.(ai.StreamingClient)
	if !ok {
		rest.Log().Warn().
			Str("provider", string(s.config.TargetProvider)).
			Msg("streaming not supported by target provider")
		http.Error(w, "Streaming not supported by target provider", http.StatusNotImplemented)
		return
	}

	universalReq, err := s.formatConverter.ConvertRequestFromFormat(providerReq)
	if err != nil {
		rest.Log().Warn().
			Err(err).
			Str("path", r.URL.Path).
			Msg("failed to convert streaming request")
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
		rest.Log().Error().
			Err(err).
			Str("model", originalModel).
			Str("provider", string(s.config.TargetProvider)).
			Msg("provider stream error")
		http.Error(w, fmt.Sprintf("Provider stream error: %v", err), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		rest.Log().Error().
			Msg("streaming not supported by server")
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

	chunkCount := 0
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			handler.OnEnd(w, flusher)
			rest.Log().Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("chunks", chunkCount).
				Dur("duration", time.Since(startTime)).
				Msg("streaming request completed")
			return
		}
		if err != nil {
			rest.Log().Error().
				Err(err).
				Int("chunks_sent", chunkCount).
				Dur("duration", time.Since(startTime)).
				Msg("streaming error")
			handler.OnError(w, flusher, err)
			return
		}

		if err := handler.OnChunk(w, flusher, chunk); err != nil {
			rest.Log().Error().
				Err(err).
				Int("chunks_sent", chunkCount).
				Msg("error formatting chunk")
			return
		}
		chunkCount++
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

	rest.Log().Info().
		Str("listen_addr", s.config.ListenAddr).
		Str("api_format", string(s.config.APIFormat)).
		Str("endpoint", endpoint).
		Str("target_provider", string(s.config.TargetProvider)).
		Str("target_model", s.config.TargetModel).
		Bool("verbose_logging", s.config.VerboseLogging).
		Msg("starting API proxy server")

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
