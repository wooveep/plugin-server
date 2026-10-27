package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"mime"
	"strings"
	"unicode/utf8"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	managedProtocolChat      = "openai_chat"
	managedProtocolResponses = "openai_responses"
	managedProtocolMessages  = "anthropic_messages"
)

// Profiles are server configuration. Neither the URL nor a response's model
// selects an adapter or changes the already frozen sale/account identity.
type managedProtocolProfile struct {
	InputProtocol    string
	UpstreamProtocol string
}

type managedProtocolRequest struct {
	Body      []byte
	Headers   [][2]string
	Streaming bool
}

type managedProtocolObservation struct {
	Usage           *executionfacts.Usage
	ExecutionResult string
	Acceptance      string
	BodyComplete    bool
	ProviderError   bool
	ResponseModel   string
}

// Deliberately no status field: conversion cannot rewrite upstream HTTP facts.
type managedProtocolClientResponse struct {
	Body    []byte
	Headers [][2]string
}

func newManagedProtocolProfile(input, upstream string) (managedProtocolProfile, error) {
	if input == "" {
		input = managedProtocolChat
	}
	if upstream == "" {
		upstream = managedProtocolChat
	}
	valid := func(value string) bool {
		return value == managedProtocolChat || value == managedProtocolResponses || value == managedProtocolMessages
	}
	if !valid(input) || !valid(upstream) || input != upstream && upstream != managedProtocolChat {
		return managedProtocolProfile{}, errors.New("unsupported managed protocol profile")
	}
	return managedProtocolProfile{InputProtocol: input, UpstreamProtocol: upstream}, nil
}

// PrepareRequest accepts the mapper's output, not the original public body.
// The caller must freeze the returned actual bytes as outbound_body_digest.
func (p managedProtocolProfile) PrepareRequest(body []byte, frozenModel, credential string) (managedProtocolRequest, error) {
	var object map[string]json.RawMessage
	if err := decodeManagedJSON(body, &object, false); err != nil || object == nil {
		return managedProtocolRequest{}, errors.New("invalid managed protocol request")
	}
	var model string
	if json.Unmarshal(object["model"], &model) != nil || model != frozenModel || !managedID(model) {
		return managedProtocolRequest{}, errors.New("managed protocol model differs from frozen mapping")
	}
	streaming := false
	if raw, exists := object["stream"]; exists {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &streaming) != nil {
			return managedProtocolRequest{}, errors.New("invalid managed protocol stream flag")
		}
	}
	if credential == "" {
		return managedProtocolRequest{}, errors.New("managed upstream credential missing")
	}
	for _, char := range credential {
		if char <= 0x20 || char > 0x7e {
			return managedProtocolRequest{}, errors.New("invalid managed upstream credential")
		}
	}
	actual := append([]byte(nil), body...)
	var err error
	if p.InputProtocol != p.UpstreamProtocol {
		switch p.InputProtocol {
		case managedProtocolResponses:
			actual, err = provider.ConvertResponsesRequestToChatCompletion(body)
		case managedProtocolMessages:
			actual, err = (&provider.ClaudeToOpenAIConverter{}).ConvertClaudeRequestToOpenAI(body)
		default:
			err = errors.New("unsupported managed protocol request conversion")
		}
	}
	if err != nil {
		// Provider errors can contain body details; expose a fixed local reason.
		return managedProtocolRequest{}, errors.New("managed protocol request conversion failed")
	}
	var converted map[string]json.RawMessage
	var convertedModel string
	if decodeManagedJSON(actual, &converted, false) != nil || json.Unmarshal(converted["model"], &convertedModel) != nil || convertedModel != frozenModel {
		return managedProtocolRequest{}, errors.New("managed protocol conversion changed frozen model")
	}
	headers := [][2]string{{"content-type", "application/json"}, {"accept", "application/json, text/event-stream"}, {"accept-encoding", "identity"}}
	if p.UpstreamProtocol == managedProtocolMessages {
		headers = append(headers, [2]string{"x-api-key", credential}, [2]string{"anthropic-version", "2023-06-01"})
	} else {
		headers = append(headers, [2]string{"authorization", "Bearer " + credential})
	}
	return managedProtocolRequest{Body: actual, Headers: headers, Streaming: streaming}, nil
}

// ObserveResponse only inspects the original upstream bytes. In particular it
// never measures usage from a converter's synthetic events or default zeros.
func (p managedProtocolProfile) ObserveResponse(status uint16, headers [][2]string, body []byte, complete bool) managedProtocolObservation {
	observation := managedProtocolObservation{
		Usage:           executionfacts.MissingTokenUsage("provider_usage_not_observed"),
		ExecutionResult: "unknown", Acceptance: "unknown", BodyComplete: complete,
	}
	if status >= 400 && status <= 599 {
		observation.ExecutionResult = "failed"
	}
	if !complete || status < 100 || status > 599 {
		return observation
	}
	stream, err := managedProtocolContentType(headers)
	if err != nil {
		return observation
	}
	if stream {
		switch p.UpstreamProtocol {
		case managedProtocolChat:
			observation.Usage, observation.ExecutionResult, observation.ProviderError = managedSSEObservation(body)
			if events, err := managedProtocolEvents(body); err == nil {
				var prior *executionfacts.Usage
				usageConflict := false
				for _, event := range events {
					if bytes.Equal(event.Data, []byte("[DONE]")) {
						continue
					}
					var object map[string]json.RawMessage
					if decodeManagedJSON(event.Data, &object, false) != nil {
						continue
					}
					if !managedProtocolMergeModel(&observation.ResponseModel, object["model"]) {
						observation.ExecutionResult = "unknown"
						observation.Usage = executionfacts.MissingTokenUsage("stream_model_conflict")
						break
					}
					if raw, exists := object["usage"]; exists && !managedProtocolNull(raw) {
						current := managedOpenAIUsage(object)
						if prior != nil && !managedProtocolSameUsage(prior, current) {
							observation.Usage = executionfacts.MissingTokenUsage("stream_usage_conflict")
							usageConflict = true
							break
						}
						prior = current
					}
				}
				if !usageConflict && observation.ExecutionResult == "failed" && observation.ProviderError && prior != nil && prior.Status == "reported" {
					// The chat parser only proves failure after all choices and DONE.
					// Complete explicit usage remains actual provider cost on errors.
					observation.Usage = prior
				}
			}
		case managedProtocolResponses:
			observation = managedResponsesStream(body)
		case managedProtocolMessages:
			observation = managedMessagesStream(body)
		}
		if observation.ExecutionResult == "unknown" {
			observation.BodyComplete = false
		}
	} else {
		var object map[string]json.RawMessage
		if decodeManagedJSON(body, &object, false) != nil || object == nil {
			return observation
		}
		managedProtocolMergeModel(&observation.ResponseModel, object["model"])
		switch p.UpstreamProtocol {
		case managedProtocolChat:
			observation.Usage = managedOpenAIUsage(object)
			observation.ExecutionResult = managedChatTerminal(object)
		case managedProtocolResponses:
			observation.Usage = managedResponsesUsage(object["usage"])
			observation.ExecutionResult = managedResponsesTerminal(object)
		case managedProtocolMessages:
			observation.Usage = managedMessagesUsage(object["usage"])
			observation.ExecutionResult = managedMessagesTerminal(object)
		}
		if raw, exists := object["error"]; exists && !managedProtocolNull(raw) {
			observation.ProviderError = true
			observation.ExecutionResult = "failed"
		}
	}
	if status >= 400 {
		observation.ExecutionResult = "failed"
	}
	if status >= 200 && status < 300 && (observation.ExecutionResult == "succeeded" || observation.ExecutionResult == "partial") {
		observation.Acceptance = "accepted"
	} else if status < 200 || status >= 300 {
		observation.Acceptance = "unknown"
		if status < 400 {
			observation.ExecutionResult = "unknown"
		}
	}
	return observation
}

func managedProtocolContentType(headers [][2]string) (bool, error) {
	for _, header := range headers {
		if strings.EqualFold(header[0], "content-encoding") && !strings.EqualFold(strings.TrimSpace(header[1]), "identity") {
			return false, errors.New("encoded managed upstream body")
		}
	}
	value, err := managedSingleHeader(headers, "content-type")
	if err != nil {
		return false, err
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(mediaType) {
	case "text/event-stream":
		return true, nil
	case "application/json":
		return false, nil
	default:
		return false, errors.New("unsupported managed upstream content type")
	}
}

func managedProtocolNull(raw json.RawMessage) bool {
	return len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func managedProtocolMergeModel(prior *string, raw json.RawMessage) bool {
	if managedProtocolNull(raw) {
		return true
	}
	var model string
	if json.Unmarshal(raw, &model) != nil || !managedID(model) || *prior != "" && *prior != model {
		return false
	}
	*prior = model
	return true
}

func managedProtocolSameUsage(a, b *executionfacts.Usage) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return bytes.Equal(left, right)
}

func managedChatTerminal(object map[string]json.RawMessage) string {
	var choices []map[string]json.RawMessage
	if json.Unmarshal(object["choices"], &choices) != nil || len(choices) == 0 {
		return "unknown"
	}
	result := "succeeded"
	for _, choice := range choices {
		var reason string
		if json.Unmarshal(choice["finish_reason"], &reason) != nil {
			return "unknown"
		}
		switch reason {
		case "stop", "tool_calls", "function_call":
		case "length", "content_filter":
			result = "partial"
		default:
			return "unknown"
		}
	}
	return result
}

func managedResponsesTerminal(object map[string]json.RawMessage) string {
	var status string
	if json.Unmarshal(object["status"], &status) != nil {
		return "unknown"
	}
	switch status {
	case "completed":
		return "succeeded"
	case "incomplete":
		return "partial"
	case "failed", "cancelled":
		return "failed"
	default:
		return "unknown"
	}
}

func managedMessagesTerminal(object map[string]json.RawMessage) string {
	var reason string
	if json.Unmarshal(object["stop_reason"], &reason) != nil {
		return "unknown"
	}
	switch reason {
	case "end_turn", "tool_use", "stop_sequence", "refusal":
		return "succeeded"
	case "max_tokens", "pause_turn":
		return "partial"
	default:
		return "unknown"
	}
}

func managedResponsesUsage(raw json.RawMessage) *executionfacts.Usage {
	var usage map[string]json.RawMessage
	if json.Unmarshal(raw, &usage) != nil || usage == nil {
		return executionfacts.MissingTokenUsage("provider_usage_not_observed")
	}
	// Reuse the chat exact-integer/cache-split validator with explicit field
	// names only. The original input is never decoded through a float or int.
	chat := map[string]json.RawMessage{"prompt_tokens": usage["input_tokens"], "completion_tokens": usage["output_tokens"]}
	if total, exists := usage["total_tokens"]; exists {
		chat["total_tokens"] = total
	}
	if details, exists := usage["input_tokens_details"]; exists {
		chat["prompt_tokens_details"] = details
	}
	encoded, _ := json.Marshal(chat)
	return managedOpenAIUsage(map[string]json.RawMessage{"usage": encoded})
}

func managedMessagesUsage(raw json.RawMessage) *executionfacts.Usage {
	missing := func() *executionfacts.Usage {
		return executionfacts.MissingTokenUsage("provider_usage_incomplete_or_invalid")
	}
	var usage map[string]json.RawMessage
	if json.Unmarshal(raw, &usage) != nil || usage == nil {
		return executionfacts.MissingTokenUsage("provider_usage_not_observed")
	}
	input, inOK := managedCount(usage["input_tokens"])
	output, outOK := managedCount(usage["output_tokens"])
	if !inOK || !outOK {
		return missing()
	}
	quantities := map[string]string{"input": input.String(), "output": output.String()}
	readRaw, hasRead := usage["cache_read_input_tokens"]
	writeRaw, hasWrite := usage["cache_creation_input_tokens"]
	if hasRead || hasWrite {
		read, readOK := managedCount(readRaw)
		write, writeOK := managedCount(writeRaw)
		if !hasRead || !hasWrite || !readOK || !writeOK {
			return missing()
		}
		miss := new(big.Int).Add(input, write)
		total := new(big.Int).Add(new(big.Int).Set(miss), read)
		if total.BitLen() > 128 {
			return missing()
		}
		quantities["input"], quantities["cache_hit"], quantities["cache_miss"] = total.String(), read.String(), miss.String()
	}
	return &executionfacts.Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: quantities}
}

type managedProtocolEvent struct {
	Name string
	Data []byte
}

// HTTP callouts deliver a single bounded buffer. Require complete SSE event
// delimiters; this is not an incremental parser or a streaming transport.
func managedProtocolEvents(body []byte) ([]managedProtocolEvent, error) {
	if len(body) == 0 || len(body) > int(defaultMaxBodyBytes) || !utf8.Valid(body) {
		return nil, errors.New("invalid managed SSE body")
	}
	body = bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf})
	var events []managedProtocolEvent
	var data []byte
	var name string
	hasData := false
	for start := 0; start < len(body); {
		end := start
		for end < len(body) && body[end] != '\r' && body[end] != '\n' {
			end++
		}
		if end == len(body) {
			return nil, errors.New("unterminated managed SSE line")
		}
		line := body[start:end]
		start = end + 1
		if body[end] == '\r' && start < len(body) && body[start] == '\n' {
			start++
		}
		if len(line) == 0 {
			if hasData {
				if len(events) >= 65536 {
					return nil, errors.New("managed SSE event budget exceeded")
				}
				events = append(events, managedProtocolEvent{Name: name, Data: bytes.Clone(data)})
			}
			data, name, hasData = nil, "", false
			continue
		}
		if line[0] == ':' {
			continue
		}
		field, value, _ := bytes.Cut(line, []byte(":"))
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}
		switch string(field) {
		case "event":
			name = string(value)
		case "data":
			if len(data)+len(value)+1 > executionfacts.MaxEnvelopeBytes {
				return nil, errors.New("managed SSE payload budget exceeded")
			}
			if hasData {
				data = append(data, '\n')
			}
			data = append(data, value...)
			hasData = true
		}
	}
	if hasData || name != "" {
		return nil, errors.New("unterminated managed SSE event")
	}
	return events, nil
}

func managedProtocolStreamUnknown(reason string) managedProtocolObservation {
	return managedProtocolObservation{Usage: executionfacts.MissingTokenUsage(reason), ExecutionResult: "unknown", Acceptance: "unknown"}
}

func managedProtocolEventObject(event managedProtocolEvent) (map[string]json.RawMessage, string, error) {
	var object map[string]json.RawMessage
	var kind string
	if decodeManagedJSON(event.Data, &object, false) != nil || object == nil || json.Unmarshal(object["type"], &kind) != nil || kind == "" || event.Name != "" && event.Name != kind {
		return nil, "", errors.New("invalid managed protocol event")
	}
	return object, kind, nil
}

func managedResponsesStream(body []byte) managedProtocolObservation {
	unknown := managedProtocolStreamUnknown("stream_terminal_not_observed")
	events, err := managedProtocolEvents(body)
	if err != nil {
		return unknown
	}
	observation := unknown
	terminal := false
	for _, event := range events {
		if bytes.Equal(event.Data, []byte("[DONE]")) && terminal {
			continue
		}
		object, kind, err := managedProtocolEventObject(event)
		if err != nil || terminal {
			return unknown
		}
		if kind == "error" {
			observation.ProviderError, observation.ExecutionResult, observation.BodyComplete = true, "failed", true
			observation.Usage = executionfacts.MissingTokenUsage("stream_provider_error")
			terminal = true
			continue
		}
		var response map[string]json.RawMessage
		if raw, exists := object["response"]; exists {
			if json.Unmarshal(raw, &response) != nil || response == nil || !managedProtocolMergeModel(&observation.ResponseModel, response["model"]) {
				return unknown
			}
		}
		switch kind {
		case "response.completed", "response.incomplete", "response.failed":
			result := managedResponsesTerminal(response)
			expected := map[string]string{"response.completed": "succeeded", "response.incomplete": "partial", "response.failed": "failed"}[kind]
			if result != expected {
				return unknown
			}
			observation.Usage = managedResponsesUsage(response["usage"])
			observation.ExecutionResult, observation.BodyComplete = result, true
			if result == "failed" || !managedProtocolNull(response["error"]) {
				observation.ProviderError, observation.ExecutionResult = true, "failed"
			}
			terminal = true
		}
	}
	if !terminal {
		return unknown
	}
	return observation
}

func managedMessagesStream(body []byte) managedProtocolObservation {
	unknown := managedProtocolStreamUnknown("stream_terminal_not_observed")
	events, err := managedProtocolEvents(body)
	if err != nil {
		return unknown
	}
	observation := unknown
	usage := map[string]json.RawMessage{}
	started, stopped, reasonSeen, finalOutput := false, false, false, false
	result := "unknown"
	for _, event := range events {
		object, kind, err := managedProtocolEventObject(event)
		if err != nil || stopped {
			return unknown
		}
		switch kind {
		case "ping":
			continue
		case "error":
			observation.ExecutionResult, observation.ProviderError, observation.BodyComplete = "failed", true, true
			observation.Usage = executionfacts.MissingTokenUsage("stream_provider_error")
			return observation
		case "message_start":
			var message map[string]json.RawMessage
			if started || json.Unmarshal(object["message"], &message) != nil || message == nil || !managedProtocolMergeModel(&observation.ResponseModel, message["model"]) || json.Unmarshal(message["usage"], &usage) != nil || usage == nil {
				return unknown
			}
			started = true
		case "message_delta":
			if !started {
				return unknown
			}
			var delta, update map[string]json.RawMessage
			if json.Unmarshal(object["delta"], &delta) != nil || delta == nil {
				return unknown
			}
			if raw, exists := delta["stop_reason"]; exists && !managedProtocolNull(raw) {
				next := managedMessagesTerminal(map[string]json.RawMessage{"stop_reason": raw})
				if reasonSeen || next == "unknown" {
					return unknown
				}
				reasonSeen, result = true, next
			}
			if raw, exists := object["usage"]; exists {
				if json.Unmarshal(raw, &update) != nil || update == nil {
					return unknown
				}
				for key, value := range update {
					if key == "output_tokens" {
						next, valid := managedCount(value)
						if !valid {
							return unknown
						}
						if previous, valid := managedCount(usage[key]); valid && next.Cmp(previous) < 0 {
							return unknown
						}
						// Earlier progress is not proof of the final cumulative count.
						finalOutput = reasonSeen
					} else if prior, exists := usage[key]; exists && !bytes.Equal(prior, value) {
						return unknown
					}
					usage[key] = value
				}
			}
		case "message_stop":
			if !started || !reasonSeen {
				return unknown
			}
			stopped = true
		case "content_block_start", "content_block_delta", "content_block_stop":
			if !started || reasonSeen {
				return unknown
			}
		default:
			return unknown
		}
	}
	if !stopped {
		return unknown
	}
	encoded, _ := json.Marshal(usage)
	observation.Usage = managedMessagesUsage(encoded)
	if !finalOutput {
		observation.Usage = executionfacts.MissingTokenUsage("stream_usage_not_observed")
	}
	observation.ExecutionResult, observation.BodyComplete = result, true
	return observation
}

// ConvertResponse is called only after ObserveResponse has been persisted in
// the original attempt. Converters are fresh per buffered response, never shared
// across calls. An actual HTTP error passes through with its real status owned
// by the caller; a truncated 2xx stream cannot be upgraded to a completed one.
func (p managedProtocolProfile) ConvertResponse(ctx wrapper.HttpContext, status uint16, headers [][2]string, body []byte) (managedProtocolClientResponse, error) {
	result := managedProtocolClientResponse{Body: bytes.Clone(body), Headers: managedResponseHeaders(headers)}
	if p.InputProtocol == p.UpstreamProtocol || status >= 400 && status <= 599 {
		return result, nil
	}
	observation := p.ObserveResponse(status, headers, body, true)
	if observation.Acceptance != "accepted" || !observation.BodyComplete || observation.ProviderError {
		return managedProtocolClientResponse{}, errors.New("managed fallback response terminal not verified")
	}
	stream, err := managedProtocolContentType(headers)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	// The existing converters have machine-int usage fields. Isolate usage from
	// those conversion structs and reattach validated original decimal integers.
	// This changes no content/tool/protocol algorithm in the provider converters.
	conversionBody, err := managedProtocolConversionInput(body, stream)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	if stream {
		if p.InputProtocol == managedProtocolResponses {
			result.Body, err = provider.NewChatCompletionToResponsesStreamConverter().Convert(conversionBody, true)
		} else {
			result.Body, err = (&provider.ClaudeToOpenAIConverter{}).ConvertOpenAIStreamResponseToClaude(ctx, conversionBody)
		}
	} else if p.InputProtocol == managedProtocolResponses {
		result.Body, err = provider.ConvertChatCompletionResponseToResponses(conversionBody)
	} else {
		result.Body, err = (&provider.ClaudeToOpenAIConverter{}).ConvertOpenAIResponseToClaude(ctx, conversionBody)
	}
	if err != nil {
		return managedProtocolClientResponse{}, errors.New("managed protocol response conversion failed")
	}
	result.Body, err = managedProtocolRestoreUsage(result.Body, stream, p.InputProtocol, observation.Usage)
	if err != nil {
		return managedProtocolClientResponse{}, err
	}
	result.Headers = nil
	for _, header := range managedResponseHeaders(headers) {
		switch strings.ToLower(header[0]) {
		case "content-type", "content-encoding", "etag", "content-md5", "digest", "content-digest", "repr-digest":
			continue
		}
		result.Headers = append(result.Headers, header)
	}
	contentType := "application/json"
	if stream {
		contentType = "text/event-stream"
	}
	result.Headers = append(result.Headers, [2]string{"content-type", contentType})
	return result, nil
}

func managedProtocolConversionInput(body []byte, stream bool) ([]byte, error) {
	convert := func(raw []byte) ([]byte, error) {
		var object map[string]json.RawMessage
		if decodeManagedJSON(raw, &object, false) != nil || object == nil {
			return nil, errors.New("invalid managed conversion input")
		}
		var choices []map[string]json.RawMessage
		if json.Unmarshal(object["choices"], &choices) != nil || len(choices) > 1 {
			return nil, errors.New("managed fallback requires a single response choice")
		}
		if !stream && len(choices) != 1 {
			return nil, errors.New("managed fallback response choice missing")
		}
		if !stream {
			var message map[string]json.RawMessage
			if json.Unmarshal(choices[0]["message"], &message) != nil || message == nil {
				return nil, errors.New("managed fallback assistant message missing")
			}
		}
		if raw, exists := object["usage"]; exists && !managedProtocolNull(raw) {
			// A bounded placeholder triggers the existing converter's usage event.
			// It is replaced before any converted bytes leave this function chain.
			object["usage"] = json.RawMessage(`{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}`)
		}
		return json.Marshal(object)
	}
	if !stream {
		return convert(body)
	}
	events, err := managedProtocolEvents(body)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	for _, event := range events {
		data := event.Data
		if !bytes.Equal(data, []byte("[DONE]")) {
			data, err = convert(data)
			if err != nil {
				return nil, err
			}
		}
		managedProtocolWriteEvent(&output, event.Name, data)
	}
	return output.Bytes(), nil
}

func managedProtocolWireUsage(protocol string, usage *executionfacts.Usage) json.RawMessage {
	if usage == nil || usage.Status != "reported" {
		return nil
	}
	quantities := usage.Quantities
	input, inputOK := managedInteger(quantities["input"])
	output, outputOK := managedInteger(quantities["output"])
	if !inputOK || !outputOK {
		return nil
	}
	wire := map[string]interface{}{"input_tokens": json.Number(input.String()), "output_tokens": json.Number(output.String())}
	if protocol == managedProtocolResponses {
		total := new(big.Int).Add(input, output)
		if total.BitLen() > 128 {
			return nil
		}
		wire["total_tokens"] = json.Number(total.String())
		if hit, exists := quantities["cache_hit"]; exists {
			wire["input_tokens_details"] = map[string]json.Number{"cached_tokens": json.Number(hit)}
		}
	} else if hit, exists := quantities["cache_hit"]; exists {
		wire["input_tokens"] = json.Number(quantities["cache_miss"])
		wire["cache_read_input_tokens"] = json.Number(hit)
		wire["cache_creation_input_tokens"] = json.Number("0")
	}
	encoded, _ := json.Marshal(wire)
	return encoded
}

func managedProtocolRestoreUsage(body []byte, stream bool, protocol string, usage *executionfacts.Usage) ([]byte, error) {
	wire := managedProtocolWireUsage(protocol, usage)
	if wire == nil && usage != nil && usage.Status == "reported" {
		return nil, errors.New("exact managed usage cannot be represented by client profile")
	}
	patch := func(raw []byte) ([]byte, error) {
		var object map[string]json.RawMessage
		if decodeManagedJSON(raw, &object, false) != nil || object == nil {
			return nil, errors.New("invalid converted managed response")
		}
		set := func(target map[string]json.RawMessage) {
			if wire == nil {
				delete(target, "usage")
			} else {
				target["usage"] = wire
			}
		}
		if !stream {
			set(object)
		} else {
			var kind string
			_ = json.Unmarshal(object["type"], &kind)
			nestedKey := ""
			if protocol == managedProtocolResponses && strings.HasPrefix(kind, "response.") {
				nestedKey = "response"
			} else if protocol == managedProtocolMessages && kind == "message_start" {
				nestedKey = "message"
			} else if protocol == managedProtocolMessages && kind == "message_delta" {
				set(object)
			}
			if nested, exists := object[nestedKey]; nestedKey != "" && exists {
				var target map[string]json.RawMessage
				if json.Unmarshal(nested, &target) != nil || target == nil {
					return nil, errors.New("invalid converted managed response object")
				}
				set(target)
				encoded, _ := json.Marshal(target)
				object[nestedKey] = encoded
			}
		}
		return json.Marshal(object)
	}
	if !stream {
		return patch(body)
	}
	events, err := managedProtocolEvents(body)
	if err != nil || len(events) == 0 {
		return nil, errors.New("managed converter emitted no complete events")
	}
	var output bytes.Buffer
	for _, event := range events {
		data, err := patch(event.Data)
		if err != nil {
			return nil, err
		}
		managedProtocolWriteEvent(&output, event.Name, data)
	}
	return output.Bytes(), nil
}

func managedProtocolWriteEvent(output *bytes.Buffer, name string, data []byte) {
	if name != "" {
		output.WriteString("event: ")
		output.WriteString(name)
		output.WriteByte('\n')
	}
	output.WriteString("data: ")
	output.Write(data)
	output.WriteString("\n\n")
}
