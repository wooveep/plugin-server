package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/protocol"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

const (
	historyNativeContextKey = "historyNative"
	historyGateContextKey   = "historyGate"
	historyBodyLimit        = 100 * 1024 * 1024
)

type historyNativeIdentity struct {
	historyNativeConfig
	Caller balanceidentity.Snapshot `json:"caller"`
}

type historyTurn struct {
	Input  []json.RawMessage `json:"input"`
	Output []json.RawMessage `json:"output"`
}

type nativeHistoryDocument struct {
	SchemaVersion string                `json:"schema_version"`
	Identity      historyNativeIdentity `json:"native_identity"`
	Protocol      protocol.ProtocolKind `json:"protocol"`
	PublicModel   string                `json:"public_model"`
	Turns         []historyTurn         `json:"turns"`
}

type nativeHistoryState struct {
	Document      nativeHistoryDocument
	Key           string
	Request       historyRequest
	MaySave       bool
	Status        int
	Stream        bool
	ResponseBytes int
	Chunks        [][]byte
}

func bypassHistory(ctx wrapper.HttpContext, reason string) types.Action {
	ctx.SetContext(historyGateContextKey, reason)
	ctx.SetUserAttribute("history_status", reason)
	ctx.DontReadResponseBody()
	if ctx.GetBoolContext(historyCaptureContextKey, false) && ctx.GetContext(historyBasisContextKey) == nil {
		// The release still needs its first-body capture when history itself is
		// off, no-store, or missing a caller. Keep downstream headers buffered.
		return types.HeaderStopIteration
	}
	ctx.DontReadRequestBody()
	return types.ActionContinue
}

func nativeHistoryHeaders(ctx wrapper.HttpContext, config PluginConfig) types.Action {
	binding := config.historyBinding
	if binding.Bypass != "" {
		return bypassHistory(ctx, binding.Bypass)
	}
	contentType, _ := proxywasm.GetHttpRequestHeader("content-type")
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
		return bypassHistory(ctx, "unsupported-content-type")
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return bypassHistory(ctx, "missing-native-caller")
	}
	for _, header := range headers {
		if strings.EqualFold(header[0], "cache-control") {
			for _, directive := range strings.Split(header[1], ",") {
				if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
					return bypassHistory(ctx, "no-store")
				}
			}
		}
	}
	// The preceding authentication filter clears caller-supplied reserved
	// headers. Do not fall back to identityHeader/Authorization in this branch.
	caller, err := balanceidentity.Read(headers)
	if err != nil || caller.TenantID == "" || caller.PrincipalID == "" {
		return bypassHistory(ctx, "missing-native-caller")
	}
	if !ctx.HasRequestBody() {
		return bypassHistory(ctx, "missing-request-body")
	}
	ctx.SetContext(historyNativeContextKey, &nativeHistoryState{Document: nativeHistoryDocument{
		SchemaVersion: "history.native.v1",
		Identity:      historyNativeIdentity{historyNativeConfig: binding.Identity, Caller: caller},
		Protocol:      binding.Protocol,
		PublicModel:   binding.PublicModel,
	}})
	ctx.SetRequestBodyBufferLimit(historyBodyLimit)
	_ = proxywasm.RemoveHttpRequestHeader("accept-encoding")
	_ = proxywasm.RemoveHttpRequestHeader("content-length")
	return types.HeaderStopIteration
}

func nativeHistoryRequestBody(ctx wrapper.HttpContext, config PluginConfig, body []byte, logger log.Log) types.Action {
	if ctx.GetStringContext(historyGateContextKey, "") != "" {
		return types.ActionContinue
	}
	state, ok := ctx.GetContext(historyNativeContextKey).(*nativeHistoryState)
	if !ok {
		return bypassHistory(ctx, "missing-native-caller")
	}
	// Only consume the trusted basis captured before the first rewrite. A leaf
	// cannot authenticate client headers or reconstruct that basis after memory.
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return bypassHistory(ctx, "missing-trusted-body-basis")
	}
	basis, err := bodybasis.Read(headers)
	if err != nil || basis.PublicModel() != state.Document.PublicModel {
		return bypassHistory(ctx, "missing-trusted-body-basis")
	}
	request, err := parseHistoryRequest(state.Document.Protocol, body, state.Document.PublicModel)
	if err != nil {
		return bypassHistory(ctx, "unsupported-request")
	}
	state.Request = request
	// The hashed namespace contains no raw credential; all five actual caller
	// values, the complete frozen binding, protocol and public model isolate it.
	namespace, err := json.Marshal(struct {
		Identity    historyNativeIdentity `json:"native_identity"`
		Protocol    protocol.ProtocolKind `json:"protocol"`
		PublicModel string                `json:"public_model"`
	}{state.Document.Identity, state.Document.Protocol, state.Document.PublicModel})
	if err != nil {
		return bypassHistory(ctx, "history-scope-error")
	}
	digest := sha256.Sum256(namespace)
	state.Key = config.CacheKeyPrefix + "native:" + hex.EncodeToString(digest[:])
	err = config.redisClient.Get(state.Key, func(response resp.Value) {
		if response.Error() != nil {
			ctx.SetUserAttribute("history_status", "redis-read-error")
			_ = proxywasm.ResumeHttpRequest()
			return
		}
		if !response.IsNull() {
			stored, err := readNativeHistory([]byte(response.String()), state.Document)
			if err != nil {
				ctx.SetUserAttribute("history_status", "invalid-history")
				_ = proxywasm.ResumeHttpRequest()
				return
			}
			state.Document = stored
		}
		state.MaySave = true
		count := getIntQueryParameter("fill_history_cnt", ctx.Path(), config.FillHistoryCnt)
		injected, err := request.inject(state.Document.Turns, count, body)
		if err != nil {
			state.MaySave = false
			ctx.SetUserAttribute("history_status", "history-injection-error")
		} else if err := proxywasm.ReplaceHttpRequestBody(injected); err != nil {
			state.MaySave = false
			ctx.SetUserAttribute("history_status", "history-injection-error")
		} else {
			ctx.SetUserAttribute("history_status", "context-applied")
		}
		_ = proxywasm.ResumeHttpRequest()
	})
	if err != nil {
		logger.Warn("history Redis read failed open")
		return bypassHistory(ctx, "redis-read-error")
	}
	return types.ActionPause
}

func readNativeHistory(raw []byte, expected nativeHistoryDocument) (nativeHistoryDocument, error) {
	var document nativeHistoryDocument
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return nativeHistoryDocument{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nativeHistoryDocument{}, fmt.Errorf("stored history must contain one JSON document")
	}
	if document.SchemaVersion != expected.SchemaVersion || document.Identity != expected.Identity ||
		document.Protocol != expected.Protocol || document.PublicModel != expected.PublicModel {
		return nativeHistoryDocument{}, fmt.Errorf("stored history scope does not match the frozen binding")
	}
	for _, turn := range document.Turns {
		if len(turn.Input) == 0 || len(turn.Output) == 0 {
			return nativeHistoryDocument{}, fmt.Errorf("stored history turn is incomplete")
		}
		for _, messages := range [][]json.RawMessage{turn.Input, turn.Output} {
			for _, raw := range messages {
				var message map[string]json.RawMessage
				if err := json.Unmarshal(raw, &message); err != nil || message == nil {
					return nativeHistoryDocument{}, fmt.Errorf("stored history message must be an object")
				}
			}
		}
	}
	return document, nil
}

func nativeHistoryResponseHeaders(ctx wrapper.HttpContext) types.Action {
	state, ok := ctx.GetContext(historyNativeContextKey).(*nativeHistoryState)
	if !ok || !state.MaySave {
		ctx.DontReadResponseBody()
		return types.ActionContinue
	}
	status, _ := proxywasm.GetHttpResponseHeader(":status")
	state.Status, _ = strconv.Atoi(status)
	if state.Status < 200 || state.Status >= 300 {
		return bypassHistory(ctx, "upstream-http-error")
	}
	headers, err := proxywasm.GetHttpResponseHeaders()
	if err != nil {
		return bypassHistory(ctx, "missing-response-headers")
	}
	for _, header := range headers {
		if strings.EqualFold(header[0], "cache-control") {
			for _, directive := range strings.Split(header[1], ",") {
				if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
					return bypassHistory(ctx, "no-store")
				}
			}
		}
	}
	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	state.Stream = mediaType == "text/event-stream"
	if !state.Stream && mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
		return bypassHistory(ctx, "unsupported-response-content-type")
	}
	return types.ActionContinue
}

func nativeHistoryResponseBody(ctx wrapper.HttpContext, config PluginConfig, chunk []byte, last bool, logger log.Log) []byte {
	if ctx.GetStringContext(historyGateContextKey, "") != "" {
		return chunk
	}
	state, ok := ctx.GetContext(historyNativeContextKey).(*nativeHistoryState)
	if !ok || !state.MaySave {
		return chunk
	}
	state.ResponseBytes += len(chunk)
	if state.ResponseBytes > historyBodyLimit {
		state.Chunks = nil
		bypassHistory(ctx, "response-too-large")
		return chunk
	}
	state.Chunks = append(state.Chunks, append([]byte(nil), chunk...))
	if !last {
		return chunk
	}
	output, reason := historyResponseOutput(state.Document.Protocol, state.Status, state.Stream, state.Chunks)
	state.Chunks = nil
	state.MaySave = false
	if reason != "" {
		ctx.SetUserAttribute("history_status", reason)
		return chunk
	}
	state.Document.Turns = append(state.Document.Turns, historyTurn{Input: state.Request.Turn, Output: output})
	if len(state.Document.Turns) > config.FillHistoryCnt {
		state.Document.Turns = state.Document.Turns[len(state.Document.Turns)-config.FillHistoryCnt:]
	}
	encoded, err := json.Marshal(state.Document)
	if err != nil {
		ctx.SetUserAttribute("history_status", "history-encode-error")
		return chunk
	}
	ctx.SetUserAttribute("history_status", "history-write-dispatched")
	// The existing RedisClient wire API accepts []interface{} command values.
	command := []interface{}{"SET", state.Key, string(encoded)}
	if config.CacheTTL != 0 {
		command = append(command, "EX", strconv.Itoa(config.CacheTTL))
	}
	if err := config.redisClient.Command(command, func(value resp.Value) {
		if value.Error() != nil {
			logger.Warn("history Redis write failed open")
			ctx.SetUserAttribute("history_status", "redis-write-error")
			return
		}
		ctx.SetUserAttribute("history_status", "history-stored")
	}); err != nil {
		logger.Warn("history Redis write failed open")
		ctx.SetUserAttribute("history_status", "redis-write-error")
		return chunk
	}
	return chunk
}
