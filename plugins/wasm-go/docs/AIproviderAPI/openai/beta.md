# Beta

> Source index: https://developers.openai.com/api/reference/llms.txt
> Fetched: 2026-04-23

This file aggregates beta reference pages that are published under the `resources/beta/...` path in the official OpenAI API reference.
## Assistants

> Source: https://developers.openai.com/api/reference/resources/beta/subresources/assistants
> Fetched: 2026-04-23

Build Assistants that can call models and use tools.

###### [List assistants](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/list)

Deprecated

GET/assistants

###### [Create assistant](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/create)

Deprecated

POST/assistants

###### [Retrieve assistant](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/retrieve)

Deprecated

GET/assistants/{assistant\_id}

###### [Modify assistant](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/update)

Deprecated

POST/assistants/{assistant\_id}

###### [Delete assistant](https://developers.openai.com/api/reference/resources/beta/subresources/assistants/methods/delete)

Deprecated

DELETE/assistants/{assistant\_id}

###### Models

Assistant object { id, created\_at, description, 10 more }

Represents an `assistant` that can call the model and use tools.

id: string

The identifier, which can be referenced in API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the assistant was created.

description: string

The description of the assistant. The maximum length is 512 characters.

maxLength512

instructions: string

The system instructions that the assistant uses. The maximum length is 256,000 characters.

maxLength256000

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: string

ID of the model to use. You can use the [List models](https://developers.openai.com/docs/api-reference/models/list) API to see all of your available models, or see our [Model overview](https://developers.openai.com/docs/models) for descriptions of them.

name: string

The name of the assistant. The maximum length is 256 characters.

maxLength256

object: "assistant"

The object type, which is always `assistant`.

tools: array of [CodeInterpreterTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20code_interpreter_tool%20%3E%20(schema)) { type }  or [FileSearchTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20file_search_tool%20%3E%20(schema)) { type, file\_search }  or [FunctionTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20function_tool%20%3E%20(schema)) { function, type }

A list of tool enabled on the assistant. There can be a maximum of 128 tools per assistant. Tools can be of types `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterTool object { type }

type: "code\_interpreter"

The type of tool being defined: `code_interpreter`

FileSearchTool object { type, file\_search }

type: "file\_search"

The type of tool being defined: `file_search`

file\_search: optional object { max\_num\_results, ranking\_options }

Overrides for the file search tool.

max\_num\_results: optional number

The maximum number of results the file search tool should output. The default is 20 for `gpt-4*` models and 5 for `gpt-3.5-turbo`. This number should be between 1 and 50 inclusive.

Note that the file search tool may output fewer than `max_num_results` results. See the [file search tool documentation](https://developers.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings) for more information.

minimum1

maximum50

ranking\_options: optional object { score\_threshold, ranker }

The ranking options for the file search. If not specified, the file search tool will use the `auto` ranker and a score\_threshold of 0.

See the [file search tool documentation](https://developers.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings) for more information.

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

ranker: optional "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

FunctionTool object { function, type }

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of tool being defined: `function`

response\_format: optional [AssistantResponseFormatOption](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20assistant_response_format_option%20%3E%20(schema))

Specifies the format that the model must output. Compatible with [GPT-4o](https://developers.openai.com/docs/models#gpt-4o), [GPT-4 Turbo](https://developers.openai.com/docs/models#gpt-4-turbo-and-gpt-4), and all GPT-3.5 Turbo models since `gpt-3.5-turbo-1106`.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured Outputs which ensures the model will match your supplied JSON schema. Learn more in the [Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables JSON mode, which ensures the message the model generates is valid JSON.

**Important:** when using JSON mode, you **must** also instruct the model to produce JSON yourself via a system or user message. Without this, the model may generate an unending stream of whitespace until the generation reaches the token limit, resulting in a long-running and seemingly “stuck” request. Also note that the message content may be partially cut off if `finish_reason="length"`, which indicates the generation exceeded `max_tokens` or the conversation exceeded the max context length.

temperature: optional number

What sampling temperature to use, between 0 and 2. Higher values like 0.8 will make the output more random, while lower values like 0.2 will make it more focused and deterministic.

minimum0

maximum2

tool\_resources: optional object { code\_interpreter, file\_search }

A set of resources that are used by the assistant’s tools. The resources are specific to the type of tool. For example, the `code_interpreter` tool requires a list of file IDs, while the `file_search` tool requires a list of vector store IDs.

code\_interpreter: optional object { file\_ids }

file\_ids: optional array of string

A list of [file](https://developers.openai.com/docs/api-reference/files) IDs made available to the `code\_interpreter“ tool. There can be a maximum of 20 files associated with the tool.

file\_search: optional object { vector\_store\_ids }

vector\_store\_ids: optional array of string

The ID of the [vector store](https://developers.openai.com/docs/api-reference/vector-stores/object) attached to this assistant. There can be a maximum of 1 vector store attached to the assistant.

top\_p: optional number

An alternative to sampling with temperature, called nucleus sampling, where the model considers the results of the tokens with top\_p probability mass. So 0.1 means only the tokens comprising the top 10% probability mass are considered.

We generally recommend altering this or temperature but not both.

minimum0

maximum1

AssistantDeleted object { id, deleted, object }

id: string

deleted: boolean

object: "assistant.deleted"

AssistantStreamEvent = object { data, event, enabled }  or object { data, event }  or object { data, event }  or 22 more

Represents an event emitted when streaming a Run.

Each event in a server-sent events stream has an `event` and `data` property:

```
event: thread.created
data: {"id": "thread_123", "object": "thread", ...}
```

We emit events whenever a new object is created, transitions to a new state, or is being
streamed in parts (deltas). For example, we emit `thread.run.created` when a new run
is created, `thread.run.completed` when a run completes, and so on. When an Assistant chooses
to create a message during a run, we emit a `thread.message.created event`, a
`thread.message.in_progress` event, many `thread.message.delta` events, and finally a
`thread.message.completed` event.

We may add additional events over time, so we recommend handling unknown events gracefully
in your code. See the [Assistants API quickstart](https://developers.openai.com/docs/assistants/overview) to learn how to
integrate the Assistants API with streaming.

One of the following:

object { data, event, enabled }

Occurs when a new [thread](https://developers.openai.com/docs/api-reference/threads/object) is created.

data: [Thread](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20thread%20%3E%20(schema)) { id, created\_at, metadata, 2 more }

Represents a thread that contains [messages](https://developers.openai.com/docs/api-reference/messages).

event: "thread.created"

enabled: optional boolean

Whether to enable input audio transcription.

object { data, event }

Occurs when a new [run](https://developers.openai.com/docs/api-reference/runs/object) is created.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.created"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) moves to a `queued` status.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.queued"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) moves to an `in_progress` status.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.in\_progress"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) moves to a `requires_action` status.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.requires\_action"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) is completed.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.completed"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) ends with status `incomplete`.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.incomplete"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) fails.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.failed"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) moves to a `cancelling` status.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.cancelling"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) is cancelled.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.cancelled"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) expires.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.expired"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) is created.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.created"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) moves to an `in_progress` state.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.in\_progress"

object { data, event }

Occurs when parts of a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) are being streamed.

data: [RunStepDeltaEvent](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step_delta_event%20%3E%20(schema)) { id, delta, object }

Represents a run step delta i.e. any changed fields on a run step during streaming.

event: "thread.run.step.delta"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) is completed.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.completed"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) fails.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.failed"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) is cancelled.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.cancelled"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) expires.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.expired"

object { data, event }

Occurs when a [message](https://developers.openai.com/docs/api-reference/messages/object) is created.

data: [Message](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message%20%3E%20(schema)) { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.message.created"

object { data, event }

Occurs when a [message](https://developers.openai.com/docs/api-reference/messages/object) moves to an `in_progress` state.

data: [Message](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message%20%3E%20(schema)) { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.message.in\_progress"

object { data, event }

Occurs when parts of a [Message](https://developers.openai.com/docs/api-reference/messages/object) are being streamed.

data: [MessageDeltaEvent](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message_delta_event%20%3E%20(schema)) { id, delta, object }

Represents a message delta i.e. any changed fields on a message during streaming.

event: "thread.message.delta"

object { data, event }

Occurs when a [message](https://developers.openai.com/docs/api-reference/messages/object) is completed.

data: [Message](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message%20%3E%20(schema)) { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.message.completed"

object { data, event }

Occurs when a [message](https://developers.openai.com/docs/api-reference/messages/object) ends before it is completed.

data: [Message](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message%20%3E%20(schema)) { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.message.incomplete"

ErrorEvent object { data, event }

Occurs when an [error](https://developers.openai.com/docs/guides/error-codes#api-errors) occurs. This can happen due to an internal server error or a timeout.

data: [ErrorObject](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20error_object%20%3E%20(schema)) { code, message, param, type }

event: "error"

DoneEvent object { data, event }

Occurs when a stream ends.

data: "[DONE]"

event: "done"

CodeInterpreterTool object { type }

type: "code\_interpreter"

The type of tool being defined: `code_interpreter`

FileSearchTool object { type, file\_search }

type: "file\_search"

The type of tool being defined: `file_search`

file\_search: optional object { max\_num\_results, ranking\_options }

Overrides for the file search tool.

max\_num\_results: optional number

The maximum number of results the file search tool should output. The default is 20 for `gpt-4*` models and 5 for `gpt-3.5-turbo`. This number should be between 1 and 50 inclusive.

Note that the file search tool may output fewer than `max_num_results` results. See the [file search tool documentation](https://developers.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings) for more information.

minimum1

maximum50

ranking\_options: optional object { score\_threshold, ranker }

The ranking options for the file search. If not specified, the file search tool will use the `auto` ranker and a score\_threshold of 0.

See the [file search tool documentation](https://developers.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings) for more information.

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

ranker: optional "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

FunctionTool object { function, type }

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of tool being defined: `function`

MessageStreamEvent = object { data, event }  or object { data, event }  or object { data, event }  or 2 more

Occurs when a [message](https://developers.openai.com/docs/api-reference/messages/object) is created.

One of the following:

object { data, event }

Occurs when a [message](https://developers.openai.com/docs/api-reference/messages/object) is created.

data: [Message](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message%20%3E%20(schema)) { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.message.created"

object { data, event }

Occurs when a [message](https://developers.openai.com/docs/api-reference/messages/object) moves to an `in_progress` state.

data: [Message](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message%20%3E%20(schema)) { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.message.in\_progress"

object { data, event }

Occurs when parts of a [Message](https://developers.openai.com/docs/api-reference/messages/object) are being streamed.

data: [MessageDeltaEvent](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message_delta_event%20%3E%20(schema)) { id, delta, object }

Represents a message delta i.e. any changed fields on a message during streaming.

event: "thread.message.delta"

object { data, event }

Occurs when a [message](https://developers.openai.com/docs/api-reference/messages/object) is completed.

data: [Message](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message%20%3E%20(schema)) { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.message.completed"

object { data, event }

Occurs when a [message](https://developers.openai.com/docs/api-reference/messages/object) ends before it is completed.

data: [Message](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message%20%3E%20(schema)) { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.message.incomplete"

RunStepStreamEvent = object { data, event }  or object { data, event }  or object { data, event }  or 4 more

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) is created.

One of the following:

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) is created.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.created"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) moves to an `in_progress` state.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.in\_progress"

object { data, event }

Occurs when parts of a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) are being streamed.

data: [RunStepDeltaEvent](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step_delta_event%20%3E%20(schema)) { id, delta, object }

Represents a run step delta i.e. any changed fields on a run step during streaming.

event: "thread.run.step.delta"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) is completed.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.completed"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) fails.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.failed"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) is cancelled.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.cancelled"

object { data, event }

Occurs when a [run step](https://developers.openai.com/docs/api-reference/run-steps/step-object) expires.

data: [RunStep](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

event: "thread.run.step.expired"

RunStreamEvent = object { data, event }  or object { data, event }  or object { data, event }  or 7 more

Occurs when a new [run](https://developers.openai.com/docs/api-reference/runs/object) is created.

One of the following:

object { data, event }

Occurs when a new [run](https://developers.openai.com/docs/api-reference/runs/object) is created.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.created"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) moves to a `queued` status.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.queued"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) moves to an `in_progress` status.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.in\_progress"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) moves to a `requires_action` status.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.requires\_action"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) is completed.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.completed"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) ends with status `incomplete`.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.incomplete"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) fails.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.failed"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) moves to a `cancelling` status.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.cancelling"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) is cancelled.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.cancelled"

object { data, event }

Occurs when a [run](https://developers.openai.com/docs/api-reference/runs/object) expires.

data: [Run](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20run%20%3E%20(schema)) { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

event: "thread.run.expired"

ThreadStreamEvent object { data, event, enabled }

Occurs when a new [thread](https://developers.openai.com/docs/api-reference/threads/object) is created.

data: [Thread](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20thread%20%3E%20(schema)) { id, created\_at, metadata, 2 more }

Represents a thread that contains [messages](https://developers.openai.com/docs/api-reference/messages).

event: "thread.created"

enabled: optional boolean

Whether to enable input audio transcription.

---

## ChatKit

> Source: https://developers.openai.com/api/reference/resources/beta/subresources/chatkit
> Fetched: 2026-04-23

###### Models

ChatKitWorkflow object { id, state\_variables, tracing, version }

Workflow metadata and state returned for the session.

id: string

Identifier of the workflow backing the session.

state\_variables: map[string or boolean or number]

State variable key-value pairs applied when invoking the workflow. Defaults to null when no overrides were provided.

One of the following:

string

boolean

number

tracing: object { enabled }

Tracing settings applied to the workflow.

enabled: boolean

Indicates whether tracing is enabled.

version: string

Specific workflow version used for the session. Defaults to null when using the latest deployment.

##### ChatKitSessions

###### [Cancel chat session](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/sessions/methods/cancel)

POST/chatkit/sessions/{session\_id}/cancel

###### [Create ChatKit session](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/sessions/methods/create)

POST/chatkit/sessions

##### ChatKitThreads

###### [List ChatKit thread items](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/list_items)

GET/chatkit/threads/{thread\_id}/items

###### [Retrieve ChatKit thread](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/retrieve)

GET/chatkit/threads/{thread\_id}

###### [Delete ChatKit thread](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/delete)

DELETE/chatkit/threads/{thread\_id}

###### [List ChatKit threads](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/list)

GET/chatkit/threads

###### Models

ChatSession object { id, chatkit\_configuration, client\_secret, 7 more }

Represents a ChatKit session and its resolved configuration.

id: string

Identifier for the ChatKit session.

chatkit\_configuration: [ChatSessionChatKitConfiguration](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_chatkit_configuration%20%3E%20(schema)) { automatic\_thread\_titling, file\_upload, history }

Resolved ChatKit feature configuration for the session.

client\_secret: string

Ephemeral client secret that authenticates session requests.

expires\_at: number

Unix timestamp (in seconds) for when the session expires.

max\_requests\_per\_1\_minute: number

Convenience copy of the per-minute request limit.

object: "chatkit.session"

Type discriminator that is always `chatkit.session`.

rate\_limits: [ChatSessionRateLimits](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_rate_limits%20%3E%20(schema)) { max\_requests\_per\_1\_minute }

Resolved rate limit values.

status: [ChatSessionStatus](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_status%20%3E%20(schema))

Current lifecycle state of the session.

user: string

User identifier associated with the session.

workflow: [ChatKitWorkflow](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit%20%3E%20(model)%20chatkit_workflow%20%3E%20(schema)) { id, state\_variables, tracing, version }

Workflow metadata for the session.

ChatSessionAutomaticThreadTitling object { enabled }

Automatic thread title preferences for the session.

enabled: boolean

Whether automatic thread titling is enabled.

ChatSessionChatKitConfiguration object { automatic\_thread\_titling, file\_upload, history }

ChatKit configuration for the session.

automatic\_thread\_titling: [ChatSessionAutomaticThreadTitling](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_automatic_thread_titling%20%3E%20(schema)) { enabled }

Automatic thread titling preferences.

file\_upload: [ChatSessionFileUpload](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_file_upload%20%3E%20(schema)) { enabled, max\_file\_size, max\_files }

Upload settings for the session.

history: [ChatSessionHistory](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_history%20%3E%20(schema)) { enabled, recent\_threads }

History retention configuration.

ChatSessionChatKitConfigurationParam object { automatic\_thread\_titling, file\_upload, history }

Optional per-session configuration settings for ChatKit behavior.

automatic\_thread\_titling: optional object { enabled }

Configuration for automatic thread titling. When omitted, automatic thread titling is enabled by default.

enabled: optional boolean

Enable automatic thread title generation. Defaults to true.

file\_upload: optional object { enabled, max\_file\_size, max\_files }

Configuration for upload enablement and limits. When omitted, uploads are disabled by default (max\_files 10, max\_file\_size 512 MB).

enabled: optional boolean

Enable uploads for this session. Defaults to false.

max\_file\_size: optional number

Maximum size in megabytes for each uploaded file. Defaults to 512 MB, which is the maximum allowable size.

maximum512

minimum1

max\_files: optional number

Maximum number of files that can be uploaded to the session. Defaults to 10.

minimum1

history: optional object { enabled, recent\_threads }

Configuration for chat history retention. When omitted, history is enabled by default with no limit on recent\_threads (null).

enabled: optional boolean

Enables chat users to access previous ChatKit threads. Defaults to true.

recent\_threads: optional number

Number of recent ChatKit threads users have access to. Defaults to unlimited when unset.

minimum1

ChatSessionExpiresAfterParam object { anchor, seconds }

Controls when the session expires relative to an anchor timestamp.

anchor: "created\_at"

Base timestamp used to calculate expiration. Currently fixed to `created_at`.

seconds: number

Number of seconds after the anchor when the session expires.

maximum600

minimum1

ChatSessionFileUpload object { enabled, max\_file\_size, max\_files }

Upload permissions and limits applied to the session.

enabled: boolean

Indicates if uploads are enabled for the session.

max\_file\_size: number

Maximum upload size in megabytes.

max\_files: number

Maximum number of uploads allowed during the session.

ChatSessionHistory object { enabled, recent\_threads }

History retention preferences returned for the session.

enabled: boolean

Indicates if chat history is persisted for the session.

recent\_threads: number

Number of prior threads surfaced in history views. Defaults to null when all history is retained.

ChatSessionRateLimits object { max\_requests\_per\_1\_minute }

Active per-minute request limit for the session.

max\_requests\_per\_1\_minute: number

Maximum allowed requests per one-minute window.

ChatSessionRateLimitsParam object { max\_requests\_per\_1\_minute }

Controls request rate limits for the session.

max\_requests\_per\_1\_minute: optional number

Maximum number of requests allowed per minute for the session. Defaults to 10.

minimum1

ChatSessionStatus = "active" or "expired" or "cancelled"

One of the following:

"active"

"expired"

"cancelled"

ChatSessionWorkflowParam object { id, state\_variables, tracing, version }

Workflow reference and overrides applied to the chat session.

id: string

Identifier for the workflow invoked by the session.

state\_variables: optional map[string or boolean or number]

State variables forwarded to the workflow. Keys may be up to 64 characters, values must be primitive types, and the map defaults to an empty object.

One of the following:

string

boolean

number

tracing: optional object { enabled }

Optional tracing overrides for the workflow invocation. When omitted, tracing is enabled by default.

enabled: optional boolean

Whether tracing is enabled during the session. Defaults to true.

version: optional string

Specific workflow version to run. Defaults to the latest deployed version.

ChatKitAttachment object { id, mime\_type, name, 2 more }

Attachment metadata included on thread items.

id: string

Identifier for the attachment.

mime\_type: string

MIME type of the attachment.

name: string

Original display name for the attachment.

preview\_url: string

Preview URL for rendering the attachment inline.

type: "image" or "file"

Attachment discriminator.

One of the following:

"image"

"file"

ChatKitResponseOutputText object { annotations, text, type }

Assistant response text accompanied by optional annotations.

annotations: array of object { source, type }  or object { source, type }

Ordered list of annotations attached to the response text.

One of the following:

File object { source, type }

Annotation that references an uploaded file.

source: object { filename, type }

File attachment referenced by the annotation.

filename: string

Filename referenced by the annotation.

type: "file"

Type discriminator that is always `file`.

type: "file"

Type discriminator that is always `file` for this annotation.

URL object { source, type }

Annotation that references a URL.

source: object { type, url }

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url`.

url: string

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url` for this annotation.

text: string

Assistant generated text.

type: "output\_text"

Type discriminator that is always `output_text`.

ChatKitThread object { id, created\_at, object, 3 more }

Represents a ChatKit thread and its current status.

id: string

Identifier of the thread.

created\_at: number

Unix timestamp (in seconds) for when the thread was created.

object: "chatkit.thread"

Type discriminator that is always `chatkit.thread`.

status: object { type }  or object { reason, type }  or object { reason, type }

Current status for the thread. Defaults to `active` for newly created threads.

One of the following:

Active object { type }

Indicates that a thread is active.

type: "active"

Status discriminator that is always `active`.

Locked object { reason, type }

Indicates that a thread is locked and cannot accept new input.

reason: string

Reason that the thread was locked. Defaults to null when no reason is recorded.

type: "locked"

Status discriminator that is always `locked`.

Closed object { reason, type }

Indicates that a thread has been closed.

reason: string

Reason that the thread was closed. Defaults to null when no reason is recorded.

type: "closed"

Status discriminator that is always `closed`.

title: string

Optional human-readable title for the thread. Defaults to null when no title has been generated.

user: string

Free-form string that identifies your end user who owns the thread.

ChatKitThreadAssistantMessageItem object { id, content, created\_at, 3 more }

Assistant-authored message within a thread.

id: string

Identifier of the thread item.

content: array of [ChatKitResponseOutputText](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_response_output_text%20%3E%20(schema)) { annotations, text, type }

Ordered assistant response segments.

annotations: array of object { source, type }  or object { source, type }

Ordered list of annotations attached to the response text.

One of the following:

File object { source, type }

Annotation that references an uploaded file.

source: object { filename, type }

File attachment referenced by the annotation.

filename: string

Filename referenced by the annotation.

type: "file"

Type discriminator that is always `file`.

type: "file"

Type discriminator that is always `file` for this annotation.

URL object { source, type }

Annotation that references a URL.

source: object { type, url }

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url`.

url: string

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url` for this annotation.

text: string

Assistant generated text.

type: "output\_text"

Type discriminator that is always `output_text`.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.assistant\_message"

Type discriminator that is always `chatkit.assistant_message`.

ChatKitThreadItemList object { data, first\_id, has\_more, 2 more }

A paginated list of thread items rendered for the ChatKit API.

data: array of [ChatKitThreadUserMessageItem](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_thread_user_message_item%20%3E%20(schema)) { id, attachments, content, 5 more }  or [ChatKitThreadAssistantMessageItem](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_thread_assistant_message_item%20%3E%20(schema)) { id, content, created\_at, 3 more }  or [ChatKitWidgetItem](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_widget_item%20%3E%20(schema)) { id, created\_at, object, 3 more }  or 3 more

A list of items

One of the following:

ChatKitThreadUserMessageItem object { id, attachments, content, 5 more }

User-authored messages within a thread.

id: string

Identifier of the thread item.

attachments: array of [ChatKitAttachment](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_attachment%20%3E%20(schema)) { id, mime\_type, name, 2 more }

Attachments associated with the user message. Defaults to an empty list.

id: string

Identifier for the attachment.

mime\_type: string

MIME type of the attachment.

name: string

Original display name for the attachment.

preview\_url: string

Preview URL for rendering the attachment inline.

type: "image" or "file"

Attachment discriminator.

One of the following:

"image"

"file"

content: array of object { text, type }  or object { text, type }

Ordered content elements supplied by the user.

One of the following:

InputText object { text, type }

Text block that a user contributed to the thread.

text: string

Plain-text content supplied by the user.

type: "input\_text"

Type discriminator that is always `input_text`.

QuotedText object { text, type }

Quoted snippet that the user referenced in their message.

text: string

Quoted text content.

type: "quoted\_text"

Type discriminator that is always `quoted_text`.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

inference\_options: object { model, tool\_choice }

Inference overrides applied to the message. Defaults to null when unset.

model: string

Model name that generated the response. Defaults to null when using the session default.

tool\_choice: object { id }

Preferred tool to invoke. Defaults to null when ChatKit should auto-select.

id: string

Identifier of the requested tool.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.user\_message"

ChatKitThreadAssistantMessageItem object { id, content, created\_at, 3 more }

Assistant-authored message within a thread.

id: string

Identifier of the thread item.

content: array of [ChatKitResponseOutputText](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_response_output_text%20%3E%20(schema)) { annotations, text, type }

Ordered assistant response segments.

annotations: array of object { source, type }  or object { source, type }

Ordered list of annotations attached to the response text.

One of the following:

File object { source, type }

Annotation that references an uploaded file.

source: object { filename, type }

File attachment referenced by the annotation.

filename: string

Filename referenced by the annotation.

type: "file"

Type discriminator that is always `file`.

type: "file"

Type discriminator that is always `file` for this annotation.

URL object { source, type }

Annotation that references a URL.

source: object { type, url }

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url`.

url: string

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url` for this annotation.

text: string

Assistant generated text.

type: "output\_text"

Type discriminator that is always `output_text`.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.assistant\_message"

Type discriminator that is always `chatkit.assistant_message`.

ChatKitWidgetItem object { id, created\_at, object, 3 more }

Thread item that renders a widget payload.

id: string

Identifier of the thread item.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.widget"

Type discriminator that is always `chatkit.widget`.

widget: string

Serialized widget payload rendered in the UI.

ChatKitClientToolCall object { id, arguments, call\_id, 7 more }

Record of a client side tool invocation initiated by the assistant.

id: string

Identifier of the thread item.

arguments: string

JSON-encoded arguments that were sent to the tool.

call\_id: string

Identifier for the client tool call.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

name: string

Tool name that was invoked.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

output: string

JSON-encoded output captured from the tool. Defaults to null while execution is in progress.

status: "in\_progress" or "completed"

Execution status for the tool call.

One of the following:

"in\_progress"

"completed"

thread\_id: string

Identifier of the parent thread.

type: "chatkit.client\_tool\_call"

Type discriminator that is always `chatkit.client_tool_call`.

ChatKitTask object { id, created\_at, heading, 5 more }

Task emitted by the workflow to show progress and status updates.

id: string

Identifier of the thread item.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

heading: string

Optional heading for the task. Defaults to null when not provided.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

summary: string

Optional summary that describes the task. Defaults to null when omitted.

task\_type: "custom" or "thought"

Subtype for the task.

One of the following:

"custom"

"thought"

thread\_id: string

Identifier of the parent thread.

type: "chatkit.task"

Type discriminator that is always `chatkit.task`.

ChatKitTaskGroup object { id, created\_at, object, 3 more }

Collection of workflow tasks grouped together in the thread.

id: string

Identifier of the thread item.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

tasks: array of object { heading, summary, type }

Tasks included in the group.

heading: string

Optional heading for the grouped task. Defaults to null when not provided.

summary: string

Optional summary that describes the grouped task. Defaults to null when omitted.

type: "custom" or "thought"

Subtype for the grouped task.

One of the following:

"custom"

"thought"

thread\_id: string

Identifier of the parent thread.

type: "chatkit.task\_group"

Type discriminator that is always `chatkit.task_group`.

first\_id: string

The ID of the first item in the list.

has\_more: boolean

Whether there are more items available.

last\_id: string

The ID of the last item in the list.

object: "list"

The type of object returned, must be `list`.

ChatKitThreadUserMessageItem object { id, attachments, content, 5 more }

User-authored messages within a thread.

id: string

Identifier of the thread item.

attachments: array of [ChatKitAttachment](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_attachment%20%3E%20(schema)) { id, mime\_type, name, 2 more }

Attachments associated with the user message. Defaults to an empty list.

id: string

Identifier for the attachment.

mime\_type: string

MIME type of the attachment.

name: string

Original display name for the attachment.

preview\_url: string

Preview URL for rendering the attachment inline.

type: "image" or "file"

Attachment discriminator.

One of the following:

"image"

"file"

content: array of object { text, type }  or object { text, type }

Ordered content elements supplied by the user.

One of the following:

InputText object { text, type }

Text block that a user contributed to the thread.

text: string

Plain-text content supplied by the user.

type: "input\_text"

Type discriminator that is always `input_text`.

QuotedText object { text, type }

Quoted snippet that the user referenced in their message.

text: string

Quoted text content.

type: "quoted\_text"

Type discriminator that is always `quoted_text`.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

inference\_options: object { model, tool\_choice }

Inference overrides applied to the message. Defaults to null when unset.

model: string

Model name that generated the response. Defaults to null when using the session default.

tool\_choice: object { id }

Preferred tool to invoke. Defaults to null when ChatKit should auto-select.

id: string

Identifier of the requested tool.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.user\_message"

ChatKitWidgetItem object { id, created\_at, object, 3 more }

Thread item that renders a widget payload.

id: string

Identifier of the thread item.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.widget"

Type discriminator that is always `chatkit.widget`.

widget: string

Serialized widget payload rendered in the UI.

ThreadDeleteResponse object { id, deleted, object }

Confirmation payload returned after deleting a thread.

id: string

Identifier of the deleted thread.

deleted: boolean

Indicates that the thread has been deleted.

object: "chatkit.thread.deleted"

Type discriminator that is always `chatkit.thread.deleted`.

---

## Sessions

> Source: https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/sessions
> Fetched: 2026-04-23

###### [Cancel chat session](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/sessions/methods/cancel)

POST/chatkit/sessions/{session\_id}/cancel

###### [Create ChatKit session](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/sessions/methods/create)

POST/chatkit/sessions

---

## Threads

> Source: https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads
> Fetched: 2026-04-23

###### [List ChatKit thread items](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/list_items)

GET/chatkit/threads/{thread\_id}/items

###### [Retrieve ChatKit thread](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/retrieve)

GET/chatkit/threads/{thread\_id}

###### [Delete ChatKit thread](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/delete)

DELETE/chatkit/threads/{thread\_id}

###### [List ChatKit threads](https://developers.openai.com/api/reference/resources/beta/subresources/chatkit/subresources/threads/methods/list)

GET/chatkit/threads

###### Models

ChatSession object { id, chatkit\_configuration, client\_secret, 7 more }

Represents a ChatKit session and its resolved configuration.

id: string

Identifier for the ChatKit session.

chatkit\_configuration: [ChatSessionChatKitConfiguration](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_chatkit_configuration%20%3E%20(schema)) { automatic\_thread\_titling, file\_upload, history }

Resolved ChatKit feature configuration for the session.

client\_secret: string

Ephemeral client secret that authenticates session requests.

expires\_at: number

Unix timestamp (in seconds) for when the session expires.

max\_requests\_per\_1\_minute: number

Convenience copy of the per-minute request limit.

object: "chatkit.session"

Type discriminator that is always `chatkit.session`.

rate\_limits: [ChatSessionRateLimits](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_rate_limits%20%3E%20(schema)) { max\_requests\_per\_1\_minute }

Resolved rate limit values.

status: [ChatSessionStatus](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_status%20%3E%20(schema))

Current lifecycle state of the session.

user: string

User identifier associated with the session.

workflow: [ChatKitWorkflow](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit%20%3E%20(model)%20chatkit_workflow%20%3E%20(schema)) { id, state\_variables, tracing, version }

Workflow metadata for the session.

ChatSessionAutomaticThreadTitling object { enabled }

Automatic thread title preferences for the session.

enabled: boolean

Whether automatic thread titling is enabled.

ChatSessionChatKitConfiguration object { automatic\_thread\_titling, file\_upload, history }

ChatKit configuration for the session.

automatic\_thread\_titling: [ChatSessionAutomaticThreadTitling](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_automatic_thread_titling%20%3E%20(schema)) { enabled }

Automatic thread titling preferences.

file\_upload: [ChatSessionFileUpload](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_file_upload%20%3E%20(schema)) { enabled, max\_file\_size, max\_files }

Upload settings for the session.

history: [ChatSessionHistory](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chat_session_history%20%3E%20(schema)) { enabled, recent\_threads }

History retention configuration.

ChatSessionChatKitConfigurationParam object { automatic\_thread\_titling, file\_upload, history }

Optional per-session configuration settings for ChatKit behavior.

automatic\_thread\_titling: optional object { enabled }

Configuration for automatic thread titling. When omitted, automatic thread titling is enabled by default.

enabled: optional boolean

Enable automatic thread title generation. Defaults to true.

file\_upload: optional object { enabled, max\_file\_size, max\_files }

Configuration for upload enablement and limits. When omitted, uploads are disabled by default (max\_files 10, max\_file\_size 512 MB).

enabled: optional boolean

Enable uploads for this session. Defaults to false.

max\_file\_size: optional number

Maximum size in megabytes for each uploaded file. Defaults to 512 MB, which is the maximum allowable size.

maximum512

minimum1

max\_files: optional number

Maximum number of files that can be uploaded to the session. Defaults to 10.

minimum1

history: optional object { enabled, recent\_threads }

Configuration for chat history retention. When omitted, history is enabled by default with no limit on recent\_threads (null).

enabled: optional boolean

Enables chat users to access previous ChatKit threads. Defaults to true.

recent\_threads: optional number

Number of recent ChatKit threads users have access to. Defaults to unlimited when unset.

minimum1

ChatSessionExpiresAfterParam object { anchor, seconds }

Controls when the session expires relative to an anchor timestamp.

anchor: "created\_at"

Base timestamp used to calculate expiration. Currently fixed to `created_at`.

seconds: number

Number of seconds after the anchor when the session expires.

maximum600

minimum1

ChatSessionFileUpload object { enabled, max\_file\_size, max\_files }

Upload permissions and limits applied to the session.

enabled: boolean

Indicates if uploads are enabled for the session.

max\_file\_size: number

Maximum upload size in megabytes.

max\_files: number

Maximum number of uploads allowed during the session.

ChatSessionHistory object { enabled, recent\_threads }

History retention preferences returned for the session.

enabled: boolean

Indicates if chat history is persisted for the session.

recent\_threads: number

Number of prior threads surfaced in history views. Defaults to null when all history is retained.

ChatSessionRateLimits object { max\_requests\_per\_1\_minute }

Active per-minute request limit for the session.

max\_requests\_per\_1\_minute: number

Maximum allowed requests per one-minute window.

ChatSessionRateLimitsParam object { max\_requests\_per\_1\_minute }

Controls request rate limits for the session.

max\_requests\_per\_1\_minute: optional number

Maximum number of requests allowed per minute for the session. Defaults to 10.

minimum1

ChatSessionStatus = "active" or "expired" or "cancelled"

One of the following:

"active"

"expired"

"cancelled"

ChatSessionWorkflowParam object { id, state\_variables, tracing, version }

Workflow reference and overrides applied to the chat session.

id: string

Identifier for the workflow invoked by the session.

state\_variables: optional map[string or boolean or number]

State variables forwarded to the workflow. Keys may be up to 64 characters, values must be primitive types, and the map defaults to an empty object.

One of the following:

string

boolean

number

tracing: optional object { enabled }

Optional tracing overrides for the workflow invocation. When omitted, tracing is enabled by default.

enabled: optional boolean

Whether tracing is enabled during the session. Defaults to true.

version: optional string

Specific workflow version to run. Defaults to the latest deployed version.

ChatKitAttachment object { id, mime\_type, name, 2 more }

Attachment metadata included on thread items.

id: string

Identifier for the attachment.

mime\_type: string

MIME type of the attachment.

name: string

Original display name for the attachment.

preview\_url: string

Preview URL for rendering the attachment inline.

type: "image" or "file"

Attachment discriminator.

One of the following:

"image"

"file"

ChatKitResponseOutputText object { annotations, text, type }

Assistant response text accompanied by optional annotations.

annotations: array of object { source, type }  or object { source, type }

Ordered list of annotations attached to the response text.

One of the following:

File object { source, type }

Annotation that references an uploaded file.

source: object { filename, type }

File attachment referenced by the annotation.

filename: string

Filename referenced by the annotation.

type: "file"

Type discriminator that is always `file`.

type: "file"

Type discriminator that is always `file` for this annotation.

URL object { source, type }

Annotation that references a URL.

source: object { type, url }

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url`.

url: string

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url` for this annotation.

text: string

Assistant generated text.

type: "output\_text"

Type discriminator that is always `output_text`.

ChatKitThread object { id, created\_at, object, 3 more }

Represents a ChatKit thread and its current status.

id: string

Identifier of the thread.

created\_at: number

Unix timestamp (in seconds) for when the thread was created.

object: "chatkit.thread"

Type discriminator that is always `chatkit.thread`.

status: object { type }  or object { reason, type }  or object { reason, type }

Current status for the thread. Defaults to `active` for newly created threads.

One of the following:

Active object { type }

Indicates that a thread is active.

type: "active"

Status discriminator that is always `active`.

Locked object { reason, type }

Indicates that a thread is locked and cannot accept new input.

reason: string

Reason that the thread was locked. Defaults to null when no reason is recorded.

type: "locked"

Status discriminator that is always `locked`.

Closed object { reason, type }

Indicates that a thread has been closed.

reason: string

Reason that the thread was closed. Defaults to null when no reason is recorded.

type: "closed"

Status discriminator that is always `closed`.

title: string

Optional human-readable title for the thread. Defaults to null when no title has been generated.

user: string

Free-form string that identifies your end user who owns the thread.

ChatKitThreadAssistantMessageItem object { id, content, created\_at, 3 more }

Assistant-authored message within a thread.

id: string

Identifier of the thread item.

content: array of [ChatKitResponseOutputText](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_response_output_text%20%3E%20(schema)) { annotations, text, type }

Ordered assistant response segments.

annotations: array of object { source, type }  or object { source, type }

Ordered list of annotations attached to the response text.

One of the following:

File object { source, type }

Annotation that references an uploaded file.

source: object { filename, type }

File attachment referenced by the annotation.

filename: string

Filename referenced by the annotation.

type: "file"

Type discriminator that is always `file`.

type: "file"

Type discriminator that is always `file` for this annotation.

URL object { source, type }

Annotation that references a URL.

source: object { type, url }

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url`.

url: string

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url` for this annotation.

text: string

Assistant generated text.

type: "output\_text"

Type discriminator that is always `output_text`.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.assistant\_message"

Type discriminator that is always `chatkit.assistant_message`.

ChatKitThreadItemList object { data, first\_id, has\_more, 2 more }

A paginated list of thread items rendered for the ChatKit API.

data: array of [ChatKitThreadUserMessageItem](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_thread_user_message_item%20%3E%20(schema)) { id, attachments, content, 5 more }  or [ChatKitThreadAssistantMessageItem](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_thread_assistant_message_item%20%3E%20(schema)) { id, content, created\_at, 3 more }  or [ChatKitWidgetItem](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_widget_item%20%3E%20(schema)) { id, created\_at, object, 3 more }  or 3 more

A list of items

One of the following:

ChatKitThreadUserMessageItem object { id, attachments, content, 5 more }

User-authored messages within a thread.

id: string

Identifier of the thread item.

attachments: array of [ChatKitAttachment](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_attachment%20%3E%20(schema)) { id, mime\_type, name, 2 more }

Attachments associated with the user message. Defaults to an empty list.

id: string

Identifier for the attachment.

mime\_type: string

MIME type of the attachment.

name: string

Original display name for the attachment.

preview\_url: string

Preview URL for rendering the attachment inline.

type: "image" or "file"

Attachment discriminator.

One of the following:

"image"

"file"

content: array of object { text, type }  or object { text, type }

Ordered content elements supplied by the user.

One of the following:

InputText object { text, type }

Text block that a user contributed to the thread.

text: string

Plain-text content supplied by the user.

type: "input\_text"

Type discriminator that is always `input_text`.

QuotedText object { text, type }

Quoted snippet that the user referenced in their message.

text: string

Quoted text content.

type: "quoted\_text"

Type discriminator that is always `quoted_text`.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

inference\_options: object { model, tool\_choice }

Inference overrides applied to the message. Defaults to null when unset.

model: string

Model name that generated the response. Defaults to null when using the session default.

tool\_choice: object { id }

Preferred tool to invoke. Defaults to null when ChatKit should auto-select.

id: string

Identifier of the requested tool.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.user\_message"

ChatKitThreadAssistantMessageItem object { id, content, created\_at, 3 more }

Assistant-authored message within a thread.

id: string

Identifier of the thread item.

content: array of [ChatKitResponseOutputText](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_response_output_text%20%3E%20(schema)) { annotations, text, type }

Ordered assistant response segments.

annotations: array of object { source, type }  or object { source, type }

Ordered list of annotations attached to the response text.

One of the following:

File object { source, type }

Annotation that references an uploaded file.

source: object { filename, type }

File attachment referenced by the annotation.

filename: string

Filename referenced by the annotation.

type: "file"

Type discriminator that is always `file`.

type: "file"

Type discriminator that is always `file` for this annotation.

URL object { source, type }

Annotation that references a URL.

source: object { type, url }

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url`.

url: string

URL referenced by the annotation.

type: "url"

Type discriminator that is always `url` for this annotation.

text: string

Assistant generated text.

type: "output\_text"

Type discriminator that is always `output_text`.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.assistant\_message"

Type discriminator that is always `chatkit.assistant_message`.

ChatKitWidgetItem object { id, created\_at, object, 3 more }

Thread item that renders a widget payload.

id: string

Identifier of the thread item.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.widget"

Type discriminator that is always `chatkit.widget`.

widget: string

Serialized widget payload rendered in the UI.

ChatKitClientToolCall object { id, arguments, call\_id, 7 more }

Record of a client side tool invocation initiated by the assistant.

id: string

Identifier of the thread item.

arguments: string

JSON-encoded arguments that were sent to the tool.

call\_id: string

Identifier for the client tool call.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

name: string

Tool name that was invoked.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

output: string

JSON-encoded output captured from the tool. Defaults to null while execution is in progress.

status: "in\_progress" or "completed"

Execution status for the tool call.

One of the following:

"in\_progress"

"completed"

thread\_id: string

Identifier of the parent thread.

type: "chatkit.client\_tool\_call"

Type discriminator that is always `chatkit.client_tool_call`.

ChatKitTask object { id, created\_at, heading, 5 more }

Task emitted by the workflow to show progress and status updates.

id: string

Identifier of the thread item.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

heading: string

Optional heading for the task. Defaults to null when not provided.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

summary: string

Optional summary that describes the task. Defaults to null when omitted.

task\_type: "custom" or "thought"

Subtype for the task.

One of the following:

"custom"

"thought"

thread\_id: string

Identifier of the parent thread.

type: "chatkit.task"

Type discriminator that is always `chatkit.task`.

ChatKitTaskGroup object { id, created\_at, object, 3 more }

Collection of workflow tasks grouped together in the thread.

id: string

Identifier of the thread item.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

tasks: array of object { heading, summary, type }

Tasks included in the group.

heading: string

Optional heading for the grouped task. Defaults to null when not provided.

summary: string

Optional summary that describes the grouped task. Defaults to null when omitted.

type: "custom" or "thought"

Subtype for the grouped task.

One of the following:

"custom"

"thought"

thread\_id: string

Identifier of the parent thread.

type: "chatkit.task\_group"

Type discriminator that is always `chatkit.task_group`.

first\_id: string

The ID of the first item in the list.

has\_more: boolean

Whether there are more items available.

last\_id: string

The ID of the last item in the list.

object: "list"

The type of object returned, must be `list`.

ChatKitThreadUserMessageItem object { id, attachments, content, 5 more }

User-authored messages within a thread.

id: string

Identifier of the thread item.

attachments: array of [ChatKitAttachment](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.chatkit.threads%20%3E%20(model)%20chatkit_attachment%20%3E%20(schema)) { id, mime\_type, name, 2 more }

Attachments associated with the user message. Defaults to an empty list.

id: string

Identifier for the attachment.

mime\_type: string

MIME type of the attachment.

name: string

Original display name for the attachment.

preview\_url: string

Preview URL for rendering the attachment inline.

type: "image" or "file"

Attachment discriminator.

One of the following:

"image"

"file"

content: array of object { text, type }  or object { text, type }

Ordered content elements supplied by the user.

One of the following:

InputText object { text, type }

Text block that a user contributed to the thread.

text: string

Plain-text content supplied by the user.

type: "input\_text"

Type discriminator that is always `input_text`.

QuotedText object { text, type }

Quoted snippet that the user referenced in their message.

text: string

Quoted text content.

type: "quoted\_text"

Type discriminator that is always `quoted_text`.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

inference\_options: object { model, tool\_choice }

Inference overrides applied to the message. Defaults to null when unset.

model: string

Model name that generated the response. Defaults to null when using the session default.

tool\_choice: object { id }

Preferred tool to invoke. Defaults to null when ChatKit should auto-select.

id: string

Identifier of the requested tool.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.user\_message"

ChatKitWidgetItem object { id, created\_at, object, 3 more }

Thread item that renders a widget payload.

id: string

Identifier of the thread item.

created\_at: number

Unix timestamp (in seconds) for when the item was created.

object: "chatkit.thread\_item"

Type discriminator that is always `chatkit.thread_item`.

thread\_id: string

Identifier of the parent thread.

type: "chatkit.widget"

Type discriminator that is always `chatkit.widget`.

widget: string

Serialized widget payload rendered in the UI.

ThreadDeleteResponse object { id, deleted, object }

Confirmation payload returned after deleting a thread.

id: string

Identifier of the deleted thread.

deleted: boolean

Indicates that the thread has been deleted.

object: "chatkit.thread.deleted"

Type discriminator that is always `chatkit.thread.deleted`.

---

## Threads

> Source: https://developers.openai.com/api/reference/resources/beta/subresources/threads
> Fetched: 2026-04-23

Build Assistants that can call models and use tools.

###### [Create thread](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/create)

Deprecated

POST/threads

###### [Create thread and run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/create_and_run)

Deprecated

POST/threads/runs

###### [Retrieve thread](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/retrieve)

Deprecated

GET/threads/{thread\_id}

###### [Modify thread](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/update)

Deprecated

POST/threads/{thread\_id}

###### [Delete thread](https://developers.openai.com/api/reference/resources/beta/subresources/threads/methods/delete)

Deprecated

DELETE/threads/{thread\_id}

###### Models

AssistantResponseFormatOption = "auto" or [ResponseFormatText](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_text%20%3E%20(schema)) { type }  or [ResponseFormatJSONObject](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_object%20%3E%20(schema)) { type }  or [ResponseFormatJSONSchema](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)) { json\_schema, type }

Specifies the format that the model must output. Compatible with [GPT-4o](https://developers.openai.com/docs/models#gpt-4o), [GPT-4 Turbo](https://developers.openai.com/docs/models#gpt-4-turbo-and-gpt-4), and all GPT-3.5 Turbo models since `gpt-3.5-turbo-1106`.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured Outputs which ensures the model will match your supplied JSON schema. Learn more in the [Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables JSON mode, which ensures the message the model generates is valid JSON.

**Important:** when using JSON mode, you **must** also instruct the model to produce JSON yourself via a system or user message. Without this, the model may generate an unending stream of whitespace until the generation reaches the token limit, resulting in a long-running and seemingly “stuck” request. Also note that the message content may be partially cut off if `finish_reason="length"`, which indicates the generation exceeded `max_tokens` or the conversation exceeded the max context length.

One of the following:

"auto"

`auto` is the default value

ResponseFormatText object { type }

Default response format. Used to generate text responses.

type: "text"

The type of response format being defined. Always `text`.

ResponseFormatJSONObject object { type }

JSON object response format. An older method of generating JSON responses.
Using `json_schema` is recommended for models that support it. Note that the
model will not generate JSON without a system or user message instructing it
to do so.

type: "json\_object"

The type of response format being defined. Always `json_object`.

ResponseFormatJSONSchema object { json\_schema, type }

JSON Schema response format. Used to generate structured JSON responses.
Learn more about [Structured Outputs](https://developers.openai.com/docs/guides/structured-outputs).

json\_schema: object { name, description, schema, strict }

Structured Outputs configuration options, including a JSON Schema.

name: string

The name of the response format. Must be a-z, A-Z, 0-9, or contain
underscores and dashes, with a maximum length of 64.

description: optional string

A description of what the response format is for, used by the model to
determine how to respond in the format.

schema: optional map[unknown]

The schema for the response format, described as a JSON Schema object.
Learn how to build JSON schemas [here](https://json-schema.org/).

strict: optional boolean

Whether to enable strict schema adherence when generating the output.
If set to true, the model will always follow the exact schema defined
in the `schema` field. Only a subset of JSON Schema is supported when
`strict` is `true`. To learn more, read the [Structured Outputs
guide](https://developers.openai.com/docs/guides/structured-outputs).

type: "json\_schema"

The type of response format being defined. Always `json_schema`.

AssistantToolChoice object { type, function }

Specifies a tool the model should use. Use to force the model to call a specific tool.

type: "function" or "code\_interpreter" or "file\_search"

The type of the tool. If type is `function`, the function name must be set

One of the following:

"function"

"code\_interpreter"

"file\_search"

function: optional [AssistantToolChoiceFunction](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20assistant_tool_choice_function%20%3E%20(schema)) { name }

AssistantToolChoiceFunction object { name }

name: string

The name of the function to call.

AssistantToolChoiceOption = "none" or "auto" or "required" or [AssistantToolChoice](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20assistant_tool_choice%20%3E%20(schema)) { type, function }

Controls which (if any) tool is called by the model.
`none` means the model will not call any tools and instead generates a message.
`auto` is the default value and means the model can pick between generating a message or calling one or more tools.
`required` means the model must call one or more tools before responding to the user.
Specifying a particular tool like `{"type": "file_search"}` or `{"type": "function", "function": {"name": "my_function"}}` forces the model to call that tool.

One of the following:

"none" or "auto" or "required"

`none` means the model will not call any tools and instead generates a message. `auto` means the model can pick between generating a message or calling one or more tools. `required` means the model must call one or more tools before responding to the user.

One of the following:

"none"

"auto"

"required"

AssistantToolChoice object { type, function }

Specifies a tool the model should use. Use to force the model to call a specific tool.

type: "function" or "code\_interpreter" or "file\_search"

The type of the tool. If type is `function`, the function name must be set

One of the following:

"function"

"code\_interpreter"

"file\_search"

function: optional [AssistantToolChoiceFunction](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20assistant_tool_choice_function%20%3E%20(schema)) { name }

Thread object { id, created\_at, metadata, 2 more }

Represents a thread that contains [messages](https://developers.openai.com/docs/api-reference/messages).

id: string

The identifier, which can be referenced in API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the thread was created.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

object: "thread"

The object type, which is always `thread`.

tool\_resources: object { code\_interpreter, file\_search }

A set of resources that are made available to the assistant’s tools in this thread. The resources are specific to the type of tool. For example, the `code_interpreter` tool requires a list of file IDs, while the `file_search` tool requires a list of vector store IDs.

code\_interpreter: optional object { file\_ids }

file\_ids: optional array of string

A list of [file](https://developers.openai.com/docs/api-reference/files) IDs made available to the `code_interpreter` tool. There can be a maximum of 20 files associated with the tool.

file\_search: optional object { vector\_store\_ids }

vector\_store\_ids: optional array of string

The [vector store](https://developers.openai.com/docs/api-reference/vector-stores/object) attached to this thread. There can be a maximum of 1 vector store attached to the thread.

ThreadDeleted object { id, deleted, object }

id: string

deleted: boolean

object: "thread.deleted"

##### ThreadsRuns

Build Assistants that can call models and use tools.

###### [List runs](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/list)

Deprecated

GET/threads/{thread\_id}/runs

###### [Create run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/create)

Deprecated

POST/threads/{thread\_id}/runs

###### [Retrieve run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/retrieve)

Deprecated

GET/threads/{thread\_id}/runs/{run\_id}

###### [Modify run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/update)

Deprecated

POST/threads/{thread\_id}/runs/{run\_id}

###### [Submit tool outputs to run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/submit_tool_outputs)

Deprecated

POST/threads/{thread\_id}/runs/{run\_id}/submit\_tool\_outputs

###### [Cancel a run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/cancel)

Deprecated

POST/threads/{thread\_id}/runs/{run\_id}/cancel

###### Models

RequiredActionFunctionToolCall object { id, function, type }

Tool call objects

id: string

The ID of the tool call. This ID must be referenced when you submit the tool outputs in using the [Submit tool outputs to run](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) endpoint.

function: object { arguments, name }

The function definition.

arguments: string

The arguments that the model expects you to pass to the function.

name: string

The name of the function.

type: "function"

The type of tool call the output is required for. For now, this is always `function`.

Run object { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

id: string

The identifier, which can be referenced in API endpoints.

assistant\_id: string

The ID of the [assistant](https://developers.openai.com/docs/api-reference/assistants) used for execution of this run.

cancelled\_at: number

The Unix timestamp (in seconds) for when the run was cancelled.

completed\_at: number

The Unix timestamp (in seconds) for when the run was completed.

created\_at: number

The Unix timestamp (in seconds) for when the run was created.

expires\_at: number

The Unix timestamp (in seconds) for when the run will expire.

failed\_at: number

The Unix timestamp (in seconds) for when the run failed.

incomplete\_details: object { reason }

Details on why the run is incomplete. Will be `null` if the run is not incomplete.

reason: optional "max\_completion\_tokens" or "max\_prompt\_tokens"

The reason why the run is incomplete. This will point to which specific token limit was reached over the course of the run.

One of the following:

"max\_completion\_tokens"

"max\_prompt\_tokens"

instructions: string

The instructions that the [assistant](https://developers.openai.com/docs/api-reference/assistants) used for this run.

last\_error: object { code, message }

The last error associated with this run. Will be `null` if there are no errors.

code: "server\_error" or "rate\_limit\_exceeded" or "invalid\_prompt"

One of `server_error`, `rate_limit_exceeded`, or `invalid_prompt`.

One of the following:

"server\_error"

"rate\_limit\_exceeded"

"invalid\_prompt"

message: string

A human-readable description of the error.

max\_completion\_tokens: number

The maximum number of completion tokens specified to have been used over the course of the run.

minimum256

max\_prompt\_tokens: number

The maximum number of prompt tokens specified to have been used over the course of the run.

minimum256

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: string

The model that the [assistant](https://developers.openai.com/docs/api-reference/assistants) used for this run.

object: "thread.run"

The object type, which is always `thread.run`.

parallel\_tool\_calls: boolean

Whether to enable [parallel function calling](https://developers.openai.com/docs/guides/function-calling#configuring-parallel-function-calling) during tool use.

required\_action: object { submit\_tool\_outputs, type }

Details on the action required to continue the run. Will be `null` if no action is required.

submit\_tool\_outputs: object { tool\_calls }

Details on the tool outputs needed for this run to continue.

tool\_calls: array of [RequiredActionFunctionToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20required_action_function_tool_call%20%3E%20(schema)) { id, function, type }

A list of the relevant tool calls.

id: string

The ID of the tool call. This ID must be referenced when you submit the tool outputs in using the [Submit tool outputs to run](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) endpoint.

function: object { arguments, name }

The function definition.

arguments: string

The arguments that the model expects you to pass to the function.

name: string

The name of the function.

type: "function"

The type of tool call the output is required for. For now, this is always `function`.

type: "submit\_tool\_outputs"

For now, this is always `submit_tool_outputs`.

response\_format: [AssistantResponseFormatOption](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20assistant_response_format_option%20%3E%20(schema))

Specifies the format that the model must output. Compatible with [GPT-4o](https://developers.openai.com/docs/models#gpt-4o), [GPT-4 Turbo](https://developers.openai.com/docs/models#gpt-4-turbo-and-gpt-4), and all GPT-3.5 Turbo models since `gpt-3.5-turbo-1106`.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured Outputs which ensures the model will match your supplied JSON schema. Learn more in the [Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables JSON mode, which ensures the message the model generates is valid JSON.

**Important:** when using JSON mode, you **must** also instruct the model to produce JSON yourself via a system or user message. Without this, the model may generate an unending stream of whitespace until the generation reaches the token limit, resulting in a long-running and seemingly “stuck” request. Also note that the message content may be partially cut off if `finish_reason="length"`, which indicates the generation exceeded `max_tokens` or the conversation exceeded the max context length.

started\_at: number

The Unix timestamp (in seconds) for when the run was started.

status: "queued" or "in\_progress" or "requires\_action" or 6 more

The status of the run, which can be either `queued`, `in_progress`, `requires_action`, `cancelling`, `cancelled`, `failed`, `completed`, `incomplete`, or `expired`.

One of the following:

"queued"

"in\_progress"

"requires\_action"

"cancelling"

"cancelled"

"failed"

"completed"

"incomplete"

"expired"

thread\_id: string

The ID of the [thread](https://developers.openai.com/docs/api-reference/threads) that was executed on as a part of this run.

tool\_choice: [AssistantToolChoiceOption](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20assistant_tool_choice_option%20%3E%20(schema))

Controls which (if any) tool is called by the model.
`none` means the model will not call any tools and instead generates a message.
`auto` is the default value and means the model can pick between generating a message or calling one or more tools.
`required` means the model must call one or more tools before responding to the user.
Specifying a particular tool like `{"type": "file_search"}` or `{"type": "function", "function": {"name": "my_function"}}` forces the model to call that tool.

tools: array of [CodeInterpreterTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20code_interpreter_tool%20%3E%20(schema)) { type }  or [FileSearchTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20file_search_tool%20%3E%20(schema)) { type, file\_search }  or [FunctionTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20function_tool%20%3E%20(schema)) { function, type }

The list of tools that the [assistant](https://developers.openai.com/docs/api-reference/assistants) used for this run.

One of the following:

CodeInterpreterTool object { type }

type: "code\_interpreter"

The type of tool being defined: `code_interpreter`

FileSearchTool object { type, file\_search }

type: "file\_search"

The type of tool being defined: `file_search`

file\_search: optional object { max\_num\_results, ranking\_options }

Overrides for the file search tool.

max\_num\_results: optional number

The maximum number of results the file search tool should output. The default is 20 for `gpt-4*` models and 5 for `gpt-3.5-turbo`. This number should be between 1 and 50 inclusive.

Note that the file search tool may output fewer than `max_num_results` results. See the [file search tool documentation](https://developers.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings) for more information.

minimum1

maximum50

ranking\_options: optional object { score\_threshold, ranker }

The ranking options for the file search. If not specified, the file search tool will use the `auto` ranker and a score\_threshold of 0.

See the [file search tool documentation](https://developers.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings) for more information.

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

ranker: optional "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

FunctionTool object { function, type }

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of tool being defined: `function`

truncation\_strategy: object { type, last\_messages }

Controls for how a thread will be truncated prior to the run. Use this to control the initial context window of the run.

type: "auto" or "last\_messages"

The truncation strategy to use for the thread. The default is `auto`. If set to `last_messages`, the thread will be truncated to the n most recent messages in the thread. When set to `auto`, messages in the middle of the thread will be dropped to fit the context length of the model, `max_prompt_tokens`.

One of the following:

"auto"

"last\_messages"

last\_messages: optional number

The number of most recent messages from the thread when constructing the context for the run.

minimum1

usage: object { completion\_tokens, prompt\_tokens, total\_tokens }

Usage statistics related to the run. This value will be `null` if the run is not in a terminal state (i.e. `in_progress`, `queued`, etc.).

completion\_tokens: number

Number of completion tokens used over the course of the run.

prompt\_tokens: number

Number of prompt tokens used over the course of the run.

total\_tokens: number

Total number of tokens used (prompt + completion).

temperature: optional number

The sampling temperature used for this run. If not set, defaults to 1.

top\_p: optional number

The nucleus sampling value used for this run. If not set, defaults to 1.

##### ThreadsRunsSteps

Build Assistants that can call models and use tools.

###### [List run steps](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/subresources/steps/methods/list)

Deprecated

GET/threads/{thread\_id}/runs/{run\_id}/steps

###### [Retrieve run step](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/subresources/steps/methods/retrieve)

Deprecated

GET/threads/{thread\_id}/runs/{run\_id}/steps/{step\_id}

###### Models

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

CodeInterpreterToolCall object { id, code\_interpreter, type }

Details of the Code Interpreter tool call the run step was involved in.

id: string

The ID of the tool call.

code\_interpreter: object { input, outputs }

The Code Interpreter tool call definition.

input: string

The input to the Code Interpreter tool call.

outputs: array of object { logs, type }  or object { image, type }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogOutput object { logs, type }

Text output from the Code Interpreter tool call as part of a run step.

logs: string

The text output from the Code Interpreter tool call.

type: "logs"

Always `logs`.

CodeInterpreterImageOutput object { image, type }

image: object { file\_id }

file\_id: string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

type: "image"

Always `image`.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

CodeInterpreterToolCallDelta object { index, type, id, code\_interpreter }

Details of the Code Interpreter tool call the run step was involved in.

index: number

The index of the tool call in the tool calls array.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

id: optional string

The ID of the tool call.

code\_interpreter: optional object { input, outputs }

The Code Interpreter tool call definition.

input: optional string

The input to the Code Interpreter tool call.

outputs: optional array of [CodeInterpreterLogs](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_logs%20%3E%20(schema)) { index, type, logs }  or [CodeInterpreterOutputImage](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_output_image%20%3E%20(schema)) { index, type, image }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

FileSearchToolCall object { id, file\_search, type }

id: string

The ID of the tool call object.

file\_search: object { ranking\_options, results }

For now, this is always going to be an empty object.

ranking\_options: optional object { ranker, score\_threshold }

The ranking options for the file search.

ranker: "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

results: optional array of object { file\_id, file\_name, score, content }

The results of the file search.

file\_id: string

The ID of the file that result was found in.

file\_name: string

The name of the file that result was found in.

score: number

The score of the result. All values must be a floating point number between 0 and 1.

minimum0

maximum1

content: optional array of object { text, type }

The content of the result that was found. The content is only included if requested via the include query parameter.

text: optional string

The text content of the file.

type: optional "text"

The type of the content.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

FileSearchToolCallDelta object { file\_search, index, type, id }

file\_search: unknown

For now, this is always going to be an empty object.

index: number

The index of the tool call in the tool calls array.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

id: optional string

The ID of the tool call object.

FunctionToolCall object { id, function, type }

id: string

The ID of the tool call object.

function: object { arguments, name, output }

The definition of the function that was called.

arguments: string

The arguments passed to the function.

name: string

The name of the function.

output: string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

FunctionToolCallDelta object { index, type, id, function }

index: number

The index of the tool call in the tool calls array.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

id: optional string

The ID of the tool call object.

function: optional object { arguments, name, output }

The definition of the function that was called.

arguments: optional string

The arguments passed to the function.

name: optional string

The name of the function.

output: optional string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

MessageCreationStepDetails object { message\_creation, type }

Details of the message creation by the run step.

message\_creation: object { message\_id }

message\_id: string

The ID of the message that was created by this run step.

type: "message\_creation"

Always `message_creation`.

RunStep object { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

id: string

The identifier of the run step, which can be referenced in API endpoints.

assistant\_id: string

The ID of the [assistant](https://developers.openai.com/docs/api-reference/assistants) associated with the run step.

cancelled\_at: number

The Unix timestamp (in seconds) for when the run step was cancelled.

completed\_at: number

The Unix timestamp (in seconds) for when the run step completed.

created\_at: number

The Unix timestamp (in seconds) for when the run step was created.

expired\_at: number

The Unix timestamp (in seconds) for when the run step expired. A step is considered expired if the parent run is expired.

failed\_at: number

The Unix timestamp (in seconds) for when the run step failed.

last\_error: object { code, message }

The last error associated with this run step. Will be `null` if there are no errors.

code: "server\_error" or "rate\_limit\_exceeded"

One of `server_error` or `rate_limit_exceeded`.

One of the following:

"server\_error"

"rate\_limit\_exceeded"

message: string

A human-readable description of the error.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

object: "thread.run.step"

The object type, which is always `thread.run.step`.

run\_id: string

The ID of the [run](https://developers.openai.com/docs/api-reference/runs) that this run step is a part of.

status: "in\_progress" or "cancelled" or "failed" or 2 more

The status of the run step, which can be either `in_progress`, `cancelled`, `failed`, `completed`, or `expired`.

One of the following:

"in\_progress"

"cancelled"

"failed"

"completed"

"expired"

step\_details: [MessageCreationStepDetails](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20message_creation_step_details%20%3E%20(schema)) { message\_creation, type }  or [ToolCallsStepDetails](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20tool_calls_step_details%20%3E%20(schema)) { tool\_calls, type }

The details of the run step.

One of the following:

MessageCreationStepDetails object { message\_creation, type }

Details of the message creation by the run step.

message\_creation: object { message\_id }

message\_id: string

The ID of the message that was created by this run step.

type: "message\_creation"

Always `message_creation`.

ToolCallsStepDetails object { tool\_calls, type }

Details of the tool call.

tool\_calls: array of [CodeInterpreterToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call%20%3E%20(schema)) { id, code\_interpreter, type }  or [FileSearchToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call%20%3E%20(schema)) { id, file\_search, type }  or [FunctionToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call%20%3E%20(schema)) { id, function, type }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCall object { id, code\_interpreter, type }

Details of the Code Interpreter tool call the run step was involved in.

id: string

The ID of the tool call.

code\_interpreter: object { input, outputs }

The Code Interpreter tool call definition.

input: string

The input to the Code Interpreter tool call.

outputs: array of object { logs, type }  or object { image, type }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogOutput object { logs, type }

Text output from the Code Interpreter tool call as part of a run step.

logs: string

The text output from the Code Interpreter tool call.

type: "logs"

Always `logs`.

CodeInterpreterImageOutput object { image, type }

image: object { file\_id }

file\_id: string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

type: "image"

Always `image`.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

FileSearchToolCall object { id, file\_search, type }

id: string

The ID of the tool call object.

file\_search: object { ranking\_options, results }

For now, this is always going to be an empty object.

ranking\_options: optional object { ranker, score\_threshold }

The ranking options for the file search.

ranker: "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

results: optional array of object { file\_id, file\_name, score, content }

The results of the file search.

file\_id: string

The ID of the file that result was found in.

file\_name: string

The name of the file that result was found in.

score: number

The score of the result. All values must be a floating point number between 0 and 1.

minimum0

maximum1

content: optional array of object { text, type }

The content of the result that was found. The content is only included if requested via the include query parameter.

text: optional string

The text content of the file.

type: optional "text"

The type of the content.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

FunctionToolCall object { id, function, type }

id: string

The ID of the tool call object.

function: object { arguments, name, output }

The definition of the function that was called.

arguments: string

The arguments passed to the function.

name: string

The name of the function.

output: string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

type: "tool\_calls"

Always `tool_calls`.

thread\_id: string

The ID of the [thread](https://developers.openai.com/docs/api-reference/threads) that was run.

type: "message\_creation" or "tool\_calls"

The type of run step, which can be either `message_creation` or `tool_calls`.

One of the following:

"message\_creation"

"tool\_calls"

usage: object { completion\_tokens, prompt\_tokens, total\_tokens }

Usage statistics related to the run step. This value will be `null` while the run step’s status is `in_progress`.

completion\_tokens: number

Number of completion tokens used over the course of the run step.

prompt\_tokens: number

Number of prompt tokens used over the course of the run step.

total\_tokens: number

Total number of tokens used (prompt + completion).

RunStepDeltaEvent object { id, delta, object }

Represents a run step delta i.e. any changed fields on a run step during streaming.

id: string

The identifier of the run step, which can be referenced in API endpoints.

delta: object { step\_details }

The delta containing the fields that have changed on the run step.

step\_details: optional [RunStepDeltaMessageDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step_delta_message_delta%20%3E%20(schema)) { type, message\_creation }  or [ToolCallDeltaObject](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20tool_call_delta_object%20%3E%20(schema)) { type, tool\_calls }

The details of the run step.

One of the following:

RunStepDeltaMessageDelta object { type, message\_creation }

Details of the message creation by the run step.

type: "message\_creation"

Always `message_creation`.

message\_creation: optional object { message\_id }

message\_id: optional string

The ID of the message that was created by this run step.

ToolCallDeltaObject object { type, tool\_calls }

Details of the tool call.

type: "tool\_calls"

Always `tool_calls`.

tool\_calls: optional array of [CodeInterpreterToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call_delta%20%3E%20(schema)) { index, type, id, code\_interpreter }  or [FileSearchToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call_delta%20%3E%20(schema)) { file\_search, index, type, id }  or [FunctionToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call_delta%20%3E%20(schema)) { index, type, id, function }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCallDelta object { index, type, id, code\_interpreter }

Details of the Code Interpreter tool call the run step was involved in.

index: number

The index of the tool call in the tool calls array.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

id: optional string

The ID of the tool call.

code\_interpreter: optional object { input, outputs }

The Code Interpreter tool call definition.

input: optional string

The input to the Code Interpreter tool call.

outputs: optional array of [CodeInterpreterLogs](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_logs%20%3E%20(schema)) { index, type, logs }  or [CodeInterpreterOutputImage](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_output_image%20%3E%20(schema)) { index, type, image }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

FileSearchToolCallDelta object { file\_search, index, type, id }

file\_search: unknown

For now, this is always going to be an empty object.

index: number

The index of the tool call in the tool calls array.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

id: optional string

The ID of the tool call object.

FunctionToolCallDelta object { index, type, id, function }

index: number

The index of the tool call in the tool calls array.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

id: optional string

The ID of the tool call object.

function: optional object { arguments, name, output }

The definition of the function that was called.

arguments: optional string

The arguments passed to the function.

name: optional string

The name of the function.

output: optional string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

object: "thread.run.step.delta"

The object type, which is always `thread.run.step.delta`.

RunStepDeltaMessageDelta object { type, message\_creation }

Details of the message creation by the run step.

type: "message\_creation"

Always `message_creation`.

message\_creation: optional object { message\_id }

message\_id: optional string

The ID of the message that was created by this run step.

RunStepInclude = "step\_details.tool\_calls[\*].file\_search.results[\*].content"

ToolCallDeltaObject object { type, tool\_calls }

Details of the tool call.

type: "tool\_calls"

Always `tool_calls`.

tool\_calls: optional array of [CodeInterpreterToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call_delta%20%3E%20(schema)) { index, type, id, code\_interpreter }  or [FileSearchToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call_delta%20%3E%20(schema)) { file\_search, index, type, id }  or [FunctionToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call_delta%20%3E%20(schema)) { index, type, id, function }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCallDelta object { index, type, id, code\_interpreter }

Details of the Code Interpreter tool call the run step was involved in.

index: number

The index of the tool call in the tool calls array.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

id: optional string

The ID of the tool call.

code\_interpreter: optional object { input, outputs }

The Code Interpreter tool call definition.

input: optional string

The input to the Code Interpreter tool call.

outputs: optional array of [CodeInterpreterLogs](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_logs%20%3E%20(schema)) { index, type, logs }  or [CodeInterpreterOutputImage](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_output_image%20%3E%20(schema)) { index, type, image }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

FileSearchToolCallDelta object { file\_search, index, type, id }

file\_search: unknown

For now, this is always going to be an empty object.

index: number

The index of the tool call in the tool calls array.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

id: optional string

The ID of the tool call object.

FunctionToolCallDelta object { index, type, id, function }

index: number

The index of the tool call in the tool calls array.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

id: optional string

The ID of the tool call object.

function: optional object { arguments, name, output }

The definition of the function that was called.

arguments: optional string

The arguments passed to the function.

name: optional string

The name of the function.

output: optional string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

ToolCallsStepDetails object { tool\_calls, type }

Details of the tool call.

tool\_calls: array of [CodeInterpreterToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call%20%3E%20(schema)) { id, code\_interpreter, type }  or [FileSearchToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call%20%3E%20(schema)) { id, file\_search, type }  or [FunctionToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call%20%3E%20(schema)) { id, function, type }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCall object { id, code\_interpreter, type }

Details of the Code Interpreter tool call the run step was involved in.

id: string

The ID of the tool call.

code\_interpreter: object { input, outputs }

The Code Interpreter tool call definition.

input: string

The input to the Code Interpreter tool call.

outputs: array of object { logs, type }  or object { image, type }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogOutput object { logs, type }

Text output from the Code Interpreter tool call as part of a run step.

logs: string

The text output from the Code Interpreter tool call.

type: "logs"

Always `logs`.

CodeInterpreterImageOutput object { image, type }

image: object { file\_id }

file\_id: string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

type: "image"

Always `image`.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

FileSearchToolCall object { id, file\_search, type }

id: string

The ID of the tool call object.

file\_search: object { ranking\_options, results }

For now, this is always going to be an empty object.

ranking\_options: optional object { ranker, score\_threshold }

The ranking options for the file search.

ranker: "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

results: optional array of object { file\_id, file\_name, score, content }

The results of the file search.

file\_id: string

The ID of the file that result was found in.

file\_name: string

The name of the file that result was found in.

score: number

The score of the result. All values must be a floating point number between 0 and 1.

minimum0

maximum1

content: optional array of object { text, type }

The content of the result that was found. The content is only included if requested via the include query parameter.

text: optional string

The text content of the file.

type: optional "text"

The type of the content.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

FunctionToolCall object { id, function, type }

id: string

The ID of the tool call object.

function: object { arguments, name, output }

The definition of the function that was called.

arguments: string

The arguments passed to the function.

name: string

The name of the function.

output: string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

type: "tool\_calls"

Always `tool_calls`.

##### ThreadsMessages

Build Assistants that can call models and use tools.

###### [List messages](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/list)

Deprecated

GET/threads/{thread\_id}/messages

###### [Create message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/create)

Deprecated

POST/threads/{thread\_id}/messages

###### [Modify message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/update)

Deprecated

POST/threads/{thread\_id}/messages/{message\_id}

###### [Retrieve message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/retrieve)

Deprecated

GET/threads/{thread\_id}/messages/{message\_id}

###### [Delete message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/delete)

Deprecated

DELETE/threads/{thread\_id}/messages/{message\_id}

###### Models

FileCitationAnnotation object { end\_index, file\_citation, start\_index, 2 more }

A citation within the message that points to a specific quote from a specific File associated with the assistant or the message. Generated when the assistant uses the “file\_search” tool to search files.

end\_index: number

minimum0

file\_citation: object { file\_id }

file\_id: string

The ID of the specific File the citation is from.

start\_index: number

minimum0

text: string

The text in the message content that needs to be replaced.

type: "file\_citation"

Always `file_citation`.

FileCitationDeltaAnnotation object { index, type, end\_index, 3 more }

A citation within the message that points to a specific quote from a specific File associated with the assistant or the message. Generated when the assistant uses the “file\_search” tool to search files.

index: number

The index of the annotation in the text content part.

type: "file\_citation"

Always `file_citation`.

end\_index: optional number

minimum0

file\_citation: optional object { file\_id, quote }

file\_id: optional string

The ID of the specific File the citation is from.

quote: optional string

The specific quote in the file.

start\_index: optional number

minimum0

text: optional string

The text in the message content that needs to be replaced.

FilePathAnnotation object { end\_index, file\_path, start\_index, 2 more }

A URL for the file that’s generated when the assistant used the `code_interpreter` tool to generate a file.

end\_index: number

minimum0

file\_path: object { file\_id }

file\_id: string

The ID of the file that was generated.

start\_index: number

minimum0

text: string

The text in the message content that needs to be replaced.

type: "file\_path"

Always `file_path`.

FilePathDeltaAnnotation object { index, type, end\_index, 3 more }

A URL for the file that’s generated when the assistant used the `code_interpreter` tool to generate a file.

index: number

The index of the annotation in the text content part.

type: "file\_path"

Always `file_path`.

end\_index: optional number

minimum0

file\_path: optional object { file\_id }

file\_id: optional string

The ID of the file that was generated.

start\_index: optional number

minimum0

text: optional string

The text in the message content that needs to be replaced.

ImageFile object { file\_id, detail }

file\_id: string

The [File](https://developers.openai.com/docs/api-reference/files) ID of the image in the message content. Set `purpose="vision"` when uploading the File if you need to later display the file content.

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image if specified by the user. `low` uses fewer tokens, you can opt in to high resolution using `high`.

One of the following:

"auto"

"low"

"high"

ImageFileContentBlock object { image\_file, type }

References an image [File](https://developers.openai.com/docs/api-reference/files) in the content of a message.

image\_file: [ImageFile](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file%20%3E%20(schema)) { file\_id, detail }

type: "image\_file"

Always `image_file`.

ImageFileDelta object { detail, file\_id }

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image if specified by the user. `low` uses fewer tokens, you can opt in to high resolution using `high`.

One of the following:

"auto"

"low"

"high"

file\_id: optional string

The [File](https://developers.openai.com/docs/api-reference/files) ID of the image in the message content. Set `purpose="vision"` when uploading the File if you need to later display the file content.

ImageFileDeltaBlock object { index, type, image\_file }

References an image [File](https://developers.openai.com/docs/api-reference/files) in the content of a message.

index: number

The index of the content part in the message.

type: "image\_file"

Always `image_file`.

image\_file: optional [ImageFileDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file_delta%20%3E%20(schema)) { detail, file\_id }

ImageURL object { url, detail }

url: string

The external URL of the image, must be a supported image types: jpeg, jpg, png, gif, webp.

formaturi

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image. `low` uses fewer tokens, you can opt in to high resolution using `high`. Default value is `auto`

One of the following:

"auto"

"low"

"high"

ImageURLContentBlock object { image\_url, type }

References an image URL in the content of a message.

image\_url: [ImageURL](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url%20%3E%20(schema)) { url, detail }

type: "image\_url"

The type of the content part.

ImageURLDelta object { detail, url }

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image. `low` uses fewer tokens, you can opt in to high resolution using `high`.

One of the following:

"auto"

"low"

"high"

url: optional string

The URL of the image, must be a supported image types: jpeg, jpg, png, gif, webp.

ImageURLDeltaBlock object { index, type, image\_url }

References an image URL in the content of a message.

index: number

The index of the content part in the message.

type: "image\_url"

Always `image_url`.

image\_url: optional [ImageURLDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url_delta%20%3E%20(schema)) { detail, url }

Message object { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

id: string

The identifier, which can be referenced in API endpoints.

assistant\_id: string

If applicable, the ID of the [assistant](https://developers.openai.com/docs/api-reference/assistants) that authored this message.

attachments: array of object { file\_id, tools }

A list of files attached to the message, and the tools they were added to.

file\_id: optional string

The ID of the file to attach to the message.

tools: optional array of [CodeInterpreterTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20code_interpreter_tool%20%3E%20(schema)) { type }  or object { type }

The tools to add this file to.

One of the following:

CodeInterpreterTool object { type }

type: "code\_interpreter"

The type of tool being defined: `code_interpreter`

FileSearchTool object { type }

type: "file\_search"

The type of tool being defined: `file_search`

completed\_at: number

The Unix timestamp (in seconds) for when the message was completed.

content: array of [ImageFileContentBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file_content_block%20%3E%20(schema)) { image\_file, type }  or [ImageURLContentBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url_content_block%20%3E%20(schema)) { image\_url, type }  or [TextContentBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text_content_block%20%3E%20(schema)) { text, type }  or [RefusalContentBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20refusal_content_block%20%3E%20(schema)) { refusal, type }

The content of the message in array of text and/or images.

One of the following:

ImageFileContentBlock object { image\_file, type }

References an image [File](https://developers.openai.com/docs/api-reference/files) in the content of a message.

image\_file: [ImageFile](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file%20%3E%20(schema)) { file\_id, detail }

type: "image\_file"

Always `image_file`.

ImageURLContentBlock object { image\_url, type }

References an image URL in the content of a message.

image\_url: [ImageURL](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url%20%3E%20(schema)) { url, detail }

type: "image\_url"

The type of the content part.

TextContentBlock object { text, type }

The text content that is part of a message.

text: [Text](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text%20%3E%20(schema)) { annotations, value }

type: "text"

Always `text`.

RefusalContentBlock object { refusal, type }

The refusal content generated by the assistant.

refusal: string

type: "refusal"

Always `refusal`.

created\_at: number

The Unix timestamp (in seconds) for when the message was created.

incomplete\_at: number

The Unix timestamp (in seconds) for when the message was marked as incomplete.

incomplete\_details: object { reason }

On an incomplete message, details about why the message is incomplete.

reason: "content\_filter" or "max\_tokens" or "run\_cancelled" or 2 more

The reason the message is incomplete.

One of the following:

"content\_filter"

"max\_tokens"

"run\_cancelled"

"run\_expired"

"run\_failed"

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

object: "thread.message"

The object type, which is always `thread.message`.

role: "user" or "assistant"

The entity that produced the message. One of `user` or `assistant`.

One of the following:

"user"

"assistant"

run\_id: string

The ID of the [run](https://developers.openai.com/docs/api-reference/runs) associated with the creation of this message. Value is `null` when messages are created manually using the create message or create thread endpoints.

status: "in\_progress" or "incomplete" or "completed"

The status of the message, which can be either `in_progress`, `incomplete`, or `completed`.

One of the following:

"in\_progress"

"incomplete"

"completed"

thread\_id: string

The [thread](https://developers.openai.com/docs/api-reference/threads) ID that this message belongs to.

MessageDeleted object { id, deleted, object }

id: string

deleted: boolean

object: "thread.message.deleted"

MessageDelta object { content, role }

The delta containing the fields that have changed on the Message.

content: optional array of [ImageFileDeltaBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file_delta_block%20%3E%20(schema)) { index, type, image\_file }  or [TextDeltaBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text_delta_block%20%3E%20(schema)) { index, type, text }  or [RefusalDeltaBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20refusal_delta_block%20%3E%20(schema)) { index, type, refusal }  or [ImageURLDeltaBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url_delta_block%20%3E%20(schema)) { index, type, image\_url }

The content of the message in array of text and/or images.

One of the following:

ImageFileDeltaBlock object { index, type, image\_file }

References an image [File](https://developers.openai.com/docs/api-reference/files) in the content of a message.

index: number

The index of the content part in the message.

type: "image\_file"

Always `image_file`.

image\_file: optional [ImageFileDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file_delta%20%3E%20(schema)) { detail, file\_id }

TextDeltaBlock object { index, type, text }

The text content that is part of a message.

index: number

The index of the content part in the message.

type: "text"

Always `text`.

text: optional [TextDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text_delta%20%3E%20(schema)) { annotations, value }

RefusalDeltaBlock object { index, type, refusal }

The refusal content that is part of a message.

index: number

The index of the refusal part in the message.

type: "refusal"

Always `refusal`.

refusal: optional string

ImageURLDeltaBlock object { index, type, image\_url }

References an image URL in the content of a message.

index: number

The index of the content part in the message.

type: "image\_url"

Always `image_url`.

image\_url: optional [ImageURLDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url_delta%20%3E%20(schema)) { detail, url }

role: optional "user" or "assistant"

The entity that produced the message. One of `user` or `assistant`.

One of the following:

"user"

"assistant"

MessageDeltaEvent object { id, delta, object }

Represents a message delta i.e. any changed fields on a message during streaming.

id: string

The identifier of the message, which can be referenced in API endpoints.

delta: [MessageDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message_delta%20%3E%20(schema)) { content, role }

The delta containing the fields that have changed on the Message.

object: "thread.message.delta"

The object type, which is always `thread.message.delta`.

RefusalContentBlock object { refusal, type }

The refusal content generated by the assistant.

refusal: string

type: "refusal"

Always `refusal`.

RefusalDeltaBlock object { index, type, refusal }

The refusal content that is part of a message.

index: number

The index of the refusal part in the message.

type: "refusal"

Always `refusal`.

refusal: optional string

Text object { annotations, value }

annotations: array of [FileCitationAnnotation](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20file_citation_annotation%20%3E%20(schema)) { end\_index, file\_citation, start\_index, 2 more }  or [FilePathAnnotation](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20file_path_annotation%20%3E%20(schema)) { end\_index, file\_path, start\_index, 2 more }

One of the following:

FileCitationAnnotation object { end\_index, file\_citation, start\_index, 2 more }

A citation within the message that points to a specific quote from a specific File associated with the assistant or the message. Generated when the assistant uses the “file\_search” tool to search files.

end\_index: number

minimum0

file\_citation: object { file\_id }

file\_id: string

The ID of the specific File the citation is from.

start\_index: number

minimum0

text: string

The text in the message content that needs to be replaced.

type: "file\_citation"

Always `file_citation`.

FilePathAnnotation object { end\_index, file\_path, start\_index, 2 more }

A URL for the file that’s generated when the assistant used the `code_interpreter` tool to generate a file.

end\_index: number

minimum0

file\_path: object { file\_id }

file\_id: string

The ID of the file that was generated.

start\_index: number

minimum0

text: string

The text in the message content that needs to be replaced.

type: "file\_path"

Always `file_path`.

value: string

The data that makes up the text.

TextContentBlock object { text, type }

The text content that is part of a message.

text: [Text](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text%20%3E%20(schema)) { annotations, value }

type: "text"

Always `text`.

TextContentBlockParam object { text, type }

The text content that is part of a message.

text: string

Text content to be sent to the model

type: "text"

Always `text`.

TextDelta object { annotations, value }

annotations: optional array of [FileCitationDeltaAnnotation](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20file_citation_delta_annotation%20%3E%20(schema)) { index, type, end\_index, 3 more }  or [FilePathDeltaAnnotation](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20file_path_delta_annotation%20%3E%20(schema)) { index, type, end\_index, 3 more }

One of the following:

FileCitationDeltaAnnotation object { index, type, end\_index, 3 more }

A citation within the message that points to a specific quote from a specific File associated with the assistant or the message. Generated when the assistant uses the “file\_search” tool to search files.

index: number

The index of the annotation in the text content part.

type: "file\_citation"

Always `file_citation`.

end\_index: optional number

minimum0

file\_citation: optional object { file\_id, quote }

file\_id: optional string

The ID of the specific File the citation is from.

quote: optional string

The specific quote in the file.

start\_index: optional number

minimum0

text: optional string

The text in the message content that needs to be replaced.

FilePathDeltaAnnotation object { index, type, end\_index, 3 more }

A URL for the file that’s generated when the assistant used the `code_interpreter` tool to generate a file.

index: number

The index of the annotation in the text content part.

type: "file\_path"

Always `file_path`.

end\_index: optional number

minimum0

file\_path: optional object { file\_id }

file\_id: optional string

The ID of the file that was generated.

start\_index: optional number

minimum0

text: optional string

The text in the message content that needs to be replaced.

value: optional string

The data that makes up the text.

TextDeltaBlock object { index, type, text }

The text content that is part of a message.

index: number

The index of the content part in the message.

type: "text"

Always `text`.

text: optional [TextDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text_delta%20%3E%20(schema)) { annotations, value }

---

## Messages

> Source: https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages
> Fetched: 2026-04-23

Build Assistants that can call models and use tools.

###### [List messages](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/list)

Deprecated

GET/threads/{thread\_id}/messages

###### [Create message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/create)

Deprecated

POST/threads/{thread\_id}/messages

###### [Modify message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/update)

Deprecated

POST/threads/{thread\_id}/messages/{message\_id}

###### [Retrieve message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/retrieve)

Deprecated

GET/threads/{thread\_id}/messages/{message\_id}

###### [Delete message](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/messages/methods/delete)

Deprecated

DELETE/threads/{thread\_id}/messages/{message\_id}

###### Models

FileCitationAnnotation object { end\_index, file\_citation, start\_index, 2 more }

A citation within the message that points to a specific quote from a specific File associated with the assistant or the message. Generated when the assistant uses the “file\_search” tool to search files.

end\_index: number

minimum0

file\_citation: object { file\_id }

file\_id: string

The ID of the specific File the citation is from.

start\_index: number

minimum0

text: string

The text in the message content that needs to be replaced.

type: "file\_citation"

Always `file_citation`.

FileCitationDeltaAnnotation object { index, type, end\_index, 3 more }

A citation within the message that points to a specific quote from a specific File associated with the assistant or the message. Generated when the assistant uses the “file\_search” tool to search files.

index: number

The index of the annotation in the text content part.

type: "file\_citation"

Always `file_citation`.

end\_index: optional number

minimum0

file\_citation: optional object { file\_id, quote }

file\_id: optional string

The ID of the specific File the citation is from.

quote: optional string

The specific quote in the file.

start\_index: optional number

minimum0

text: optional string

The text in the message content that needs to be replaced.

FilePathAnnotation object { end\_index, file\_path, start\_index, 2 more }

A URL for the file that’s generated when the assistant used the `code_interpreter` tool to generate a file.

end\_index: number

minimum0

file\_path: object { file\_id }

file\_id: string

The ID of the file that was generated.

start\_index: number

minimum0

text: string

The text in the message content that needs to be replaced.

type: "file\_path"

Always `file_path`.

FilePathDeltaAnnotation object { index, type, end\_index, 3 more }

A URL for the file that’s generated when the assistant used the `code_interpreter` tool to generate a file.

index: number

The index of the annotation in the text content part.

type: "file\_path"

Always `file_path`.

end\_index: optional number

minimum0

file\_path: optional object { file\_id }

file\_id: optional string

The ID of the file that was generated.

start\_index: optional number

minimum0

text: optional string

The text in the message content that needs to be replaced.

ImageFile object { file\_id, detail }

file\_id: string

The [File](https://developers.openai.com/docs/api-reference/files) ID of the image in the message content. Set `purpose="vision"` when uploading the File if you need to later display the file content.

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image if specified by the user. `low` uses fewer tokens, you can opt in to high resolution using `high`.

One of the following:

"auto"

"low"

"high"

ImageFileContentBlock object { image\_file, type }

References an image [File](https://developers.openai.com/docs/api-reference/files) in the content of a message.

image\_file: [ImageFile](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file%20%3E%20(schema)) { file\_id, detail }

type: "image\_file"

Always `image_file`.

ImageFileDelta object { detail, file\_id }

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image if specified by the user. `low` uses fewer tokens, you can opt in to high resolution using `high`.

One of the following:

"auto"

"low"

"high"

file\_id: optional string

The [File](https://developers.openai.com/docs/api-reference/files) ID of the image in the message content. Set `purpose="vision"` when uploading the File if you need to later display the file content.

ImageFileDeltaBlock object { index, type, image\_file }

References an image [File](https://developers.openai.com/docs/api-reference/files) in the content of a message.

index: number

The index of the content part in the message.

type: "image\_file"

Always `image_file`.

image\_file: optional [ImageFileDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file_delta%20%3E%20(schema)) { detail, file\_id }

ImageURL object { url, detail }

url: string

The external URL of the image, must be a supported image types: jpeg, jpg, png, gif, webp.

formaturi

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image. `low` uses fewer tokens, you can opt in to high resolution using `high`. Default value is `auto`

One of the following:

"auto"

"low"

"high"

ImageURLContentBlock object { image\_url, type }

References an image URL in the content of a message.

image\_url: [ImageURL](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url%20%3E%20(schema)) { url, detail }

type: "image\_url"

The type of the content part.

ImageURLDelta object { detail, url }

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image. `low` uses fewer tokens, you can opt in to high resolution using `high`.

One of the following:

"auto"

"low"

"high"

url: optional string

The URL of the image, must be a supported image types: jpeg, jpg, png, gif, webp.

ImageURLDeltaBlock object { index, type, image\_url }

References an image URL in the content of a message.

index: number

The index of the content part in the message.

type: "image\_url"

Always `image_url`.

image\_url: optional [ImageURLDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url_delta%20%3E%20(schema)) { detail, url }

Message object { id, assistant\_id, attachments, 11 more }

Represents a message within a [thread](https://developers.openai.com/docs/api-reference/threads).

id: string

The identifier, which can be referenced in API endpoints.

assistant\_id: string

If applicable, the ID of the [assistant](https://developers.openai.com/docs/api-reference/assistants) that authored this message.

attachments: array of object { file\_id, tools }

A list of files attached to the message, and the tools they were added to.

file\_id: optional string

The ID of the file to attach to the message.

tools: optional array of [CodeInterpreterTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20code_interpreter_tool%20%3E%20(schema)) { type }  or object { type }

The tools to add this file to.

One of the following:

CodeInterpreterTool object { type }

type: "code\_interpreter"

The type of tool being defined: `code_interpreter`

FileSearchTool object { type }

type: "file\_search"

The type of tool being defined: `file_search`

completed\_at: number

The Unix timestamp (in seconds) for when the message was completed.

content: array of [ImageFileContentBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file_content_block%20%3E%20(schema)) { image\_file, type }  or [ImageURLContentBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url_content_block%20%3E%20(schema)) { image\_url, type }  or [TextContentBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text_content_block%20%3E%20(schema)) { text, type }  or [RefusalContentBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20refusal_content_block%20%3E%20(schema)) { refusal, type }

The content of the message in array of text and/or images.

One of the following:

ImageFileContentBlock object { image\_file, type }

References an image [File](https://developers.openai.com/docs/api-reference/files) in the content of a message.

image\_file: [ImageFile](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file%20%3E%20(schema)) { file\_id, detail }

type: "image\_file"

Always `image_file`.

ImageURLContentBlock object { image\_url, type }

References an image URL in the content of a message.

image\_url: [ImageURL](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url%20%3E%20(schema)) { url, detail }

type: "image\_url"

The type of the content part.

TextContentBlock object { text, type }

The text content that is part of a message.

text: [Text](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text%20%3E%20(schema)) { annotations, value }

type: "text"

Always `text`.

RefusalContentBlock object { refusal, type }

The refusal content generated by the assistant.

refusal: string

type: "refusal"

Always `refusal`.

created\_at: number

The Unix timestamp (in seconds) for when the message was created.

incomplete\_at: number

The Unix timestamp (in seconds) for when the message was marked as incomplete.

incomplete\_details: object { reason }

On an incomplete message, details about why the message is incomplete.

reason: "content\_filter" or "max\_tokens" or "run\_cancelled" or 2 more

The reason the message is incomplete.

One of the following:

"content\_filter"

"max\_tokens"

"run\_cancelled"

"run\_expired"

"run\_failed"

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

object: "thread.message"

The object type, which is always `thread.message`.

role: "user" or "assistant"

The entity that produced the message. One of `user` or `assistant`.

One of the following:

"user"

"assistant"

run\_id: string

The ID of the [run](https://developers.openai.com/docs/api-reference/runs) associated with the creation of this message. Value is `null` when messages are created manually using the create message or create thread endpoints.

status: "in\_progress" or "incomplete" or "completed"

The status of the message, which can be either `in_progress`, `incomplete`, or `completed`.

One of the following:

"in\_progress"

"incomplete"

"completed"

thread\_id: string

The [thread](https://developers.openai.com/docs/api-reference/threads) ID that this message belongs to.

MessageDeleted object { id, deleted, object }

id: string

deleted: boolean

object: "thread.message.deleted"

MessageDelta object { content, role }

The delta containing the fields that have changed on the Message.

content: optional array of [ImageFileDeltaBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file_delta_block%20%3E%20(schema)) { index, type, image\_file }  or [TextDeltaBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text_delta_block%20%3E%20(schema)) { index, type, text }  or [RefusalDeltaBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20refusal_delta_block%20%3E%20(schema)) { index, type, refusal }  or [ImageURLDeltaBlock](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url_delta_block%20%3E%20(schema)) { index, type, image\_url }

The content of the message in array of text and/or images.

One of the following:

ImageFileDeltaBlock object { index, type, image\_file }

References an image [File](https://developers.openai.com/docs/api-reference/files) in the content of a message.

index: number

The index of the content part in the message.

type: "image\_file"

Always `image_file`.

image\_file: optional [ImageFileDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_file_delta%20%3E%20(schema)) { detail, file\_id }

TextDeltaBlock object { index, type, text }

The text content that is part of a message.

index: number

The index of the content part in the message.

type: "text"

Always `text`.

text: optional [TextDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text_delta%20%3E%20(schema)) { annotations, value }

RefusalDeltaBlock object { index, type, refusal }

The refusal content that is part of a message.

index: number

The index of the refusal part in the message.

type: "refusal"

Always `refusal`.

refusal: optional string

ImageURLDeltaBlock object { index, type, image\_url }

References an image URL in the content of a message.

index: number

The index of the content part in the message.

type: "image\_url"

Always `image_url`.

image\_url: optional [ImageURLDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20image_url_delta%20%3E%20(schema)) { detail, url }

role: optional "user" or "assistant"

The entity that produced the message. One of `user` or `assistant`.

One of the following:

"user"

"assistant"

MessageDeltaEvent object { id, delta, object }

Represents a message delta i.e. any changed fields on a message during streaming.

id: string

The identifier of the message, which can be referenced in API endpoints.

delta: [MessageDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20message_delta%20%3E%20(schema)) { content, role }

The delta containing the fields that have changed on the Message.

object: "thread.message.delta"

The object type, which is always `thread.message.delta`.

RefusalContentBlock object { refusal, type }

The refusal content generated by the assistant.

refusal: string

type: "refusal"

Always `refusal`.

RefusalDeltaBlock object { index, type, refusal }

The refusal content that is part of a message.

index: number

The index of the refusal part in the message.

type: "refusal"

Always `refusal`.

refusal: optional string

Text object { annotations, value }

annotations: array of [FileCitationAnnotation](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20file_citation_annotation%20%3E%20(schema)) { end\_index, file\_citation, start\_index, 2 more }  or [FilePathAnnotation](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20file_path_annotation%20%3E%20(schema)) { end\_index, file\_path, start\_index, 2 more }

One of the following:

FileCitationAnnotation object { end\_index, file\_citation, start\_index, 2 more }

A citation within the message that points to a specific quote from a specific File associated with the assistant or the message. Generated when the assistant uses the “file\_search” tool to search files.

end\_index: number

minimum0

file\_citation: object { file\_id }

file\_id: string

The ID of the specific File the citation is from.

start\_index: number

minimum0

text: string

The text in the message content that needs to be replaced.

type: "file\_citation"

Always `file_citation`.

FilePathAnnotation object { end\_index, file\_path, start\_index, 2 more }

A URL for the file that’s generated when the assistant used the `code_interpreter` tool to generate a file.

end\_index: number

minimum0

file\_path: object { file\_id }

file\_id: string

The ID of the file that was generated.

start\_index: number

minimum0

text: string

The text in the message content that needs to be replaced.

type: "file\_path"

Always `file_path`.

value: string

The data that makes up the text.

TextContentBlock object { text, type }

The text content that is part of a message.

text: [Text](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text%20%3E%20(schema)) { annotations, value }

type: "text"

Always `text`.

TextContentBlockParam object { text, type }

The text content that is part of a message.

text: string

Text content to be sent to the model

type: "text"

Always `text`.

TextDelta object { annotations, value }

annotations: optional array of [FileCitationDeltaAnnotation](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20file_citation_delta_annotation%20%3E%20(schema)) { index, type, end\_index, 3 more }  or [FilePathDeltaAnnotation](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20file_path_delta_annotation%20%3E%20(schema)) { index, type, end\_index, 3 more }

One of the following:

FileCitationDeltaAnnotation object { index, type, end\_index, 3 more }

A citation within the message that points to a specific quote from a specific File associated with the assistant or the message. Generated when the assistant uses the “file\_search” tool to search files.

index: number

The index of the annotation in the text content part.

type: "file\_citation"

Always `file_citation`.

end\_index: optional number

minimum0

file\_citation: optional object { file\_id, quote }

file\_id: optional string

The ID of the specific File the citation is from.

quote: optional string

The specific quote in the file.

start\_index: optional number

minimum0

text: optional string

The text in the message content that needs to be replaced.

FilePathDeltaAnnotation object { index, type, end\_index, 3 more }

A URL for the file that’s generated when the assistant used the `code_interpreter` tool to generate a file.

index: number

The index of the annotation in the text content part.

type: "file\_path"

Always `file_path`.

end\_index: optional number

minimum0

file\_path: optional object { file\_id }

file\_id: optional string

The ID of the file that was generated.

start\_index: optional number

minimum0

text: optional string

The text in the message content that needs to be replaced.

value: optional string

The data that makes up the text.

TextDeltaBlock object { index, type, text }

The text content that is part of a message.

index: number

The index of the content part in the message.

type: "text"

Always `text`.

text: optional [TextDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.messages%20%3E%20(model)%20text_delta%20%3E%20(schema)) { annotations, value }

---

## Runs

> Source: https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs
> Fetched: 2026-04-23

Build Assistants that can call models and use tools.

###### [List runs](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/list)

Deprecated

GET/threads/{thread\_id}/runs

###### [Create run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/create)

Deprecated

POST/threads/{thread\_id}/runs

###### [Retrieve run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/retrieve)

Deprecated

GET/threads/{thread\_id}/runs/{run\_id}

###### [Modify run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/update)

Deprecated

POST/threads/{thread\_id}/runs/{run\_id}

###### [Submit tool outputs to run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/submit_tool_outputs)

Deprecated

POST/threads/{thread\_id}/runs/{run\_id}/submit\_tool\_outputs

###### [Cancel a run](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/methods/cancel)

Deprecated

POST/threads/{thread\_id}/runs/{run\_id}/cancel

###### Models

RequiredActionFunctionToolCall object { id, function, type }

Tool call objects

id: string

The ID of the tool call. This ID must be referenced when you submit the tool outputs in using the [Submit tool outputs to run](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) endpoint.

function: object { arguments, name }

The function definition.

arguments: string

The arguments that the model expects you to pass to the function.

name: string

The name of the function.

type: "function"

The type of tool call the output is required for. For now, this is always `function`.

Run object { id, assistant\_id, cancelled\_at, 24 more }

Represents an execution run on a [thread](https://developers.openai.com/docs/api-reference/threads).

id: string

The identifier, which can be referenced in API endpoints.

assistant\_id: string

The ID of the [assistant](https://developers.openai.com/docs/api-reference/assistants) used for execution of this run.

cancelled\_at: number

The Unix timestamp (in seconds) for when the run was cancelled.

completed\_at: number

The Unix timestamp (in seconds) for when the run was completed.

created\_at: number

The Unix timestamp (in seconds) for when the run was created.

expires\_at: number

The Unix timestamp (in seconds) for when the run will expire.

failed\_at: number

The Unix timestamp (in seconds) for when the run failed.

incomplete\_details: object { reason }

Details on why the run is incomplete. Will be `null` if the run is not incomplete.

reason: optional "max\_completion\_tokens" or "max\_prompt\_tokens"

The reason why the run is incomplete. This will point to which specific token limit was reached over the course of the run.

One of the following:

"max\_completion\_tokens"

"max\_prompt\_tokens"

instructions: string

The instructions that the [assistant](https://developers.openai.com/docs/api-reference/assistants) used for this run.

last\_error: object { code, message }

The last error associated with this run. Will be `null` if there are no errors.

code: "server\_error" or "rate\_limit\_exceeded" or "invalid\_prompt"

One of `server_error`, `rate_limit_exceeded`, or `invalid_prompt`.

One of the following:

"server\_error"

"rate\_limit\_exceeded"

"invalid\_prompt"

message: string

A human-readable description of the error.

max\_completion\_tokens: number

The maximum number of completion tokens specified to have been used over the course of the run.

minimum256

max\_prompt\_tokens: number

The maximum number of prompt tokens specified to have been used over the course of the run.

minimum256

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: string

The model that the [assistant](https://developers.openai.com/docs/api-reference/assistants) used for this run.

object: "thread.run"

The object type, which is always `thread.run`.

parallel\_tool\_calls: boolean

Whether to enable [parallel function calling](https://developers.openai.com/docs/guides/function-calling#configuring-parallel-function-calling) during tool use.

required\_action: object { submit\_tool\_outputs, type }

Details on the action required to continue the run. Will be `null` if no action is required.

submit\_tool\_outputs: object { tool\_calls }

Details on the tool outputs needed for this run to continue.

tool\_calls: array of [RequiredActionFunctionToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs%20%3E%20(model)%20required_action_function_tool_call%20%3E%20(schema)) { id, function, type }

A list of the relevant tool calls.

id: string

The ID of the tool call. This ID must be referenced when you submit the tool outputs in using the [Submit tool outputs to run](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) endpoint.

function: object { arguments, name }

The function definition.

arguments: string

The arguments that the model expects you to pass to the function.

name: string

The name of the function.

type: "function"

The type of tool call the output is required for. For now, this is always `function`.

type: "submit\_tool\_outputs"

For now, this is always `submit_tool_outputs`.

response\_format: [AssistantResponseFormatOption](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20assistant_response_format_option%20%3E%20(schema))

Specifies the format that the model must output. Compatible with [GPT-4o](https://developers.openai.com/docs/models#gpt-4o), [GPT-4 Turbo](https://developers.openai.com/docs/models#gpt-4-turbo-and-gpt-4), and all GPT-3.5 Turbo models since `gpt-3.5-turbo-1106`.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables Structured Outputs which ensures the model will match your supplied JSON schema. Learn more in the [Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables JSON mode, which ensures the message the model generates is valid JSON.

**Important:** when using JSON mode, you **must** also instruct the model to produce JSON yourself via a system or user message. Without this, the model may generate an unending stream of whitespace until the generation reaches the token limit, resulting in a long-running and seemingly “stuck” request. Also note that the message content may be partially cut off if `finish_reason="length"`, which indicates the generation exceeded `max_tokens` or the conversation exceeded the max context length.

started\_at: number

The Unix timestamp (in seconds) for when the run was started.

status: "queued" or "in\_progress" or "requires\_action" or 6 more

The status of the run, which can be either `queued`, `in_progress`, `requires_action`, `cancelling`, `cancelled`, `failed`, `completed`, `incomplete`, or `expired`.

One of the following:

"queued"

"in\_progress"

"requires\_action"

"cancelling"

"cancelled"

"failed"

"completed"

"incomplete"

"expired"

thread\_id: string

The ID of the [thread](https://developers.openai.com/docs/api-reference/threads) that was executed on as a part of this run.

tool\_choice: [AssistantToolChoiceOption](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads%20%3E%20(model)%20assistant_tool_choice_option%20%3E%20(schema))

Controls which (if any) tool is called by the model.
`none` means the model will not call any tools and instead generates a message.
`auto` is the default value and means the model can pick between generating a message or calling one or more tools.
`required` means the model must call one or more tools before responding to the user.
Specifying a particular tool like `{"type": "file_search"}` or `{"type": "function", "function": {"name": "my_function"}}` forces the model to call that tool.

tools: array of [CodeInterpreterTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20code_interpreter_tool%20%3E%20(schema)) { type }  or [FileSearchTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20file_search_tool%20%3E%20(schema)) { type, file\_search }  or [FunctionTool](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.assistants%20%3E%20(model)%20function_tool%20%3E%20(schema)) { function, type }

The list of tools that the [assistant](https://developers.openai.com/docs/api-reference/assistants) used for this run.

One of the following:

CodeInterpreterTool object { type }

type: "code\_interpreter"

The type of tool being defined: `code_interpreter`

FileSearchTool object { type, file\_search }

type: "file\_search"

The type of tool being defined: `file_search`

file\_search: optional object { max\_num\_results, ranking\_options }

Overrides for the file search tool.

max\_num\_results: optional number

The maximum number of results the file search tool should output. The default is 20 for `gpt-4*` models and 5 for `gpt-3.5-turbo`. This number should be between 1 and 50 inclusive.

Note that the file search tool may output fewer than `max_num_results` results. See the [file search tool documentation](https://developers.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings) for more information.

minimum1

maximum50

ranking\_options: optional object { score\_threshold, ranker }

The ranking options for the file search. If not specified, the file search tool will use the `auto` ranker and a score\_threshold of 0.

See the [file search tool documentation](https://developers.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings) for more information.

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

ranker: optional "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

FunctionTool object { function, type }

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of tool being defined: `function`

truncation\_strategy: object { type, last\_messages }

Controls for how a thread will be truncated prior to the run. Use this to control the initial context window of the run.

type: "auto" or "last\_messages"

The truncation strategy to use for the thread. The default is `auto`. If set to `last_messages`, the thread will be truncated to the n most recent messages in the thread. When set to `auto`, messages in the middle of the thread will be dropped to fit the context length of the model, `max_prompt_tokens`.

One of the following:

"auto"

"last\_messages"

last\_messages: optional number

The number of most recent messages from the thread when constructing the context for the run.

minimum1

usage: object { completion\_tokens, prompt\_tokens, total\_tokens }

Usage statistics related to the run. This value will be `null` if the run is not in a terminal state (i.e. `in_progress`, `queued`, etc.).

completion\_tokens: number

Number of completion tokens used over the course of the run.

prompt\_tokens: number

Number of prompt tokens used over the course of the run.

total\_tokens: number

Total number of tokens used (prompt + completion).

temperature: optional number

The sampling temperature used for this run. If not set, defaults to 1.

top\_p: optional number

The nucleus sampling value used for this run. If not set, defaults to 1.

##### RunsSteps

Build Assistants that can call models and use tools.

###### [List run steps](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/subresources/steps/methods/list)

Deprecated

GET/threads/{thread\_id}/runs/{run\_id}/steps

###### [Retrieve run step](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/subresources/steps/methods/retrieve)

Deprecated

GET/threads/{thread\_id}/runs/{run\_id}/steps/{step\_id}

###### Models

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

CodeInterpreterToolCall object { id, code\_interpreter, type }

Details of the Code Interpreter tool call the run step was involved in.

id: string

The ID of the tool call.

code\_interpreter: object { input, outputs }

The Code Interpreter tool call definition.

input: string

The input to the Code Interpreter tool call.

outputs: array of object { logs, type }  or object { image, type }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogOutput object { logs, type }

Text output from the Code Interpreter tool call as part of a run step.

logs: string

The text output from the Code Interpreter tool call.

type: "logs"

Always `logs`.

CodeInterpreterImageOutput object { image, type }

image: object { file\_id }

file\_id: string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

type: "image"

Always `image`.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

CodeInterpreterToolCallDelta object { index, type, id, code\_interpreter }

Details of the Code Interpreter tool call the run step was involved in.

index: number

The index of the tool call in the tool calls array.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

id: optional string

The ID of the tool call.

code\_interpreter: optional object { input, outputs }

The Code Interpreter tool call definition.

input: optional string

The input to the Code Interpreter tool call.

outputs: optional array of [CodeInterpreterLogs](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_logs%20%3E%20(schema)) { index, type, logs }  or [CodeInterpreterOutputImage](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_output_image%20%3E%20(schema)) { index, type, image }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

FileSearchToolCall object { id, file\_search, type }

id: string

The ID of the tool call object.

file\_search: object { ranking\_options, results }

For now, this is always going to be an empty object.

ranking\_options: optional object { ranker, score\_threshold }

The ranking options for the file search.

ranker: "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

results: optional array of object { file\_id, file\_name, score, content }

The results of the file search.

file\_id: string

The ID of the file that result was found in.

file\_name: string

The name of the file that result was found in.

score: number

The score of the result. All values must be a floating point number between 0 and 1.

minimum0

maximum1

content: optional array of object { text, type }

The content of the result that was found. The content is only included if requested via the include query parameter.

text: optional string

The text content of the file.

type: optional "text"

The type of the content.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

FileSearchToolCallDelta object { file\_search, index, type, id }

file\_search: unknown

For now, this is always going to be an empty object.

index: number

The index of the tool call in the tool calls array.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

id: optional string

The ID of the tool call object.

FunctionToolCall object { id, function, type }

id: string

The ID of the tool call object.

function: object { arguments, name, output }

The definition of the function that was called.

arguments: string

The arguments passed to the function.

name: string

The name of the function.

output: string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

FunctionToolCallDelta object { index, type, id, function }

index: number

The index of the tool call in the tool calls array.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

id: optional string

The ID of the tool call object.

function: optional object { arguments, name, output }

The definition of the function that was called.

arguments: optional string

The arguments passed to the function.

name: optional string

The name of the function.

output: optional string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

MessageCreationStepDetails object { message\_creation, type }

Details of the message creation by the run step.

message\_creation: object { message\_id }

message\_id: string

The ID of the message that was created by this run step.

type: "message\_creation"

Always `message_creation`.

RunStep object { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

id: string

The identifier of the run step, which can be referenced in API endpoints.

assistant\_id: string

The ID of the [assistant](https://developers.openai.com/docs/api-reference/assistants) associated with the run step.

cancelled\_at: number

The Unix timestamp (in seconds) for when the run step was cancelled.

completed\_at: number

The Unix timestamp (in seconds) for when the run step completed.

created\_at: number

The Unix timestamp (in seconds) for when the run step was created.

expired\_at: number

The Unix timestamp (in seconds) for when the run step expired. A step is considered expired if the parent run is expired.

failed\_at: number

The Unix timestamp (in seconds) for when the run step failed.

last\_error: object { code, message }

The last error associated with this run step. Will be `null` if there are no errors.

code: "server\_error" or "rate\_limit\_exceeded"

One of `server_error` or `rate_limit_exceeded`.

One of the following:

"server\_error"

"rate\_limit\_exceeded"

message: string

A human-readable description of the error.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

object: "thread.run.step"

The object type, which is always `thread.run.step`.

run\_id: string

The ID of the [run](https://developers.openai.com/docs/api-reference/runs) that this run step is a part of.

status: "in\_progress" or "cancelled" or "failed" or 2 more

The status of the run step, which can be either `in_progress`, `cancelled`, `failed`, `completed`, or `expired`.

One of the following:

"in\_progress"

"cancelled"

"failed"

"completed"

"expired"

step\_details: [MessageCreationStepDetails](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20message_creation_step_details%20%3E%20(schema)) { message\_creation, type }  or [ToolCallsStepDetails](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20tool_calls_step_details%20%3E%20(schema)) { tool\_calls, type }

The details of the run step.

One of the following:

MessageCreationStepDetails object { message\_creation, type }

Details of the message creation by the run step.

message\_creation: object { message\_id }

message\_id: string

The ID of the message that was created by this run step.

type: "message\_creation"

Always `message_creation`.

ToolCallsStepDetails object { tool\_calls, type }

Details of the tool call.

tool\_calls: array of [CodeInterpreterToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call%20%3E%20(schema)) { id, code\_interpreter, type }  or [FileSearchToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call%20%3E%20(schema)) { id, file\_search, type }  or [FunctionToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call%20%3E%20(schema)) { id, function, type }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCall object { id, code\_interpreter, type }

Details of the Code Interpreter tool call the run step was involved in.

id: string

The ID of the tool call.

code\_interpreter: object { input, outputs }

The Code Interpreter tool call definition.

input: string

The input to the Code Interpreter tool call.

outputs: array of object { logs, type }  or object { image, type }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogOutput object { logs, type }

Text output from the Code Interpreter tool call as part of a run step.

logs: string

The text output from the Code Interpreter tool call.

type: "logs"

Always `logs`.

CodeInterpreterImageOutput object { image, type }

image: object { file\_id }

file\_id: string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

type: "image"

Always `image`.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

FileSearchToolCall object { id, file\_search, type }

id: string

The ID of the tool call object.

file\_search: object { ranking\_options, results }

For now, this is always going to be an empty object.

ranking\_options: optional object { ranker, score\_threshold }

The ranking options for the file search.

ranker: "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

results: optional array of object { file\_id, file\_name, score, content }

The results of the file search.

file\_id: string

The ID of the file that result was found in.

file\_name: string

The name of the file that result was found in.

score: number

The score of the result. All values must be a floating point number between 0 and 1.

minimum0

maximum1

content: optional array of object { text, type }

The content of the result that was found. The content is only included if requested via the include query parameter.

text: optional string

The text content of the file.

type: optional "text"

The type of the content.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

FunctionToolCall object { id, function, type }

id: string

The ID of the tool call object.

function: object { arguments, name, output }

The definition of the function that was called.

arguments: string

The arguments passed to the function.

name: string

The name of the function.

output: string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

type: "tool\_calls"

Always `tool_calls`.

thread\_id: string

The ID of the [thread](https://developers.openai.com/docs/api-reference/threads) that was run.

type: "message\_creation" or "tool\_calls"

The type of run step, which can be either `message_creation` or `tool_calls`.

One of the following:

"message\_creation"

"tool\_calls"

usage: object { completion\_tokens, prompt\_tokens, total\_tokens }

Usage statistics related to the run step. This value will be `null` while the run step’s status is `in_progress`.

completion\_tokens: number

Number of completion tokens used over the course of the run step.

prompt\_tokens: number

Number of prompt tokens used over the course of the run step.

total\_tokens: number

Total number of tokens used (prompt + completion).

RunStepDeltaEvent object { id, delta, object }

Represents a run step delta i.e. any changed fields on a run step during streaming.

id: string

The identifier of the run step, which can be referenced in API endpoints.

delta: object { step\_details }

The delta containing the fields that have changed on the run step.

step\_details: optional [RunStepDeltaMessageDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step_delta_message_delta%20%3E%20(schema)) { type, message\_creation }  or [ToolCallDeltaObject](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20tool_call_delta_object%20%3E%20(schema)) { type, tool\_calls }

The details of the run step.

One of the following:

RunStepDeltaMessageDelta object { type, message\_creation }

Details of the message creation by the run step.

type: "message\_creation"

Always `message_creation`.

message\_creation: optional object { message\_id }

message\_id: optional string

The ID of the message that was created by this run step.

ToolCallDeltaObject object { type, tool\_calls }

Details of the tool call.

type: "tool\_calls"

Always `tool_calls`.

tool\_calls: optional array of [CodeInterpreterToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call_delta%20%3E%20(schema)) { index, type, id, code\_interpreter }  or [FileSearchToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call_delta%20%3E%20(schema)) { file\_search, index, type, id }  or [FunctionToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call_delta%20%3E%20(schema)) { index, type, id, function }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCallDelta object { index, type, id, code\_interpreter }

Details of the Code Interpreter tool call the run step was involved in.

index: number

The index of the tool call in the tool calls array.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

id: optional string

The ID of the tool call.

code\_interpreter: optional object { input, outputs }

The Code Interpreter tool call definition.

input: optional string

The input to the Code Interpreter tool call.

outputs: optional array of [CodeInterpreterLogs](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_logs%20%3E%20(schema)) { index, type, logs }  or [CodeInterpreterOutputImage](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_output_image%20%3E%20(schema)) { index, type, image }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

FileSearchToolCallDelta object { file\_search, index, type, id }

file\_search: unknown

For now, this is always going to be an empty object.

index: number

The index of the tool call in the tool calls array.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

id: optional string

The ID of the tool call object.

FunctionToolCallDelta object { index, type, id, function }

index: number

The index of the tool call in the tool calls array.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

id: optional string

The ID of the tool call object.

function: optional object { arguments, name, output }

The definition of the function that was called.

arguments: optional string

The arguments passed to the function.

name: optional string

The name of the function.

output: optional string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

object: "thread.run.step.delta"

The object type, which is always `thread.run.step.delta`.

RunStepDeltaMessageDelta object { type, message\_creation }

Details of the message creation by the run step.

type: "message\_creation"

Always `message_creation`.

message\_creation: optional object { message\_id }

message\_id: optional string

The ID of the message that was created by this run step.

RunStepInclude = "step\_details.tool\_calls[\*].file\_search.results[\*].content"

ToolCallDeltaObject object { type, tool\_calls }

Details of the tool call.

type: "tool\_calls"

Always `tool_calls`.

tool\_calls: optional array of [CodeInterpreterToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call_delta%20%3E%20(schema)) { index, type, id, code\_interpreter }  or [FileSearchToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call_delta%20%3E%20(schema)) { file\_search, index, type, id }  or [FunctionToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call_delta%20%3E%20(schema)) { index, type, id, function }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCallDelta object { index, type, id, code\_interpreter }

Details of the Code Interpreter tool call the run step was involved in.

index: number

The index of the tool call in the tool calls array.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

id: optional string

The ID of the tool call.

code\_interpreter: optional object { input, outputs }

The Code Interpreter tool call definition.

input: optional string

The input to the Code Interpreter tool call.

outputs: optional array of [CodeInterpreterLogs](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_logs%20%3E%20(schema)) { index, type, logs }  or [CodeInterpreterOutputImage](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_output_image%20%3E%20(schema)) { index, type, image }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

FileSearchToolCallDelta object { file\_search, index, type, id }

file\_search: unknown

For now, this is always going to be an empty object.

index: number

The index of the tool call in the tool calls array.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

id: optional string

The ID of the tool call object.

FunctionToolCallDelta object { index, type, id, function }

index: number

The index of the tool call in the tool calls array.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

id: optional string

The ID of the tool call object.

function: optional object { arguments, name, output }

The definition of the function that was called.

arguments: optional string

The arguments passed to the function.

name: optional string

The name of the function.

output: optional string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

ToolCallsStepDetails object { tool\_calls, type }

Details of the tool call.

tool\_calls: array of [CodeInterpreterToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call%20%3E%20(schema)) { id, code\_interpreter, type }  or [FileSearchToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call%20%3E%20(schema)) { id, file\_search, type }  or [FunctionToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call%20%3E%20(schema)) { id, function, type }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCall object { id, code\_interpreter, type }

Details of the Code Interpreter tool call the run step was involved in.

id: string

The ID of the tool call.

code\_interpreter: object { input, outputs }

The Code Interpreter tool call definition.

input: string

The input to the Code Interpreter tool call.

outputs: array of object { logs, type }  or object { image, type }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogOutput object { logs, type }

Text output from the Code Interpreter tool call as part of a run step.

logs: string

The text output from the Code Interpreter tool call.

type: "logs"

Always `logs`.

CodeInterpreterImageOutput object { image, type }

image: object { file\_id }

file\_id: string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

type: "image"

Always `image`.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

FileSearchToolCall object { id, file\_search, type }

id: string

The ID of the tool call object.

file\_search: object { ranking\_options, results }

For now, this is always going to be an empty object.

ranking\_options: optional object { ranker, score\_threshold }

The ranking options for the file search.

ranker: "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

results: optional array of object { file\_id, file\_name, score, content }

The results of the file search.

file\_id: string

The ID of the file that result was found in.

file\_name: string

The name of the file that result was found in.

score: number

The score of the result. All values must be a floating point number between 0 and 1.

minimum0

maximum1

content: optional array of object { text, type }

The content of the result that was found. The content is only included if requested via the include query parameter.

text: optional string

The text content of the file.

type: optional "text"

The type of the content.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

FunctionToolCall object { id, function, type }

id: string

The ID of the tool call object.

function: object { arguments, name, output }

The definition of the function that was called.

arguments: string

The arguments passed to the function.

name: string

The name of the function.

output: string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

type: "tool\_calls"

Always `tool_calls`.

---

## Steps

> Source: https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/subresources/steps
> Fetched: 2026-04-23

Build Assistants that can call models and use tools.

###### [List run steps](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/subresources/steps/methods/list)

Deprecated

GET/threads/{thread\_id}/runs/{run\_id}/steps

###### [Retrieve run step](https://developers.openai.com/api/reference/resources/beta/subresources/threads/subresources/runs/subresources/steps/methods/retrieve)

Deprecated

GET/threads/{thread\_id}/runs/{run\_id}/steps/{step\_id}

###### Models

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

CodeInterpreterToolCall object { id, code\_interpreter, type }

Details of the Code Interpreter tool call the run step was involved in.

id: string

The ID of the tool call.

code\_interpreter: object { input, outputs }

The Code Interpreter tool call definition.

input: string

The input to the Code Interpreter tool call.

outputs: array of object { logs, type }  or object { image, type }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogOutput object { logs, type }

Text output from the Code Interpreter tool call as part of a run step.

logs: string

The text output from the Code Interpreter tool call.

type: "logs"

Always `logs`.

CodeInterpreterImageOutput object { image, type }

image: object { file\_id }

file\_id: string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

type: "image"

Always `image`.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

CodeInterpreterToolCallDelta object { index, type, id, code\_interpreter }

Details of the Code Interpreter tool call the run step was involved in.

index: number

The index of the tool call in the tool calls array.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

id: optional string

The ID of the tool call.

code\_interpreter: optional object { input, outputs }

The Code Interpreter tool call definition.

input: optional string

The input to the Code Interpreter tool call.

outputs: optional array of [CodeInterpreterLogs](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_logs%20%3E%20(schema)) { index, type, logs }  or [CodeInterpreterOutputImage](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_output_image%20%3E%20(schema)) { index, type, image }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

FileSearchToolCall object { id, file\_search, type }

id: string

The ID of the tool call object.

file\_search: object { ranking\_options, results }

For now, this is always going to be an empty object.

ranking\_options: optional object { ranker, score\_threshold }

The ranking options for the file search.

ranker: "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

results: optional array of object { file\_id, file\_name, score, content }

The results of the file search.

file\_id: string

The ID of the file that result was found in.

file\_name: string

The name of the file that result was found in.

score: number

The score of the result. All values must be a floating point number between 0 and 1.

minimum0

maximum1

content: optional array of object { text, type }

The content of the result that was found. The content is only included if requested via the include query parameter.

text: optional string

The text content of the file.

type: optional "text"

The type of the content.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

FileSearchToolCallDelta object { file\_search, index, type, id }

file\_search: unknown

For now, this is always going to be an empty object.

index: number

The index of the tool call in the tool calls array.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

id: optional string

The ID of the tool call object.

FunctionToolCall object { id, function, type }

id: string

The ID of the tool call object.

function: object { arguments, name, output }

The definition of the function that was called.

arguments: string

The arguments passed to the function.

name: string

The name of the function.

output: string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

FunctionToolCallDelta object { index, type, id, function }

index: number

The index of the tool call in the tool calls array.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

id: optional string

The ID of the tool call object.

function: optional object { arguments, name, output }

The definition of the function that was called.

arguments: optional string

The arguments passed to the function.

name: optional string

The name of the function.

output: optional string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

MessageCreationStepDetails object { message\_creation, type }

Details of the message creation by the run step.

message\_creation: object { message\_id }

message\_id: string

The ID of the message that was created by this run step.

type: "message\_creation"

Always `message_creation`.

RunStep object { id, assistant\_id, cancelled\_at, 13 more }

Represents a step in execution of a run.

id: string

The identifier of the run step, which can be referenced in API endpoints.

assistant\_id: string

The ID of the [assistant](https://developers.openai.com/docs/api-reference/assistants) associated with the run step.

cancelled\_at: number

The Unix timestamp (in seconds) for when the run step was cancelled.

completed\_at: number

The Unix timestamp (in seconds) for when the run step completed.

created\_at: number

The Unix timestamp (in seconds) for when the run step was created.

expired\_at: number

The Unix timestamp (in seconds) for when the run step expired. A step is considered expired if the parent run is expired.

failed\_at: number

The Unix timestamp (in seconds) for when the run step failed.

last\_error: object { code, message }

The last error associated with this run step. Will be `null` if there are no errors.

code: "server\_error" or "rate\_limit\_exceeded"

One of `server_error` or `rate_limit_exceeded`.

One of the following:

"server\_error"

"rate\_limit\_exceeded"

message: string

A human-readable description of the error.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

object: "thread.run.step"

The object type, which is always `thread.run.step`.

run\_id: string

The ID of the [run](https://developers.openai.com/docs/api-reference/runs) that this run step is a part of.

status: "in\_progress" or "cancelled" or "failed" or 2 more

The status of the run step, which can be either `in_progress`, `cancelled`, `failed`, `completed`, or `expired`.

One of the following:

"in\_progress"

"cancelled"

"failed"

"completed"

"expired"

step\_details: [MessageCreationStepDetails](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20message_creation_step_details%20%3E%20(schema)) { message\_creation, type }  or [ToolCallsStepDetails](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20tool_calls_step_details%20%3E%20(schema)) { tool\_calls, type }

The details of the run step.

One of the following:

MessageCreationStepDetails object { message\_creation, type }

Details of the message creation by the run step.

message\_creation: object { message\_id }

message\_id: string

The ID of the message that was created by this run step.

type: "message\_creation"

Always `message_creation`.

ToolCallsStepDetails object { tool\_calls, type }

Details of the tool call.

tool\_calls: array of [CodeInterpreterToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call%20%3E%20(schema)) { id, code\_interpreter, type }  or [FileSearchToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call%20%3E%20(schema)) { id, file\_search, type }  or [FunctionToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call%20%3E%20(schema)) { id, function, type }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCall object { id, code\_interpreter, type }

Details of the Code Interpreter tool call the run step was involved in.

id: string

The ID of the tool call.

code\_interpreter: object { input, outputs }

The Code Interpreter tool call definition.

input: string

The input to the Code Interpreter tool call.

outputs: array of object { logs, type }  or object { image, type }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogOutput object { logs, type }

Text output from the Code Interpreter tool call as part of a run step.

logs: string

The text output from the Code Interpreter tool call.

type: "logs"

Always `logs`.

CodeInterpreterImageOutput object { image, type }

image: object { file\_id }

file\_id: string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

type: "image"

Always `image`.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

FileSearchToolCall object { id, file\_search, type }

id: string

The ID of the tool call object.

file\_search: object { ranking\_options, results }

For now, this is always going to be an empty object.

ranking\_options: optional object { ranker, score\_threshold }

The ranking options for the file search.

ranker: "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

results: optional array of object { file\_id, file\_name, score, content }

The results of the file search.

file\_id: string

The ID of the file that result was found in.

file\_name: string

The name of the file that result was found in.

score: number

The score of the result. All values must be a floating point number between 0 and 1.

minimum0

maximum1

content: optional array of object { text, type }

The content of the result that was found. The content is only included if requested via the include query parameter.

text: optional string

The text content of the file.

type: optional "text"

The type of the content.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

FunctionToolCall object { id, function, type }

id: string

The ID of the tool call object.

function: object { arguments, name, output }

The definition of the function that was called.

arguments: string

The arguments passed to the function.

name: string

The name of the function.

output: string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

type: "tool\_calls"

Always `tool_calls`.

thread\_id: string

The ID of the [thread](https://developers.openai.com/docs/api-reference/threads) that was run.

type: "message\_creation" or "tool\_calls"

The type of run step, which can be either `message_creation` or `tool_calls`.

One of the following:

"message\_creation"

"tool\_calls"

usage: object { completion\_tokens, prompt\_tokens, total\_tokens }

Usage statistics related to the run step. This value will be `null` while the run step’s status is `in_progress`.

completion\_tokens: number

Number of completion tokens used over the course of the run step.

prompt\_tokens: number

Number of prompt tokens used over the course of the run step.

total\_tokens: number

Total number of tokens used (prompt + completion).

RunStepDeltaEvent object { id, delta, object }

Represents a run step delta i.e. any changed fields on a run step during streaming.

id: string

The identifier of the run step, which can be referenced in API endpoints.

delta: object { step\_details }

The delta containing the fields that have changed on the run step.

step\_details: optional [RunStepDeltaMessageDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20run_step_delta_message_delta%20%3E%20(schema)) { type, message\_creation }  or [ToolCallDeltaObject](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20tool_call_delta_object%20%3E%20(schema)) { type, tool\_calls }

The details of the run step.

One of the following:

RunStepDeltaMessageDelta object { type, message\_creation }

Details of the message creation by the run step.

type: "message\_creation"

Always `message_creation`.

message\_creation: optional object { message\_id }

message\_id: optional string

The ID of the message that was created by this run step.

ToolCallDeltaObject object { type, tool\_calls }

Details of the tool call.

type: "tool\_calls"

Always `tool_calls`.

tool\_calls: optional array of [CodeInterpreterToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call_delta%20%3E%20(schema)) { index, type, id, code\_interpreter }  or [FileSearchToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call_delta%20%3E%20(schema)) { file\_search, index, type, id }  or [FunctionToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call_delta%20%3E%20(schema)) { index, type, id, function }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCallDelta object { index, type, id, code\_interpreter }

Details of the Code Interpreter tool call the run step was involved in.

index: number

The index of the tool call in the tool calls array.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

id: optional string

The ID of the tool call.

code\_interpreter: optional object { input, outputs }

The Code Interpreter tool call definition.

input: optional string

The input to the Code Interpreter tool call.

outputs: optional array of [CodeInterpreterLogs](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_logs%20%3E%20(schema)) { index, type, logs }  or [CodeInterpreterOutputImage](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_output_image%20%3E%20(schema)) { index, type, image }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

FileSearchToolCallDelta object { file\_search, index, type, id }

file\_search: unknown

For now, this is always going to be an empty object.

index: number

The index of the tool call in the tool calls array.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

id: optional string

The ID of the tool call object.

FunctionToolCallDelta object { index, type, id, function }

index: number

The index of the tool call in the tool calls array.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

id: optional string

The ID of the tool call object.

function: optional object { arguments, name, output }

The definition of the function that was called.

arguments: optional string

The arguments passed to the function.

name: optional string

The name of the function.

output: optional string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

object: "thread.run.step.delta"

The object type, which is always `thread.run.step.delta`.

RunStepDeltaMessageDelta object { type, message\_creation }

Details of the message creation by the run step.

type: "message\_creation"

Always `message_creation`.

message\_creation: optional object { message\_id }

message\_id: optional string

The ID of the message that was created by this run step.

RunStepInclude = "step\_details.tool\_calls[\*].file\_search.results[\*].content"

ToolCallDeltaObject object { type, tool\_calls }

Details of the tool call.

type: "tool\_calls"

Always `tool_calls`.

tool\_calls: optional array of [CodeInterpreterToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call_delta%20%3E%20(schema)) { index, type, id, code\_interpreter }  or [FileSearchToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call_delta%20%3E%20(schema)) { file\_search, index, type, id }  or [FunctionToolCallDelta](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call_delta%20%3E%20(schema)) { index, type, id, function }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCallDelta object { index, type, id, code\_interpreter }

Details of the Code Interpreter tool call the run step was involved in.

index: number

The index of the tool call in the tool calls array.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

id: optional string

The ID of the tool call.

code\_interpreter: optional object { input, outputs }

The Code Interpreter tool call definition.

input: optional string

The input to the Code Interpreter tool call.

outputs: optional array of [CodeInterpreterLogs](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_logs%20%3E%20(schema)) { index, type, logs }  or [CodeInterpreterOutputImage](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_output_image%20%3E%20(schema)) { index, type, image }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogs object { index, type, logs }

Text output from the Code Interpreter tool call as part of a run step.

index: number

The index of the output in the outputs array.

type: "logs"

Always `logs`.

logs: optional string

The text output from the Code Interpreter tool call.

CodeInterpreterOutputImage object { index, type, image }

index: number

The index of the output in the outputs array.

type: "image"

Always `image`.

image: optional object { file\_id }

file\_id: optional string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

FileSearchToolCallDelta object { file\_search, index, type, id }

file\_search: unknown

For now, this is always going to be an empty object.

index: number

The index of the tool call in the tool calls array.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

id: optional string

The ID of the tool call object.

FunctionToolCallDelta object { index, type, id, function }

index: number

The index of the tool call in the tool calls array.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

id: optional string

The ID of the tool call object.

function: optional object { arguments, name, output }

The definition of the function that was called.

arguments: optional string

The arguments passed to the function.

name: optional string

The name of the function.

output: optional string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

ToolCallsStepDetails object { tool\_calls, type }

Details of the tool call.

tool\_calls: array of [CodeInterpreterToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20code_interpreter_tool_call%20%3E%20(schema)) { id, code\_interpreter, type }  or [FileSearchToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20file_search_tool_call%20%3E%20(schema)) { id, file\_search, type }  or [FunctionToolCall](https://developers.openai.com/api/reference/resources/beta#(resource)%20beta.threads.runs.steps%20%3E%20(model)%20function_tool_call%20%3E%20(schema)) { id, function, type }

An array of tool calls the run step was involved in. These can be associated with one of three types of tools: `code_interpreter`, `file_search`, or `function`.

One of the following:

CodeInterpreterToolCall object { id, code\_interpreter, type }

Details of the Code Interpreter tool call the run step was involved in.

id: string

The ID of the tool call.

code\_interpreter: object { input, outputs }

The Code Interpreter tool call definition.

input: string

The input to the Code Interpreter tool call.

outputs: array of object { logs, type }  or object { image, type }

The outputs from the Code Interpreter tool call. Code Interpreter can output one or more items, including text (`logs`) or images (`image`). Each of these are represented by a different object type.

One of the following:

CodeInterpreterLogOutput object { logs, type }

Text output from the Code Interpreter tool call as part of a run step.

logs: string

The text output from the Code Interpreter tool call.

type: "logs"

Always `logs`.

CodeInterpreterImageOutput object { image, type }

image: object { file\_id }

file\_id: string

The [file](https://developers.openai.com/docs/api-reference/files) ID of the image.

type: "image"

Always `image`.

type: "code\_interpreter"

The type of tool call. This is always going to be `code_interpreter` for this type of tool call.

FileSearchToolCall object { id, file\_search, type }

id: string

The ID of the tool call object.

file\_search: object { ranking\_options, results }

For now, this is always going to be an empty object.

ranking\_options: optional object { ranker, score\_threshold }

The ranking options for the file search.

ranker: "auto" or "default\_2024\_08\_21"

The ranker to use for the file search. If not specified will use the `auto` ranker.

One of the following:

"auto"

"default\_2024\_08\_21"

score\_threshold: number

The score threshold for the file search. All values must be a floating point number between 0 and 1.

minimum0

maximum1

results: optional array of object { file\_id, file\_name, score, content }

The results of the file search.

file\_id: string

The ID of the file that result was found in.

file\_name: string

The name of the file that result was found in.

score: number

The score of the result. All values must be a floating point number between 0 and 1.

minimum0

maximum1

content: optional array of object { text, type }

The content of the result that was found. The content is only included if requested via the include query parameter.

text: optional string

The text content of the file.

type: optional "text"

The type of the content.

type: "file\_search"

The type of tool call. This is always going to be `file_search` for this type of tool call.

FunctionToolCall object { id, function, type }

id: string

The ID of the tool call object.

function: object { arguments, name, output }

The definition of the function that was called.

arguments: string

The arguments passed to the function.

name: string

The name of the function.

output: string

The output of the function. This will be `null` if the outputs have not been [submitted](https://developers.openai.com/docs/api-reference/runs/submitToolOutputs) yet.

type: "function"

The type of tool call. This is always going to be `function` for this type of tool call.

type: "tool\_calls"

Always `tool_calls`.
