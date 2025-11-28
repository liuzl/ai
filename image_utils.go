package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// Maximum size limits for downloaded media to prevent memory exhaustion
	maxImageSize    = 100 * 1024 * 1024 // 100 MB
	maxMediaSize    = 500 * 1024 * 1024 // 500 MB for video/audio
	maxResponseSize = 10 * 1024 * 1024  // 10 MB for API responses
)

// downloadImageToBase64 downloads an image from a URL and converts it to base64.
// This is used for providers like Gemini that don't support image URLs directly.
// The context should already have a timeout if needed.
func downloadImageToBase64(ctx context.Context, imageURL string) (string, string, error) {
	// Create HTTP client - timeout is controlled by the context
	client := &http.Client{}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create image download request: %w", err)
	}

	// Set User-Agent to avoid 403 errors from servers that block requests without it
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AI-Library/1.0; +https://github.com/liuzl/ai)")
	req.Header.Set("Accept", "image/*")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to download image from URL: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to download image: HTTP %d", resp.StatusCode)
	}

	// Detect format from Content-Type header
	format := detectImageFormat(resp.Header.Get("Content-Type"), imageURL)

	// Stream encode to base64 using strings.Builder to minimize memory usage
	// (Avoids holding the raw image bytes in memory)
	var b strings.Builder
	// Optional: Pre-allocate builder if Content-Length is available and reasonable
	if resp.ContentLength > 0 && resp.ContentLength <= maxImageSize {
		// Base64 expansion is roughly 4/3
		growSize := int(resp.ContentLength*4/3 + 4)
		b.Grow(growSize)
	}

	encoder := base64.NewEncoder(base64.StdEncoding, &b)

	// Copy with size limit
	if _, err := io.Copy(encoder, io.LimitReader(resp.Body, maxImageSize)); err != nil {
		encoder.Close()
		return "", "", fmt.Errorf("failed to read/encode image data: %w", err)
	}
	encoder.Close()

	return b.String(), format, nil
}

// detectImageFormat detects image format from Content-Type header or URL extension.
func detectImageFormat(contentType, imageURL string) string {
	// Try to detect from Content-Type header first
	if contentType != "" {
		contentType = strings.ToLower(contentType)
		if strings.Contains(contentType, "image/jpeg") || strings.Contains(contentType, "image/jpg") {
			return "jpg"
		}
		if strings.Contains(contentType, "image/png") {
			return "png"
		}
		if strings.Contains(contentType, "image/gif") {
			return "gif"
		}
		if strings.Contains(contentType, "image/webp") {
			return "webp"
		}
	}

	// Fallback to URL extension
	imageURL = strings.ToLower(imageURL)
	if strings.HasSuffix(imageURL, ".jpg") || strings.HasSuffix(imageURL, ".jpeg") {
		return "jpg"
	}
	if strings.HasSuffix(imageURL, ".png") {
		return "png"
	}
	if strings.HasSuffix(imageURL, ".gif") {
		return "gif"
	}
	if strings.HasSuffix(imageURL, ".webp") {
		return "webp"
	}

	// Default to png
	return "png"
}

// downloadMediaToBase64 downloads media (audio, video, document) from a URL and converts it to base64.
// This is a generic function for downloading any media type.
func downloadMediaToBase64(ctx context.Context, mediaURL string) (string, error) {
	// Create HTTP client with context timeout
	client := &http.Client{}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create media download request: %w", err)
	}

	// Set User-Agent to avoid 403 errors
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AI-Library/1.0; +https://github.com/liuzl/ai)")
	req.Header.Set("Accept", "*/*")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download media from URL: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download media: HTTP %d", resp.StatusCode)
	}

	// Stream encode to base64 using strings.Builder to minimize memory usage
	var b strings.Builder
	// Optional: Pre-allocate builder if Content-Length is available and reasonable
	if resp.ContentLength > 0 && resp.ContentLength <= maxMediaSize {
		// Base64 expansion is roughly 4/3
		growSize := int(resp.ContentLength*4/3 + 4)
		b.Grow(growSize)
	}

	encoder := base64.NewEncoder(base64.StdEncoding, &b)

	// Copy with size limit
	if _, err := io.Copy(encoder, io.LimitReader(resp.Body, maxMediaSize)); err != nil {
		encoder.Close()
		return "", fmt.Errorf("failed to read/encode media data: %w", err)
	}
	encoder.Close()

	return b.String(), nil
}
