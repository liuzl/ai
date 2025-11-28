package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/liuzl/ai"
	"zliu.org/goutil/rest"
)

//go:embed static
var staticFiles embed.FS

// ServerConfig holds server configuration
type ServerConfig struct {
	ListenAddr string
	Verbose    bool
}

// ProxyServer is the main proxy server
type ProxyServer struct {
	config           *ProxyConfig
	serverCfg        *ServerConfig
	clientPool       *ClientPool
	converterFactory *ai.FormatConverterFactory
	metrics          *MetricsCollector
	httpServer       *http.Server
}

// NewProxyServer creates a new ProxyServer
func NewProxyServer(cfg *ProxyConfig, serverCfg *ServerConfig) (*ProxyServer, error) {
	s := &ProxyServer{
		config:           cfg,
		serverCfg:        serverCfg,
		clientPool:       NewClientPool(),
		converterFactory: &ai.FormatConverterFactory{},
		metrics:          NewMetricsCollector(),
	}

	// Validate that all configured providers have credentials
	if err := s.validateProviders(); err != nil {
		return nil, fmt.Errorf("provider validation failed: %w", err)
	}

	return s, nil
}

// Start starts the HTTP server
func (s *ProxyServer) Start() error {
	// Setup routes
	mux := s.setupRoutes()

	// Apply middleware
	handler := s.applyMiddleware(mux)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    s.serverCfg.ListenAddr,
		Handler: handler,
	}

	rest.Log().Info().Msgf("Starting proxy server on %s", s.serverCfg.ListenAddr)
	rest.Log().Info().Msgf("Configured %d models", len(s.config.Models))

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *ProxyServer) Shutdown(ctx context.Context) error {
	rest.Log().Info().Msg("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// setupRoutes configures all HTTP routes
func (s *ProxyServer) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Observability endpoints
	mux.HandleFunc("/health", s.handleHealth)
	mux.Handle("/metrics", s.metrics.Handler())

	// Format-specific endpoints
	mux.HandleFunc("/openai/v1/chat/completions", s.handleOpenAI)
	mux.HandleFunc("/anthropic/v1/messages", s.handleAnthropic)
	mux.HandleFunc("/gemini/v1/models/", s.handleGemini)
	mux.HandleFunc("/gemini/v1beta/models/", s.handleGemini)

	// Embedded UI - serve at root path
	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		rest.Log().Warn().Err(err).Msg("failed to load embedded UI")
	} else {
		mux.Handle("/", http.FileServer(http.FS(fsys)))
		rest.Log().Info().Msg("embedded UI enabled at /")
	}

	return mux
}

// applyMiddleware applies middleware chain
func (s *ProxyServer) applyMiddleware(h http.Handler) http.Handler {
	// Apply in reverse order (last middleware wraps first)
	h = RecoveryMiddleware()(h)
	h = LoggingMiddleware()(h)
	h = RequestIDMiddleware(h)
	return h
}

// handleHealth handles the /health endpoint
func (s *ProxyServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"status":    "healthy",
		"models":    len(s.config.Models),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

// validateProviders checks that all configured providers have valid credentials
func (s *ProxyServer) validateProviders() error {
	providers := s.config.GetProviders()

	for _, provider := range providers {
		// Try to create a client for each provider
		_, err := s.clientPool.GetClient(provider)
		if err != nil {
			return fmt.Errorf("failed to initialize provider %s: %w", provider, err)
		}
	}

	rest.Log().Info().Msgf("Validated %d providers", len(providers))
	return nil
}
