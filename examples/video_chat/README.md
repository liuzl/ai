# Video Analysis Example

This example demonstrates how to use video input with AI models. Video analysis is primarily supported by **Google Gemini** models.

## Supported Providers

- ✅ **Gemini** - Full support for video analysis
- ❌ OpenAI - Not supported
- ❌ Anthropic - Not supported

## Supported Video Formats

- MP4
- MPEG
- MOV
- AVI
- FLV
- MPG
- WEBM
- WMV
- 3GPP

**Limitations**:
- Maximum duration: ~1 hour
- Maximum file size: 2GB

## Prerequisites

```bash
# Set environment variables
export AI_PROVIDER=gemini
export GEMINI_API_KEY="your-gemini-api-key"
export GEMINI_MODEL="gemini-2.0-flash-exp"  # or gemini-1.5-pro, gemini-1.5-flash
```

## Running the Example

```bash
go run main.go
```

## Use Cases

1. **Content Summarization**: Generate summaries of video content
2. **Scene Detection**: Identify and describe different scenes
3. **Object Recognition**: Detect objects, people, and activities
4. **Text Extraction**: Read text/captions displayed in videos
5. **Action Recognition**: Identify actions and events
6. **Quality Assessment**: Analyze video quality and production value
7. **Educational Content**: Answer questions about educational videos
8. **Sports Analysis**: Analyze sports footage and plays
9. **Security**: Monitor security camera footage
10. **Accessibility**: Generate video descriptions for accessibility

## Example Output

```
=== Video Analysis Example ===
This example requires Gemini API

Analyzing video...
Video URL: https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4
(This may take a moment as the video is being processed...)

AI Response:
This is an animated short film featuring a large, friendly white rabbit as the main character (Big Buck Bunny).

The story unfolds in a peaceful forest setting where the bunny is enjoying nature. Three smaller, mischievous creatures (possibly rodents) appear and start causing trouble, harassing the bunny and destroying things he cares about.

The video has beautiful 3D animation with vibrant colors. The setting is a lush green forest with flowers, trees, and natural scenery. The main character is a large, rotund white rabbit with a gentle demeanor initially, though he becomes assertive when defending himself.

The animation style is professional and polished, with expressive character animations and detailed environmental design.

=== Scene-by-Scene Analysis ===
Detailed Analysis:

1. **Timeline of Key Scenes**:
   - Opening: Big Buck Bunny waking up peacefully in the forest
   - Act 1: Bunny interacting gently with butterflies and other creatures
   - Conflict: Three small creatures harassing the bunny
   - Climax: Bunny defending himself against the troublemakers
   - Resolution: Peace returns to the forest

2. **Main Character Description**:
   - Large white rabbit with fluffy fur
   - Gentle and kind natured
   - Initially passive but becomes protective when provoked
   - Expressive face showing various emotions

3. **Mood and Tone**:
   - Overall tone: Lighthearted with moments of conflict
   - Beautiful, serene natural setting
   - Mix of comedy and action
   - Family-friendly content

4. **Text/Captions**:
   - Title appears: "Big Buck Bunny"
   - Opening credits visible at the start
   - Possibly copyright/license information
```

## Code Example with Base64

For local video files:

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
	// Read local video file
	videoData, err := os.ReadFile("path/to/video.mp4")
	if err != nil {
		log.Fatal(err)
	}

	// Convert to base64
	base64Video := base64.StdEncoding.EncodeToString(videoData)

	// Create client
	client, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	// Use base64 video
	req := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("Summarize this video in 3 bullet points"),
				ai.NewVideoPartFromBase64(base64Video, "mp4"),
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

## Multi-Modal Combination

You can combine video with other modalities:

```go
req := &ai.Request{
	Messages: []ai.Message{
		ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
			ai.NewTextPart("Compare these two videos and describe the differences:"),
			ai.NewVideoPartFromURL("url1.mp4", "mp4"),
			ai.NewVideoPartFromURL("url2.mp4", "mp4"),
		}),
	},
}
```

## Notes

- The example uses Big Buck Bunny, an open-source animated film
- Video files are automatically downloaded and converted to base64 for Gemini
- Large videos will take longer to process and may consume more API quota
- Processing time depends on video length and complexity
- Consider using shorter clips for faster responses and lower costs
