package gemini

import (
	"encoding/json"
	"time"
)

// Request Types

// GenerateContentRequest represents the main request structure for generating content
type GenerateContentRequest struct {
	Contents          []Content         `json:"contents"`
	GenerationConfig  *GenerationConfig `json:"generationConfig,omitempty"`
	SafetySettings    []SafetySetting   `json:"safetySettings,omitempty"`
	Tools             []Tool            `json:"tools,omitempty"`
	SystemInstruction *Content          `json:"systemInstruction,omitempty"`
}

// Content represents a content item with parts and optional role
type Content struct {
	Parts []Part  `json:"parts"`
	Role  *string `json:"role,omitempty"`
}

// Part represents a content part (text, image, file data, etc.)
type Part struct {
	Text             *string           `json:"text,omitempty"`
	InlineData       *InlineData       `json:"inlineData,omitempty"`
	FileData         *FileData         `json:"fileData,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
}

// InlineData represents inline data (like base64 encoded images)
type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// FileData represents file data reference
type FileData struct {
	MimeType string `json:"mimeType"`
	FileURI  string `json:"fileUri"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// FunctionResponse represents a function response
type FunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

// GenerationConfig represents configuration for content generation
type GenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	CandidateCount  *int     `json:"candidateCount,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

// SafetySetting represents safety settings for content generation
type SafetySetting struct {
	Category  HarmCategory       `json:"category"`
	Threshold HarmBlockThreshold `json:"threshold"`
}

// HarmCategory represents different categories of harmful content
type HarmCategory string

const (
	HarmCategoryHarassment       HarmCategory = "HARM_CATEGORY_HARASSMENT"
	HarmCategoryHateSpeech       HarmCategory = "HARM_CATEGORY_HATE_SPEECH"
	HarmCategorySexuallyExplicit HarmCategory = "HARM_CATEGORY_SEXUALLY_EXPLICIT"
	HarmCategoryDangerousContent HarmCategory = "HARM_CATEGORY_DANGEROUS_CONTENT"
)

// HarmBlockThreshold represents the threshold for blocking harmful content
type HarmBlockThreshold string

const (
	BlockThresholdUnspecified HarmBlockThreshold = "BLOCK_THRESHOLD_UNSPECIFIED"
	BlockLowAndAbove          HarmBlockThreshold = "BLOCK_LOW_AND_ABOVE"
	BlockMediumAndAbove       HarmBlockThreshold = "BLOCK_MEDIUM_AND_ABOVE"
	BlockOnlyHigh             HarmBlockThreshold = "BLOCK_ONLY_HIGH"
	BlockNone                 HarmBlockThreshold = "BLOCK_NONE"
)

// Tool represents a tool that can be used by the model
type Tool struct {
	FunctionDeclarations  []FunctionDeclaration  `json:"functionDeclarations,omitempty"`
	GoogleSearchRetrieval *GoogleSearchRetrieval `json:"googleSearchRetrieval,omitempty"`
}

// FunctionDeclaration represents a function declaration
type FunctionDeclaration struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Parameters  *Schema `json:"parameters"`
}

// Schema represents a JSON schema
type Schema struct {
	Type                 string            `json:"type"`
	Format               *string           `json:"format,omitempty"`
	Description          *string           `json:"description,omitempty"`
	Nullable             *bool             `json:"nullable,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	Enum                 []string          `json:"enum,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Required             []string          `json:"required,omitempty"`
	Ref                  *string           `json:"$ref,omitempty"`
	AdditionalProperties *Schema           `json:"additionalProperties,omitempty"`
}

// GoogleSearchRetrieval represents Google Search retrieval tool
type GoogleSearchRetrieval struct {
	DisableAttribution bool `json:"disableAttribution"`
}

// Response Types

// GenerateContentResponse represents the response from content generation
type GenerateContentResponse struct {
	Candidates     []Candidate     `json:"candidates"`
	PromptFeedback *PromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *UsageMetadata  `json:"usageMetadata,omitempty"`
}

// Candidate represents a generated content candidate
type Candidate struct {
	Content       Content        `json:"content"`
	FinishReason  *FinishReason  `json:"finishReason,omitempty"`
	Index         *int           `json:"index,omitempty"`
	SafetyRatings []SafetyRating `json:"safetyRatings,omitempty"`
	TokenCount    *int           `json:"tokenCount,omitempty"`
}

// FinishReason represents the reason why generation finished
type FinishReason string

const (
	FinishReasonUnspecified FinishReason = "FINISH_REASON_UNSPECIFIED"
	FinishReasonStop        FinishReason = "STOP"
	FinishReasonMaxTokens   FinishReason = "MAX_TOKENS"
	FinishReasonSafety      FinishReason = "SAFETY"
	FinishReasonRecitation  FinishReason = "RECITATION"
	FinishReasonOther       FinishReason = "OTHER"
)

// SafetyRating represents a safety rating for generated content
type SafetyRating struct {
	Category    HarmCategory    `json:"category"`
	Probability HarmProbability `json:"probability"`
	Blocked     *bool           `json:"blocked,omitempty"`
}

// HarmProbability represents the probability of harm
type HarmProbability string

const (
	HarmProbabilityUnspecified HarmProbability = "HARM_PROBABILITY_UNSPECIFIED"
	HarmProbabilityNegligible  HarmProbability = "NEGLIGIBLE"
	HarmProbabilityLow         HarmProbability = "LOW"
	HarmProbabilityMedium      HarmProbability = "MEDIUM"
	HarmProbabilityHigh        HarmProbability = "HIGH"
)

// PromptFeedback represents feedback about the prompt
type PromptFeedback struct {
	SafetyRatings []SafetyRating `json:"safetyRatings"`
	BlockReason   *BlockReason   `json:"blockReason,omitempty"`
}

// BlockReason represents the reason for blocking
type BlockReason string

const (
	BlockReasonUnspecified BlockReason = "BLOCK_REASON_UNSPECIFIED"
	BlockReasonSafety      BlockReason = "SAFETY"
	BlockReasonOther       BlockReason = "OTHER"
)

// UsageMetadata represents usage metadata
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// Embedding Types

// EmbedContentRequest represents a request for generating embeddings
type EmbedContentRequest struct {
	Model    string  `json:"model"`
	Content  Content `json:"content"`
	TaskType *string `json:"taskType,omitempty"`
	Title    *string `json:"title,omitempty"`
}

// EmbedContentResponse represents the response from embedding generation
type EmbedContentResponse struct {
	Embedding Embedding `json:"embedding"`
}

// Embedding represents an embedding
type Embedding struct {
	Values []float64 `json:"values"`
}

// Batch Embedding Types

// BatchEmbedContentsRequest represents a batch request for embeddings
type BatchEmbedContentsRequest struct {
	Requests []EmbedContentRequest `json:"requests"`
}

// BatchEmbedContentsResponse represents a batch response for embeddings
type BatchEmbedContentsResponse struct {
	Embeddings []EmbedContentResponse `json:"embeddings"`
}

// Model Types

// Model represents a model
type Model struct {
	Name                       string   `json:"name"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	Temperature                *float64 `json:"temperature,omitempty"`
	TopP                       *float64 `json:"topP,omitempty"`
	TopK                       *int     `json:"topK,omitempty"`
}

// ListModelsResponse represents the response from listing models
type ListModelsResponse struct {
	Models []Model `json:"models"`
}

// Count Tokens Types

// CountTokensRequest represents a request for counting tokens
type CountTokensRequest struct {
	Contents []Content `json:"contents"`
}

// CountTokensResponse represents the response from token counting
type CountTokensResponse struct {
	TotalTokens int `json:"totalTokens"`
}

// File Types

// File represents a file
type File struct {
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	MimeType       string     `json:"mimeType"`
	SizeBytes      string     `json:"sizeBytes"`
	CreateTime     time.Time  `json:"createTime"`
	UpdateTime     time.Time  `json:"updateTime"`
	ExpirationTime *time.Time `json:"expirationTime,omitempty"`
	Sha256Hash     string     `json:"sha256Hash"`
	URI            string     `json:"uri"`
}

// UploadFileRequest represents a request for uploading a file
type UploadFileRequest struct {
	File     File   `json:"file"`
	FileData []byte `json:"fileData"`
}

// UploadFileResponse represents the response from file upload
type UploadFileResponse struct {
	File File `json:"file"`
}

// ListFilesResponse represents the response from listing files
type ListFilesResponse struct {
	Files []File `json:"files"`
}

// DeleteFileResponse represents the response from file deletion
type DeleteFileResponse struct {
	// Empty response
}

// Error Types

// Error represents an API error
type Error struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Status  string            `json:"status"`
	Details []json.RawMessage `json:"details,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error Error `json:"error"`
}

// Helper functions for creating pointers
func StringPtr(s string) *string {
	return &s
}

func IntPtr(i int) *int {
	return &i
}

func Float64Ptr(f float64) *float64 {
	return &f
}

func BoolPtr(b bool) *bool {
	return &b
}
