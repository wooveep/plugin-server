package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
)

type fuzzDeterministicReader struct {
	mu  sync.Mutex
	rng *rand.ChaCha8
}

func newFuzzDeterministicReader() *fuzzDeterministicReader {
	var seed [32]byte
	copy(seed[:], "canonical-fuzz-entropy-seed-v1!!")
	return &fuzzDeterministicReader{rng: rand.NewChaCha8(seed)}
}

func (r *fuzzDeterministicReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	n, err := r.rng.Read(p)
	r.mu.Unlock()
	return n, err
}

func assertSafeCanonicalError(t *testing.T, err error, secretProbe string) {
	t.Helper()
	if err == nil {
		return
	}
	var rej canonical.RejectionError
	if !errors.As(err, &rej) {
		return
	}
	code := rej.SafeCode()
	if code == "" || len(code) > 64 || canonical.SafeIdentifier(code) != code {
		t.Fatalf("unsafe RejectionError.SafeCode(): %q", code)
	}
	field := rej.SafeField()
	if field != "" && (len(field) > 64 || canonical.SafeIdentifier(field) != field) {
		t.Fatalf("unsafe RejectionError.SafeField(): %q", field)
	}
	detail := rej.SafeDetail()
	if !strings.HasPrefix(detail, "code=") {
		t.Fatalf("unexpected SafeDetail prefix: %q", detail)
	}
	if secretProbe != "" && strings.Contains(detail, secretProbe) {
		t.Fatalf("SafeDetail leaked secret probe %q in %q", secretProbe, detail)
	}
}

func representativeFuzzTargets() []CanonicalTarget {
	qwenCompat := CanonicalTarget{
		WireFormat:               WireChatCompletions,
		Supplier:                 "qwen",
		Model:                    "qwen-plus",
		APIVersion:               "v1",
		Streaming:                true,
		AcceptedLimitScopes:      LimitScopeMaskAll,
		SupportsCompletionTokens: true,
		SupportsThinking:         true,
		SupportsAdaptiveThinking: true,
		SupportsPreserveThinking: true,
		SupportsStrictSchema:     true,
		SupportsURLImages:        true,
		AllowedImageDetails: []canonical.ImageDetail{
			canonical.ImageDetailAuto,
			canonical.ImageDetailLow,
			canonical.ImageDetailHigh,
		},
	}
	qwenCompatibleChatPolicy("qwen-plus").applyToTarget(&qwenCompat)

	return []CanonicalTarget{
		{
			WireFormat:               WireChatCompletions,
			Supplier:                 "openai",
			Model:                    "gpt-4o",
			APIVersion:               "v1",
			Streaming:                false,
			SupportsDeveloperRole:    true,
			AcceptedLimitScopes:      LimitScopeMaskAll,
			SupportsCompletionTokens: true,
			SupportsThinking:         true,
			SupportsStrictSchema:     true,
			SupportsURLImages:        true,
			SupportsStore:            true,
			SupportsPromptCacheKey:   true,
			AllowedImageDetails: []canonical.ImageDetail{
				canonical.ImageDetailAuto,
				canonical.ImageDetailLow,
				canonical.ImageDetailHigh,
				canonical.ImageDetailOriginal,
			},
		},
		qwenCompat,
		{
			WireFormat:               WireDashScopeText,
			Supplier:                 "qwen",
			Model:                    "qwen-plus",
			APIVersion:               "2024-05-15",
			Streaming:                false,
			AcceptedLimitScopes:      LimitScopeMaskAll,
			SupportsCompletionTokens: false,
			SupportsThinking:         true,
			SupportsAdaptiveThinking: true,
			SupportsPreserveThinking: true,
			ForbidsForcedToolThink:   true,
			IncrementalOutputMode:    IncrementalAuto,
			SupportsStrictSchema:     true,
		},
		{
			WireFormat:               WireDashScopeText,
			Supplier:                 "qwen",
			Model:                    "qwen-flash",
			APIVersion:               "2024-05-15",
			Streaming:                true,
			AcceptedLimitScopes:      LimitScopeMaskAll,
			SupportsThinking:         true,
			SupportsAdaptiveThinking: true,
			ForbidsForcedToolThink:   true,
			IncrementalOutputMode:    IncrementalForceTrue,
			SupportsStrictSchema:     true,
		},
		{
			WireFormat:                  WireDashScopeMultimodal,
			Supplier:                    "qwen",
			Model:                       "qwen3.6-plus",
			APIVersion:                  "2024-05-15",
			Streaming:                   true,
			AcceptedLimitScopes:         LimitScopeMaskAll,
			SupportsThinking:            true,
			SupportsAdaptiveThinking:    true,
			ForbidsForcedToolThink:      true,
			IncrementalOutputMode:       IncrementalAuto,
			SupportsURLImages:           true,
			SupportsContentCacheControl: true,
			AllowedImageDetails: []canonical.ImageDetail{
				canonical.ImageDetailAuto,
				canonical.ImageDetailLow,
				canonical.ImageDetailHigh,
			},
		},
		{
			WireFormat:           WireGeminiGenerateContent,
			Supplier:             "gemini",
			Model:                "gemini-2.5-pro",
			APIVersion:           "v1beta",
			Streaming:            true,
			AcceptedLimitScopes:  LimitScopeMaskAll,
			SupportsThinking:     true,
			SupportsStrictSchema: true,
			SupportsURLImages:    false,
			GeminiThinkingBudget: 1024,
			GeminiSafetySettings: map[string]string{"HARM_CATEGORY_HATE_SPEECH": "BLOCK_ONLY_HIGH"},
			AllowedImageDetails:  []canonical.ImageDetail{canonical.ImageDetailAuto, canonical.ImageDetailLow, canonical.ImageDetailHigh},
		},
	}
}

func seedCanonicalFuzzCorpus(f *testing.F) {
	seeds := [][]byte{
		// Malformed / edge JSON boundaries
		[]byte(``),
		[]byte(`null`),
		[]byte(`[]`),
		[]byte(`{}`),
		[]byte(`{"model":null}`),
		[]byte(`{"model":"m","stream":null,"messages":[{"role":"user","content":"hi"}]}`),
		append([]byte(`{"model":"m","input":"bad-utf8-`), 0xff, 0xfe, '"', '}'),
		[]byte(`{"model":"m","messages":[{"role":"user","content":"\ud800"}]}`),

		// OpenAI Chat: zero scalars, single-string stop, >4 stop, partial assistant, tool null content, 9007199254740993
		[]byte(`{"model":"qwen-plus","temperature":0,"top_p":0,"presence_penalty":0,"frequency_penalty":0,"seed":0,"stop":"\n","max_completion_tokens":64,"enable_search":false,"messages":[{"role":"developer","content":"dev"},{"role":"system","content":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral"}}]},{"role":"user","content":"calc"},{"role":"assistant","content":null,"reasoning_content":"think","tool_calls":[{"id":"call_1","type":"function","function":{"name":"f","arguments":"{\"n\":9007199254740993}"}}]},{"role":"tool","tool_call_id":"call_1","content":null},{"role":"assistant","content":"prefix","partial":true}],"tools":[{"type":"function","function":{"name":"f","strict":true,"parameters":{"type":"object","properties":{"n":{"type":"integer","enum":[9007199254740993]}}}}}],"tool_choice":"auto","response_format":{"type":"json_schema","json_schema":{"name":"out","strict":true,"schema":{"type":"object"}}}}`),
		[]byte(`{"model":"m","stop":["a","b","c","d","e"],"messages":[{"role":"user","content":"too many stops"}]}`),
		[]byte(`{"model":"m","max_tokens":0,"messages":[{"role":"user","content":"zero limit"}]}`),

		// OpenAI Responses: valid multi-turn, Data URL image, detail:original, H1 prompt reject vs null, opaque reasoning skip vs bare reject
		[]byte(`{"model":"qwen-plus","instructions":"inst","stream":true,"temperature":0,"max_output_tokens":128,"store":false,"truncation":"disabled","include":["reasoning.encrypted_content"],"service_tier":"default","top_logprobs":2,"moderation":"auto","prompt":null,"previous_response_id":null,"conversation":"","tool_choice":null,"reasoning":{"effort":"medium","summary":"auto"},"input":[{"type":"message","role":"developer","content":"rule"},{"type":"message","role":"user","content":[{"type":"input_text","text":"see image"},{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KGgo=","detail":"high"}]},{"type":"reasoning","id":"rs_1","summary":[],"encrypted_content":"gAAAAABopaque"},{"type":"message","role":"assistant","content":[]},{"type":"function_call","id":"fc_1","call_id":"c_1","name":"op","arguments":"{\"id\":9007199254740993}"},{"type":"function_call_output","call_id":"c_1","output":[]}],"tools":[{"type":"function","name":"op","strict":false,"parameters":{"type":"object"}}]}`),
		[]byte(`{"model":"qwen-plus","input":"hello","prompt":{"id":"pmpt_secret_probe_value"}}`),
		[]byte(`{"model":"qwen-plus","input":"hello","previous_response_id":"resp_secret_probe_value"}`),
		[]byte(`{"model":"qwen-plus","input":"hello","background":true}`),
		[]byte(`{"model":"qwen-plus","input":[{"type":"reasoning"},{"type":"message","role":"user","content":"bare reasoning"}]}`),
		[]byte(`{"model":"qwen-plus","input":[{"role":"user","content":[{"type":"input_image","image_url":"https://example.com/a.png","detail":"original"}]}]}`),
		[]byte(`{"model":"qwen-plus","input":[{"role":"user","content":[{"type":"input_image","file_id":"file_123"}]}]}`),
		[]byte(`{"model":"qwen-plus","input":"hi","tools":[{"type":"tool_search"}]}`),

		// Anthropic Messages: 2023-06-01 adaptive+effort, cch= strip, cache_control order, H2 bash tool reject & multimodal tool_result
		[]byte(`{"model":"claude-sonnet-4-5","max_tokens":256,"temperature":0,"top_p":0,"thinking":{"type":"adaptive"},"output_config":{"effort":"max"},"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=1.0; cch=deadbeef;\nSystem prompt","cache_control":{"type":"ephemeral","ttl":"1h"}}],"messages":[{"role":"user","content":[{"type":"text","text":"run","cache_control":{"type":"ephemeral","ttl":"5m"}}]},{"role":"assistant","content":[{"type":"thinking","thinking":"step 1","signature":"sig_abc"},{"type":"redacted_thinking","data":"redacted_blob"},{"type":"tool_use","id":"toolu_1","name":"inspect","input":{"big":9007199254740993}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","is_error":true,"content":[{"type":"text","text":"screenshot"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgo="}}]},{"type":"text","text":"follow up"}]}],"tools":[{"type":"custom","name":"inspect","input_schema":{"type":"object"},"cache_control":{"type":"ephemeral","ttl":"1h"}}],"tool_choice":{"type":"auto"}}`),
		[]byte(`{"model":"claude","max_tokens":0,"messages":[{"role":"user","content":"warmup"}]}`),
		[]byte(`{"model":"claude","max_tokens":128,"messages":[{"role":"user","content":"hi"}],"tools":[{"type":"bash_20250124","name":"bash"}]}`),
		[]byte(`{"model":"claude","max_tokens":128,"thinking":{"type":"enabled","budget_tokens":1024},"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"f","input_schema":{"type":"object"}}],"tool_choice":{"type":"any"}}`),
		[]byte(`{"model":"claude","max_tokens":128,"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"bad","cache_control":{"type":"ephemeral"}}]}]}`),

		// Supplier Responses & SSE Event Payloads (Chat, DashScope, Gemini)
		[]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1763553600,"model":"qwen-plus","choices":[{"index":0,"message":{"role":"assistant","content":"ok","reasoning_content":"thought","tool_calls":[{"id":"call_1","type":"function","function":{"name":"f","arguments":"{\"x\":9007199254740993}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`),
		[]byte(`{"request_id":"req-ds-1","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","reasoning_content":"ds thought","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"f","arguments":"{\"k\":\"v\"}"}}]}}]},"usage":{"input_tokens":20,"output_tokens":8,"total_tokens":28}}`),
		[]byte(`{"request_id":"req-ds-2","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"delta","tool_calls":[{"index":0,"type":"function","function":{}}]}}]}}`),
		[]byte(`{"responseId":"gem-1","candidates":[{"index":0,"finishReason":"STOP","content":{"role":"model","parts":[{"text":"gem thought","thought":true,"thoughtSignature":"sig_g1"},{"functionCall":{"id":"gc_1","name":"f","args":{"big":9007199254740993}}}]}}],"usageMetadata":{"promptTokenCount":50,"cachedContentTokenCount":10,"candidatesTokenCount":12,"thoughtsTokenCount":8,"totalTokenCount":70}}`),
		[]byte(`{"promptFeedback":{"blockReason":"SAFETY"}}`),
	}
	for _, s := range seeds {
		f.Add(s)
	}
}

func FuzzCanonicalRequestBoundary(f *testing.F) {
	seedCanonicalFuzzCorpus(f)

	restoreEntropy := clientresponse.SetIdentityEntropySource(newFuzzDeterministicReader())
	f.Cleanup(restoreEntropy)

	protocols := []canonical.Protocol{
		canonical.ProtocolOpenAIChat,
		canonical.ProtocolOpenAIResponses,
		canonical.ProtocolAnthropicMessages,
	}
	targets := representativeFuzzTargets()
	defaultTools := []fixedtools.Tool{fixedtools.WebSearch}
	removedTools := map[fixedtools.Tool]bool{fixedtools.WebSearch: true}

	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<20 {
			return
		}
		const secretProbe = "secret_probe_value"

		// 1. Client request decode -> fixedtools inspect/filter -> target validate -> supplier encode.
		for _, proto := range protocols {
			req, err := DecodeCanonicalRequest(proto, raw)
			if err != nil {
				assertSafeCanonicalError(t, err, secretProbe)
				continue
			}

			snapshot := req.CloneForAttempt()
			if _, ftErr := fixedtools.InspectCanonical(req, defaultTools); ftErr != nil {
				assertSafeCanonicalError(t, ftErr, secretProbe)
			}
			filtered := fixedtools.FilterCanonical(req, defaultTools, removedTools)
			if !reflect.DeepEqual(req, snapshot) {
				t.Fatalf("InspectCanonical/FilterCanonical mutated base Request for %s", proto)
			}

			for _, target := range targets {
				attemptReq := filtered.CloneForAttempt()
				attemptReq.Model = target.Model
				if vErr := ValidateCanonicalTarget(attemptReq, target); vErr != nil {
					assertSafeCanonicalError(t, vErr, secretProbe)
					continue
				}
				encoded, eErr := EncodeCanonicalRequest(attemptReq, target)
				if eErr != nil {
					assertSafeCanonicalError(t, eErr, secretProbe)
					continue
				}
				if !json.Valid(encoded) {
					t.Fatalf("EncodeCanonicalRequest(%s -> %s) produced invalid JSON: %s", proto, target.WireFormat, string(encoded))
				}
			}

			if !reflect.DeepEqual(req, snapshot) {
				t.Fatalf("target validation/encoding mutated base Request for %s", proto)
			}
		}

		// 2. Supplier non-streaming response decode -> typed client projection -> client response writer.
		for _, target := range targets {
			resp, err := DecodeCanonicalResponse(target, raw)
			if err != nil {
				assertSafeCanonicalError(t, err, secretProbe)
				continue
			}
			usage := clientresponse.ReadSupplierUsage(raw, string(target.WireFormat), false)
			for _, outProto := range protocols {
				policy := clientresponse.Policy{
					Protocol:      string(outProto),
					Profile:       clientresponse.Qwen,
					PublicModel:   "public-fuzz-model",
					UpstreamModel: target.Model,
					RequestID:     "req-fuzz-1",
					CreatedAt:     1763553600,
					Aliases:       &clientresponse.ResourceAliases{},
				}
				projected, pErr := clientresponse.ProjectCanonical(policy, resp, usage)
				if pErr != nil {
					continue
				}
				clientBytes, wErr := EncodeCanonicalClientResponse(projected)
				if wErr == nil && !json.Valid(clientBytes) {
					t.Fatalf("EncodeCanonicalClientResponse(%s) produced invalid JSON: %s", outProto, string(clientBytes))
				}
			}
		}

		// 3. Supplier per-attempt streaming event decoder state isolation & zero-panic check.
		for _, target := range targets {
			dec, err := NewCanonicalEventDecoder(target)
			if err != nil {
				assertSafeCanonicalError(t, err, secretProbe)
				continue
			}
			_ = dec.Decode("", raw, func(_ canonical.Event) error { return nil })
			if nl := bytes.IndexByte(raw, '\n'); nl >= 0 {
				decSplit, sErr := NewCanonicalEventDecoder(target)
				if sErr == nil {
					_ = decSplit.Decode("result", raw[:nl], func(_ canonical.Event) error { return nil })
					_ = decSplit.Decode("result", raw[nl:], func(_ canonical.Event) error { return nil })
					_ = decSplit.End()
				}
			}
			_ = dec.End()
		}
	})
}
