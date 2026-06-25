package main

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/tidwall/gjson"
)

type requestCacheEligibility struct {
	Cacheable       bool
	SemanticAllowed bool
	Reason          string
}

func evaluateRequestCacheEligibility(body gjson.Result, key string) requestCacheEligibility {
	if requestHasToolOrFunctionMaterial(body) {
		return requestCacheEligibility{Cacheable: false, SemanticAllowed: false, Reason: "tool or function call request"}
	}
	if requestHasStochasticSampling(body) {
		return requestCacheEligibility{Cacheable: true, SemanticAllowed: false, Reason: "sampling settings require exact cache"}
	}
	if requestHasDynamicDataIntent(key) {
		return requestCacheEligibility{Cacheable: true, SemanticAllowed: false, Reason: "dynamic data intent requires exact cache"}
	}
	if requestHasWeakIntent(key) {
		return requestCacheEligibility{Cacheable: true, SemanticAllowed: false, Reason: "weak intent requires exact cache"}
	}
	if requestHasAmbiguousMultiTurnIntent(body, key) {
		return requestCacheEligibility{Cacheable: true, SemanticAllowed: false, Reason: "ambiguous multi-turn intent requires exact cache"}
	}
	return requestCacheEligibility{Cacheable: true, SemanticAllowed: true}
}

func requestHasToolOrFunctionMaterial(body gjson.Result) bool {
	if hasNonEmptyJSONValue(body.Get("tools")) ||
		hasNonEmptyJSONValue(body.Get("functions")) ||
		hasActiveChoice(body.Get("tool_choice")) ||
		hasActiveChoice(body.Get("function_call")) {
		return true
	}
	for _, message := range body.Get("messages").Array() {
		role := strings.ToLower(strings.TrimSpace(message.Get("role").String()))
		if role == "tool" || role == "function" {
			return true
		}
		if hasNonEmptyJSONValue(message.Get("tool_calls")) ||
			hasNonEmptyJSONValue(message.Get("function_call")) {
			return true
		}
	}
	return false
}

func hasNonEmptyJSONValue(value gjson.Result) bool {
	if !value.Exists() {
		return false
	}
	if value.IsArray() {
		return len(value.Array()) > 0
	}
	if value.IsObject() {
		return len(value.Map()) > 0
	}
	return strings.TrimSpace(value.String()) != ""
}

func hasActiveChoice(value gjson.Result) bool {
	if !value.Exists() {
		return false
	}
	if value.IsObject() {
		choiceType := strings.ToLower(strings.TrimSpace(value.Get("type").String()))
		name := strings.TrimSpace(value.Get("function.name").String())
		return (choiceType != "" && choiceType != "none") || name != ""
	}
	choice := normalizeIntentText(value.String())
	return choice != "" && choice != "none"
}

func requestHasStochasticSampling(body gjson.Result) bool {
	if numberGreaterThan(body, "temperature", 0) {
		return true
	}
	if topP := body.Get("top_p"); topP.Exists() {
		value := topP.Float()
		if value > 0 && value < 1 {
			return true
		}
	}
	if numberGreaterThan(body, "top_k", 0) {
		return true
	}
	if n := body.Get("n"); n.Exists() && n.Int() != 1 {
		return true
	}
	return numberNotZero(body, "presence_penalty") || numberNotZero(body, "frequency_penalty")
}

func numberGreaterThan(body gjson.Result, path string, threshold float64) bool {
	value := body.Get(path)
	return value.Exists() && value.Float() > threshold
}

func numberNotZero(body gjson.Result, path string) bool {
	value := body.Get(path)
	return value.Exists() && value.Float() != 0
}

func requestHasDynamicDataIntent(key string) bool {
	intent := normalizeIntentText(key)
	dynamicTerms := []string{
		"today", "now", "current", "latest", "weather", "news", "stock", "price", "exchange rate", "score",
		"今天", "现在", "当前", "最新", "天气", "新闻", "股价", "价格", "汇率", "比分",
	}
	return containsAny(intent, dynamicTerms)
}

func requestHasWeakIntent(key string) bool {
	intent := normalizeIntentText(key)
	weakIntents := map[string]struct{}{
		"hi": {}, "hello": {}, "thanks": {}, "thank you": {}, "ok": {}, "okay": {},
		"continue": {}, "go on": {}, "next": {}, "second one": {},
		"你好": {}, "谢谢": {}, "继续": {}, "下一个": {}, "第二个": {},
	}
	_, ok := weakIntents[intent]
	return ok
}

func requestHasAmbiguousMultiTurnIntent(body gjson.Result, key string) bool {
	if len(body.Get("messages").Array()) <= 1 {
		return false
	}
	intent := normalizeIntentText(key)
	ambiguousTerms := []string{
		"it", "that", "this", "those", "these", "previous", "above", "former", "latter", "second one",
		"它", "这个", "那个", "这些", "那些", "上一个", "前一个", "后一个", "第二个", "继续",
	}
	if containsAny(intent, ambiguousTerms) {
		return true
	}
	if runeCount(intent) <= 8 && len(strings.Fields(intent)) <= 3 {
		return true
	}
	return false
}

func normalizeIntentText(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), " \t\r\n?？!！.。,:，;；"))
}

func containsAny(value string, terms []string) bool {
	for _, term := range terms {
		if containsIntentTerm(value, term) {
			return true
		}
	}
	return false
}

func containsIntentTerm(value string, term string) bool {
	term = normalizeIntentText(term)
	if term == "" {
		return false
	}
	if isASCIITerm(term) {
		return containsTokenSequence(intentTokens(value), intentTokens(term))
	}
	return strings.Contains(value, term)
}

func isASCIITerm(value string) bool {
	for _, r := range value {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func intentTokens(value string) []string {
	tokens := make([]string, 0)
	var current strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func containsTokenSequence(tokens []string, termTokens []string) bool {
	if len(termTokens) == 0 || len(tokens) < len(termTokens) {
		return false
	}
	for i := 0; i <= len(tokens)-len(termTokens); i++ {
		matched := true
		for j := range termTokens {
			if tokens[i+j] != termTokens[j] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func runeCount(value string) int {
	return len([]rune(value))
}

func isSuccessfulHTTPStatus(status string) bool {
	if strings.TrimSpace(status) == "" {
		return false
	}
	code, err := strconv.Atoi(strings.TrimSpace(status))
	return err == nil && code >= 200 && code < 300
}

func responseHasToolCall(body gjson.Result) bool {
	for _, choice := range body.Get("choices").Array() {
		if hasNonEmptyJSONValue(choice.Get("message.tool_calls")) ||
			hasNonEmptyJSONValue(choice.Get("message.function_call")) ||
			hasNonEmptyJSONValue(choice.Get("delta.tool_calls")) ||
			hasNonEmptyJSONValue(choice.Get("delta.function_call")) {
			return true
		}
	}
	return false
}
