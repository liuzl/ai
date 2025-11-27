package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/liuzl/ai"
)

// handleOpenAI handles OpenAI format requests
func (s *ProxyServer) handleOpenAI(w http.ResponseWriter, r *http.Request) {
	s.handleRequest(w, r, ai.ProviderOpenAI)
}

// handleAnthropic handles Anthropic format requests
func (s *ProxyServer) handleAnthropic(w http.ResponseWriter, r *http.Request) {
	s.handleRequest(w, r, ai.ProviderAnthropic)
}

// handleGemini handles Gemini format requests
func (s *ProxyServer) handleGemini(w http.ResponseWriter, r *http.Request) {
	s.handleRequest(w, r, ai.ProviderGemini)
}

// handleRequest is the core request handling logic
func (s *ProxyServer) handleRequest(w http.ResponseWriter, r *http.Request, format ai.Provider) {
	// Get request context
	requestID := GetRequestID(r.Context())
	startTime := time.Now()

	// Only accept POST requests
	if r.Method != http.MethodPost {
		s.handleError(w, r, format, "", "", fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	// Get format converter
	converter, err := s.converterFactory.GetConverter(format)
	if err != nil {
		s.handleError(w, r, format, "", "", err, http.StatusInternalServerError)
		return
	}

	// Decode provider-specific request
	providerReq, err := converter.DecodeRequest(r)
	if err != nil {
		s.handleError(w, r, format, "", "", fmt.Errorf("failed to decode request: %w", err), http.StatusBadRequest)
		return
	}

	// Convert to Universal format to extract model
	universalReq, err := converter.ConvertRequestFromFormat(providerReq)
	if err != nil {
		s.handleError(w, r, format, "", "", fmt.Errorf("failed to convert request: %w", err), http.StatusBadRequest)
		return
	}

	model := universalReq.Model

	// Look up provider for model in config
	provider, err := s.config.GetProviderForModel(model)
	if err != nil {
		s.handleError(w, r, format, model, "", err, http.StatusBadRequest)
		return
	}

	// Increment active requests
	s.metrics.IncActiveRequests(string(format), string(provider))
	defer s.metrics.DecActiveRequests(string(format), string(provider))

	// Get client from pool
	client, err := s.clientPool.GetClient(provider)
	if err != nil {
		s.handleError(w, r, format, model, string(provider), err, http.StatusInternalServerError)
		return
	}

	// Check if streaming
	if converter.IsStreaming(providerReq) {
		s.handleStream(w, r, format, model, string(provider), converter, providerReq, client)
		return
	}

	// Call backend (non-streaming)
	universalResp, err := client.Generate(r.Context(), universalReq)
	if err != nil {
		s.handleError(w, r, format, model, string(provider), err, http.StatusInternalServerError)
		return
	}

	// Convert response to original format
	providerResp, err := converter.ConvertResponseToFormat(universalResp, model)
	if err != nil {
		s.handleError(w, r, format, model, string(provider), fmt.Errorf("failed to convert response: %w", err), http.StatusInternalServerError)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(providerResp); err != nil {
		s.logger.Error(LogEntry{
			RequestID: requestID,
			Message:   "failed to encode response",
			Error:     err.Error(),
			Format:    string(format),
			Model:     model,
			Provider:  string(provider),
		})
		return
	}

	// Record metrics and log
	duration := time.Since(startTime)
	s.metrics.RecordRequest(string(format), model, string(provider), "success", duration)
	s.logger.Info(LogEntry{
		RequestID:  requestID,
		Message:    "request completed successfully",
		Duration:   float64(duration.Milliseconds()),
		StatusCode: http.StatusOK,
		Format:     string(format),
		Model:      model,
		Provider:   string(provider),
		Streaming:  false,
	})
}

// handleStream handles streaming requests
func (s *ProxyServer) handleStream(
	w http.ResponseWriter,
	r *http.Request,
	format ai.Provider,
	model string,
	provider string,
	converter ai.FormatConverter,
	providerReq any,
	client ai.Client,
) {
	requestID := GetRequestID(r.Context())
	startTime := time.Now()

	// Get streaming client
	streamingClient, ok := client.(ai.StreamingClient)
	if !ok {
		s.handleError(w, r, format, model, provider,
			fmt.Errorf("provider %s does not support streaming", provider),
			http.StatusNotImplemented)
		return
	}

	// Convert to Universal format
	universalReq, err := converter.ConvertRequestFromFormat(providerReq)
	if err != nil {
		s.handleError(w, r, format, model, provider,
			fmt.Errorf("failed to convert request: %w", err),
			http.StatusBadRequest)
		return
	}

	// Start streaming
	streamReader, err := streamingClient.Stream(r.Context(), universalReq)
	if err != nil {
		s.handleError(w, r, format, model, provider, err, http.StatusInternalServerError)
		return
	}
	defer streamReader.Close()

	// Create stream handler for format
	streamHandler := converter.NewStreamHandler(requestID, model)

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.handleError(w, r, format, model, provider,
			fmt.Errorf("streaming not supported"),
			http.StatusInternalServerError)
		return
	}

	// Start streaming
	streamHandler.OnStart(w, flusher)

	// Read and forward chunks
	for {
		chunk, err := streamReader.Recv()
		if err != nil {
			streamHandler.OnError(w, flusher, err)
			s.logger.Error(LogEntry{
				RequestID: requestID,
				Message:   "streaming error",
				Error:     err.Error(),
				Format:    string(format),
				Model:     model,
				Provider:  provider,
			})
			break
		}

		if chunk.Done {
			streamHandler.OnEnd(w, flusher)
			break
		}

		if err := streamHandler.OnChunk(w, flusher, chunk); err != nil {
			streamHandler.OnError(w, flusher, err)
			s.logger.Error(LogEntry{
				RequestID: requestID,
				Message:   "failed to write chunk",
				Error:     err.Error(),
				Format:    string(format),
				Model:     model,
				Provider:  provider,
			})
			break
		}
	}

	// Record metrics and log
	duration := time.Since(startTime)
	s.metrics.RecordRequest(string(format), model, provider, "success", duration)
	s.logger.Info(LogEntry{
		RequestID:  requestID,
		Message:    "streaming request completed",
		Duration:   float64(duration.Milliseconds()),
		StatusCode: http.StatusOK,
		Format:     string(format),
		Model:      model,
		Provider:   provider,
		Streaming:  true,
	})
}

// handleError handles error responses
func (s *ProxyServer) handleError(
	w http.ResponseWriter,
	r *http.Request,
	format ai.Provider,
	model string,
	provider string,
	err error,
	statusCode int,
) {
	requestID := GetRequestID(r.Context())

	// Determine error type
	errorType := getErrorType(err)

	// Record metrics
	s.metrics.RecordError(string(format), model, provider, errorType)

	// Log error
	s.logger.Error(LogEntry{
		RequestID:  requestID,
		Message:    "request failed",
		Error:      err.Error(),
		StatusCode: statusCode,
		Format:     string(format),
		Model:      model,
		Provider:   provider,
	})

	// Write error response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"message":    err.Error(),
			"type":       errorType,
			"request_id": requestID,
		},
	}

	json.NewEncoder(w).Encode(errorResponse)
}

// getErrorType maps errors to error types for metrics
func getErrorType(err error) string {
	if err == nil {
		return "unknown"
	}

	switch err.(type) {
	case *ai.AuthenticationError:
		return "auth"
	case *ai.RateLimitError:
		return "rate_limit"
	case *ai.InvalidRequestError:
		return "invalid_request"
	case *ai.TimeoutError:
		return "timeout"
	case *ai.NetworkError:
		return "network"
	case *ai.ServerError:
		return "server_error"
	default:
		errStr := err.Error()
		if strings.Contains(errStr, "unknown model") {
			return "unknown_model"
		}
		if strings.Contains(errStr, "failed to decode") {
			return "decode_error"
		}
		if strings.Contains(errStr, "failed to convert") {
			return "conversion_error"
		}
		return "unknown"
	}
}
