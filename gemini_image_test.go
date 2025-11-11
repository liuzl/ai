package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGeminiImageURLAutoConversion tests that Gemini automatically converts image URLs to base64
func TestGeminiImageURLAutoConversion(t *testing.T) {
	// Create a test image (1x1 PNG)
	testImageData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xDD, 0x8D,
		0xB4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82, // IEND chunk
	}
	expectedBase64 := base64.StdEncoding.EncodeToString(testImageData)

	// Create mock image server
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write(testImageData)
	}))
	defer imageServer.Close()

	// Create mock Gemini API server
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify the image was converted to base64
		contents, ok := reqBody["contents"].([]any)
		if !ok || len(contents) == 0 {
			t.Error("No contents in request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		content := contents[0].(map[string]any)
		parts, ok := content["parts"].([]any)
		if !ok || len(parts) < 2 {
			t.Error("Expected at least 2 parts (text + image)")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Check that the second part is an inline image
		imagePart := parts[1].(map[string]any)
		inlineData, ok := imagePart["inlineData"].(map[string]any)
		if !ok {
			t.Error("Expected inlineData in image part")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify base64 data matches
		actualData, ok := inlineData["data"].(string)
		if !ok {
			t.Error("Expected string data in inlineData")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if actualData != expectedBase64 {
			t.Errorf("Base64 data mismatch.\nExpected: %s\nGot: %s", expectedBase64, actualData)
		}

		// Verify MIME type
		mimeType, ok := inlineData["mimeType"].(string)
		if !ok || mimeType != "image/png" {
			t.Errorf("Expected mimeType 'image/png', got: %s", mimeType)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {
					"parts": [{"text": "I can see the image"}]
				}
			}]
		}`))
	}))
	defer geminiServer.Close()

	// Create client with mock server
	client, err := NewClient(
		WithProvider(ProviderGemini),
		WithAPIKey("test-key"),
		WithBaseURL(geminiServer.URL),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create request with image URL
	req := &Request{
		Messages: []Message{
			{
				Role: RoleUser,
				ContentParts: []ContentPart{
					{Type: ContentTypeText, Text: "What's in this image?"},
					NewImagePartFromURL(imageServer.URL),
				},
			},
		},
	}

	// Make request
	ctx := context.Background()
	resp, err := client.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "I can see the image") {
		t.Errorf("Unexpected response: %s", resp.Text)
	}
}

// TestGeminiImageURLDownloadFailure tests error handling when image download fails
func TestGeminiImageURLDownloadFailure(t *testing.T) {
	// Create mock Gemini API server (won't be reached)
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Should not reach API server when image download fails")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer geminiServer.Close()

	// Create client
	client, err := NewClient(
		WithProvider(ProviderGemini),
		WithAPIKey("test-key"),
		WithBaseURL(geminiServer.URL),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create request with invalid image URL
	req := &Request{
		Messages: []Message{
			{
				Role: RoleUser,
				ContentParts: []ContentPart{
					{Type: ContentTypeText, Text: "What's in this image?"},
					NewImagePartFromURL("http://invalid-url-that-does-not-exist.example.com/image.png"),
				},
			},
		},
	}

	// Make request - should fail during image download
	ctx := context.Background()
	_, err = client.Generate(ctx, req)
	if err == nil {
		t.Fatal("Expected error when image download fails, got nil")
	}

	if !strings.Contains(err.Error(), "failed to download image") {
		t.Errorf("Expected error about image download, got: %v", err)
	}
}

// TestGeminiImageBase64StillWorks tests that base64 images still work (no regression)
func TestGeminiImageBase64StillWorks(t *testing.T) {
	testImageData := []byte{0x89, 0x50, 0x4E, 0x47}
	base64Data := base64.StdEncoding.EncodeToString(testImageData)

	// Create mock Gemini API server
	geminiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request contains inline data
		var reqBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [{
				"content": {
					"parts": [{"text": "Base64 image received"}]
				}
			}]
		}`))
	}))
	defer geminiServer.Close()

	// Create client
	client, err := NewClient(
		WithProvider(ProviderGemini),
		WithAPIKey("test-key"),
		WithBaseURL(geminiServer.URL),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create request with base64 image
	req := &Request{
		Messages: []Message{
			{
				Role: RoleUser,
				ContentParts: []ContentPart{
					{Type: ContentTypeText, Text: "What's in this image?"},
					NewImagePartFromBase64(base64Data, "png"),
				},
			},
		},
	}

	// Make request
	ctx := context.Background()
	resp, err := client.Generate(ctx, req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "Base64 image received") {
		t.Errorf("Unexpected response: %s", resp.Text)
	}
}
