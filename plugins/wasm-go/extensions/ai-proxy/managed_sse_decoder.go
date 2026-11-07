package main

import (
	"bytes"
	"errors"
	"unicode/utf8"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

// managedSSEDecoder observes SSE framing across HTTP body chunks. Its caller
// owns forwarding. Partial lines and events are bounded independently of the
// total response size.
type managedSSEDecoder struct {
	line      []byte
	data      []byte
	name      string
	hasData   bool
	firstLine bool
	skipLF    bool
	onComment func()
}

func (d *managedSSEDecoder) push(chunk []byte, event func(managedProtocolEvent)) error {
	for len(chunk) != 0 {
		if d.skipLF {
			d.skipLF = false
			if chunk[0] == '\n' {
				chunk = chunk[1:]
				continue
			}
		}
		end := bytes.IndexAny(chunk, "\r\n")
		if end < 0 {
			if len(d.line)+len(chunk) > executionfacts.MaxEnvelopeBytes {
				return errors.New("managed SSE line budget exceeded")
			}
			d.line = append(d.line, chunk...)
			return nil
		}
		line := chunk[:end]
		d.skipLF = chunk[end] == '\r'
		chunk = chunk[end+1:]
		if len(d.line) != 0 {
			if len(d.line)+len(line) > executionfacts.MaxEnvelopeBytes {
				return errors.New("managed SSE line budget exceeded")
			}
			line = append(d.line, line...)
			d.line = nil
		}
		if !d.firstLine {
			d.firstLine = true
			line = bytes.TrimPrefix(line, []byte{0xef, 0xbb, 0xbf})
		}
		if !utf8.Valid(line) {
			return errors.New("invalid managed SSE line")
		}
		if len(line) == 0 {
			if d.hasData {
				event(managedProtocolEvent{Name: d.name, Data: d.data})
			}
			d.data, d.name, d.hasData = nil, "", false
			continue
		}
		// Comments carry liveness, not provider data. Notify as soon as a
		// complete comment line arrives, independently of a following event.
		if line[0] == ':' {
			if d.onComment != nil {
				d.onComment()
			}
			continue
		}
		field, value, _ := bytes.Cut(line, []byte(":"))
		if len(value) != 0 && value[0] == ' ' {
			value = value[1:]
		}
		switch string(field) {
		case "event":
			d.name = string(value)
		case "data":
			if len(d.data)+len(value)+1 > executionfacts.MaxEnvelopeBytes {
				return errors.New("managed SSE payload budget exceeded")
			}
			if d.hasData {
				d.data = append(d.data, '\n')
			}
			d.data = append(d.data, value...)
			d.hasData = true
		}
	}
	return nil
}

func (d *managedSSEDecoder) finish() error {
	if !d.firstLine || len(d.line) != 0 || d.hasData || d.name != "" {
		return errors.New("incomplete managed SSE framing")
	}
	return nil
}
