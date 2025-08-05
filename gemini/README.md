# Gemini API Go Client

A Go client library for Google Gemini API, providing complete REST API encapsulation.

## Features

- ✅ Complete content generation API support
- ✅ Embedding generation API support
- ✅ Batch embedding API support
- ✅ Token counting API support
- ✅ Model listing and query API support
- ✅ File upload and management API support
- ✅ Safety settings support
- ✅ Tools and function calling support
- ✅ Automatic retry mechanism
- ✅ Error handling
- ✅ Context support
- ✅ API version configuration support

## Installation

```bash
go get github.com/liuzl/ai/gemini
```

## Quick Start

### 1. Create Client

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/liuzl/ai/gemini"
)

func main() {
    // Create client
    client := gemini.NewClient("your-api-key-here",
        gemini.WithTimeout(30*time.Second),
        gemini.WithMaxRetries(3),
        gemini.WithAPIVersion("v1beta"), // Optional: specify API version
    )
    
    ctx := context.Background()
    
    // Generate content
    req := &gemini.GenerateContentRequest{
        Contents: []gemini.Content{
            {
                Parts: []gemini.Part{
                    {
                        Text: gemini.StringPtr("Explain quantum computing concepts"),
                    },
                },
            },
        },
        GenerationConfig: &gemini.GenerationConfig{
            Temperature:     gemini.Float64Ptr(0.7),
            MaxOutputTokens: gemini.IntPtr(1000),
        },
    }
    
    resp, err := client.GenerateContent(ctx, "gemini-2.5-flash", req)
    if err != nil {
        log.Fatal(err)
    }
    
    if len(resp.Candidates) > 0 {
        fmt.Printf("Generated text: %s\n", *resp.Candidates[0].Content.Parts[0].Text)
    }
}
```

### 2. Configuration Options

The client supports various configuration options:

```go
client := gemini.NewClient("your-api-key",
    // Base URL configuration
    gemini.WithBaseURL("https://generativelanguage.googleapis.com"),
    
    // API version configuration
    gemini.WithAPIVersion("v1beta"),
    
    // Timeout configuration
    gemini.WithTimeout(30*time.Second),
    
    // Retry configuration
    gemini.WithMaxRetries(3),
    
    // Custom HTTP client
    gemini.WithHTTPClient(&http.Client{
        Timeout: 60 * time.Second,
    }),
)
```

### 3. Generate Embeddings

```go
// Generate embeddings
embedReq := &gemini.EmbedContentRequest{
    Model: "embedding-001",
    Content: gemini.Content{
        Parts: []gemini.Part{
            {
                Text: gemini.StringPtr("This is a sample text for embedding"),
            },
        },
    },
}

embedResp, err := client.EmbedContent(ctx, embedReq)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Embedding dimensions: %d\n", len(embedResp.Embedding.Values))
```

### 4. Use Safety Settings

```go
req := &gemini.GenerateContentRequest{
    Contents: []gemini.Content{
        {
            Parts: []gemini.Part{
                {
                    Text: gemini.StringPtr("Write a story about a hero"),
                },
            },
        },
    },
    SafetySettings: []gemini.SafetySetting{
        {
            Category:  gemini.HarmCategoryDangerousContent,
            Threshold: gemini.BlockMediumAndAbove,
        },
        {
            Category:  gemini.HarmCategoryHarassment,
            Threshold: gemini.BlockLowAndAbove,
        },
    },
}
```

### 5. Use Tools and Function Calling

```go
// Define function
function := gemini.FunctionDeclaration{
    Name:        "get_weather",
    Description: "Get current weather for a location",
    Parameters: &gemini.Schema{
        Type: "object",
        Properties: map[string]gemini.Schema{
            "location": {
                Type:        "string",
                Description: gemini.StringPtr("City and state, e.g. San Francisco, CA"),
            },
            "unit": {
                Type: "string",
                Enum: []string{"celsius", "fahrenheit"},
            },
        },
        Required: []string{"location"},
    },
}

req := &gemini.GenerateContentRequest{
    Contents: []gemini.Content{
        {
            Parts: []gemini.Part{
                {
                    Text: gemini.StringPtr("What's the weather like in Tokyo?"),
                },
            },
        },
    },
    Tools: []gemini.Tool{
        {
            FunctionDeclarations: []gemini.FunctionDeclaration{function},
        },
    },
}
```

### 6. File Operations

```go
// Upload file
uploadReq := &gemini.UploadFileRequest{
    File: gemini.File{
        DisplayName: "example.pdf",
        MimeType:    "application/pdf",
    },
    FileData: fileBytes, // File byte data
}

uploadResp, err := client.UploadFile(ctx, uploadReq)
if err != nil {
    log.Fatal(err)
}

// List files
filesResp, err := client.ListFiles(ctx)
if err != nil {
    log.Fatal(err)
}

for _, file := range filesResp.Files {
    fmt.Printf("File: %s (%s)\n", file.DisplayName, file.MimeType)
}
```

## API Methods

### Content Generation

- `GenerateContent(ctx, model, req)` - Generate content

### Embeddings

- `EmbedContent(ctx, req)` - Generate single embedding
- `BatchEmbedContents(ctx, req)` - Generate batch embeddings

### Token Counting

- `CountTokens(ctx, model, req)` - Count tokens

### Model Management

- `ListModels(ctx)` - List available models
- `GetModel(ctx, model)` - Get model information

### File Management

- `UploadFile(ctx, req)` - Upload file
- `ListFiles(ctx)` - List files
- `GetFile(ctx, name)` - Get file information
- `DeleteFile(ctx, name)` - Delete file

## Configuration Options

### Client Options

- `WithBaseURL(baseURL)` - Set base URL
- `WithAPIVersion(apiVersion)` - Set API version
- `WithTimeout(timeout)` - Set timeout
- `WithMaxRetries(maxRetries)` - Set maximum retries
- `WithHTTPClient(httpClient)` - Set custom HTTP client

### Generation Configuration

- `Temperature` - Temperature parameter (0.0-1.0)
- `TopP` - Top-p parameter
- `TopK` - Top-k parameter
- `MaxOutputTokens` - Maximum output tokens
- `CandidateCount` - Number of candidates
- `StopSequences` - Stop sequences

### Safety Settings

- `HarmCategoryHarassment` - Harassment content
- `HarmCategoryHateSpeech` - Hate speech content
- `HarmCategorySexuallyExplicit` - Sexually explicit content
- `HarmCategoryDangerousContent` - Dangerous content

- `BlockThresholdUnspecified` - Unspecified
- `BlockLowAndAbove` - Low and above
- `BlockMediumAndAbove` - Medium and above
- `BlockOnlyHigh` - High only
- `BlockNone` - Block none

## URL Construction

The client automatically constructs complete API URLs:

```
BaseURL + "/" + APIVersion + Path
```

Example:
- **BaseURL**: `https://generativelanguage.googleapis.com`
- **APIVersion**: `v1beta`
- **Path**: `/models/gemini-2.0-flash-exp:generateContent`

**Final URL**: `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-exp:generateContent`

## Error Handling

The client returns detailed error information:

```go
resp, err := client.GenerateContent(ctx, model, req)
if err != nil {
    if apiErr, ok := err.(*gemini.APIError); ok {
        fmt.Printf("API error %d: %s\n", apiErr.StatusCode, apiErr.APIError.Message)
    } else {
        fmt.Printf("Other error: %v\n", err)
    }
    return
}
```

## License

MIT License
