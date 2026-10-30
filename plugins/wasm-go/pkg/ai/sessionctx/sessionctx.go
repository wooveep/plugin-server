package sessionctx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	defaultTenantHeader    = "x-mse-tenant"
	defaultConsumerHeader  = "x-mse-consumer"
	defaultSessionHeader   = "x-openclaw-session-key"
	defaultRequestIDHeader = "x-request-id"
)

type RequestFactOptions struct {
	TenantHeader    string
	ConsumerHeader  string
	SessionHeader   string
	RequestIDHeader string
}

type RequestFacts struct {
	Tenant    string
	Consumer  string
	SessionID string
	RequestID string
}

func ExtractRequestFacts(headers [][2]string, opts RequestFactOptions, propertyRequestID string) RequestFacts {
	tenantHeader := defaultString(opts.TenantHeader, defaultTenantHeader)
	consumerHeader := defaultString(opts.ConsumerHeader, defaultConsumerHeader)
	sessionHeader := defaultString(opts.SessionHeader, defaultSessionHeader)
	requestIDHeader := defaultString(opts.RequestIDHeader, defaultRequestIDHeader)

	requestID := HeaderValue(headers, requestIDHeader)
	if requestID == "" {
		requestID = propertyRequestID
	}
	return RequestFacts{
		Tenant:    HeaderValue(headers, tenantHeader),
		Consumer:  HeaderValue(headers, consumerHeader),
		SessionID: HeaderValue(headers, sessionHeader),
		RequestID: requestID,
	}
}

func HeaderValue(headers [][2]string, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header[0], name) {
			return header[1]
		}
	}
	return ""
}

func IsJSONContentType(contentType string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func PathMatchesSuffixes(path string, suffixes []string) bool {
	if len(suffixes) == 0 {
		return true
	}
	pathOnly := strings.Split(path, "?")[0]
	for _, suffix := range suffixes {
		if suffix == "" {
			continue
		}
		if strings.HasSuffix(pathOnly, suffix) {
			return true
		}
	}
	return false
}

type OpenAIChatRequest struct {
	Model             string                     `json:"model,omitempty"`
	Stream            bool                       `json:"stream,omitempty"`
	Messages          []OpenAIMessage            `json:"messages,omitempty"`
	Tools             json.RawMessage            `json:"tools,omitempty"`
	ToolChoice        json.RawMessage            `json:"tool_choice,omitempty"`
	ResponseFormat    json.RawMessage            `json:"response_format,omitempty"`
	Temperature       json.RawMessage            `json:"temperature,omitempty"`
	TopP              json.RawMessage            `json:"top_p,omitempty"`
	Seed              json.RawMessage            `json:"seed,omitempty"`
	MaxTokens         json.RawMessage            `json:"max_tokens,omitempty"`
	MaxCompletion     json.RawMessage            `json:"max_completion_tokens,omitempty"`
	Stop              json.RawMessage            `json:"stop,omitempty"`
	N                 json.RawMessage            `json:"n,omitempty"`
	PresencePenalty   json.RawMessage            `json:"presence_penalty,omitempty"`
	FrequencyPenalty  json.RawMessage            `json:"frequency_penalty,omitempty"`
	LogitBias         json.RawMessage            `json:"logit_bias,omitempty"`
	Logprobs          json.RawMessage            `json:"logprobs,omitempty"`
	TopLogprobs       json.RawMessage            `json:"top_logprobs,omitempty"`
	ParallelToolCalls json.RawMessage            `json:"parallel_tool_calls,omitempty"`
	Extra             map[string]json.RawMessage `json:"-"`
}

type OpenAIMessage struct {
	Role         string                     `json:"role,omitempty"`
	Content      interface{}                `json:"content,omitempty"`
	Name         string                     `json:"name,omitempty"`
	ToolCallID   string                     `json:"tool_call_id,omitempty"`
	ToolCalls    json.RawMessage            `json:"tool_calls,omitempty"`
	FunctionCall json.RawMessage            `json:"function_call,omitempty"`
	Refusal      json.RawMessage            `json:"refusal,omitempty"`
	Annotations  json.RawMessage            `json:"annotations,omitempty"`
	Audio        json.RawMessage            `json:"audio,omitempty"`
	Extra        map[string]json.RawMessage `json:"-"`
	contentSet   bool
}

func (request *OpenAIChatRequest) UnmarshalJSON(body []byte) error {
	type requestAlias OpenAIChatRequest
	var parsed requestAlias
	if err := json.Unmarshal(body, &parsed); err != nil {
		return err
	}
	extra, err := unknownRawFields(body, openAIChatRequestFields)
	if err != nil {
		return err
	}
	*request = OpenAIChatRequest(parsed)
	request.Extra = extra
	return nil
}

func (message *OpenAIMessage) UnmarshalJSON(body []byte) error {
	type messageAlias OpenAIMessage
	var parsed messageAlias
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&parsed); err != nil {
		return err
	}
	extra, err := unknownRawFields(body, openAIMessageFields)
	if err != nil {
		return err
	}
	*message = OpenAIMessage(parsed)
	message.contentSet = fieldExists(body, "content")
	message.Extra = extra
	return nil
}

func (message OpenAIMessage) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(message.Extra)+8)
	copyExtraRawFields(fields, message.Extra, openAIMessageFields)
	if err := setStringField(fields, "role", message.Role); err != nil {
		return nil, err
	}
	if message.Content != nil || message.contentSet {
		if err := setField(fields, "content", message.Content); err != nil {
			return nil, err
		}
	}
	if err := setStringField(fields, "name", message.Name); err != nil {
		return nil, err
	}
	if err := setStringField(fields, "tool_call_id", message.ToolCallID); err != nil {
		return nil, err
	}
	setRawField(fields, "tool_calls", message.ToolCalls)
	setRawField(fields, "function_call", message.FunctionCall)
	setRawField(fields, "refusal", message.Refusal)
	setRawField(fields, "annotations", message.Annotations)
	setRawField(fields, "audio", message.Audio)
	return json.Marshal(fields)
}

func ParseOpenAIChatRequest(body []byte) (OpenAIChatRequest, error) {
	var request OpenAIChatRequest
	err := json.Unmarshal(body, &request)
	return request, err
}

func CurrentUserIntent(messages []OpenAIMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		return textContent(messages[i].Content)
	}
	return ""
}

func ReplaceOpenAIChatMessages(body []byte, messages []OpenAIMessage) ([]byte, error) {
	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	if request == nil {
		return nil, errors.New("openai chat request body must be a JSON object")
	}
	if messages == nil {
		messages = []OpenAIMessage{}
	}
	messageBody, err := json.Marshal(messages)
	if err != nil {
		return nil, err
	}
	request["messages"] = messageBody
	return json.Marshal(request)
}

type RequestDigestInput struct {
	Model             string                     `json:"model,omitempty"`
	Messages          []OpenAIMessage            `json:"messages,omitempty"`
	Tools             json.RawMessage            `json:"tools,omitempty"`
	ToolChoice        json.RawMessage            `json:"tool_choice,omitempty"`
	ResponseFormat    json.RawMessage            `json:"response_format,omitempty"`
	Temperature       json.RawMessage            `json:"temperature,omitempty"`
	TopP              json.RawMessage            `json:"top_p,omitempty"`
	Seed              json.RawMessage            `json:"seed,omitempty"`
	MaxTokens         json.RawMessage            `json:"max_tokens,omitempty"`
	MaxCompletion     json.RawMessage            `json:"max_completion_tokens,omitempty"`
	Stop              json.RawMessage            `json:"stop,omitempty"`
	N                 json.RawMessage            `json:"n,omitempty"`
	PresencePenalty   json.RawMessage            `json:"presence_penalty,omitempty"`
	FrequencyPenalty  json.RawMessage            `json:"frequency_penalty,omitempty"`
	LogitBias         json.RawMessage            `json:"logit_bias,omitempty"`
	Logprobs          json.RawMessage            `json:"logprobs,omitempty"`
	TopLogprobs       json.RawMessage            `json:"top_logprobs,omitempty"`
	ParallelToolCalls json.RawMessage            `json:"parallel_tool_calls,omitempty"`
	Extra             map[string]json.RawMessage `json:"-"`
}

type ScopedRequestDigestInput struct {
	Tenant      string
	Consumer    string
	SessionID   string
	Route       string
	RequestPath string
	Model       string
	BodyDigest  string
	RequestID   string
}

func BuildRequestDigest(input RequestDigestInput) (string, error) {
	payload := map[string]interface{}{}
	if input.Model != "" {
		payload["model"] = input.Model
	}
	if input.Messages != nil {
		messages, err := canonicalValue(input.Messages)
		if err != nil {
			return "", err
		}
		payload["messages"] = messages
	}
	if err := addCanonicalRawField(payload, "tools", input.Tools); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "tool_choice", input.ToolChoice); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "response_format", input.ResponseFormat); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "temperature", input.Temperature); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "top_p", input.TopP); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "seed", input.Seed); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "max_tokens", input.MaxTokens); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "max_completion_tokens", input.MaxCompletion); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "stop", input.Stop); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "n", input.N); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "presence_penalty", input.PresencePenalty); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "frequency_penalty", input.FrequencyPenalty); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "logit_bias", input.LogitBias); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "logprobs", input.Logprobs); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "top_logprobs", input.TopLogprobs); err != nil {
		return "", err
	}
	if err := addCanonicalRawField(payload, "parallel_tool_calls", input.ParallelToolCalls); err != nil {
		return "", err
	}
	if err := addCanonicalExtraFields(payload, input.Extra); err != nil {
		return "", err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func BuildOpenAIChatRequestDigest(request OpenAIChatRequest) (string, error) {
	return BuildRequestDigest(RequestDigestInput{
		Model:             request.Model,
		Messages:          request.Messages,
		Tools:             request.Tools,
		ToolChoice:        request.ToolChoice,
		ResponseFormat:    request.ResponseFormat,
		Temperature:       request.Temperature,
		TopP:              request.TopP,
		Seed:              request.Seed,
		MaxTokens:         request.MaxTokens,
		MaxCompletion:     request.MaxCompletion,
		Stop:              request.Stop,
		N:                 request.N,
		PresencePenalty:   request.PresencePenalty,
		FrequencyPenalty:  request.FrequencyPenalty,
		LogitBias:         request.LogitBias,
		Logprobs:          request.Logprobs,
		TopLogprobs:       request.TopLogprobs,
		ParallelToolCalls: request.ParallelToolCalls,
		Extra:             request.Extra,
	})
}

func BuildScopedRequestDigest(input ScopedRequestDigestInput) (string, error) {
	payload := map[string]string{}
	addString := func(key, value string) {
		if value = strings.TrimSpace(value); value != "" {
			payload[key] = value
		}
	}
	addString("tenant", input.Tenant)
	addString("consumer", input.Consumer)
	addString("session_id", input.SessionID)
	addString("route", input.Route)
	addString("request_path", input.RequestPath)
	addString("model", input.Model)
	addString("body_digest", input.BodyDigest)

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type OpenAIChatResponse struct {
	AssistantContent  string
	FinishReason      string
	FinishReasons     []string
	Usage             Usage
	ContainsToolCalls bool
}

type ResponseParseOptions struct {
	AdditionalToolCallPaths []string
}

func ContainsToolUse(response OpenAIChatResponse) bool {
	return response.ContainsToolCalls
}

func ResponseUsage(response OpenAIChatResponse) Usage {
	return response.Usage
}

func ResponseFinishReason(response OpenAIChatResponse) string {
	if len(response.FinishReasons) > 0 {
		return strings.Join(response.FinishReasons, ",")
	}
	return response.FinishReason
}

func ParseOpenAIChatResponse(body []byte) (OpenAIChatResponse, error) {
	return ParseOpenAIChatResponseWithOptions(body, ResponseParseOptions{})
}

func ParseOpenAIChatResponseWithOptions(body []byte, opts ResponseParseOptions) (OpenAIChatResponse, error) {
	var raw struct {
		Choices []struct {
			Message struct {
				Content      interface{}   `json:"content"`
				ToolCalls    []interface{} `json:"tool_calls"`
				FunctionCall interface{}   `json:"function_call"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return OpenAIChatResponse{}, err
	}

	response := OpenAIChatResponse{Usage: raw.Usage}
	if hasAnyJSONPath(body, opts.AdditionalToolCallPaths) {
		response.ContainsToolCalls = true
	}
	if len(raw.Choices) == 0 {
		return response, nil
	}
	firstChoice := raw.Choices[0]
	response.AssistantContent = textContent(firstChoice.Message.Content)
	response.FinishReason = firstChoice.FinishReason
	for _, choice := range raw.Choices {
		if choice.FinishReason != "" {
			response.FinishReasons = append(response.FinishReasons, choice.FinishReason)
		}
		if len(choice.Message.ToolCalls) > 0 ||
			choice.Message.FunctionCall != nil ||
			isToolCallFinishReason(choice.FinishReason) {
			response.ContainsToolCalls = true
		}
	}
	return response, nil
}

type StreamCaptureOptions struct {
	AdditionalToolCallPaths []string
}

type StreamCapture struct {
	content                 strings.Builder
	buffer                  string
	finishReason            string
	usage                   Usage
	containsToolCalls       bool
	additionalToolCallPaths []string
}

func NewStreamCapture(opts StreamCaptureOptions) *StreamCapture {
	return &StreamCapture{additionalToolCallPaths: opts.AdditionalToolCallPaths}
}

func (c *StreamCapture) AppendSSE(chunk []byte) error {
	c.buffer += string(chunk)
	for {
		index := strings.IndexAny(c.buffer, "\r\n")
		if index < 0 {
			return nil
		}
		separator := c.buffer[index]
		line := strings.TrimSpace(c.buffer[:index])
		c.buffer = c.buffer[index+1:]
		if separator == '\r' && strings.HasPrefix(c.buffer, "\n") {
			c.buffer = c.buffer[1:]
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event struct {
			Choices []struct {
				Delta struct {
					Content      json.RawMessage `json:"content"`
					ToolCalls    []interface{}   `json:"tool_calls"`
					FunctionCall interface{}     `json:"function_call"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *Usage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return err
		}
		if event.Usage != nil {
			c.usage = *event.Usage
		}
		if hasAnyJSONPath([]byte(payload), c.additionalToolCallPaths) {
			c.containsToolCalls = true
		}
		for _, choice := range event.Choices {
			c.content.WriteString(rawMessageTextContent(choice.Delta.Content))
			if len(choice.Delta.ToolCalls) > 0 ||
				choice.Delta.FunctionCall != nil ||
				rawContentContainsToolCalls(choice.Delta.Content) ||
				isToolCallFinishReason(choice.FinishReason) {
				c.containsToolCalls = true
			}
			if choice.FinishReason != "" {
				c.finishReason = choice.FinishReason
			}
		}
	}
}

func (c *StreamCapture) AssistantContent() string {
	return c.content.String()
}

func (c *StreamCapture) FinishReason() string {
	return c.finishReason
}

func (c *StreamCapture) Usage() Usage {
	return c.usage
}

func (c *StreamCapture) ContainsToolCalls() bool {
	return c.containsToolCalls
}

type EventEnvelopeInput struct {
	EventKind     string
	Tenant        string
	Consumer      string
	RequestID     string
	RequestDigest string
	StartedAtMS   int64
	EndedAtMS     int64
}

type EventEnvelope struct {
	EventID        string `json:"event_id"`
	IdempotencyKey string `json:"idempotency_key"`
	EventKind      string `json:"event_kind"`
	Tenant         string `json:"tenant,omitempty"`
	Consumer       string `json:"consumer,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	RequestDigest  string `json:"request_digest,omitempty"`
	StartedAtMS    int64  `json:"started_at_ms,omitempty"`
	EndedAtMS      int64  `json:"ended_at_ms,omitempty"`
}

type RedisStreamXADDInput struct {
	Stream         string
	ID             string
	Field          string
	MaxLen         int64
	ApproximateMax bool
	Event          interface{}
}

type FailOpenLogInput struct {
	Operation  string
	Reason     string
	RequestID  string
	Route      string
	Model      string
	Stream     string
	StatusCode int
	Err        error
}

func NewEventEnvelope(input EventEnvelopeInput) EventEnvelope {
	idempotency := stableDigest(struct {
		EventKind     string
		Tenant        string
		Consumer      string
		RequestID     string
		RequestDigest string
	}{
		EventKind:     input.EventKind,
		Tenant:        input.Tenant,
		Consumer:      input.Consumer,
		RequestID:     input.RequestID,
		RequestDigest: input.RequestDigest,
	})
	return EventEnvelope{
		EventID:        stableDigest(input),
		IdempotencyKey: idempotency,
		EventKind:      input.EventKind,
		Tenant:         input.Tenant,
		Consumer:       input.Consumer,
		RequestID:      input.RequestID,
		RequestDigest:  input.RequestDigest,
		StartedAtMS:    input.StartedAtMS,
		EndedAtMS:      input.EndedAtMS,
	}
}

func BuildRedisStreamXADD(input RedisStreamXADDInput) ([]string, error) {
	if input.Stream == "" {
		return nil, errors.New("redis stream name is required")
	}
	if input.MaxLen < 0 {
		return nil, errors.New("redis stream max length must be non-negative")
	}
	if isNilEvent(input.Event) {
		return nil, errors.New("redis stream event is required")
	}
	streamID := defaultString(input.ID, "*")
	field := defaultString(input.Field, "event")
	eventBody, err := json.Marshal(input.Event)
	if err != nil {
		return nil, err
	}

	command := []string{"XADD", input.Stream}
	if input.MaxLen > 0 {
		command = append(command, "MAXLEN")
		if input.ApproximateMax {
			command = append(command, "~")
		}
		command = append(command, strconv.FormatInt(input.MaxLen, 10))
	}
	command = append(command, streamID, field, string(eventBody))
	return command, nil
}

func FailOpenLogFields(input FailOpenLogInput, sensitiveValues ...string) [][2]string {
	fields := [][2]string{{"fail_open", "true"}}
	fields = appendLogField(fields, "operation", input.Operation, sensitiveValues...)
	fields = appendLogField(fields, "reason", input.Reason, sensitiveValues...)
	fields = appendLogField(fields, "request_id", input.RequestID, sensitiveValues...)
	fields = appendLogField(fields, "route", input.Route, sensitiveValues...)
	fields = appendLogField(fields, "model", input.Model, sensitiveValues...)
	fields = appendLogField(fields, "stream", input.Stream, sensitiveValues...)
	if input.StatusCode > 0 {
		fields = append(fields, [2]string{"status_code", strconv.Itoa(input.StatusCode)})
	}
	if input.Err != nil {
		fields = appendLogField(fields, "error", input.Err.Error(), sensitiveValues...)
	}
	return fields
}

var sensitiveLogKeys = []string{
	"authorization",
	"x-api-key",
	"x-internal-bearer",
	"x-redis-password",
	"x-provider-api-key",
}

var openAIChatRequestFields = stringSet(
	"model",
	"stream",
	"messages",
	"tools",
	"tool_choice",
	"response_format",
	"temperature",
	"top_p",
	"seed",
	"max_tokens",
	"max_completion_tokens",
	"stop",
	"n",
	"presence_penalty",
	"frequency_penalty",
	"logit_bias",
	"logprobs",
	"top_logprobs",
	"parallel_tool_calls",
)

var openAIChatRequestDigestExcludedFields = stringSet(
	"stream",
)

var openAIMessageFields = stringSet(
	"role",
	"content",
	"name",
	"tool_call_id",
	"tool_calls",
	"function_call",
	"refusal",
	"annotations",
	"audio",
)

func RedactForLog(value string, sensitiveValues ...string) string {
	result := value
	for _, key := range sensitiveLogKeys {
		result = redactLogKey(result, key)
	}
	for _, sensitiveValue := range orderedSensitiveValues(sensitiveValues) {
		if sensitiveValue == "" {
			continue
		}
		result = strings.ReplaceAll(result, sensitiveValue, "[redacted]")
	}
	return result
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func textContent(value interface{}) string {
	switch content := value.(type) {
	case string:
		return content
	case []interface{}:
		var parts []string
		for _, item := range content {
			object, ok := item.(map[string]interface{})
			if !ok || object["type"] != "text" {
				continue
			}
			text, _ := object["text"].(string)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func rawMessageTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return textContent(value)
}

func rawContentContainsToolCalls(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var content struct {
		ToolCalls []interface{} `json:"tool_calls"`
	}
	if err := json.Unmarshal(raw, &content); err != nil {
		return false
	}
	return len(content.ToolCalls) > 0
}

func isToolCallFinishReason(reason string) bool {
	return reason == "tool_calls" || reason == "function_call"
}

func hasAnyJSONPath(body []byte, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	value, err := canonicalJSON(body)
	if err != nil {
		return false
	}
	for _, path := range paths {
		if jsonPathHasValue(value, path) {
			return true
		}
	}
	return false
}

func jsonPathHasValue(value interface{}, path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	current := value
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			return false
		}
		switch typed := current.(type) {
		case map[string]interface{}:
			next, ok := typed[segment]
			if !ok {
				return false
			}
			current = next
		case []interface{}:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return false
			}
			current = typed[index]
		default:
			return false
		}
	}
	return jsonPathValuePresent(current)
}

func jsonPathValuePresent(value interface{}) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case []interface{}:
		return len(typed) > 0
	case map[string]interface{}:
		return len(typed) > 0
	case string:
		return typed != ""
	case bool:
		return typed
	case json.Number:
		number, err := typed.Float64()
		if err != nil {
			return false
		}
		return number != 0
	case float64:
		return typed != 0
	default:
		return true
	}
}

func appendLogField(fields [][2]string, name, value string, sensitiveValues ...string) [][2]string {
	if value == "" {
		return fields
	}
	return append(fields, [2]string{name, RedactForLog(value, sensitiveValues...)})
}

func isNilEvent(value interface{}) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func orderedSensitiveValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	ordered := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ordered = append(ordered, value)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return len(ordered[i]) > len(ordered[j])
	})
	return ordered
}

func stableDigest(value interface{}) string {
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func redactLogKey(value, key string) string {
	var out strings.Builder
	lower := strings.ToLower(value)
	searchStart := 0
	for {
		index := strings.Index(lower[searchStart:], key)
		if index < 0 {
			out.WriteString(value[searchStart:])
			return out.String()
		}
		index += searchStart
		out.WriteString(value[searchStart : index+len(key)])
		cursor := index + len(key)
		for cursor < len(value) && unicode.IsSpace(rune(value[cursor])) {
			out.WriteByte(value[cursor])
			cursor++
		}
		if cursor < len(value) && value[cursor] == '"' {
			out.WriteByte(value[cursor])
			cursor++
		}
		for cursor < len(value) && unicode.IsSpace(rune(value[cursor])) {
			out.WriteByte(value[cursor])
			cursor++
		}
		if cursor >= len(value) || (value[cursor] != '=' && value[cursor] != ':') {
			searchStart = cursor
			continue
		}
		out.WriteByte(value[cursor])
		cursor++
		for cursor < len(value) && unicode.IsSpace(rune(value[cursor])) {
			out.WriteByte(value[cursor])
			cursor++
		}
		quotedValue := cursor < len(value) && value[cursor] == '"'
		if quotedValue {
			out.WriteByte(value[cursor])
			cursor++
		}
		out.WriteString("[redacted]")
		if quotedValue {
			cursor = quotedLogValueEnd(value, cursor)
			if cursor < len(value) && value[cursor] == '"' {
				out.WriteByte(value[cursor])
				cursor++
			}
		} else {
			cursor = logValueEnd(value, cursor)
		}
		searchStart = cursor
	}
}

func stringSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func unknownRawFields(body []byte, knownFields map[string]struct{}) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, nil
	}
	for field := range knownFields {
		delete(fields, field)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	return fields, nil
}

func fieldExists(body []byte, field string) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return false
	}
	_, ok := fields[field]
	return ok
}

func copyExtraRawFields(fields map[string]json.RawMessage, extra map[string]json.RawMessage, knownFields map[string]struct{}) {
	for field, value := range extra {
		if len(value) == 0 {
			continue
		}
		if _, known := knownFields[field]; known {
			continue
		}
		fields[field] = value
	}
}

func setStringField(fields map[string]json.RawMessage, name, value string) error {
	if value == "" {
		return nil
	}
	return setField(fields, name, value)
}

func setField(fields map[string]json.RawMessage, name string, value interface{}) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	fields[name] = body
	return nil
}

func setRawField(fields map[string]json.RawMessage, name string, value json.RawMessage) {
	if len(value) == 0 {
		return
	}
	fields[name] = value
}

func addCanonicalRawField(fields map[string]interface{}, name string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	value, err := canonicalRawMessage(raw)
	if err != nil {
		return err
	}
	fields[name] = value
	return nil
}

func addCanonicalExtraFields(fields map[string]interface{}, extra map[string]json.RawMessage) error {
	for field, raw := range extra {
		if len(raw) == 0 {
			continue
		}
		if _, exists := fields[field]; exists {
			continue
		}
		if _, excluded := openAIChatRequestDigestExcludedFields[field]; excluded {
			continue
		}
		value, err := canonicalRawMessage(raw)
		if err != nil {
			return err
		}
		fields[field] = value
	}
	return nil
}

func canonicalRawMessage(raw json.RawMessage) (interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	return canonicalJSON(raw)
}

func canonicalValue(value interface{}) (interface{}, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return canonicalJSON(body)
}

func canonicalJSON(body []byte) (interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("invalid JSON: multiple values")
		}
		return nil, err
	}
	return value, nil
}

func logValueEnd(value string, start int) int {
	cursor := start
	for cursor < len(value) {
		if unicode.IsSpace(rune(value[cursor])) && nextLogTokenHasSeparator(value[cursor+1:]) {
			return cursor
		}
		cursor++
	}
	return cursor
}

func quotedLogValueEnd(value string, start int) int {
	escaped := false
	for cursor := start; cursor < len(value); cursor++ {
		switch {
		case escaped:
			escaped = false
		case value[cursor] == '\\':
			escaped = true
		case value[cursor] == '"':
			return cursor
		}
	}
	return len(value)
}

func nextLogTokenHasSeparator(value string) bool {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed == "" {
		return false
	}
	for i, ch := range trimmed {
		if unicode.IsSpace(ch) {
			return false
		}
		if ch == '=' || ch == ':' {
			return i > 0
		}
	}
	return false
}
