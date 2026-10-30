# Graders

> Source: https://developers.openai.com/api/reference/resources/graders
> Fetched: 2026-04-23

#### GradersGrader Models

##### Models

GraderInputs = array of string or [ResponseInputText](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_text%20%3E%20(schema)) { text, type }  or object { text, type }  or 2 more

A list of inputs, each of which may be either an input text, output text, input
image, or input audio object.

One of the following:

TextInput = string

A text input to the model.

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

OutputText object { text, type }

A text output from the model.

text: string

The text output from the model.

type: "output\_text"

The type of the output text. Always `output_text`.

InputImage object { image\_url, type, detail }

An image input block used within EvalItem content arrays.

image\_url: string

The URL of the image input.

type: "input\_image"

The type of the image input. Always `input_image`.

detail: optional string

The detail level of the image to be sent to the model. One of `high`, `low`, or `auto`. Defaults to `auto`.

ResponseInputAudio object { input\_audio, type }

An audio input to the model.

input\_audio: object { data, format }

data: string

Base64-encoded audio data.

format: "mp3" or "wav"

The format of the audio data. Currently supported formats are `mp3` and
`wav`.

One of the following:

"mp3"

"wav"

type: "input\_audio"

The type of the input item. Always `input_audio`.

LabelModelGrader object { input, labels, model, 3 more }

A LabelModelGrader object which uses a model to assign labels to each item
in the evaluation.

input: array of object { content, role, type }

content: string or [ResponseInputText](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_text%20%3E%20(schema)) { text, type }  or object { text, type }  or 3 more

Inputs to the model - can contain template strings. Supports text, output text, input images, and input audio, either as a single item or an array of items.

One of the following:

TextInput = string

A text input to the model.

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

OutputText object { text, type }

A text output from the model.

text: string

The text output from the model.

type: "output\_text"

The type of the output text. Always `output_text`.

InputImage object { image\_url, type, detail }

An image input block used within EvalItem content arrays.

image\_url: string

The URL of the image input.

type: "input\_image"

The type of the image input. Always `input_image`.

detail: optional string

The detail level of the image to be sent to the model. One of `high`, `low`, or `auto`. Defaults to `auto`.

ResponseInputAudio object { input\_audio, type }

An audio input to the model.

input\_audio: object { data, format }

data: string

Base64-encoded audio data.

format: "mp3" or "wav"

The format of the audio data. Currently supported formats are `mp3` and
`wav`.

One of the following:

"mp3"

"wav"

type: "input\_audio"

The type of the input item. Always `input_audio`.

GraderInputs = array of string or [ResponseInputText](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_text%20%3E%20(schema)) { text, type }  or object { text, type }  or 2 more

A list of inputs, each of which may be either an input text, output text, input
image, or input audio object.

One of the following:

TextInput = string

A text input to the model.

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

OutputText object { text, type }

A text output from the model.

text: string

The text output from the model.

type: "output\_text"

The type of the output text. Always `output_text`.

InputImage object { image\_url, type, detail }

An image input block used within EvalItem content arrays.

image\_url: string

The URL of the image input.

type: "input\_image"

The type of the image input. Always `input_image`.

detail: optional string

The detail level of the image to be sent to the model. One of `high`, `low`, or `auto`. Defaults to `auto`.

ResponseInputAudio object { input\_audio, type }

An audio input to the model.

input\_audio: object { data, format }

data: string

Base64-encoded audio data.

format: "mp3" or "wav"

The format of the audio data. Currently supported formats are `mp3` and
`wav`.

One of the following:

"mp3"

"wav"

type: "input\_audio"

The type of the input item. Always `input_audio`.

role: "user" or "assistant" or "system" or "developer"

The role of the message input. One of `user`, `assistant`, `system`, or
`developer`.

One of the following:

"user"

"assistant"

"system"

"developer"

type: optional "message"

The type of the message input. Always `message`.

labels: array of string

The labels to assign to each item in the evaluation.

model: string

The model to use for the evaluation. Must support structured outputs.

name: string

The name of the grader.

passing\_labels: array of string

The labels that indicate a passing result. Must be a subset of labels.

type: "label\_model"

The object type, which is always `label_model`.

MultiGrader object { calculate\_output, graders, name, type }

A MultiGrader object combines the output of multiple graders to produce a single score.

calculate\_output: string

A formula to calculate the output based on grader results.

graders: [StringCheckGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20string_check_grader%20%3E%20(schema)) { input, name, operation, 2 more }  or [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }  or [PythonGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20python_grader%20%3E%20(schema)) { name, source, type, image\_tag }  or 2 more

A StringCheckGrader object that performs a string comparison between input and reference using a specified operation.

One of the following:

StringCheckGrader object { input, name, operation, 2 more }

A StringCheckGrader object that performs a string comparison between input and reference using a specified operation.

input: string

The input text. This may include template strings.

name: string

The name of the grader.

operation: "eq" or "ne" or "like" or "ilike"

The string check operation to perform. One of `eq`, `ne`, `like`, or `ilike`.

One of the following:

"eq"

"ne"

"like"

"ilike"

reference: string

The reference text. This may include template strings.

type: "string\_check"

The object type, which is always `string_check`.

TextSimilarityGrader object { evaluation\_metric, input, name, 2 more }

A TextSimilarityGrader object which grades text based on similarity metrics.

evaluation\_metric: "cosine" or "fuzzy\_match" or "bleu" or 8 more

The evaluation metric to use. One of `cosine`, `fuzzy_match`, `bleu`,
`gleu`, `meteor`, `rouge_1`, `rouge_2`, `rouge_3`, `rouge_4`, `rouge_5`,
or `rouge_l`.

One of the following:

"cosine"

"fuzzy\_match"

"bleu"

"gleu"

"meteor"

"rouge\_1"

"rouge\_2"

"rouge\_3"

"rouge\_4"

"rouge\_5"

"rouge\_l"

input: string

The text being graded.

name: string

The name of the grader.

reference: string

The text being graded against.

type: "text\_similarity"

The type of grader.

PythonGrader object { name, source, type, image\_tag }

A PythonGrader object that runs a python script on the input.

name: string

The name of the grader.

source: string

The source code of the python script.

type: "python"

The object type, which is always `python`.

image\_tag: optional string

The image tag to use for the python script.

ScoreModelGrader object { input, model, name, 3 more }

A ScoreModelGrader object that uses a model to assign a score to the input.

input: array of object { content, role, type }

The input messages evaluated by the grader. Supports text, output text, input image, and input audio content blocks, and may include template strings.

content: string or [ResponseInputText](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_text%20%3E%20(schema)) { text, type }  or object { text, type }  or 3 more

Inputs to the model - can contain template strings. Supports text, output text, input images, and input audio, either as a single item or an array of items.

One of the following:

TextInput = string

A text input to the model.

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

OutputText object { text, type }

A text output from the model.

text: string

The text output from the model.

type: "output\_text"

The type of the output text. Always `output_text`.

InputImage object { image\_url, type, detail }

An image input block used within EvalItem content arrays.

image\_url: string

The URL of the image input.

type: "input\_image"

The type of the image input. Always `input_image`.

detail: optional string

The detail level of the image to be sent to the model. One of `high`, `low`, or `auto`. Defaults to `auto`.

ResponseInputAudio object { input\_audio, type }

An audio input to the model.

input\_audio: object { data, format }

data: string

Base64-encoded audio data.

format: "mp3" or "wav"

The format of the audio data. Currently supported formats are `mp3` and
`wav`.

One of the following:

"mp3"

"wav"

type: "input\_audio"

The type of the input item. Always `input_audio`.

GraderInputs = array of string or [ResponseInputText](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_text%20%3E%20(schema)) { text, type }  or object { text, type }  or 2 more

A list of inputs, each of which may be either an input text, output text, input
image, or input audio object.

One of the following:

TextInput = string

A text input to the model.

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

OutputText object { text, type }

A text output from the model.

text: string

The text output from the model.

type: "output\_text"

The type of the output text. Always `output_text`.

InputImage object { image\_url, type, detail }

An image input block used within EvalItem content arrays.

image\_url: string

The URL of the image input.

type: "input\_image"

The type of the image input. Always `input_image`.

detail: optional string

The detail level of the image to be sent to the model. One of `high`, `low`, or `auto`. Defaults to `auto`.

ResponseInputAudio object { input\_audio, type }

An audio input to the model.

input\_audio: object { data, format }

data: string

Base64-encoded audio data.

format: "mp3" or "wav"

The format of the audio data. Currently supported formats are `mp3` and
`wav`.

One of the following:

"mp3"

"wav"

type: "input\_audio"

The type of the input item. Always `input_audio`.

role: "user" or "assistant" or "system" or "developer"

The role of the message input. One of `user`, `assistant`, `system`, or
`developer`.

One of the following:

"user"

"assistant"

"system"

"developer"

type: optional "message"

The type of the message input. Always `message`.

model: string

The model to use for the evaluation.

name: string

The name of the grader.

type: "score\_model"

The object type, which is always `score_model`.

range: optional array of number

The range of the score. Defaults to `[0, 1]`.

sampling\_params: optional object { max\_completions\_tokens, reasoning\_effort, seed, 2 more }

The sampling parameters for the model.

max\_completions\_tokens: optional number

The maximum number of tokens the grader model may generate in its response.

minimum1

reasoning\_effort: optional [ReasoningEffort](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20reasoning_effort%20%3E%20(schema))

Constrains effort on reasoning for
[reasoning models](https://platform.openai.com/docs/guides/reasoning).
Currently supported values are `none`, `minimal`, `low`, `medium`, `high`, and `xhigh`. Reducing
reasoning effort can result in faster responses and fewer tokens used
on reasoning in a response.

- `gpt-5.1` defaults to `none`, which does not perform reasoning. The supported reasoning values for `gpt-5.1` are `none`, `low`, `medium`, and `high`. Tool calls are supported for all reasoning values in gpt-5.1.
- All models before `gpt-5.1` default to `medium` reasoning effort, and do not support `none`.
- The `gpt-5-pro` model defaults to (and only supports) `high` reasoning effort.
- `xhigh` is supported for all models after `gpt-5.1-codex-max`.

seed: optional number

A seed value to initialize the randomness, during sampling.

temperature: optional number

A higher temperature increases randomness in the outputs.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

LabelModelGrader object { input, labels, model, 3 more }

A LabelModelGrader object which uses a model to assign labels to each item
in the evaluation.

input: array of object { content, role, type }

content: string or [ResponseInputText](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_text%20%3E%20(schema)) { text, type }  or object { text, type }  or 3 more

Inputs to the model - can contain template strings. Supports text, output text, input images, and input audio, either as a single item or an array of items.

One of the following:

TextInput = string

A text input to the model.

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

OutputText object { text, type }

A text output from the model.

text: string

The text output from the model.

type: "output\_text"

The type of the output text. Always `output_text`.

InputImage object { image\_url, type, detail }

An image input block used within EvalItem content arrays.

image\_url: string

The URL of the image input.

type: "input\_image"

The type of the image input. Always `input_image`.

detail: optional string

The detail level of the image to be sent to the model. One of `high`, `low`, or `auto`. Defaults to `auto`.

ResponseInputAudio object { input\_audio, type }

An audio input to the model.

input\_audio: object { data, format }

data: string

Base64-encoded audio data.

format: "mp3" or "wav"

The format of the audio data. Currently supported formats are `mp3` and
`wav`.

One of the following:

"mp3"

"wav"

type: "input\_audio"

The type of the input item. Always `input_audio`.

GraderInputs = array of string or [ResponseInputText](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_text%20%3E%20(schema)) { text, type }  or object { text, type }  or 2 more

A list of inputs, each of which may be either an input text, output text, input
image, or input audio object.

One of the following:

TextInput = string

A text input to the model.

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

OutputText object { text, type }

A text output from the model.

text: string

The text output from the model.

type: "output\_text"

The type of the output text. Always `output_text`.

InputImage object { image\_url, type, detail }

An image input block used within EvalItem content arrays.

image\_url: string

The URL of the image input.

type: "input\_image"

The type of the image input. Always `input_image`.

detail: optional string

The detail level of the image to be sent to the model. One of `high`, `low`, or `auto`. Defaults to `auto`.

ResponseInputAudio object { input\_audio, type }

An audio input to the model.

input\_audio: object { data, format }

data: string

Base64-encoded audio data.

format: "mp3" or "wav"

The format of the audio data. Currently supported formats are `mp3` and
`wav`.

One of the following:

"mp3"

"wav"

type: "input\_audio"

The type of the input item. Always `input_audio`.

role: "user" or "assistant" or "system" or "developer"

The role of the message input. One of `user`, `assistant`, `system`, or
`developer`.

One of the following:

"user"

"assistant"

"system"

"developer"

type: optional "message"

The type of the message input. Always `message`.

labels: array of string

The labels to assign to each item in the evaluation.

model: string

The model to use for the evaluation. Must support structured outputs.

name: string

The name of the grader.

passing\_labels: array of string

The labels that indicate a passing result. Must be a subset of labels.

type: "label\_model"

The object type, which is always `label_model`.

name: string

The name of the grader.

type: "multi"

The object type, which is always `multi`.

PythonGrader object { name, source, type, image\_tag }

A PythonGrader object that runs a python script on the input.

name: string

The name of the grader.

source: string

The source code of the python script.

type: "python"

The object type, which is always `python`.

image\_tag: optional string

The image tag to use for the python script.

ScoreModelGrader object { input, model, name, 3 more }

A ScoreModelGrader object that uses a model to assign a score to the input.

input: array of object { content, role, type }

The input messages evaluated by the grader. Supports text, output text, input image, and input audio content blocks, and may include template strings.

content: string or [ResponseInputText](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_text%20%3E%20(schema)) { text, type }  or object { text, type }  or 3 more

Inputs to the model - can contain template strings. Supports text, output text, input images, and input audio, either as a single item or an array of items.

One of the following:

TextInput = string

A text input to the model.

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

OutputText object { text, type }

A text output from the model.

text: string

The text output from the model.

type: "output\_text"

The type of the output text. Always `output_text`.

InputImage object { image\_url, type, detail }

An image input block used within EvalItem content arrays.

image\_url: string

The URL of the image input.

type: "input\_image"

The type of the image input. Always `input_image`.

detail: optional string

The detail level of the image to be sent to the model. One of `high`, `low`, or `auto`. Defaults to `auto`.

ResponseInputAudio object { input\_audio, type }

An audio input to the model.

input\_audio: object { data, format }

data: string

Base64-encoded audio data.

format: "mp3" or "wav"

The format of the audio data. Currently supported formats are `mp3` and
`wav`.

One of the following:

"mp3"

"wav"

type: "input\_audio"

The type of the input item. Always `input_audio`.

GraderInputs = array of string or [ResponseInputText](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_text%20%3E%20(schema)) { text, type }  or object { text, type }  or 2 more

A list of inputs, each of which may be either an input text, output text, input
image, or input audio object.

One of the following:

TextInput = string

A text input to the model.

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

OutputText object { text, type }

A text output from the model.

text: string

The text output from the model.

type: "output\_text"

The type of the output text. Always `output_text`.

InputImage object { image\_url, type, detail }

An image input block used within EvalItem content arrays.

image\_url: string

The URL of the image input.

type: "input\_image"

The type of the image input. Always `input_image`.

detail: optional string

The detail level of the image to be sent to the model. One of `high`, `low`, or `auto`. Defaults to `auto`.

ResponseInputAudio object { input\_audio, type }

An audio input to the model.

input\_audio: object { data, format }

data: string

Base64-encoded audio data.

format: "mp3" or "wav"

The format of the audio data. Currently supported formats are `mp3` and
`wav`.

One of the following:

"mp3"

"wav"

type: "input\_audio"

The type of the input item. Always `input_audio`.

role: "user" or "assistant" or "system" or "developer"

The role of the message input. One of `user`, `assistant`, `system`, or
`developer`.

One of the following:

"user"

"assistant"

"system"

"developer"

type: optional "message"

The type of the message input. Always `message`.

model: string

The model to use for the evaluation.

name: string

The name of the grader.

type: "score\_model"

The object type, which is always `score_model`.

range: optional array of number

The range of the score. Defaults to `[0, 1]`.

sampling\_params: optional object { max\_completions\_tokens, reasoning\_effort, seed, 2 more }

The sampling parameters for the model.

max\_completions\_tokens: optional number

The maximum number of tokens the grader model may generate in its response.

minimum1

reasoning\_effort: optional [ReasoningEffort](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20reasoning_effort%20%3E%20(schema))

Constrains effort on reasoning for
[reasoning models](https://platform.openai.com/docs/guides/reasoning).
Currently supported values are `none`, `minimal`, `low`, `medium`, `high`, and `xhigh`. Reducing
reasoning effort can result in faster responses and fewer tokens used
on reasoning in a response.

- `gpt-5.1` defaults to `none`, which does not perform reasoning. The supported reasoning values for `gpt-5.1` are `none`, `low`, `medium`, and `high`. Tool calls are supported for all reasoning values in gpt-5.1.
- All models before `gpt-5.1` default to `medium` reasoning effort, and do not support `none`.
- The `gpt-5-pro` model defaults to (and only supports) `high` reasoning effort.
- `xhigh` is supported for all models after `gpt-5.1-codex-max`.

seed: optional number

A seed value to initialize the randomness, during sampling.

temperature: optional number

A higher temperature increases randomness in the outputs.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

StringCheckGrader object { input, name, operation, 2 more }

A StringCheckGrader object that performs a string comparison between input and reference using a specified operation.

input: string

The input text. This may include template strings.

name: string

The name of the grader.

operation: "eq" or "ne" or "like" or "ilike"

The string check operation to perform. One of `eq`, `ne`, `like`, or `ilike`.

One of the following:

"eq"

"ne"

"like"

"ilike"

reference: string

The reference text. This may include template strings.

type: "string\_check"

The object type, which is always `string_check`.

TextSimilarityGrader object { evaluation\_metric, input, name, 2 more }

A TextSimilarityGrader object which grades text based on similarity metrics.

evaluation\_metric: "cosine" or "fuzzy\_match" or "bleu" or 8 more

The evaluation metric to use. One of `cosine`, `fuzzy_match`, `bleu`,
`gleu`, `meteor`, `rouge_1`, `rouge_2`, `rouge_3`, `rouge_4`, `rouge_5`,
or `rouge_l`.

One of the following:

"cosine"

"fuzzy\_match"

"bleu"

"gleu"

"meteor"

"rouge\_1"

"rouge\_2"

"rouge\_3"

"rouge\_4"

"rouge\_5"

"rouge\_l"

input: string

The text being graded.

name: string

The name of the grader.

reference: string

The text being graded against.

type: "text\_similarity"

The type of grader.
