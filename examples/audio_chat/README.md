
# Audio Analysis Example

This example demonstrates how to use audio input with AI models. Audio analysis is primarily supported by **Google Gemini** models.

## Supported Providers

- ✅ **Gemini** - Full support for audio analysis
- ❌ OpenAI - Not supported in chat completions API (use Whisper API instead)
- ❌ Anthropic - Not currently supported

## Supported Audio Formats

- MP3
- WAV
- AIFF
- AAC
- OGG
- FLAC

**Maximum duration**: ~9.5 hours

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

1. **Music Analysis**: Identify instruments, tempo, genre, mood
2. **Speech Transcription**: Convert speech to text
3. **Sound Classification**: Identify environmental sounds
4. **Audio Quality Assessment**: Analyze audio quality issues
5. **Language Detection**: Identify languages in multilingual audio
6. **Emotion Detection**: Analyze emotional tone in speech

## Example Output

```
=== Audio Analysis Example ===
This example requires Gemini API

Analyzing audio file...
Audio URL: https://www2.cs.uic.edu/~i101/SoundFiles/BabyElephantWalk60.wav

AI Response:
This is a playful and whimsical piece of music. I can hear:

Instruments:
- Trumpet or brass section playing the main melody
- Tuba providing the bass line
- Light percussion (possibly tambourine or bells)
- Possibly a xylophone or marimba

The mood is cheerful, lighthearted, and child-friendly. It has a bouncy, walking rhythm that sounds like it could be from a cartoon or children's program. The melody is repetitive and catchy, with a comedic quality.

=== Detailed Audio Analysis ===
Detailed Analysis:
1. **Instruments**:
   - Trumpet/Brass: Carries the main melodic theme
   - Tuba: Provides a bouncing bass line
   - Percussion: Light, possibly wooden blocks or claves
   - High-pitched melodic instrument: Possibly glockenspiel

2. **Tempo and Rhythm**:
   - Moderate tempo, approximately 120-130 BPM
   - Bouncy, march-like rhythm
   - Strong emphasis on the downbeat
   - Syncopated melody creates a "walking" feeling

3. **Emotions**:
   - Playful and amusing
   - Lighthearted and carefree
   - Nostalgic (reminiscent of classic cartoons)
   - Whimsical and slightly silly

4. **Duration**: Approximately 60 seconds
```

## Code Example with Base64

If you have a local audio file, you can convert it to base64:

```go
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/liuzl/ai"
)

func main() {
	// Read local audio file
	audioData, err := os.ReadFile("path/to/audio.mp3")
	if err != nil {
		log.Fatal(err)
	}

	// Convert to base64
	base64Audio := base64.StdEncoding.EncodeToString(audioData)

	// Create client
	client, err := ai.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	// Use base64 audio
	req := &ai.Request{
		Messages: []ai.Message{
			ai.NewMultimodalMessage(ai.RoleUser, []ai.ContentPart{
				ai.NewTextPart("What's in this audio?"),
				ai.NewAudioPartFromBase64(base64Audio, "mp3"),
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

## Notes

- The example uses a public domain audio file for demonstration
- Audio files are automatically downloaded and converted to base64 for Gemini
- Large audio files may take longer to process
- Ensure your API key has sufficient quota for audio processing
