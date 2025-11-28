package ai

import (
	"bufio"
	"bytes"
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

// jsonArrayDecoder decodes streaming JSON array format: [{obj1},{obj2},{obj3}]
// Used by Gemini API which returns comma-separated JSON objects in an array.
type jsonArrayDecoder struct {
	reader    *bufio.Reader
	firstRead bool
	finished  bool
}

func newJSONArrayDecoder(r io.Reader) *jsonArrayDecoder {
	return &jsonArrayDecoder{
		reader:    bufio.NewReader(r),
		firstRead: true,
	}
}

// Next returns the next JSON object as an sseEvent.
func (d *jsonArrayDecoder) Next() (*sseEvent, error) {
	if d.finished {
		return nil, io.EOF
	}

	// Skip initial '[' and whitespace
	if d.firstRead {
		d.firstRead = false
		if err := d.skipUntil('['); err != nil {
			return nil, err
		}
	}

	// Skip whitespace and check for array end or comma
	for {
		b, err := d.reader.ReadByte()
		if err != nil {
			return nil, err
		}

		// Skip whitespace
		if b == ' ' || b == '\n' || b == '\r' || b == '\t' {
			continue
		}

		// End of array
		if b == ']' {
			d.finished = true
			return nil, io.EOF
		}

		// Skip comma between objects
		if b == ',' {
			continue
		}

		// Start of JSON object
		if b == '{' {
			// Unread the '{'
			if err := d.reader.UnreadByte(); err != nil {
				return nil, err
			}
			break
		}

		// Unexpected character
		return nil, io.ErrUnexpectedEOF
	}

	// Read complete JSON object
	objBytes, err := d.readJSONObject()
	if err != nil {
		return nil, err
	}

	return &sseEvent{
		Event: "",
		Data:  objBytes,
	}, nil
}

// skipUntil reads bytes until the target byte is found
func (d *jsonArrayDecoder) skipUntil(target byte) error {
	for {
		b, err := d.reader.ReadByte()
		if err != nil {
			return err
		}
		if b == target {
			return nil
		}
	}
}

// readJSONObject reads a complete JSON object from the stream
func (d *jsonArrayDecoder) readJSONObject() ([]byte, error) {
	var buf bytes.Buffer
	depth := 0
	inString := false
	escaped := false

	for {
		b, err := d.reader.ReadByte()
		if err != nil {
			return nil, err
		}

		buf.WriteByte(b)

		if escaped {
			escaped = false
			continue
		}

		if b == '\\' {
			escaped = true
			continue
		}

		if b == '"' {
			inString = !inString
			continue
		}

		if !inString {
			switch b {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					// Complete object
					return buf.Bytes(), nil
				}
			}
		}
	}
}
