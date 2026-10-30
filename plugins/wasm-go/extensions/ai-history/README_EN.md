---
title: AI History Dialogue
keywords: [ AI Gateway, AI History Dialogue ]
description: AI History Dialogue Plugin Configuration Reference
---
## Functional Description
`AI History Dialogue` implements user identity recognition based on request headers and automatically caches the historical dialogues of corresponding users, which are then automatically filled into the context in subsequent dialogues. It also supports users to actively query historical dialogues.

**Note**

> When the path suffix matches `ai-history/query`, it will return the historical dialogues.

## Runtime Properties
Plugin Execution Phase: `Default Phase`
Plugin Execution Priority: `650`

## Configuration Fields
| Name              | Data Type | Required | Default Value         | Description                                                                                             |
|-------------------|-----------|----------|-----------------------|---------------------------------------------------------------------------------------------------------|
| identityHeader    | string    | optional | "Authorization"       | The request header for identity resolution, can be Authorization, X-Mse-Consumer, etc.                  |
| fillHistoryCnt    | integer   | optional | 3                     | Default number of historical dialogues to be filled.                                                    |
| cacheKeyPrefix    | string    | optional | "higress-ai-history:" | Prefix for Redis cache key.                                                                             |
| cacheTTL          | integer   | optional | 0                     | Cache expiration time in seconds, default value is 0, meaning it never expires.                         |
| redis.serviceName | string    | required | -                     | Redis service name, full FQDN name with service type, e.g., my-redis.dns, redis.my-ns.svc.cluster.local |
| redis.servicePort | integer   | optional | 6379                  | Redis service port.                                                                                     |
| redis.timeout     | integer   | optional | 1000                  | Timeout for requests to Redis, in milliseconds.                                                         |
| redis.username    | string    | optional | -                     | Username for logging into Redis.                                                                        |
| redis.password    | string    | optional | -                     | Password for logging into Redis.                                                                        |
| redis.database    | int       | optional | 0                     | The database ID used, for example, configured as 1, corresponds to `SELECT 1`.                          |


## Usage Example
### Configuration Information
```yaml
redis:
  serviceName: my-redis.dns
  timeout: 2000
```

### Request Example
**Auto-fill Request Example:**

First Round Request:
```
 curl 'http://example.com/api/openai/v1/chat/completions?fill_history_cnt=3' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer sk-Nzf7RtkdS4s0zFyn5575124129254d9bAf9473A5D7D06dD3'
  --data-raw '{"model":"qwen-long","frequency_penalty":0,"max_tokens":800,"stream":false,"messages":[
        {
            "role": "user",
            "content": "Can Higress replace Nginx?"
        }
    ],"presence_penalty":0,"temperature":0.7,"top_p":0.95}'
```
After Request Fill:
> First round request, no fill. Consistent with the original request.

First Round Response:
```json
{
  "id": "02f4c621-820e-97d4-a905-1e3d0d8f59c6",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "While both Higress and Nginx have gateway functionalities, their design philosophies and application scenarios differ. Nginx is better known as a high-performance HTTP and reverse proxy server, while Higress is a cloud-native gateway that integrates many cloud-native features such as service mesh, observability, and security management in addition to basic routing capabilities.\n\nTherefore, if you want to deploy applications in a cloud-native environment and wish to obtain advanced features required for modern applications, such as service governance, gray release, circuit breaker and rate limiting, and security authentication, then Higress can be a good alternative to Nginx. However, if it's a relatively simple static website or only requires basic reverse proxy functionality, traditional Nginx configurations may be simpler and more direct."
      },
      "finish_reason": "stop"
    }
  ],
  "created": 1724077770,
  "model": "qwen-long",
  "object": "chat.completion",
  "usage": {
    "prompt_tokens": 7316,
    "completion_tokens": 164,
    "total_tokens": 7480
  }
}
```

Second Round Request:
```
 curl 'http://example.com/api/openai/v1/chat/completions?fill_history_cnt=3' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer sk-Nzf7RtkdS4s0zFyn5575124129254d9bAf9473A5D7D06dD3'
  --data-raw '{"model":"qwen-long","frequency_penalty":0,"max_tokens":800,"stream":false,"messages":[
        {
            "role": "user",
            "content": "What about Spring Cloud GateWay?"
        }
    ],"presence_penalty":0,"temperature":0.7,"top_p":0.95}'
```
After Request Fill:
> Second round request, automatically filled with the historical dialogue from the previous round.
``` 
 curl 'http://example.com/api/openai/v1/chat/completions?fill_history_cnt=3' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer sk-Nzf7RtkdS4s0zFyn5575124129254d9bAf9473A5D7D06dD3'
  --data-raw '{"model":"qwen-long","frequency_penalty":0,"max_tokens":800,"stream":false,"messages":[
         {
            "role": "user",
            "content": "Can Higress replace Nginx?"
        },
        {
            "role": "assistant",
            "content": "While both Higress and Nginx have gateway functionalities, their design philosophies and application scenarios differ. Nginx is better known as a high-performance HTTP and reverse proxy server, while Higress is a cloud-native gateway that integrates many cloud-native features such as service mesh, observability, and security management in addition to basic routing capabilities.\n\nTherefore, if you want to deploy applications in a cloud-native environment and wish to obtain advanced features required for modern applications, such as service governance, gray release, circuit breaker and rate limiting, and security authentication, then Higress can be a good alternative to Nginx. However, if it's a relatively simple static website or only requires basic reverse proxy functionality, traditional Nginx configurations may be simpler and more direct."
        },
        {
            "role": "user",
            "content": "What about Spring Cloud GateWay?"
        }
    ],"presence_penalty":0,"temperature":0.7,"top_p":0.95}'
```

Each round of requests only needs to include the current question and the number of historical dialogues to fill in, enabling automatic filling of historical dialogues.

**Example of Retrieving Historical Data:**
```
curl 'http://example.com/api/openai/v1/chat/completions/ai-history/query?cnt=3' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer sk-Nzf7RtkdS4s0zFyn5575124129254d9bAf9473A5D7D06dD3'
```

Response Example:
```json
[
  {
    "role": "user",
    "content": "Can Higress replace Nginx?"
  },
  {
    "role": "assistant",
    "content": "While both Higress and Nginx have gateway functionalities, their design philosophies and application scenarios differ. Nginx is better known as a high-performance HTTP and reverse proxy server, while Higress is a cloud-native gateway that integrates many cloud-native features such as service mesh, observability, and security management in addition to basic routing capabilities.\\n\\nTherefore, if you want to deploy applications in a cloud-native environment and wish to obtain advanced features required for modern applications, such as service governance, gray release, circuit breaker and rate limiting, and security authentication, then Higress can be a good alternative to Nginx. However, if it's a relatively simple static website or only requires basic reverse proxy functionality, traditional Nginx configurations may be simpler and more direct."
  },
  {
    "role": "user",
    "content": "What about Spring Cloud GateWay?"
  },
  {
    "role": "assistant",
    "content": "Compared to Spring Cloud Gateway, Higress is also an API gateway, but there are some key differences between them:\\n\\n- **Design Philosophy**: Spring Cloud Gateway mainly targets service-to-service communication and routing in microservice architectures. As part of the Spring Cloud ecosystem, it is more focused on scenarios for Java developers in microservices. In contrast, Higress, as a cloud-native gateway, not only focuses on service communication but also offers a range of cloud-native features such as service mesh, observability, security management, etc.\\n- **Deployment Method**: Spring Cloud Gateway typically runs as part of microservice applications on application servers, whereas Higress usually deploys as an independent microservice or containerized service in Kubernetes environments, suitable for modern cloud-native deployment models.\\n- **Scalability and Integration**: Higress provides wider integration and support, for example, deep integration with ecosystems like Istio, Kubernetes, etc., making it better suited for complex cloud-native environments.\\n\\nTherefore, if your application is built on Spring Cloud and you want a lightweight, easy-to-integrate service gateway, then Spring Cloud Gateway might be a suitable choice. However, if you are building or refactoring cloud-native applications and require more powerful routing rules, service governance, observability, etc., then Higress is likely a better choice."
  },
  {
    "role": "user",
    "content": "Can Higress replace Nginx?"
  },
  {
    "role": "assistant",
    "content": "While both Higress and Nginx have gateway functionalities, their design philosophies and application scenarios differ. Nginx is better known as a high-performance HTTP and reverse proxy server, while Higress is a cloud-native gateway that integrates many cloud-native features such as service mesh, observability, and security management in addition to basic routing capabilities.\\n\\nTherefore, if you want to deploy applications in a cloud-native environment and wish to obtain advanced features required for modern applications, such as service governance, gray release, circuit breaker and rate limiting, and security authentication, then Higress can be a good alternative to Nginx. However, if it's a relatively simple static website or only requires basic reverse proxy functionality, traditional Nginx configurations may be simpler and more direct."
  }
]
```

Returns three historical dialogues. If the `cnt` parameter is not provided, it will default to returning all cached historical dialogues.

## Managed route and protocol support (source candidate)

Managed rules are complete configurations; they do not inherit another route's binding. An explicitly bound history policy must be enabled. `enabled: false` and request/response `Cache-Control: no-store` bypass history. Publish `path_scope: bound_target`, `client_protocol`, `operation_kind`, `public_model` and the five-field `native_identity` together. Supported client protocols are `openai_chat`, `openai_responses` and `anthropic_messages`; `text_generate` includes multimodal input with text output. URL aliases never select a protocol. Other operations/protocols, non-text or incomplete output and pending tool results pass through with a `history_status` reason. JSON and SSE capture reuse the existing protocol adapters.

The binding contains release_id, target_id, route_id, policy_id and positive int64 policy_version only. Runtime caller account_id, api_key_id, credential_generation, tenant_id and principal_id are read once from preceding native authentication. They are not client configuration. The Redis namespace hashes that complete identity, protocol and original public model. Missing caller or trusted pre-rewrite body basis bypasses without legacy fallback. The publisher must clear reserved client headers and capture the original basis before any history/memory rewrite; this leaf does not change filter priority.

Native conversation items retain attachments, tool context, extension fields and precise JSON numbers. Injection retains current system/developer messages, Claude system, Responses instructions and other fields. Requests already containing multiple user turns are not injected again; their final turn can still be recorded. Provider-managed Responses continuation is not combined with a second history. Pending tool responses are not made into text answers; a later completed text response retains the original tool chain in its input. Native rules do not short-circuit an alias containing the legacy history-query path. Existing Redis connection, history length and TTL semantics remain. Legacy standalone configuration continues to use its original explicit identity header and query endpoint.

A new exact ART package version, Console/SDK policy projection, trusted body-basis wiring, filter ordering and real publication verification remain shared integration work. Focused source/host checks are not Wasm build or deployment acceptance.

Trusted publication assigns exactly one first-body capturer from the exact artifact phases and priorities within each phase. Set `capture_body_basis: true` only when history is that first rewriter. Capture clears client basis headers and completes before enabled/no-store/caller bypasses. The default false consumes a preceding trusted basis. Later memory uses false and model-mapper uses `consume_body_basis: true`; without preprocessors, mapper keeps `capture_body_basis: true`. Header presence never selects a mode. This module does not change filter metadata.
