package main

import (
	"bytes"
	"strconv"

	"github.com/higress-group/wasm-go/pkg/tokenusage"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const statisticsObservedUsageKey = "ai-statistics-observed-usage"

type statisticsObservedUsage struct {
	input, output, total *int64
	pending              []byte
	overflow             bool
}

// The shared parser intentionally derives/defaults counters for legacy callers.
// Bound statistics keep only counters actually present in a valid response frame.
func observeStatisticsUsage(ctx wrapper.HttpContext, config AIStatisticsConfig, body []byte, streaming bool) tokenusage.TokenUsage {
	if config.pathScope != pathScopeBoundTarget {
		return tokenusage.GetTokenUsage(ctx, body)
	}
	observed, _ := ctx.GetContext(statisticsObservedUsageKey).(*statisticsObservedUsage)
	if observed == nil {
		observed = &statisticsObservedUsage{}
		ctx.SetContext(statisticsObservedUsageKey, observed)
	}
	frames := [][]byte{body}
	if streaming {
		if observed.overflow {
			return tokenusage.TokenUsage{}
		}
		if len(observed.pending)+len(body) > int(defaultMaxBodyBytes) {
			observed.pending = nil
			observed.overflow = true
			return tokenusage.TokenUsage{}
		}
		observed.pending = wrapper.UnifySSEChunk(append(observed.pending, body...))
		frames = bytes.Split(observed.pending, []byte("\n\n"))
		observed.pending = append([]byte(nil), frames[len(frames)-1]...)
		frames = frames[:len(frames)-1]
	}
	var usage tokenusage.TokenUsage
	for _, frame := range frames {
		payload := bytes.TrimSpace(frame)
		if !gjson.ValidBytes(payload) {
			var data []byte
			for _, line := range bytes.Split(frame, []byte("\n")) {
				if bytes.HasPrefix(line, []byte("data:")) {
					if len(data) > 0 {
						data = append(data, '\n')
					}
					data = append(data, bytes.TrimPrefix(bytes.TrimPrefix(line, []byte("data:")), []byte(" "))...)
				}
			}
			payload = data
		}
		if !gjson.ValidBytes(payload) {
			continue
		}
		usage = tokenusage.GetTokenUsage(ctx, payload)
		for _, field := range []struct {
			target **int64
			paths  []string
		}{
			{&observed.input, []string{tokenusage.UsageInputTokensPathOpenAIChatCompletions, tokenusage.UsageInputTokensPathOpenAIImages, tokenusage.UsageInputTokensPathOpenAIResponses, tokenusage.UsageInputTokensPathGemini, tokenusage.UsageInputTokensPathAnthropicMessages}},
			{&observed.output, []string{tokenusage.UsageOutputTokensPathOpenAIChatCompletions, tokenusage.UsageOutputTokensPathOpenAIImages, tokenusage.UsageOutputTokensPathOpenAIResponses, tokenusage.UsageOutputTokensPathGemini, tokenusage.UsageOutputTokensPathAnthropicMessages}},
			{&observed.total, []string{tokenusage.UsageTotalTokensPathOpenAIChatCompletions, tokenusage.UsageTotalTokensPathOpenAIResponses, tokenusage.UsageTotalTokensPathGemini}},
		} {
			if result := wrapper.GetValueFromBody(payload, field.paths); result != nil {
				value, ok := observedStatisticsCounter(*result)
				if ok {
					*field.target = &value
				} else {
					*field.target = nil
				}
			}
		}
		for _, detail := range []struct {
			values     map[string]int64
			paths      []string
			contextKey string
		}{
			{usage.InputTokenDetails, []string{tokenusage.UsageInputTokensDetailsPathOpenAIChatCompletions, tokenusage.UsageInputTokensDetailsPathOpenAIResponses, "usage.input_tokens_details"}, tokenusage.CtxKeyInputTokenDetails},
			{usage.OutputTokenDetails, []string{tokenusage.UsageOutputTokensDetailsPathOpenAIChatCompletions, tokenusage.UsageOutputTokensDetailsPathOpenAIResponses, "usage.output_tokens_details"}, tokenusage.CtxKeyOutputTokenDetails},
		} {
			for key := range detail.values {
				var known bool
				var scalarPath string
				switch key {
				case tokenusage.InputTokenDetailsKeyCachedTokens:
					scalarPath = tokenusage.UsageCachedTokensPathKimi
				case tokenusage.InputTokenDetailsKeyAnthropicMessagesUsageCacheCreationInputTokens:
					scalarPath = tokenusage.UsageCacheCreationInputTokensPathAnthropicMessages
				case tokenusage.InputTokenDetailsKeyAnthropicMessagesUsageCacheReadInputTokens:
					scalarPath = tokenusage.UsageCacheReadInputTokensPathAnthropicMessages
				case tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheHitTokens:
					scalarPath = tokenusage.UsagePromptCacheHitTokensPathDeepSeek
				case tokenusage.InputTokenDetailsKeyDeepSeekPromptCacheMissTokens:
					scalarPath = tokenusage.UsagePromptCacheMissTokensPathDeepSeek
				case tokenusage.InputTokenDetailsKeyGeminiCachedContentTokenCount:
					scalarPath = tokenusage.UsageMetadataCachedContentTokenCountPathGemini
				case tokenusage.InputTokenDetailsKeyGeminiToolUsePromptTokenCount:
					scalarPath = tokenusage.UsageMetadataToolUsePromptTokenCountPathGemini
				case tokenusage.OutputTokenDetailsKeyGeminiThoughtsTokenCount:
					scalarPath = tokenusage.UsageMetadataThoughtsTokenCountPathGemini
				case tokenusage.OutputTokenDetailsKeyDoubaoGeneratedImages:
					scalarPath = tokenusage.UsageGeneratedImagesPathDoubao
				}
				if scalarPath != "" {
					if value := gjson.GetBytes(payload, scalarPath); value.Exists() {
						_, known = observedStatisticsCounter(value)
					}
				}
				for _, path := range detail.paths {
					if known {
						break
					}
					object := gjson.GetBytes(payload, path)
					if value, exists := object.Map()[key]; exists {
						_, known = observedStatisticsCounter(value)
						break
					}
				}
				if !known {
					delete(detail.values, key)
				}
			}
			if len(detail.values) > 0 {
				ctx.SetContext(detail.contextKey, detail.values)
			}
		}
	}
	for _, field := range []struct {
		key, span string
		value     *int64
	}{
		{tokenusage.CtxKeyInputToken, ArmsInputToken, observed.input},
		{tokenusage.CtxKeyOutputToken, ArmsOutputToken, observed.output},
		{tokenusage.CtxKeyTotalToken, ArmsTotalToken, observed.total},
	} {
		if field.value == nil {
			if ctx.GetUserAttribute(field.key) != nil {
				ctx.SetUserAttribute(field.key, nil)
			}
		} else {
			ctx.SetUserAttribute(field.key, *field.value)
			setSpanAttribute(field.span, *field.value)
		}
	}
	if observed.input == nil || observed.output == nil || observed.total == nil {
		// The existing span block expects a complete usage tuple.
		usage.TotalToken = 0
	}
	return usage
}

func observedStatisticsCounter(result gjson.Result) (int64, bool) {
	if result.Type != gjson.Number || result.Raw == "" {
		return 0, false
	}
	for _, c := range result.Raw {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	value, err := strconv.ParseInt(result.Raw, 10, 64)
	return value, err == nil
}
