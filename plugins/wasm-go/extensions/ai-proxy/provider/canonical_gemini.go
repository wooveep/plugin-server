package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/higress-group/wasm-go/pkg/ai/canonical"
	"github.com/higress-group/wasm-go/pkg/ai/fixedtools"
)

type geminiWireInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiWireFunctionCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type geminiWireFunctionResponse struct {
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type geminiWirePart struct {
	Text             *string                     `json:"text,omitempty"`
	Thought          *bool                       `json:"thought,omitempty"`
	ThoughtSignature string                      `json:"thoughtSignature,omitempty"`
	InlineData       *geminiWireInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *geminiWireFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiWireFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiWireContent struct {
	Role  string           `json:"role,omitempty"`
	Parts []geminiWirePart `json:"parts"`
}

type geminiWireSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type geminiWireThinkingConfig struct {
	IncludeThoughts *bool  `json:"includeThoughts,omitempty"`
	ThinkingBudget  *int64 `json:"thinkingBudget,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
}

type geminiWireGenerationConfig struct {
	Temperature      *float64                  `json:"temperature,omitempty"`
	TopP             *float64                  `json:"topP,omitempty"`
	TopK             *int                      `json:"topK,omitempty"`
	PresencePenalty  *float64                  `json:"presencePenalty,omitempty"`
	FrequencyPenalty *float64                  `json:"frequencyPenalty,omitempty"`
	Seed             *int64                    `json:"seed,omitempty"`
	CandidateCount   *int                      `json:"candidateCount,omitempty"`
	MaxOutputTokens  *int                      `json:"maxOutputTokens,omitempty"`
	StopSequences    []string                  `json:"stopSequences,omitempty"`
	ResponseLogprobs *bool                     `json:"responseLogprobs,omitempty"`
	Logprobs         *int                      `json:"logprobs,omitempty"`
	ResponseMimeType string                    `json:"responseMimeType,omitempty"`
	ResponseSchema   json.RawMessage           `json:"responseSchema,omitempty"`
	ThinkingConfig   *geminiWireThinkingConfig `json:"thinkingConfig,omitempty"`
}

func (g geminiWireGenerationConfig) isEmpty() bool {
	return g.Temperature == nil &&
		g.TopP == nil &&
		g.TopK == nil &&
		g.PresencePenalty == nil &&
		g.FrequencyPenalty == nil &&
		g.Seed == nil &&
		g.CandidateCount == nil &&
		g.MaxOutputTokens == nil &&
		len(g.StopSequences) == 0 &&
		g.ResponseLogprobs == nil &&
		g.Logprobs == nil &&
		g.ResponseMimeType == "" &&
		len(g.ResponseSchema) == 0 &&
		g.ThinkingConfig == nil
}

type geminiWireFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type geminiWireEmptyStruct struct{}

type geminiWireTool struct {
	FunctionDeclarations []geminiWireFunctionDeclaration `json:"functionDeclarations,omitempty"`
	GoogleSearch         *geminiWireEmptyStruct          `json:"googleSearch,omitempty"`
	CodeExecution        *geminiWireEmptyStruct          `json:"codeExecution,omitempty"`
}

type geminiWireFunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type geminiWireToolConfig struct {
	FunctionCallingConfig *geminiWireFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type geminiWireOutRequest struct {
	SystemInstruction *geminiWireContent          `json:"systemInstruction,omitempty"`
	Contents          []geminiWireContent         `json:"contents"`
	Tools             []geminiWireTool            `json:"tools,omitempty"`
	ToolConfig        *geminiWireToolConfig       `json:"toolConfig,omitempty"`
	SafetySettings    []geminiWireSafetySetting   `json:"safetySettings,omitempty"`
	GenerationConfig  *geminiWireGenerationConfig `json:"generationConfig,omitempty"`
}

func compactJSONRaw(raw []byte) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty JSON")
	}
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("invalid JSON")
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err != nil {
		return nil, err
	}
	return json.RawMessage(bytes.Clone(buf.Bytes())), nil
}

func compactGeminiJSONObject(raw []byte, errCode, errField, errMsg string) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return json.RawMessage("{}"), nil
	}
	if trimmed[0] != '{' || !json.Valid(trimmed) {
		return nil, canonical.NewRejectionError(errCode, errField, errMsg)
	}
	compacted, err := compactJSONRaw(trimmed)
	if err != nil {
		return nil, canonical.NewRejectionError(errCode, errField, errMsg)
	}
	return compacted, nil
}

func encodeGeminiImagePart(img *canonical.ImagePart, target CanonicalTarget, errField string) (*geminiWireInlineData, error) {
	if img == nil {
		return nil, canonical.NewRejectionError("invalid_image_content", errField, "image part is missing image metadata")
	}
	if img.Source == canonical.ImageFileID || img.FileID != "" {
		return nil, canonical.NewRejectionError("unsupported_image_content", "file_id", "image file_id cannot be resolved across protocols")
	}
	lowerURL := strings.ToLower(img.URL)
	if img.Source == canonical.ImageURL || strings.HasPrefix(lowerURL, "http://") || strings.HasPrefix(lowerURL, "https://") {
		return nil, canonical.NewRejectionError("unsupported_image_url", "image_url", "remote image URLs are unsupported on managed Gemini targets")
	}
	if !img.Detail.Valid() {
		return nil, canonical.NewRejectionError("invalid_image_detail", "detail", "invalid image detail")
	}
	if img.Detail != canonical.ImageDetailUnset {
		if target.AllowedImageDetails != nil {
			if !slices.Contains(target.AllowedImageDetails, img.Detail) {
				return nil, canonical.NewRejectionError("unsupported_image_detail", "detail", "image detail is unsupported on target")
			}
		} else if img.Detail == canonical.ImageDetailOriginal {
			return nil, canonical.NewRejectionError("unsupported_image_detail", "detail", "image detail original requires explicit target capability")
		}
	}

	if img.Source == canonical.ImageDataURL && img.MediaType != "" && img.Base64Ref != "" {
		return &geminiWireInlineData{
			MimeType: img.MediaType,
			Data:     img.Base64Ref,
		}, nil
	}
	if strings.HasPrefix(img.URL, "data:") {
		parsed, err := parseCanonicalImageURL(img.URL, img.Detail, errField)
		if err != nil {
			return nil, err
		}
		return &geminiWireInlineData{
			MimeType: parsed.MediaType,
			Data:     parsed.Base64Ref,
		}, nil
	}
	return nil, canonical.NewRejectionError("unsupported_image_url", errField, "Gemini inlineData requires a base64 data URL")
}

func mapGeminiThinkingLevel(effort string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "minimal":
		return "MINIMAL", true
	case "low":
		return "LOW", true
	case "medium":
		return "MEDIUM", true
	case "high", "xhigh", "max":
		return "HIGH", true
	default:
		return "", false
	}
}

func buildGeminiThinkingConfig(req canonical.Request, target CanonicalTarget, model string) (*geminiWireThinkingConfig, error) {
	isDefaultThinkingModel := geminiThinkingModels[model] || strings.HasPrefix(model, "gemini-3") || target.GeminiThinkingBudget != 0
	isGemini3 := strings.HasPrefix(model, "gemini-3")

	switch req.Thinking.Mode {
	case canonical.ThinkingDisabled:
		if !isDefaultThinkingModel && !req.Thinking.BudgetTokens.IsPresent() && !req.Thinking.Effort.IsPresent() && req.SourceProtocol == canonical.ProtocolOpenAIChat && !target.SupportsThinking {
			return nil, nil
		}
		inc := false
		cfg := &geminiWireThinkingConfig{
			IncludeThoughts: &inc,
		}
		if isGemini3 {
			cfg.ThinkingLevel = "MINIMAL"
		} else {
			var b int64
			if req.Thinking.BudgetTokens.IsPresent() {
				b = int64(req.Thinking.BudgetTokens.Val)
			}
			cfg.ThinkingBudget = &b
		}
		return cfg, nil

	case canonical.ThinkingEnabled, canonical.ThinkingAdaptive:
		inc := true
		cfg := &geminiWireThinkingConfig{
			IncludeThoughts: &inc,
		}
		if req.Thinking.BudgetTokens.IsPresent() {
			if req.Thinking.BudgetTokens.Val < 0 && req.Thinking.BudgetTokens.Val != -1 {
				return nil, canonical.NewRejectionError("invalid_thinking_config", "thinking_budget", "thinking_budget must be non-negative or -1")
			}
			b := int64(req.Thinking.BudgetTokens.Val)
			cfg.ThinkingBudget = &b
		} else if req.Thinking.Effort.IsPresent() && req.Thinking.Effort.Val != "" && req.Thinking.Effort.Val != "none" {
			lvl, ok := mapGeminiThinkingLevel(req.Thinking.Effort.Val)
			if !ok {
				return nil, canonical.NewRejectionError("invalid_thinking_config", "reasoning_effort", "unsupported reasoning effort level")
			}
			if isGemini3 || (target.GeminiThinkingBudget == 0 && !geminiThinkingModels[model]) {
				cfg.ThinkingLevel = lvl
			} else if target.GeminiThinkingBudget != 0 {
				b := target.GeminiThinkingBudget
				cfg.ThinkingBudget = &b
			}
		} else if target.GeminiThinkingBudget != 0 {
			b := target.GeminiThinkingBudget
			cfg.ThinkingBudget = &b
		}
		return cfg, nil

	case canonical.ThinkingUnset:
		if req.Thinking.Effort.IsPresent() && req.Thinking.Effort.Val != "" {
			if req.Thinking.Effort.Val == "none" {
				inc := false
				b := int64(0)
				if isGemini3 {
					return &geminiWireThinkingConfig{IncludeThoughts: &inc, ThinkingLevel: "MINIMAL"}, nil
				}
				return &geminiWireThinkingConfig{IncludeThoughts: &inc, ThinkingBudget: &b}, nil
			}
			lvl, ok := mapGeminiThinkingLevel(req.Thinking.Effort.Val)
			if !ok {
				return nil, canonical.NewRejectionError("invalid_thinking_config", "reasoning_effort", "unsupported reasoning effort level")
			}
			inc := true
			if isGemini3 || (target.GeminiThinkingBudget == 0 && !geminiThinkingModels[model]) {
				return &geminiWireThinkingConfig{IncludeThoughts: &inc, ThinkingLevel: lvl}, nil
			}
			cfg := &geminiWireThinkingConfig{IncludeThoughts: &inc}
			if target.GeminiThinkingBudget != 0 {
				b := target.GeminiThinkingBudget
				cfg.ThinkingBudget = &b
			}
			return cfg, nil
		}
		if req.Thinking.BudgetTokens.IsPresent() {
			b := int64(req.Thinking.BudgetTokens.Val)
			inc := b != 0
			return &geminiWireThinkingConfig{
				IncludeThoughts: &inc,
				ThinkingBudget:  &b,
			}, nil
		}
		if isDefaultThinkingModel {
			inc := true
			cfg := &geminiWireThinkingConfig{
				IncludeThoughts: &inc,
			}
			if target.GeminiThinkingBudget != 0 {
				b := target.GeminiThinkingBudget
				cfg.ThinkingBudget = &b
			}
			return cfg, nil
		}
		return nil, nil

	default:
		return nil, canonical.NewRejectionError("invalid_thinking_config", "thinking", "unsupported thinking mode")
	}
}

func buildGeminiFunctionResponseObject(joinedText string, isError bool) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace([]byte(joinedText))
	if len(trimmed) > 0 && trimmed[0] == '{' && json.Valid(trimmed) {
		compacted, err := compactJSONRaw(trimmed)
		if err == nil {
			if !isError {
				return compacted, nil
			}
			return json.Marshal(struct {
				Error   json.RawMessage `json:"error"`
				IsError bool            `json:"is_error"`
			}{
				Error:   compacted,
				IsError: true,
			})
		}
	}
	if isError {
		return json.Marshal(struct {
			Error   string `json:"error"`
			IsError bool   `json:"is_error"`
		}{
			Error:   joinedText,
			IsError: true,
		})
	}
	return json.Marshal(struct {
		Output string `json:"output"`
	}{
		Output: joinedText,
	})
}

func appendGeminiContent(contents []geminiWireContent, role string, parts []geminiWirePart) []geminiWireContent {
	if len(parts) == 0 {
		return contents
	}
	if len(contents) > 0 && contents[len(contents)-1].Role == role {
		contents[len(contents)-1].Parts = append(contents[len(contents)-1].Parts, parts...)
		return contents
	}
	return append(contents, geminiWireContent{
		Role:  role,
		Parts: parts,
	})
}

func encodeGeminiCanonicalRequest(req canonical.Request, target CanonicalTarget) ([]byte, error) {
	if target.WireFormat != WireGeminiGenerateContent {
		return nil, canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "expected gemini_generate_content")
	}

	if req.PreviousResponseID.IsPresent() && req.PreviousResponseID.Val != "" {
		return nil, canonical.NewRejectionError("unsupported_stateful_reference", "previous_response_id", "previous_response_id is unsupported on Gemini")
	}
	if req.ConversationPresent {
		return nil, canonical.NewRejectionError("unsupported_stateful_reference", "conversation", "conversation is unsupported on Gemini")
	}
	if req.Background {
		return nil, canonical.NewRejectionError("unsupported_background_mode", "background", "background execution is unsupported on Gemini")
	}

	model := target.Model
	if model == "" {
		model = req.Model
	}

	var genCfg geminiWireGenerationConfig

	// R02 / DEF-05: Map all output limit sources (max_tokens, max_completion_tokens, max_output_tokens).
	if req.Limits.Value.IsPresent() {
		limitField := "max_tokens"
		switch req.Limits.Source {
		case canonical.LimitMaxCompletionTokens:
			limitField = "max_completion_tokens"
		case canonical.LimitMaxOutputTokens:
			limitField = "max_output_tokens"
		}
		v := req.Limits.Value.Val
		if v < 0 {
			return nil, canonical.NewRejectionError("invalid_output_limit", limitField, "output token limit must be non-negative")
		}
		if v == 0 {
			return nil, canonical.NewRejectionError("unsupported_zero_output_tokens", limitField, "zero output token warmup is unsupported on Gemini")
		}
		if target.AcceptedLimitScopes != LimitScopeMaskNone && !target.AcceptedLimitScopes.Allows(req.Limits.Scope) {
			return nil, canonical.NewRejectionError("unsupported_output_limit_scope", limitField, "output token limit scope is unsupported on target")
		}
		genCfg.MaxOutputTokens = &v
	}

	// DEF-01: Pointer float64/int64 scalars preserving explicit 0 and fractional penalties.
	if req.Sampling.Temperature.IsPresent() {
		v := req.Sampling.Temperature.Val
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "temperature", "temperature must be non-negative")
		}
		genCfg.Temperature = &v
	}
	if req.Sampling.TopP.IsPresent() {
		v := req.Sampling.TopP.Val
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "top_p", "top_p must be within [0, 1]")
		}
		genCfg.TopP = &v
	}
	if req.Sampling.TopK.IsPresent() {
		v := req.Sampling.TopK.Val
		if v < 0 {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "top_k", "top_k must be non-negative")
		}
		genCfg.TopK = &v
	}
	if req.Sampling.PresencePenalty.IsPresent() {
		v := req.Sampling.PresencePenalty.Val
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "presence_penalty", "invalid presence_penalty")
		}
		genCfg.PresencePenalty = &v
	}
	if req.Sampling.FrequencyPenalty.IsPresent() {
		v := req.Sampling.FrequencyPenalty.Val
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "frequency_penalty", "invalid frequency_penalty")
		}
		genCfg.FrequencyPenalty = &v
	}
	if req.Sampling.Seed.IsPresent() {
		v := req.Sampling.Seed.Val
		genCfg.Seed = &v
	}
	if req.Sampling.N.IsPresent() {
		v := req.Sampling.N.Val
		if v <= 0 {
			return nil, canonical.NewRejectionError("invalid_sampling_parameter", "n", "n must be at least 1")
		}
		genCfg.CandidateCount = &v
	}
	if req.Sampling.StopPresent {
		if len(req.Sampling.Stop) > 5 {
			return nil, canonical.NewRejectionError("invalid_stop", "stop", "stopSequences exceeds Gemini maximum of 5")
		}
		if len(req.Sampling.Stop) > 0 {
			genCfg.StopSequences = slices.Clone(req.Sampling.Stop)
		}
	}
	if req.Sampling.Logprobs.IsPresent() {
		v := req.Sampling.Logprobs.Val
		genCfg.ResponseLogprobs = &v
	}
	if req.Sampling.TopLogprobs.IsPresent() && req.SourceProtocol == canonical.ProtocolOpenAIChat {
		v := req.Sampling.TopLogprobs.Val
		genCfg.Logprobs = &v
	}

	// DS05: Structured output via responseMimeType / responseSchema (preserving raw schema integers).
	switch req.ResponseFormat.Kind {
	case canonical.ResponseFormatUnset:
	case canonical.ResponseFormatText:
		genCfg.ResponseMimeType = "text/plain"
	case canonical.ResponseFormatJSONObject:
		genCfg.ResponseMimeType = "application/json"
	case canonical.ResponseFormatJSONSchema:
		trimmedSchema := bytes.TrimSpace(req.ResponseFormat.Schema)
		if len(trimmedSchema) == 0 || bytes.Equal(trimmedSchema, []byte("null")) {
			return nil, canonical.NewRejectionError("invalid_response_format", "response_format", "json_schema requires a non-empty schema")
		}
		if !target.SupportsStrictSchema {
			return nil, canonical.NewRejectionError("unsupported_strict_schema", "response_format", "json_schema structured output is unsupported on target")
		}
		compactedSchema, err := compactJSONRaw(trimmedSchema)
		if err != nil {
			return nil, canonical.NewRejectionError("invalid_response_format", "response_format", "invalid json_schema")
		}
		genCfg.ResponseMimeType = "application/json"
		genCfg.ResponseSchema = compactedSchema
	default:
		return nil, canonical.NewRejectionError("unsupported_response_format", "response_format", "unsupported response_format")
	}

	thinkingCfg, err := buildGeminiThinkingConfig(req, target, model)
	if err != nil {
		return nil, err
	}
	genCfg.ThinkingConfig = thinkingCfg

	var out geminiWireOutRequest

	// Tools & toolConfig.functionCallingConfig (R04 / GE01).
	var funcDecls []geminiWireFunctionDeclaration
	enableGoogleSearch := req.FixedTools.EnableSearch.IsPresent() && req.FixedTools.EnableSearch.Val
	enableCodeExecution := req.FixedTools.EnableCodeInterpreter.IsPresent() && req.FixedTools.EnableCodeInterpreter.Val

	for _, t := range req.Tools {
		switch t.Kind {
		case canonical.ToolKindFunction:
			if t.Name == "" {
				return nil, canonical.NewRejectionError("invalid_tool_definition", "tools", "function declaration requires name")
			}
			if t.Strict.IsPresent() && t.Strict.Val && !target.SupportsStrictSchema {
				return nil, canonical.NewRejectionError("unsupported_strict_schema", "tools", "strict tool schema is unsupported on target")
			}
			var paramsRaw json.RawMessage
			if trimmedParams := bytes.TrimSpace(t.Schema); len(trimmedParams) > 0 && !bytes.Equal(trimmedParams, []byte("null")) {
				compacted, err := compactJSONRaw(trimmedParams)
				if err != nil {
					return nil, canonical.NewRejectionError("invalid_tool_definition", "tools", "invalid function parameters schema")
				}
				paramsRaw = compacted
			}
			funcDecls = append(funcDecls, geminiWireFunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  paramsRaw,
			})
		case canonical.ToolKindFixedServer:
			fk, ok := fixedtools.Kind(t.FixedType)
			if !ok {
				field := canonical.SafeIdentifier(t.FixedType)
				if field == "" {
					field = "tools"
				}
				return nil, canonical.NewRejectionError("unsupported_tool_type", field, "unsupported fixed tool on Gemini")
			}
			switch fk {
			case fixedtools.WebSearch:
				if !req.FixedTools.EnableSearch.IsPresent() || req.FixedTools.EnableSearch.Val {
					enableGoogleSearch = true
				}
			case fixedtools.CodeInterpreter:
				if !req.FixedTools.EnableCodeInterpreter.IsPresent() || req.FixedTools.EnableCodeInterpreter.Val {
					enableCodeExecution = true
				}
			default:
				return nil, canonical.NewRejectionError("unsupported_tool_type", string(fk), "fixed server tool is unsupported on Gemini")
			}
		}
	}

	if len(funcDecls) > 0 {
		out.Tools = append(out.Tools, geminiWireTool{FunctionDeclarations: funcDecls})
	}
	if enableGoogleSearch {
		out.Tools = append(out.Tools, geminiWireTool{GoogleSearch: &geminiWireEmptyStruct{}})
	}
	if enableCodeExecution {
		out.Tools = append(out.Tools, geminiWireTool{CodeExecution: &geminiWireEmptyStruct{}})
	}

	forcedToolChoice := req.ToolChoice.Mode == canonical.ToolChoiceRequired ||
		req.ToolChoice.Mode == canonical.ToolChoiceAny ||
		req.ToolChoice.Mode == canonical.ToolChoiceFunction ||
		req.ToolChoice.Mode == canonical.ToolChoiceFixed
	if forcedToolChoice {
		if req.ToolChoice.Mode == canonical.ToolChoiceFixed {
			return nil, canonical.NewRejectionError("unsupported_tool_choice", "tool_choice", "fixed server tool_choice is unsupported on Gemini")
		}
		if len(funcDecls) == 0 {
			return nil, canonical.NewRejectionError("invalid_tool_choice", "tool_choice", "forced tool_choice requires declared function tools")
		}
		thinkingActive := req.Thinking.Mode == canonical.ThinkingEnabled ||
			req.Thinking.Mode == canonical.ThinkingAdaptive ||
			(req.Thinking.Mode == canonical.ThinkingUnset && req.Thinking.Effort.IsPresent() && req.Thinking.Effort.Val != "" && req.Thinking.Effort.Val != "none")
		if thinkingActive && (target.ForbidsForcedToolThink || (req.SourceProtocol == canonical.ProtocolAnthropicMessages && req.Thinking.Mode == canonical.ThinkingEnabled)) {
			return nil, canonical.NewRejectionError("conflicting_thinking_tool_choice", "tool_choice", "forced tool_choice cannot be combined with active thinking")
		}
	}

	switch req.ToolChoice.Mode {
	case canonical.ToolChoiceUnset:
	case canonical.ToolChoiceAuto:
		out.ToolConfig = &geminiWireToolConfig{
			FunctionCallingConfig: &geminiWireFunctionCallingConfig{Mode: "AUTO"},
		}
	case canonical.ToolChoiceNone:
		out.ToolConfig = &geminiWireToolConfig{
			FunctionCallingConfig: &geminiWireFunctionCallingConfig{Mode: "NONE"},
		}
	case canonical.ToolChoiceRequired, canonical.ToolChoiceAny:
		out.ToolConfig = &geminiWireToolConfig{
			FunctionCallingConfig: &geminiWireFunctionCallingConfig{Mode: "ANY"},
		}
	case canonical.ToolChoiceFunction:
		if req.ToolChoice.Name == "" {
			return nil, canonical.NewRejectionError("invalid_tool_choice", "tool_choice", "function tool_choice requires name")
		}
		out.ToolConfig = &geminiWireToolConfig{
			FunctionCallingConfig: &geminiWireFunctionCallingConfig{
				Mode:                 "ANY",
				AllowedFunctionNames: []string{req.ToolChoice.Name},
			},
		}
	}

	// Deterministic safetySettings from target.GeminiSafetySettings.
	if len(target.GeminiSafetySettings) > 0 {
		categories := make([]string, 0, len(target.GeminiSafetySettings))
		for cat := range target.GeminiSafetySettings {
			categories = append(categories, cat)
		}
		slices.Sort(categories)
		out.SafetySettings = make([]geminiWireSafetySetting, 0, len(categories))
		for _, cat := range categories {
			out.SafetySettings = append(out.SafetySettings, geminiWireSafetySetting{
				Category:  cat,
				Threshold: target.GeminiSafetySettings[cat],
			})
		}
	}

	// R06 #1: Concatenate Instructions, SystemParts, and all system/developer messages into systemInstruction.parts.
	var sysParts []geminiWirePart
	if req.Instructions.IsPresent() && req.Instructions.Val != "" {
		txt := req.Instructions.Val
		sysParts = append(sysParts, geminiWirePart{Text: &txt})
	}
	for _, p := range req.SystemParts {
		if p.Kind == canonical.PartImage {
			if _, err := encodeGeminiImagePart(p.Image, target, "image_url"); err != nil {
				return nil, err
			}
			return nil, canonical.NewRejectionError("unsupported_image_content", "system", "Gemini systemInstruction supports text parts only")
		}
		if p.Kind == canonical.PartText {
			txt := p.Text
			sysParts = append(sysParts, geminiWirePart{Text: &txt})
		}
	}

	callNamesByID := make(map[string]string)
	contents := make([]geminiWireContent, 0, len(req.Items))

	for i := 0; i < len(req.Items); {
		item := req.Items[i]

		if item.Kind == canonical.ItemMessage && (item.Role == canonical.RoleSystem || item.Role == canonical.RoleDeveloper) {
			for _, p := range item.Parts {
				if p.Kind == canonical.PartImage {
					if _, err := encodeGeminiImagePart(p.Image, target, "image_url"); err != nil {
						return nil, err
					}
					return nil, canonical.NewRejectionError("unsupported_image_content", "system", "Gemini systemInstruction supports text parts only")
				}
				if p.Kind == canonical.PartText {
					txt := p.Text
					sysParts = append(sysParts, geminiWirePart{Text: &txt})
				}
			}
			i++
			continue
		}

		// Model turn consolidation across contiguous reasoning, assistant message, and function_call items.
		isModelItem := item.Kind == canonical.ItemReasoning ||
			item.Kind == canonical.ItemFunctionCall ||
			(item.Kind == canonical.ItemMessage && item.Role == canonical.RoleAssistant)
		if isModelItem {
			var modelParts []geminiWirePart
			var pendingTextParts []geminiWirePart
			hasNonEmptySemanticPart := false
			hadMessagesReasoning := false
			hasBareEmptyAssistant := false

			flushPendingText := func() {
				if len(pendingTextParts) > 0 {
					modelParts = append(modelParts, pendingTextParts...)
					pendingTextParts = nil
				}
			}

			for i < len(req.Items) {
				cur := req.Items[i]
				if cur.Kind == canonical.ItemReasoning {
					if cur.Reasoning != nil {
						if cur.Reasoning.Cache != nil {
							return nil, canonical.NewRejectionError("invalid_cache_control", "thinking", "thinking blocks cannot carry cache_control")
						}
						if cur.Reasoning.Origin.Protocol == canonical.ProtocolAnthropicMessages {
							hadMessagesReasoning = true
						}
						joined := strings.Join(cur.Reasoning.Text, "")
						// R06 #6 / T12: thoughtSignature is forwarded ONLY when originating from Google.
						sig := ""
						if cur.Reasoning.Signature != "" && cur.Reasoning.Origin.Supplier == "google" {
							sig = cur.Reasoning.Signature
						}
						if joined != "" || sig != "" {
							flushPendingText()
							thoughtFlag := true
							var txtPtr *string
							if joined != "" {
								txtPtr = &joined
							}
							part := geminiWirePart{
								Text:             txtPtr,
								ThoughtSignature: sig,
							}
							if joined != "" {
								part.Thought = &thoughtFlag
							}
							modelParts = append(modelParts, part)
							hasNonEmptySemanticPart = true
						}
					}
					i++
					continue
				}
				if cur.Kind == canonical.ItemFunctionCall {
					if cur.Call != nil {
						if cur.Call.Name == "" {
							return nil, canonical.NewRejectionError("invalid_tool_call", "function_call", "functionCall requires name")
						}
						if cur.Call.CallID != "" {
							callNamesByID[cur.Call.CallID] = cur.Call.Name
						}
						var argsObj json.RawMessage
						var err error
						switch cur.Call.Args.Kind {
						case canonical.ArgsObject:
							argsObj, err = compactGeminiJSONObject(cur.Call.Args.Object, "invalid_tool_arguments", "function_call.arguments", "Gemini functionCall.args must be a valid JSON object")
						case canonical.ArgsText:
							argsObj, err = compactGeminiJSONObject([]byte(cur.Call.Args.Text), "invalid_tool_arguments", "function_call.arguments", "Gemini functionCall.args must be a valid JSON object")
						default:
							argsObj = json.RawMessage("{}")
						}
						if err != nil {
							return nil, err
						}

						// R06 #6: Only preserve Google-originated thoughtSignature.
						sig := ""
						if cur.Call.ThoughtSignature != "" && cur.Call.SignatureOrigin.Supplier == "google" {
							sig = cur.Call.ThoughtSignature
						}

						// Drop placeholder empty assistant text parts when paired with functionCall.
						filteredPending := pendingTextParts[:0]
						for _, pt := range pendingTextParts {
							if pt.Text == nil || *pt.Text != "" || pt.InlineData != nil {
								filteredPending = append(filteredPending, pt)
							}
						}
						pendingTextParts = filteredPending
						flushPendingText()

						modelParts = append(modelParts, geminiWirePart{
							FunctionCall: &geminiWireFunctionCall{
								ID:   cur.Call.CallID,
								Name: cur.Call.Name,
								Args: argsObj,
							},
							ThoughtSignature: sig,
						})
						hasNonEmptySemanticPart = true
					}
					i++
					continue
				}
				if cur.Kind == canonical.ItemMessage && cur.Role == canonical.RoleAssistant {
					if len(cur.Parts) == 0 {
						hasBareEmptyAssistant = true
					} else {
						for _, p := range cur.Parts {
							switch p.Kind {
							case canonical.PartText:
								if p.Text == "" && hasNonEmptySemanticPart {
									continue
								}
								txt := p.Text
								if txt != "" {
									hasNonEmptySemanticPart = true
								}
								pendingTextParts = append(pendingTextParts, geminiWirePart{Text: &txt})
							case canonical.PartRefusal:
								ref := p.Refusal
								if ref != "" {
									hasNonEmptySemanticPart = true
								}
								pendingTextParts = append(pendingTextParts, geminiWirePart{Text: &ref})
							case canonical.PartImage:
								inline, err := encodeGeminiImagePart(p.Image, target, "image_url")
								if err != nil {
									return nil, err
								}
								hasNonEmptySemanticPart = true
								pendingTextParts = append(pendingTextParts, geminiWirePart{InlineData: inline})
							default:
								return nil, canonical.NewRejectionError("unsupported_content_part_type", "content", "unsupported assistant content part kind")
							}
						}
					}
					i++
					continue
				}
				break
			}

			if hasNonEmptySemanticPart {
				filteredPending := pendingTextParts[:0]
				for _, pt := range pendingTextParts {
					if pt.Text == nil || *pt.Text != "" || pt.InlineData != nil {
						filteredPending = append(filteredPending, pt)
					}
				}
				pendingTextParts = filteredPending
			}
			flushPendingText()

			if len(modelParts) == 0 && (hadMessagesReasoning || hasBareEmptyAssistant) {
				empty := ""
				modelParts = []geminiWirePart{{Text: &empty}}
			}
			contents = appendGeminiContent(contents, "model", modelParts)
			continue
		}

		// R06 #2: Historical ItemFunctionResult -> role:"user" functionResponse parts (with call_id -> name resolution & structured is_error).
		if item.Kind == canonical.ItemFunctionResult {
			var userParts []geminiWirePart
			var companionImages []*geminiWireInlineData

			for i < len(req.Items) && req.Items[i].Kind == canonical.ItemFunctionResult {
				res := req.Items[i].Result
				i++
				if res == nil {
					continue
				}
				fnName := res.Name
				if fnName == "" && res.CallID != "" {
					fnName = callNamesByID[res.CallID]
				}
				if fnName == "" {
					return nil, canonical.NewRejectionError("missing_tool_call_association", "tool_call_id", "cannot resolve function name for Gemini functionResponse without matching function_call")
				}

				var textParts []string
				for _, p := range res.Parts {
					switch p.Kind {
					case canonical.PartImage:
						if req.SourceProtocol != canonical.ProtocolAnthropicMessages {
							return nil, canonical.NewRejectionError("unsupported_image_content", "function_call_output", "multimodal function_call_output is unsupported")
						}
						inline, err := encodeGeminiImagePart(p.Image, target, "tool_result")
						if err != nil {
							return nil, err
						}
						companionImages = append(companionImages, inline)
					case canonical.PartText:
						textParts = append(textParts, p.Text)
					}
				}

				var joinedText string
				if res.IsStringForm && len(textParts) == 1 {
					joinedText = textParts[0]
				} else if res.IsStringForm {
					joinedText = strings.Join(textParts, "")
				} else {
					var nonEmpty []string
					for _, t := range textParts {
						if t != "" || len(textParts) == 1 {
							nonEmpty = append(nonEmpty, t)
						}
					}
					joinedText = strings.Join(nonEmpty, "\n")
				}

				respObj, err := buildGeminiFunctionResponseObject(joinedText, res.IsError)
				if err != nil {
					return nil, fmt.Errorf("marshal Gemini functionResponse: %w", err)
				}
				userParts = append(userParts, geminiWirePart{
					FunctionResponse: &geminiWireFunctionResponse{
						ID:       res.CallID,
						Name:     fnName,
						Response: respObj,
					},
				})
			}

			if len(companionImages) > 0 {
				hasNextUserText := false
				if i < len(req.Items) && req.Items[i].Kind == canonical.ItemMessage && req.Items[i].Role == canonical.RoleUser &&
					req.SourceProtocol == canonical.ProtocolAnthropicMessages {
					nextUser := req.Items[i]
					i++
					for _, p := range nextUser.Parts {
						if p.Kind == canonical.PartText && p.Text != "" {
							hasNextUserText = true
							break
						}
					}
					if !hasNextUserText {
						hdr := "Attached image(s) from tool result:"
						userParts = append(userParts, geminiWirePart{Text: &hdr})
					}
					for _, p := range nextUser.Parts {
						switch p.Kind {
						case canonical.PartText:
							txt := p.Text
							userParts = append(userParts, geminiWirePart{Text: &txt})
						case canonical.PartImage:
							inline, err := encodeGeminiImagePart(p.Image, target, "image_url")
							if err != nil {
								return nil, err
							}
							userParts = append(userParts, geminiWirePart{InlineData: inline})
						}
					}
				} else {
					hdr := "Attached image(s) from tool result:"
					userParts = append(userParts, geminiWirePart{Text: &hdr})
				}
				for _, inline := range companionImages {
					userParts = append(userParts, geminiWirePart{InlineData: inline})
				}
			}

			contents = appendGeminiContent(contents, roleUser, userParts)
			continue
		}

		// User message items
		if item.Kind == canonical.ItemMessage && item.Role == canonical.RoleUser {
			userParts := make([]geminiWirePart, 0, len(item.Parts))
			for _, p := range item.Parts {
				switch p.Kind {
				case canonical.PartText:
					txt := p.Text
					userParts = append(userParts, geminiWirePart{Text: &txt})
				case canonical.PartImage:
					inline, err := encodeGeminiImagePart(p.Image, target, "image_url")
					if err != nil {
						return nil, err
					}
					userParts = append(userParts, geminiWirePart{InlineData: inline})
				default:
					return nil, canonical.NewRejectionError("unsupported_content_part_type", "content", "unsupported user content part kind")
				}
			}
			if len(userParts) == 0 {
				empty := ""
				userParts = []geminiWirePart{{Text: &empty}}
			}
			contents = appendGeminiContent(contents, roleUser, userParts)
			i++
			continue
		}

		return nil, canonical.NewRejectionError("unsupported_message_role", "messages.role", fmt.Sprintf("unsupported canonical item role %q for Gemini", item.Role))
	}

	if len(contents) == 0 {
		return nil, canonical.NewRejectionError("missing_messages", "contents", "Gemini generateContent requires at least one non-system content message")
	}

	if len(sysParts) > 0 {
		out.SystemInstruction = &geminiWireContent{Parts: sysParts}
	}
	out.Contents = contents
	if !genCfg.isEmpty() {
		out.GenerationConfig = &genCfg
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical Gemini request: %w", err)
	}
	return encoded, nil
}

// --- Gemini generateContent / streamGenerateContent Native Decoders ---

type geminiWireRespFunctionCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type geminiWireRespPart struct {
	Text             wireOptional[string]        `json:"text"`
	Thought          bool                        `json:"thought"`
	ThoughtSignature string                      `json:"thoughtSignature"`
	InlineData       *geminiWireInlineData       `json:"inlineData"`
	FunctionCall     *geminiWireRespFunctionCall `json:"functionCall"`
}

type geminiWireRespContent struct {
	Role  string               `json:"role"`
	Parts []geminiWireRespPart `json:"parts"`
}

type geminiWireRespCandidate struct {
	Index         *int                   `json:"index"`
	Content       *geminiWireRespContent `json:"content"`
	FinishReason  string                 `json:"finishReason"`
	FinishMessage string                 `json:"finishMessage"`
}

type geminiWirePromptFeedback struct {
	BlockReason        string `json:"blockReason"`
	BlockReasonMessage string `json:"blockReasonMessage"`
}

type geminiWireRespEnvelope struct {
	ResponseID     string                    `json:"responseId"`
	ModelVersion   string                    `json:"modelVersion"`
	Candidates     []geminiWireRespCandidate `json:"candidates"`
	PromptFeedback *geminiWirePromptFeedback `json:"promptFeedback"`
	Error          json.RawMessage           `json:"error"`
	Object         string                    `json:"object"`
	EventType      string                    `json:"event_type"`
}

func isGeminiInteractionsPayload(eventName string, env geminiWireRespEnvelope) bool {
	if eventName != "" && eventName != "message" && eventName != "error" && eventName != "ping" {
		if strings.HasPrefix(eventName, "interaction.") ||
			strings.HasPrefix(eventName, "step.") ||
			strings.HasPrefix(eventName, "content.") {
			return true
		}
	}
	if strings.EqualFold(env.Object, "interaction") || strings.EqualFold(env.Object, "interaction.event") {
		return true
	}
	return env.EventType != ""
}

func classifyGeminiFinishReason(raw string, hasToolCalls bool) canonical.FinishReason {
	normalized := strings.TrimSpace(raw)
	if normalized == "" || normalized == "FINISH_REASON_UNSPECIFIED" {
		return canonical.FinishReason{Category: canonical.FinishNone}
	}
	switch normalized {
	case "STOP":
		if hasToolCalls {
			return canonical.FinishReason{Category: canonical.FinishToolCalls, RawReason: normalized}
		}
		return canonical.FinishReason{Category: canonical.FinishStop, RawReason: normalized}
	case "MAX_TOKENS":
		return canonical.FinishReason{Category: canonical.FinishLength, RawReason: normalized}
	case "SAFETY",
		"RECITATION",
		"LANGUAGE",
		"BLOCKLIST",
		"PROHIBITED_CONTENT",
		"SPII",
		"IMAGE_SAFETY",
		"IMAGE_PROHIBITED_CONTENT",
		"IMAGE_RECITATION":
		return canonical.FinishReason{Category: canonical.FinishContentFilter, RawReason: normalized}
	default:
		return canonical.FinishReason{Category: canonical.FinishUnknown, RawReason: normalized}
	}
}

func normalizeGeminiCallArgsObject(raw json.RawMessage) (json.RawMessage, string) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return json.RawMessage("{}"), "{}"
	}
	if compacted, err := compactJSONRaw(trimmed); err == nil {
		return compacted, string(compacted)
	}
	cloned := bytes.Clone(trimmed)
	return cloned, string(cloned)
}

func decodeGeminiCanonicalResponse(target CanonicalTarget, body []byte) (canonical.Response, error) {
	if target.WireFormat != WireGeminiGenerateContent {
		return canonical.Response{}, canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "expected gemini_generate_content")
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return canonical.Response{}, fmt.Errorf("invalid Gemini generateContent response JSON")
	}

	var env geminiWireRespEnvelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return canonical.Response{}, fmt.Errorf("unmarshal Gemini generateContent response: %w", err)
	}
	if isGeminiInteractionsPayload("", env) {
		return canonical.Response{}, fmt.Errorf("unsupported Gemini Interactions API payload in generateContent decoder")
	}
	if errRaw := bytes.TrimSpace(env.Error); len(errRaw) > 0 && !bytes.Equal(errRaw, []byte("null")) {
		return canonical.Response{}, fmt.Errorf("upstream Gemini error payload")
	}

	model := env.ModelVersion
	if model == "" {
		model = target.Model
	}
	resp := canonical.Response{
		ID:    env.ResponseID,
		Model: model,
	}

	// Candidateless promptFeedback.blockReason -> FinishContentFilter + RawReason (downstream-responses.md:80-83).
	if len(env.Candidates) == 0 {
		if env.PromptFeedback != nil && env.PromptFeedback.BlockReason != "" && env.PromptFeedback.BlockReason != "BLOCK_REASON_UNSPECIFIED" {
			resp.Choices = []canonical.Choice{{
				Index: 0,
				Finish: canonical.FinishReason{
					Category:  canonical.FinishContentFilter,
					RawReason: env.PromptFeedback.BlockReason,
				},
			}}
		}
		return resp, nil
	}

	googleOrigin := canonical.OpaqueOrigin{
		Protocol: canonical.ProtocolGeminiGenerate,
		Supplier: "google",
	}
	resp.Choices = make([]canonical.Choice, 0, len(env.Candidates))

	for ordinal, cand := range env.Candidates {
		cIdx := ordinal
		if cand.Index != nil {
			cIdx = *cand.Index
		}

		choice := canonical.Choice{
			Index: cIdx,
		}
		hasToolCalls := false

		if cand.Content != nil {
			var pendingVisible []canonical.Part
			flushVisible := func() {
				if len(pendingVisible) == 0 {
					return
				}
				choice.Items = append(choice.Items, canonical.Item{
					Kind:           canonical.ItemMessage,
					Role:           canonical.RoleAssistant,
					Parts:          pendingVisible,
					MessageIsArray: len(pendingVisible) > 1,
				})
				pendingVisible = nil
			}

			for _, p := range cand.Content.Parts {
				if p.FunctionCall != nil {
					flushVisible()
					hasToolCalls = true
					argsObj, _ := normalizeGeminiCallArgsObject(p.FunctionCall.Args)
					var sigOrigin canonical.OpaqueOrigin
					if p.ThoughtSignature != "" {
						sigOrigin = googleOrigin
					}
					choice.Items = append(choice.Items, canonical.Item{
						Kind: canonical.ItemFunctionCall,
						Role: canonical.RoleAssistant,
						Call: &canonical.FunctionCall{
							CallID:           p.FunctionCall.ID, // Preserves native ID or "" for F07 B02 salted slot alias
							Name:             p.FunctionCall.Name,
							Args:             canonical.Arguments{Kind: canonical.ArgsObject, Object: argsObj},
							ThoughtSignature: p.ThoughtSignature,
							SignatureOrigin:  sigOrigin,
						},
					})
					continue
				}

				if p.Thought {
					flushVisible()
					thoughtText := p.Text.toCanonical().ValueOr("")
					sig := p.ThoughtSignature
					if thoughtText == "" && sig == "" {
						continue
					}
					if n := len(choice.Items); n > 0 && choice.Items[n-1].Kind == canonical.ItemReasoning && choice.Items[n-1].Reasoning != nil && choice.Items[n-1].Reasoning.Signature == "" {
						if thoughtText != "" {
							choice.Items[n-1].Reasoning.Text = append(choice.Items[n-1].Reasoning.Text, thoughtText)
						}
						if sig != "" {
							choice.Items[n-1].Reasoning.Signature = sig
						}
						continue
					}
					var texts []string
					if thoughtText != "" {
						texts = []string{thoughtText}
					}
					choice.Items = append(choice.Items, canonical.Item{
						Kind: canonical.ItemReasoning,
						Role: canonical.RoleAssistant,
						Reasoning: &canonical.ReasoningItem{
							Text:      texts,
							Signature: sig,
							Origin:    googleOrigin,
						},
					})
					continue
				}

				// Visible part (plus optional trailing Google thoughtSignature on non-thought part).
				if p.ThoughtSignature != "" {
					attached := false
					for ri := len(choice.Items) - 1; ri >= 0; ri-- {
						if choice.Items[ri].Kind == canonical.ItemReasoning && choice.Items[ri].Reasoning != nil && choice.Items[ri].Reasoning.Signature == "" {
							choice.Items[ri].Reasoning.Signature = p.ThoughtSignature
							choice.Items[ri].Reasoning.Origin = googleOrigin
							attached = true
							break
						}
					}
					if !attached {
						flushVisible()
						choice.Items = append(choice.Items, canonical.Item{
							Kind: canonical.ItemReasoning,
							Role: canonical.RoleAssistant,
							Reasoning: &canonical.ReasoningItem{
								Signature: p.ThoughtSignature,
								Origin:    googleOrigin,
							},
						})
					}
				}

				if p.InlineData != nil && p.InlineData.Data != "" {
					mimeType := p.InlineData.MimeType
					if mimeType == "" {
						mimeType = "application/octet-stream"
					}
					pendingVisible = append(pendingVisible, canonical.Part{
						Kind: canonical.PartImage,
						Image: &canonical.ImagePart{
							Source:    canonical.ImageDataURL,
							URL:       "data:" + mimeType + ";base64," + p.InlineData.Data,
							MediaType: mimeType,
							Base64Ref: p.InlineData.Data,
						},
					})
				}
				if p.Text.state == canonical.Present {
					if p.Text.val != "" || (!hasToolCalls && len(choice.Items) == 0 && len(pendingVisible) == 0) {
						pendingVisible = append(pendingVisible, canonical.Part{
							Kind: canonical.PartText,
							Text: p.Text.val,
						})
					}
				}
			}
			flushVisible()
		}

		choice.Finish = classifyGeminiFinishReason(cand.FinishReason, hasToolCalls)
		resp.Choices = append(resp.Choices, choice)
	}

	return resp, nil
}

type geminiStreamChoiceState struct {
	nextItemIndex  int
	activeKind     canonical.ItemKind
	reasoningIndex int
	messageIndex   int
	nextToolIndex  int
}

func (cs *geminiStreamChoiceState) allocItemIndex() int {
	idx := cs.nextItemIndex
	cs.nextItemIndex++
	return idx
}

type geminiCanonicalEventDecoder struct {
	started    bool
	responseID string
	model      string
	choices    map[int]*geminiStreamChoiceState
}

func newGeminiCanonicalEventDecoder(target CanonicalTarget) (CanonicalEventDecoder, error) {
	if target.WireFormat != WireGeminiGenerateContent {
		return nil, canonical.NewRejectionError("unsupported_wire_format", string(target.WireFormat), "expected gemini_generate_content")
	}
	return &geminiCanonicalEventDecoder{
		model:   target.Model,
		choices: make(map[int]*geminiStreamChoiceState),
	}, nil
}

func (d *geminiCanonicalEventDecoder) ensureStarted(emit func(canonical.Event) error) error {
	if d.started {
		return nil
	}
	d.started = true
	return emit(canonical.Event{
		Kind:  canonical.EventResponseStart,
		ID:    d.responseID,
		Model: d.model,
		Role:  canonical.RoleAssistant,
	})
}

func (d *geminiCanonicalEventDecoder) Decode(name string, payload []byte, emit func(canonical.Event) error) error {
	if name != "" && name != "message" && name != "error" && name != "ping" {
		if strings.HasPrefix(name, "interaction.") || strings.HasPrefix(name, "step.") || strings.HasPrefix(name, "content.") {
			return fmt.Errorf("unsupported Gemini Interactions API event %q in generateContent decoder", name)
		}
	}

	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		if name == "ping" {
			return emit(canonical.Event{Kind: canonical.EventPing})
		}
		return nil
	}
	if bytes.Equal(trimmed, []byte(streamEndDataValue)) {
		return emit(canonical.Event{
			Kind:  canonical.EventProtocolDone,
			ID:    d.responseID,
			Model: d.model,
		})
	}

	var env geminiWireRespEnvelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		if name == "error" {
			return emit(canonical.Event{
				Kind:     canonical.EventProviderError,
				ID:       d.responseID,
				Model:    d.model,
				RawError: bytes.Clone(trimmed),
			})
		}
		return fmt.Errorf("unmarshal Gemini stream event: %w", err)
	}
	if isGeminiInteractionsPayload(name, env) {
		return fmt.Errorf("unsupported Gemini Interactions API payload in generateContent decoder")
	}

	if env.ResponseID != "" {
		d.responseID = env.ResponseID
	}
	if env.ModelVersion != "" {
		d.model = env.ModelVersion
	}

	errRaw := bytes.TrimSpace(env.Error)
	if name == "error" || (len(errRaw) > 0 && !bytes.Equal(errRaw, []byte("null"))) {
		if len(errRaw) == 0 || bytes.Equal(errRaw, []byte("null")) {
			errRaw = trimmed
		}
		return emit(canonical.Event{
			Kind:     canonical.EventProviderError,
			ID:       d.responseID,
			Model:    d.model,
			RawError: bytes.Clone(errRaw),
		})
	}

	if len(env.Candidates) == 0 {
		if env.PromptFeedback != nil && env.PromptFeedback.BlockReason != "" && env.PromptFeedback.BlockReason != "BLOCK_REASON_UNSPECIFIED" {
			if err := d.ensureStarted(emit); err != nil {
				return err
			}
			return emit(canonical.Event{
				Kind:        canonical.EventChoiceFinish,
				ChoiceIndex: 0,
				ID:          d.responseID,
				Model:       d.model,
				Finish: canonical.FinishReason{
					Category:  canonical.FinishContentFilter,
					RawReason: env.PromptFeedback.BlockReason,
				},
			})
		}
		return nil
	}

	googleOrigin := canonical.OpaqueOrigin{
		Protocol: canonical.ProtocolGeminiGenerate,
		Supplier: "google",
	}

	for ordinal, cand := range env.Candidates {
		cIdx := ordinal
		if cand.Index != nil {
			cIdx = *cand.Index
		}
		cs, exists := d.choices[cIdx]
		if !exists {
			cs = &geminiStreamChoiceState{
				reasoningIndex: -1,
				messageIndex:   -1,
			}
			d.choices[cIdx] = cs
		}

		if cand.Content != nil {
			for _, p := range cand.Content.Parts {
				if p.FunctionCall != nil {
					cs.activeKind = canonical.ItemFunctionCall
					tIdx := cs.nextToolIndex
					cs.nextToolIndex++
					itemIdx := cs.allocItemIndex()
					_, argsText := normalizeGeminiCallArgsObject(p.FunctionCall.Args)

					var sigOrigin canonical.OpaqueOrigin
					if p.ThoughtSignature != "" {
						sigOrigin = googleOrigin
					}
					if err := d.ensureStarted(emit); err != nil {
						return err
					}
					if err := emit(canonical.Event{
						Kind:             canonical.EventToolCallStart,
						ChoiceIndex:      cIdx,
						ItemIndex:        itemIdx,
						ToolIndex:        tIdx,
						CallID:           p.FunctionCall.ID, // "" when omitted -> F07 B02 salted alias
						ID:               d.responseID,
						Model:            d.model,
						Role:             canonical.RoleAssistant,
						ToolName:         p.FunctionCall.Name,
						ThoughtSignature: p.ThoughtSignature,
						SignatureOrigin:  sigOrigin,
					}); err != nil {
						return err
					}
					if err := emit(canonical.Event{
						Kind:             canonical.EventToolCallArgsDelta,
						ChoiceIndex:      cIdx,
						ItemIndex:        itemIdx,
						ToolIndex:        tIdx,
						CallID:           p.FunctionCall.ID,
						ID:               d.responseID,
						Model:            d.model,
						Role:             canonical.RoleAssistant,
						ToolName:         p.FunctionCall.Name,
						Text:             argsText,
						ThoughtSignature: p.ThoughtSignature,
						SignatureOrigin:  sigOrigin,
					}); err != nil {
						return err
					}
					continue
				}

				if p.Thought {
					thoughtText := p.Text.toCanonical().ValueOr("")
					if thoughtText == "" && p.ThoughtSignature == "" {
						continue
					}
					if cs.activeKind != canonical.ItemReasoning || cs.reasoningIndex < 0 {
						cs.reasoningIndex = cs.allocItemIndex()
						cs.activeKind = canonical.ItemReasoning
					}
					if err := d.ensureStarted(emit); err != nil {
						return err
					}
					if thoughtText != "" {
						if err := emit(canonical.Event{
							Kind:            canonical.EventReasoningDelta,
							ChoiceIndex:     cIdx,
							ItemIndex:       cs.reasoningIndex,
							PartIndex:       0,
							ID:              d.responseID,
							Model:           d.model,
							Role:            canonical.RoleAssistant,
							Text:            thoughtText,
							SignatureOrigin: googleOrigin,
						}); err != nil {
							return err
						}
					}
					if p.ThoughtSignature != "" {
						if err := emit(canonical.Event{
							Kind:             canonical.EventReasoningSignatureDelta,
							ChoiceIndex:      cIdx,
							ItemIndex:        cs.reasoningIndex,
							ID:               d.responseID,
							Model:            d.model,
							Role:             canonical.RoleAssistant,
							Text:             p.ThoughtSignature,
							ThoughtSignature: p.ThoughtSignature,
							SignatureOrigin:  googleOrigin,
						}); err != nil {
							return err
						}
					}
					continue
				}

				if p.Text.state == canonical.Present && p.Text.val != "" {
					if cs.activeKind != canonical.ItemMessage || cs.messageIndex < 0 {
						cs.messageIndex = cs.allocItemIndex()
						cs.activeKind = canonical.ItemMessage
					}
					if err := d.ensureStarted(emit); err != nil {
						return err
					}
					if err := emit(canonical.Event{
						Kind:        canonical.EventTextDelta,
						ChoiceIndex: cIdx,
						ItemIndex:   cs.messageIndex,
						PartIndex:   0,
						ID:          d.responseID,
						Model:       d.model,
						Role:        canonical.RoleAssistant,
						Text:        p.Text.val,
					}); err != nil {
						return err
					}
				}

				if p.ThoughtSignature != "" {
					if cs.reasoningIndex < 0 {
						cs.reasoningIndex = cs.allocItemIndex()
					}
					if err := d.ensureStarted(emit); err != nil {
						return err
					}
					if err := emit(canonical.Event{
						Kind:             canonical.EventReasoningSignatureDelta,
						ChoiceIndex:      cIdx,
						ItemIndex:        cs.reasoningIndex,
						ID:               d.responseID,
						Model:            d.model,
						Role:             canonical.RoleAssistant,
						Text:             p.ThoughtSignature,
						ThoughtSignature: p.ThoughtSignature,
						SignatureOrigin:  googleOrigin,
					}); err != nil {
						return err
					}
				}
			}
		}

		if cand.FinishReason != "" {
			finish := classifyGeminiFinishReason(cand.FinishReason, cs.nextToolIndex > 0)
			if finish.Category != canonical.FinishNone {
				if err := d.ensureStarted(emit); err != nil {
					return err
				}
				if err := emit(canonical.Event{
					Kind:        canonical.EventChoiceFinish,
					ChoiceIndex: cIdx,
					ID:          d.responseID,
					Model:       d.model,
					Finish:      finish,
				}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (d *geminiCanonicalEventDecoder) End() error {
	return nil
}
