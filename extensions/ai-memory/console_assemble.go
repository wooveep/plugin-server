package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-memory/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	memoryAssembleSchemaVersion = 1

	memoryAssembleDecisionInject     = "inject"
	memoryAssembleDecisionRecentOnly = "recent_only"
	memoryAssembleDecisionSkip       = "skip"
	memoryAssembleDecisionBypass     = "bypass"

	memoryAssembleResponseContextKey = "memoryAssembleResponse"

	maxMemoryAssembleRequestBytes  = 256 * 1024
	maxMemoryAssembleResponseBytes = 256 * 1024
)

type memoryNamedFact struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

type memoryAssembleRequest struct {
	NativeIdentity    *memoryNativeIdentity `json:"native_identity,omitempty"`
	SchemaVersion     int                   `json:"schema_version"`
	Tenant            string                `json:"tenant"`
	Consumer          string                `json:"consumer"`
	SessionID         string                `json:"session_id,omitempty"`
	Route             memoryNamedFact       `json:"route"`
	Model             memoryNamedFact       `json:"model"`
	RequestID         string                `json:"request_id"`
	RequestPath       string                `json:"request_path"`
	CurrentQuestion   string                `json:"current_question"`
	MessagesDigest    string                `json:"messages_digest"`
	MemoryMode        string                `json:"memory_mode"`
	RecentWindowTurns int                   `json:"recent_window_turns,omitempty"`
	MemoryTokenBudget int                   `json:"memory_token_budget,omitempty"`
	SemanticTopK      int                   `json:"semantic_top_k,omitempty"`
	PolicyVersion     string                `json:"policy_version,omitempty"`
}

type memoryConsoleAssembleResponse struct {
	SchemaVersion  int                     `json:"schema_version"`
	Decision       string                  `json:"decision"`
	MemoryMessage  *memoryAssembleMessage  `json:"memory_message,omitempty"`
	MemoryMessages []memoryAssembleMessage `json:"memory_messages,omitempty"`
	RecentMessages []memoryAssembleMessage `json:"recent_messages,omitempty"`
	Trace          map[string]interface{}  `json:"trace,omitempty"`
	Diagnostics    map[string]interface{}  `json:"diagnostics,omitempty"`
	Error          string                  `json:"error,omitempty"`
}

type memoryConsoleAssembleEnvelope struct {
	Data  *memoryConsoleAssembleResponse `json:"data,omitempty"`
	Error interface{}                    `json:"error,omitempty"`
}

type memoryAssembleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func buildMemoryAssembleRequestBody(ctx wrapper.HttpContext, c config.PluginConfig) ([]byte, error) {
	route := memoryNamedFact{Name: ctx.GetStringContext(memoryRouteContextKey, "")}
	if c.NativeIdentity != nil {
		route = memoryNamedFact{ID: c.NativeIdentity.RouteID, Name: ctx.GetStringContext(memoryRouteNameContextKey, "")}
	}
	return json.Marshal(memoryAssembleRequest{
		NativeIdentity:    nativeMemoryIdentity(ctx, c),
		SchemaVersion:     memoryAssembleSchemaVersion,
		Tenant:            ctx.GetStringContext(memoryTenantContextKey, ""),
		Consumer:          ctx.GetStringContext(memoryConsumerContextKey, ""),
		SessionID:         ctx.GetStringContext(memorySessionContextKey, ""),
		Route:             route,
		Model:             memoryNamedFact{Name: ctx.GetStringContext(memoryModelContextKey, "")},
		RequestID:         ctx.GetStringContext(memoryRequestIDContextKey, ""),
		RequestPath:       ctx.GetStringContext(memoryRequestPathContextKey, ""),
		CurrentQuestion:   ctx.GetStringContext(memoryUserContentContextKey, ""),
		MessagesDigest:    ctx.GetStringContext(memoryRequestDigestContextKey, ""),
		MemoryMode:        c.Route.MemoryMode,
		RecentWindowTurns: c.Route.RecentWindowTurns,
		MemoryTokenBudget: c.Route.MemoryTokenBudget,
		SemanticTopK:      c.Route.SemanticTopK,
		PolicyVersion:     c.Route.PolicyVersion,
	})
}

func shouldUseMemoryAssemble(c config.PluginConfig) bool {
	return c.Route.MemoryMode == config.MemoryModeDigest || c.Route.MemoryMode == config.MemoryModeSemantic
}

func dispatchMemoryAssemble(ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) error {
	body, err := buildMemoryAssembleRequestBody(ctx, c)
	if err != nil {
		return err
	}
	if len(body) > maxMemoryAssembleRequestBytes {
		return errors.New("Console assemble request too large")
	}
	cluster := wrapper.FQDNCluster{
		FQDN: c.ConsoleInternal.ServiceName,
		Port: int64(c.ConsoleInternal.ServicePort),
	}
	headers := [][2]string{
		{":method", http.MethodPost},
		{":path", c.ConsoleInternal.AssemblePath},
		{":authority", cluster.HostName()},
		{"content-type", "application/json"},
	}
	if strings.TrimSpace(c.ConsoleInternal.AuthToken) != "" {
		headers = append(headers, [2]string{"authorization", "Bearer " + strings.TrimSpace(c.ConsoleInternal.AuthToken)})
	}
	timeout := memoryAssembleTimeout(c)
	_, err = proxywasm.DispatchHttpCall(cluster.ClusterName(), headers, body, nil, timeout, func(numHeaders, bodySize, numTrailers int) {
		if bodySize > maxMemoryAssembleResponseBytes {
			log.Warnf("[ai-memory] Console assemble response too large, fail open")
			replaceMemoryRequestBodyWithRecentFallback(ctx, log)
			proxywasm.ResumeHttpRequest()
			return
		}
		responseBody, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
		if err != nil {
			log.Warnf("[ai-memory] Console assemble response body unavailable, fail open")
			replaceMemoryRequestBodyWithRecentFallback(ctx, log)
			proxywasm.ResumeHttpRequest()
			return
		}
		responseHeaders, _ := proxywasm.GetHttpCallResponseHeaders()
		statusCode := httpStatusFromHeaders(responseHeaders)
		handleMemoryAssembleResponse(statusCode, responseBody, ctx, c, log)
	})
	return err
}

func handleMemoryAssembleResponse(statusCode int, body []byte, ctx wrapper.HttpContext, c config.PluginConfig, log log.Log) {
	if statusCode != http.StatusOK {
		log.Warnf("[ai-memory] Console assemble returned status %d, fail open", statusCode)
		replaceMemoryRequestBodyWithRecentFallback(ctx, log)
		proxywasm.ResumeHttpRequest()
		return
	}
	response, err := parseMemoryAssembleResponse(body)
	if err != nil {
		log.Warnf("[ai-memory] Console assemble response rejected, fail open: %v", err)
		replaceMemoryRequestBodyWithRecentFallback(ctx, log)
		proxywasm.ResumeHttpRequest()
		return
	}
	ctx.SetContext(memoryAssembleResponseContextKey, response)
	input := memoryAssemblyInputFromResponse(response, c.Route.InjectRole)
	if len(input.Recent) == 0 && memoryAssembleDecisionAllowsRecent(response.Decision) {
		input.Recent = recentMemoryMessages(ctx)
	}
	knownEmpty := false
	if input.MemoryMessage == nil && memoryAssembleDecisionAllowsRecent(response.Decision) {
		// Only Chat's original assembly rule excludes Recent for a complete
		// client/History conversation; other protocols keep their original rule.
		current, _ := ctx.GetContext(memoryCurrentMessagesContextKey).([]sessionctx.OpenAIMessage)
		if countCurrentUserMessages(current) > 1 {
			input.Recent = nil
			knownEmpty = true
		} else {
			knownEmpty = input.Recent != nil && len(input.Recent) == 0
		}
	}
	if knownEmpty {
		if err := writeMemoryCacheDigest(input); err != nil {
			log.Warnf("[ai-memory] trusted cache digest unavailable for empty assembled context: %v", err)
		}
	} else if input.MemoryMessage != nil || len(input.Recent) > 0 {
		replaceMemoryRequestBody(ctx, input, log)
	}
	proxywasm.ResumeHttpRequest()
}

func memoryAssembleTimeout(c config.PluginConfig) uint32 {
	if c.Route.AssembleTimeoutMS > 0 {
		return uint32(c.Route.AssembleTimeoutMS)
	}
	if c.ConsoleInternal.TimeoutMS > 0 {
		return uint32(c.ConsoleInternal.TimeoutMS)
	}
	return 100
}

func httpStatusFromHeaders(headers [][2]string) int {
	for _, header := range headers {
		if header[0] != ":status" {
			continue
		}
		statusCode, err := strconv.Atoi(header[1])
		if err != nil {
			return http.StatusBadGateway
		}
		return statusCode
	}
	return http.StatusBadGateway
}

func parseMemoryAssembleResponse(body []byte) (memoryConsoleAssembleResponse, error) {
	if len(body) > maxMemoryAssembleResponseBytes {
		return memoryConsoleAssembleResponse{}, errors.New("Console assemble response too large")
	}
	var response memoryConsoleAssembleResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return memoryConsoleAssembleResponse{}, errors.New("Console assemble response is not valid JSON")
	}
	fromEnvelope := false
	if strings.TrimSpace(response.Decision) == "" {
		var envelope memoryConsoleAssembleEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			return memoryConsoleAssembleResponse{}, errors.New("Console assemble response is not valid JSON")
		}
		if envelope.Data == nil {
			return memoryConsoleAssembleResponse{}, errors.New("unsupported Console assemble schema version")
		}
		response = *envelope.Data
		fromEnvelope = true
	}
	if response.SchemaVersion != memoryAssembleSchemaVersion && !(fromEnvelope && response.SchemaVersion == 0) {
		return memoryConsoleAssembleResponse{}, errors.New("unsupported Console assemble schema version")
	}
	if strings.TrimSpace(response.Error) != "" {
		return memoryConsoleAssembleResponse{}, errors.New("Console assemble returned error response")
	}
	if !validMemoryAssembleDecision(response.Decision) {
		return memoryConsoleAssembleResponse{}, errors.New("unsupported Console assemble decision")
	}
	if response.MemoryMessage != nil {
		if err := validateMemoryMessage(*response.MemoryMessage, true); err != nil {
			return memoryConsoleAssembleResponse{}, err
		}
	}
	for _, message := range response.MemoryMessages {
		if err := validateMemoryMessage(message, true); err != nil {
			return memoryConsoleAssembleResponse{}, err
		}
	}
	if response.MemoryMessage == nil && len(response.MemoryMessages) > 0 {
		response.MemoryMessage = &response.MemoryMessages[0]
	}
	for _, message := range response.RecentMessages {
		if err := validateMemoryMessage(message, false); err != nil {
			return memoryConsoleAssembleResponse{}, err
		}
	}
	return response, nil
}

func validMemoryAssembleDecision(decision string) bool {
	switch decision {
	case memoryAssembleDecisionInject, memoryAssembleDecisionRecentOnly, memoryAssembleDecisionSkip, memoryAssembleDecisionBypass:
		return true
	default:
		return false
	}
}

func memoryAssembleDecisionAllowsRecent(decision string) bool {
	return decision == memoryAssembleDecisionInject || decision == memoryAssembleDecisionRecentOnly
}

func validateMemoryMessage(message memoryAssembleMessage, allowSystem bool) error {
	switch message.Role {
	case "user", "assistant":
	case "system":
		if !allowSystem {
			return errors.New("system role is not supported in recent messages")
		}
	case "developer":
		if !allowSystem {
			return errors.New("developer role is not supported in recent messages")
		}
	default:
		return errors.New("unsupported memory message role")
	}
	if len(message.Content) > maxRecentMessageBytes {
		return errors.New("memory message too large")
	}
	return nil
}

func (m memoryAssembleMessage) toOpenAIMessage() sessionctx.OpenAIMessage {
	return sessionctx.OpenAIMessage{
		Role:    m.Role,
		Content: m.Content,
	}
}

func memoryAssembleMessagesToOpenAI(messages []memoryAssembleMessage) []sessionctx.OpenAIMessage {
	out := make([]sessionctx.OpenAIMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, message.toOpenAIMessage())
	}
	return out
}
