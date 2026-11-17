package wrapper

import (
	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"strconv"
	"time"
	"unicode/utf8"
)

const OriginalRequestStartContext = "modelfusion.original-request-start"
const originalRequestIDContext = "modelfusion.original-request-id"

const (
	originalLogCapturedProperty      = "modelfusion_original_log_captured_v1"
	currentLogCapturedProperty       = "modelfusion_current_log_captured_v1"
	currentLogIDProperty             = "modelfusion_current_log_request_id_v1"
	currentLogIDJSONProperty         = "modelfusion_current_log_request_id_json_v1"
	originalLogIDProperty            = "modelfusion_original_log_request_id_v1"
	originalLogIDJSONProperty        = "modelfusion_original_log_request_id_json_v1"
	originalLogUserAgentProperty     = "modelfusion_original_log_user_agent_v1"
	originalLogUserAgentJSONProperty = "modelfusion_original_log_user_agent_json_v1"
	originalLogForwardedProperty     = "modelfusion_original_log_x_forwarded_for_v1"
	originalLogForwardedJSONProperty = "modelfusion_original_log_x_forwarded_for_json_v1"
)

// PreserveOriginalRequestID retains the gateway ID across supplier header reconstruction and fallback.
func PreserveOriginalRequestID(ctx HttpContext) string {
	if _, ok := ctx.GetContext(OriginalRequestStartContext).(time.Time); !ok {
		start := time.Now()
		if raw, err := proxywasm.GetProperty([]string{"modelfusion_original_request_start"}); err == nil {
			if timestamp, err := strconv.ParseInt(string(raw), 10, 64); err == nil && timestamp > 0 {
				start = time.UnixMilli(timestamp)
			}
		}
		ctx.SetContext(OriginalRequestStartContext, start)
		if err := proxywasm.SetProperty([]string{"modelfusion_original_request_start"}, []byte(strconv.FormatInt(start.UnixMilli(), 10))); err != nil {
			proxywasm.LogWarnf("original request start filter-state write failed: %v", err)
		}
	}
	requestID, _ := ctx.GetContext(originalRequestIDContext).(string)
	if requestID == "" {
		raw, err := proxywasm.GetProperty([]string{"modelfusion_original_request_id"})
		if err == nil {
			requestID = string(raw)
		}
	}
	if requestID == "" || requestID == "-" {
		requestID, _ = proxywasm.GetHttpRequestHeader("x-request-id")
		if requestID == "" || requestID == "-" {
			if raw, err := proxywasm.GetProperty([]string{"x_request_id"}); err == nil {
				requestID = string(raw)
			}
		}
		if requestID == "" || requestID == "-" {
			requestID = uuid.New().String()
		}
	}
	ctx.SetContext(originalRequestIDContext, requestID)
	if err := proxywasm.SetProperty([]string{"modelfusion_original_request_id"}, []byte(MarshalStr(requestID))); err != nil {
		proxywasm.LogWarnf("original request ID filter-state write failed: %v", err)
	}
	return requestID
}

// preserveOriginalRequestLogMetadata promotes proven original values before legacy state is rewritten.
// A preceding old wrapper preserved the ID but did not preserve UA or XFF.
// X-Forwarded-For is an untrusted declared header, never a verified client address.
func preserveOriginalRequestLogMetadata(requestID string, legacyID string, firstWrapper bool, continuation bool) {
	if raw, err := proxywasm.GetProperty([]string{originalLogCapturedProperty}); err == nil && len(raw) > 0 {
		return
	}
	continuation = continuation || RouterContinuationPending()
	if err := proxywasm.SetProperty([]string{originalLogCapturedProperty}, []byte("0")); err != nil {
		proxywasm.LogWarnf("original request log capture marker write failed: %v", err)
		return
	}
	userAgent, forwardedFor := "", ""
	if continuation {
		// No original snapshot survived from the outer chain. Its legacy
		// FilterChain slot and the current headers cannot prove an original.
		requestID = ""
	} else if firstWrapper {
		// No earlier wrapper/start state exists in this original filter chain.
		// Use the core's first ID decision, never a later current-header guess.
		userAgent, _ = proxywasm.GetHttpRequestHeader("user-agent")
		forwardedFor, _ = proxywasm.GetHttpRequestHeader("x-forwarded-for")
	} else {
		requestID = legacyID
		// Old wrappers repeatedly read and MarshalStr this slot. Escaped values
		// have ambiguous depth and cannot safely be decoded into an original ID.
		if len(requestID) > 256 || !utf8.ValidString(requestID) || MarshalStr(requestID) != requestID {
			requestID = ""
		}
	}
	if len(requestID) > 256 || requestID == "-" {
		requestID = ""
	}
	for _, field := range []struct {
		rawProperty  string
		jsonProperty string
		value        string
		limit        int
	}{
		{originalLogIDProperty, originalLogIDJSONProperty, requestID, 256},
		{originalLogUserAgentProperty, originalLogUserAgentJSONProperty, userAgent, 512},
		{originalLogForwardedProperty, originalLogForwardedJSONProperty, forwardedFor, 2048},
	} {
		value := originalRequestLogValue(field.value, field.limit)
		if value == "" {
			continue
		}
		if err := proxywasm.SetProperty([]string{field.rawProperty}, []byte(value)); err != nil {
			proxywasm.LogWarnf("original request log value write failed: %v", err)
			return
		}
		if jsonValue := MarshalStr(value); jsonValue != "" {
			if err := proxywasm.SetProperty([]string{field.jsonProperty}, []byte(jsonValue)); err != nil {
				proxywasm.LogWarnf("original request log JSON value write failed: %v", err)
				return
			}
		}
	}
	if err := proxywasm.SetProperty([]string{originalLogCapturedProperty}, []byte("1")); err != nil {
		proxywasm.LogWarnf("original request log capture completion write failed: %v", err)
	}
}

func (ctx *CommonHttpCtx[C]) originalRequestLogState() (bool, bool) {
	return ctx.plugin.originalLogReady, ctx.originalLogContinuation
}

// PreserveCurrentRequestLogID retains the post-editor ID before Router replaces headers.
// It is a log-only value; it never becomes an original ID or a supplier header.
func PreserveCurrentRequestLogID(ctx HttpContext) {
	state, ok := ctx.(interface{ originalRequestLogState() (bool, bool) })
	if !ok {
		return
	}
	ready, continuation := state.originalRequestLogState()
	if !ready || continuation || RouterContinuationPending() {
		return
	}
	if raw, err := proxywasm.GetProperty([]string{currentLogCapturedProperty}); err == nil && len(raw) > 0 {
		return
	}
	if err := proxywasm.SetProperty([]string{currentLogCapturedProperty}, []byte("0")); err != nil {
		proxywasm.LogWarnf("current request log capture marker write failed: %v", err)
		return
	}
	requestID, _ := proxywasm.GetHttpRequestHeader("x-request-id")
	if len(requestID) > 256 || requestID == "-" {
		requestID = ""
	}
	requestID = originalRequestLogValue(requestID, 256)
	if requestID != "" {
		if err := proxywasm.SetProperty([]string{currentLogIDProperty}, []byte(requestID)); err != nil {
			proxywasm.LogWarnf("current request log ID write failed: %v", err)
			return
		}
		if jsonValue := MarshalStr(requestID); jsonValue != "" {
			if err := proxywasm.SetProperty([]string{currentLogIDJSONProperty}, []byte(jsonValue)); err != nil {
				proxywasm.LogWarnf("current request log JSON ID write failed: %v", err)
				return
			}
		}
	}
	if err := proxywasm.SetProperty([]string{currentLogCapturedProperty}, []byte("1")); err != nil {
		proxywasm.LogWarnf("current request log capture completion write failed: %v", err)
	}
}

func originalRequestLogValue(value string, limit int) string {
	if !utf8.ValidString(value) {
		return ""
	}
	if len(value) > limit {
		value = value[:limit]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}
