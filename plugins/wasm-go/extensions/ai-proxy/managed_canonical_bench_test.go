package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"math/rand/v2"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

const (
	benchCallID        = "50000000-0000-0000-0000-000000000099"
	benchBusinessUTC   = "2026-11-19T12:00:00Z"
	benchUpstreamReqID = "a8a7295d-0699-991f-849a-97f767160077"
	benchChatStreamID  = "chatcmpl-bench-stream-01"
	benchPublicModel   = "public-model"
)

type benchDeterministicRand struct {
	mu  sync.Mutex
	rng *rand.ChaCha8
}

func newBenchDeterministicRand() *benchDeterministicRand {
	var seed [32]byte
	copy(seed[:], "canonical-ir-bench-deterministic")
	return &benchDeterministicRand{rng: rand.NewChaCha8(seed)}
}

func (r *benchDeterministicRand) Read(p []byte) (int, error) {
	r.mu.Lock()
	n, err := r.rng.Read(p)
	r.mu.Unlock()
	return n, err
}

type benchPriceRedis struct{}

func (benchPriceRedis) Command(_ []interface{}, callback wrapper.RedisResponseCallback) error {
	callback(resp.StringValue("web_search"))
	return nil
}

type benchProviderConfig struct {
	Type                 string   `json:"type"`
	APITokens            []string `json:"apiTokens"`
	QwenEnableCompatible bool     `json:"qwenEnableCompatible"`
	QwenEnableSearch     bool     `json:"qwenEnableSearch"`
}

type benchPluginConfigEnvelope struct {
	Provider     benchProviderConfig `json:"provider"`
	ProtocolMode string              `json:"protocol_mode"`
	PathScope    string              `json:"path_scope"`
	Managed      managedSettings     `json:"managed_execution"`
	Release      managedRelease      `json:"_modelfusion_execution"`
}

type benchRequestCase struct {
	name             string
	inputProtocol    string
	upstreamProtocol string
	upstreamPath     string
	authority        string
	upstreamModel    string
	compatible       bool
	publicBody       []byte
	mappedBody       []byte
	basis            bodybasis.Snapshot
	check            func(b *testing.B, outbound []byte, state *managedExecutionState)
}

type benchStreamCase struct {
	name             string
	inputProtocol    string
	upstreamProtocol string
	upstreamPath     string
	authority        string
	upstreamModel    string
	compatible       bool
	incremental      bool
	publicRequest    []byte
	chunks           [][]byte
	totalBytes       int
	eventCount       int
	check            func(b *testing.B, projected []byte, observed managedProtocolObservation)
}

var (
	benchFixturesOnce sync.Once
	benchRequestCases []benchRequestCase
	benchStreamCases  []benchStreamCase
)

func initBenchFixtures(b *testing.B) {
	b.Helper()
	benchFixturesOnce.Do(func() {
		text1KiB := makeBenchPromptText(1024)
		text32KiB := makeBenchPromptText(32 * 1024)
		b64Image2MiB := makeBenchRandomBase64(0x2026111902, 2*1024*1024)
		b64Image8MiB := makeBenchRandomBase64(0x2026111908, 8*1024*1024)

		resp1KiBPub := fmt.Sprintf(`{"model":%q,"stream":true,"instructions":"Follow user constraints strictly.","max_output_tokens":256,"temperature":0,"enable_search":false,"reasoning":{"effort":"low","summary":"auto"},"truncation":"disabled","include":["reasoning.encrypted_content"],"input":[{"type":"message","role":"developer","content":"Use structured facts."},{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]}],"tools":[{"type":"function","name":"lookup_metric","description":"Query metric","strict":true,"parameters":{"type":"object","properties":{"metric":{"type":"string"}},"required":["metric"],"additionalProperties":false}}],"tool_choice":"auto"}`, benchPublicModel, text1KiB)

		resp32KiBPub := fmt.Sprintf(`{"model":%q,"stream":true,"instructions":"Summarize long context accurately.","max_output_tokens":512,"temperature":0.2,"enable_search":false,"input":[{"type":"message","role":"developer","content":"Preserve exact identifiers."},{"type":"message","role":"user","content":[{"type":"input_text","text":%q}]}],"tools":[{"type":"function","name":"save_digest","description":"Persist digest","parameters":{"type":"object","properties":{"summary":{"type":"string"}}}}],"tool_choice":"auto"}`, benchPublicModel, text32KiB)

		msg2MiBPub := fmt.Sprintf(`{"model":%q,"stream":true,"max_tokens":512,"thinking":{"type":"adaptive"},"output_config":{"effort":"medium"},"system":[{"type":"text","text":"Inspect diagram details.","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":[{"type":"text","text":"Describe all components in this image."},{"type":"image","source":{"type":"base64","media_type":"image/png","data":%q}}]}],"tools":[{"name":"tag_region","description":"Annotate region","input_schema":{"type":"object","properties":{"label":{"type":"string"}}}}],"tool_choice":{"type":"auto"}}`, benchPublicModel, b64Image2MiB)

		dataURL8MiB := "data:image/png;base64," + b64Image8MiB
		resp8MiBPub := fmt.Sprintf(`{"model":%q,"stream":false,"instructions":"Inspect high-res scan.","max_output_tokens":256,"enable_search":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Extract table values."},{"type":"input_image","image_url":%q,"detail":"high"}]}]}`, benchPublicModel, dataURL8MiB)

		respMultiTurnPub := fmt.Sprintf(`{"model":%q,"stream":true,"instructions":"Execute parallel tools accurately.","max_output_tokens":512,"temperature":0,"store":false,"enable_search":false,"text":{"format":{"type":"json_schema","name":"audit_report","strict":true,"schema":{"type":"object","properties":{"ledger_seq":{"type":"integer","enum":[9007199254740993]},"ok":{"type":"boolean"}},"required":["ledger_seq","ok"],"additionalProperties":false}}},"input":[{"type":"message","role":"developer","content":"Never round 64-bit IDs."},{"type":"message","role":"user","content":[{"type":"input_text","text":"Verify account 9007199254740993 and region hz."}]},{"type":"reasoning","id":"rs_prev_1","summary":[],"encrypted_content":"gAAAAABopaque_skip"},{"type":"reasoning","id":"rs_prev_2","content":[{"type":"reasoning_text","text":"Checking ledger and region in parallel."}]},{"type":"function_call","id":"fc_1","call_id":"call_p1","name":"query_ledger","arguments":"{\"account_id\":9007199254740993,\"shard\":\"hz\"}"},{"type":"function_call","id":"fc_2","call_id":"call_p2","name":"query_region","arguments":"{\"region\":\"cn-hangzhou\"}"},{"type":"function_call_output","call_id":"call_p1","output":"{\"balance\":\"100.50\",\"seq\":9007199254740993}"},{"type":"function_call_output","call_id":"call_p2","output":"{\"healthy\":true}"}],"tools":[{"type":"function","name":"query_ledger","description":"Query ledger","strict":true,"parameters":{"type":"object","properties":{"account_id":{"type":"integer"},"shard":{"type":"string"}},"required":["account_id","shard"]}},{"type":"function","name":"query_region","description":"Query region","parameters":{"type":"object","properties":{"region":{"type":"string"}}}}],"tool_choice":"auto","parallel_tool_calls":true}`, benchPublicModel)

		chatNative1KiBPub := fmt.Sprintf(`{"model":%q,"stream":true,"stream_options":{"include_usage":true},"temperature":0,"max_completion_tokens":256,"enable_search":false,"messages":[{"role":"system","content":[{"type":"text","text":"Be concise.","cache_control":{"type":"ephemeral"}}]},{"role":"user","content":%q}],"tools":[{"type":"function","function":{"name":"lookup_metric","description":"Query metric","parameters":{"type":"object","properties":{"metric":{"type":"string"}}}}}],"tool_choice":"auto"}`, benchPublicModel, text1KiB)

		rawCases := []struct {
			name             string
			inputProtocol    string
			upstreamProtocol string
			upstreamPath     string
			authority        string
			upstreamModel    string
			compatible       bool
			publicJSON       string
			check            func(b *testing.B, outbound []byte, state *managedExecutionState)
		}{
			{
				name:             "ResponsesToDashScopeText_1KiB",
				inputProtocol:    managedProtocolResponses,
				upstreamProtocol: managedProtocolDashScopeText,
				upstreamPath:     "/api/v1/services/aigc/text-generation/generation",
				authority:        "dashscope.aliyuncs.com",
				upstreamModel:    "qwen-flash",
				compatible:       false,
				publicJSON:       resp1KiBPub,
				check: func(b *testing.B, outbound []byte, _ *managedExecutionState) {
					if gjson.GetBytes(outbound, "model").String() != "qwen-flash" {
						b.Fatalf("unexpected model: %s", gjson.GetBytes(outbound, "model").String())
					}
					if gjson.GetBytes(outbound, "parameters.temperature").Raw != "0" {
						b.Fatalf("explicit temperature 0 lost: %s", gjson.GetBytes(outbound, "parameters.temperature").Raw)
					}
					if gjson.GetBytes(outbound, "parameters.enable_search").Bool() {
						b.Fatal("removed default web_search was re-enabled")
					}
					if !gjson.GetBytes(outbound, "parameters.incremental_output").Bool() {
						b.Fatal("expected incremental_output=true when streaming with thinking")
					}
				},
			},
			{
				name:             "ResponsesToDashScopeText_32KiB",
				inputProtocol:    managedProtocolResponses,
				upstreamProtocol: managedProtocolDashScopeText,
				upstreamPath:     "/api/v1/services/aigc/text-generation/generation",
				authority:        "dashscope.aliyuncs.com",
				upstreamModel:    "qwen-plus",
				compatible:       false,
				publicJSON:       resp32KiBPub,
				check: func(b *testing.B, outbound []byte, _ *managedExecutionState) {
					if gjson.GetBytes(outbound, "model").String() != "qwen-plus" {
						b.Fatalf("unexpected model: %s", gjson.GetBytes(outbound, "model").String())
					}
					if len(gjson.GetBytes(outbound, "input.messages.2.content").String()) < 32*1024 {
						b.Fatal("32KiB prompt content truncated")
					}
				},
			},
			{
				name:             "MessagesToDashScopeMultimodal_2MiBImage",
				inputProtocol:    managedProtocolMessages,
				upstreamProtocol: managedProtocolDashScopeMultimodal,
				upstreamPath:     "/api/v1/services/aigc/multimodal-generation/generation",
				authority:        "dashscope.aliyuncs.com",
				upstreamModel:    "qwen3.6-plus",
				compatible:       false,
				publicJSON:       msg2MiBPub,
				check: func(b *testing.B, outbound []byte, state *managedExecutionState) {
					if gjson.GetBytes(outbound, "model").String() != "qwen3.6-plus" {
						b.Fatalf("unexpected model: %s", gjson.GetBytes(outbound, "model").String())
					}
					if !gjson.GetBytes(outbound, "parameters.enable_thinking").Bool() {
						b.Fatal("expected adaptive thinking to enable DashScope thinking")
					}
					img := gjson.GetBytes(outbound, "input.messages.1.content.1.image").String()
					if !strings.HasPrefix(img, "data:image/png;base64,") || len(img) < len(b64Image2MiB) {
						b.Fatalf("2MiB Data URL image corrupted, len=%d", len(img))
					}
					if state.cacheMode != managedCacheExplicit {
						b.Fatalf("expected explicit cache mode from system cache_control, got %q", state.cacheMode)
					}
				},
			},
			{
				name:             "ResponsesToDashScopeMultimodal_8MiBImage",
				inputProtocol:    managedProtocolResponses,
				upstreamProtocol: managedProtocolDashScopeMultimodal,
				upstreamPath:     "/api/v1/services/aigc/multimodal-generation/generation",
				authority:        "dashscope.aliyuncs.com",
				upstreamModel:    "qwen3.6-plus",
				compatible:       false,
				publicJSON:       resp8MiBPub,
				check: func(b *testing.B, outbound []byte, _ *managedExecutionState) {
					img := gjson.GetBytes(outbound, "input.messages.1.content.1.image").String()
					if img != dataURL8MiB {
						b.Fatalf("8MiB Data URL image mismatch, got len=%d want len=%d", len(img), len(dataURL8MiB))
					}
				},
			},
			{
				name:             "ResponsesToChat_MultiTurnParallelFunctions",
				inputProtocol:    managedProtocolResponses,
				upstreamProtocol: managedProtocolChat,
				upstreamPath:     "/compatible-mode/v1/chat/completions",
				authority:        "dashscope.aliyuncs.com",
				upstreamModel:    "qwen-plus",
				compatible:       true,
				publicJSON:       respMultiTurnPub,
				check: func(b *testing.B, outbound []byte, _ *managedExecutionState) {
					if !bytes.Contains(outbound, []byte("9007199254740993")) {
						b.Fatal("64-bit integer 9007199254740993 lost precision")
					}
					if gjson.GetBytes(outbound, "store").Exists() {
						b.Fatal("store must be omitted for non-OpenAI compatible upstream")
					}
					if gjson.GetBytes(outbound, "messages.3.tool_calls.#").Int() != 2 {
						b.Fatalf("expected 2 parallel tool_calls at messages.3, got %s", gjson.GetBytes(outbound, "messages").Raw)
					}
				},
			},
			{
				name:             "ChatNativeSameProtocol_1KiB",
				inputProtocol:    managedProtocolChat,
				upstreamProtocol: managedProtocolChat,
				upstreamPath:     "/compatible-mode/v1/chat/completions",
				authority:        "dashscope.aliyuncs.com",
				upstreamModel:    "qwen-plus",
				compatible:       true,
				publicJSON:       chatNative1KiBPub,
				check: func(b *testing.B, outbound []byte, _ *managedExecutionState) {
					if gjson.GetBytes(outbound, "model").String() != "qwen-plus" {
						b.Fatalf("unexpected model: %s", gjson.GetBytes(outbound, "model").String())
					}
					if len(gjson.GetBytes(outbound, "messages.1.content").String()) < 1024 {
						b.Fatal("1KiB native Chat content truncated")
					}
				},
			},
		}

		benchRequestCases = make([]benchRequestCase, 0, len(rawCases))
		for _, rc := range rawCases {
			pub := []byte(rc.publicJSON)
			mapped := bytes.Replace(pub, []byte(`"`+benchPublicModel+`"`), []byte(`"`+rc.upstreamModel+`"`), 1)
			basis, err := bodybasis.Read([][2]string{
				{bodybasis.DigestHeader, bodybasis.DigestBytes(pub)},
				{bodybasis.PublicModelHeader, benchPublicModel},
			})
			if err != nil {
				b.Fatalf("bodybasis.Read failed for %s: %v", rc.name, err)
			}
			benchRequestCases = append(benchRequestCases, benchRequestCase{
				name:             rc.name,
				inputProtocol:    rc.inputProtocol,
				upstreamProtocol: rc.upstreamProtocol,
				upstreamPath:     rc.upstreamPath,
				authority:        rc.authority,
				upstreamModel:    rc.upstreamModel,
				compatible:       rc.compatible,
				publicBody:       pub,
				mappedBody:       mapped,
				basis:            basis,
				check:            rc.check,
			})
		}

		benchStreamCases = []benchStreamCase{
			makeDashScopeText64ChunksStreamCase(),
			makeDashScopeCumulativeParallelToolsStreamCase(),
			makeDashScopeIncrementalThinkingToolsMessagesCase(),
			makeChatToResponsesTextAndToolsStreamCase(),
		}
	})
}

func makeBenchPromptText(targetBytes int) string {
	const unit = "Modelfusion canonical IR benchmark segment verifying UTF-8, tool calls, and token accounting. "
	var sb strings.Builder
	sb.Grow(targetBytes + len(unit))
	for sb.Len() < targetBytes {
		sb.WriteString(unit)
	}
	return sb.String()[:targetBytes]
}

func makeBenchRandomBase64(seedVal uint64, rawByteLen int) string {
	var seed [32]byte
	for i := range 8 {
		seed[i] = byte(seedVal >> (i * 8))
	}
	rng := rand.NewChaCha8(seed)
	raw := make([]byte, rawByteLen)
	_, _ = rng.Read(raw)
	return base64.StdEncoding.EncodeToString(raw)
}

func makeDashScopeText64ChunksStreamCase() benchStreamCase {
	chunks := make([][]byte, 0, 65)
	totalBytes := 0
	for i := 1; i <= 64; i++ {
		frame := fmt.Sprintf(
			"id:%d\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"segment-%02d \"}}]},\"usage\":{\"input_tokens\":512,\"output_tokens\":%d,\"total_tokens\":%d}}\n\n",
			i, benchUpstreamReqID, i, i*4, 512+i*4,
		)
		raw := []byte(frame)
		chunks = append(chunks, raw)
		totalBytes += len(raw)
	}
	tail := fmt.Sprintf(
		"id:65\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"stop\",\"message\":{\"role\":\"assistant\",\"content\":\"\"}}]},\"usage\":{\"input_tokens\":512,\"output_tokens\":256,\"total_tokens\":768,\"prompt_tokens_details\":{\"cached_tokens\":128}}}\n\n",
		benchUpstreamReqID,
	)
	tailBytes := []byte(tail)
	chunks = append(chunks, tailBytes)
	totalBytes += len(tailBytes)

	return benchStreamCase{
		name:             "DashScopeTextToResponses_64Chunks",
		inputProtocol:    managedProtocolResponses,
		upstreamProtocol: managedProtocolDashScopeText,
		upstreamPath:     "/api/v1/services/aigc/text-generation/generation",
		authority:        "dashscope.aliyuncs.com",
		upstreamModel:    "qwen-flash",
		compatible:       false,
		incremental:      true,
		publicRequest:    []byte(`{"model":"public-model","stream":true,"input":"stream 64 segments"}`),
		chunks:           chunks,
		totalBytes:       totalBytes,
		eventCount:       len(chunks),
		check: func(b *testing.B, projected []byte, observed managedProtocolObservation) {
			text := string(projected)
			if !strings.Contains(text, `"type":"response.output_text.delta"`) || !strings.Contains(text, `"type":"response.completed"`) {
				b.Fatalf("missing Responses text delta or completed terminal: %s", text)
			}
			if !strings.Contains(text, "segment-01 ") || !strings.Contains(text, "segment-64 ") {
				b.Fatal("missing streamed text segments")
			}
			if observed.Usage.Status != "reported" || observed.Usage.Quantities["input"] != "512" || observed.Usage.Quantities["output"] != "256" || observed.Usage.Quantities["cache_hit"] != "128" {
				b.Fatalf("unexpected observed usage: %+v", observed.Usage)
			}
		},
	}
}

func makeDashScopeCumulativeParallelToolsStreamCase() benchStreamCase {
	rawFrames := []string{
		fmt.Sprintf("id:1\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"Analyzing ledger \"}}]},\"usage\":{\"input_tokens\":320,\"output_tokens\":5,\"total_tokens\":325}}\n\n", benchUpstreamReqID),
		fmt.Sprintf("id:2\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"Analyzing ledger and weather in parallel.\"}}]},\"usage\":{\"input_tokens\":320,\"output_tokens\":12,\"total_tokens\":332}}\n\n", benchUpstreamReqID),
		fmt.Sprintf("id:3\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"Analyzing ledger and weather in parallel.\",\"tool_calls\":[{\"index\":0,\"id\":\"call_hz_1\",\"type\":\"function\",\"function\":{\"name\":\"query_ledger\",\"arguments\":\"{\\\"id\\\":\"}},{\"index\":1,\"id\":\"call_hz_2\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\"}}]}}]},\"usage\":{\"input_tokens\":320,\"output_tokens\":24,\"total_tokens\":344}}\n\n", benchUpstreamReqID),
		fmt.Sprintf("id:4\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"Analyzing ledger and weather in parallel.\",\"tool_calls\":[{\"index\":0,\"id\":\"call_hz_1\",\"type\":\"function\",\"function\":{\"name\":\"query_ledger\",\"arguments\":\"{\\\"id\\\":9007199254740993}\"}},{\"index\":1,\"id\":\"call_hz_2\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\\\"Hangzhou\\\"}\"}}]}}]},\"usage\":{\"input_tokens\":320,\"output_tokens\":38,\"total_tokens\":358}}\n\n", benchUpstreamReqID),
		fmt.Sprintf("id:5\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"tool_calls\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"Analyzing ledger and weather in parallel.\",\"tool_calls\":[{\"index\":0,\"id\":\"call_hz_1\",\"type\":\"function\",\"function\":{\"name\":\"query_ledger\",\"arguments\":\"{\\\"id\\\":9007199254740993}\"}},{\"index\":1,\"id\":\"call_hz_2\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\\\"Hangzhou\\\"}\"}}]}}]},\"usage\":{\"input_tokens\":320,\"output_tokens\":38,\"total_tokens\":358,\"prompt_tokens_details\":{\"cached_tokens\":64},\"output_tokens_details\":{\"reasoning_tokens\":12}}}\n\n", benchUpstreamReqID),
	}
	chunks := make([][]byte, 0, len(rawFrames))
	totalBytes := 0
	for _, f := range rawFrames {
		b := []byte(f)
		chunks = append(chunks, b)
		totalBytes += len(b)
	}
	return benchStreamCase{
		name:             "DashScopeCumulativeParallelToolsReasoningToResponses",
		inputProtocol:    managedProtocolResponses,
		upstreamProtocol: managedProtocolDashScopeText,
		upstreamPath:     "/api/v1/services/aigc/text-generation/generation",
		authority:        "dashscope.aliyuncs.com",
		upstreamModel:    "qwen-plus",
		compatible:       false,
		incremental:      false,
		publicRequest:    []byte(`{"model":"public-model","stream":true,"input":"parallel tools"}`),
		chunks:           chunks,
		totalBytes:       totalBytes,
		eventCount:       len(chunks),
		check: func(b *testing.B, projected []byte, observed managedProtocolObservation) {
			text := string(projected)
			if !strings.Contains(text, `"type":"response.reasoning_text.delta"`) || !strings.Contains(text, `"type":"response.function_call_arguments.delta"`) || !strings.Contains(text, `"type":"response.completed"`) {
				b.Fatalf("missing expected Responses reasoning/tool/completed events: %s", text)
			}
			if !strings.Contains(text, "9007199254740993") || !strings.Contains(text, "Hangzhou") {
				b.Fatalf("missing parallel tool call arguments in output: %s", text)
			}
			if observed.Usage.Quantities["input"] != "320" || observed.Usage.Quantities["output"] != "38" || observed.Usage.Quantities["reasoning_tokens"] != "12" {
				b.Fatalf("unexpected observed usage: %+v", observed.Usage)
			}
		},
	}
}

func makeDashScopeIncrementalThinkingToolsMessagesCase() benchStreamCase {
	rawFrames := []string{
		fmt.Sprintf("id:1\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"Step 1: check city. \"}}]},\"usage\":{\"input_tokens\":140,\"output_tokens\":6,\"total_tokens\":146}}\n\n", benchUpstreamReqID),
		fmt.Sprintf("id:2\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"Step 2: invoke weather tool.\"}}]},\"usage\":{\"input_tokens\":140,\"output_tokens\":12,\"total_tokens\":152}}\n\n", benchUpstreamReqID),
		fmt.Sprintf("id:3\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"\",\"tool_calls\":[{\"index\":0,\"id\":\"call_hz_1\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"\"}}]}}]},\"usage\":{\"input_tokens\":140,\"output_tokens\":15,\"total_tokens\":155}}\n\n", benchUpstreamReqID),
		fmt.Sprintf("id:4\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"\",\"tool_calls\":[{\"index\":0,\"id\":\"call_hz_1\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\"}}]}}]},\"usage\":{\"input_tokens\":140,\"output_tokens\":20,\"total_tokens\":160}}\n\n", benchUpstreamReqID),
		fmt.Sprintf("id:5\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"\",\"tool_calls\":[{\"index\":0,\"id\":\"\",\"type\":\"function\",\"function\":{\"arguments\":\"\\\"Hangzhou\\\"}\"}}]}}]},\"usage\":{\"input_tokens\":140,\"output_tokens\":25,\"total_tokens\":165}}\n\n", benchUpstreamReqID),
		fmt.Sprintf("id:6\nevent:result\n:HTTP_STATUS/200\ndata:{\"request_id\":%q,\"output\":{\"choices\":[{\"finish_reason\":\"tool_calls\",\"message\":{\"role\":\"assistant\",\"content\":\"\",\"reasoning_content\":\"\",\"tool_calls\":[{\"index\":0,\"type\":\"function\",\"function\":{}}]}}]},\"usage\":{\"input_tokens\":140,\"output_tokens\":25,\"total_tokens\":165,\"prompt_tokens_details\":{\"cached_tokens\":32},\"output_tokens_details\":{\"reasoning_tokens\":12}}}\n\n", benchUpstreamReqID),
	}
	chunks := make([][]byte, 0, len(rawFrames))
	totalBytes := 0
	for _, f := range rawFrames {
		b := []byte(f)
		chunks = append(chunks, b)
		totalBytes += len(b)
	}
	return benchStreamCase{
		name:             "DashScopeIncrementalToolsThinkingToMessages",
		inputProtocol:    managedProtocolMessages,
		upstreamProtocol: managedProtocolDashScopeText,
		upstreamPath:     "/api/v1/services/aigc/text-generation/generation",
		authority:        "dashscope.aliyuncs.com",
		upstreamModel:    "qwen-plus",
		compatible:       false,
		incremental:      true,
		publicRequest:    []byte(`{"model":"public-model","stream":true,"max_tokens":256,"messages":[{"role":"user","content":"weather"}]}`),
		chunks:           chunks,
		totalBytes:       totalBytes,
		eventCount:       len(chunks),
		check: func(b *testing.B, projected []byte, observed managedProtocolObservation) {
			text := string(projected)
			if !strings.Contains(text, `"type":"thinking_delta"`) || !strings.Contains(text, `"type":"input_json_delta"`) || !strings.Contains(text, `"stop_reason":"tool_use"`) || !strings.Contains(text, "event: message_stop") {
				b.Fatalf("missing expected Messages thinking/tool/stop events: %s", text)
			}
			if observed.Usage.Quantities["input"] != "140" || observed.Usage.Quantities["output"] != "25" || observed.Usage.Quantities["cache_hit"] != "32" {
				b.Fatalf("unexpected observed usage: %+v", observed.Usage)
			}
		},
	}
}

func makeChatToResponsesTextAndToolsStreamCase() benchStreamCase {
	rawFrames := []string{
		fmt.Sprintf("data: {\"id\":%q,\"object\":\"chat.completion.chunk\",\"created\":1763553600,\"model\":\"qwen-plus\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Checking \"}}]}\n\n", benchChatStreamID),
		fmt.Sprintf("data: {\"id\":%q,\"object\":\"chat.completion.chunk\",\"created\":1763553600,\"model\":\"qwen-plus\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"both endpoints.\"}}]}\n\n", benchChatStreamID),
		fmt.Sprintf("data: {\"id\":%q,\"object\":\"chat.completion.chunk\",\"created\":1763553600,\"model\":\"qwen-plus\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_hz_1\",\"type\":\"function\",\"function\":{\"name\":\"ping_a\",\"arguments\":\"{\\\"id\\\":9007199254740993}\"}},{\"index\":1,\"id\":\"call_hz_2\",\"type\":\"function\",\"function\":{\"name\":\"ping_b\",\"arguments\":\"{\\\"ok\\\":true}\"}}]}}]}\n\n", benchChatStreamID),
		fmt.Sprintf("data: {\"id\":%q,\"object\":\"chat.completion.chunk\",\"created\":1763553600,\"model\":\"qwen-plus\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n", benchChatStreamID),
		fmt.Sprintf("data: {\"id\":%q,\"object\":\"chat.completion.chunk\",\"created\":1763553600,\"model\":\"qwen-plus\",\"choices\":[],\"usage\":{\"prompt_tokens\":9007199254740993,\"completion_tokens\":42,\"total_tokens\":9007199254741035}}\n\n", benchChatStreamID),
		"data: [DONE]\n\n",
	}
	chunks := make([][]byte, 0, len(rawFrames))
	totalBytes := 0
	for _, f := range rawFrames {
		b := []byte(f)
		chunks = append(chunks, b)
		totalBytes += len(b)
	}
	return benchStreamCase{
		name:             "ChatToResponses_TextAndTools",
		inputProtocol:    managedProtocolResponses,
		upstreamProtocol: managedProtocolChat,
		upstreamPath:     "/compatible-mode/v1/chat/completions",
		authority:        "dashscope.aliyuncs.com",
		upstreamModel:    "qwen-plus",
		compatible:       true,
		incremental:      true,
		publicRequest:    []byte(`{"model":"public-model","stream":true,"input":"check endpoints"}`),
		chunks:           chunks,
		totalBytes:       totalBytes,
		eventCount:       len(chunks),
		check: func(b *testing.B, projected []byte, observed managedProtocolObservation) {
			text := string(projected)
			if !strings.Contains(text, `"type":"response.output_text.delta"`) || !strings.Contains(text, `"type":"response.function_call_arguments.done"`) || !strings.Contains(text, `"type":"response.completed"`) {
				b.Fatalf("missing expected Chat->Responses events: %s", text)
			}
			if observed.Usage.Quantities["input"] != "9007199254740993" || observed.Usage.Quantities["output"] != "42" {
				b.Fatalf("64-bit token counter lost in Chat->Responses observation: %+v", observed.Usage)
			}
		},
	}
}

type benchHostEnv struct {
	host       proxytest.HostEmulator
	id         uint32
	ctx        wrapper.HttpContext
	managedCfg *managedExecutionConfig
}

func newBenchHostEnv(b *testing.B, inputProtocol, upstreamProtocol, upstreamPath, authority, upstreamModel string, compatible bool) *benchHostEnv {
	b.Helper()
	digest := func(ch string) string { return "sha256:" + strings.Repeat(ch, 64) }
	descriptor := managedDescriptor{
		ReleaseID:              "release-bench-1",
		TargetID:               "target-bench-1",
		BindingID:              "binding-bench-1",
		ProviderID:             "provider-bench-1",
		ProviderAccountID:      "purchase-bench-1",
		RegionEndpointRef:      "region-bench-1",
		ServiceRef:             "service-bench-1",
		ClusterRef:             "outbound|443||dashscope.static",
		CredentialPoolRef:      "pool-bench-1",
		AdapterRef:             "ai-proxy.native-text.v1:" + inputProtocol + ":" + upstreamProtocol,
		CredentialPoolRevision: "1",
		AdapterRevision:        "1",
		MapperDigest:           digest("a"),
		ProxyDigest:            digest("b"),
		UpstreamModel:          upstreamModel,
		ConfigCanonicalization: "mf-json.v1",
		Transport: managedTransport{
			Host:           authority,
			SNI:            authority,
			PathAdapterRef: "ai-proxy.native-text.v1:" + inputProtocol + ":" + upstreamProtocol,
		},
		Capabilities: map[string]managedCapability{
			"text_generate": {
				SendMode:              "single_send",
				ResponseClassifierRef: "openai-text-v1",
				MaxAttempts:           1,
				MaxFallbackHops:       0,
			},
		},
	}
	descRaw, err := json.Marshal(descriptor)
	if err != nil {
		b.Fatal(err)
	}
	descriptor.DescriptorDigest, err = managedDescriptorDigest(descRaw)
	if err != nil {
		b.Fatal(err)
	}

	streamProto := ""
	if inputProtocol != managedProtocolChat {
		streamProto = inputProtocol
	}
	envelope := benchPluginConfigEnvelope{
		Provider: benchProviderConfig{
			Type:                 "qwen",
			APITokens:            []string{"controlled-upstream-key"},
			QwenEnableCompatible: compatible,
			QwenEnableSearch:     false,
		},
		ProtocolMode: "auto",
		PathScope:    "bound_target",
		Managed: managedSettings{
			SendTransport:    "router",
			StreamProjection: true,
			StreamProtocol:   streamProto,
			RouterToolUsage:  true,
			ResponseProfile:  clientresponse.Qwen,
			ToolPolicy: &managedToolSettings{
				Allowed:   []fixedtools.Tool{fixedtools.WebSearch},
				Supported: []fixedtools.Tool{},
				Defaults:  []fixedtools.Tool{fixedtools.WebSearch},
				Supplier:  "qwen",
				PriceKey:  "modelfusion:tool-prices:asset-1",
			},
			ProducerID:         "managed-bench",
			CallIdentitySource: "admission",
			PrivateStream:      "private-execution-stream",
			Redis:              managedRedisSettings{ServiceName: "redis.static"},
			UpstreamPath:       upstreamPath,
			Authority:          authority,
			CredentialValue:    "controlled-upstream-key",
			InputProtocol:      inputProtocol,
			UpstreamProtocol:   upstreamProtocol,
		},
		Release: managedRelease{
			SchemaVersion:      "modelfusion-execution.v1",
			Descriptor:         descriptor,
			RouteID:            "route-bench-1",
			SalesAssetID:       "asset-1",
			PublicModel:        benchPublicModel,
			CredentialRef:      "credential-bench-1",
			CredentialRevision: "1",
		},
	}
	cfgBytes, err := json.Marshal(envelope)
	if err != nil {
		b.Fatal(err)
	}

	env := &benchHostEnv{}
	vm := wrapper.NewCommonVmCtx("ai-proxy",
		wrapper.ParseConfig(func(value gjson.Result, cfg *config.PluginConfig) error {
			cfg.FromJson(value)
			if err := cfg.Validate(); err != nil {
				return err
			}
			if err := cfg.Complete(); err != nil {
				return err
			}
			if err := configureManagedExecution(value, cfg); err != nil {
				return err
			}
			parsed, ok := cfg.ManagedExecution.(*managedExecutionConfig)
			if !ok || parsed == nil {
				return fmt.Errorf("missing managed execution config")
			}
			parsed.priceRedis = benchPriceRedis{}
			env.managedCfg = parsed
			return nil
		}),
		wrapper.ProcessRequestHeaders(func(ctx wrapper.HttpContext, cfg config.PluginConfig) types.Action {
			env.ctx = ctx
			action, _ := onManagedRequestHeaders(ctx, cfg)
			return action
		}),
	)

	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration(cfgBytes))
	b.Cleanup(reset)
	host.RegisterForeignFunction("declare_property", func([]byte) []byte { return []byte{0} })
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	if status := host.StartVM(); status != types.OnVMStartStatusOK {
		b.Fatalf("VM start failed: %v", status)
	}
	if status := host.StartPlugin(); status != types.OnPluginStartStatusOK {
		b.Fatalf("plugin start failed: %v", status)
	}
	env.host = host
	env.id = host.InitializeHttpContext()
	if err := host.SetProperty([]string{"cluster_name"}, []byte(descriptor.ClusterRef)); err != nil {
		b.Fatal(err)
	}
	reqHeaders := [][2]string{
		{":method", "POST"},
		{":path", "/v1/responses"},
		{":authority", "api.example.com"},
		{"content-type", "application/json"},
	}
	_ = host.CallOnRequestHeaders(env.id, reqHeaders, false)
	return env
}

func newBenchSeededAliases() *clientresponse.ResourceAliases {
	aliases := &clientresponse.ResourceAliases{}
	seeds := []clientresponse.ResourceAlias{
		{Kind: "response", Public: "resp_pf_00000000000000000000000000000001", Original: benchUpstreamReqID},
		{Kind: "response", Public: "resp_pf_00000000000000000000000000000002", Original: benchChatStreamID},
		{Kind: "response", Public: "resp_pf_00000000000000000000000000000003", Original: benchCallID},
		{Kind: "response", Public: "resp_pf_00000000000000000000000000000004", Original: ""},
		{Kind: "message", Public: "msg_pf_00000000000000000000000000000001", Original: benchUpstreamReqID},
		{Kind: "message", Public: "msg_pf_00000000000000000000000000000002", Original: benchChatStreamID},
		{Kind: "message", Public: "msg_pf_00000000000000000000000000000003", Original: benchCallID},
		{Kind: "message", Public: "msg_pf_00000000000000000000000000000004", Original: ""},
		{Kind: "chat", Public: "chatcmpl_pf_00000000000000000000000000000001", Original: benchUpstreamReqID},
		{Kind: "chat", Public: "chatcmpl_pf_00000000000000000000000000000002", Original: benchChatStreamID},
		{Kind: "chat", Public: "chatcmpl_pf_00000000000000000000000000000003", Original: benchCallID},
		{Kind: "chat", Public: "chatcmpl_pf_00000000000000000000000000000004", Original: ""},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000001", Original: "msg_0"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000002", Original: "msg_1"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000003", Original: "msg_" + benchUpstreamReqID},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000004", Original: "msg_" + benchChatStreamID},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000005", Original: "msg_" + benchUpstreamReqID + "_0"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000006", Original: "msg_" + benchUpstreamReqID + "_1"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000007", Original: "msg_" + benchChatStreamID + "_0"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000008", Original: "msg_" + benchChatStreamID + "_1"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000009", Original: "rs_" + benchUpstreamReqID},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000010", Original: "rs_" + benchChatStreamID},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000011", Original: "rs_" + benchUpstreamReqID + "_0"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000012", Original: "rs_" + benchUpstreamReqID + "_1"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000013", Original: "rs_" + benchChatStreamID + "_0"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000014", Original: "rs_" + benchChatStreamID + "_1"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000015", Original: "fc_call_hz_1"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000016", Original: "fc_call_hz_2"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000017", Original: "fc_" + benchUpstreamReqID + "_0"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000018", Original: "fc_" + benchUpstreamReqID + "_1"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000019", Original: "fc_" + benchChatStreamID + "_0"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000020", Original: "fc_" + benchChatStreamID + "_1"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000021", Original: "call_hz_1"},
		{Kind: "item", Public: "item_pf_00000000000000000000000000000022", Original: "call_hz_2"},
		{Kind: "call", Public: "call_pf_00000000000000000000000000000001", Original: "call_hz_1"},
		{Kind: "call", Public: "call_pf_00000000000000000000000000000002", Original: "call_hz_2"},
	}
	for saltIdx, salt := range []string{benchUpstreamReqID, benchChatStreamID, benchCallID} {
		for idx := range 8 {
			seq := (saltIdx+1)*100 + idx
			seeds = append(seeds,
				clientresponse.ResourceAlias{
					Kind:     "item",
					Public:   fmt.Sprintf("item_pf_%032d", seq),
					Original: fmt.Sprintf("%s:item:0:%d", salt, idx),
				},
				clientresponse.ResourceAlias{
					Kind:     "item",
					Public:   fmt.Sprintf("item_pf_%032d", 1000+seq),
					Original: fmt.Sprintf("%s:item:0:call:%d", salt, idx),
				},
				clientresponse.ResourceAlias{
					Kind:     "call",
					Public:   fmt.Sprintf("call_pf_%032d", seq),
					Original: fmt.Sprintf("%s:call:0:%d", salt, idx),
				},
			)
		}
	}
	for _, record := range seeds {
		aliases.Seed(record)
	}
	return aliases
}

func resetBenchStreamContext(ctx wrapper.HttpContext, upstreamModel string, incremental bool) {
	ctx.SetContext("incrementalStreaming", incremental)
	ctx.SetContext("finalRequestModel", upstreamModel)
	ctx.SetContext("qwenPushedIncrementalToolCalls", nil)
	ctx.SetContext("pushedMessage", nil)
	ctx.SetContext("contentPushed", nil)
	ctx.SetContext("reasoningContentPushed", nil)
	ctx.SetContext("hasContentDelta", nil)
	ctx.SetContext("bufferedReasoning", nil)
}

func BenchmarkProtocolRequestPipeline(b *testing.B) {
	initBenchFixtures(b)
	origLogOut := stdlog.Writer()
	stdlog.SetOutput(io.Discard)
	uuid.SetRand(newBenchDeterministicRand())
	b.Cleanup(func() {
		uuid.SetRand(nil)
		stdlog.SetOutput(origLogOut)
	})

	benchIdentity := balanceidentity.Snapshot{
		AccountID:            "10000000-0000-0000-0000-000000000001",
		APIKeyID:             "20000000-0000-0000-0000-000000000001",
		CredentialGeneration: "9007199254740993",
		TenantID:             "30000000-0000-0000-0000-000000000001",
		PrincipalID:          "40000000-0000-0000-0000-000000000001",
	}

	for _, tc := range benchRequestCases {
		b.Run(tc.name, func(b *testing.B) {
			env := newBenchHostEnv(b, tc.inputProtocol, tc.upstreamProtocol, tc.upstreamPath, tc.authority, tc.upstreamModel, tc.compatible)
			var lastReq managedProtocolRequest
			var lastState *managedExecutionState

			b.SetBytes(int64(len(tc.mappedBody)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				state := &managedExecutionState{
					config:        env.managedCfg,
					sourceConfig:  env.managedCfg,
					identity:      benchIdentity,
					callID:        benchCallID,
					businessUTC:   benchBusinessUTC,
					publicRequest: tc.publicBody,
				}
				ready := false
				authorizeManagedTools(env.ctx, state, tc.mappedBody, func() { ready = true })
				if !ready {
					b.Fatal("authorizeManagedTools did not complete synchronously")
				}
				prepared, ok := prepareManagedSelectedRequest(env.ctx, state, tc.mappedBody)
				if !ok {
					b.Fatal("prepareManagedSelectedRequest rejected valid benchmark request")
				}
				if err := prepareManagedExecution(state, tc.basis, prepared.Body); err != nil {
					b.Fatalf("prepareManagedExecution failed: %v", err)
				}
				lastReq = prepared
				lastState = state
			}
			b.StopTimer()

			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			b.ReportMetric(float64(ms.HeapInuse), "heap-inuse-B")

			if len(lastReq.Body) == 0 || lastState == nil {
				b.Fatal("empty outbound body produced")
			}
			tc.check(b, lastReq.Body, lastState)
		})
	}
}

func BenchmarkProtocolStreamPipeline(b *testing.B) {
	initBenchFixtures(b)
	origLogOut := stdlog.Writer()
	stdlog.SetOutput(io.Discard)
	uuid.SetRand(newBenchDeterministicRand())
	b.Cleanup(func() {
		uuid.SetRand(nil)
		stdlog.SetOutput(origLogOut)
	})

	for _, tc := range benchStreamCases {
		b.Run(tc.name, func(b *testing.B) {
			env := newBenchHostEnv(b, tc.inputProtocol, tc.upstreamProtocol, tc.upstreamPath, tc.authority, tc.upstreamModel, tc.compatible)
			if err := env.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")); err != nil {
				b.Fatal(err)
			}
			respHeaders := [][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
				{"x-request-id", benchUpstreamReqID},
			}
			_ = env.host.CallOnResponseHeaders(env.id, respHeaders, false)

			seededAliases := newBenchSeededAliases()
			var projected bytes.Buffer
			var lastObserved managedProtocolObservation

			b.SetBytes(int64(tc.totalBytes))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				projected.Reset()
				resetBenchStreamContext(env.ctx, tc.upstreamModel, tc.incremental)
				state := &managedExecutionState{
					config:        env.managedCfg,
					sourceConfig:  env.managedCfg,
					callID:        benchCallID,
					businessUTC:   benchBusinessUTC,
					streaming:     true,
					publicRequest: tc.publicRequest,
					publicAliases: seededAliases,
					router:        &managedRouterResponse{},
				}
				if action := observeManagedRouterHeaders(env.ctx, state); action != types.ActionContinue {
					b.Fatalf("observeManagedRouterHeaders returned %v", action)
				}
				for idx, chunk := range tc.chunks {
					out := projectManagedRouterChunk(env.ctx, state, chunk, idx == len(tc.chunks)-1)
					projected.Write(out)
				}
				if state.router.projectionFailed || state.router.projectedError {
					b.Fatal("stream projection failed during benchmark")
				}
				lastObserved = state.router.streamObservation()
			}
			b.StopTimer()

			if recs := seededAliases.Records(); len(recs) != 0 {
				b.Fatalf("unseeded alias triggered crypto/rand: %+v", recs)
			}

			elapsed := b.Elapsed()
			b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N*tc.eventCount), "ns/event")

			tc.check(b, projected.Bytes(), lastObserved)
		})
	}
}
