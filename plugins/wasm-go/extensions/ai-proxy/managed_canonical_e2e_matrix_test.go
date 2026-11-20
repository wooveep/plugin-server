package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/config"
	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/provider"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/ai/resourceref"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
	"github.com/tidwall/sjson"
)

type e2eMatrixDeterministicEntropy struct {
	rng *rand.ChaCha8
}

func newE2EMatrixDeterministicEntropy() *e2eMatrixDeterministicEntropy {
	var seed [32]byte
	binary.LittleEndian.PutUint64(seed[0:8], 0x20261119c0ffee01)
	binary.LittleEndian.PutUint64(seed[8:16], 0x51a7eed5b02c0de2)
	return &e2eMatrixDeterministicEntropy{rng: rand.NewChaCha8(seed)}
}

func (e *e2eMatrixDeterministicEntropy) Read(p []byte) (int, error) {
	return e.rng.Read(p)
}

var e2ePublicPFTokenPattern = regexp.MustCompile(`\b(resp|item|call|msg|chatcmpl)_pf_[0-9a-f]{32}\b`)
var e2eSignedToolCiphertext = regexp.MustCompile(`(call_pf_s_[0-9a-f]{32})_[A-Za-z0-9_-]+`)

func canonicalizeE2EPublicPFIDs(raw []byte) string {
	seen := make(map[string]string, 8)
	counts := make(map[string]int, 5)
	return e2ePublicPFTokenPattern.ReplaceAllStringFunc(string(raw), func(tok string) string {
		if mapped, ok := seen[tok]; ok {
			return mapped
		}
		prefix, _, _ := strings.Cut(tok, "_pf_")
		counts[prefix]++
		mapped := fmt.Sprintf("%s_pf_%04d", prefix, counts[prefix])
		seen[tok] = mapped
		return mapped
	})
}

func stripSSEKeepAliveComments(raw []byte) []byte {
	return bytes.ReplaceAll(raw, []byte(": keep-alive\n\n"), nil)
}

func newE2EMatrixRouterHost(
	t *testing.T,
	providerJSON string,
	authority string,
	inputProtocol string,
	upstreamProtocol string,
	upstreamPath string,
	upstreamModel string,
	profile clientresponse.Profile,
) *managedHost {
	t.Helper()
	fixture := managedConfigFixture()
	fixture["provider"] = json.RawMessage(providerJSON)
	release := fixture["_modelfusion_execution"].(managedRelease)
	release.Descriptor.UpstreamModel = upstreamModel
	release.Descriptor.AdapterRef = "ai-proxy.native-text.v1:" + inputProtocol + ":" + upstreamProtocol
	release.Descriptor.Transport.PathAdapterRef = release.Descriptor.AdapterRef
	release.Descriptor.Transport.Host = authority
	release.Descriptor.Transport.SNI = authority
	descRaw, err := json.Marshal(release.Descriptor)
	require.NoError(t, err)
	release.Descriptor.DescriptorDigest, err = managedDescriptorDigest(descRaw)
	require.NoError(t, err)
	fixture["_modelfusion_execution"] = release

	settings := fixture["managed_execution"].(managedSettings)
	settings.Authority = authority
	settings.InputProtocol = inputProtocol
	settings.UpstreamProtocol = upstreamProtocol
	settings.UpstreamPath = upstreamPath
	settings.SendTransport = "router"
	settings.StreamProjection = true
	if inputProtocol != managedProtocolChat {
		settings.StreamProtocol = inputProtocol
	}
	settings.ResponseProfile = profile
	fixture["managed_execution"] = settings
	fixture["protocol_mode"] = "auto"

	h := newManagedHost(t, fixture)
	require.NoError(t, h.host.SetProperty([]string{"cluster_name"}, []byte(release.Descriptor.ClusterRef)))
	return h
}

func splitSSEEventPayloadIntoMultiDataLines(rawSSE []byte) []byte {
	events, err := managedProtocolEvents(rawSSE)
	if err != nil {
		return rawSSE
	}
	var out bytes.Buffer
	for _, ev := range events {
		if ev.Name != "" {
			out.WriteString("event: ")
			out.WriteString(ev.Name)
			out.WriteByte('\n')
		}
		trimmed := bytes.TrimSpace(ev.Data)
		commaIdx := bytes.IndexByte(trimmed, ',')
		if len(trimmed) > 0 && trimmed[0] == '{' && commaIdx > 0 && commaIdx+1 < len(trimmed) {
			// Split JSON object across two SSE data: lines right after a top-level comma;
			// managedSSEDecoder joins consecutive data: lines with '\n', which is valid JSON whitespace.
			inString := false
			escaped := false
			splitPos := -1
			for i := 0; i < len(trimmed); i++ {
				b := trimmed[i]
				if escaped {
					escaped = false
					continue
				}
				if b == '\\' && inString {
					escaped = true
					continue
				}
				if b == '"' {
					inString = !inString
					continue
				}
				if !inString && b == ',' && i+1 < len(trimmed) {
					splitPos = i + 1
					break
				}
			}
			if splitPos > 0 {
				out.WriteString("data: ")
				out.Write(trimmed[:splitPos])
				out.WriteByte('\n')
				out.WriteString("data: ")
				out.Write(trimmed[splitPos:])
				out.WriteString("\n\n")
				continue
			}
		}
		out.WriteString("data: ")
		out.Write(ev.Data)
		out.WriteString("\n\n")
	}
	return out.Bytes()
}

func splitAtByteOffsets(payload []byte, offsets []int) [][]byte {
	var chunks [][]byte
	prev := 0
	for _, off := range offsets {
		if off <= prev || off >= len(payload) {
			continue
		}
		chunks = append(chunks, payload[prev:off])
		prev = off
	}
	if prev < len(payload) {
		chunks = append(chunks, payload[prev:])
	}
	return chunks
}

// 1. TestCanonicalSSEPartitionMatrix (T20)
// Verifies byte-for-byte canonical stream & exact Usage invariance across 11 SSE partition/framing
// modes on 4 representative cross-wire streams, plus MaxEnvelopeBytes B-1/B/B+1, invalid UTF-8,
// and unterminated payload error enforcement.
func TestCanonicalSSEPartitionMatrix(t *testing.T) {
	type streamMatrixCase struct {
		name             string
		providerJSON     string
		authority        string
		inputProtocol    string
		upstreamProtocol string
		upstreamPath     string
		upstreamModel    string
		profile          clientresponse.Profile
		publicRequest    []byte
		rawSSE           []byte
		wantQuantities   map[string]string
		mustContain      []string
	}

	cases := []streamMatrixCase{
		{
			name:             "chat_to_responses",
			providerJSON:     `{"type":"openai","apiTokens":["controlled-upstream-key"]}`,
			authority:        "api.openai.example",
			inputProtocol:    managedProtocolResponses,
			upstreamProtocol: managedProtocolChat,
			upstreamPath:     "/v1/chat/completions",
			upstreamModel:    "gpt-4.1",
			profile:          clientresponse.OpenAI,
			publicRequest: []byte(`{
				"model":"public-model",
				"stream":true,
				"input":"查一下杭州天气🌤️",
				"tools":[{"type":"function","name":"get_weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}]
			}`),
			rawSSE: []byte(
				"data: {\"id\":\"chatcmpl-t20-1\",\"object\":\"chat.completion.chunk\",\"created\":1761559200,\"model\":\"gpt-4.1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"reasoning_content\":\"先核查实时气象数据…\"}}]}\n\n" +
					"data: {\"id\":\"chatcmpl-t20-1\",\"object\":\"chat.completion.chunk\",\"created\":1761559200,\"model\":\"gpt-4.1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"你好，杭州天气🌤️正常\"}}]}\n\n" +
					"data: {\"id\":\"chatcmpl-t20-1\",\"object\":\"chat.completion.chunk\",\"created\":1761559200,\"model\":\"gpt-4.1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_hz_01\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\\\"杭\"}}]}}]}\n\n" +
					"data: {\"id\":\"chatcmpl-t20-1\",\"object\":\"chat.completion.chunk\",\"created\":1761559200,\"model\":\"gpt-4.1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"州\\\",\\\"trace\\\":9007199254740993}\"}}]}}]}\n\n" +
					"data: {\"id\":\"chatcmpl-t20-1\",\"object\":\"chat.completion.chunk\",\"created\":1761559200,\"model\":\"gpt-4.1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
					"data: {\"id\":\"chatcmpl-t20-1\",\"object\":\"chat.completion.chunk\",\"created\":1761559200,\"model\":\"gpt-4.1\",\"choices\":[],\"usage\":{\"prompt_tokens\":9007199254740993,\"completion_tokens\":42,\"total_tokens\":9007199254741035,\"prompt_tokens_details\":{\"cached_tokens\":128},\"completion_tokens_details\":{\"reasoning_tokens\":11}}}\n\n" +
					"data: [DONE]\n\n",
			),
			wantQuantities: map[string]string{
				"input":            "9007199254740993",
				"output":           "42",
				"cache_hit":        "128",
				"cache_miss":       "9007199254740865",
				"reasoning_tokens": "11",
			},
			mustContain: []string{
				`"type":"response.reasoning_text.delta"`,
				`你好，杭州天气🌤️正常`,
				`9007199254740993`,
				`"type":"response.completed"`,
			},
		},
		{
			name:             "dashscope_cumulative_tools_reasoning_to_responses",
			providerJSON:     `{"type":"qwen","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":false,"qwenEnableSearch":false}`,
			authority:        "dashscope.aliyuncs.com",
			inputProtocol:    managedProtocolResponses,
			upstreamProtocol: managedProtocolDashScopeText,
			upstreamPath:     "/api/v1/services/aigc/text-generation/generation",
			upstreamModel:    "qwen-flash",
			profile:          clientresponse.Qwen,
			publicRequest: []byte(`{
				"model":"public-model",
				"stream":true,
				"input":"读取杭州🌤️气象配置",
				"tools":[{"type":"function","name":"read_weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}]
			}`),
			rawSSE: []byte(
				"id:1\nevent:result\ndata:" + `{"request_id":"11111111-2222-4333-8444-555555555555","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"核查杭州"}}]},"usage":{"input_tokens":2048,"output_tokens":3,"total_tokens":2051}}` + "\n\n" +
					"id:2\nevent:result\ndata:" + `{"request_id":"11111111-2222-4333-8444-555555555555","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"核查杭州🌤️天气","tool_calls":[{"index":0,"id":"call_ds_cum_1","type":"function","function":{"name":"read_weather","arguments":""}}]}}]},"usage":{"input_tokens":2048,"output_tokens":8,"total_tokens":2056}}` + "\n\n" +
					"id:3\nevent:result\ndata:" + `{"request_id":"11111111-2222-4333-8444-555555555555","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"核查杭州🌤️天气","tool_calls":[{"index":0,"id":"call_ds_cum_1","type":"function","function":{"name":"read_weather","arguments":"{\"city\":\"杭"}}]}}]},"usage":{"input_tokens":2048,"output_tokens":15,"total_tokens":2063}}` + "\n\n" +
					"id:4\nevent:result\ndata:" + `{"request_id":"11111111-2222-4333-8444-555555555555","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":"","reasoning_content":"核查杭州🌤️天气","tool_calls":[{"index":0,"id":"call_ds_cum_1","type":"function","function":{"name":"read_weather","arguments":"{\"city\":\"杭州\",\"id\":9007199254740993}"}}]}}]},"usage":{"input_tokens":2048,"output_tokens":24,"total_tokens":2072}}` + "\n\n" +
					"id:5\nevent:result\ndata:" + `{"request_id":"11111111-2222-4333-8444-555555555555","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","reasoning_content":"核查杭州🌤️天气","tool_calls":[{"index":0,"id":"call_ds_cum_1","type":"function","function":{"name":"read_weather","arguments":"{\"city\":\"杭州\",\"id\":9007199254740993}"}}]}}]},"usage":{"input_tokens":2048,"output_tokens":24,"total_tokens":2072,"prompt_tokens_details":{"cached_tokens":512},"output_tokens_details":{"reasoning_tokens":7}}}` + "\n\n",
			),
			wantQuantities: map[string]string{
				"input":            "2048",
				"output":           "24",
				"cache_hit":        "512",
				"cache_miss":       "1536",
				"reasoning_tokens": "7",
			},
			mustContain: []string{
				`"type":"response.reasoning_text.delta"`,
				`"type":"response.function_call_arguments.delta"`,
				`9007199254740993`,
				`"type":"response.completed"`,
			},
		},
		{
			name:             "dashscope_incremental_thinking_tools_to_messages",
			providerJSON:     `{"type":"qwen","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":false,"qwenEnableSearch":false}`,
			authority:        "dashscope.aliyuncs.com",
			inputProtocol:    managedProtocolMessages,
			upstreamProtocol: managedProtocolDashScopeMultimodal,
			upstreamPath:     "/api/v1/services/aigc/multimodal-generation/generation",
			upstreamModel:    "qwen3.6-plus",
			profile:          clientresponse.Qwen,
			publicRequest: []byte(`{
				"model":"public-model",
				"stream":true,
				"max_tokens":256,
				"thinking":{"type":"enabled","budget_tokens":1024},
				"messages":[{"role":"user","content":[{"type":"text","text":"查杭州🌤️天气"}]}],
				"tools":[{"name":"get_weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}]
			}`),
			rawSSE: []byte(
				"id:1\nevent:result\ndata:" + `{"request_id":"22222222-3333-4444-8555-666666666666","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":[],"reasoning_content":"思考中："}}]},"usage":{"input_tokens":150,"output_tokens":4,"total_tokens":154}}` + "\n\n" +
					"id:2\nevent:result\ndata:" + `{"request_id":"22222222-3333-4444-8555-666666666666","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":[],"reasoning_content":"查询杭州🌤️"}}]},"usage":{"input_tokens":150,"output_tokens":9,"total_tokens":159}}` + "\n\n" +
					"id:3\nevent:result\ndata:" + `{"request_id":"22222222-3333-4444-8555-666666666666","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":[],"reasoning_content":"","tool_calls":[{"index":0,"id":"call_ds_inc_1","type":"function","function":{"name":"get_weather","arguments":""}}]}}]},"usage":{"input_tokens":150,"output_tokens":13,"total_tokens":163}}` + "\n\n" +
					"id:4\nevent:result\ndata:" + `{"request_id":"22222222-3333-4444-8555-666666666666","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":[],"reasoning_content":"","tool_calls":[{"index":0,"id":"","type":"function","function":{"arguments":"{\"city\":\"杭"}}]}}]},"usage":{"input_tokens":150,"output_tokens":20,"total_tokens":170}}` + "\n\n" +
					"id:5\nevent:result\ndata:" + `{"request_id":"22222222-3333-4444-8555-666666666666","output":{"choices":[{"finish_reason":"null","message":{"role":"assistant","content":[],"reasoning_content":"","tool_calls":[{"index":0,"id":"","type":"function","function":{"arguments":"州\",\"seq\":9007199254740993}"}}]}}]},"usage":{"input_tokens":150,"output_tokens":29,"total_tokens":179}}` + "\n\n" +
					"id:6\nevent:result\ndata:" + `{"request_id":"22222222-3333-4444-8555-666666666666","output":{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":[],"reasoning_content":"","tool_calls":[{"index":0,"type":"function","function":{}}]}}]},"usage":{"input_tokens":150,"output_tokens":29,"total_tokens":179,"prompt_tokens_details":{"cached_tokens":50},"output_tokens_details":{"reasoning_tokens":9}}}` + "\n\n",
			),
			wantQuantities: map[string]string{
				"input":            "150",
				"output":           "29",
				"cache_hit":        "50",
				"cache_miss":       "100",
				"reasoning_tokens": "9",
			},
			mustContain: []string{
				`"type":"thinking_delta"`,
				`查询杭州🌤️`,
				`"type":"input_json_delta"`,
				`"stop_reason":"tool_use"`,
				`event: message_stop`,
			},
		},
		{
			name:             "gemini_to_responses",
			providerJSON:     `{"type":"gemini","apiTokens":["controlled-upstream-key"],"apiVersion":"v1beta"}`,
			authority:        "generativelanguage.googleapis.com",
			inputProtocol:    managedProtocolResponses,
			upstreamProtocol: managedProtocolChat,
			upstreamPath:     "/v1beta/models/gemini-2.5-pro:streamGenerateContent",
			upstreamModel:    "gemini-2.5-pro",
			profile:          "",
			publicRequest: []byte(`{
				"model":"public-model",
				"stream":true,
				"input":"查询杭州🌤️实时天气",
				"tools":[{"type":"function","name":"get_weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}]
			}`),
			rawSSE: []byte(
				"data: {\"responseId\":\"gem-resp-t20\",\"modelVersion\":\"gemini-2.5-pro\",\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"thought\":true,\"text\":\"检索杭州🌤️气象…\"}]}}]}\n\n" +
					"data: {\"responseId\":\"gem-resp-t20\",\"modelVersion\":\"gemini-2.5-pro\",\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"当前杭州晴朗☀️\"},{\"functionCall\":{\"name\":\"get_weather\",\"args\":{\"city\":\"杭州\",\"id\":9007199254740993}},\"thoughtSignature\":\"sig_google_stream_1\"}]}}]}\n\n" +
					"data: {\"responseId\":\"gem-resp-t20\",\"modelVersion\":\"gemini-2.5-pro\",\"candidates\":[{\"index\":0,\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":120,\"cachedContentTokenCount\":30,\"toolUsePromptTokenCount\":8,\"candidatesTokenCount\":19,\"thoughtsTokenCount\":6,\"totalTokenCount\":153}}\n\n",
			),
			wantQuantities: map[string]string{
				"input":            "128",
				"output":           "25",
				"cache_hit":        "30",
				"cache_miss":       "98",
				"reasoning_tokens": "6",
			},
			mustContain: []string{
				`"type":"response.reasoning_text.delta"`,
				`当前杭州晴朗☀️`,
				`9007199254740993`,
				`"type":"response.completed"`,
			},
		},
	}

	replayRouterPartitions := func(t *testing.T, tc streamMatrixCase, chunks [][]byte, expectImmediateKeepAlive bool) ([]byte, executionfacts.Envelope) {
		t.Helper()
		restoreEntropy := clientresponse.SetIdentityEntropySource(newE2EMatrixDeterministicEntropy())
		defer restoreEntropy()

		h := newE2EMatrixRouterHost(t, tc.providerJSON, tc.authority, tc.inputProtocol, tc.upstreamProtocol, tc.upstreamPath, tc.upstreamModel, tc.profile)
		defer h.host.CompleteHttpContext(h.id)

		mapped := bytes.Replace(tc.publicRequest, []byte(`"public-model"`), []byte(`"`+tc.upstreamModel+`"`), 1)
		h.start(t, tc.publicRequest, mapped)

		var injected bytes.Buffer
		origInject := injectManagedRouterTerminal
		t.Cleanup(func() { injectManagedRouterTerminal = origInject })
		injectManagedRouterTerminal = func(body []byte, _ bool) error {
			injected.Write(body)
			return nil
		}

		require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
		require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id, [][2]string{
			{":status", "200"},
			{"content-type", "text/event-stream"},
		}, false))

		var streamed bytes.Buffer
		sawImmediateKeepAlive := false
		for idx, chunk := range chunks {
			isEnd := idx == len(chunks)-1
			h.host.CallOnResponseBody(h.id, chunk, isEnd)
			emitted := h.host.GetCurrentResponseBody(h.id)
			if expectImmediateKeepAlive && bytes.Contains(chunk, []byte(": keep-alive")) && bytes.Contains(emitted, []byte(": keep-alive\n\n")) {
				sawImmediateKeepAlive = true
			}
			streamed.Write(emitted)
		}
		if expectImmediateKeepAlive {
			require.True(t, sawImmediateKeepAlive, "comment line must forward ': keep-alive\\n\\n' immediately without waiting for following event")
		}
		drainDashScopeRedis(h)

		fullPublic := append(streamed.Bytes(), injected.Bytes()...)
		state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
		require.NotNil(t, state.lastDelivery)
		return fullPublic, state.lastDelivery.Envelope()
	}

	for _, tc := range cases {
		t.Run("partition_invariance/"+tc.name, func(t *testing.T) {
			var wholeBytes []byte
			var wholeDelivery executionfacts.Envelope
			t.Run("whole", func(t *testing.T) {
				wholeBytes, wholeDelivery = replayRouterPartitions(t, tc, [][]byte{tc.rawSSE}, false)
			})
			require.Equal(t, "succeeded", wholeDelivery.DeliveryState)
			require.Equal(t, "reported", wholeDelivery.Usage.Status)
			for k, wantVal := range tc.wantQuantities {
				require.Equal(t, wantVal, wholeDelivery.Usage.Quantities[k], "quantity %s mismatch", k)
			}
			for _, snippet := range tc.mustContain {
				require.Contains(t, string(wholeBytes), snippet)
			}
			require.NotContains(t, string(wholeBytes), "response_incomplete")

			// Keep the deterministic resource IDs and all wire fields in the
			// comparison. Separate deliveries intentionally seal with fresh keys.
			wholeBytes = e2eSignedToolCiphertext.ReplaceAll(wholeBytes, []byte("$1"))
			wholeEvents, err := clientresponse.Events(wholeBytes)
			require.NoError(t, err)
			require.NotEmpty(t, wholeEvents)

			// Build 1-byte chunks.
			oneByteChunks := make([][]byte, len(tc.rawSSE))
			for i := range tc.rawSSE {
				oneByteChunks[i] = tc.rawSSE[i : i+1]
			}

			// Build UTF-8 multi-byte rune interior cut offsets (splitting 3-byte CJK & 4-byte emoji sequences).
			var utf8Cuts []int
			for i := 0; i < len(tc.rawSSE); i++ {
				if tc.rawSSE[i] >= 0xe0 && i+2 < len(tc.rawSSE) {
					utf8Cuts = append(utf8Cuts, i+1, i+2)
					if tc.rawSSE[i] >= 0xf0 && i+3 < len(tc.rawSSE) {
						utf8Cuts = append(utf8Cuts, i+3)
					}
				}
			}
			require.NotEmpty(t, utf8Cuts, "fixture must contain multi-byte UTF-8 runes")

			// Build JSON mid-token cut offsets.
			var jsonTokenCuts []int
			for _, token := range []string{`"choices"`, `"candidates"`, `"arguments"`, `"finish_reason"`, `"finishReason"`, `9007199254740993`, `[DONE]`, `"usage"`} {
				searchFrom := 0
				for {
					pos := bytes.Index(tc.rawSSE[searchFrom:], []byte(token))
					if pos < 0 {
						break
					}
					absPos := searchFrom + pos + len(token)/2
					jsonTokenCuts = append(jsonTokenCuts, absPos)
					searchFrom += pos + len(token)
				}
			}
			require.NotEmpty(t, jsonTokenCuts)

			// UTF-8 BOM prefixed stream (also split inside the 3-byte BOM).
			bomSSE := append([]byte{0xef, 0xbb, 0xbf}, tc.rawSSE...)
			bomChunks := [][]byte{bomSSE[:1], bomSSE[1:2], bomSSE[2:10], bomSSE[10:]}

			// Lone \r line endings.
			loneCRSSE := bytes.ReplaceAll(tc.rawSSE, []byte("\n"), []byte("\r"))

			// Split \r\n right between \r and \n on every CRLF boundary.
			crlfSSE := bytes.ReplaceAll(tc.rawSSE, []byte("\n"), []byte("\r\n"))
			var splitCRLFCuts []int
			for i := 0; i+1 < len(crlfSSE); i++ {
				if crlfSSE[i] == '\r' && crlfSSE[i+1] == '\n' {
					splitCRLFCuts = append(splitCRLFCuts, i+1)
				}
			}
			require.NotEmpty(t, splitCRLFCuts)

			// Multi-data: lines per JSON event.
			multiDataSSE := splitSSEEventPayloadIntoMultiDataLines(tc.rawSSE)
			require.NotEqual(t, tc.rawSSE, multiDataSSE, "multi-data transform must split JSON across multiple data: lines")

			// Fixed-seed (seed=20261119) random chunking.
			rng := rand.New(rand.NewPCG(20261119, 20261119))
			var randomChunks [][]byte
			for pos := 0; pos < len(tc.rawSSE); {
				step := 1 + rng.IntN(23)
				if pos+step > len(tc.rawSSE) {
					step = len(tc.rawSSE) - pos
				}
				randomChunks = append(randomChunks, tc.rawSSE[pos:pos+step])
				pos += step
			}

			partitions := []struct {
				mode   string
				chunks [][]byte
			}{
				{mode: "byte_by_byte", chunks: oneByteChunks},
				{mode: "utf8_mid_rune", chunks: splitAtByteOffsets(tc.rawSSE, utf8Cuts)},
				{mode: "json_mid_token", chunks: splitAtByteOffsets(tc.rawSSE, jsonTokenCuts)},
				{mode: "utf8_bom", chunks: bomChunks},
				{mode: "lone_cr", chunks: splitAtByteOffsets(loneCRSSE, []int{7, 31, len(loneCRSSE) / 2})},
				{mode: "lf", chunks: [][]byte{tc.rawSSE[:len(tc.rawSSE)/3], tc.rawSSE[len(tc.rawSSE)/3:]}},
				{mode: "split_crlf", chunks: splitAtByteOffsets(crlfSSE, splitCRLFCuts)},
				{mode: "multi_data_lines", chunks: splitAtByteOffsets(multiDataSSE, []int{13, 64, len(multiDataSSE) / 2})},
				{mode: "random_seed_20261119", chunks: randomChunks},
			}

			for _, part := range partitions {
				t.Run(part.mode, func(t *testing.T) {
					gotBytes, gotDelivery := replayRouterPartitions(t, tc, part.chunks, false)
					gotBytes = e2eSignedToolCiphertext.ReplaceAll(gotBytes, []byte("$1"))
					require.Equal(t, wholeBytes, gotBytes, "partition %s diverged from whole stream", part.mode)
					require.Equal(t, wholeDelivery.DeliveryState, gotDelivery.DeliveryState)
					require.Equal(t, wholeDelivery.Usage, gotDelivery.Usage)
				})
			}

			t.Run("keep_alive_comments", func(t *testing.T) {
				mid := bytes.Index(tc.rawSSE, []byte("\n\n"))
				require.Greater(t, mid, 0)
				commentChunks := [][]byte{
					[]byte(": keep-alive\n"),
					tc.rawSSE[:mid+2],
					[]byte(": keep-alive heartbeat\r\n"),
					tc.rawSSE[mid+2:],
					[]byte(": keep-alive tail\n"),
				}
				gotBytes, gotDelivery := replayRouterPartitions(t, tc, commentChunks, true)
				gotBytes = e2eSignedToolCiphertext.ReplaceAll(gotBytes, []byte("$1"))
				require.Equal(t, stripSSEKeepAliveComments(wholeBytes), stripSSEKeepAliveComments(gotBytes))
				gotEvents, evErr := clientresponse.Events(gotBytes)
				require.NoError(t, evErr)
				require.Equal(t, wholeEvents, gotEvents)
				require.Equal(t, wholeDelivery.DeliveryState, gotDelivery.DeliveryState)
				require.Equal(t, wholeDelivery.Usage, gotDelivery.Usage)
			})
		})
	}

	t.Run("envelope_budget_and_invalid_framing", func(t *testing.T) {
		budget := executionfacts.MaxEnvelopeBytes

		// 1. Line budget B-1, B (pass) vs B+1 (explicit error).
		for _, lineLen := range []int{budget - 1, budget, budget + 1} {
			t.Run(fmt.Sprintf("line_budget_%d", lineLen), func(t *testing.T) {
				lineBytes := make([]byte, lineLen)
				lineBytes[0] = ':'
				for i := 1; i < lineLen; i++ {
					lineBytes[i] = 'a'
				}
				var dec managedSSEDecoder
				errFirst := dec.push(lineBytes[:lineLen/2], func(managedProtocolEvent) {})
				require.NoError(t, errFirst)
				tailWithLF := append(bytes.Clone(lineBytes[lineLen/2:]), '\n')
				errSecond := dec.push(tailWithLF, func(managedProtocolEvent) {})
				if lineLen <= budget {
					require.NoError(t, errSecond)
					require.NoError(t, dec.finish())
				} else {
					require.EqualError(t, errSecond, "managed SSE line budget exceeded")
				}
			})
		}

		// 2. Payload budget B-1, B (pass) vs B+1 (explicit error), verified on both decoder and Router stream.
		for _, payloadLen := range []int{budget - 1, budget, budget + 1} {
			t.Run(fmt.Sprintf("payload_budget_%d", payloadLen), func(t *testing.T) {
				// Construct two data: lines whose assembled payload (val1 + '\n' + val2) equals payloadLen bytes.
				prefix := []byte(`{"id":"chat-b","model":"gpt-4.1","choices":[{"index":0,"delta":{"content":"`)
				suffix := []byte(`"}}]}`)
				fillTotal := payloadLen - len(prefix) - len(suffix) - 1 // -1 for '\n' inserted between two data: lines
				require.Greater(t, fillTotal, 10)
				fillA := bytes.Repeat([]byte("x"), fillTotal/2)
				fillB := bytes.Repeat([]byte("y"), fillTotal-len(fillA))

				var sseBuf bytes.Buffer
				sseBuf.WriteString("data: ")
				sseBuf.Write(prefix)
				sseBuf.Write(fillA)
				sseBuf.WriteByte('\n')
				sseBuf.WriteString("data: ")
				sseBuf.Write(fillB)
				sseBuf.Write(suffix)
				sseBuf.WriteString("\n\n")

				var dec managedSSEDecoder
				var capturedLen int
				pushErr := dec.push(sseBuf.Bytes(), func(ev managedProtocolEvent) {
					capturedLen = len(ev.Data)
				})
				if payloadLen <= budget {
					require.NoError(t, pushErr)
					require.NoError(t, dec.finish())
					// Note: JSON string cannot contain raw unescaped '\n', so test Router end-to-end with valid JSON whitespace split!
					require.Equal(t, payloadLen, capturedLen)
				} else {
					require.EqualError(t, pushErr, "managed SSE payload budget exceeded")
				}

				// Router end-to-end check with valid JSON split across two data: lines at comma boundary.
				jsonPart1 := []byte(`{"id":"chat-b","model":"gpt-4.1",`)
				jsonHead2 := []byte(`"choices":[{"index":0,"delta":{"content":"`)
				jsonTail2 := []byte(`"}}]}`)
				jsonFillLen := payloadLen - len(jsonPart1) - 1 - len(jsonHead2) - len(jsonTail2)
				var routerSSE bytes.Buffer
				routerSSE.WriteString("data: ")
				routerSSE.Write(jsonPart1)
				routerSSE.WriteByte('\n')
				routerSSE.WriteString("data: ")
				routerSSE.Write(jsonHead2)
				routerSSE.Write(bytes.Repeat([]byte("z"), jsonFillLen))
				routerSSE.Write(jsonTail2)
				routerSSE.WriteString("\n\n")
				routerSSE.WriteString("data: {\"id\":\"chat-b\",\"model\":\"gpt-4.1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":4,\"total_tokens\":14}}\n\ndata: [DONE]\n\n")

				h := newE2EMatrixRouterHost(
					t,
					`{"type":"openai","apiTokens":["controlled-upstream-key"]}`,
					"api.openai.example",
					managedProtocolResponses,
					managedProtocolChat,
					"/v1/chat/completions",
					"gpt-4.1",
					clientresponse.OpenAI,
				)
				pubReq := []byte(`{"model":"public-model","stream":true,"input":"budget"}`)
				h.start(t, pubReq, bytes.Replace(pubReq, []byte("public-model"), []byte("gpt-4.1"), 1))
				var injected bytes.Buffer
				origInject := injectManagedRouterTerminal
				t.Cleanup(func() { injectManagedRouterTerminal = origInject })
				injectManagedRouterTerminal = func(body []byte, _ bool) error {
					injected.Write(body)
					return nil
				}
				require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
				require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false))
				h.host.CallOnResponseBody(h.id, routerSSE.Bytes(), true)
				drainDashScopeRedis(h)
				respStr := string(h.host.GetCurrentResponseBody(h.id)) + injected.String()
				state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
				require.NotNil(t, state.lastDelivery)
				if payloadLen <= budget {
					require.Contains(t, respStr, `"type":"response.completed"`)
					require.Equal(t, "succeeded", state.lastDelivery.Envelope().DeliveryState)
				} else {
					require.NotContains(t, respStr, `"type":"response.completed"`)
					require.Contains(t, respStr, `response_incomplete`)
					require.Equal(t, "failed", state.lastDelivery.Envelope().DeliveryState)
				}
				h.host.CompleteHttpContext(h.id)
			})
		}

		// 3. Invalid UTF-8 byte sequence and unterminated SSE payload at EOF must fail explicitly.
		for _, bad := range []struct {
			name       string
			raw        []byte
			wantDecErr string
		}{
			{
				name:       "invalid_utf8_line",
				raw:        append([]byte("data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\""), 0xff, '"', '}', '}', ']', '}', '\n', '\n'),
				wantDecErr: "invalid managed SSE line",
			},
			{
				name:       "unterminated_event_without_blank_line",
				raw:        []byte("data: {\"id\":\"1\",\"model\":\"gpt-4.1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":\"stop\"}]}\n"),
				wantDecErr: "incomplete managed SSE framing",
			},
			{
				name:       "unterminated_partial_line_without_lf",
				raw:        []byte("data: {\"id\":\"1\",\"model\":\"gpt-4.1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"}}]}"),
				wantDecErr: "incomplete managed SSE framing",
			},
		} {
			t.Run(bad.name, func(t *testing.T) {
				var dec managedSSEDecoder
				err := dec.push(bad.raw, func(managedProtocolEvent) {})
				if err == nil {
					err = dec.finish()
				}
				require.EqualError(t, err, bad.wantDecErr)

				h := newE2EMatrixRouterHost(
					t,
					`{"type":"openai","apiTokens":["controlled-upstream-key"]}`,
					"api.openai.example",
					managedProtocolResponses,
					managedProtocolChat,
					"/v1/chat/completions",
					"gpt-4.1",
					clientresponse.OpenAI,
				)
				defer h.host.CompleteHttpContext(h.id)
				pubReq := []byte(`{"model":"public-model","stream":true,"input":"hi"}`)
				h.start(t, pubReq, bytes.Replace(pubReq, []byte("public-model"), []byte("gpt-4.1"), 1))
				require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
				require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false))
				h.host.CallOnResponseBody(h.id, bad.raw, true)
				respStr := string(h.host.GetCurrentResponseBody(h.id))
				require.NotContains(t, respStr, `"type":"response.completed"`)
				require.Contains(t, respStr, `response_incomplete`)
				state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
				require.NotNil(t, state.lastDelivery)
				require.Equal(t, "failed", state.lastDelivery.Envelope().DeliveryState)
			})
		}
	})
}

// 2. FuzzCanonicalSSEPartition (T26 Part 2)
// Fuzzes main-package managedSSEDecoder + projectManagedRouterChunk canonical streaming pipeline
// across Chat, DashScope (incremental & cumulative), and Gemini wire formats and all 3 client protocols,
// verifying zero panic, zero out-of-bounds, partition invariance on valid streams, and per-attempt isolation.
func FuzzCanonicalSSEPartition(f *testing.F) {
	origLogWriter := stdlog.Writer()
	stdlog.SetOutput(io.Discard)
	f.Cleanup(func() { stdlog.SetOutput(origLogWriter) })

	fuzzHost, resetFuzzHost := proxytest.NewHostEmulator(proxytest.NewEmulatorOption())
	f.Cleanup(resetFuzzHost)
	fuzzHost.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	fuzzHost.RegisterForeignFunction("declare_property", func([]byte) []byte { return []byte{0} })

	type fuzzRouteSpec struct {
		providerJSON     string
		authority        string
		inputProtocol    string
		upstreamProtocol string
		upstreamPath     string
		upstreamModel    string
		profile          clientresponse.Profile
		publicReq        []byte
		cleanStream      []byte
	}

	buildPublicReq := func(inProto string, thinking bool) []byte {
		switch inProto {
		case managedProtocolChat:
			if thinking {
				return []byte(`{"model":"fuzz-model","stream":true,"stream_options":{"include_usage":true},"enable_thinking":true,"messages":[{"role":"user","content":"ping"}],"tools":[{"type":"function","function":{"name":"fn_a","parameters":{"type":"object"}}}]}`)
			}
			return []byte(`{"model":"fuzz-model","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"ping"}],"tools":[{"type":"function","function":{"name":"fn_a","parameters":{"type":"object"}}}]}`)
		case managedProtocolMessages:
			if thinking {
				return []byte(`{"model":"fuzz-model","stream":true,"max_tokens":128,"thinking":{"type":"enabled","budget_tokens":1024},"messages":[{"role":"user","content":"ping"}],"tools":[{"name":"fn_a","input_schema":{"type":"object"}}]}`)
			}
			return []byte(`{"model":"fuzz-model","stream":true,"max_tokens":128,"messages":[{"role":"user","content":"ping"}],"tools":[{"name":"fn_a","input_schema":{"type":"object"}}]}`)
		default:
			if thinking {
				return []byte(`{"model":"fuzz-model","stream":true,"reasoning":{"effort":"low"},"input":"ping","tools":[{"type":"function","name":"fn_a","parameters":{"type":"object"}}]}`)
			}
			return []byte(`{"model":"fuzz-model","stream":true,"input":"ping","tools":[{"type":"function","name":"fn_a","parameters":{"type":"object"}}]}`)
		}
	}

	cleanChatSSE := []byte(
		"data: {\"id\":\"c-1\",\"model\":\"fuzz-model\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok🌤️\"}}]}\n\n" +
			"data: {\"id\":\"c-1\",\"model\":\"fuzz-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":3,\"total_tokens\":14}}\n\n" +
			"data: [DONE]\n\n",
	)
	cleanDashIncSSE := []byte(
		"event:result\ndata:{\"request_id\":\"33333333-4444-4555-8666-777777777777\",\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"ok\",\"reasoning_content\":\"think\"}}]},\"usage\":{\"input_tokens\":12,\"output_tokens\":2,\"total_tokens\":14}}\n\n" +
			"event:result\ndata:{\"request_id\":\"33333333-4444-4555-8666-777777777777\",\"output\":{\"choices\":[{\"finish_reason\":\"stop\",\"message\":{\"role\":\"assistant\",\"content\":\"!\"}}]},\"usage\":{\"input_tokens\":12,\"output_tokens\":4,\"total_tokens\":16,\"output_tokens_details\":{\"reasoning_tokens\":1}}}\n\n",
	)
	cleanDashCumSSE := []byte(
		"event:result\ndata:{\"request_id\":\"44444444-5555-4666-8777-888888888888\",\"output\":{\"choices\":[{\"finish_reason\":\"null\",\"message\":{\"role\":\"assistant\",\"content\":\"hel\"}}]},\"usage\":{\"input_tokens\":12,\"output_tokens\":1,\"total_tokens\":13}}\n\n" +
			"event:result\ndata:{\"request_id\":\"44444444-5555-4666-8777-888888888888\",\"output\":{\"choices\":[{\"finish_reason\":\"stop\",\"message\":{\"role\":\"assistant\",\"content\":\"hello\"}}]},\"usage\":{\"input_tokens\":12,\"output_tokens\":2,\"total_tokens\":14}}\n\n",
	)
	cleanGeminiSSE := []byte(
		"data: {\"responseId\":\"g-1\",\"modelVersion\":\"fuzz-model\",\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"thought\":true,\"text\":\"hm\"},{\"text\":\"ok\"}]}}]}\n\n" +
			"data: {\"responseId\":\"g-1\",\"modelVersion\":\"fuzz-model\",\"candidates\":[{\"index\":0,\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":2,\"thoughtsTokenCount\":1,\"totalTokenCount\":13}}\n\n",
	)

	clientProtos := []string{managedProtocolResponses, managedProtocolMessages, managedProtocolChat}
	var specs []fuzzRouteSpec
	var parsedConfigs []*managedExecutionConfig

	for _, inProto := range clientProtos {
		if inProto != managedProtocolChat {
			prof := clientresponse.OpenAI
			if inProto == managedProtocolMessages {
				prof = clientresponse.DeepSeek
			}
			specs = append(specs, fuzzRouteSpec{
				providerJSON:     `{"type":"openai","apiTokens":["controlled-upstream-key"]}`,
				authority:        "api.openai.example",
				inputProtocol:    inProto,
				upstreamProtocol: managedProtocolChat,
				upstreamPath:     "/v1/chat/completions",
				upstreamModel:    "fuzz-model",
				profile:          prof,
				publicReq:        buildPublicReq(inProto, false),
				cleanStream:      cleanChatSSE,
			})
		}
		specs = append(specs,
			fuzzRouteSpec{
				providerJSON:     `{"type":"qwen","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":false}`,
				authority:        "dashscope.aliyuncs.com",
				inputProtocol:    inProto,
				upstreamProtocol: managedProtocolDashScopeText,
				upstreamPath:     "/api/v1/services/aigc/text-generation/generation",
				upstreamModel:    "fuzz-model",
				profile:          clientresponse.Qwen,
				publicReq:        buildPublicReq(inProto, true),
				cleanStream:      cleanDashIncSSE,
			},
			fuzzRouteSpec{
				providerJSON:     `{"type":"qwen","apiTokens":["controlled-upstream-key"],"qwenEnableCompatible":false}`,
				authority:        "dashscope.aliyuncs.com",
				inputProtocol:    inProto,
				upstreamProtocol: managedProtocolDashScopeText,
				upstreamPath:     "/api/v1/services/aigc/text-generation/generation",
				upstreamModel:    "fuzz-model",
				profile:          clientresponse.Qwen,
				publicReq:        buildPublicReq(inProto, false),
				cleanStream:      cleanDashCumSSE,
			},
			fuzzRouteSpec{
				providerJSON:     `{"type":"gemini","apiTokens":["controlled-upstream-key"],"apiVersion":"v1beta"}`,
				authority:        "generativelanguage.googleapis.com",
				inputProtocol:    inProto,
				upstreamProtocol: managedProtocolChat,
				upstreamPath:     "/v1beta/models/fuzz-model:streamGenerateContent",
				upstreamModel:    "fuzz-model",
				profile:          "",
				publicReq:        buildPublicReq(inProto, true),
				cleanStream:      cleanGeminiSSE,
			},
		)
	}

	for _, sp := range specs {
		fixture := managedConfigFixture()
		fixture["provider"] = json.RawMessage(sp.providerJSON)
		release := fixture["_modelfusion_execution"].(managedRelease)
		release.Descriptor.UpstreamModel = sp.upstreamModel
		release.Descriptor.AdapterRef = "ai-proxy.native-text.v1:" + sp.inputProtocol + ":" + sp.upstreamProtocol
		release.Descriptor.Transport.PathAdapterRef = release.Descriptor.AdapterRef
		release.Descriptor.Transport.Host = sp.authority
		release.Descriptor.Transport.SNI = sp.authority
		descRaw, err := json.Marshal(release.Descriptor)
		require.NoError(f, err)
		release.Descriptor.DescriptorDigest, err = managedDescriptorDigest(descRaw)
		require.NoError(f, err)
		fixture["_modelfusion_execution"] = release

		settings := fixture["managed_execution"].(managedSettings)
		settings.Authority = sp.authority
		settings.InputProtocol = sp.inputProtocol
		settings.UpstreamProtocol = sp.upstreamProtocol
		settings.UpstreamPath = sp.upstreamPath
		settings.SendTransport = "router"
		settings.StreamProjection = true
		if sp.inputProtocol != managedProtocolChat {
			settings.StreamProtocol = sp.inputProtocol
		}
		settings.ResponseProfile = sp.profile
		fixture["managed_execution"] = settings
		fixture["protocol_mode"] = "auto"

		rawCfg, err := json.Marshal(fixture)
		require.NoError(f, err)
		val := gjson.ParseBytes(rawCfg)
		var pluginCfg config.PluginConfig
		pluginCfg.FromJson(val)
		require.NoError(f, pluginCfg.Validate())
		require.NoError(f, pluginCfg.Complete())
		require.NoError(f, configureManagedExecution(val, &pluginCfg))
		parsedConfigs = append(parsedConfigs, pluginCfg.ManagedExecution.(*managedExecutionConfig))
	}

	runCanonicalRouterStreamAttempt := func(cfg *managedExecutionConfig, publicReq []byte, chunks [][]byte) ([]byte, bool, bool, managedProtocolObservation) {
		decodedReq, err := provider.DecodeCanonicalRequest(canonical.Protocol(cfg.profile.InputProtocol), publicReq)
		if err != nil {
			return nil, true, false, managedProtocolObservation{}
		}
		statusCode := uint16(200)
		respHeaders := [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}
		state := &managedExecutionState{
			config:        cfg,
			sourceConfig:  cfg,
			callID:        "50000000-0000-0000-0000-000000000099",
			attemptID:     "60000000-0000-0000-0000-000000000099",
			businessUTC:   "2026-10-27T10:00:00Z",
			publicRequest: bytes.Clone(publicReq),
			baseCanonical: &decodedReq,
			streaming:     true,
		}
		r := &managedRouterResponse{
			headers: respHeaders,
			status:  &statusCode,
			digest:  sha256.New(),
		}
		state.router = r
		rc, initErr := newManagedCanonicalRouterStream(nil, state)
		r.canonical = rc
		if initErr != nil {
			r.projectionFailed = true
		}
		if cfg.vendor != nil && cfg.vendor.NativeResponseFormat() != "" {
			r.supplier = &managedRouterSupplierStream{
				state:       state,
				format:      cfg.vendor.NativeResponseFormat(),
				finished:    make(map[string]string),
				collectText: true,
				incremental: rc.target.IncrementalOutputMode != provider.IncrementalForceFalse,
			}
		} else {
			switch cfg.profile.UpstreamProtocol {
			case managedProtocolChat:
				r.chat = &managedChatStream{collectText: true}
			case managedProtocolDashScopeText, managedProtocolDashScopeMultimodal:
				r.dashScope = &managedDashScopeStream{
					protocol:    cfg.profile.UpstreamProtocol,
					incremental: rc.target.IncrementalOutputMode != provider.IncrementalForceFalse,
				}
			}
		}

		var out bytes.Buffer
		if len(chunks) == 0 {
			out.Write(projectManagedRouterChunk(nil, state, nil, true))
		} else {
			for i, ch := range chunks {
				out.Write(projectManagedRouterChunk(nil, state, ch, i == len(chunks)-1))
			}
		}
		return out.Bytes(), r.projectionFailed, r.projectedError, r.streamObservation()
	}

	// Pre-compute normalized reference outputs for clean per-attempt isolation verification.
	cleanNormRefs := make([]string, len(specs))
	for idx, sp := range specs {
		cleanOut, failed, projErr, obs := runCanonicalRouterStreamAttempt(parsedConfigs[idx], sp.publicReq, [][]byte{sp.cleanStream})
		require.False(f, failed)
		require.False(f, projErr)
		require.Equal(f, "succeeded", obs.ExecutionResult)
		require.True(f, obs.BodyComplete)
		cleanNormRefs[idx] = canonicalizeE2EPublicPFIDs(cleanOut)
	}

	// Seed rich SSE streams and diverse split masks.
	f.Add(uint8(0), cleanChatSSE, []byte{1, 1, 1, 2, 3, 5, 8, 13})
	f.Add(uint8(0), append([]byte{0xef, 0xbb, 0xbf}, bytes.ReplaceAll(cleanChatSSE, []byte("\n"), []byte("\r\n"))...), []byte{1, 2, 4, 7, 11})
	f.Add(uint8(0), bytes.ReplaceAll(cleanChatSSE, []byte("\n"), []byte("\r")), []byte{3, 1, 4, 1, 5, 9})
	f.Add(uint8(0), splitSSEEventPayloadIntoMultiDataLines(cleanChatSSE), []byte{5, 7, 2, 19})
	f.Add(uint8(0), []byte(": keep-alive\ndata: {\"id\":\"c-t\",\"model\":\"fuzz-model\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"fn_a\",\"arguments\":\"{\\\"x\\\":9007199254740993}\"}}]}}]}\n\ndata: {\"id\":\"c-t\",\"model\":\"fuzz-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":5,\"total_tokens\":14}}\n\ndata: [DONE]\n\n"), []byte{2, 6, 1, 9, 4})
	f.Add(uint8(1), cleanDashIncSSE, []byte{1, 3, 2, 5, 1, 8})
	f.Add(uint8(2), cleanDashCumSSE, []byte{4, 1, 2, 9, 3})
	f.Add(uint8(3), cleanGeminiSSE, []byte{1, 2, 1, 7, 4, 3})
	f.Add(uint8(3), []byte("data: {\"promptFeedback\":{\"blockReason\":\"SAFETY\"}}\n\n"), []byte{1, 5, 3})
	f.Add(uint8(1), []byte("event: error\ndata: {\"code\":\"Throttling.RateQuota\",\"message\":\"rate limit\"}\n\n"), []byte{2, 4, 6})
	f.Add(uint8(0), []byte("data: {\"broken_json\"\n\n"), []byte{1, 2, 3})
	f.Add(uint8(0), []byte{0x64, 0x61, 0x74, 0x61, 0x3a, 0x20, 0xff, 0x0a, 0x0a}, []byte{1, 1, 1})
	f.Add(uint8(2), []byte("data: [DONE]\n\ndata: {\"extra\":1}\n\n"), []byte{3, 2, 4})

	f.Fuzz(func(t *testing.T, routeSelector uint8, rawStream []byte, splitMask []byte) {
		if len(rawStream) > 64*1024 {
			return
		}
		idx := int(routeSelector) % len(specs)
		cfg := parsedConfigs[idx]
		sp := specs[idx]

		wholeOut, wholeFailed, wholeProjErr, wholeObs := runCanonicalRouterStreamAttempt(cfg, sp.publicReq, [][]byte{rawStream})

		// Partition rawStream according to splitMask (including 0-length chunk stress when mask byte == 0).
		var chunks [][]byte
		if len(rawStream) > 0 {
			pos := 0
			maskIdx := 0
			for pos < len(rawStream) {
				var step int
				if len(splitMask) == 0 {
					step = 1
				} else {
					b := int(splitMask[maskIdx%len(splitMask)])
					maskIdx++
					if b == 0 && len(chunks) < 8 {
						chunks = append(chunks, rawStream[pos:pos])
						continue
					}
					step = (b % 37) + 1
				}
				if pos+step > len(rawStream) {
					step = len(rawStream) - pos
				}
				chunks = append(chunks, rawStream[pos:pos+step])
				pos += step
			}
		}

		partOut, partFailed, partProjErr, partObs := runCanonicalRouterStreamAttempt(cfg, sp.publicReq, chunks)

		if !wholeFailed && !wholeProjErr {
			require.False(t, partFailed, "valid whole stream failed when partitioned")
			require.False(t, partProjErr, "valid whole stream projected error when partitioned")
			require.Equal(t, wholeObs, partObs, "streamObservation diverged under partitioning")
			require.Equal(t, canonicalizeE2EPublicPFIDs(wholeOut), canonicalizeE2EPublicPFIDs(partOut), "canonical SSE output diverged under partitioning")
		} else if wholeFailed {
			require.True(t, partFailed || partProjErr, "invalid stream must never become a verified terminal under partitioning")
		}

		// Verify per-attempt state isolation on the shared *managedExecutionConfig immediately after fuzz input.
		afterOut, afterFailed, afterProjErr, afterObs := runCanonicalRouterStreamAttempt(cfg, sp.publicReq, [][]byte{sp.cleanStream})
		require.False(t, afterFailed)
		require.False(t, afterProjErr)
		require.Equal(t, "succeeded", afterObs.ExecutionResult)
		require.True(t, afterObs.BodyComplete)
		require.Equal(t, cleanNormRefs[idx], canonicalizeE2EPublicPFIDs(afterOut))
	})
}

// 3. TestCanonicalC04GeminiSixCellTwoTurnHistory (C04 + T19)
// Covers all 3 clients (Chat / Responses / Messages) x {JSON, SSE} = 6 cells -> Gemini generateContent / streamGenerateContent:
//   - Outbound Turn 1 & Turn 2 functionCall + functionResponse (recovering function name by call_id, preserving 9007199254740993,
//     thoughtSignature forwarded only for Google origin and never forged from foreign Anthropic signatures, multi-system merged,
//     maxOutputTokens preserving max_completion_tokens/max_output_tokens/max_tokens, Data URL -> inlineData, remote URL 400 + 0 sends)
//   - Inbound thought:true -> reasoning, promptFeedback.blockReason non-retryable policy rejection, and GE02 exact counters.
func TestCanonicalC04GeminiSixCellTwoTurnHistory(t *testing.T) {
	const toolSchema = `{"type":"object","properties":{"order_id":{"type":"integer","maximum":9007199254740993}},"required":["order_id"],"additionalProperties":false}`
	newGeminiHost := func(t *testing.T, inputProtocol string, router bool) *managedHost {
		t.Helper()
		if router {
			return newE2EMatrixRouterHost(
				t,
				`{"type":"gemini","apiTokens":["controlled-upstream-key"],"apiVersion":"v1beta"}`,
				"generativelanguage.googleapis.com",
				inputProtocol,
				managedProtocolChat,
				"/v1beta/models/gemini-3-flash-preview:streamGenerateContent",
				"gemini-3-flash-preview",
				"",
			)
		}
		fixture := managedConfigFixture()
		fixture["provider"] = json.RawMessage(`{"type":"gemini","apiTokens":["controlled-upstream-key"],"apiVersion":"v1beta"}`)
		release := fixture["_modelfusion_execution"].(managedRelease)
		release.Descriptor.UpstreamModel = "gemini-3-flash-preview"
		release.Descriptor.Transport.Host = "generativelanguage.googleapis.com"
		release.Descriptor.Transport.SNI = "generativelanguage.googleapis.com"
		descRaw, err := json.Marshal(release.Descriptor)
		require.NoError(t, err)
		release.Descriptor.DescriptorDigest, err = managedDescriptorDigest(descRaw)
		require.NoError(t, err)
		fixture["_modelfusion_execution"] = release

		settings := fixture["managed_execution"].(managedSettings)
		settings.Authority = "generativelanguage.googleapis.com"
		settings.InputProtocol = inputProtocol
		settings.UpstreamProtocol = managedProtocolChat
		fixture["managed_execution"] = settings
		fixture["protocol_mode"] = "auto"
		return newManagedHost(t, fixture)
	}

	// GE02 usageMetadata: promptTokenCount=100 (includes cachedContentTokenCount=40), toolUsePromptTokenCount=10 -> input=110, cache_hit=40, cache_miss=70;
	// candidatesTokenCount=20, thoughtsTokenCount=15 -> output=35, reasoning_tokens=15; totalTokenCount=145.
	const ge02Usage = `{"promptTokenCount":100,"cachedContentTokenCount":40,"toolUsePromptTokenCount":10,"candidatesTokenCount":20,"thoughtsTokenCount":15,"totalTokenCount":145}`

	for _, inProto := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", inProto, stream), func(t *testing.T) {
				var replayCallID string
				savedResources := make(map[string]string)
				saveResources := func(t *testing.T, h *managedHost) {
					t.Helper()
					for _, call := range h.host.GetRedisCalloutAttributesFromContext(h.id) {
						if !bytes.Contains(call.Query, []byte("modelfusion-public-resource-write.v1")) {
							continue
						}
						command, _, err := resp.NewReader(bytes.NewReader(call.Query)).ReadValue()
						require.NoError(t, err)
						parts := command.Array()
						count := parts[2].Integer()
						for i := range count {
							value := parts[4+count+i].String()
							require.NotContains(t, value, "sig_google_turn1", "the registry retains only IDs and ownership")
							require.NotContains(t, value, "native-gemini-call", "the provider ID travels with the signature")
							savedResources[parts[3+i].String()] = value
						}
						h.host.CallOnRedisCallResponse(call.CalloutID, 0, []byte(":1\r\n"))
					}
				}
				t.Run("turn1", func(t *testing.T) {
					// --- Turn 1: Multi-system + Data URL image + token limit + tool -> Gemini ---
					h1 := newGeminiHost(t, inProto, stream)
					var turn1Pub []byte
					switch inProto {
					case managedProtocolChat:
						turn1Pub = []byte(fmt.Sprintf(`{
						"model":"public-model",
						"stream":%t,
						"stream_options":{"include_usage":true},
						"max_completion_tokens":256,
						"messages":[
							{"role":"system","content":"System rule one."},
							{"role":"system","content":"System rule two."},
							{"role":"user","content":[
								{"type":"text","text":"Inspect order label"},
								{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgo="}}
							]}
						],
						"tools":[{"type":"function","function":{"name":"lookup_order","description":"Lookup order","parameters":%s}}],
						"tool_choice":"auto"
					}`, stream, toolSchema))
					case managedProtocolResponses:
						turn1Pub = []byte(fmt.Sprintf(`{
						"model":"public-model",
						"stream":%t,
						"instructions":"System rule one.",
						"max_output_tokens":256,
						"input":[
							{"role":"developer","content":"System rule two."},
							{"role":"user","content":[
								{"type":"input_text","text":"Inspect order label"},
								{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KGgo="}
							]}
						],
						"tools":[{"type":"function","name":"lookup_order","description":"Lookup order","parameters":%s}],
						"tool_choice":"auto"
					}`, stream, toolSchema))
					case managedProtocolMessages:
						turn1Pub = []byte(fmt.Sprintf(`{
						"model":"public-model",
						"stream":%t,
						"max_tokens":256,
						"system":[{"type":"text","text":"System rule one."},{"type":"text","text":"System rule two."}],
						"messages":[
							{"role":"user","content":[
								{"type":"text","text":"Inspect order label"},
								{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgo="}}
							]}
						],
						"tools":[{"name":"lookup_order","description":"Lookup order","input_schema":%s}],
						"tool_choice":{"type":"auto"}
					}`, stream, toolSchema))
					}

					h1.start(t, turn1Pub, bytes.Replace(turn1Pub, []byte(`"public-model"`), []byte(`"gemini-3-flash-preview"`), 1))

					var out1 []byte
					var reqHeaders1 [][2]string
					if stream {
						reqHeaders1 = h1.host.GetCurrentRequestHeaders(h1.id)
						out1 = h1.host.GetCurrentRequestBody(h1.id)
					} else {
						calls1 := h1.host.GetCalloutAttributesFromContext(h1.id)
						require.Len(t, calls1, 1)
						reqHeaders1 = calls1[0].Headers
						out1 = calls1[0].Body
					}

					gotPath1, err := managedSingleHeader(reqHeaders1, ":path")
					require.NoError(t, err)
					if stream {
						require.Equal(t, "/v1beta/models/gemini-3-flash-preview:streamGenerateContent?alt=sse", gotPath1)
					} else {
						require.Equal(t, "/v1beta/models/gemini-3-flash-preview:generateContent", gotPath1)
					}
					apiKey1, err := managedSingleHeader(reqHeaders1, "x-goog-api-key")
					require.NoError(t, err)
					require.Equal(t, "controlled-upstream-key", apiKey1)

					// Multi-system merged into systemInstruction.parts; maxOutputTokens preserved; Data URL -> inlineData.
					require.Equal(t, int64(2), gjson.GetBytes(out1, "systemInstruction.parts.#").Int(), string(out1))
					require.Equal(t, "System rule one.", gjson.GetBytes(out1, "systemInstruction.parts.0.text").String())
					require.Equal(t, "System rule two.", gjson.GetBytes(out1, "systemInstruction.parts.1.text").String())
					require.Equal(t, int64(256), gjson.GetBytes(out1, "generationConfig.maxOutputTokens").Int(), string(out1))
					require.Equal(t, "Inspect order label", gjson.GetBytes(out1, "contents.0.parts.0.text").String())
					require.Equal(t, "image/png", gjson.GetBytes(out1, "contents.0.parts.1.inlineData.mimeType").String())
					require.Equal(t, "iVBORw0KGgo=", gjson.GetBytes(out1, "contents.0.parts.1.inlineData.data").String())
					require.Equal(t, "lookup_order", gjson.GetBytes(out1, "tools.0.functionDeclarations.0.name").String())
					require.Equal(t, toolSchema, gjson.GetBytes(out1, "tools.0.functionDeclarations.0.parametersJsonSchema").Raw)
					require.False(t, gjson.GetBytes(out1, "tools.0.functionDeclarations.0.parameters").Exists())
					require.Equal(t, "AUTO", gjson.GetBytes(out1, "toolConfig.functionCallingConfig.mode").String())

					var resp1Str string
					var deliv1 executionfacts.Envelope
					if stream {
						var injected1 bytes.Buffer
						origInject := injectManagedRouterTerminal
						t.Cleanup(func() { injectManagedRouterTerminal = origInject })
						injectManagedRouterTerminal = func(body []byte, _ bool) error {
							injected1.Write(body)
							return nil
						}
						require.NoError(t, h1.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
						require.Equal(t, types.ActionContinue, h1.host.CallOnResponseHeaders(h1.id, [][2]string{
							{":status", "200"},
							{"content-type", "text/event-stream"},
						}, false))
						sse1 := "data: {\"responseId\":\"gem-c04-1\",\"modelVersion\":\"gemini-3-flash-preview\",\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"thought\":true,\"text\":\"Validating order 9007199254740993...\"}]}}]}\n\n" +
							"data: {\"responseId\":\"gem-c04-1\",\"modelVersion\":\"gemini-3-flash-preview\",\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"functionCall\":{\"name\":\"lookup_order\",\"args\":{\"order_id\":9007199254740993}},\"thoughtSignature\":\"sig_google_turn1\"}]}}]}\n\n" +
							"data: {\"responseId\":\"gem-c04-1\",\"modelVersion\":\"gemini-3-flash-preview\",\"candidates\":[{\"index\":0,\"finishReason\":\"STOP\"}],\"usageMetadata\":" + ge02Usage + "}\n\n"
						h1.host.CallOnResponseBody(h1.id, []byte(sse1), true)
						saveResources(t, h1)
						drainDashScopeRedis(h1)
						resp1Str = string(h1.host.GetCurrentResponseBody(h1.id)) + injected1.String()
						deliv1 = h1.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState).lastDelivery.Envelope()
					} else {
						json1 := []byte(`{"responseId":"gem-c04-1","modelVersion":"gemini-3-flash-preview","candidates":[{"index":0,"content":{"role":"model","parts":[{"thought":true,"text":"Validating order 9007199254740993..."},{"functionCall":{"id":"native-gemini-call","name":"lookup_order","args":{"order_id":9007199254740993}},"thoughtSignature":"sig_google_turn1"}]},"finishReason":"STOP"}],"usageMetadata":` + ge02Usage + `}`)
						h1.rawResponse(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, json1)
						saveResources(t, h1)
						sent1 := h1.host.GetSentLocalResponse(h1.id)
						require.NotNil(t, sent1)
						require.EqualValues(t, 200, sent1.StatusCode)
						resp1Str = string(sent1.Data)
						deliv1 = h1.bridge(t).Envelope()
					}

					// Verify GE02 accounting quantities (no double-counted cache/tool tokens).
					require.Equal(t, "succeeded", deliv1.DeliveryState)
					require.Equal(t, "reported", deliv1.Usage.Status)
					require.Equal(t, "110", deliv1.Usage.Quantities["input"])
					require.Equal(t, "35", deliv1.Usage.Quantities["output"])
					require.Equal(t, "40", deliv1.Usage.Quantities["cache_hit"])
					require.Equal(t, "70", deliv1.Usage.Quantities["cache_miss"])
					require.Equal(t, "15", deliv1.Usage.Quantities["reasoning_tokens"])

					// Verify thought:true routed exclusively to reasoning (never leaked to visible text) + exact 9007199254740993 + public usage.
					require.Contains(t, resp1Str, "Validating order 9007199254740993...")
					require.Contains(t, resp1Str, "9007199254740993")
					require.NotContains(t, resp1Str, "sig_google_turn1")
					require.NotContains(t, resp1Str, "native-gemini-call")
					if stream {
						events, err := clientresponse.Events([]byte(resp1Str))
						require.NoError(t, err)
						path := map[string]string{
							managedProtocolChat:      "choices.0.delta.tool_calls.0.id",
							managedProtocolResponses: "item.call_id",
							managedProtocolMessages:  "content_block.id",
						}[inProto]
						for _, event := range events {
							if id := gjson.GetBytes(event.Data, path).String(); id != "" {
								replayCallID = id
								break
							}
						}
					} else {
						path := map[string]string{
							managedProtocolChat:      "choices.0.message.tool_calls.0.id",
							managedProtocolResponses: "output.1.call_id",
							managedProtocolMessages:  "content.1.id",
						}[inProto]
						replayCallID = gjson.Get(resp1Str, path).String()
					}
					require.NotEmpty(t, replayCallID)
					for _, value := range savedResources {
						require.NotContains(t, value, replayCallID, "the continuation payload belongs to the client")
					}
					switch inProto {
					case managedProtocolChat:
						if !stream {
							require.Equal(t, "Validating order 9007199254740993...", gjson.Get(resp1Str, "choices.0.message.reasoning_content").String())
							require.Equal(t, gjson.Null, gjson.Get(resp1Str, "choices.0.message.content").Type)
							require.Equal(t, "tool_calls", gjson.Get(resp1Str, "choices.0.finish_reason").String())
							require.Equal(t, int64(110), gjson.Get(resp1Str, "usage.prompt_tokens").Int())
							require.Equal(t, int64(40), gjson.Get(resp1Str, "usage.prompt_tokens_details.cached_tokens").Int())
							require.Equal(t, int64(35), gjson.Get(resp1Str, "usage.completion_tokens").Int())
							require.Equal(t, int64(15), gjson.Get(resp1Str, "usage.completion_tokens_details.reasoning_tokens").Int())
						} else {
							require.Contains(t, resp1Str, `"reasoning_content":"Validating order 9007199254740993..."`)
							require.NotContains(t, resp1Str, `"content":"Validating order`)
							require.Contains(t, resp1Str, `"prompt_tokens":110`)
							require.Contains(t, resp1Str, `"completion_tokens":35`)
						}
					case managedProtocolResponses:
						if !stream {
							require.Equal(t, "reasoning", gjson.Get(resp1Str, "output.0.type").String())
							require.Equal(t, "function_call", gjson.Get(resp1Str, "output.1.type").String())
							require.Equal(t, int64(110), gjson.Get(resp1Str, "usage.input_tokens").Int())
							require.Equal(t, int64(40), gjson.Get(resp1Str, "usage.input_tokens_details.cached_tokens").Int())
							require.Equal(t, int64(35), gjson.Get(resp1Str, "usage.output_tokens").Int())
							require.Equal(t, int64(15), gjson.Get(resp1Str, "usage.output_tokens_details.reasoning_tokens").Int())
						} else {
							require.Contains(t, resp1Str, `"type":"response.reasoning_text.delta"`)
							require.NotContains(t, resp1Str, `"type":"response.output_text.delta"`)
							require.Contains(t, resp1Str, `"input_tokens":110`)
							require.Contains(t, resp1Str, `"output_tokens":35`)
						}
					case managedProtocolMessages:
						if !stream {
							require.Equal(t, "thinking", gjson.Get(resp1Str, "content.0.type").String())
							require.Equal(t, "tool_use", gjson.Get(resp1Str, "content.1.type").String())
							require.Equal(t, "tool_use", gjson.Get(resp1Str, "stop_reason").String())
							require.Equal(t, int64(70), gjson.Get(resp1Str, "usage.input_tokens").Int())
							require.Equal(t, int64(40), gjson.Get(resp1Str, "usage.cache_read_input_tokens").Int())
							require.Equal(t, int64(35), gjson.Get(resp1Str, "usage.output_tokens").Int())
						} else {
							require.Contains(t, resp1Str, `"type":"thinking_delta"`)
							require.NotContains(t, resp1Str, `"type":"text_delta"`)
							require.Contains(t, resp1Str, `"stop_reason":"tool_use"`)
							require.Contains(t, resp1Str, `"output_tokens":35`)
						}
					}
					h1.host.CompleteHttpContext(h1.id)
				})

				t.Run("turn2", func(t *testing.T) {
					// --- Turn 2: Two-turn functionCall + functionResponse outbound verification ---
					h2 := newGeminiHost(t, inProto, stream)
					var turn2Pub []byte
					switch inProto {
					case managedProtocolChat:
						turn2Pub = []byte(fmt.Sprintf(`{
						"model":"public-model",
						"stream":%t,
						"stream_options":{"include_usage":true},
						"messages":[
							{"role":"user","content":"Inspect order label"},
							{"role":"assistant","reasoning_content":"Validating order...","tool_calls":[{"id":"call_ord_99","type":"function","function":{"name":"lookup_order","arguments":"{\"order_id\":9007199254740993}"}}]},
							{"role":"tool","tool_call_id":"call_ord_99","content":"{\"status\":\"shipped\",\"tracking_no\":9007199254740993}"}
						],
						"tools":[{"type":"function","function":{"name":"lookup_order","parameters":%s}}]
					}`, stream, toolSchema))
					case managedProtocolResponses:
						turn2Pub = []byte(fmt.Sprintf(`{
						"model":"public-model",
						"stream":%t,
						"input":[
							{"role":"user","content":"Inspect order label"},
							{"type":"reasoning","content":[{"type":"reasoning_text","text":"Validating order..."}]},
							{"type":"function_call","call_id":"call_ord_99","name":"lookup_order","arguments":"{\"order_id\":9007199254740993}"},
							{"type":"function_call_output","call_id":"call_ord_99","output":"{\"status\":\"shipped\",\"tracking_no\":9007199254740993}"}
						],
						"tools":[{"type":"function","name":"lookup_order","parameters":%s}]
					}`, stream, toolSchema))
					case managedProtocolMessages:
						// Includes foreign Anthropic signature "sig_anthropic_foreign" which MUST NOT be forged as Gemini thoughtSignature.
						turn2Pub = []byte(fmt.Sprintf(`{
						"model":"public-model",
						"stream":%t,
						"max_tokens":256,
						"messages":[
							{"role":"user","content":"Inspect order label"},
							{"role":"assistant","content":[
								{"type":"thinking","thinking":"Validating order...","signature":"sig_anthropic_foreign"},
								{"type":"tool_use","id":"call_ord_99","name":"lookup_order","input":{"order_id":9007199254740993}}
							]},
							{"role":"user","content":[
								{"type":"tool_result","tool_use_id":"call_ord_99","content":"{\"status\":\"shipped\",\"tracking_no\":9007199254740993}"}
							]}
						],
						"tools":[{"name":"lookup_order","input_schema":%s}]
					}`, stream, toolSchema))
					}

					require.NotEmpty(t, replayCallID)
					turn2Pub = bytes.ReplaceAll(turn2Pub, []byte("call_ord_99"), []byte(replayCallID))
					h2.start(t, turn2Pub, bytes.Replace(turn2Pub, []byte(`"public-model"`), []byte(`"gemini-3-flash-preview"`), 1))
					for _, call := range h2.host.GetRedisCalloutAttributesFromContext(h2.id) {
						command, _, err := resp.NewReader(bytes.NewReader(call.Query)).ReadValue()
						require.NoError(t, err)
						parts := command.Array()
						if parts[0].String() != "MGET" {
							continue
						}
						var reply []resp.Value
						for _, key := range parts[1:] {
							record, ok := savedResources[key.String()]
							require.True(t, ok, "continuation must use a record persisted from turn 1")
							reply = append(reply, resp.StringValue(record))
						}
						var raw bytes.Buffer
						require.NoError(t, resp.NewWriter(&raw).WriteArray(reply))
						h2.host.CallOnRedisCallResponse(call.CalloutID, 0, raw.Bytes())
					}
					state := h2.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
					require.NotNil(t, state.resourceOwner, "continuation must select the original owner before credential qualification")
					var out2 []byte
					if stream {
						out2 = h2.host.GetCurrentRequestBody(h2.id)
					} else {
						calls2 := h2.host.GetCalloutAttributesFromContext(h2.id)
						require.Len(t, calls2, 1)
						out2 = calls2[0].Body
					}

					// Verify Turn 2 model functionCall + user functionResponse (name recovered via call_id, 9007199254740993 preserved, foreign signature stripped).
					require.Equal(t, toolSchema, gjson.GetBytes(out2, "tools.0.functionDeclarations.0.parametersJsonSchema").Raw)
					require.False(t, gjson.GetBytes(out2, "tools.0.functionDeclarations.0.parameters").Exists())
					require.Equal(t, "model", gjson.GetBytes(out2, "contents.1.role").String(), string(out2))
					require.True(t, gjson.GetBytes(out2, "contents.1.parts.0.thought").Bool(), string(out2))
					require.False(t, gjson.GetBytes(out2, "contents.1.parts.0.thoughtSignature").Exists(), "foreign signature must not be forwarded to Gemini: %s", string(out2))
					require.Equal(t, "lookup_order", gjson.GetBytes(out2, "contents.1.parts.1.functionCall.name").String())
					require.Equal(t, "9007199254740993", gjson.GetBytes(out2, "contents.1.parts.1.functionCall.args.order_id").Raw)
					require.Equal(t, "sig_google_turn1", gjson.GetBytes(out2, "contents.1.parts.1.thoughtSignature").String())
					callID := gjson.GetBytes(out2, "contents.1.parts.1.functionCall.id")

					require.Equal(t, "user", gjson.GetBytes(out2, "contents.2.role").String(), string(out2))
					require.Equal(t, "lookup_order", gjson.GetBytes(out2, "contents.2.parts.0.functionResponse.name").String(), "functionResponse must recover function name by call_id")
					resultID := gjson.GetBytes(out2, "contents.2.parts.0.functionResponse.id")
					if stream {
						require.False(t, callID.Exists())
						require.False(t, resultID.Exists())
					} else {
						require.Equal(t, "native-gemini-call", callID.String())
						require.Equal(t, callID.String(), resultID.String())
					}
					require.Equal(t, "shipped", gjson.GetBytes(out2, "contents.2.parts.0.functionResponse.response.status").String())
					require.Equal(t, "9007199254740993", gjson.GetBytes(out2, "contents.2.parts.0.functionResponse.response.tracking_no").Raw)

					if stream {
						var injected2 bytes.Buffer
						origInject2 := injectManagedRouterTerminal
						t.Cleanup(func() { injectManagedRouterTerminal = origInject2 })
						injectManagedRouterTerminal = func(body []byte, _ bool) error {
							injected2.Write(body)
							return nil
						}
						require.NoError(t, h2.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
						require.Equal(t, types.ActionContinue, h2.host.CallOnResponseHeaders(h2.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false))
						sse2 := "data: {\"responseId\":\"gem-c04-2\",\"modelVersion\":\"gemini-3-flash-preview\",\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Order shipped.\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":140,\"candidatesTokenCount\":6,\"totalTokenCount\":146}}\n\n"
						h2.host.CallOnResponseBody(h2.id, []byte(sse2), true)
						drainDashScopeRedis(h2)
						resp2Str := string(h2.host.GetCurrentResponseBody(h2.id)) + injected2.String()
						require.Contains(t, resp2Str, "Order shipped.")
					} else {
						json2 := []byte(`{"responseId":"gem-c04-2","modelVersion":"gemini-3-flash-preview","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Order shipped."}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":140,"candidatesTokenCount":6,"totalTokenCount":146}}`)
						h2.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, json2)
						sent2 := h2.host.GetSentLocalResponse(h2.id)
						require.NotNil(t, sent2)
						require.EqualValues(t, 200, sent2.StatusCode)
						require.Contains(t, string(sent2.Data), "Order shipped.")
					}
					h2.host.CompleteHttpContext(h2.id)
				})
			})
		}
	}

	// Verify Google-originated thoughtSignature IS preserved on Gemini outbound encoding (R06 #6 / T12).
	t.Run("google_origin_thought_signature_forwarded_cross_origin_dropped", func(t *testing.T) {
		gemTarget := provider.CanonicalTarget{
			WireFormat: provider.WireGeminiGenerateContent,
			Supplier:   "gemini",
			Model:      "gemini-3-flash-preview",
		}
		googleOrigin := canonical.OpaqueOrigin{Protocol: canonical.ProtocolGeminiGenerate, Supplier: "google"}
		anthropicOrigin := canonical.OpaqueOrigin{Protocol: canonical.ProtocolAnthropicMessages, Supplier: "anthropic"}
		req := canonical.Request{
			SourceProtocol: canonical.ProtocolOpenAIResponses,
			Model:          "gemini-3-flash-preview",
			Items: []canonical.Item{
				{Kind: canonical.ItemMessage, Role: canonical.RoleUser, Parts: []canonical.Part{{Kind: canonical.PartText, Text: "check"}}},
				{Kind: canonical.ItemReasoning, Role: canonical.RoleAssistant, Reasoning: &canonical.ReasoningItem{Text: []string{"google thought"}, Signature: "sig_google_ok", Origin: googleOrigin}},
				{Kind: canonical.ItemReasoning, Role: canonical.RoleAssistant, Reasoning: &canonical.ReasoningItem{Text: []string{"claude thought"}, Signature: "sig_claude_drop", Origin: anthropicOrigin}},
				{Kind: canonical.ItemFunctionCall, Role: canonical.RoleAssistant, Call: &canonical.FunctionCall{CallID: "c1", Name: "lookup_order", Args: canonical.Arguments{Kind: canonical.ArgsObject, Object: json.RawMessage(`{"order_id":9007199254740993}`)}, ThoughtSignature: "sig_google_call_ok", SignatureOrigin: googleOrigin}},
			},
		}
		encoded, err := provider.EncodeCanonicalRequest(req, gemTarget)
		require.NoError(t, err)
		require.Equal(t, "sig_google_ok", gjson.GetBytes(encoded, "contents.1.parts.0.thoughtSignature").String(), string(encoded))
		require.False(t, gjson.GetBytes(encoded, "contents.1.parts.1.thoughtSignature").Exists(), string(encoded))
		require.Equal(t, "sig_google_call_ok", gjson.GetBytes(encoded, "contents.1.parts.2.thoughtSignature").String(), string(encoded))
	})

	// Remote HTTPS image URL must be rejected locally with 400 + invalid_request_error + x-should-retry: false + 0 sends.
	t.Run("remote_image_url_rejected_locally_400_zero_sends", func(t *testing.T) {
		for _, inProto := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
			t.Run(inProto, func(t *testing.T) {
				h := newGeminiHost(t, inProto, false)
				var req []byte
				switch inProto {
				case managedProtocolChat:
					req = []byte(`{"model":"public-model","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://example.com/cat.png"}}]}]}`)
				case managedProtocolResponses:
					req = []byte(`{"model":"public-model","input":[{"role":"user","content":[{"type":"input_image","image_url":"https://example.com/cat.png"}]}]}`)
				case managedProtocolMessages:
					req = []byte(`{"model":"public-model","max_tokens":64,"messages":[{"role":"user","content":[{"type":"image","source":{"type":"url","url":"https://example.com/cat.png"}}]}]}`)
				}
				h.start(t, req, bytes.Replace(req, []byte("public-model"), []byte("gemini-3-flash-preview"), 1))
				require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id), "remote URL must be rejected before upstream send")
				sent := h.host.GetSentLocalResponse(h.id)
				require.NotNil(t, sent)
				require.EqualValues(t, 400, sent.StatusCode)
				require.Equal(t, "invalid_request_error", gjson.GetBytes(sent.Data, "error.type").String(), string(sent.Data))
				require.Equal(t, "false", managedResponseHeaderValue(sent.Headers, "x-should-retry"))
				h.host.CompleteHttpContext(h.id)
			})
		}
	})

	// promptFeedback.blockReason classified as non-retryable policy rejection across JSON and SSE.
	t.Run("prompt_feedback_block_reason_non_retryable_rejection", func(t *testing.T) {
		for _, inProto := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
			pubJ := []byte(`{"model":"public-model","messages":[{"role":"user","content":"blocked"}]}`)
			if inProto == managedProtocolResponses {
				pubJ = []byte(`{"model":"public-model","input":"blocked"}`)
			} else if inProto == managedProtocolMessages {
				pubJ = []byte(`{"model":"public-model","max_tokens":64,"messages":[{"role":"user","content":"blocked"}]}`)
			}
			t.Run(inProto+"/json", func(t *testing.T) {
				hj := newGeminiHost(t, inProto, false)
				hj.start(t, pubJ, bytes.Replace(pubJ, []byte("public-model"), []byte("gemini-3-flash-preview"), 1))
				hj.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, []byte(`{"promptFeedback":{"blockReason":"SAFETY","blockReasonMessage":"Blocked by safety policy"}}`))
				sentJ := hj.host.GetSentLocalResponse(hj.id)
				require.NotNil(t, sentJ)
				require.EqualValues(t, 400, sentJ.StatusCode)
				require.Contains(t, string(sentJ.Data), "content_policy_violation")
				require.Equal(t, "false", managedResponseHeaderValue(sentJ.Headers, "x-should-retry"))
				require.Equal(t, "failed", hj.bridge(t).Envelope().DeliveryState)
				hj.host.CompleteHttpContext(hj.id)
			})

			t.Run(inProto+"/sse", func(t *testing.T) {
				hs := newGeminiHost(t, inProto, true)
				pubS := bytes.Replace(pubJ, []byte(`"model":"public-model"`), []byte(`"model":"public-model","stream":true`), 1)
				hs.start(t, pubS, bytes.Replace(pubS, []byte("public-model"), []byte("gemini-3-flash-preview"), 1))
				require.NoError(t, hs.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
				require.Equal(t, types.ActionContinue, hs.host.CallOnResponseHeaders(hs.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false))
				hs.host.CallOnResponseBody(hs.id, []byte("data: {\"promptFeedback\":{\"blockReason\":\"SAFETY\"}}\n\n"), true)
				respS := string(hs.host.GetCurrentResponseBody(hs.id))
				require.Contains(t, respS, "content_policy_violation", respS)
				stateS := hs.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
				require.Equal(t, "failed", stateS.lastDelivery.Envelope().DeliveryState)
				hs.host.CompleteHttpContext(hs.id)
			})
		}
	})
}

// 4. TestCanonicalC05MoonshotSixCellJSONAndSSE (C05)
// Covers all 3 clients (Chat / Responses / Messages) x {JSON, SSE} = 6 cells -> Moonshot nested choices[0].usage,
// verifying lossless 9007199254740993 presentation and exact accounting Usage.Quantities.
func TestCanonicalC05MoonshotSixCellJSONAndSSE(t *testing.T) {
	for _, inProto := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", inProto, stream), func(t *testing.T) {
				var h *managedHost
				if stream {
					h = newE2EMatrixRouterHost(
						t,
						`{"type":"moonshot","apiTokens":["controlled-upstream-key"]}`,
						"api.moonshot.cn",
						inProto,
						managedProtocolChat,
						"/v1/chat/completions",
						"kimi-k2",
						"",
					)
				} else {
					fixture := managedConfigFixture()
					fixture["provider"] = json.RawMessage(`{"type":"moonshot","apiTokens":["controlled-upstream-key"]}`)
					release := fixture["_modelfusion_execution"].(managedRelease)
					release.Descriptor.UpstreamModel = "kimi-k2"
					release.Descriptor.Transport.Host = "api.moonshot.cn"
					release.Descriptor.Transport.SNI = "api.moonshot.cn"
					descRaw, err := json.Marshal(release.Descriptor)
					require.NoError(t, err)
					release.Descriptor.DescriptorDigest, err = managedDescriptorDigest(descRaw)
					require.NoError(t, err)
					fixture["_modelfusion_execution"] = release

					settings := fixture["managed_execution"].(managedSettings)
					settings.Authority = "api.moonshot.cn"
					settings.InputProtocol = inProto
					settings.UpstreamProtocol = managedProtocolChat
					fixture["managed_execution"] = settings
					fixture["protocol_mode"] = "auto"
					h = newManagedHost(t, fixture)
				}
				defer h.host.CompleteHttpContext(h.id)

				var pubReq []byte
				switch inProto {
				case managedProtocolChat:
					pubReq = []byte(fmt.Sprintf(`{"model":"public-model","stream":%t,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"Hello Kimi"}]}`, stream))
				case managedProtocolResponses:
					pubReq = []byte(fmt.Sprintf(`{"model":"public-model","stream":%t,"input":"Hello Kimi"}`, stream))
				case managedProtocolMessages:
					pubReq = []byte(fmt.Sprintf(`{"model":"public-model","stream":%t,"max_tokens":128,"messages":[{"role":"user","content":"Hello Kimi"}]}`, stream))
				}

				h.start(t, pubReq, bytes.Replace(pubReq, []byte("public-model"), []byte("kimi-k2"), 1))

				var publicResp []byte
				var respHeaders [][2]string
				var deliv executionfacts.Envelope

				if stream {
					var injected bytes.Buffer
					origInject := injectManagedRouterTerminal
					t.Cleanup(func() { injectManagedRouterTerminal = origInject })
					injectManagedRouterTerminal = func(body []byte, _ bool) error {
						injected.Write(body)
						return nil
					}
					require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
					respHeaders = [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}
					require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id, respHeaders, false))
					moonshotSSE := "data: {\"id\":\"cmpl-moon-1\",\"object\":\"chat.completion.chunk\",\"created\":1761559200,\"model\":\"kimi-k2\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Moonshot \"}}]}\n\n" +
						"data: {\"id\":\"cmpl-moon-1\",\"object\":\"chat.completion.chunk\",\"created\":1761559200,\"model\":\"kimi-k2\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"reply.\"},\"finish_reason\":\"stop\",\"usage\":{\"prompt_tokens\":9007199254740993,\"completion_tokens\":27,\"total_tokens\":9007199254741020,\"prompt_tokens_details\":{\"cached_tokens\":64}}}]}\n\n" +
						"data: [DONE]\n\n"
					h.host.CallOnResponseBody(h.id, []byte(moonshotSSE), true)
					drainDashScopeRedis(h)
					publicResp = append(h.host.GetCurrentResponseBody(h.id), injected.Bytes()...)
					deliv = h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState).lastDelivery.Envelope()
				} else {
					moonshotJSON := []byte(`{"id":"cmpl-moon-1","object":"chat.completion","created":1761559200,"model":"kimi-k2","choices":[{"index":0,"message":{"role":"assistant","content":"Moonshot reply."},"finish_reason":"stop"}],"usage":{"prompt_tokens":9007199254740993,"completion_tokens":27,"total_tokens":9007199254741020,"prompt_tokens_details":{"cached_tokens":64}}}`)
					h.response(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, moonshotJSON)
					sent := h.host.GetSentLocalResponse(h.id)
					require.NotNil(t, sent)
					require.EqualValues(t, 200, sent.StatusCode)
					publicResp = sent.Data
					respHeaders = sent.Headers
					deliv = h.bridge(t).Envelope()
				}

				// Verify accounting delivery usage from nested choices[0].usage.
				require.Equal(t, "succeeded", deliv.DeliveryState)
				require.Equal(t, "reported", deliv.Usage.Status)
				require.Equal(t, "9007199254740993", deliv.Usage.Quantities["input"])
				require.Equal(t, "27", deliv.Usage.Quantities["output"])
				require.Equal(t, "64", deliv.Usage.Quantities["cache_hit"])
				require.Equal(t, "9007199254740929", deliv.Usage.Quantities["cache_miss"])

				// Verify client-facing usage projection through native protocol observer.
				nativeProfile, err := newManagedProtocolProfile(inProto, inProto)
				require.NoError(t, err)
				observedClient := nativeProfile.ObserveResponse(200, respHeaders, publicResp, true)
				require.Equal(t, "accepted", observedClient.Acceptance)
				require.Equal(t, "9007199254740993", observedClient.Usage.Quantities["input"])
				require.Equal(t, "27", observedClient.Usage.Quantities["output"])
				require.Equal(t, "64", observedClient.Usage.Quantities["cache_hit"])
				require.Contains(t, string(publicResp), "Moonshot")
			})
		}

		// Also verify Moonshot nested choices[0].usage on buffered http_call SSE across all 3 clients.
		t.Run(inProto+"/buffered_httpcall_sse", func(t *testing.T) {
			fixture := managedConfigFixture()
			fixture["provider"] = json.RawMessage(`{"type":"moonshot","apiTokens":["controlled-upstream-key"]}`)
			release := fixture["_modelfusion_execution"].(managedRelease)
			release.Descriptor.UpstreamModel = "kimi-k2"
			release.Descriptor.Transport.Host = "api.moonshot.cn"
			release.Descriptor.Transport.SNI = "api.moonshot.cn"
			descRaw, err := json.Marshal(release.Descriptor)
			require.NoError(t, err)
			release.Descriptor.DescriptorDigest, err = managedDescriptorDigest(descRaw)
			require.NoError(t, err)
			fixture["_modelfusion_execution"] = release

			settings := fixture["managed_execution"].(managedSettings)
			settings.Authority = "api.moonshot.cn"
			settings.InputProtocol = inProto
			settings.UpstreamProtocol = managedProtocolChat
			fixture["managed_execution"] = settings
			fixture["protocol_mode"] = "auto"

			hb := newManagedHost(t, fixture)
			defer hb.host.CompleteHttpContext(hb.id)

			var pubReq []byte
			switch inProto {
			case managedProtocolChat:
				pubReq = []byte(`{"model":"public-model","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"Hello Kimi"}]}`)
			case managedProtocolResponses:
				pubReq = []byte(`{"model":"public-model","stream":true,"input":"Hello Kimi"}`)
			case managedProtocolMessages:
				pubReq = []byte(`{"model":"public-model","stream":true,"max_tokens":128,"messages":[{"role":"user","content":"Hello Kimi"}]}`)
			}
			hb.start(t, pubReq, bytes.Replace(pubReq, []byte("public-model"), []byte("kimi-k2"), 1))
			moonshotSSE := []byte(
				"data: {\"id\":\"cmpl-moon-buf\",\"object\":\"chat.completion.chunk\",\"created\":1761559200,\"model\":\"kimi-k2\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Buffered Moonshot.\"},\"finish_reason\":\"stop\",\"usage\":{\"prompt_tokens\":9007199254740993,\"completion_tokens\":27,\"total_tokens\":9007199254741020,\"prompt_tokens_details\":{\"cached_tokens\":64}}}]}\n\n" +
					"data: [DONE]\n\n",
			)
			hb.response(t, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, moonshotSSE)
			sent := hb.host.GetSentLocalResponse(hb.id)
			require.NotNil(t, sent)
			require.EqualValues(t, 200, sent.StatusCode)
			deliv := hb.bridge(t).Envelope()
			require.Equal(t, "succeeded", deliv.DeliveryState)
			require.Equal(t, "reported", deliv.Usage.Status)
			require.Equal(t, "9007199254740993", deliv.Usage.Quantities["input"])
			require.Equal(t, "27", deliv.Usage.Quantities["output"])
			require.Equal(t, "64", deliv.Usage.Quantities["cache_hit"])
		})
	}
}

// 5. TestCanonicalC03MultimodalAndC02TextImageReject (C02/C03 + H2 + T08/T09)
//   - Messages multimodal tool_result + mixed text/image across 3 clients x {JSON, SSE} -> dashscope_multimodal
//     emits companion user image message with zero Base64 re-encoding and preserves visual array content;
//   - Identical image inputs -> dashscope_text fail locally with 400 invalid_request_error + x-should-retry: false + 0 upstream sends;
//   - Responses function_call_output with image + detail:"original" / file_id fail locally with 400 + 0 upstream sends.
func TestCanonicalC03MultimodalAndC02TextImageReject(t *testing.T) {
	// Valid base64 payload with unpadded/distinctive valid characters to detect any decode+re-encode mutation.
	const exactRawBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	const exactDataURL = "data:image/png;base64," + exactRawBase64

	// Part A: C03 6-cell matrix (3 clients x JSON/SSE) + Messages multimodal tool_result -> dashscope_multimodal.
	for _, inProto := range []string{managedProtocolChat, managedProtocolResponses, managedProtocolMessages} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("c03_multimodal/%s/stream=%t", inProto, stream), func(t *testing.T) {
				h := newDashScopeConversionHost(t, inProto, managedProtocolDashScopeMultimodal, "qwen3.6-plus")
				defer h.host.CompleteHttpContext(h.id)

				var pubReq []byte
				switch inProto {
				case managedProtocolChat:
					pubReq = []byte(fmt.Sprintf(`{
						"model":"public-model",
						"stream":%t,
						"stream_options":{"include_usage":true},
						"messages":[
							{"role":"user","content":[
								{"type":"text","text":"Compare chart and photo"},
								{"type":"image_url","image_url":{"url":"%s"}},
								{"type":"image_url","image_url":{"url":"https://example.com/chart.png"}}
							]}
						]
					}`, stream, exactDataURL))
				case managedProtocolResponses:
					pubReq = []byte(fmt.Sprintf(`{
						"model":"public-model",
						"stream":%t,
						"input":[
							{"role":"user","content":[
								{"type":"input_text","text":"Compare chart and photo"},
								{"type":"input_image","image_url":"%s"},
								{"type":"input_image","image_url":"https://example.com/chart.png"}
							]}
						]
					}`, stream, exactDataURL))
				case managedProtocolMessages:
					pubReq = []byte(fmt.Sprintf(`{
						"model":"public-model",
						"stream":%t,
						"max_tokens":128,
						"messages":[
							{"role":"user","content":[
								{"type":"text","text":"Capture UI"},
								{"type":"image","source":{"type":"url","url":"https://example.com/chart.png"}}
							]},
							{"role":"assistant","content":[
								{"type":"tool_use","id":"toolu_shot_1","name":"take_screenshot","input":{"selector":"#app"}}
							]},
							{"role":"user","content":[
								{"type":"tool_result","tool_use_id":"toolu_shot_1","content":[
									{"type":"text","text":"Screenshot ready"},
									{"type":"image","source":{"type":"base64","media_type":"image/png","data":"%s"}}
								]}
							]}
						],
						"tools":[{"name":"take_screenshot","input_schema":{"type":"object"}}]
					}`, stream, exactRawBase64))
				}

				h.start(t, pubReq, bytes.Replace(pubReq, []byte("public-model"), []byte("qwen3.6-plus"), 1))
				outbound := h.host.GetCurrentRequestBody(h.id)
				require.NotEmpty(t, outbound)

				// Visual content arrays preserved + zero Base64 re-encoding verified on wire and baseCanonical.
				require.Contains(t, string(outbound), exactDataURL)
				require.Contains(t, string(outbound), "https://example.com/chart.png")
				require.True(t, gjson.GetBytes(outbound, "input.messages.0.content").IsArray(), string(outbound))

				state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
				require.NotNil(t, state.baseCanonical)

				if inProto == managedProtocolMessages {
					// H2: Messages multimodal tool_result emits role:"tool" (no name) + companion role:"user" image message!
					require.Equal(t, "tool", gjson.GetBytes(outbound, "input.messages.2.role").String(), string(outbound))
					require.Equal(t, "toolu_shot_1", gjson.GetBytes(outbound, "input.messages.2.tool_call_id").String())
					require.Equal(t, "Screenshot ready", gjson.GetBytes(outbound, "input.messages.2.content").String())
					require.False(t, gjson.GetBytes(outbound, "input.messages.2.name").Exists(), "DashScope tool message must omit name")

					require.Equal(t, "user", gjson.GetBytes(outbound, "input.messages.3.role").String(), string(outbound))
					require.Equal(t, "Attached image(s) from tool result:", gjson.GetBytes(outbound, "input.messages.3.content.0.text").String())
					require.Equal(t, exactDataURL, gjson.GetBytes(outbound, "input.messages.3.content.1.image").String())
					require.Equal(t, exactRawBase64, state.baseCanonical.Items[2].Result.Parts[1].Image.Base64Ref)
				}

				var injected bytes.Buffer
				origInject := injectManagedRouterTerminal
				t.Cleanup(func() { injectManagedRouterTerminal = origInject })
				injectManagedRouterTerminal = func(body []byte, _ bool) error {
					injected.Write(body)
					return nil
				}
				require.NoError(t, h.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
				if stream {
					require.Equal(t, types.ActionContinue, h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false))
					sse := "event:result\ndata:{\"request_id\":\"55555555-6666-4777-8888-999999999999\",\"output\":{\"choices\":[{\"finish_reason\":\"stop\",\"message\":{\"role\":\"assistant\",\"content\":[{\"text\":\"Visual analysis complete.\"}]}}]},\"usage\":{\"input_tokens\":320,\"output_tokens\":6,\"total_tokens\":326}}\n\n"
					h.host.CallOnResponseBody(h.id, []byte(sse), true)
				} else {
					require.Equal(t, types.HeaderStopIteration, h.host.CallOnResponseHeaders(h.id, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, false))
					js := `{"request_id":"55555555-6666-4777-8888-999999999999","output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":[{"text":"Visual analysis complete."}]}}]},"usage":{"input_tokens":320,"output_tokens":6,"total_tokens":326}}`
					h.host.CallOnResponseBody(h.id, []byte(js), true)
				}
				drainDashScopeRedis(h)
				respStr := string(h.host.GetCurrentResponseBody(h.id)) + injected.String()
				require.Contains(t, respStr, "Visual analysis complete.")
				require.Equal(t, "succeeded", state.lastDelivery.Envelope().DeliveryState)
			})
		}
	}

	// Part B: C02 dashscope_text rejects all image inputs (including Messages multimodal tool_result) locally with 400 + 0 sends.
	t.Run("c02_dashscope_text_rejects_images_400_zero_sends", func(t *testing.T) {
		rejectCases := []struct {
			name    string
			inProto string
			body    []byte
		}{
			{
				name:    "messages_multimodal_tool_result",
				inProto: managedProtocolMessages,
				body: []byte(fmt.Sprintf(`{
					"model":"public-model",
					"max_tokens":128,
					"messages":[
						{"role":"user","content":"Capture UI"},
						{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"shot","input":{}}]},
						{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"%s"}}]}]}
					]
				}`, exactRawBase64)),
			},
			{
				name:    "messages_user_mixed_text_image",
				inProto: managedProtocolMessages,
				body:    []byte(fmt.Sprintf(`{"model":"public-model","max_tokens":128,"messages":[{"role":"user","content":[{"type":"text","text":"look"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"%s"}}]}]}`, exactRawBase64)),
			},
			{
				name:    "chat_user_mixed_text_image",
				inProto: managedProtocolChat,
				body:    []byte(fmt.Sprintf(`{"model":"public-model","messages":[{"role":"user","content":[{"type":"text","text":"look"},{"type":"image_url","image_url":{"url":"%s"}}]}]}`, exactDataURL)),
			},
			{
				name:    "responses_user_mixed_text_image",
				inProto: managedProtocolResponses,
				body:    []byte(fmt.Sprintf(`{"model":"public-model","input":[{"role":"user","content":[{"type":"input_text","text":"look"},{"type":"input_image","image_url":"%s"}]}]}`, exactDataURL)),
			},
		}

		for _, rc := range rejectCases {
			t.Run(rc.name, func(t *testing.T) {
				h := newDashScopeConversionHost(t, rc.inProto, managedProtocolDashScopeText, "qwen-flash")
				defer h.host.CompleteHttpContext(h.id)
				h.start(t, rc.body, bytes.Replace(rc.body, []byte("public-model"), []byte("qwen-flash"), 1))

				state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
				require.False(t, state.dispatchStarted, "dashscope_text image rejection must occur before upstream dispatch")
				require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))

				sent := h.host.GetSentLocalResponse(h.id)
				require.NotNil(t, sent)
				require.EqualValues(t, 400, sent.StatusCode)
				require.Equal(t, "invalid_request_error", gjson.GetBytes(sent.Data, "error.type").String(), string(sent.Data))
				require.Equal(t, "false", managedResponseHeaderValue(sent.Headers, "x-should-retry"))
				require.NotEmpty(t, managedResponseHeaderValue(sent.Headers, "x-request-id"))
			})
		}
	})

	// Part C: T08/T09 Responses function_call_output with image & unsupported detail:"original" / file_id rejected locally with 400 + 0 sends.
	t.Run("responses_function_call_output_image_and_t09_guards_400_zero_sends", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			body []byte
		}{
			{
				name: "function_call_output_with_image",
				body: []byte(fmt.Sprintf(`{
					"model":"public-model",
					"input":[
						{"type":"function_call","call_id":"c_img","name":"shot","arguments":"{}"},
						{"type":"function_call_output","call_id":"c_img","output":[{"type":"input_image","image_url":"%s"}]}
					]
				}`, exactDataURL)),
			},
			{
				name: "input_image_detail_original",
				body: []byte(fmt.Sprintf(`{
					"model":"public-model",
					"input":[{"role":"user","content":[{"type":"input_image","image_url":"%s","detail":"original"}]}]
				}`, exactDataURL)),
			},
			{
				name: "input_image_file_id",
				body: []byte(`{
					"model":"public-model",
					"input":[{"role":"user","content":[{"type":"input_image","file_id":"file-xyz987"}]}]
				}`),
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				h := newDashScopeConversionHost(t, managedProtocolResponses, managedProtocolDashScopeMultimodal, "qwen3.6-plus")
				defer h.host.CompleteHttpContext(h.id)
				h.start(t, tc.body, bytes.Replace(tc.body, []byte("public-model"), []byte("qwen3.6-plus"), 1))

				state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
				require.False(t, state.dispatchStarted)
				require.Empty(t, h.host.GetCalloutAttributesFromContext(h.id))

				sent := h.host.GetSentLocalResponse(h.id)
				require.NotNil(t, sent)
				require.EqualValues(t, 400, sent.StatusCode)
				require.Equal(t, "invalid_request_error", gjson.GetBytes(sent.Data, "error.type").String(), string(sent.Data))
				require.Equal(t, "false", managedResponseHeaderValue(sent.Headers, "x-should-retry"))
			})
		}
	})
}

// 6. TestCanonicalT23AttemptIsolationAndHybridFallback (T23 + B02 + B03)
//   - Part 1: Hybrid fallback (same-wire primary openai_responses -> openai_responses 503 -> cross-wire fallback openai_responses -> dashscope_text)
//     and cross-wire -> cross-wire fallback (openai_responses -> openai_chat 503 -> openai_responses -> dashscope_text),
//     proving primary's removed fixed tools never pollute fallback and baseCanonical remains immutable.
//   - Part 2: B02 cross-turn salted slot alias non-collision: Turn 1 (upstream without native item/call IDs) persists item_pf_*/call_pf_*
//     to Redis; Turn 2 restores via MGET + Seed() and emits new message + tool call with strictly distinct public IDs.
func TestCanonicalT23AttemptIsolationAndHybridFallback(t *testing.T) {
	configureTwoTargetToolFallbackHost := func(t *testing.T, primaryUpstreamProtocol string) *managedHost {
		t.Helper()
		fixture := managedTextProtocolFixture(
			t,
			true,
			managedProtocolProfile{
				InputProtocol:    managedProtocolResponses,
				UpstreamProtocol: primaryUpstreamProtocol,
				supplier:         "deepseek",
			},
			"qwen",
		)
		directory := fixture[managedTextDirectoryField].(managedTextDirectory)
		require.Len(t, directory.Targets, 2)

		primaryTools := managedToolSettings{
			Allowed:   []fixedtools.Tool{fixedtools.WebSearch, fixedtools.CodeInterpreter},
			Supported: []fixedtools.Tool{}, // Primary supports NEITHER fixed tool!
			Defaults:  []fixedtools.Tool{fixedtools.WebSearch},
			Supplier:  "deepseek",
			PriceKey:  "modelfusion:tool-prices:asset-1",
		}
		fallbackTools := managedToolSettings{
			Allowed:   []fixedtools.Tool{fixedtools.WebSearch, fixedtools.CodeInterpreter},
			Supported: []fixedtools.Tool{fixedtools.WebSearch, fixedtools.CodeInterpreter}, // Fallback supports BOTH!
			Defaults:  []fixedtools.Tool{fixedtools.WebSearch},
			Supplier:  "qwen",
			PriceKey:  "modelfusion:tool-prices:asset-1",
		}

		updateTargetEntry := func(entry *managedTextDirectoryEntry, upProto, upPath, authority, providerType, upModel string, qwenNative bool, maxOutputTokens int, toolCfg managedToolSettings) {
			proxyRaw := []byte(entry.Proxy)
			var err error
			proxyRaw, err = sjson.SetBytes(proxyRaw, "managed_execution.input_protocol", managedProtocolResponses)
			require.NoError(t, err)
			proxyRaw, err = sjson.SetBytes(proxyRaw, "managed_execution.upstream_protocol", upProto)
			require.NoError(t, err)
			proxyRaw, err = sjson.SetBytes(proxyRaw, "managed_execution.upstream_path", upPath)
			require.NoError(t, err)
			proxyRaw, err = sjson.SetBytes(proxyRaw, "managed_execution.authority", authority)
			require.NoError(t, err)
			proxyRaw, err = sjson.SetBytes(proxyRaw, "managed_execution.max_output_tokens", maxOutputTokens)
			require.NoError(t, err)
			proxyRaw, err = sjson.SetBytes(proxyRaw, "managed_execution.tool_policy", toolCfg)
			require.NoError(t, err)
			proxyRaw, err = sjson.SetBytes(proxyRaw, "providers.0.type", providerType)
			require.NoError(t, err)
			if qwenNative {
				proxyRaw, err = sjson.SetBytes(proxyRaw, "providers.0.qwenEnableCompatible", false)
				require.NoError(t, err)
			}
			proxyDigest, err := mfjson.Digest(proxyRaw)
			require.NoError(t, err)

			execRaw := []byte(entry.Execution)
			adapterRef := "ai-proxy.native-text.v1:" + managedProtocolResponses + ":" + upProto
			execRaw, err = sjson.SetBytes(execRaw, "descriptor.upstream_model", upModel)
			require.NoError(t, err)
			execRaw, err = sjson.SetBytes(execRaw, "descriptor.adapter_ref", adapterRef)
			require.NoError(t, err)
			execRaw, err = sjson.SetBytes(execRaw, "descriptor.transport_contract.host", authority)
			require.NoError(t, err)
			execRaw, err = sjson.SetBytes(execRaw, "descriptor.transport_contract.sni", authority)
			require.NoError(t, err)
			execRaw, err = sjson.SetBytes(execRaw, "descriptor.transport_contract.path_adapter_ref", adapterRef)
			require.NoError(t, err)
			execRaw, err = sjson.SetBytes(execRaw, "descriptor.proxy_digest", proxyDigest)
			require.NoError(t, err)

			descDigest, err := managedDescriptorDigest([]byte(gjson.GetBytes(execRaw, "descriptor").Raw))
			require.NoError(t, err)
			execRaw, err = sjson.SetBytes(execRaw, "descriptor.descriptor_digest", descDigest)
			require.NoError(t, err)

			entry.Proxy = proxyRaw
			entry.Execution = execRaw
		}

		primaryPath := "/v1/responses"
		if primaryUpstreamProtocol == managedProtocolChat {
			primaryPath = "/v1/chat/completions"
		}
		updateTargetEntry(&directory.Targets[0], primaryUpstreamProtocol, primaryPath, "primary.deepseek.example", "deepseek", "upstream-model", false, 1024, primaryTools)
		updateTargetEntry(&directory.Targets[1], managedProtocolDashScopeText, "/api/v1/services/aigc/text-generation/generation", "dashscope.aliyuncs.com", "qwen", "second-upstream-model", true, 32768, fallbackTools)

		// Rebuild top-level plugin configuration from Targets[0] + _modelfusion_execution + _modelfusion_execution_targets.
		rootRaw := []byte(directory.Targets[0].Proxy)
		var err error
		rootRaw, err = sjson.SetRawBytes(rootRaw, "_modelfusion_execution", directory.Targets[0].Execution)
		require.NoError(t, err)
		dirBytes, err := json.Marshal(directory)
		require.NoError(t, err)
		rootRaw, err = sjson.SetRawBytes(rootRaw, managedTextDirectoryField, dirBytes)
		require.NoError(t, err)

		clear(fixture)
		require.NoError(t, json.Unmarshal(rootRaw, &fixture))
		return newManagedTextHost(t, fixture)
	}

	for _, mode := range []struct {
		name            string
		primaryUpstream string
	}{
		{name: "hybrid_same_wire_to_cross_wire_ir", primaryUpstream: managedProtocolResponses},
		{name: "cross_wire_ir_to_cross_wire_ir", primaryUpstream: managedProtocolChat},
	} {
		t.Run(mode.name, func(t *testing.T) {
			h := configureTwoTargetToolFallbackHost(t, mode.primaryUpstream)
			defer h.host.CompleteHttpContext(h.id)

			original := []byte(`{
				"model":"public-model",
				"input":"Compute and search",
				"max_output_tokens":393216,
				"tools":[
					{"type":"code_interpreter"},
					{"type":"function","name":"calc","parameters":{"type":"object","properties":{"expr":{"type":"string"}}}}
				]
			}`)
			h.start(t, original, bytes.Replace(original, []byte("public-model"), []byte("upstream-model"), 1))

			// 1. Reply to priceRedis GET modelfusion:tool-prices:asset-1.
			priceLookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
			require.Len(t, priceLookups, 1)
			var priceReply bytes.Buffer
			require.NoError(t, resp.NewWriter(&priceReply).WriteValue(resp.StringValue("web_search,code_interpreter")))
			h.host.CallOnRedisCallResponse(priceLookups[0].CalloutID, 0, priceReply.Bytes())

			// 2. Reply to primary target health lookup.
			respondManagedTextHealth(t, h, map[string]bool{"credential-1": true})

			statePrimary := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			require.NotNil(t, statePrimary.text)
			basePtrBefore := statePrimary.text.baseCanonical
			if mode.primaryUpstream == managedProtocolChat {
				require.NotNil(t, basePtrBefore, "cross-wire authorizeManagedTools must decode and cache baseCanonical once on state.text")
				require.Equal(t, "upstream-model", basePtrBefore.Model)
				require.Len(t, basePtrBefore.Tools, 2, "baseCanonical must retain original unfiltered tools")
			}

			// Verify Primary send stripped unsupported code_interpreter & default web_search.
			primarySends := h.host.GetCalloutAttributesFromContext(h.id)
			require.Len(t, primarySends, 1)
			primaryBody := primarySends[0].Body
			require.Equal(t, int64(1), gjson.GetBytes(primaryBody, "tools.#").Int(), string(primaryBody))
			require.False(t, gjson.GetBytes(primaryBody, "enable_search").Bool(), string(primaryBody))
			require.False(t, gjson.GetBytes(primaryBody, "enable_code_interpreter").Bool(), string(primaryBody))
			if mode.primaryUpstream == managedProtocolResponses {
				require.Equal(t, "calc", gjson.GetBytes(primaryBody, "tools.0.name").String())
				require.EqualValues(t, 393216, gjson.GetBytes(primaryBody, "max_output_tokens").Int())
			} else {
				require.Equal(t, "calc", gjson.GetBytes(primaryBody, "tools.0.function.name").String())
				require.EqualValues(t, 1024, gjson.GetBytes(primaryBody, "max_completion_tokens").Int())
			}
			require.True(t, statePrimary.removedTools[fixedtools.WebSearch])
			require.True(t, statePrimary.removedTools[fixedtools.CodeInterpreter])

			// 3. Primary fails with 503 -> triggers fallback to target-2 (dashscope_text).
			h.host.CallOnHttpCallResponse(
				primarySends[0].CalloutID,
				[][2]string{{":status", "503"}, {"content-type", "application/json"}},
				nil,
				[]byte(`{"error":{"code":"server_error","message":"primary unavailable"}}`),
			)
			respondManagedTextHealth(t, h, map[string]bool{"second-credential": true})
			respondManagedTextHealth(t, h, map[string]bool{"second-credential": true})

			// 4. Verify Fallback target-2 (dashscope_text) receives BOTH default web_search AND requested code_interpreter + calc!
			fallbackSends := h.host.GetCalloutAttributesFromContext(h.id)
			require.Len(t, fallbackSends, 1)
			fallbackBody := fallbackSends[0].Body
			require.Equal(t, "second-upstream-model", gjson.GetBytes(fallbackBody, "model").String(), string(fallbackBody))
			require.EqualValues(t, 32768, gjson.GetBytes(fallbackBody, "parameters.max_tokens").Int())
			require.True(t, gjson.GetBytes(fallbackBody, "parameters.enable_search").Bool(), "primary removedTools leaked into fallback: %s", string(fallbackBody))
			require.Equal(t, int64(2), gjson.GetBytes(fallbackBody, "parameters.tools.#").Int(), "fallback must retain code_interpreter + calc: %s", string(fallbackBody))
			require.Equal(t, "code_interpreter", gjson.GetBytes(fallbackBody, "parameters.tools.0.type").String(), string(fallbackBody))
			require.Equal(t, "calc", gjson.GetBytes(fallbackBody, "parameters.tools.1.function.name").String(), string(fallbackBody))

			// Verify shared state.text.baseCanonical pointer and contents were NEVER mutated across attempts!
			sharedBase := statePrimary.text.baseCanonical
			require.NotNil(t, sharedBase)
			if basePtrBefore != nil {
				require.Same(t, basePtrBefore, sharedBase)
				require.Equal(t, "upstream-model", sharedBase.Model)
			}
			require.Len(t, sharedBase.Tools, 2)
			require.Equal(t, 393216, sharedBase.Limits.Value.Val)
			require.Equal(t, "code_interpreter", sharedBase.Tools[0].FixedType)
			require.Equal(t, "calc", sharedBase.Tools[1].Name)

			// 5. Complete fallback DashScope response and verify final 200 OK Responses delivery.
			h.host.CallOnHttpCallResponse(
				fallbackSends[0].CalloutID,
				[][2]string{{":status", "200"}, {"content-type", "application/json"}},
				nil,
				[]byte(`{"request_id":"66666666-7777-4888-8999-aaaaaaaaaaaa","output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"Fallback succeeded."}}]},"usage":{"input_tokens":45,"output_tokens":7,"total_tokens":52}}`),
			)
			drainDashScopeRedis(h)

			sent := h.host.GetSentLocalResponse(h.id)
			require.NotNil(t, sent)
			require.EqualValues(t, 200, sent.StatusCode)
			require.Equal(t, "completed", gjson.GetBytes(sent.Data, "status").String(), string(sent.Data))
			require.Contains(t, string(sent.Data), "Fallback succeeded.")
		})
	}

	// Part 2: B02 Cross-Turn Salted Slot Alias Non-Collision (JSON & SSE).
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("b02_cross_turn_salted_alias_no_collision/stream=%t", stream), func(t *testing.T) {
			redisKV := make(map[string]string)

			newB02GeminiResponsesHost := func(t *testing.T) *managedHost {
				t.Helper()
				if stream {
					return newE2EMatrixRouterHost(
						t,
						`{"type":"gemini","apiTokens":["controlled-upstream-key"],"apiVersion":"v1beta"}`,
						"generativelanguage.googleapis.com",
						managedProtocolResponses,
						managedProtocolChat,
						"/v1beta/models/gemini-2.5-pro:streamGenerateContent",
						"gemini-2.5-pro",
						"",
					)
				}
				fixture := managedConfigFixture()
				fixture["provider"] = json.RawMessage(`{"type":"gemini","apiTokens":["controlled-upstream-key"],"apiVersion":"v1beta"}`)
				release := fixture["_modelfusion_execution"].(managedRelease)
				release.Descriptor.UpstreamModel = "gemini-2.5-pro"
				release.Descriptor.Transport.Host = "generativelanguage.googleapis.com"
				release.Descriptor.Transport.SNI = "generativelanguage.googleapis.com"
				descRaw, err := json.Marshal(release.Descriptor)
				require.NoError(t, err)
				release.Descriptor.DescriptorDigest, err = managedDescriptorDigest(descRaw)
				require.NoError(t, err)
				fixture["_modelfusion_execution"] = release

				settings := fixture["managed_execution"].(managedSettings)
				settings.Authority = "generativelanguage.googleapis.com"
				settings.InputProtocol = managedProtocolResponses
				settings.UpstreamProtocol = managedProtocolChat
				fixture["managed_execution"] = settings
				fixture["protocol_mode"] = "auto"
				return newManagedHost(t, fixture)
			}

			captureEvalRecordsIntoKV := func(t *testing.T, h *managedHost) int {
				t.Helper()
				saved := 0
				for _, call := range h.host.GetRedisCalloutAttributesFromContext(h.id) {
					cmd, _, err := resp.NewReader(bytes.NewReader(call.Query)).ReadValue()
					require.NoError(t, err)
					args := cmd.Array()
					if len(args) > 3 && args[0].String() == "EVAL" {
						numKeys := args[2].Integer()
						for k := 0; k < numKeys; k++ {
							key := args[3+k].String()
							val := args[4+numKeys+k].String()
							redisKV[key] = val
							saved++
						}
					}
					h.host.CallOnRedisCallResponse(call.CalloutID, 0, []byte(":1\r\n"))
				}
				return saved
			}

			answerMGETFromKV := func(t *testing.T, h *managedHost) int {
				t.Helper()
				lookups := h.host.GetRedisCalloutAttributesFromContext(h.id)
				require.Len(t, lookups, 1, "Turn 2 must issue MGET to restore Turn 1 public resource aliases")
				cmd, _, err := resp.NewReader(bytes.NewReader(lookups[0].Query)).ReadValue()
				require.NoError(t, err)
				args := cmd.Array()
				require.Equal(t, "MGET", args[0].String())
				vals := make([]resp.Value, 0, len(args)-1)
				for _, keyArg := range args[1:] {
					stored, exists := redisKV[keyArg.String()]
					require.True(t, exists, "missing persisted resource key in Redis: %s", keyArg.String())
					var rec resourceref.Record
					require.NoError(t, json.Unmarshal([]byte(stored), &rec))
					vals = append(vals, resp.StringValue(stored))
				}
				var reply bytes.Buffer
				require.NoError(t, resp.NewWriter(&reply).WriteArray(vals))
				h.host.CallOnRedisCallResponse(lookups[0].CalloutID, 0, reply.Bytes())
				return len(vals)
			}

			extractResponsesIDs := func(t *testing.T, raw []byte, isStream bool) (respID, msgItemID, callItemID, callID string) {
				t.Helper()
				doc := raw
				if isStream {
					evs, err := clientresponse.Events(raw)
					require.NoError(t, err)
					for _, ev := range evs {
						if ev.Name == "response.completed" {
							doc = []byte(gjson.GetBytes(ev.Data, "response").Raw)
						}
					}
				}
				respID = gjson.GetBytes(doc, "id").String()
				msgItemID = gjson.GetBytes(doc, "output.0.id").String()
				callItemID = gjson.GetBytes(doc, "output.1.id").String()
				callID = gjson.GetBytes(doc, "output.1.call_id").String()
				require.True(t, strings.HasPrefix(respID, "resp_pf_"), string(doc))
				require.True(t, strings.HasPrefix(msgItemID, "item_pf_"), string(doc))
				require.True(t, strings.HasPrefix(callItemID, "item_pf_"), string(doc))
				require.True(t, strings.HasPrefix(callID, "call_pf_"), string(doc))
				require.NotEqual(t, msgItemID, callItemID)
				return respID, msgItemID, callItemID, callID
			}

			startWithAdmittedCallID := func(t *testing.T, h *managedHost, callID string, original, mapped []byte) {
				t.Helper()
				headers := managedRequestHeaders(t, original)
				for i := range headers {
					if headers[i][0] == managedCallHeader {
						headers[i][1] = callID
					}
				}
				require.Equal(t, types.HeaderStopIteration, h.host.CallOnRequestHeaders(h.id, headers, false))
				require.Equal(t, types.ActionPause, h.host.CallOnRequestBody(h.id, mapped, true))
			}

			// --- Turn 1: Gemini returns message + functionCall with NO native responseId and NO functionCall.id ---
			var t1RespID, t1MsgItemID, t1CallItemID, t1CallID string
			t.Run("turn1", func(t *testing.T) {
				h1 := newB02GeminiResponsesHost(t)
				turn1Req := []byte(fmt.Sprintf(`{
					"model":"public-model",
					"stream":%t,
					"input":"Step 1 check",
					"tools":[{"type":"function","name":"step_tool","parameters":{"type":"object"}}]
				}`, stream))
				startWithAdmittedCallID(t, h1, "50000000-0000-0000-0000-000000000001", turn1Req, bytes.Replace(turn1Req, []byte("public-model"), []byte("gemini-2.5-pro"), 1))

				var turn1Out []byte
				if stream {
					var injected1 bytes.Buffer
					origInject1 := injectManagedRouterTerminal
					t.Cleanup(func() { injectManagedRouterTerminal = origInject1 })
					injectManagedRouterTerminal = func(body []byte, _ bool) error {
						injected1.Write(body)
						return nil
					}
					require.NoError(t, h1.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
					require.Equal(t, types.ActionContinue, h1.host.CallOnResponseHeaders(h1.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false))
					gemNoIDsSSE1 := "data: {\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Step 1 note\"},{\"functionCall\":{\"name\":\"step_tool\",\"args\":{\"step\":1}}}]}}]}\n\n" +
						"data: {\"candidates\":[{\"index\":0,\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":20,\"candidatesTokenCount\":8,\"totalTokenCount\":28}}\n\n"
					h1.host.CallOnResponseBody(h1.id, []byte(gemNoIDsSSE1), true)
					saved1 := captureEvalRecordsIntoKV(t, h1)
					require.GreaterOrEqual(t, saved1, 3, "Turn 1 must persist salted item_pf_* and call_pf_* records")
					turn1Out = append(h1.host.GetCurrentResponseBody(h1.id), injected1.Bytes()...)
				} else {
					gemNoIDsJSON1 := []byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Step 1 note"},{"functionCall":{"name":"step_tool","args":{"step":1}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":8,"totalTokenCount":28}}`)
					h1.rawResponse(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, gemNoIDsJSON1)
					saved1 := captureEvalRecordsIntoKV(t, h1)
					require.GreaterOrEqual(t, saved1, 3, "Turn 1 must persist salted item_pf_* and call_pf_* records")
					sent1 := h1.host.GetSentLocalResponse(h1.id)
					require.NotNil(t, sent1)
					turn1Out = sent1.Data
				}
				t1RespID, t1MsgItemID, t1CallItemID, t1CallID = extractResponsesIDs(t, turn1Out, stream)
				h1.host.CompleteHttpContext(h1.id)
			})

			// --- Turn 2: Client sends back Turn 1's item_pf_* / call_pf_* -> MGET + Seed() -> Gemini again omits native IDs ---
			t.Run("turn2", func(t *testing.T) {
				h2 := newB02GeminiResponsesHost(t)
				turn2Req := []byte(fmt.Sprintf(`{
					"model":"public-model",
					"stream":%t,
					"input":[
						{"role":"user","content":"Step 1 check"},
						{"type":"message","id":"%s","role":"assistant","content":[{"type":"output_text","text":"Step 1 note"}]},
						{"type":"function_call","id":"%s","call_id":"%s","name":"step_tool","arguments":"{\"step\":1}"},
						{"type":"function_call_output","call_id":"%s","output":"{\"ok\":true}"}
					],
					"tools":[{"type":"function","name":"step_tool","parameters":{"type":"object"}}]
				}`, stream, t1MsgItemID, t1CallItemID, t1CallID, t1CallID))
				startWithAdmittedCallID(t, h2, "50000000-0000-0000-0000-000000000002", turn2Req, bytes.Replace(turn2Req, []byte("public-model"), []byte("gemini-2.5-pro"), 1))

				restoredCount := answerMGETFromKV(t, h2)
				require.Equal(t, 4, restoredCount)

				var turn2Out []byte
				if stream {
					var injected2 bytes.Buffer
					origInject2 := injectManagedRouterTerminal
					t.Cleanup(func() { injectManagedRouterTerminal = origInject2 })
					injectManagedRouterTerminal = func(body []byte, _ bool) error {
						injected2.Write(body)
						return nil
					}
					require.NoError(t, h2.host.SetProperty([]string{"response", "code_details"}, []byte("via_upstream")))
					require.Equal(t, types.ActionContinue, h2.host.CallOnResponseHeaders(h2.id, [][2]string{{":status", "200"}, {"content-type", "text/event-stream"}}, false))
					gemNoIDsSSE2 := "data: {\"candidates\":[{\"index\":0,\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"Step 2 note\"},{\"functionCall\":{\"name\":\"step_tool\",\"args\":{\"step\":2}}}]}}]}\n\n" +
						"data: {\"candidates\":[{\"index\":0,\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":35,\"candidatesTokenCount\":9,\"totalTokenCount\":44}}\n\n"
					h2.host.CallOnResponseBody(h2.id, []byte(gemNoIDsSSE2), true)
					_ = captureEvalRecordsIntoKV(t, h2)
					turn2Out = append(h2.host.GetCurrentResponseBody(h2.id), injected2.Bytes()...)
				} else {
					gemNoIDsJSON2 := []byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Step 2 note"},{"functionCall":{"name":"step_tool","args":{"step":2}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":35,"candidatesTokenCount":9,"totalTokenCount":44}}`)
					h2.rawResponse(t, [][2]string{{":status", "200"}, {"content-type", "application/json"}}, gemNoIDsJSON2)
					_ = captureEvalRecordsIntoKV(t, h2)
					sent2 := h2.host.GetSentLocalResponse(h2.id)
					require.NotNil(t, sent2)
					turn2Out = sent2.Data
				}
				t2RespID, t2MsgItemID, t2CallItemID, t2CallID := extractResponsesIDs(t, turn2Out, stream)

				// B02 invariant: Turn 2 salted slot aliases MUST NOT collide with Turn 1 seeded public IDs!
				require.NotEqual(t, t1RespID, t2RespID)
				require.NotEqual(t, t1MsgItemID, t2MsgItemID, "Turn 2 message item_pf_* collided with Turn 1")
				require.NotEqual(t, t1CallItemID, t2CallItemID, "Turn 2 function_call item_pf_* collided with Turn 1")
				require.NotEqual(t, t1CallID, t2CallID, "Turn 2 call_pf_* collided with Turn 1")
				h2.host.CompleteHttpContext(h2.id)
			})
		})
	}
}
