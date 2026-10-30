# Chat

> Source: https://developers.openai.com/api/reference/resources/chat
> Fetched: 2026-04-23

#### ChatCompletions

Given a list of messages comprising a conversation, the model will return a response.

##### [Create chat completion](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)

POST/chat/completions

##### [List Chat Completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/list)

GET/chat/completions

##### [Get chat completion](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/retrieve)

GET/chat/completions/{completion\_id}

##### [Update chat completion](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/update)

POST/chat/completions/{completion\_id}

##### [Delete chat completion](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/delete)

DELETE/chat/completions/{completion\_id}

##### Models

ChatCompletion object { id, choices, created, 5 more }

Represents a chat completion response returned by model, based on the provided input.

id: string

A unique identifier for the chat completion.

choices: array of object { finish\_reason, index, logprobs, message }

A list of chat completion choices. Can be more than one if `n` is greater than 1.

finish\_reason: "stop" or "length" or "tool\_calls" or 2 more

The reason the model stopped generating tokens. This will be `stop` if the model hit a natural stop point or a provided stop sequence,
`length` if the maximum number of tokens specified in the request was reached,
`content_filter` if content was omitted due to a flag from our content filters,
`tool_calls` if the model called a tool, or `function_call` (deprecated) if the model called a function.

One of the following:

"stop"

"length"

"tool\_calls"

"content\_filter"

"function\_call"

index: number

The index of the choice in the list of choices.

logprobs: object { content, refusal }

Log probability information for the choice.

content: array of [ChatCompletionTokenLogprob](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)) { token, bytes, logprob, top\_logprobs }

A list of message content tokens with log probability information.

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

top\_logprobs: array of object { token, bytes, logprob }

List of the most likely tokens and their log probability, at this token position. In rare cases, there may be fewer than the number of requested `top_logprobs` returned.

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

refusal: array of [ChatCompletionTokenLogprob](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)) { token, bytes, logprob, top\_logprobs }

A list of message refusal tokens with log probability information.

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

top\_logprobs: array of object { token, bytes, logprob }

List of the most likely tokens and their log probability, at this token position. In rare cases, there may be fewer than the number of requested `top_logprobs` returned.

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

message: [ChatCompletionMessage](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)) { content, refusal, role, 4 more }

A chat completion message generated by the model.

created: number

The Unix timestamp (in seconds) of when the chat completion was created.

model: string

The model used for the chat completion.

object: "chat.completion"

The object type, which is always `chat.completion`.

service\_tier: optional "auto" or "default" or "flex" or 2 more

Specifies the processing type used for serving the request.

- If set to ‘auto’, then the request will be processed with the service tier configured in the Project settings. Unless otherwise configured, the Project will use ‘default’.
- If set to ‘default’, then the request will be processed with the standard pricing and performance for the selected model.
- If set to ‘[flex](https://developers.openai.com/docs/guides/flex-processing)’ or ‘[priority](https://openai.com/api-priority-processing/)’, then the request will be processed with the corresponding service tier.
- When not set, the default behavior is ‘auto’.

When the `service_tier` parameter is set, the response body will include the `service_tier` value based on the processing mode actually used to serve the request. This response value may be different from the value set in the parameter.

One of the following:

"auto"

"default"

"flex"

"scale"

"priority"

Deprecatedsystem\_fingerprint: optional string

This fingerprint represents the backend configuration that the model runs with.

Can be used in conjunction with the `seed` request parameter to understand when backend changes have been made that might impact determinism.

usage: optional [CompletionUsage](https://developers.openai.com/api/reference/resources/completions#(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)) { completion\_tokens, prompt\_tokens, total\_tokens, 2 more }

Usage statistics for the completion request.

ChatCompletionAllowedToolChoice object { allowed\_tools, type }

Constrains the tools available to the model to a pre-defined set.

allowed\_tools: [ChatCompletionAllowedTools](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20ChatCompletionAllowedTools%20%3E%20(schema)) { mode, tools }

Constrains the tools available to the model to a pre-defined set.

type: "allowed\_tools"

Allowed tool configuration type. Always `allowed_tools`.

ChatCompletionAssistantMessageParam object { role, audio, content, 4 more }

Messages sent by the model in response to user messages.

role: "assistant"

The role of the messages author, in this case `assistant`.

audio: optional object { id }

Data about a previous audio response from the model.
[Learn more](https://developers.openai.com/docs/guides/audio).

id: string

Unique identifier for a previous audio response from the model.

content: optional string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }  or [ChatCompletionContentPartRefusal](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_refusal%20%3E%20(schema)) { refusal, type }

The contents of the assistant message. Required unless `tool_calls` or `function_call` is specified.

One of the following:

TextContent = string

The contents of the assistant message.

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }  or [ChatCompletionContentPartRefusal](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_refusal%20%3E%20(schema)) { refusal, type }

An array of content parts with a defined type. Can be one or more of type `text`, or exactly one of type `refusal`.

One of the following:

ChatCompletionContentPartText object { text, type }

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

text: string

The text content.

type: "text"

The type of the content part.

ChatCompletionContentPartRefusal object { refusal, type }

refusal: string

The refusal message generated by the model.

type: "refusal"

The type of the content part.

Deprecatedfunction\_call: optional object { arguments, name }

Deprecated and replaced by `tool_calls`. The name and arguments of a function that should be called, as generated by the model.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: string

The name of the function to call.

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

refusal: optional string

The refusal message by the assistant.

tool\_calls: optional array of [ChatCompletionMessageToolCall](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_tool_call%20%3E%20(schema))

The tool calls generated by the model, such as function calls.

One of the following:

ChatCompletionMessageFunctionToolCall object { id, function, type }

A call to a function tool created by the model.

id: string

The ID of the tool call.

function: object { arguments, name }

The function that the model called.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: string

The name of the function to call.

type: "function"

The type of the tool. Currently, only `function` is supported.

ChatCompletionMessageCustomToolCall object { id, custom, type }

A call to a custom tool created by the model.

id: string

The ID of the tool call.

custom: object { input, name }

The custom tool that the model called.

input: string

The input for the custom tool call generated by the model.

name: string

The name of the custom tool to call.

type: "custom"

The type of the tool. Always `custom`.

ChatCompletionAudio object { id, data, expires\_at, transcript }

If the audio output modality is requested, this object contains data
about the audio response from the model. [Learn more](https://developers.openai.com/docs/guides/audio).

id: string

Unique identifier for this audio response.

data: string

Base64 encoded audio bytes generated by the model, in the format
specified in the request.

expires\_at: number

The Unix timestamp (in seconds) for when this audio response will
no longer be accessible on the server for use in multi-turn
conversations.

transcript: string

Transcript of the audio generated by the model.

ChatCompletionAudioParam object { format, voice }

Parameters for audio output. Required when audio output is requested with
`modalities: ["audio"]`. [Learn more](https://developers.openai.com/docs/guides/audio).

format: "wav" or "aac" or "mp3" or 3 more

Specifies the output audio format. Must be one of `wav`, `mp3`, `flac`,
`opus`, or `pcm16`.

One of the following:

"wav"

"aac"

"mp3"

"flac"

"opus"

"pcm16"

voice: string or "alloy" or "ash" or "ballad" or 7 more or object { id }

The voice the model uses to respond. Supported built-in voices are
`alloy`, `ash`, `ballad`, `coral`, `echo`, `fable`, `nova`, `onyx`,
`sage`, `shimmer`, `marin`, and `cedar`. You may also provide a
custom voice object with an `id`, for example `{ "id": "voice_1234" }`.

One of the following:

string

"alloy" or "ash" or "ballad" or 7 more

One of the following:

"alloy"

"ash"

"ballad"

"coral"

"echo"

"sage"

"shimmer"

"verse"

"marin"

"cedar"

ID object { id }

Custom voice reference.

id: string

The custom voice ID, e.g. `voice_1234`.

ChatCompletionChunk object { id, choices, created, 5 more }

Represents a streamed chunk of a chat completion response returned
by the model, based on the provided input.
[Learn more](https://developers.openai.com/docs/guides/streaming-responses).

id: string

A unique identifier for the chat completion. Each chunk has the same ID.

choices: array of object { delta, finish\_reason, index, logprobs }

A list of chat completion choices. Can contain more than one elements if `n` is greater than 1. Can also be empty for the
last chunk if you set `stream_options: {"include_usage": true}`.

delta: object { content, function\_call, refusal, 2 more }

A chat completion delta generated by streamed model responses.

content: optional string

The contents of the chunk message.

Deprecatedfunction\_call: optional object { arguments, name }

Deprecated and replaced by `tool_calls`. The name and arguments of a function that should be called, as generated by the model.

arguments: optional string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: optional string

The name of the function to call.

refusal: optional string

The refusal message generated by the model.

role: optional "developer" or "system" or "user" or 2 more

The role of the author of this message.

One of the following:

"developer"

"system"

"user"

"assistant"

"tool"

tool\_calls: optional array of object { index, id, function, type }

index: number

id: optional string

The ID of the tool call.

function: optional object { arguments, name }

arguments: optional string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: optional string

The name of the function to call.

type: optional "function"

The type of the tool. Currently, only `function` is supported.

finish\_reason: "stop" or "length" or "tool\_calls" or 2 more

The reason the model stopped generating tokens. This will be `stop` if the model hit a natural stop point or a provided stop sequence,
`length` if the maximum number of tokens specified in the request was reached,
`content_filter` if content was omitted due to a flag from our content filters,
`tool_calls` if the model called a tool, or `function_call` (deprecated) if the model called a function.

One of the following:

"stop"

"length"

"tool\_calls"

"content\_filter"

"function\_call"

index: number

The index of the choice in the list of choices.

logprobs: optional object { content, refusal }

Log probability information for the choice.

content: array of [ChatCompletionTokenLogprob](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)) { token, bytes, logprob, top\_logprobs }

A list of message content tokens with log probability information.

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

top\_logprobs: array of object { token, bytes, logprob }

List of the most likely tokens and their log probability, at this token position. In rare cases, there may be fewer than the number of requested `top_logprobs` returned.

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

refusal: array of [ChatCompletionTokenLogprob](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_token_logprob%20%3E%20(schema)) { token, bytes, logprob, top\_logprobs }

A list of message refusal tokens with log probability information.

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

top\_logprobs: array of object { token, bytes, logprob }

List of the most likely tokens and their log probability, at this token position. In rare cases, there may be fewer than the number of requested `top_logprobs` returned.

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

created: number

The Unix timestamp (in seconds) of when the chat completion was created. Each chunk has the same timestamp.

model: string

The model to generate the completion.

object: "chat.completion.chunk"

The object type, which is always `chat.completion.chunk`.

service\_tier: optional "auto" or "default" or "flex" or 2 more

Specifies the processing type used for serving the request.

- If set to ‘auto’, then the request will be processed with the service tier configured in the Project settings. Unless otherwise configured, the Project will use ‘default’.
- If set to ‘default’, then the request will be processed with the standard pricing and performance for the selected model.
- If set to ‘[flex](https://developers.openai.com/docs/guides/flex-processing)’ or ‘[priority](https://openai.com/api-priority-processing/)’, then the request will be processed with the corresponding service tier.
- When not set, the default behavior is ‘auto’.

When the `service_tier` parameter is set, the response body will include the `service_tier` value based on the processing mode actually used to serve the request. This response value may be different from the value set in the parameter.

One of the following:

"auto"

"default"

"flex"

"scale"

"priority"

Deprecatedsystem\_fingerprint: optional string

This fingerprint represents the backend configuration that the model runs with.
Can be used in conjunction with the `seed` request parameter to understand when backend changes have been made that might impact determinism.

usage: optional [CompletionUsage](https://developers.openai.com/api/reference/resources/completions#(resource)%20completions%20%3E%20(model)%20completion_usage%20%3E%20(schema)) { completion\_tokens, prompt\_tokens, total\_tokens, 2 more }

An optional field that will only be present when you set
`stream_options: {"include_usage": true}` in your request. When present, it
contains a null value **except for the last chunk** which contains the
token usage statistics for the entire request.

**NOTE:** If the stream is interrupted or cancelled, you may not
receive the final usage chunk which contains the total token usage for
the request.

ChatCompletionContentPart = [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }  or [ChatCompletionContentPartImage](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema)) { image\_url, type }  or [ChatCompletionContentPartInputAudio](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_input_audio%20%3E%20(schema)) { input\_audio, type }  or object { file, type }

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

One of the following:

ChatCompletionContentPartText object { text, type }

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

text: string

The text content.

type: "text"

The type of the content part.

ChatCompletionContentPartImage object { image\_url, type }

Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

image\_url: object { url, detail }

url: string

Either a URL of the image or the base64 encoded image data.

formaturi

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image. Learn more in the [Vision guide](https://developers.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).

One of the following:

"auto"

"low"

"high"

type: "image\_url"

The type of the content part.

ChatCompletionContentPartInputAudio object { input\_audio, type }

Learn about [audio inputs](https://developers.openai.com/docs/guides/audio).

input\_audio: object { data, format }

data: string

Base64 encoded audio data.

format: "wav" or "mp3"

The format of the encoded audio data. Currently supports “wav” and “mp3”.

One of the following:

"wav"

"mp3"

type: "input\_audio"

The type of the content part. Always `input_audio`.

FileContentPart object { file, type }

Learn about [file inputs](https://developers.openai.com/docs/guides/text) for text generation.

file: object { file\_data, file\_id, filename }

file\_data: optional string

The base64 encoded file data, used when passing the file to the model
as a string.

file\_id: optional string

The ID of an uploaded file to use as input.

filename: optional string

The name of the file, used when passing the file to the model as a
string.

type: "file"

The type of the content part. Always `file`.

ChatCompletionContentPartImage object { image\_url, type }

Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

image\_url: object { url, detail }

url: string

Either a URL of the image or the base64 encoded image data.

formaturi

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image. Learn more in the [Vision guide](https://developers.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).

One of the following:

"auto"

"low"

"high"

type: "image\_url"

The type of the content part.

ChatCompletionContentPartInputAudio object { input\_audio, type }

Learn about [audio inputs](https://developers.openai.com/docs/guides/audio).

input\_audio: object { data, format }

data: string

Base64 encoded audio data.

format: "wav" or "mp3"

The format of the encoded audio data. Currently supports “wav” and “mp3”.

One of the following:

"wav"

"mp3"

type: "input\_audio"

The type of the content part. Always `input_audio`.

ChatCompletionContentPartRefusal object { refusal, type }

refusal: string

The refusal message generated by the model.

type: "refusal"

The type of the content part.

ChatCompletionContentPartText object { text, type }

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

text: string

The text content.

type: "text"

The type of the content part.

ChatCompletionCustomTool object { custom, type }

A custom tool that processes input using a specified format.

custom: object { name, description, format }

Properties of the custom tool.

name: string

The name of the custom tool, used to identify it in tool calls.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional object { type }  or object { grammar, type }

The input format for the custom tool. Default is unconstrained text.

One of the following:

TextFormat object { type }

Unconstrained free-form text.

type: "text"

Unconstrained text format. Always `text`.

GrammarFormat object { grammar, type }

A grammar defined by the user.

grammar: object { definition, syntax }

Your chosen grammar.

definition: string

The grammar definition.

syntax: "lark" or "regex"

The syntax of the grammar definition. One of `lark` or `regex`.

One of the following:

"lark"

"regex"

type: "grammar"

Grammar format. Always `grammar`.

type: "custom"

The type of the custom tool. Always `custom`.

ChatCompletionDeleted object { id, deleted, object }

id: string

The ID of the chat completion that was deleted.

deleted: boolean

Whether the chat completion was deleted.

object: "chat.completion.deleted"

The type of object being deleted.

ChatCompletionDeveloperMessageParam object { content, role, name }

Developer-provided instructions that the model should follow, regardless of
messages sent by the user. With o1 models and newer, `developer` messages
replace the previous `system` messages.

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

The contents of the developer message.

One of the following:

TextContent = string

The contents of the developer message.

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

An array of content parts with a defined type. For developer messages, only type `text` is supported.

text: string

The text content.

type: "text"

The type of the content part.

role: "developer"

The role of the messages author, in this case `developer`.

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

ChatCompletionFunctionCallOption object { name }

Specifying a particular function via `{"name": "my_function"}` forces the model to call that function.

name: string

The name of the function to call.

ChatCompletionFunctionMessageParam object { content, name, role }

content: string

The contents of the function message.

name: string

The name of the function to call.

role: "function"

The role of the messages author, in this case `function`.

ChatCompletionFunctionTool object { function, type }

A function tool that can be used to generate a response.

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of the tool. Currently, only `function` is supported.

ChatCompletionMessage object { content, refusal, role, 4 more }

A chat completion message generated by the model.

content: string

The contents of the message.

refusal: string

The refusal message generated by the model.

role: "assistant"

The role of the author of this message.

annotations: optional array of object { type, url\_citation }

Annotations for the message, when applicable, as when using the
[web search tool](https://developers.openai.com/docs/guides/tools-web-search?api-mode=chat).

type: "url\_citation"

The type of the URL citation. Always `url_citation`.

url\_citation: object { end\_index, start\_index, title, url }

A URL citation when using web search.

end\_index: number

The index of the last character of the URL citation in the message.

start\_index: number

The index of the first character of the URL citation in the message.

title: string

The title of the web resource.

url: string

The URL of the web resource.

audio: optional [ChatCompletionAudio](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_audio%20%3E%20(schema)) { id, data, expires\_at, transcript }

If the audio output modality is requested, this object contains data
about the audio response from the model. [Learn more](https://developers.openai.com/docs/guides/audio).

Deprecatedfunction\_call: optional object { arguments, name }

Deprecated and replaced by `tool_calls`. The name and arguments of a function that should be called, as generated by the model.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: string

The name of the function to call.

tool\_calls: optional array of [ChatCompletionMessageToolCall](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_tool_call%20%3E%20(schema))

The tool calls generated by the model, such as function calls.

One of the following:

ChatCompletionMessageFunctionToolCall object { id, function, type }

A call to a function tool created by the model.

id: string

The ID of the tool call.

function: object { arguments, name }

The function that the model called.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: string

The name of the function to call.

type: "function"

The type of the tool. Currently, only `function` is supported.

ChatCompletionMessageCustomToolCall object { id, custom, type }

A call to a custom tool created by the model.

id: string

The ID of the tool call.

custom: object { input, name }

The custom tool that the model called.

input: string

The input for the custom tool call generated by the model.

name: string

The name of the custom tool to call.

type: "custom"

The type of the tool. Always `custom`.

ChatCompletionMessageCustomToolCall object { id, custom, type }

A call to a custom tool created by the model.

id: string

The ID of the tool call.

custom: object { input, name }

The custom tool that the model called.

input: string

The input for the custom tool call generated by the model.

name: string

The name of the custom tool to call.

type: "custom"

The type of the tool. Always `custom`.

ChatCompletionMessageFunctionToolCall object { id, function, type }

A call to a function tool created by the model.

id: string

The ID of the tool call.

function: object { arguments, name }

The function that the model called.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: string

The name of the function to call.

type: "function"

The type of the tool. Currently, only `function` is supported.

ChatCompletionMessageParam = [ChatCompletionDeveloperMessageParam](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_developer_message_param%20%3E%20(schema)) { content, role, name }  or [ChatCompletionSystemMessageParam](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_system_message_param%20%3E%20(schema)) { content, role, name }  or [ChatCompletionUserMessageParam](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_user_message_param%20%3E%20(schema)) { content, role, name }  or 3 more

Developer-provided instructions that the model should follow, regardless of
messages sent by the user. With o1 models and newer, `developer` messages
replace the previous `system` messages.

One of the following:

ChatCompletionDeveloperMessageParam object { content, role, name }

Developer-provided instructions that the model should follow, regardless of
messages sent by the user. With o1 models and newer, `developer` messages
replace the previous `system` messages.

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

The contents of the developer message.

One of the following:

TextContent = string

The contents of the developer message.

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

An array of content parts with a defined type. For developer messages, only type `text` is supported.

text: string

The text content.

type: "text"

The type of the content part.

role: "developer"

The role of the messages author, in this case `developer`.

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

ChatCompletionSystemMessageParam object { content, role, name }

Developer-provided instructions that the model should follow, regardless of
messages sent by the user. With o1 models and newer, use `developer` messages
for this purpose instead.

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

The contents of the system message.

One of the following:

TextContent = string

The contents of the system message.

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

An array of content parts with a defined type. For system messages, only type `text` is supported.

text: string

The text content.

type: "text"

The type of the content part.

role: "system"

The role of the messages author, in this case `system`.

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

ChatCompletionUserMessageParam object { content, role, name }

Messages sent by an end user, containing prompts or additional context
information.

content: string or array of [ChatCompletionContentPart](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema))

The contents of the user message.

One of the following:

TextContent = string

The text contents of the message.

ArrayOfContentParts = array of [ChatCompletionContentPart](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema))

An array of content parts with a defined type. Supported options differ based on the [model](https://developers.openai.com/docs/models) being used to generate the response. Can contain text, image, or audio inputs.

One of the following:

ChatCompletionContentPartText object { text, type }

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

text: string

The text content.

type: "text"

The type of the content part.

ChatCompletionContentPartImage object { image\_url, type }

Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

image\_url: object { url, detail }

url: string

Either a URL of the image or the base64 encoded image data.

formaturi

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image. Learn more in the [Vision guide](https://developers.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).

One of the following:

"auto"

"low"

"high"

type: "image\_url"

The type of the content part.

ChatCompletionContentPartInputAudio object { input\_audio, type }

Learn about [audio inputs](https://developers.openai.com/docs/guides/audio).

input\_audio: object { data, format }

data: string

Base64 encoded audio data.

format: "wav" or "mp3"

The format of the encoded audio data. Currently supports “wav” and “mp3”.

One of the following:

"wav"

"mp3"

type: "input\_audio"

The type of the content part. Always `input_audio`.

FileContentPart object { file, type }

Learn about [file inputs](https://developers.openai.com/docs/guides/text) for text generation.

file: object { file\_data, file\_id, filename }

file\_data: optional string

The base64 encoded file data, used when passing the file to the model
as a string.

file\_id: optional string

The ID of an uploaded file to use as input.

filename: optional string

The name of the file, used when passing the file to the model as a
string.

type: "file"

The type of the content part. Always `file`.

role: "user"

The role of the messages author, in this case `user`.

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

ChatCompletionAssistantMessageParam object { role, audio, content, 4 more }

Messages sent by the model in response to user messages.

role: "assistant"

The role of the messages author, in this case `assistant`.

audio: optional object { id }

Data about a previous audio response from the model.
[Learn more](https://developers.openai.com/docs/guides/audio).

id: string

Unique identifier for a previous audio response from the model.

content: optional string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }  or [ChatCompletionContentPartRefusal](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_refusal%20%3E%20(schema)) { refusal, type }

The contents of the assistant message. Required unless `tool_calls` or `function_call` is specified.

One of the following:

TextContent = string

The contents of the assistant message.

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }  or [ChatCompletionContentPartRefusal](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_refusal%20%3E%20(schema)) { refusal, type }

An array of content parts with a defined type. Can be one or more of type `text`, or exactly one of type `refusal`.

One of the following:

ChatCompletionContentPartText object { text, type }

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

text: string

The text content.

type: "text"

The type of the content part.

ChatCompletionContentPartRefusal object { refusal, type }

refusal: string

The refusal message generated by the model.

type: "refusal"

The type of the content part.

Deprecatedfunction\_call: optional object { arguments, name }

Deprecated and replaced by `tool_calls`. The name and arguments of a function that should be called, as generated by the model.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: string

The name of the function to call.

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

refusal: optional string

The refusal message by the assistant.

tool\_calls: optional array of [ChatCompletionMessageToolCall](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_tool_call%20%3E%20(schema))

The tool calls generated by the model, such as function calls.

One of the following:

ChatCompletionMessageFunctionToolCall object { id, function, type }

A call to a function tool created by the model.

id: string

The ID of the tool call.

function: object { arguments, name }

The function that the model called.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: string

The name of the function to call.

type: "function"

The type of the tool. Currently, only `function` is supported.

ChatCompletionMessageCustomToolCall object { id, custom, type }

A call to a custom tool created by the model.

id: string

The ID of the tool call.

custom: object { input, name }

The custom tool that the model called.

input: string

The input for the custom tool call generated by the model.

name: string

The name of the custom tool to call.

type: "custom"

The type of the tool. Always `custom`.

ChatCompletionToolMessageParam object { content, role, tool\_call\_id }

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

The contents of the tool message.

One of the following:

TextContent = string

The contents of the tool message.

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

An array of content parts with a defined type. For tool messages, only type `text` is supported.

text: string

The text content.

type: "text"

The type of the content part.

role: "tool"

The role of the messages author, in this case `tool`.

tool\_call\_id: string

Tool call that this message is responding to.

ChatCompletionFunctionMessageParam object { content, name, role }

content: string

The contents of the function message.

name: string

The name of the function to call.

role: "function"

The role of the messages author, in this case `function`.

ChatCompletionMessageToolCall = [ChatCompletionMessageFunctionToolCall](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_function_tool_call%20%3E%20(schema)) { id, function, type }  or [ChatCompletionMessageCustomToolCall](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message_custom_tool_call%20%3E%20(schema)) { id, custom, type }

A call to a function tool created by the model.

One of the following:

ChatCompletionMessageFunctionToolCall object { id, function, type }

A call to a function tool created by the model.

id: string

The ID of the tool call.

function: object { arguments, name }

The function that the model called.

arguments: string

The arguments to call the function with, as generated by the model in JSON format. Note that the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema. Validate the arguments in your code before calling your function.

name: string

The name of the function to call.

type: "function"

The type of the tool. Currently, only `function` is supported.

ChatCompletionMessageCustomToolCall object { id, custom, type }

A call to a custom tool created by the model.

id: string

The ID of the tool call.

custom: object { input, name }

The custom tool that the model called.

input: string

The input for the custom tool call generated by the model.

name: string

The name of the custom tool to call.

type: "custom"

The type of the tool. Always `custom`.

ChatCompletionModality = "text" or "audio"

One of the following:

"text"

"audio"

ChatCompletionNamedToolChoice object { function, type }

Specifies a tool the model should use. Use to force the model to call a specific function.

function: object { name }

name: string

The name of the function to call.

type: "function"

For function calling, the type is always `function`.

ChatCompletionNamedToolChoiceCustom object { custom, type }

Specifies a tool the model should use. Use to force the model to call a specific custom tool.

custom: object { name }

name: string

The name of the custom tool to call.

type: "custom"

For custom tool calling, the type is always `custom`.

ChatCompletionPredictionContent object { content, type }

Static predicted output content, such as the content of a text file that is
being regenerated.

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

The content that should be matched when generating a model response.
If generated tokens would match this content, the entire model response
can be returned much more quickly.

One of the following:

TextContent = string

The content used for a Predicted Output. This is often the
text of a file you are regenerating with minor changes.

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

An array of content parts with a defined type. Supported options differ based on the [model](https://developers.openai.com/docs/models) being used to generate the response. Can contain text inputs.

text: string

The text content.

type: "text"

The type of the content part.

type: "content"

The type of the predicted content you want to provide. This type is
currently always `content`.

ChatCompletionRole = "developer" or "system" or "user" or 3 more

The role of the author of a message

One of the following:

"developer"

"system"

"user"

"assistant"

"tool"

"function"

ChatCompletionStoreMessage = [ChatCompletionMessage](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_message%20%3E%20(schema)) { content, refusal, role, 4 more }

A chat completion message generated by the model.

id: string

The identifier of the chat message.

content\_parts: optional array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }  or [ChatCompletionContentPartImage](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_image%20%3E%20(schema)) { image\_url, type }

If a content parts array was provided, this is an array of `text` and `image_url` parts.
Otherwise, null.

One of the following:

ChatCompletionContentPartText object { text, type }

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

text: string

The text content.

type: "text"

The type of the content part.

ChatCompletionContentPartImage object { image\_url, type }

Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

image\_url: object { url, detail }

url: string

Either a URL of the image or the base64 encoded image data.

formaturi

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image. Learn more in the [Vision guide](https://developers.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).

One of the following:

"auto"

"low"

"high"

type: "image\_url"

The type of the content part.

ChatCompletionStreamOptions object { include\_obfuscation, include\_usage }

Options for streaming response. Only set this when you set `stream: true`.

include\_obfuscation: optional boolean

When true, stream obfuscation will be enabled. Stream obfuscation adds
random characters to an `obfuscation` field on streaming delta events to
normalize payload sizes as a mitigation to certain side-channel attacks.
These obfuscation fields are included by default, but add a small amount
of overhead to the data stream. You can set `include_obfuscation` to
false to optimize for bandwidth if you trust the network links between
your application and the OpenAI API.

include\_usage: optional boolean

If set, an additional chunk will be streamed before the `data: [DONE]`
message. The `usage` field on this chunk shows the token usage statistics
for the entire request, and the `choices` field will always be an empty
array.

All other chunks will also include a `usage` field, but with a null
value. **NOTE:** If the stream is interrupted, you may not receive the
final usage chunk which contains the total token usage for the request.

ChatCompletionSystemMessageParam object { content, role, name }

Developer-provided instructions that the model should follow, regardless of
messages sent by the user. With o1 models and newer, use `developer` messages
for this purpose instead.

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

The contents of the system message.

One of the following:

TextContent = string

The contents of the system message.

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

An array of content parts with a defined type. For system messages, only type `text` is supported.

text: string

The text content.

type: "text"

The type of the content part.

role: "system"

The role of the messages author, in this case `system`.

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

ChatCompletionTokenLogprob object { token, bytes, logprob, top\_logprobs }

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

top\_logprobs: array of object { token, bytes, logprob }

List of the most likely tokens and their log probability, at this token position. In rare cases, there may be fewer than the number of requested `top_logprobs` returned.

token: string

The token.

bytes: array of number

A list of integers representing the UTF-8 bytes representation of the token. Useful in instances where characters are represented by multiple tokens and their byte representations must be combined to generate the correct text representation. Can be `null` if there is no bytes representation for the token.

logprob: number

The log probability of this token, if it is within the top 20 most likely tokens. Otherwise, the value `-9999.0` is used to signify that the token is very unlikely.

ChatCompletionTool = [ChatCompletionFunctionTool](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)) { function, type }  or [ChatCompletionCustomTool](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_custom_tool%20%3E%20(schema)) { custom, type }

A function tool that can be used to generate a response.

One of the following:

ChatCompletionFunctionTool object { function, type }

A function tool that can be used to generate a response.

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of the tool. Currently, only `function` is supported.

ChatCompletionCustomTool object { custom, type }

A custom tool that processes input using a specified format.

custom: object { name, description, format }

Properties of the custom tool.

name: string

The name of the custom tool, used to identify it in tool calls.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional object { type }  or object { grammar, type }

The input format for the custom tool. Default is unconstrained text.

One of the following:

TextFormat object { type }

Unconstrained free-form text.

type: "text"

Unconstrained text format. Always `text`.

GrammarFormat object { grammar, type }

A grammar defined by the user.

grammar: object { definition, syntax }

Your chosen grammar.

definition: string

The grammar definition.

syntax: "lark" or "regex"

The syntax of the grammar definition. One of `lark` or `regex`.

One of the following:

"lark"

"regex"

type: "grammar"

Grammar format. Always `grammar`.

type: "custom"

The type of the custom tool. Always `custom`.

ChatCompletionToolChoiceOption = "none" or "auto" or "required" or [ChatCompletionAllowedToolChoice](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_allowed_tool_choice%20%3E%20(schema)) { allowed\_tools, type }  or [ChatCompletionNamedToolChoice](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice%20%3E%20(schema)) { function, type }  or [ChatCompletionNamedToolChoiceCustom](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_named_tool_choice_custom%20%3E%20(schema)) { custom, type }

Controls which (if any) tool is called by the model.
`none` means the model will not call any tool and instead generates a message.
`auto` means the model can pick between generating a message or calling one or more tools.
`required` means the model must call one or more tools.
Specifying a particular tool via `{"type": "function", "function": {"name": "my_function"}}` forces the model to call that tool.

`none` is the default when no tools are present. `auto` is the default if tools are present.

One of the following:

ToolChoiceMode = "none" or "auto" or "required"

`none` means the model will not call any tool and instead generates a message. `auto` means the model can pick between generating a message or calling one or more tools. `required` means the model must call one or more tools.

One of the following:

"none"

"auto"

"required"

ChatCompletionAllowedToolChoice object { allowed\_tools, type }

Constrains the tools available to the model to a pre-defined set.

allowed\_tools: [ChatCompletionAllowedTools](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20ChatCompletionAllowedTools%20%3E%20(schema)) { mode, tools }

Constrains the tools available to the model to a pre-defined set.

type: "allowed\_tools"

Allowed tool configuration type. Always `allowed_tools`.

ChatCompletionNamedToolChoice object { function, type }

Specifies a tool the model should use. Use to force the model to call a specific function.

function: object { name }

name: string

The name of the function to call.

type: "function"

For function calling, the type is always `function`.

ChatCompletionNamedToolChoiceCustom object { custom, type }

Specifies a tool the model should use. Use to force the model to call a specific custom tool.

custom: object { name }

name: string

The name of the custom tool to call.

type: "custom"

For custom tool calling, the type is always `custom`.

ChatCompletionToolMessageParam object { content, role, tool\_call\_id }

content: string or array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

The contents of the tool message.

One of the following:

TextContent = string

The contents of the tool message.

ArrayOfContentParts = array of [ChatCompletionContentPartText](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part_text%20%3E%20(schema)) { text, type }

An array of content parts with a defined type. For tool messages, only type `text` is supported.

text: string

The text content.

type: "text"

The type of the content part.

role: "tool"

The role of the messages author, in this case `tool`.

tool\_call\_id: string

Tool call that this message is responding to.

ChatCompletionUserMessageParam object { content, role, name }

Messages sent by an end user, containing prompts or additional context
information.

content: string or array of [ChatCompletionContentPart](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema))

The contents of the user message.

One of the following:

TextContent = string

The text contents of the message.

ArrayOfContentParts = array of [ChatCompletionContentPart](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_content_part%20%3E%20(schema))

An array of content parts with a defined type. Supported options differ based on the [model](https://developers.openai.com/docs/models) being used to generate the response. Can contain text, image, or audio inputs.

One of the following:

ChatCompletionContentPartText object { text, type }

Learn about [text inputs](https://developers.openai.com/docs/guides/text-generation).

text: string

The text content.

type: "text"

The type of the content part.

ChatCompletionContentPartImage object { image\_url, type }

Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

image\_url: object { url, detail }

url: string

Either a URL of the image or the base64 encoded image data.

formaturi

detail: optional "auto" or "low" or "high"

Specifies the detail level of the image. Learn more in the [Vision guide](https://developers.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).

One of the following:

"auto"

"low"

"high"

type: "image\_url"

The type of the content part.

ChatCompletionContentPartInputAudio object { input\_audio, type }

Learn about [audio inputs](https://developers.openai.com/docs/guides/audio).

input\_audio: object { data, format }

data: string

Base64 encoded audio data.

format: "wav" or "mp3"

The format of the encoded audio data. Currently supports “wav” and “mp3”.

One of the following:

"wav"

"mp3"

type: "input\_audio"

The type of the content part. Always `input_audio`.

FileContentPart object { file, type }

Learn about [file inputs](https://developers.openai.com/docs/guides/text) for text generation.

file: object { file\_data, file\_id, filename }

file\_data: optional string

The base64 encoded file data, used when passing the file to the model
as a string.

file\_id: optional string

The ID of an uploaded file to use as input.

filename: optional string

The name of the file, used when passing the file to the model as a
string.

type: "file"

The type of the content part. Always `file`.

role: "user"

The role of the messages author, in this case `user`.

name: optional string

An optional name for the participant. Provides the model information to differentiate between participants of the same role.

ChatCompletionAllowedTools object { mode, tools }

Constrains the tools available to the model to a pre-defined set.

mode: "auto" or "required"

Constrains the tools available to the model to a pre-defined set.

`auto` allows the model to pick from among the allowed tools and generate a
message.

`required` requires the model to call one or more of the allowed tools.

One of the following:

"auto"

"required"

tools: array of map[unknown]

A list of tool definitions that the model should be allowed to call.

For the Chat Completions API, the list of tool definitions might look like:

```
[
  { "type": "function", "function": { "name": "get_weather" } },
  { "type": "function", "function": { "name": "get_time" } }
]
```

#### ChatCompletionsMessages

Given a list of messages comprising a conversation, the model will return a response.

##### [Get chat messages](https://developers.openai.com/api/reference/resources/chat/subresources/completions/subresources/messages/methods/list)

GET/chat/completions/{completion\_id}/messages
