package wrapper

import (
	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"strconv"
	"time"
)

const OriginalRequestStartContext = "modelfusion.original-request-start"
const originalRequestIDContext = "modelfusion.original-request-id"

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
