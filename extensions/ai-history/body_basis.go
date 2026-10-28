package main

import (
	"mime"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	historyCaptureContextKey = "historyCaptureBodyBasis"
	historyBasisContextKey   = "historyOriginalBodyBasis"
	historyBasisRejectedKey  = "historyBodyBasisRejected"
)

// The publisher assigns this role to exactly the first body rewriter, using
// the exact artifacts' phase/order. Header presence never selects this role.
func prepareHistoryBodyBasis(ctx wrapper.HttpContext) types.Action {
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return rejectHistoryBodyBasis(ctx, 500, "header_read_failed")
	}
	filtered := make([][2]string, 0, len(headers))
	var contentType, encoding string
	var contentTypes, encodings int
	for _, header := range headers {
		switch {
		case strings.EqualFold(header[0], bodybasis.DigestHeader), strings.EqualFold(header[0], bodybasis.PublicModelHeader):
			continue
		case strings.EqualFold(header[0], "content-type"):
			contentType = header[1]
			contentTypes++
		case strings.EqualFold(header[0], "content-encoding"):
			encoding = header[1]
			encodings++
		}
		filtered = append(filtered, header)
	}
	if proxywasm.ReplaceHttpRequestHeaders(filtered) != nil {
		return rejectHistoryBodyBasis(ctx, 500, "basis_clear_failed")
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if contentTypes != 1 || err != nil || !(strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json")) {
		return rejectHistoryBodyBasis(ctx, 415, "json_required")
	}
	if encodings > 1 || encoding != "" && !strings.EqualFold(encoding, "identity") {
		return rejectHistoryBodyBasis(ctx, 415, "content_encoding_unsupported")
	}
	if !ctx.HasRequestBody() {
		return rejectHistoryBodyBasis(ctx, 400, "body_required")
	}
	ctx.SetContext(historyCaptureContextKey, true)
	ctx.SetRequestBodyBufferLimit(historyBodyLimit)
	if !wrapper.RequireRequestBody(ctx) {
		return rejectHistoryBodyBasis(ctx, 500, "body_context_unavailable")
	}
	return types.ActionContinue
}

func captureHistoryBodyBasis(ctx wrapper.HttpContext, body []byte) types.Action {
	if ctx.GetBoolContext(historyBasisRejectedKey, false) {
		return types.ActionPause
	}
	if !ctx.GetBoolContext(historyCaptureContextKey, false) || ctx.GetContext(historyBasisContextKey) != nil {
		return types.ActionContinue
	}
	basis, err := bodybasis.Capture(body)
	if err != nil {
		return rejectHistoryBodyBasis(ctx, 400, "invalid_model_body")
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return rejectHistoryBodyBasis(ctx, 500, "header_read_failed")
	}
	if proxywasm.ReplaceHttpRequestHeaders(append(headers, basis.Headers()...)) != nil {
		return rejectHistoryBodyBasis(ctx, 500, "basis_write_failed")
	}
	ctx.SetContext(historyBasisContextKey, basis)
	return types.ActionContinue
}

func rejectHistoryBodyBasis(ctx wrapper.HttpContext, status uint32, code string) types.Action {
	ctx.SetContext(historyBasisRejectedKey, true)
	ctx.SetContext(historyGateContextKey, "body-basis-"+code)
	ctx.DontReadRequestBody()
	ctx.DontReadResponseBody()
	body := []byte(`{"error":{"code":"` + code + `","type":"history_body_basis_error"}}`)
	if proxywasm.SendHttpResponse(status, [][2]string{{"content-type", "application/json"}}, body, -1) != nil {
		proxywasm.LogCritical("ai-history could not send body-basis rejection")
	}
	return types.ActionPause
}
