package ai

import (
	"bufio"
	"io"
	"strings"
)

// sseEvent represents a single Server-Sent Event message.
type sseEvent struct {
	Event string
	Data  []byte
}

// sseDecoder provides minimal SSE parsing suitable for provider streaming APIs.
type sseDecoder struct {
	r *bufio.Reader
}

func newSSEDecoder(r io.Reader) *sseDecoder {
	return &sseDecoder{r: bufio.NewReader(r)}
}

// Next returns the next SSE event or io.EOF when the stream ends.
func (d *sseDecoder) Next() (*sseEvent, error) {
	var (
		eventName string
		dataLines []string
	)

	for {
		line, err := d.r.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}

		line = strings.TrimRight(line, "\r\n")

		switch {
		case strings.HasPrefix(line, ":"):
			// Comment line - ignore.
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(line[len("data:"):]))
		case line == "":
			// Blank line denotes end of event block.
			if len(dataLines) > 0 || eventName != "" {
				return &sseEvent{
					Event: eventName,
					Data:  []byte(strings.Join(dataLines, "\n")),
				}, nil
			}
		default:
			// Non-prefixed line is treated as data per SSE spec.
			dataLines = append(dataLines, line)
		}

		if err == io.EOF {
			if len(dataLines) > 0 || eventName != "" {
				return &sseEvent{
					Event: eventName,
					Data:  []byte(strings.Join(dataLines, "\n")),
				}, nil
			}
			return nil, io.EOF
		}
	}
}
