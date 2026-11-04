package clientresponse

import (
	"bytes"
	"errors"
	"strings"
	"unicode/utf8"
)

type Event struct {
	Name string
	Data []byte
}

// Events parses a complete bounded SSE response. Transport buffering is owned by
// the existing sender; this codec does not claim incremental upstream delivery.
func Events(body []byte) ([]Event, error) {
	if len(body) > 100*1024*1024 || !utf8.Valid(body) {
		return nil, errors.New("public response exceeds its body limit")
	}
	body = bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf})
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text != "" && !strings.HasSuffix(text, "\n\n") {
		return nil, errors.New("public SSE response has an unterminated event")
	}
	var events []Event
	for _, block := range strings.Split(text, "\n\n") {
		var e Event
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "event:") {
				e.Name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				if len(e.Data) > 0 {
					e.Data = append(e.Data, '\n')
				}
				e.Data = append(e.Data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")...)
			}
		}
		if len(e.Data) > 0 {
			events = append(events, e)
		}
	}
	return events, nil
}

func AppendEvent(output *bytes.Buffer, event Event) {
	if event.Name != "" {
		output.WriteString("event: ")
		output.WriteString(event.Name)
		output.WriteByte('\n')
	}
	output.WriteString("data: ")
	output.Write(event.Data)
	output.WriteString("\n\n")
}
