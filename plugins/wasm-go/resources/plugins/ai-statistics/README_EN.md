---
title: AI Statistics
keywords: [higress, AI, observability]
description: AI Statistics plugin configuration reference
---

> Plugin version: `2.0.4`

## Introduction

Provides basic AI observability capabilities, including metrics, logs, and traces. Safe base facts do not depend on ai-proxy; ai-proxy schema-v2 metadata is optional trusted enrichment for bounded logical operations and path templates.

## Runtime Properties

Plugin Phase: `DEFAULT`
Plugin Priority: `900`

## Configuration instructions

The default request of the plug-in conforms to the openai protocol format and provides the following basic observable values. Users do not need special configuration:

- metric: It provides indicators such as input token, output token, rt of the first token (streaming request), total request rt, etc., and supports observation in the four dimensions of gateway, routing, service, and model.
- log: Provides input_token, output_token, model, llm_service_duration, llm_first_token_duration and other fields

Users can also expand observable values ​​through configuration:

| Name             | Type  | Required | Default | Description |
|----------------|-------|------|-----|------------------------|
| `use_default_attributes` | bool | optional | false | Enables the full built-in attribute set, including messages, answer, and question. Intended for debugging and auditing. |
| `use_default_response_attributes` | bool | optional | false | Enables the lightweight built-in response set with model and token statistics. Recommended for high-throughput production use. |
| `attributes` | []Attribute | optional  | -   | Information that the user wants to record in log/span |
| `disable_openai_usage` | bool | optional  | false   | When using a non-OpenAI-compatible protocol, the support for model and token is non-standard. Setting the configuration to true can prevent errors. |
| `value_length_limit` | int | optional  | conditional | Length limit for each value |
| `path_scope` | string | optional | `legacy_suffixes` | `legacy_suffixes` or `bound_target` |
| `enable_path_suffixes` | []string | optional | see legacy behavior below | Legacy-only path gating, including explicit `*`; bound_target only accepts omission or [] |
| `enable_methods` | []string | optional | none | Optional exact-case filtering in legacy_suffixes; bound_target only accepts omission or [] |
| `enable_content_types` | []string | optional | all when omitted | Only buffer response bodies for configured content types; an omitted list matches all content types |
| `session_id_header` | string | optional  | -   | Specify the header name to read session ID from. If not configured, it will automatically search in the following priority: `x-openclaw-session-key`, `x-clawdbot-session-key`, `x-moltbot-session-key`, `x-agent-session`. Session ID can be used to trace multi-turn Agent conversations |

The conditional runtime defaults for `value_length_limit` are `32000` (plain/custom configuration), `10485760` (full default attributes), and `4000` (lightweight default response attributes). Content parsing or buffering choices do not suppress terminal safe base facts.

### Request scope

- Missing or explicit `path_scope: legacy_suffixes` preserves historical behavior. When either built-in default-attribute mode is enabled and suffixes are omitted, the effective defaults are `/completions` and `/messages`; without built-in defaults, omission preserves the existing all-path behavior.
- `path_scope: bound_target` uses the Higress binding target. Omitted or literal-empty `enable_path_suffixes: []` and `enable_methods: []` add no narrowing and synthesize no path list.
- bound_target rejects every explicit non-empty path/method list, including ["*"], so a matched AI route cannot silently escape statistics. Switching to it clears inherited legacy filters. Explicit intersections remain available in legacy_suffixes.
- ai-statistics matches `enable_methods` with exact case; examples use canonical uppercase methods. Omitted route-level fields inherit global configuration.

```yaml
# Legacy-compatible effective defaults
use_default_response_attributes: true
path_scope: legacy_suffixes
enable_path_suffixes:
  - /completions
  - /messages
```

```yaml
# Bound target with no additional filters
path_scope: bound_target
```

```yaml
# Explicit intersection for standalone legacy deployments
path_scope: legacy_suffixes
enable_path_suffixes:
  - /v1/responses
enable_methods:
  - POST
```

With only _rules_ and no global configuration, each matching rule parses a complete statistics configuration. Set its attributes, content types, and usage options there. With a valid initialized global configuration, rules override only the original three routing fields; other complete configuration fields belong at the root and are rejected in rules. Managed publication uses path_scope: bound_target with both filter lists omitted or empty; Higress bindings determine scope.

## Safe base observability contract

Safe base facts stand independently; ai-proxy schema-v2 metadata is optional trusted enrichment.

Every eligible completed request emits bounded `route` and `path`, a normalized and bounded `method`, `status_code`, positive `latency_ms`, and bounded `error_class`, plus exactly one `request_count` and one `request_duration_ms`. These safe base facts are independent of model, logical operation, token, or usage extraction. Opaque successes, opaque errors, and parsing failures still emit the base facts; unavailable optional dimensions are omitted rather than guessed. Trusted provider parsers may enrich model/token facts, but arbitrary paths or opaque bodies never authorize inference. Raw response bodies and unbounded provider error text never enter metric attributes.

In bound_target, token extensions retain only nonnegative integers actually present in valid JSON or complete SSE frames. Input, output, and total remain independently known or missing; missing or invalid values are never filled with zero, and partial usage does not derive total. Reported zero is retained. The existing parser/counter supports decimal integers through int64; larger values stay unknown without changing response bytes. Fragmented SSE frames use the existing body limit; incomplete or oversized data only stops optional parsing, while base facts still finalize. Legacy extraction remains compatible.

Base statistics cover matching requests that reached this plugin callback. A preceding authentication filter can reject before it runs; this plugin cannot invent statistics for that path, which requires verification of actual filter order and native gateway observation. These source changes require a newly built, precisely registered artifact; an existing version number does not prove they are active.

Version `2.0.4` also guarantees that the per-configuration counter cache is initialized on first use, including rule-override lifecycle paths, so the first eligible base metric cannot fail on a nil cache.

Response headers capture only the bounded status and error projection; they do not freeze completed-request latency. The first real terminal seam—non-streaming response-body EOS, streaming EOS, or `wrapper.ProcessStreamDone` fallback for no body callback, an empty body, an unsupported Content-Type, or a parser-skipped lifecycle—uses the same terminal timestamp for metrics and AILog. A request-scoped completion marker makes all terminal callbacks emit exactly once, while response headers and body bytes remain transparent.

A status is trustworthy only when it contains exactly three ASCII digits and is within `100..599`. Every other form uses `status_code=0` and `status_unknown`. Valid `2xx..3xx` uses `none`, `4xx` uses `http_4xx`, `5xx` uses `http_5xx`, and other valid statuses use `status_unknown`.

`route` comes only from trusted Higress `route_name`. It must be non-empty valid UTF-8, at most 128 bytes, and contain no control character; otherwise it is `route_unknown`. `path` accepts only the actually selected adapter's `matched:true` schema v2 `operation_template`. The template must be origin-form, have exactly one leading `/`, be at most 128 bytes, and contain only ASCII letters/digits plus `/ . _ - ~ { }`, with no authority, query, fragment, control character, or Unicode.

No metadata, malformed or over-1024-byte metadata, `matched:false`, schema v1, an unknown schema, or v2 metadata missing either required operation field can provide a trusted template; those cases use `provider_native_unknown`. A present but invalid or oversized selected v2 template uses `path_unknown`, and the submitted value is never emitted. Logical `normalized_operation` must be non-empty, at most 128 bytes, must not start with `/`, may contain only ASCII letters/digits plus `/ . _ - ~`, and must contain no query, fragment, authority, or control character. It is used separately for optional AILog operation enrichment and is never a path template. Schema v1 may retain only a logical operation that passes this validation; it does not trust adapter, adapter version, provider, capability, or usage profile. Unknown, malformed, or unmatched metadata contributes no optional enrichment.

Path attribution never uses a raw path, query, request headers, credentials, request body, or response body. It also never uses dynamic request segments or a hash of the raw path. `usage_missing` is an ai-billing event field, not an ai-statistics fact or metric dimension. When usage, model, logical operation, or token extraction is unavailable or disabled, ai-statistics still emits safe base count/duration and terminal AILog facts, omits only unavailable optional dimensions, and does not create a synthetic `usage_missing` metric label.

Attribute Configuration instructions:

| Name                    | Type   | Required | Default | Description                                                                                                                                                  |
| ----------------------- | ------ | -------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `key`                   | string | required | -       | attribute key                                                                                                                                                |
| `value_source`          | string | conditional | -    | Required for custom attributes; optional for built-in attributes. Values are `fixed_value`, `request_header`, `request_body`, `response_header`, `response_body`, `response_streaming_body` |
| `value`                 | string | conditional | -    | Required for custom attributes; optional for built-in attributes. Specifies how to obtain the attribute value                                                |
| `default_value`         | string | optional | -       | default value for attribute                                                                                                                                  |
| `rule`                  | string | optional | -       | Rule to extract attribute from streaming response, optional values ​​are `first`, `replace`, `append`                                                        |
| `apply_to_log`          | bool   | optional | false   | Whether to record the extracted information in the log                                                                                                       |
| `apply_to_span`         | bool   | optional | false   | Whether to record the extracted information in the link tracking span                                                                                        |
| `trace_span_key`        | string | optional | -       | span attribute key, default is the value of `key`                                                                                                            |
| `as_separate_log_field` | bool   | optional | false   | Whether to use a separate log field, the field name is equal to the value of `key`                                                                           |

The meanings of various values for `value_source` ​​are as follows:

- `fixed_value`: fixed value
- `request_header`: The attribute is obtained through the http request header
- `request_body`: The attribute is obtained through the http request body
- `response_header`: The attribute is obtained through the http response header
- `response_body`: The attribute is obtained through the http response body
- `response_streaming_body`: The attribute is obtained through the http streaming response body

When `value_source` is `response_streaming_body`, `rule` should be configured to specify how to obtain the specified value from the streaming body. The meaning of the value is as follows:

- `first`: extract value from the first valid chunk
- `replace`: extract value from the last valid chunk
- `append`: join value pieces from all valid chunks

Note: for `first` and `replace`, a valid chunk excludes missing paths, JSON `null`, and empty string `""` results. If no chunk contains a valid value, the extracted result is `nil`; configurations that depend on an empty string as an explicit upstream clear signal should account for this behavior difference. The `append` rule keeps its existing concatenation behavior.

### Built-in Attributes

The plugin provides several built-in attribute keys that can be used directly without configuring `value_source` and `value`. These built-in attributes automatically extract corresponding values from requests/responses:

| Built-in Key | Description | Use Case |
|--------------|-------------|----------|
| `question` | User's question content | Supports OpenAI/Claude message formats |
| `answer` | AI's answer content | Supports OpenAI/Claude message formats, both streaming and non-streaming |
| `tool_calls` | Tool call information | OpenAI/Claude tool calls |
| `reasoning` | Reasoning process | OpenAI o1 and other reasoning models |
| `reasoning_tokens` | Number of reasoning tokens (e.g., o1 model) | OpenAI Chat Completions, extracted from `output_token_details.reasoning_tokens` |
| `cached_tokens` | Number of cached tokens | OpenAI Chat Completions, extracted from `input_token_details.cached_tokens` |
| `input_token_details` | Complete input token details (object) | OpenAI/Gemini/Anthropic, includes cache, tool usage, etc. |
| `output_token_details` | Complete output token details (object) | OpenAI/Gemini/Anthropic, includes reasoning tokens, generated images, etc. |

When using built-in attributes, you only need to set `key`, `apply_to_log`, etc., without setting `value_source` and `value`.

**Notes**:
- `reasoning_tokens` and `cached_tokens` are convenience fields extracted from token details, applicable to OpenAI Chat Completions API
- `input_token_details` and `output_token_details` will record the complete token details object as a JSON string

## Configuration example

If you want to record ai-statistic related statistical values in the gateway access log, you need to modify log_format and add a new field based on the original log_format. The example is as follows:

```yaml
'{"ai_log":"%FILTER_STATE(wasm.ai_log:PLAIN)%"}'
```

If the field is set with `as_separate_log_field`, for example:

```yaml
attributes:
  - key: consumer
    value_source: request_header
    value: x-mse-consumer
    apply_to_log: true
    as_separate_log_field: true
```

Then to print in the log, you need to set log_format additionally:

```
'{"consumer":"%FILTER_STATE(wasm.consumer:PLAIN)%"}'
```

### Empty

#### Metric

```
# counter, cumulative count of input tokens
route_upstream_model_consumer_metric_input_token{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 24

# counter, cumulative count of output tokens
route_upstream_model_consumer_metric_output_token{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 507

# counter, cumulative total duration of both streaming and non-streaming requests
route_upstream_model_consumer_metric_llm_service_duration{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 6470

# counter, cumulative count of both streaming and non-streaming requests
route_upstream_model_consumer_metric_llm_duration_count{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 2

# counter, cumulative latency of the first token in streaming requests
route_upstream_model_consumer_metric_llm_first_token_duration{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 340

# counter, cumulative count of streaming requests
route_upstream_model_consumer_metric_llm_stream_duration_count{ai_route="ai-route-aliyun.internal",ai_cluster="outbound|443||llm-aliyun.internal.dns",ai_model="qwen-turbo",ai_consumer="none"} 1
```

Below are some example usages of these metrics:

Average latency of the first token in streaming requests:

```
irate(route_upstream_model_consumer_metric_llm_first_token_duration[2m])
/
irate(route_upstream_model_consumer_metric_llm_stream_duration_count[2m])
```

Average process duration of both streaming and non-streaming requests:

```
irate(route_upstream_model_consumer_metric_llm_service_duration[2m])
/
irate(route_upstream_model_consumer_metric_llm_duration_count[2m])
```

#### Log

```json
{
  "ai_log": "{\"model\":\"qwen-turbo\",\"input_token\":\"10\",\"output_token\":\"69\",\"llm_first_token_duration\":\"309\",\"llm_service_duration\":\"1955\"}"
}
```

If the request contains a session ID header, the log will automatically include a `session_id` field:

```json
{
  "ai_log": "{\"session_id\":\"sess_abc123\",\"model\":\"qwen-turbo\",\"input_token\":\"10\",\"output_token\":\"69\",\"llm_first_token_duration\":\"309\",\"llm_service_duration\":\"1955\"}"
}
```

#### Trace

When the configuration is empty, no additional attributes will be added to the span.

### Record Token Details

Use built-in attributes to record token details for OpenAI Chat Completions:

```yaml
attributes:
  # Use convenient built-in attributes to extract specific fields
  - key: reasoning_tokens  # Reasoning tokens (o1 and other reasoning models)
    apply_to_log: true
  - key: cached_tokens  # Cached tokens from prompt caching
    apply_to_log: true
  # Record complete token details objects
  - key: input_token_details
    apply_to_log: true
  - key: output_token_details
    apply_to_log: true
```

#### Log Example

For requests using prompt caching and reasoning models, the log might look like:

```json
{
  "ai_log": "{\"model\":\"gpt-4o\",\"input_token\":\"100\",\"output_token\":\"50\",\"reasoning_tokens\":\"25\",\"cached_tokens\":\"80\",\"input_token_details\":\"{\\\"cached_tokens\\\":80}\",\"output_token_details\":\"{\\\"reasoning_tokens\\\":25}\",\"llm_service_duration\":\"2000\"}"
}
```

Where:
- `reasoning_tokens`: 25 - Number of tokens generated during reasoning
- `cached_tokens`: 80 - Number of tokens read from cache
- `input_token_details`: Complete input token details (JSON format)
- `output_token_details`: Complete output token details (JSON format)

These details are useful for:
1. **Cost optimization**: Understanding cache hit rates to optimize prompt caching strategy
2. **Performance analysis**: Analyzing reasoning token ratio to evaluate actual overhead of reasoning models
3. **Usage statistics**: Fine-grained statistics of various token types

## Debugging

### Verifying ai_log Content

During testing or debugging, you can enable Higress debug logging to verify the ai_log content:

```bash
# Log format example
2026/01/31 23:29:30 proxy_debug_log: [ai-statistics] [nil] [test-request-id] [ai_log] attributes to be written: {"question":"What is 2+2?","answer":"4","reasoning":"...","tool_calls":[...],"session_id":"sess_123","model":"gpt-4","input_token":20,"output_token":10}
```

This debug log allows you to verify:
- Whether question/answer/reasoning are correctly extracted
- Whether tool_calls are properly concatenated (especially arguments in streaming scenarios)
- Whether session_id is correctly identified
- Whether all fields match expectations

### Extract token usage information from non-openai protocols

When setting the protocol to original in ai-proxy, taking Alibaba Cloud Bailian as an example, you can make the following configuration to specify how to extract `model`, `input_token`, `output_token`

```yaml
attributes:
  - key: model
    value_source: response_body
    value: usage.models.0.model_id
    apply_to_log: true
    apply_to_span: false
  - key: input_token
    value_source: response_body
    value: usage.models.0.input_tokens
    apply_to_log: true
    apply_to_span: false
  - key: output_token
    value_source: response_body
    value: usage.models.0.output_tokens
    apply_to_log: true
    apply_to_span: false
```

#### Metric

```
route_upstream_model_consumer_metric_input_token{ai_route="bailian",ai_cluster="qwen",ai_model="qwen-max"} 343
route_upstream_model_consumer_metric_output_token{ai_route="bailian",ai_cluster="qwen",ai_model="qwen-max"} 153
route_upstream_model_consumer_metric_llm_service_duration{ai_route="bailian",ai_cluster="qwen",ai_model="qwen-max"} 3725
route_upstream_model_consumer_metric_llm_duration_count{ai_route="bailian",ai_cluster="qwen",ai_model="qwen-max"} 1
```

#### Log

```json
{
  "ai_log": "{\"model\":\"qwen-max\",\"input_token\":\"343\",\"output_token\":\"153\",\"llm_service_duration\":\"19110\"}"
}
```

#### Trace

Three additional attributes `model`, `input_token`, and `output_token` can be seen in the trace spans.

### Cooperate with authentication and authentication record consumer

```yaml
attributes:
  - key: consumer
    value_source: request_header
    value: x-mse-consumer
    apply_to_log: true
```

### Record questions and answers

#### Record only current turn's question and answer

```yaml
attributes:
  - key: question # Record the current turn's question (last user message)
    value_source: request_body
    value: messages.@reverse.0.content
    apply_to_log: true
  - key: answer
    value_source: response_streaming_body
    value: choices.0.delta.content
    rule: append
    apply_to_log: true
  - key: answer
    value_source: response_body
    value: choices.0.message.content
    apply_to_log: true
```

#### Record complete multi-turn conversation history (Recommended)

For multi-turn Agent conversation scenarios, using built-in attributes greatly simplifies the configuration:

```yaml
session_id_header: "x-session-id"  # Optional, specify session ID header
attributes:
  - key: messages     # Complete conversation history
    value_source: request_body
    value: messages
    apply_to_log: true
  - key: question     # Built-in, auto-extracts last user message
    apply_to_log: true
  - key: answer       # Built-in, auto-extracts answer
    apply_to_log: true
  - key: reasoning    # Built-in, auto-extracts reasoning process
    apply_to_log: true
  - key: tool_calls   # Built-in, auto-extracts tool calls
    apply_to_log: true
```

**Built-in Attributes:**

The plugin provides the following built-in attribute keys that automatically extract values without configuring `value_source` and `value` fields:

| Built-in Key | Description | Default value_source |
|-------------|-------------|----------------------|
| `question` | Automatically extracts the last user message | `request_body` |
| `answer` | Automatically extracts answer content (supports OpenAI/Claude protocols) | `response_streaming_body` / `response_body` |
| `tool_calls` | Automatically extracts and assembles tool calls (streaming scenarios auto-concatenate arguments by index) | `response_streaming_body` / `response_body` |
| `reasoning` | Automatically extracts reasoning process (reasoning_content, e.g., DeepSeek-R1) | `response_streaming_body` / `response_body` |

> **Note**: If `value_source` and `value` are configured, the configured values take priority for backward compatibility.

Example log output:

```json
{
  "ai_log": "{\"session_id\":\"sess_abc123\",\"messages\":[{\"role\":\"user\",\"content\":\"What's the weather in Beijing?\"}],\"question\":\"What's the weather in Beijing?\",\"reasoning\":\"The user wants to know the weather in Beijing, I need to call the weather query tool.\",\"tool_calls\":[{\"index\":0,\"id\":\"call_abc123\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"location\\\":\\\"Beijing\\\"}\"}}],\"model\":\"deepseek-reasoner\"}"
}
```

**Streaming tool_calls handling:**

The plugin automatically identifies each independent tool call by the `index` field, concatenates fragmented `arguments` strings, and outputs the complete tool call list.

### Path and Content Type Filtering Configuration Examples

#### Process Only Specific AI Paths

```yaml
enable_path_suffixes:
  - "/v1/chat/completions"
  - "/v1/embeddings"
  - "/generateContent"
```

#### Process Only Specific Content Types

```yaml
enable_content_types:
  - "text/event-stream"
  - "application/json"
```

#### Process All Paths (Wildcard)

```yaml
enable_path_suffixes:
  - "*"
```

#### Complete Configuration Example

```yaml
enable_path_suffixes:
  - "/v1/chat/completions"
  - "/v1/embeddings"
  - "/generateContent"
enable_content_types:
  - "text/event-stream"
  - "application/json"
attributes:
  - key: model
    value_source: request_body
    value: model
    apply_to_log: true
  - key: consumer
    value_source: request_header
    value: x-mse-consumer
    apply_to_log: true
```
