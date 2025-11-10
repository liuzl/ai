package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMultimodalImageURL tests image input via URL for all providers
func TestMultimodalImageURL(t *testing.T) {
	testCases := []struct {
		name     string
		provider Provider
	}{
		{"OpenAI", ProviderOpenAI},
		{"Anthropic", ProviderAnthropic},
	}

	for _, tc := range testCases {
		t.Run(string(tc.provider), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				bodyStr := string(body)

				// Verify that the request contains image URL
				if !strings.Contains(bodyStr, "https://example.com/image.jpg") {
					t.Errorf("Expected image URL in request body, got: %s", bodyStr)
				}

				// Verify provider-specific format
				switch tc.provider {
				case ProviderOpenAI:
					if !strings.Contains(bodyStr, `"image_url"`) {
						t.Errorf("OpenAI request should contain image_url field")
					}
				case ProviderAnthropic:
					if !strings.Contains(bodyStr, `"image"`) || !strings.Contains(bodyStr, `"url"`) {
						t.Errorf("Anthropic request should contain image type with url")
					}
				}

				// Return mock response
				var resp string
				switch tc.provider {
				case ProviderOpenAI:
					resp = `{"choices":[{"message":{"role":"assistant","content":"I see an image"}}]}`
				case ProviderAnthropic:
					resp = `{"content":[{"type":"text","text":"I see an image"}]}`
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(resp))
			}))
			defer server.Close()

			client, err := NewClient(
				WithProvider(tc.provider),
				WithAPIKey("test-key"),
				WithBaseURL(server.URL),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			req := &Request{
				Messages: []Message{
					NewMultimodalMessage(RoleUser, []ContentPart{
						NewTextPart("What's in this image?"),
						NewImagePartFromURL("https://example.com/image.jpg"),
					}),
				},
			}

			resp, err := client.Generate(context.Background(), req)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			if !strings.Contains(resp.Text, "I see an image") {
				t.Errorf("Expected response text, got: %s", resp.Text)
			}
		})
	}
}

// TestMultimodalImageBase64 tests base64 image input for all providers
func TestMultimodalImageBase64(t *testing.T) {
	testCases := []struct {
		name     string
		provider Provider
	}{
		{"OpenAI", ProviderOpenAI},
		{"Gemini", ProviderGemini},
		{"Anthropic", ProviderAnthropic},
	}

	base64Image := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	for _, tc := range testCases {
		t.Run(string(tc.provider), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				bodyStr := string(body)

				// Verify that the request contains base64 image data
				if !strings.Contains(bodyStr, base64Image) {
					t.Errorf("Expected base64 image in request body")
				}

				// Verify provider-specific format
				switch tc.provider {
				case ProviderOpenAI:
					if !strings.Contains(bodyStr, `"image_url"`) && !strings.Contains(bodyStr, "data:image") {
						t.Errorf("OpenAI request should contain image_url field with data URI")
					}
				case ProviderGemini:
					if !strings.Contains(bodyStr, `"inlineData"`) || !strings.Contains(bodyStr, `"mimeType"`) {
						t.Errorf("Gemini request should contain inlineData with mimeType")
					}
				case ProviderAnthropic:
					if !strings.Contains(bodyStr, `"image"`) || !strings.Contains(bodyStr, `"base64"`) {
						t.Errorf("Anthropic request should contain image type with base64")
					}
				}

				// Return mock response
				var resp string
				switch tc.provider {
				case ProviderOpenAI:
					resp = `{"choices":[{"message":{"role":"assistant","content":"I see an image"}}]}`
				case ProviderGemini:
					resp = `{"candidates":[{"content":{"parts":[{"text":"I see an image"}]}}]}`
				case ProviderAnthropic:
					resp = `{"content":[{"type":"text","text":"I see an image"}]}`
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(resp))
			}))
			defer server.Close()

			client, err := NewClient(
				WithProvider(tc.provider),
				WithAPIKey("test-key"),
				WithBaseURL(server.URL),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			req := &Request{
				Messages: []Message{
					NewMultimodalMessage(RoleUser, []ContentPart{
						NewTextPart("What's in this image?"),
						NewImagePartFromBase64(base64Image, "png"),
					}),
				},
			}

			resp, err := client.Generate(context.Background(), req)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			if !strings.Contains(resp.Text, "I see an image") {
				t.Errorf("Expected response text, got: %s", resp.Text)
			}
		})
	}
}

// TestMultimodalMixedContent tests text + multiple images
func TestMultimodalMixedContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		// Verify multiple images
		if !strings.Contains(bodyStr, "image1.jpg") || !strings.Contains(bodyStr, "image2.jpg") {
			t.Errorf("Expected both images in request")
		}

		resp := `{"choices":[{"message":{"role":"assistant","content":"I see two images"}}]}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client, err := NewClient(
		WithProvider(ProviderOpenAI),
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &Request{
		Messages: []Message{
			NewMultimodalMessage(RoleUser, []ContentPart{
				NewTextPart("Compare these images:"),
				NewImagePartFromURL("https://example.com/image1.jpg"),
				NewImagePartFromURL("https://example.com/image2.jpg"),
			}),
		},
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(resp.Text, "I see two images") {
		t.Errorf("Expected response text, got: %s", resp.Text)
	}
}

// TestBackwardCompatibilityTextOnly tests that text-only messages still work
func TestBackwardCompatibilityTextOnly(t *testing.T) {
	testCases := []struct {
		name     string
		provider Provider
	}{
		{"OpenAI", ProviderOpenAI},
		{"Gemini", ProviderGemini},
		{"Anthropic", ProviderAnthropic},
	}

	for _, tc := range testCases {
		t.Run(string(tc.provider), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				bodyStr := string(body)

				// Verify that text-only content still works
				if !strings.Contains(bodyStr, "Hello") {
					t.Errorf("Expected text content in request")
				}

				// Return mock response
				var resp string
				switch tc.provider {
				case ProviderOpenAI:
					resp = `{"choices":[{"message":{"role":"assistant","content":"Hello back"}}]}`
				case ProviderGemini:
					resp = `{"candidates":[{"content":{"parts":[{"text":"Hello back"}]}}]}`
				case ProviderAnthropic:
					resp = `{"content":[{"type":"text","text":"Hello back"}]}`
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(resp))
			}))
			defer server.Close()

			client, err := NewClient(
				WithProvider(tc.provider),
				WithAPIKey("test-key"),
				WithBaseURL(server.URL),
			)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			// Test using old Message constructor
			req := &Request{
				Messages: []Message{
					NewTextMessage(RoleUser, "Hello"),
				},
			}

			resp, err := client.Generate(context.Background(), req)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			if !strings.Contains(resp.Text, "Hello back") {
				t.Errorf("Expected response text, got: %s", resp.Text)
			}
		})
	}
}

// TestImagePartFromBase64WithDataURI tests base64 with data URI prefix
func TestImagePartFromBase64WithDataURI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		// The data URI prefix should be handled correctly
		var reqData map[string]interface{}
		if err := json.Unmarshal(body, &reqData); err != nil {
			t.Fatalf("Failed to parse request: %v", err)
		}

		resp := `{"candidates":[{"content":{"parts":[{"text":"Processed"}]}}]}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client, err := NewClient(
		WithProvider(ProviderGemini),
		WithAPIKey("test-key"),
		WithBaseURL(server.URL),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with data URI prefix
	dataURI := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	req := &Request{
		Messages: []Message{
			NewMultimodalMessage(RoleUser, []ContentPart{
				NewTextPart("Process this"),
				NewImagePartFromBase64(dataURI, "png"),
			}),
		},
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if resp.Text != "Processed" {
		t.Errorf("Expected 'Processed', got: %s", resp.Text)
	}
}

// TestHelperFunctions tests the helper functions for creating content parts
func TestHelperFunctions(t *testing.T) {
	t.Run("NewTextPart", func(t *testing.T) {
		part := NewTextPart("Hello")
		if part.Type != ContentTypeText || part.Text != "Hello" {
			t.Errorf("NewTextPart failed: %+v", part)
		}
	})

	t.Run("NewImagePartFromURL", func(t *testing.T) {
		part := NewImagePartFromURL("https://example.com/image.jpg")
		if part.Type != ContentTypeImage || part.ImageSource == nil {
			t.Errorf("NewImagePartFromURL failed: %+v", part)
		}
		if part.ImageSource.Type != ImageSourceTypeURL || part.ImageSource.URL != "https://example.com/image.jpg" {
			t.Errorf("Image source incorrect: %+v", part.ImageSource)
		}
	})

	t.Run("NewImagePartFromBase64", func(t *testing.T) {
		part := NewImagePartFromBase64("base64data", "png")
		if part.Type != ContentTypeImage || part.ImageSource == nil {
			t.Errorf("NewImagePartFromBase64 failed: %+v", part)
		}
		if part.ImageSource.Type != ImageSourceTypeBase64 || part.ImageSource.Data != "base64data" || part.ImageSource.Format != "png" {
			t.Errorf("Image source incorrect: %+v", part.ImageSource)
		}
	})

	t.Run("NewTextMessage", func(t *testing.T) {
		msg := NewTextMessage(RoleUser, "Hello")
		if msg.Role != RoleUser || msg.Content != "Hello" {
			t.Errorf("NewTextMessage failed: %+v", msg)
		}
	})

	t.Run("NewMultimodalMessage", func(t *testing.T) {
		parts := []ContentPart{
			NewTextPart("Hello"),
			NewImagePartFromURL("https://example.com/image.jpg"),
		}
		msg := NewMultimodalMessage(RoleUser, parts)
		if msg.Role != RoleUser || len(msg.ContentParts) != 2 {
			t.Errorf("NewMultimodalMessage failed: %+v", msg)
		}
	})
}
