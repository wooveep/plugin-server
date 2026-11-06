package main

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/tokenestimate"
)

// managedDashScopeStream observes original events while Router forwards the
// bytes. Only the last provider result can supply cumulative usage.
type managedDashScopeStream struct {
	protocol      string
	decoder       managedSSEDecoder
	last          []byte
	framingError  bool
	providerError bool
	thinking      bool
	incremental   bool
	text          strings.Builder
	textLimited   bool
}

func (s *managedDashScopeStream) push(chunk []byte) {
	if s.framingError {
		return
	}
	s.framingError = s.decoder.push(chunk, func(event managedProtocolEvent) {
		if bytes.Equal(event.Data, []byte("[DONE]")) {
			return
		}
		s.last = event.Data
		s.providerError = s.providerError || event.Name == "error"
		s.thinking = s.thinking || managedQwenReasoning(event.Data)
		if s.textLimited || s.providerError {
			return
		}
		text := tokenestimate.ResponseText(event.Data)
		if text == "" {
			return
		}
		if !s.incremental {
			s.text.Reset()
		}
		if s.text.Len()+len(text) > int(defaultMaxBodyBytes) {
			s.textLimited = true
			s.text.Reset()
			return
		}
		s.text.WriteString(text)
	}) != nil
}

func (s *managedDashScopeStream) observation(status uint16, headers [][2]string) managedProtocolObservation {
	if s.framingError || s.decoder.finish() != nil || len(s.last) == 0 {
		return managedProtocolStreamUnknown("qwen_stream_terminal_not_observed")
	}
	observed := managedQwenDocumentObservation(s.protocol, status, headers, s.last, true, s.providerError, 0)
	if observed.ExecutionResult == "unknown" {
		return managedProtocolStreamUnknown("qwen_stream_terminal_not_observed")
	}
	return observed
}

func (s *managedDashScopeStream) estimateBody() []byte {
	body, _ := json.Marshal(struct {
		Text string `json:"output_text"`
	}{Text: s.text.String()})
	return body
}
