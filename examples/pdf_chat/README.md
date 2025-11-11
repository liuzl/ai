# PDF Document Analysis Example

This example demonstrates how to analyze PDF documents with AI models. PDF support is available with **Gemini** and **Anthropic** models.

## Supported Providers

- ✅ **Gemini** - Native PDF support
- ✅ **Anthropic** - PDF support (Claude 3.5 Sonnet and later)
- ❌ OpenAI - Not natively supported (requires text extraction first)

## Prerequisites

### For Gemini

```bash
export AI_PROVIDER=gemini
export GEMINI_API_KEY="your-gemini-api-key"
export GEMINI_MODEL="gemini-2.0-flash-exp"  # or gemini-1.5-pro, gemini-1.5-flash
```

### For Anthropic

```bash
export AI_PROVIDER=anthropic
export ANTHROPIC_API_KEY="your-anthropic-api-key"
export ANTHROPIC_MODEL="claude-3-5-sonnet-20241022"
```

## Running the Example

```bash
go run main.go
```

## Use Cases

1. **Document Summarization**: Quickly summarize long PDF documents
2. **Question Answering**: Ask specific questions about PDF content
3. **Information Extraction**: Extract specific data, tables, figures
4. **Research Analysis**: Analyze research papers, find key findings
5. **Contract Review**: Review legal documents and contracts
6. **Report Analysis**: Analyze business reports, financial statements
7. **Educational**: Answer questions about textbooks, study materials
8. **Technical Documentation**: Understand technical manuals and specifications
9. **Multi-Document Analysis**: Compare multiple PDF documents
10. **Citation Extraction**: Extract references and citations

## Example Output

```
=== PDF Document Analysis Example ===

Analyzing PDF document...
PDF URL: https://arxiv.org/pdf/1706.03762.pdf
(This may take a moment as the PDF is being processed...)

AI Response:
1. **Title and Authors**:
   - Title: "Attention Is All You Need"
   - Authors: Ashish Vaswani, Noam Shazeer, Niki Parmar, Jakob Uszkoreit, Llion Jones,
     Aidan N. Gomez, Łukasz Kaiser, Illia Polosukhin (Google Brain and Google Research)

2. **Main Contribution Summary**:
   This paper introduces the Transformer, a novel neural network architecture for
   sequence transduction tasks that relies entirely on attention mechanisms, dispensing
   with recurrence and convolutions. The model achieves state-of-the-art results on
   machine translation tasks while being more parallelizable and requiring significantly
   less time to train.

3. **Key Innovation**:
   The key innovation is the self-attention mechanism that allows the model to process
   all positions in a sequence simultaneously, unlike RNNs which process sequentially.
   This parallel processing enables:
   - Faster training
   - Better handling of long-range dependencies
   - More efficient use of computational resources

=== Question Answering ===
Detailed Explanation:

1. **What is the Transformer architecture?**
   The Transformer is an encoder-decoder architecture that uses stacked self-attention
   and point-wise fully connected layers. It processes input sequences in parallel
   rather than sequentially, using multi-head attention mechanisms to model dependencies.

2. **Advantages over RNNs**:
   - **Parallelization**: Can process entire sequences simultaneously
   - **Long-range dependencies**: Direct connections between all positions
   - **Training speed**: Much faster to train
   - **Computational efficiency**: Better use of modern hardware (GPUs/TPUs)
   - **No vanishing gradients**: Shorter paths for gradient flow

3. **Key Components**:
   - **Multi-Head Attention**: Allows model to attend to different positions
   - **Positional Encoding**: Injects sequence order information
   - **Feed-Forward Networks**: Applied to each position independently
   - **Layer Normalization**: Stabilizes training
   - **Residual Connections**: Helps with gradient flow

=== Information Extraction ===
Extracted Information:

1. **BLEU Scores**:
   - English-to-German: 28.4 BLEU (new state-of-the-art)
   - English-to-French: 41.0 BLEU

2. **Training Details**:
   - Dataset: WMT 2014 (4.5M sentence pairs for EN-DE, 36M for EN-FR)
   - Training time: 3.5 days on 8 P100 GPUs for the base model
   - 12 hours for the big model on 8 P100 GPUs

3. **Model Parameters**:
   - Base model: ~65 million parameters
   - Big model: ~213 million parameters
```

## Code Example with Base64

For local PDF files:

```go
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/liuzl/ai"
)

func main() {
	// Read local PDF file
	pdfData, err := os.ReadFile("path/to/document.pdf")
	if err != nil {
		log.Fatal(err)
	}

	// Convert to base64
	base64PDF := base64.StdEncoding.EncodeToString(pdfData)

	// Create client
	client, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	// Use base64 PDF
	req := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Summarize this document"),
				ai.NewPDFPartFromBase64(base64PDF),
			}),
		},
	}

	resp, err := client.Generate(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Text)
}
```

## Multi-Document Analysis

Compare multiple PDFs:

```go
req := &ai.Request{
	Messages: []ai.Message{
		ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
			ai.NewTextPart("Compare these two research papers and highlight the key differences in their approaches:"),
			ai.NewPDFPartFromURL("paper1.pdf"),
			ai.NewPDFPartFromURL("paper2.pdf"),
		}),
	},
}
```

## Conversational Document Q&A

Have a conversation about a document:

```go
req := &ai.Request{
	Messages: []ai.Message{
		// First question
		ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
			ai.NewTextPart("What is the main topic of this document?"),
			ai.NewPDFPartFromURL("document.pdf"),
		}),

		// AI response (from previous call)
		ai.NewTextMessage(ai.RoleAssistant, previousResponse),

		// Follow-up question (document doesn't need to be sent again)
		ai.NewTextMessage(ai.RoleUser, "Can you elaborate on section 3?"),
	},
}
```

## Notes

- The example uses a famous AI research paper (Attention Is All You Need)
- PDF files are automatically downloaded and converted for the API
- Large PDFs may take longer to process and consume more tokens
- Some complex PDFs with unusual formatting may have parsing issues
- Consider the token limits of your chosen model when processing long documents

## Limitations

- Maximum PDF size varies by provider (check provider documentation)
- Scanned PDFs (images) may require OCR capabilities
- Complex layouts (multi-column, tables) may affect accuracy
- Some special characters or fonts may not be processed correctly
