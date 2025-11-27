package ai

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDownloadImageToBase64(t *testing.T) {
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

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write(testImageData)
	}))
	defer server.Close()

	// Test successful download with timeout in context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	base64Data, format, err := downloadImageToBase64(ctx, server.URL)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if base64Data != expectedBase64 {
		t.Errorf("Base64 data mismatch.\nExpected: %s\nGot: %s", expectedBase64, base64Data)
	}

	if format != "png" {
		t.Errorf("Expected format 'png', got: %s", format)
	}
}

func TestDownloadImageToBase64_JPEG(t *testing.T) {
	// Create mock server with JPEG content type
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0}) // JPEG header
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, format, err := downloadImageToBase64(ctx, server.URL)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if format != "jpg" {
		t.Errorf("Expected format 'jpg', got: %s", format)
	}
}

func TestDownloadImageToBase64_404(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := downloadImageToBase64(ctx, server.URL)
	if err == nil {
		t.Fatal("Expected error for 404 response, got nil")
	}

	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("Expected error about 404, got: %v", err)
	}
}

func TestDownloadImageToBase64_Timeout(t *testing.T) {
	// Create mock server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Use very short timeout in context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, err := downloadImageToBase64(ctx, server.URL)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

func TestDownloadImageToBase64_ContextCancellation(t *testing.T) {
	// Create mock server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Use cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, err := downloadImageToBase64(ctx, server.URL)
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}
}

func TestDetectImageFormat(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		url         string
		expected    string
	}{
		{
			name:        "PNG from Content-Type",
			contentType: "image/png",
			url:         "https://example.com/test",
			expected:    "png",
		},
		{
			name:        "JPEG from Content-Type",
			contentType: "image/jpeg",
			url:         "https://example.com/test",
			expected:    "jpg",
		},
		{
			name:        "JPG from Content-Type",
			contentType: "image/jpg",
			url:         "https://example.com/test",
			expected:    "jpg",
		},
		{
			name:        "GIF from Content-Type",
			contentType: "image/gif",
			url:         "https://example.com/test",
			expected:    "gif",
		},
		{
			name:        "WebP from Content-Type",
			contentType: "image/webp",
			url:         "https://example.com/test",
			expected:    "webp",
		},
		{
			name:        "PNG from URL extension",
			contentType: "",
			url:         "https://example.com/image.png",
			expected:    "png",
		},
		{
			name:        "JPEG from URL extension",
			contentType: "",
			url:         "https://example.com/image.jpeg",
			expected:    "jpg",
		},
		{
			name:        "JPG from URL extension",
			contentType: "",
			url:         "https://example.com/image.jpg",
			expected:    "jpg",
		},
		{
			name:        "GIF from URL extension",
			contentType: "",
			url:         "https://example.com/image.gif",
			expected:    "gif",
		},
		{
			name:        "Default to PNG",
			contentType: "",
			url:         "https://example.com/test",
			expected:    "png",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detectImageFormat(tc.contentType, tc.url)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}
