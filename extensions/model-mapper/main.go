package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// @Title model-mapper
// @Name model-mapper
// @Category ai
// @Phase UNSPECIFIED_PHASE
// @Priority 200

const (
	maxRequestBodyBytes = 1 << 20
	maxRuntimeBodyBytes = 100 * 1024 * 1024
	rejectedContextKey  = "model-mapper.rejected"
	mappedContextKey    = "model-mapper.mapped"
	basisContextKey     = "model-mapper.original-basis"
)

type Config struct {
	modelMapping     map[string]string
	captureBodyBasis bool
	consumeBodyBasis bool
}

func main() {}

func init() {
	wrapper.SetCtx(
		"model-mapper",
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessRequestBody(onHttpRequestBody),
	)
}

func parseConfig(input gjson.Result, config *Config) error {
	if !input.IsObject() {
		return errors.New("model-mapper configuration must be an object")
	}
	parsed := Config{modelMapping: make(map[string]string)}
	seen := make(map[string]bool)
	var parseErr error
	input.ForEach(func(key, value gjson.Result) bool {
		name := key.String()
		if name != "modelMapping" && name != "capture_body_basis" && name != "consume_body_basis" {
			return true
		}
		if seen[name] {
			parseErr = fmt.Errorf("duplicate configuration field %q", name)
			return false
		}
		seen[name] = true
		switch name {
		case "capture_body_basis":
			if value.Type != gjson.True && value.Type != gjson.False {
				parseErr = errors.New("capture_body_basis must be a boolean")
				return false
			}
			parsed.captureBodyBasis = value.Bool()
		case "consume_body_basis":
			if value.Type != gjson.True && value.Type != gjson.False {
				parseErr = errors.New("consume_body_basis must be a boolean")
				return false
			}
			parsed.consumeBodyBasis = value.Bool()
		case "modelMapping":
			if !value.IsObject() {
				parseErr = errors.New("modelMapping must be an object")
				return false
			}
			value.ForEach(func(source, target gjson.Result) bool {
				model := source.String()
				if _, duplicate := parsed.modelMapping[model]; duplicate {
					parseErr = fmt.Errorf("duplicate modelMapping key %q", model)
					return false
				}
				if bodybasis.ValidateModel(model) != nil || target.Type != gjson.String ||
					bodybasis.ValidateModel(target.Str) != nil {
					parseErr = errors.New("modelMapping keys and targets must be nonblank header-safe strings")
					return false
				}
				parsed.modelMapping[model] = target.Str
				return true
			})
		}
		return parseErr == nil
	})
	if parseErr != nil {
		return parseErr
	}
	if parsed.captureBodyBasis && parsed.consumeBodyBasis {
		return errors.New("capture_body_basis and consume_body_basis are mutually exclusive")
	}
	*config = parsed
	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config Config) types.Action {
	ctx.DisableReroute()
	ctx.DontReadResponseBody()
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return reject(ctx, 500, "header_read_failed")
	}
	if config.consumeBodyBasis {
		// Only trusted publication configuration selects consume mode. The first
		// rewriter has already removed client values and captured the original.
		basis, err := bodybasis.Read(headers)
		if err != nil {
			return reject(ctx, 400, "original_body_basis_missing")
		}
		ctx.SetContext(basisContextKey, basis)
	} else {
		// Standalone mapping keeps the original explicit capture behavior.
		if err := proxywasm.ReplaceHttpRequestHeaders(withoutBasis(headers, false)); err != nil {
			return reject(ctx, 500, "basis_clear_failed")
		}
	}
	if ctx.Method() != "POST" {
		return reject(ctx, 405, "post_required")
	}
	contentType, ok := singleHeader(headers, "content-type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if !ok || err != nil || !strings.EqualFold(mediaType, "application/json") {
		return reject(ctx, 415, "json_required")
	}
	if encoding, ok := singleHeader(headers, "content-encoding"); !ok || encoding != "" && !strings.EqualFold(encoding, "identity") {
		return reject(ctx, 415, "content_encoding_unsupported")
	}
	if !ctx.HasRequestBody() {
		return reject(ctx, 400, "body_required")
	}
	limit := maxRequestBodyBytes
	if config.consumeBodyBasis {
		limit = maxRuntimeBodyBytes
	}
	ctx.SetRequestBodyBufferLimit(uint32(limit))
	if !wrapper.RequireRequestBody(ctx) {
		return reject(ctx, 500, "body_context_unavailable")
	}
	// Do not let downstream header callbacks run before the original basis exists.
	return types.HeaderStopIteration
}

func onHttpRequestBody(ctx wrapper.HttpContext, config Config, body []byte) types.Action {
	if ctx.GetBoolContext(rejectedContextKey, false) {
		return types.ActionPause
	}
	if ctx.GetBoolContext(mappedContextKey, false) {
		return types.ActionContinue
	}
	limit := maxRequestBodyBytes
	if config.consumeBodyBasis {
		limit = maxRuntimeBodyBytes
	}
	if len(body) > limit {
		return reject(ctx, 413, "body_too_large")
	}
	var basis bodybasis.Snapshot
	if config.consumeBodyBasis {
		var ok bool
		basis, ok = ctx.GetContext(basisContextKey).(bodybasis.Snapshot)
		if !ok {
			return reject(ctx, 400, "original_body_basis_missing")
		}
		// Validate rewritten JSON without capturing it as the original body.
		if err := mfjson.ValidateRuntimeBody(body, maxRuntimeBodyBytes); err != nil {
			return reject(ctx, 400, "invalid_model_body")
		}
		var object map[string]json.RawMessage
		var model string
		if json.Unmarshal(body, &object) != nil || json.Unmarshal(object["model"], &model) != nil || model != basis.PublicModel() {
			return reject(ctx, 400, "body_basis_model_differs")
		}
	} else {
		var err error
		basis, err = bodybasis.Capture(body)
		if err != nil {
			return reject(ctx, 400, "invalid_model_body")
		}
	}
	target, found := config.modelMapping[basis.PublicModel()]
	if !found {
		return reject(ctx, 422, "model_mapping_missing")
	}
	rewritten, err := sjson.SetBytes(body, "model", target)
	if err != nil {
		return reject(ctx, 500, "model_rewrite_failed")
	}
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return reject(ctx, 500, "header_read_failed")
	}
	headers = withoutBasis(headers, true)
	if config.captureBodyBasis || config.consumeBodyBasis {
		headers = append(headers, basis.Headers()...)
	}
	if err := proxywasm.ReplaceHttpRequestBody(rewritten); err != nil {
		return reject(ctx, 500, "body_replace_failed")
	}
	if err := proxywasm.ReplaceHttpRequestHeaders(headers); err != nil {
		return reject(ctx, 500, "basis_write_failed")
	}
	ctx.SetContext(mappedContextKey, true)
	return types.ActionContinue
}

func withoutBasis(headers [][2]string, removeLength bool) [][2]string {
	filtered := make([][2]string, 0, len(headers))
	for _, header := range headers {
		isBasis := strings.EqualFold(header[0], bodybasis.DigestHeader) ||
			strings.EqualFold(header[0], bodybasis.PublicModelHeader)
		if isBasis || removeLength && strings.EqualFold(header[0], "content-length") {
			continue
		}
		filtered = append(filtered, header)
	}
	return filtered
}

// Missing optional headers return an empty value. Duplicates are ambiguous.
func singleHeader(headers [][2]string, name string) (string, bool) {
	var value string
	var count int
	for _, header := range headers {
		if strings.EqualFold(header[0], name) {
			value = header[1]
			count++
		}
	}
	return value, count <= 1
}

func reject(ctx wrapper.HttpContext, status uint32, code string) types.Action {
	ctx.SetContext(rejectedContextKey, true)
	ctx.DontReadRequestBody()
	response := []byte(`{"error":{"code":"` + code + `","type":"model_mapper_error"}}`)
	if err := proxywasm.SendHttpResponse(status, [][2]string{{"content-type", "application/json"}}, response, -1); err != nil {
		proxywasm.LogCritical("model-mapper could not send local rejection")
	}
	return types.ActionPause
}
