package main

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	memoryResponseStatusContextKey     = "memoryResponseStatus"
	memoryResponseBodyBufferContextKey = "memoryResponseBodyBuffer"
	memoryResponseCaptureContextKey    = "memoryResponseCapture"
	memoryResponseOverflowContextKey   = "memoryResponseOverflow"
	memoryStreamCaptureContextKey      = "memoryStreamCapture"
	memoryStreamChunksContextKey       = "memoryStreamChunks"
	memoryStreamParseFailedContextKey  = "memoryStreamParseFailed"
)

type memoryResponseCapture struct {
	AssistantContent  string
	FinishReason      string
	Usage             sessionctx.Usage
	StatusCode        int
	ContainsToolCalls bool
	ParseFailed       bool
	UnsafeContent     bool
	IsStream          bool
}

func memoryCaptureResponseStatus(ctx wrapper.HttpContext, log logs.Log) {
	statusHeader, _ := proxywasm.GetHttpResponseHeader(":status")
	statusCode, err := strconv.Atoi(strings.TrimSpace(statusHeader))
	if err != nil {
		log.Warnf("[ai-memory] invalid response status %q, recording 0: %v", statusHeader, err)
		statusCode = 0
	}
	ctx.SetContext(memoryResponseStatusContextKey, statusCode)
}

func memoryCaptureNonStreamingResponse(body []byte, statusCode int, responseValueFrom string, toolCallPaths []string) memoryResponseCapture {
	capture := memoryResponseCapture{StatusCode: statusCode}
	response, err := sessionctx.ParseOpenAIChatResponseWithOptions(body, sessionctx.ResponseParseOptions{
		AdditionalToolCallPaths: toolCallPaths,
	})
	if err != nil {
		capture.ParseFailed = true
		return capture
	}
	capture.AssistantContent = memoryAssistantContentFromPath(body, responseValueFrom, response.AssistantContent)
	capture.FinishReason = sessionctx.ResponseFinishReason(response)
	capture.Usage = sessionctx.ResponseUsage(response)
	capture.ContainsToolCalls = sessionctx.ContainsToolUse(response)
	return capture
}

func memoryCaptureNonStreamingResponseForProtocol(body []byte, statusCode int, kind protocol.ProtocolKind, responseValueFrom string, toolCallPaths []string) memoryResponseCapture {
	if kind == protocol.ProtocolChatCompletions {
		return memoryCaptureNonStreamingResponse(body, statusCode, responseValueFrom, toolCallPaths)
	}

	adapter := memoryProtocolAdapterForKind(kind)
	exchange, err := adapter.CaptureResponse(protocolResponseCaptureInput(statusCode, body))
	if err != nil {
		return memoryResponseCapture{StatusCode: statusCode, ParseFailed: true}
	}
	return memoryResponseCaptureFromProtocolExchange(statusCode, false, exchange)
}

func memoryCaptureNonStreamingResponseChunk(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log logs.Log) {
	memoryCaptureNonStreamingResponseChunkWithLimit(ctx, c, chunk, isLastChunk, maxResponseBodyBytes, log)
}

func memoryCaptureNonStreamingResponseChunkWithLimit(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, limit int, log logs.Log) {
	if memoryGateReason(ctx) != "" || ctx.GetBoolContext(memoryStreamContextKey, false) {
		return
	}
	if ctx.GetBoolContext(memoryResponseOverflowContextKey, false) {
		return
	}
	if !isLastChunk {
		if len(chunk) > 0 {
			buffered := ctx.GetByteSliceContext(memoryResponseBodyBufferContextKey, nil)
			if memoryResponseCaptureExceedsLimit(len(buffered), len(chunk), limit) {
				log.Warn("[ai-memory] non-streaming response capture exceeded buffer limit, failed open")
				memoryStoreParseFailedResponseCapture(ctx)
				return
			}
			buffered = append(buffered, chunk...)
			ctx.SetContext(memoryResponseBodyBufferContextKey, buffered)
		}
		return
	}

	body := chunk
	buffered := ctx.GetByteSliceContext(memoryResponseBodyBufferContextKey, nil)
	if memoryResponseCaptureExceedsLimit(len(buffered), len(chunk), limit) {
		log.Warn("[ai-memory] non-streaming response capture exceeded buffer limit, failed open")
		memoryStoreParseFailedResponseCapture(ctx)
		return
	}
	if len(buffered) > 0 {
		body = append(append([]byte(nil), buffered...), chunk...)
	}
	statusCode := 0
	if value, ok := ctx.GetContext(memoryResponseStatusContextKey).(int); ok {
		statusCode = value
	}
	kind := protocol.ProtocolKind(ctx.GetStringContext(memoryProtocolContextKey, ""))
	if reason := memoryOutputGate(kind, body); reason != "" {
		if reason == "response-parse-failed" {
			// Retain the existing safe failure event, never extracted text from
			// a response whose output shape could not be classified.
			ctx.SetContext(memoryResponseCaptureContextKey, memoryResponseCapture{StatusCode: statusCode, ParseFailed: true})
		} else {
			markMemoryGate(ctx, reason)
		}
		ctx.SetContext(memoryResponseBodyBufferContextKey, nil)
		return
	}
	capture := memoryCaptureNonStreamingResponseForProtocol(body, statusCode, kind, c.Route.ResponseValueFrom, c.Route.ToolCallsFrom)
	if capture.ParseFailed {
		log.Warnf("[ai-memory] parse non-streaming response failed open")
	}
	capture = memoryApplyRawContentGate(ctx, c, capture)
	ctx.SetContext(memoryResponseCaptureContextKey, capture)
}

func memoryCaptureStreamingResponseChunk(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log logs.Log) {
	if memoryGateReason(ctx) != "" || !ctx.GetBoolContext(memoryStreamContextKey, false) {
		return
	}
	if memoryStreamingCaptureUsesProtocolAdapter(ctx) {
		memoryCaptureProtocolStreamingResponseChunk(ctx, c, chunk, isLastChunk, log)
		return
	}
	capture := memoryStreamCapture(ctx, c)
	if !ctx.GetBoolContext(memoryStreamParseFailedContextKey, false) {
		if err := capture.AppendSSE(memoryFinalStreamingChunk(chunk, isLastChunk)); err != nil {
			log.Warn("[ai-memory] parse streaming response failed open")
			ctx.SetContext(memoryStreamParseFailedContextKey, true)
		}
	}
	if capture.outputGate != "" {
		markMemoryGate(ctx, capture.outputGate)
		ctx.SetContext(memoryStreamCaptureContextKey, nil)
		return
	}
	if !isLastChunk {
		return
	}
	statusCode := memoryStoredResponseStatus(ctx)
	if ctx.GetBoolContext(memoryStreamParseFailedContextKey, false) {
		ctx.SetContext(memoryResponseCaptureContextKey, memoryResponseCapture{
			StatusCode:  statusCode,
			ParseFailed: true,
			IsStream:    true,
		})
		return
	}
	responseCapture := memoryResponseCapture{
		AssistantContent:  capture.AssistantContent(),
		FinishReason:      capture.FinishReason(),
		Usage:             capture.Usage(),
		StatusCode:        statusCode,
		ContainsToolCalls: capture.ContainsToolCalls(),
		IsStream:          true,
	}
	ctx.SetContext(memoryResponseCaptureContextKey, memoryApplyRawContentGate(ctx, c, responseCapture))
}

func memoryStreamingCaptureUsesProtocolAdapter(ctx wrapper.HttpContext) bool {
	return protocol.ProtocolKind(ctx.GetStringContext(memoryProtocolContextKey, "")) != protocol.ProtocolChatCompletions
}

func memoryCaptureProtocolStreamingResponseChunk(ctx wrapper.HttpContext, c config.PluginConfig, chunk []byte, isLastChunk bool, log logs.Log) {
	if !ctx.GetBoolContext(memoryStreamParseFailedContextKey, false) && len(chunk) > 0 {
		chunks, _ := ctx.GetContext(memoryStreamChunksContextKey).([][]byte)
		chunks = append(chunks, memoryFinalStreamingChunk(chunk, isLastChunk))
		ctx.SetContext(memoryStreamChunksContextKey, chunks)
	}
	if !isLastChunk {
		return
	}
	statusCode := memoryStoredResponseStatus(ctx)
	if ctx.GetBoolContext(memoryStreamParseFailedContextKey, false) {
		ctx.SetContext(memoryResponseCaptureContextKey, memoryResponseCapture{
			StatusCode:  statusCode,
			ParseFailed: true,
			IsStream:    true,
		})
		return
	}
	adapter := memoryProtocolAdapterForKind(protocol.ProtocolKind(ctx.GetStringContext(memoryProtocolContextKey, "")))
	chunks, _ := ctx.GetContext(memoryStreamChunksContextKey).([][]byte)
	exchange, err := adapter.CaptureStream(protocolResponseStreamInput(statusCode, chunks))
	if err != nil {
		log.Warn("[ai-memory] parse streaming response failed open")
		ctx.SetContext(memoryResponseCaptureContextKey, memoryResponseCapture{
			StatusCode:  statusCode,
			ParseFailed: true,
			IsStream:    true,
		})
		return
	}
	kind := protocol.ProtocolKind(ctx.GetStringContext(memoryProtocolContextKey, ""))
	for _, frame := range exchange.StreamChunks {
		if reason := memoryOutputGate(kind, frame.Data); reason != "" {
			markMemoryGate(ctx, reason)
			ctx.SetContext(memoryStreamChunksContextKey, nil)
			return
		}
	}
	responseCapture := memoryResponseCaptureFromProtocolExchange(statusCode, true, exchange)
	ctx.SetContext(memoryResponseCaptureContextKey, memoryApplyRawContentGate(ctx, c, responseCapture))
}

func protocolResponseCaptureInput(statusCode int, body []byte) protocol.ResponseCaptureInput {
	return protocol.ResponseCaptureInput{StatusCode: statusCode, Body: body}
}

func protocolResponseStreamInput(statusCode int, chunks [][]byte) protocol.ResponseStreamInput {
	return protocol.ResponseStreamInput{StatusCode: statusCode, Chunks: chunks}
}

func memoryResponseCaptureFromProtocolExchange(statusCode int, isStream bool, exchange protocol.NormalizedExchange) memoryResponseCapture {
	return memoryResponseCapture{
		AssistantContent:  exchange.Response.Text,
		FinishReason:      exchange.Response.FinishReason,
		Usage:             memorySessionUsageFromProtocol(exchange.Usage),
		StatusCode:        statusCode,
		ContainsToolCalls: exchange.Response.ContainsToolCalls,
		ParseFailed:       exchange.Response.ParseFailed,
		UnsafeContent:     exchange.Response.UnsafeContent,
		IsStream:          isStream,
	}
}

func memoryApplyRawContentGate(ctx wrapper.HttpContext, c config.PluginConfig, capture memoryResponseCapture) memoryResponseCapture {
	if !memoryRawContentAllowed(ctx, c) {
		capture.AssistantContent = ""
	}
	return capture
}

func memoryStreamCapture(ctx wrapper.HttpContext, c config.PluginConfig) *memoryStreamingCapture {
	if capture, ok := ctx.GetContext(memoryStreamCaptureContextKey).(*memoryStreamingCapture); ok && capture != nil {
		return capture
	}
	capture := newMemoryStreamingCapture(c.Route.StreamValueFrom, c.Route.ToolCallsFrom)
	ctx.SetContext(memoryStreamCaptureContextKey, capture)
	return capture
}

type memoryStreamingCapture struct {
	parser          *sessionctx.StreamCapture
	streamValueFrom string
	content         strings.Builder
	buffer          string
	outputGate      string
}

func newMemoryStreamingCapture(streamValueFrom string, toolCallPaths []string) *memoryStreamingCapture {
	return &memoryStreamingCapture{
		parser: sessionctx.NewStreamCapture(sessionctx.StreamCaptureOptions{
			AdditionalToolCallPaths: toolCallPaths,
		}),
		streamValueFrom: streamValueFrom,
	}
}

func (c *memoryStreamingCapture) AppendSSE(chunk []byte) error {
	if err := c.parser.AppendSSE(chunk); err != nil {
		return err
	}
	c.buffer += string(chunk)
	for {
		index := strings.IndexAny(c.buffer, "\r\n")
		if index < 0 {
			return nil
		}
		separator := c.buffer[index]
		line := strings.TrimSpace(c.buffer[:index])
		c.buffer = c.buffer[index+1:]
		if separator == '\r' && strings.HasPrefix(c.buffer, "\n") {
			c.buffer = c.buffer[1:]
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		if reason := memoryOutputGate(protocol.ProtocolChatCompletions, []byte(payload)); reason != "" {
			c.outputGate = reason
			return nil
		}
		c.content.WriteString(memoryAssistantContentFromPath([]byte(payload), c.streamValueFrom, ""))
	}
}

func (c *memoryStreamingCapture) AssistantContent() string {
	if c.content.Len() > 0 {
		return c.content.String()
	}
	return c.parser.AssistantContent()
}

func (c *memoryStreamingCapture) FinishReason() string {
	return c.parser.FinishReason()
}

func (c *memoryStreamingCapture) Usage() sessionctx.Usage {
	return c.parser.Usage()
}

func (c *memoryStreamingCapture) ContainsToolCalls() bool {
	return c.parser.ContainsToolCalls()
}

func memoryFinalStreamingChunk(chunk []byte, isLastChunk bool) []byte {
	if !isLastChunk {
		return chunk
	}
	if len(chunk) == 0 {
		return []byte("\n")
	}
	last := chunk[len(chunk)-1]
	if last == '\n' || last == '\r' {
		return chunk
	}
	out := append([]byte(nil), chunk...)
	out = append(out, '\n')
	return out
}

func memoryResponseCaptureExceedsLimit(currentBytes int, chunkBytes int, limit int) bool {
	return limit > 0 && currentBytes > limit-chunkBytes
}

func memoryStoreParseFailedResponseCapture(ctx wrapper.HttpContext) {
	statusCode := memoryStoredResponseStatus(ctx)
	ctx.SetContext(memoryResponseBodyBufferContextKey, []byte(nil))
	ctx.SetContext(memoryResponseOverflowContextKey, true)
	ctx.SetContext(memoryResponseCaptureContextKey, memoryResponseCapture{
		StatusCode:  statusCode,
		ParseFailed: true,
	})
}

func memoryStoredResponseStatus(ctx wrapper.HttpContext) int {
	if value, ok := ctx.GetContext(memoryResponseStatusContextKey).(int); ok {
		return value
	}
	return 0
}

func memoryAssistantContentFromPath(body []byte, path string, fallback string) string {
	if path == "" {
		return fallback
	}
	value := gjson.GetBytes(body, path)
	if !value.Exists() {
		return fallback
	}
	content := memoryTextFromGJSON(value)
	if content == "" {
		return fallback
	}
	return content
}

func memoryTextFromGJSON(value gjson.Result) string {
	if value.Type == gjson.String {
		return value.String()
	}
	if value.Type != gjson.JSON {
		return value.String()
	}
	var content interface{}
	if err := json.Unmarshal([]byte(value.Raw), &content); err != nil {
		return ""
	}
	return memoryTextContent(content)
}

func memoryTextContent(value interface{}) string {
	switch content := value.(type) {
	case string:
		return content
	case []interface{}:
		var parts []string
		for _, item := range content {
			object, ok := item.(map[string]interface{})
			if !ok || object["type"] != "text" {
				continue
			}
			text, _ := object["text"].(string)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}
