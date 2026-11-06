package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"mime"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/sjson"
)

const (
	managedProtocolChat                = "openai_chat"
	managedProtocolCompletions         = "openai_completions"
	managedProtocolResponses           = "openai_responses"
	managedProtocolMessages            = "anthropic_messages"
	managedProtocolDashScopeText       = "dashscope_text"
	managedProtocolDashScopeMultimodal = "dashscope_multimodal"
)

// Profiles are server configuration. Neither the URL nor a response's model
// selects an adapter or changes the already frozen sale/account identity.
type managedProtocolProfile struct {
	InputProtocol    string
	UpstreamProtocol string
	supplier         string
}

type managedProtocolRequest struct {
	UpstreamPath       string
	Body               []byte
	Headers            [][2]string
	Streaming          bool
	ExpectedEmbeddings int
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
	if input == upstream && (input == managedProtocolEmbeddings || input == managedProtocolCompletions) {
		return managedProtocolProfile{InputProtocol: input, UpstreamProtocol: upstream}, nil
	}
	if operation, _, _ := executionfacts.RetrievalProtocol(input); operation != "" && input == upstream {
		return managedProtocolProfile{InputProtocol: input, UpstreamProtocol: upstream}, nil
	}
	if input == upstream && (input == managedProtocolDashScopeText || input == managedProtocolDashScopeMultimodal) {
		return managedProtocolProfile{InputProtocol: input, UpstreamProtocol: upstream}, nil
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
	if p.InputProtocol == managedProtocolEmbeddings && p.UpstreamProtocol == managedProtocolEmbeddings {
		return prepareManagedEmbeddingRequest(body, frozenModel, credential)
	}
	if operation, _, _ := executionfacts.RetrievalProtocol(p.UpstreamProtocol); operation != "" {
		return prepareManagedRetrievalRequest(body, frozenModel, credential)
	}
	var object map[string]json.RawMessage
	if err := decodeManagedJSON(body, &object, false); err != nil || object == nil {
		return managedProtocolRequest{}, errors.New("invalid managed protocol request")
	}
	var model string
	if json.Unmarshal(object["model"], &model) != nil || model != frozenModel || !managedID(model) {
		return managedProtocolRequest{}, errors.New("managed protocol model differs from frozen mapping")
	}
	if p.InputProtocol == managedProtocolCompletions {
		var prompt string
		if managedProtocolNull(object["prompt"]) || json.Unmarshal(object["prompt"], &prompt) != nil {
			return managedProtocolRequest{}, errors.New("Completions requires a string prompt")
		}
	}
	if p.InputProtocol == managedProtocolDashScopeText || p.InputProtocol == managedProtocolDashScopeMultimodal {
		var input map[string]json.RawMessage
		var messages []json.RawMessage
		if json.Unmarshal(object["input"], &input) != nil || json.Unmarshal(input["messages"], &messages) != nil || len(messages) == 0 {
			return managedProtocolRequest{}, errors.New("DashScope text requires input.messages")
		}
		var parameters map[string]json.RawMessage
		if raw, exists := object["parameters"]; exists && (json.Unmarshal(raw, &parameters) != nil || parameters == nil) {
			return managedProtocolRequest{}, errors.New("invalid DashScope parameters")
		}
		// This registration authorizes text output only on a shared multimodal
		// endpoint. Other operations and output modalities need their own release.
		for _, field := range []string{"modalities", "output_modalities"} {
			if raw, exists := parameters[field]; exists {
				var modalities []string
				if json.Unmarshal(raw, &modalities) != nil || len(modalities) != 1 || modalities[0] != "text" {
					return managedProtocolRequest{}, errors.New("DashScope registration supports text output only")
				}
			}
		}
	}
	streaming := false
	if raw, exists := object["stream"]; exists {
		if json.Unmarshal(raw, &streaming) != nil || p.InputProtocol != managedProtocolCompletions && managedProtocolNull(raw) {
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
			actual, err = (&provider.ClaudeToOpenAIConverter{}).ConvertClaudeRequestToOpenAIWithOptions(body, provider.ClaudeToOpenAIConvertOptions{UseMaxCompletionTokens: p.supplier == "openai"})
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
	if streaming && p.UpstreamProtocol == managedProtocolChat {
		// Managed billing needs the provider's original usage, including when a
		// client suppresses it. Freeze the resulting wire bytes after this step.
		actual, err = sjson.SetBytes(actual, "stream_options.include_usage", true)
		if err != nil {
			return managedProtocolRequest{}, errors.New("managed Chat stream usage request failed")
		}
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
	if p.UpstreamProtocol == managedProtocolEmbeddings {
		return managedEmbeddingObservation(status, headers, body, complete, 0)
	}
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
		case managedProtocolChat, managedProtocolCompletions:
			chat := managedChatStream{completions: p.UpstreamProtocol == managedProtocolCompletions}
			chat.push(body)
			observation = chat.observation()
		case managedProtocolResponses:
			observation = managedResponsesStream(body)
		case managedProtocolMessages:
			observation = managedMessagesStream(body, p.supplier == "qwen")
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
		case managedProtocolCompletions:
			var choices []map[string]json.RawMessage
			if string(object["object"]) == `"text_completion"` &&
				json.Unmarshal(object["choices"], &choices) == nil && len(choices) == 1 &&
				!managedProtocolNull(choices[0]["finish_reason"]) {
				observation.ExecutionResult = managedCompletionChoice(choices[0])
				if observation.ExecutionResult != "unknown" {
					observation.Usage = managedOpenAIUsage(object)
				}
			}
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

// DeepSeek emits the same choice shape for JSON and SSE.
func managedCompletionChoice(choice map[string]json.RawMessage) string {
	var content, reason string
	if managedProtocolNull(choice["text"]) || json.Unmarshal(choice["text"], &content) != nil {
		return "unknown"
	}
	if managedProtocolNull(choice["finish_reason"]) {
		return ""
	}
	if json.Unmarshal(choice["finish_reason"], &reason) != nil {
		return "unknown"
	}
	switch reason {
	case "stop":
		return "succeeded"
	case "length", "content_filter", "insufficient_system_resource", "aborted":
		return "partial"
	default:
		return "unknown"
	}
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
	if details, exists := usage["output_tokens_details"]; exists {
		chat["completion_tokens_details"] = details
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
	write, hasWrite := managedCacheWriteCount(usage)
	if hasWrite && write == nil {
		return executionfacts.MissingTokenUsage(managedCacheWriteConflict)
	}
	if hasRead || hasWrite {
		read := new(big.Int)
		if hasRead {
			var ok bool
			read, ok = managedCount(readRaw)
			if !ok {
				return missing()
			}
		}
		miss := new(big.Int).Set(input)
		if hasWrite {
			miss.Add(miss, write)
		}
		total := new(big.Int).Add(new(big.Int).Set(miss), read)
		if total.BitLen() > 128 {
			return missing()
		}
		quantities["input"], quantities["cache_hit"], quantities["cache_miss"] = total.String(), read.String(), miss.String()
		if hasWrite {
			managedCacheWrite(quantities, write, usage["cache_creation"], usage["cache_write_tokens_details"])
		}
		quantities["cache_read_mode"] = "1"
	}
	return &executionfacts.Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: quantities}
}

type managedProtocolEvent struct {
	Name string
	Data []byte
}

func managedProtocolEvents(body []byte) ([]managedProtocolEvent, error) {
	if len(body) > int(defaultMaxBodyBytes) {
		return nil, errors.New("managed SSE body budget exceeded")
	}
	var events []managedProtocolEvent
	var decoder managedSSEDecoder
	if err := decoder.push(body, func(event managedProtocolEvent) {
		events = append(events, event)
	}); err != nil {
		return nil, err
	}
	return events, decoder.finish()
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
	stream := newManagedNativeStream(managedProtocolResponses, false, false)
	stream.push(body, nil)
	return stream.observation()
}

func managedMessagesStream(body []byte, terminalInput bool) managedProtocolObservation {
	stream := newManagedNativeStream(managedProtocolMessages, terminalInput, false)
	stream.push(body, nil)
	return stream.observation()
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
	result.Headers = managedProtocolConvertedHeaders(headers, stream)
	return result, nil
}

func managedProtocolConvertedHeaders(headers [][2]string, stream bool) [][2]string {
	var result [][2]string
	for _, header := range managedResponseHeaders(headers) {
		switch strings.ToLower(header[0]) {
		case "content-type", "content-encoding", "etag", "content-md5", "digest", "content-digest", "repr-digest":
			continue
		}
		result = append(result, header)
	}
	contentType := "application/json"
	if stream {
		contentType = "text/event-stream"
	}
	result = append(result, [2]string{"content-type", contentType})
	return result
}

func managedProtocolConversionObject(raw []byte, stream bool) ([]byte, error) {
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

func managedProtocolConversionInput(body []byte, stream bool) ([]byte, error) {
	if !stream {
		return managedProtocolConversionObject(body, false)
	}
	events, err := managedProtocolEvents(body)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	for _, event := range events {
		data := event.Data
		if !bytes.Equal(data, []byte("[DONE]")) {
			data, err = managedProtocolConversionObject(data, true)
			if err != nil {
				return nil, err
			}
		}
		managedProtocolWriteEvent(&output, event.Name, data)
	}
	return output.Bytes(), nil
}

func managedProtocolWireUsage(protocol string, usage *executionfacts.Usage) json.RawMessage {
	if usage == nil || usage.Status != "reported" && !(protocol == managedProtocolEmbeddings && usage.Status == "estimated") {
		return nil
	}
	quantities := usage.Quantities
	input, inputOK := managedInteger(quantities["input"])
	if !inputOK {
		return nil
	}
	wire := map[string]json.RawMessage{}
	if protocol == managedProtocolEmbeddings {
		wire["prompt_tokens"], wire["total_tokens"] = json.RawMessage(input.String()), json.RawMessage(input.String())
		if usage.Status == "estimated" {
			wire["source"] = json.RawMessage(`"estimator"`)
		}
	} else {
		output, outputOK := managedInteger(quantities["output"])
		if !outputOK {
			return nil
		}
		if protocol == managedProtocolChat || protocol == managedProtocolResponses {
			total := new(big.Int).Add(input, output)
			if total.BitLen() > 128 {
				return nil
			}
			wire["total_tokens"] = json.RawMessage(total.String())
		}
		if protocol == managedProtocolMessages {
			wire["input_tokens"], wire["output_tokens"] = json.RawMessage(input.String()), json.RawMessage(output.String())
			if hit, exists := quantities["cache_hit"]; exists {
				miss, valid := managedInteger(quantities["cache_miss"])
				if !valid {
					return nil
				}
				if write, ok := managedInteger(quantities["cache_write"]); ok {
					miss.Sub(miss, write)
					wire["cache_creation_input_tokens"] = json.RawMessage(write.String())
				} else {
					// Retain the existing converter's paired cache counters.
					// The public projection still reads the original report.
					wire["cache_creation_input_tokens"] = json.RawMessage("0")
				}
				wire["input_tokens"], wire["cache_read_input_tokens"] = json.RawMessage(miss.String()), json.RawMessage(hit)
			}
		} else {
			inputKey, outputKey := "input_tokens", "output_tokens"
			inputDetailsKey, outputDetailsKey := "input_tokens_details", "output_tokens_details"
			if protocol == managedProtocolChat {
				inputKey, outputKey = "prompt_tokens", "completion_tokens"
				inputDetailsKey, outputDetailsKey = "prompt_tokens_details", "completion_tokens_details"
			}
			wire[inputKey], wire[outputKey] = json.RawMessage(input.String()), json.RawMessage(output.String())
			details := make(map[string]json.Number, 2)
			for meter, field := range map[string]string{"cache_hit": "cached_tokens", "cache_write": "cache_write_tokens"} {
				if value, exists := quantities[meter]; exists {
					details[field] = json.Number(value)
				}
			}
			if len(details) > 0 {
				wire[inputDetailsKey], _ = json.Marshal(details)
			}
			if reasoning, exists := quantities["reasoning_tokens"]; exists {
				wire[outputDetailsKey], _ = json.Marshal(map[string]json.Number{"reasoning_tokens": json.Number(reasoning)})
			}
		}
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil
	}
	return encoded
}

func managedProtocolRestoreUsage(body []byte, stream bool, protocol string, usage *executionfacts.Usage) ([]byte, error) {
	wire := managedProtocolWireUsage(protocol, usage)
	if wire == nil && usage != nil && usage.Status == "reported" {
		return nil, errors.New("exact managed usage cannot be represented by client profile")
	}
	if stream && protocol == managedProtocolChat {
		events, err := managedProtocolEvents(body)
		if err != nil {
			return nil, err
		}
		var output bytes.Buffer
		var last map[string]json.RawMessage
		for _, event := range events {
			if bytes.Equal(event.Data, []byte("[DONE]")) {
				if wire != nil {
					if last == nil {
						return nil, errors.New("converted Chat response has no content frame")
					}
					last["choices"], last["usage"] = json.RawMessage("[]"), wire
					encoded, err := json.Marshal(last)
					if err != nil {
						return nil, err
					}
					managedProtocolWriteEvent(&output, "", encoded)
				}
				managedProtocolWriteEvent(&output, event.Name, event.Data)
				continue
			}
			var object map[string]json.RawMessage
			if decodeManagedJSON(event.Data, &object, false) != nil || object == nil {
				return nil, errors.New("invalid converted Chat frame")
			}
			_, hadUsage := object["usage"]
			delete(object, "usage")
			last = object
			if hadUsage {
				var choices []struct {
					Delta        map[string]json.RawMessage `json:"delta"`
					FinishReason *string                    `json:"finish_reason"`
				}
				if err := json.Unmarshal(object["choices"], &choices); err != nil {
					return nil, errors.New("invalid converted Chat usage frame")
				}
				// Qwen's content converter emits an empty delta for its generated
				// usage. Omit that payload-free frame; exact usage is emitted at DONE.
				if len(choices) == 0 || len(choices) == 1 && len(choices[0].Delta) == 0 && choices[0].FinishReason == nil {
					continue
				}
			}
			encoded, err := json.Marshal(object)
			if err != nil {
				return nil, err
			}
			managedProtocolWriteEvent(&output, event.Name, encoded)
		}
		return output.Bytes(), nil
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
