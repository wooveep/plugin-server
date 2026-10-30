# Evals

> Source: https://developers.openai.com/api/reference/resources/evals
> Fetched: 2026-04-23

Manage and run evals in the OpenAI platform.

##### [List evals](https://developers.openai.com/api/reference/resources/evals/methods/list)

GET/evals

##### [Create eval](https://developers.openai.com/api/reference/resources/evals/methods/create)

POST/evals

##### [Get an eval](https://developers.openai.com/api/reference/resources/evals/methods/retrieve)

GET/evals/{eval\_id}

##### [Update an eval](https://developers.openai.com/api/reference/resources/evals/methods/update)

POST/evals/{eval\_id}

##### [Delete an eval](https://developers.openai.com/api/reference/resources/evals/methods/delete)

DELETE/evals/{eval\_id}

##### Models

EvalCustomDataSourceConfig object { schema, type }

A CustomDataSourceConfig which specifies the schema of your `item` and optionally `sample` namespaces.
The response schema defines the shape of the data that will be:

- Used to define your testing criteria and
- What data is required when creating a run

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "custom"

The type of data source. Always `custom`.

EvalStoredCompletionsDataSourceConfig object { schema, type, metadata }

Deprecated in favor of LogsDataSourceConfig.

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "stored\_completions"

The type of data source. Always `stored_completions`.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

EvalListResponse object { id, created\_at, data\_source\_config, 4 more }

An Eval object with a data source config and testing criteria.
An Eval represents a task to be done for your LLM integration.
Like:

- Improve the quality of my chatbot
- See how well my chatbot handles customer support
- Check if o4-mini is better at my usecase than gpt-4o

id: string

Unique identifier for the evaluation.

created\_at: number

The Unix timestamp (in seconds) for when the eval was created.

data\_source\_config: [EvalCustomDataSourceConfig](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals%20%3E%20(model)%20eval_custom_data_source_config%20%3E%20(schema)) { schema, type }  or object { schema, type, metadata }  or [EvalStoredCompletionsDataSourceConfig](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals%20%3E%20(model)%20eval_stored_completions_data_source_config%20%3E%20(schema)) { schema, type, metadata }

Configuration of data sources used in runs of the evaluation.

One of the following:

EvalCustomDataSourceConfig object { schema, type }

A CustomDataSourceConfig which specifies the schema of your `item` and optionally `sample` namespaces.
The response schema defines the shape of the data that will be:

- Used to define your testing criteria and
- What data is required when creating a run

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "custom"

The type of data source. Always `custom`.

LogsDataSourceConfig object { schema, type, metadata }

A LogsDataSourceConfig which specifies the metadata property of your logs query.
This is usually metadata like `usecase=chatbot` or `prompt-version=v2`, etc.
The schema returned by this data source config is used to defined what variables are available in your evals.
`item` and `sample` are both defined when using this data source config.

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "logs"

The type of data source. Always `logs`.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

EvalStoredCompletionsDataSourceConfig object { schema, type, metadata }

Deprecated in favor of LogsDataSourceConfig.

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "stored\_completions"

The type of data source. Always `stored_completions`.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

name: string

The name of the evaluation.

object: "eval"

The object type.

testing\_criteria: array of [LabelModelGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20label_model_grader%20%3E%20(schema)) { input, labels, model, 3 more }  or [StringCheckGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20string_check_grader%20%3E%20(schema)) { input, name, operation, 2 more }  or [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }  or 2 more

A list of testing criteria.

One of the following:

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

TextSimilarityGrader = [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }

A TextSimilarityGrader object which grades text based on similarity metrics.

pass\_threshold: number

The threshold for the score.

PythonGrader = [PythonGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20python_grader%20%3E%20(schema)) { name, source, type, image\_tag }

A PythonGrader object that runs a python script on the input.

pass\_threshold: optional number

The threshold for the score.

ScoreModelGrader = [ScoreModelGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20score_model_grader%20%3E%20(schema)) { input, model, name, 3 more }

A ScoreModelGrader object that uses a model to assign a score to the input.

pass\_threshold: optional number

The threshold for the score.

EvalCreateResponse object { id, created\_at, data\_source\_config, 4 more }

An Eval object with a data source config and testing criteria.
An Eval represents a task to be done for your LLM integration.
Like:

- Improve the quality of my chatbot
- See how well my chatbot handles customer support
- Check if o4-mini is better at my usecase than gpt-4o

id: string

Unique identifier for the evaluation.

created\_at: number

The Unix timestamp (in seconds) for when the eval was created.

data\_source\_config: [EvalCustomDataSourceConfig](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals%20%3E%20(model)%20eval_custom_data_source_config%20%3E%20(schema)) { schema, type }  or object { schema, type, metadata }  or [EvalStoredCompletionsDataSourceConfig](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals%20%3E%20(model)%20eval_stored_completions_data_source_config%20%3E%20(schema)) { schema, type, metadata }

Configuration of data sources used in runs of the evaluation.

One of the following:

EvalCustomDataSourceConfig object { schema, type }

A CustomDataSourceConfig which specifies the schema of your `item` and optionally `sample` namespaces.
The response schema defines the shape of the data that will be:

- Used to define your testing criteria and
- What data is required when creating a run

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "custom"

The type of data source. Always `custom`.

LogsDataSourceConfig object { schema, type, metadata }

A LogsDataSourceConfig which specifies the metadata property of your logs query.
This is usually metadata like `usecase=chatbot` or `prompt-version=v2`, etc.
The schema returned by this data source config is used to defined what variables are available in your evals.
`item` and `sample` are both defined when using this data source config.

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "logs"

The type of data source. Always `logs`.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

EvalStoredCompletionsDataSourceConfig object { schema, type, metadata }

Deprecated in favor of LogsDataSourceConfig.

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "stored\_completions"

The type of data source. Always `stored_completions`.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

name: string

The name of the evaluation.

object: "eval"

The object type.

testing\_criteria: array of [LabelModelGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20label_model_grader%20%3E%20(schema)) { input, labels, model, 3 more }  or [StringCheckGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20string_check_grader%20%3E%20(schema)) { input, name, operation, 2 more }  or [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }  or 2 more

A list of testing criteria.

One of the following:

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

TextSimilarityGrader = [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }

A TextSimilarityGrader object which grades text based on similarity metrics.

pass\_threshold: number

The threshold for the score.

PythonGrader = [PythonGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20python_grader%20%3E%20(schema)) { name, source, type, image\_tag }

A PythonGrader object that runs a python script on the input.

pass\_threshold: optional number

The threshold for the score.

ScoreModelGrader = [ScoreModelGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20score_model_grader%20%3E%20(schema)) { input, model, name, 3 more }

A ScoreModelGrader object that uses a model to assign a score to the input.

pass\_threshold: optional number

The threshold for the score.

EvalRetrieveResponse object { id, created\_at, data\_source\_config, 4 more }

An Eval object with a data source config and testing criteria.
An Eval represents a task to be done for your LLM integration.
Like:

- Improve the quality of my chatbot
- See how well my chatbot handles customer support
- Check if o4-mini is better at my usecase than gpt-4o

id: string

Unique identifier for the evaluation.

created\_at: number

The Unix timestamp (in seconds) for when the eval was created.

data\_source\_config: [EvalCustomDataSourceConfig](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals%20%3E%20(model)%20eval_custom_data_source_config%20%3E%20(schema)) { schema, type }  or object { schema, type, metadata }  or [EvalStoredCompletionsDataSourceConfig](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals%20%3E%20(model)%20eval_stored_completions_data_source_config%20%3E%20(schema)) { schema, type, metadata }

Configuration of data sources used in runs of the evaluation.

One of the following:

EvalCustomDataSourceConfig object { schema, type }

A CustomDataSourceConfig which specifies the schema of your `item` and optionally `sample` namespaces.
The response schema defines the shape of the data that will be:

- Used to define your testing criteria and
- What data is required when creating a run

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "custom"

The type of data source. Always `custom`.

LogsDataSourceConfig object { schema, type, metadata }

A LogsDataSourceConfig which specifies the metadata property of your logs query.
This is usually metadata like `usecase=chatbot` or `prompt-version=v2`, etc.
The schema returned by this data source config is used to defined what variables are available in your evals.
`item` and `sample` are both defined when using this data source config.

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "logs"

The type of data source. Always `logs`.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

EvalStoredCompletionsDataSourceConfig object { schema, type, metadata }

Deprecated in favor of LogsDataSourceConfig.

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "stored\_completions"

The type of data source. Always `stored_completions`.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

name: string

The name of the evaluation.

object: "eval"

The object type.

testing\_criteria: array of [LabelModelGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20label_model_grader%20%3E%20(schema)) { input, labels, model, 3 more }  or [StringCheckGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20string_check_grader%20%3E%20(schema)) { input, name, operation, 2 more }  or [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }  or 2 more

A list of testing criteria.

One of the following:

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

TextSimilarityGrader = [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }

A TextSimilarityGrader object which grades text based on similarity metrics.

pass\_threshold: number

The threshold for the score.

PythonGrader = [PythonGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20python_grader%20%3E%20(schema)) { name, source, type, image\_tag }

A PythonGrader object that runs a python script on the input.

pass\_threshold: optional number

The threshold for the score.

ScoreModelGrader = [ScoreModelGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20score_model_grader%20%3E%20(schema)) { input, model, name, 3 more }

A ScoreModelGrader object that uses a model to assign a score to the input.

pass\_threshold: optional number

The threshold for the score.

EvalUpdateResponse object { id, created\_at, data\_source\_config, 4 more }

An Eval object with a data source config and testing criteria.
An Eval represents a task to be done for your LLM integration.
Like:

- Improve the quality of my chatbot
- See how well my chatbot handles customer support
- Check if o4-mini is better at my usecase than gpt-4o

id: string

Unique identifier for the evaluation.

created\_at: number

The Unix timestamp (in seconds) for when the eval was created.

data\_source\_config: [EvalCustomDataSourceConfig](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals%20%3E%20(model)%20eval_custom_data_source_config%20%3E%20(schema)) { schema, type }  or object { schema, type, metadata }  or [EvalStoredCompletionsDataSourceConfig](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals%20%3E%20(model)%20eval_stored_completions_data_source_config%20%3E%20(schema)) { schema, type, metadata }

Configuration of data sources used in runs of the evaluation.

One of the following:

EvalCustomDataSourceConfig object { schema, type }

A CustomDataSourceConfig which specifies the schema of your `item` and optionally `sample` namespaces.
The response schema defines the shape of the data that will be:

- Used to define your testing criteria and
- What data is required when creating a run

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "custom"

The type of data source. Always `custom`.

LogsDataSourceConfig object { schema, type, metadata }

A LogsDataSourceConfig which specifies the metadata property of your logs query.
This is usually metadata like `usecase=chatbot` or `prompt-version=v2`, etc.
The schema returned by this data source config is used to defined what variables are available in your evals.
`item` and `sample` are both defined when using this data source config.

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "logs"

The type of data source. Always `logs`.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

EvalStoredCompletionsDataSourceConfig object { schema, type, metadata }

Deprecated in favor of LogsDataSourceConfig.

schema: map[unknown]

The json schema for the run data source items.
Learn how to build JSON schemas [here](https://json-schema.org/).

type: "stored\_completions"

The type of data source. Always `stored_completions`.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

name: string

The name of the evaluation.

object: "eval"

The object type.

testing\_criteria: array of [LabelModelGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20label_model_grader%20%3E%20(schema)) { input, labels, model, 3 more }  or [StringCheckGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20string_check_grader%20%3E%20(schema)) { input, name, operation, 2 more }  or [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }  or 2 more

A list of testing criteria.

One of the following:

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

TextSimilarityGrader = [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }

A TextSimilarityGrader object which grades text based on similarity metrics.

pass\_threshold: number

The threshold for the score.

PythonGrader = [PythonGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20python_grader%20%3E%20(schema)) { name, source, type, image\_tag }

A PythonGrader object that runs a python script on the input.

pass\_threshold: optional number

The threshold for the score.

ScoreModelGrader = [ScoreModelGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20score_model_grader%20%3E%20(schema)) { input, model, name, 3 more }

A ScoreModelGrader object that uses a model to assign a score to the input.

pass\_threshold: optional number

The threshold for the score.

EvalDeleteResponse object { deleted, eval\_id, object }

deleted: boolean

eval\_id: string

object: string

#### EvalsRuns

Manage and run evals in the OpenAI platform.

##### [Get eval runs](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/list)

GET/evals/{eval\_id}/runs

##### [Create eval run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/create)

POST/evals/{eval\_id}/runs

##### [Get an eval run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/retrieve)

GET/evals/{eval\_id}/runs/{run\_id}

##### [Cancel eval run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/cancel)

POST/evals/{eval\_id}/runs/{run\_id}

##### [Delete eval run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/methods/delete)

DELETE/evals/{eval\_id}/runs/{run\_id}

##### Models

CreateEvalCompletionsRunDataSource object { source, type, input\_messages, 2 more }

A CompletionsRunDataSource object describing a model sampling configuration.

source: object { content, type }  or object { id, type }  or object { type, created\_after, created\_before, 3 more }

Determines what populates the `item` namespace in this run’s data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

StoredCompletionsRunDataSource object { type, created\_after, created\_before, 3 more }

A StoredCompletionsRunDataSource configuration describing a set of filters

type: "stored\_completions"

The type of source. Always `stored_completions`.

created\_after: optional number

An optional Unix timestamp to filter items created after this time.

created\_before: optional number

An optional Unix timestamp to filter items created before this time.

limit: optional number

An optional maximum number of items to return.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: optional string

An optional model to filter by (e.g., ‘gpt-4o’).

type: "completions"

The type of run data source. Always `completions`.

input\_messages: optional object { template, type }  or object { item\_reference, type }

Used when sampling from a model. Dictates the structure of the messages passed into the model. Can either be a reference to a prebuilt trajectory (ie, `item.input_trajectory`), or a template with variable references to the `item` namespace.

One of the following:

TemplateInputMessages object { template, type }

template: array of [EasyInputMessage](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20easy_input_message%20%3E%20(schema)) { content, role, phase, type }  or object { content, role, type }

A list of chat messages forming the prompt or context. May include variable references to the `item` namespace, ie {{item.name}}.

One of the following:

EasyInputMessage object { content, role, phase, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

content: string or [ResponseInputMessageContentList](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_message_content_list%20%3E%20(schema)) { , ,  }

Text, image, or audio input to the model, used to generate a response.
Can also contain previous assistant responses.

One of the following:

TextInput = string

A text input to the model.

ResponseInputMessageContentList = array of [ResponseInputContent](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_content%20%3E%20(schema))

A list of one or many input items to the model, containing different content
types.

One of the following:

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

ResponseInputImage object { detail, type, file\_id, image\_url }

An image input to the model. Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

detail: "low" or "high" or "auto" or "original"

The detail level of the image to be sent to the model. One of `high`, `low`, `auto`, or `original`. Defaults to `auto`.

One of the following:

"low"

"high"

"auto"

"original"

type: "input\_image"

The type of the input item. Always `input_image`.

file\_id: optional string

The ID of the file to be sent to the model.

image\_url: optional string

The URL of the image to be sent to the model. A fully qualified URL or base64 encoded image in a data URL.

ResponseInputFile object { type, detail, file\_data, 3 more }

A file input to the model.

type: "input\_file"

The type of the input item. Always `input_file`.

detail: optional "low" or "high"

The detail level of the file to be sent to the model. Use `low` for the default rendering behavior, or `high` to render the file at higher quality. Defaults to `low`.

One of the following:

"low"

"high"

file\_data: optional string

The content of the file to be sent to the model.

file\_id: optional string

The ID of the file to be sent to the model.

file\_url: optional string

The URL of the file to be sent to the model.

filename: optional string

The name of the file to be sent to the model.

role: "user" or "assistant" or "system" or "developer"

The role of the message input. One of `user`, `assistant`, `system`, or
`developer`.

One of the following:

"user"

"assistant"

"system"

"developer"

phase: optional "commentary" or "final\_answer"

Labels an `assistant` message as intermediate commentary (`commentary`) or the final answer (`final_answer`).
For models like `gpt-5.3-codex` and beyond, when sending follow-up requests, preserve and resend
phase on all assistant messages — dropping it can degrade performance. Not used for user messages.

One of the following:

"commentary"

"final\_answer"

type: optional "message"

The type of the message input. Always `message`.

EvalMessageObject object { content, role, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

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

type: "template"

The type of input messages. Always `template`.

ItemReferenceInputMessages object { item\_reference, type }

item\_reference: string

A reference to a variable in the `item` namespace. Ie, “item.input\_trajectory”

type: "item\_reference"

The type of input messages. Always `item_reference`.

model: optional string

The name of the model to use for generating completions (e.g. “o3-mini”).

sampling\_params: optional object { max\_completion\_tokens, reasoning\_effort, response\_format, 4 more }

max\_completion\_tokens: optional number

The maximum number of tokens in the generated output.

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

response\_format: optional [ResponseFormatText](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_text%20%3E%20(schema)) { type }  or [ResponseFormatJSONSchema](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)) { json\_schema, type }  or [ResponseFormatJSONObject](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_object%20%3E%20(schema)) { type }

An object specifying the format that the model must output.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables
Structured Outputs which ensures the model will match your supplied JSON
schema. Learn more in the [Structured Outputs
guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables the older JSON mode, which
ensures the message the model generates is valid JSON. Using `json_schema`
is preferred for models that support it.

One of the following:

ResponseFormatText object { type }

Default response format. Used to generate text responses.

type: "text"

The type of response format being defined. Always `text`.

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

ResponseFormatJSONObject object { type }

JSON object response format. An older method of generating JSON responses.
Using `json_schema` is recommended for models that support it. Note that the
model will not generate JSON without a system or user message instructing it
to do so.

type: "json\_object"

The type of response format being defined. Always `json_object`.

seed: optional number

A seed value to initialize the randomness, during sampling.

temperature: optional number

A higher temperature increases randomness in the outputs.

tools: optional array of [ChatCompletionFunctionTool](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)) { function, type }

A list of tools the model may call. Currently, only functions are supported as a tool. Use this to provide a list of functions the model may generate JSON inputs for. A max of 128 functions are supported.

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of the tool. Currently, only `function` is supported.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

CreateEvalJSONLRunDataSource object { source, type }

A JsonlRunDataSource object with that specifies a JSONL file that matches the eval

source: object { content, type }  or object { id, type }

Determines what populates the `item` namespace in the data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

type: "jsonl"

The type of data source. Always `jsonl`.

EvalAPIError object { code, message }

An object representing an error response from the Eval API.

code: string

The error code.

message: string

The error message.

RunListResponse object { id, created\_at, data\_source, 11 more }

A schema representing an evaluation run.

id: string

Unique identifier for the evaluation run.

created\_at: number

Unix timestamp (in seconds) when the evaluation run was created.

data\_source: [CreateEvalJSONLRunDataSource](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20create_eval_jsonl_run_data_source%20%3E%20(schema)) { source, type }  or [CreateEvalCompletionsRunDataSource](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20create_eval_completions_run_data_source%20%3E%20(schema)) { source, type, input\_messages, 2 more }  or object { source, type, input\_messages, 2 more }

Information about the run’s data source.

One of the following:

CreateEvalJSONLRunDataSource object { source, type }

A JsonlRunDataSource object with that specifies a JSONL file that matches the eval

source: object { content, type }  or object { id, type }

Determines what populates the `item` namespace in the data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

type: "jsonl"

The type of data source. Always `jsonl`.

CreateEvalCompletionsRunDataSource object { source, type, input\_messages, 2 more }

A CompletionsRunDataSource object describing a model sampling configuration.

source: object { content, type }  or object { id, type }  or object { type, created\_after, created\_before, 3 more }

Determines what populates the `item` namespace in this run’s data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

StoredCompletionsRunDataSource object { type, created\_after, created\_before, 3 more }

A StoredCompletionsRunDataSource configuration describing a set of filters

type: "stored\_completions"

The type of source. Always `stored_completions`.

created\_after: optional number

An optional Unix timestamp to filter items created after this time.

created\_before: optional number

An optional Unix timestamp to filter items created before this time.

limit: optional number

An optional maximum number of items to return.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: optional string

An optional model to filter by (e.g., ‘gpt-4o’).

type: "completions"

The type of run data source. Always `completions`.

input\_messages: optional object { template, type }  or object { item\_reference, type }

Used when sampling from a model. Dictates the structure of the messages passed into the model. Can either be a reference to a prebuilt trajectory (ie, `item.input_trajectory`), or a template with variable references to the `item` namespace.

One of the following:

TemplateInputMessages object { template, type }

template: array of [EasyInputMessage](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20easy_input_message%20%3E%20(schema)) { content, role, phase, type }  or object { content, role, type }

A list of chat messages forming the prompt or context. May include variable references to the `item` namespace, ie {{item.name}}.

One of the following:

EasyInputMessage object { content, role, phase, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

content: string or [ResponseInputMessageContentList](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_message_content_list%20%3E%20(schema)) { , ,  }

Text, image, or audio input to the model, used to generate a response.
Can also contain previous assistant responses.

One of the following:

TextInput = string

A text input to the model.

ResponseInputMessageContentList = array of [ResponseInputContent](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_content%20%3E%20(schema))

A list of one or many input items to the model, containing different content
types.

One of the following:

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

ResponseInputImage object { detail, type, file\_id, image\_url }

An image input to the model. Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

detail: "low" or "high" or "auto" or "original"

The detail level of the image to be sent to the model. One of `high`, `low`, `auto`, or `original`. Defaults to `auto`.

One of the following:

"low"

"high"

"auto"

"original"

type: "input\_image"

The type of the input item. Always `input_image`.

file\_id: optional string

The ID of the file to be sent to the model.

image\_url: optional string

The URL of the image to be sent to the model. A fully qualified URL or base64 encoded image in a data URL.

ResponseInputFile object { type, detail, file\_data, 3 more }

A file input to the model.

type: "input\_file"

The type of the input item. Always `input_file`.

detail: optional "low" or "high"

The detail level of the file to be sent to the model. Use `low` for the default rendering behavior, or `high` to render the file at higher quality. Defaults to `low`.

One of the following:

"low"

"high"

file\_data: optional string

The content of the file to be sent to the model.

file\_id: optional string

The ID of the file to be sent to the model.

file\_url: optional string

The URL of the file to be sent to the model.

filename: optional string

The name of the file to be sent to the model.

role: "user" or "assistant" or "system" or "developer"

The role of the message input. One of `user`, `assistant`, `system`, or
`developer`.

One of the following:

"user"

"assistant"

"system"

"developer"

phase: optional "commentary" or "final\_answer"

Labels an `assistant` message as intermediate commentary (`commentary`) or the final answer (`final_answer`).
For models like `gpt-5.3-codex` and beyond, when sending follow-up requests, preserve and resend
phase on all assistant messages — dropping it can degrade performance. Not used for user messages.

One of the following:

"commentary"

"final\_answer"

type: optional "message"

The type of the message input. Always `message`.

EvalMessageObject object { content, role, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

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

type: "template"

The type of input messages. Always `template`.

ItemReferenceInputMessages object { item\_reference, type }

item\_reference: string

A reference to a variable in the `item` namespace. Ie, “item.input\_trajectory”

type: "item\_reference"

The type of input messages. Always `item_reference`.

model: optional string

The name of the model to use for generating completions (e.g. “o3-mini”).

sampling\_params: optional object { max\_completion\_tokens, reasoning\_effort, response\_format, 4 more }

max\_completion\_tokens: optional number

The maximum number of tokens in the generated output.

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

response\_format: optional [ResponseFormatText](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_text%20%3E%20(schema)) { type }  or [ResponseFormatJSONSchema](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)) { json\_schema, type }  or [ResponseFormatJSONObject](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_object%20%3E%20(schema)) { type }

An object specifying the format that the model must output.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables
Structured Outputs which ensures the model will match your supplied JSON
schema. Learn more in the [Structured Outputs
guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables the older JSON mode, which
ensures the message the model generates is valid JSON. Using `json_schema`
is preferred for models that support it.

One of the following:

ResponseFormatText object { type }

Default response format. Used to generate text responses.

type: "text"

The type of response format being defined. Always `text`.

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

ResponseFormatJSONObject object { type }

JSON object response format. An older method of generating JSON responses.
Using `json_schema` is recommended for models that support it. Note that the
model will not generate JSON without a system or user message instructing it
to do so.

type: "json\_object"

The type of response format being defined. Always `json_object`.

seed: optional number

A seed value to initialize the randomness, during sampling.

temperature: optional number

A higher temperature increases randomness in the outputs.

tools: optional array of [ChatCompletionFunctionTool](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)) { function, type }

A list of tools the model may call. Currently, only functions are supported as a tool. Use this to provide a list of functions the model may generate JSON inputs for. A max of 128 functions are supported.

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of the tool. Currently, only `function` is supported.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

ResponsesRunDataSource object { source, type, input\_messages, 2 more }

A ResponsesRunDataSource object describing a model sampling configuration.

source: object { content, type }  or object { id, type }  or object { type, created\_after, created\_before, 8 more }

Determines what populates the `item` namespace in this run’s data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

EvalResponsesSource object { type, created\_after, created\_before, 8 more }

A EvalResponsesSource object describing a run data source configuration.

type: "responses"

The type of run data source. Always `responses`.

created\_after: optional number

Only include items created after this timestamp (inclusive). This is a query parameter used to select responses.

minimum0

created\_before: optional number

Only include items created before this timestamp (inclusive). This is a query parameter used to select responses.

minimum0

instructions\_search: optional string

Optional string to search the ‘instructions’ field. This is a query parameter used to select responses.

metadata: optional unknown

Metadata filter for the responses. This is a query parameter used to select responses.

model: optional string

The name of the model to find responses for. This is a query parameter used to select responses.

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

temperature: optional number

Sampling temperature. This is a query parameter used to select responses.

tools: optional array of string

List of tool names. This is a query parameter used to select responses.

top\_p: optional number

Nucleus sampling parameter. This is a query parameter used to select responses.

users: optional array of string

List of user identifiers. This is a query parameter used to select responses.

type: "responses"

The type of run data source. Always `responses`.

input\_messages: optional object { template, type }  or object { item\_reference, type }

Used when sampling from a model. Dictates the structure of the messages passed into the model. Can either be a reference to a prebuilt trajectory (ie, `item.input_trajectory`), or a template with variable references to the `item` namespace.

One of the following:

InputMessagesTemplate object { template, type }

template: array of object { content, role }  or object { content, role, type }

A list of chat messages forming the prompt or context. May include variable references to the `item` namespace, ie {{item.name}}.

One of the following:

ChatMessage object { content, role }

content: string

The content of the message.

role: string

The role of the message (e.g. “system”, “assistant”, “user”).

EvalMessageObject object { content, role, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

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

type: "template"

The type of input messages. Always `template`.

InputMessagesItemReference object { item\_reference, type }

item\_reference: string

A reference to a variable in the `item` namespace. Ie, “item.name”

type: "item\_reference"

The type of input messages. Always `item_reference`.

model: optional string

The name of the model to use for generating completions (e.g. “o3-mini”).

sampling\_params: optional object { max\_completion\_tokens, reasoning\_effort, seed, 4 more }

max\_completion\_tokens: optional number

The maximum number of tokens in the generated output.

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

text: optional object { format }

Configuration options for a text response from the model. Can be plain
text or structured JSON data. Learn more:

- [Text inputs and outputs](https://developers.openai.com/docs/guides/text)
- [Structured Outputs](https://developers.openai.com/docs/guides/structured-outputs)

format: optional [ResponseFormatTextConfig](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_format_text_config%20%3E%20(schema))

An object specifying the format that the model must output.

Configuring `{ "type": "json_schema" }` enables Structured Outputs,
which ensures the model will match your supplied JSON schema. Learn more in the
[Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

The default format is `{ "type": "text" }` with no additional options.

**Not recommended for gpt-4o and newer models:**

Setting to `{ "type": "json_object" }` enables the older JSON mode, which
ensures the message the model generates is valid JSON. Using `json_schema`
is preferred for models that support it.

tools: optional array of object { name, parameters, strict, 3 more }  or object { type, vector\_store\_ids, filters, 2 more }  or object { type }  or 12 more

An array of tools the model may call while generating a response. You
can specify which tool to use by setting the `tool_choice` parameter.

The two categories of tools you can provide the model are:

- **Built-in tools**: Tools that are provided by OpenAI that extend the
  model’s capabilities, like [web search](https://developers.openai.com/docs/guides/tools-web-search)
  or [file search](https://developers.openai.com/docs/guides/tools-file-search). Learn more about
  [built-in tools](https://developers.openai.com/docs/guides/tools).
- **Function calls (custom tools)**: Functions that are defined by you,
  enabling the model to call your own code. Learn more about
  [function calling](https://developers.openai.com/docs/guides/function-calling).

One of the following:

Function object { name, parameters, strict, 3 more }

Defines a function in your own code the model can choose to call. Learn more about [function calling](https://platform.openai.com/docs/guides/function-calling).

name: string

The name of the function to call.

parameters: map[unknown]

A JSON schema object describing the parameters of the function.

strict: boolean

Whether to enforce strict parameter validation. Default `true`.

type: "function"

The type of the function tool. Always `function`.

defer\_loading: optional boolean

Whether this function is deferred and loaded via tool search.

description: optional string

A description of the function. Used by the model to determine whether or not to call the function.

FileSearch object { type, vector\_store\_ids, filters, 2 more }

A tool that searches for relevant content from uploaded files. Learn more about the [file search tool](https://platform.openai.com/docs/guides/tools-file-search).

type: "file\_search"

The type of the file search tool. Always `file_search`.

vector\_store\_ids: array of string

The IDs of the vector stores to search.

filters: optional [ComparisonFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20comparison_filter%20%3E%20(schema)) { key, type, value }  or [CompoundFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20compound_filter%20%3E%20(schema)) { filters, type }

A filter to apply.

One of the following:

ComparisonFilter object { key, type, value }

A filter used to compare a specified attribute key to a given value using a defined comparison operation.

key: string

The key to compare against the value.

type: "eq" or "ne" or "gt" or 5 more

Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`.

- `eq`: equals
- `ne`: not equal
- `gt`: greater than
- `gte`: greater than or equal
- `lt`: less than
- `lte`: less than or equal
- `in`: in
- `nin`: not in

One of the following:

"eq"

"ne"

"gt"

"gte"

"lt"

"lte"

"in"

"nin"

value: string or number or boolean or array of string or number

The value to compare against the attribute key; supports string, number, or boolean types.

One of the following:

string

number

boolean

array of string or number

One of the following:

string

number

CompoundFilter object { filters, type }

Combine multiple filters using `and` or `or`.

filters: array of [ComparisonFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20comparison_filter%20%3E%20(schema)) { key, type, value }  or unknown

Array of filters to combine. Items can be `ComparisonFilter` or `CompoundFilter`.

One of the following:

ComparisonFilter object { key, type, value }

A filter used to compare a specified attribute key to a given value using a defined comparison operation.

key: string

The key to compare against the value.

type: "eq" or "ne" or "gt" or 5 more

Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`.

- `eq`: equals
- `ne`: not equal
- `gt`: greater than
- `gte`: greater than or equal
- `lt`: less than
- `lte`: less than or equal
- `in`: in
- `nin`: not in

One of the following:

"eq"

"ne"

"gt"

"gte"

"lt"

"lte"

"in"

"nin"

value: string or number or boolean or array of string or number

The value to compare against the attribute key; supports string, number, or boolean types.

One of the following:

string

number

boolean

array of string or number

One of the following:

string

number

unknown

type: "and" or "or"

Type of operation: `and` or `or`.

One of the following:

"and"

"or"

max\_num\_results: optional number

The maximum number of results to return. This number should be between 1 and 50 inclusive.

ranking\_options: optional object { hybrid\_search, ranker, score\_threshold }

Ranking options for search.

hybrid\_search: optional object { embedding\_weight, text\_weight }

Weights that control how reciprocal rank fusion balances semantic embedding matches versus sparse keyword matches when hybrid search is enabled.

embedding\_weight: number

The weight of the embedding in the reciprocal ranking fusion.

text\_weight: number

The weight of the text in the reciprocal ranking fusion.

ranker: optional "auto" or "default-2024-11-15"

The ranker to use for the file search.

One of the following:

"auto"

"default-2024-11-15"

score\_threshold: optional number

The score threshold for the file search, a number between 0 and 1. Numbers closer to 1 will attempt to return only the most relevant results, but may return fewer results.

Computer object { type }

A tool that controls a virtual computer. Learn more about the [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).

type: "computer"

The type of the computer tool. Always `computer`.

ComputerUsePreview object { display\_height, display\_width, environment, type }

A tool that controls a virtual computer. Learn more about the [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).

display\_height: number

The height of the computer display.

display\_width: number

The width of the computer display.

environment: "windows" or "mac" or "linux" or 2 more

The type of computer environment to control.

One of the following:

"windows"

"mac"

"linux"

"ubuntu"

"browser"

type: "computer\_use\_preview"

The type of the computer use tool. Always `computer_use_preview`.

WebSearch object { type, filters, search\_context\_size, user\_location }

Search the Internet for sources related to the prompt. Learn more about the
[web search tool](https://developers.openai.com/docs/guides/tools-web-search).

type: "web\_search" or "web\_search\_2025\_08\_26"

The type of the web search tool. One of `web_search` or `web_search_2025_08_26`.

One of the following:

"web\_search"

"web\_search\_2025\_08\_26"

filters: optional object { allowed\_domains }

Filters for the search.

allowed\_domains: optional array of string

Allowed domains for the search. If not provided, all domains are allowed.
Subdomains of the provided domains are allowed as well.

Example: `["pubmed.ncbi.nlm.nih.gov"]`

search\_context\_size: optional "low" or "medium" or "high"

High level guidance for the amount of context window space to use for the search. One of `low`, `medium`, or `high`. `medium` is the default.

One of the following:

"low"

"medium"

"high"

user\_location: optional object { city, country, region, 2 more }

The approximate location of the user.

city: optional string

Free text input for the city of the user, e.g. `San Francisco`.

country: optional string

The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of the user, e.g. `US`.

region: optional string

Free text input for the region of the user, e.g. `California`.

timezone: optional string

The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the user, e.g. `America/Los_Angeles`.

type: optional "approximate"

The type of location approximation. Always `approximate`.

Mcp object { server\_label, type, allowed\_tools, 7 more }

Give the model access to additional tools via remote Model Context Protocol
(MCP) servers. [Learn more about MCP](https://developers.openai.com/docs/guides/tools-remote-mcp).

server\_label: string

A label for this MCP server, used to identify it in tool calls.

type: "mcp"

The type of the MCP tool. Always `mcp`.

allowed\_tools: optional array of string or object { read\_only, tool\_names }

List of allowed tool names or a filter object.

One of the following:

McpAllowedTools = array of string

A string array of allowed tool names

McpToolFilter object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

authorization: optional string

An OAuth access token that can be used with a remote MCP server, either
with a custom MCP server URL or a service connector. Your application
must handle the OAuth authorization flow and provide the token here.

connector\_id: optional "connector\_dropbox" or "connector\_gmail" or "connector\_googlecalendar" or 5 more

Identifier for service connectors, like those available in ChatGPT. One of
`server_url` or `connector_id` must be provided. Learn more about service
connectors [here](https://developers.openai.com/docs/guides/tools-remote-mcp#connectors).

Currently supported `connector_id` values are:

- Dropbox: `connector_dropbox`
- Gmail: `connector_gmail`
- Google Calendar: `connector_googlecalendar`
- Google Drive: `connector_googledrive`
- Microsoft Teams: `connector_microsoftteams`
- Outlook Calendar: `connector_outlookcalendar`
- Outlook Email: `connector_outlookemail`
- SharePoint: `connector_sharepoint`

One of the following:

"connector\_dropbox"

"connector\_gmail"

"connector\_googlecalendar"

"connector\_googledrive"

"connector\_microsoftteams"

"connector\_outlookcalendar"

"connector\_outlookemail"

"connector\_sharepoint"

defer\_loading: optional boolean

Whether this MCP tool is deferred and discovered via tool search.

headers: optional map[string]

Optional HTTP headers to send to the MCP server. Use for authentication
or other purposes.

require\_approval: optional object { always, never }  or "always" or "never"

Specify which of the MCP server’s tools require approval.

One of the following:

McpToolApprovalFilter object { always, never }

Specify which of the MCP server’s tools require approval. Can be
`always`, `never`, or a filter object associated with tools
that require approval.

always: optional object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

never: optional object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

McpToolApprovalSetting = "always" or "never"

Specify a single approval policy for all tools. One of `always` or
`never`. When set to `always`, all tools will require approval. When
set to `never`, all tools will not require approval.

One of the following:

"always"

"never"

server\_description: optional string

Optional description of the MCP server, used to provide more context.

server\_url: optional string

The URL for the MCP server. One of `server_url` or `connector_id` must be
provided.

CodeInterpreter object { container, type }

A tool that runs Python code to help generate a response to a prompt.

container: string or object { type, file\_ids, memory\_limit, network\_policy }

The code interpreter container. Can be a container ID or an object that
specifies uploaded file IDs to make available to your code, along with an
optional `memory_limit` setting.

One of the following:

string

The container ID.

CodeInterpreterToolAuto object { type, file\_ids, memory\_limit, network\_policy }

Configuration for a code interpreter container. Optionally specify the IDs of the files to run the code on.

type: "auto"

Always `auto`.

file\_ids: optional array of string

An optional list of uploaded files to make available to your code.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit for the code interpreter container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional [ContainerNetworkPolicyDisabled](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_disabled%20%3E%20(schema)) { type }  or [ContainerNetworkPolicyAllowlist](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_allowlist%20%3E%20(schema)) { allowed\_domains, type, domain\_secrets }

Network access policy for the container.

One of the following:

ContainerNetworkPolicyDisabled object { type }

type: "disabled"

Disable outbound network access. Always `disabled`.

ContainerNetworkPolicyAllowlist object { allowed\_domains, type, domain\_secrets }

allowed\_domains: array of string

A list of allowed domains when type is `allowlist`.

type: "allowlist"

Allow outbound network access only to specified domains. Always `allowlist`.

domain\_secrets: optional array of [ContainerNetworkPolicyDomainSecret](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_domain_secret%20%3E%20(schema)) { domain, name, value }

Optional domain-scoped secrets for allowlisted domains.

domain: string

The domain associated with the secret.

minLength1

name: string

The name of the secret to inject for the domain.

minLength1

value: string

The secret value to inject for the domain.

maxLength10485760

minLength1

type: "code\_interpreter"

The type of the code interpreter tool. Always `code_interpreter`.

ImageGeneration object { type, action, background, 9 more }

A tool that generates images using the GPT image models.

type: "image\_generation"

The type of the image generation tool. Always `image_generation`.

action: optional "generate" or "edit" or "auto"

Whether to generate a new image or edit an existing image. Default: `auto`.

One of the following:

"generate"

"edit"

"auto"

background: optional "transparent" or "opaque" or "auto"

Background type for the generated image. One of `transparent`,
`opaque`, or `auto`. Default: `auto`.

One of the following:

"transparent"

"opaque"

"auto"

input\_fidelity: optional "high" or "low"

Control how much effort the model will exert to match the style and features, especially facial features, of input images. This parameter is only supported for `gpt-image-1` and `gpt-image-1.5` and later models, unsupported for `gpt-image-1-mini`. Supports `high` and `low`. Defaults to `low`.

One of the following:

"high"

"low"

input\_image\_mask: optional object { file\_id, image\_url }

Optional mask for inpainting. Contains `image_url`
(string, optional) and `file_id` (string, optional).

file\_id: optional string

File ID for the mask image.

image\_url: optional string

Base64-encoded mask image.

model: optional string or "gpt-image-1" or "gpt-image-1-mini" or "gpt-image-1.5"

The image generation model to use. Default: `gpt-image-1`.

One of the following:

string

"gpt-image-1" or "gpt-image-1-mini" or "gpt-image-1.5"

The image generation model to use. Default: `gpt-image-1`.

One of the following:

"gpt-image-1"

"gpt-image-1-mini"

"gpt-image-1.5"

moderation: optional "auto" or "low"

Moderation level for the generated image. Default: `auto`.

One of the following:

"auto"

"low"

output\_compression: optional number

Compression level for the output image. Default: 100.

minimum0

maximum100

output\_format: optional "png" or "webp" or "jpeg"

The output format of the generated image. One of `png`, `webp`, or
`jpeg`. Default: `png`.

One of the following:

"png"

"webp"

"jpeg"

partial\_images: optional number

Number of partial images to generate in streaming mode, from 0 (default value) to 3.

minimum0

maximum3

quality: optional "low" or "medium" or "high" or "auto"

The quality of the generated image. One of `low`, `medium`, `high`,
or `auto`. Default: `auto`.

One of the following:

"low"

"medium"

"high"

"auto"

size: optional "1024x1024" or "1024x1536" or "1536x1024" or "auto"

The size of the generated image. One of `1024x1024`, `1024x1536`,
`1536x1024`, or `auto`. Default: `auto`.

One of the following:

"1024x1024"

"1024x1536"

"1536x1024"

"auto"

LocalShell object { type }

A tool that allows the model to execute shell commands in a local environment.

type: "local\_shell"

The type of the local shell tool. Always `local_shell`.

Shell object { type, environment }

A tool that allows the model to execute shell commands.

type: "shell"

The type of the shell tool. Always `shell`.

environment: optional [ContainerAuto](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_auto%20%3E%20(schema)) { type, file\_ids, memory\_limit, 2 more }  or [LocalEnvironment](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20local_environment%20%3E%20(schema)) { type, skills }  or [ContainerReference](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_reference%20%3E%20(schema)) { container\_id, type }

One of the following:

ContainerAuto object { type, file\_ids, memory\_limit, 2 more }

type: "container\_auto"

Automatically creates a container for this request

file\_ids: optional array of string

An optional list of uploaded files to make available to your code.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit for the container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional [ContainerNetworkPolicyDisabled](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_disabled%20%3E%20(schema)) { type }  or [ContainerNetworkPolicyAllowlist](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_allowlist%20%3E%20(schema)) { allowed\_domains, type, domain\_secrets }

Network access policy for the container.

One of the following:

ContainerNetworkPolicyDisabled object { type }

type: "disabled"

Disable outbound network access. Always `disabled`.

ContainerNetworkPolicyAllowlist object { allowed\_domains, type, domain\_secrets }

allowed\_domains: array of string

A list of allowed domains when type is `allowlist`.

type: "allowlist"

Allow outbound network access only to specified domains. Always `allowlist`.

domain\_secrets: optional array of [ContainerNetworkPolicyDomainSecret](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_domain_secret%20%3E%20(schema)) { domain, name, value }

Optional domain-scoped secrets for allowlisted domains.

domain: string

The domain associated with the secret.

minLength1

name: string

The name of the secret to inject for the domain.

minLength1

value: string

The secret value to inject for the domain.

maxLength10485760

minLength1

skills: optional array of [SkillReference](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20skill_reference%20%3E%20(schema)) { skill\_id, type, version }  or [InlineSkill](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20inline_skill%20%3E%20(schema)) { description, name, source, type }

An optional list of skills referenced by id or inline data.

One of the following:

SkillReference object { skill\_id, type, version }

skill\_id: string

The ID of the referenced skill.

maxLength64

minLength1

type: "skill\_reference"

References a skill created with the /v1/skills endpoint.

version: optional string

Optional skill version. Use a positive integer or ‘latest’. Omit for default.

InlineSkill object { description, name, source, type }

description: string

The description of the skill.

name: string

The name of the skill.

source: [InlineSkillSource](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20inline_skill_source%20%3E%20(schema)) { data, media\_type, type }

Inline skill payload

type: "inline"

Defines an inline skill for this request.

LocalEnvironment object { type, skills }

type: "local"

Use a local computer environment.

skills: optional array of [LocalSkill](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20local_skill%20%3E%20(schema)) { description, name, path }

An optional list of skills.

description: string

The description of the skill.

name: string

The name of the skill.

path: string

The path to the directory containing the skill.

ContainerReference object { container\_id, type }

container\_id: string

The ID of the referenced container.

type: "container\_reference"

References a container created with the /v1/containers endpoint

Custom object { name, type, defer\_loading, 2 more }

A custom tool that processes input using a specified format. Learn more about [custom tools](https://developers.openai.com/docs/guides/function-calling#custom-tools)

name: string

The name of the custom tool, used to identify it in tool calls.

type: "custom"

The type of the custom tool. Always `custom`.

defer\_loading: optional boolean

Whether this tool should be deferred and discovered via tool search.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional [CustomToolInputFormat](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20custom_tool_input_format%20%3E%20(schema))

The input format for the custom tool. Default is unconstrained text.

Namespace object { description, name, tools, type }

Groups function/custom tools under a shared namespace.

description: string

A description of the namespace shown to the model.

minLength1

name: string

The namespace name used in tool calls (for example, `crm`).

minLength1

tools: array of object { name, type, defer\_loading, 3 more }  or object { name, type, defer\_loading, 2 more }

The function/custom tools available inside this namespace.

One of the following:

Function object { name, type, defer\_loading, 3 more }

name: string

maxLength128

minLength1

type: "function"

defer\_loading: optional boolean

Whether this function should be deferred and discovered via tool search.

description: optional string

parameters: optional unknown

strict: optional boolean

Custom object { name, type, defer\_loading, 2 more }

A custom tool that processes input using a specified format. Learn more about [custom tools](https://developers.openai.com/docs/guides/function-calling#custom-tools)

name: string

The name of the custom tool, used to identify it in tool calls.

type: "custom"

The type of the custom tool. Always `custom`.

defer\_loading: optional boolean

Whether this tool should be deferred and discovered via tool search.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional [CustomToolInputFormat](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20custom_tool_input_format%20%3E%20(schema))

The input format for the custom tool. Default is unconstrained text.

type: "namespace"

The type of the tool. Always `namespace`.

ToolSearch object { type, description, execution, parameters }

Hosted or BYOT tool search configuration for deferred tools.

type: "tool\_search"

The type of the tool. Always `tool_search`.

description: optional string

Description shown to the model for a client-executed tool search tool.

execution: optional "server" or "client"

Whether tool search is executed by the server or by the client.

One of the following:

"server"

"client"

parameters: optional unknown

Parameter schema for a client-executed tool search tool.

WebSearchPreview object { type, search\_content\_types, search\_context\_size, user\_location }

This tool searches the web for relevant results to use in a response. Learn more about the [web search tool](https://platform.openai.com/docs/guides/tools-web-search).

type: "web\_search\_preview" or "web\_search\_preview\_2025\_03\_11"

The type of the web search tool. One of `web_search_preview` or `web_search_preview_2025_03_11`.

One of the following:

"web\_search\_preview"

"web\_search\_preview\_2025\_03\_11"

search\_content\_types: optional array of "text" or "image"

One of the following:

"text"

"image"

search\_context\_size: optional "low" or "medium" or "high"

High level guidance for the amount of context window space to use for the search. One of `low`, `medium`, or `high`. `medium` is the default.

One of the following:

"low"

"medium"

"high"

user\_location: optional object { type, city, country, 2 more }

The user’s location.

type: "approximate"

The type of location approximation. Always `approximate`.

city: optional string

Free text input for the city of the user, e.g. `San Francisco`.

country: optional string

The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of the user, e.g. `US`.

region: optional string

Free text input for the region of the user, e.g. `California`.

timezone: optional string

The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the user, e.g. `America/Los_Angeles`.

ApplyPatch object { type }

Allows the assistant to create, delete, or update files using unified diffs.

type: "apply\_patch"

The type of the tool. Always `apply_patch`.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

error: [EvalAPIError](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20eval_api_error%20%3E%20(schema)) { code, message }

An object representing an error response from the Eval API.

eval\_id: string

The identifier of the associated evaluation.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: string

The model that is evaluated, if applicable.

name: string

The name of the evaluation run.

object: "eval.run"

The type of the object. Always “eval.run”.

per\_model\_usage: array of object { cached\_tokens, completion\_tokens, invocation\_count, 3 more }

Usage statistics for each model during the evaluation run.

cached\_tokens: number

The number of tokens retrieved from cache.

completion\_tokens: number

The number of completion tokens generated.

invocation\_count: number

The number of invocations.

model\_name: string

The name of the model.

prompt\_tokens: number

The number of prompt tokens used.

total\_tokens: number

The total number of tokens used.

per\_testing\_criteria\_results: array of object { failed, passed, testing\_criteria }

Results per testing criteria applied during the evaluation run.

failed: number

Number of tests failed for this criteria.

passed: number

Number of tests passed for this criteria.

testing\_criteria: string

A description of the testing criteria.

report\_url: string

The URL to the rendered evaluation run report on the UI dashboard.

result\_counts: object { errored, failed, passed, total }

Counters summarizing the outcomes of the evaluation run.

errored: number

Number of output items that resulted in an error.

failed: number

Number of output items that failed to pass the evaluation.

passed: number

Number of output items that passed the evaluation.

total: number

Total number of executed output items.

status: string

The status of the evaluation run.

RunCreateResponse object { id, created\_at, data\_source, 11 more }

A schema representing an evaluation run.

id: string

Unique identifier for the evaluation run.

created\_at: number

Unix timestamp (in seconds) when the evaluation run was created.

data\_source: [CreateEvalJSONLRunDataSource](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20create_eval_jsonl_run_data_source%20%3E%20(schema)) { source, type }  or [CreateEvalCompletionsRunDataSource](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20create_eval_completions_run_data_source%20%3E%20(schema)) { source, type, input\_messages, 2 more }  or object { source, type, input\_messages, 2 more }

Information about the run’s data source.

One of the following:

CreateEvalJSONLRunDataSource object { source, type }

A JsonlRunDataSource object with that specifies a JSONL file that matches the eval

source: object { content, type }  or object { id, type }

Determines what populates the `item` namespace in the data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

type: "jsonl"

The type of data source. Always `jsonl`.

CreateEvalCompletionsRunDataSource object { source, type, input\_messages, 2 more }

A CompletionsRunDataSource object describing a model sampling configuration.

source: object { content, type }  or object { id, type }  or object { type, created\_after, created\_before, 3 more }

Determines what populates the `item` namespace in this run’s data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

StoredCompletionsRunDataSource object { type, created\_after, created\_before, 3 more }

A StoredCompletionsRunDataSource configuration describing a set of filters

type: "stored\_completions"

The type of source. Always `stored_completions`.

created\_after: optional number

An optional Unix timestamp to filter items created after this time.

created\_before: optional number

An optional Unix timestamp to filter items created before this time.

limit: optional number

An optional maximum number of items to return.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: optional string

An optional model to filter by (e.g., ‘gpt-4o’).

type: "completions"

The type of run data source. Always `completions`.

input\_messages: optional object { template, type }  or object { item\_reference, type }

Used when sampling from a model. Dictates the structure of the messages passed into the model. Can either be a reference to a prebuilt trajectory (ie, `item.input_trajectory`), or a template with variable references to the `item` namespace.

One of the following:

TemplateInputMessages object { template, type }

template: array of [EasyInputMessage](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20easy_input_message%20%3E%20(schema)) { content, role, phase, type }  or object { content, role, type }

A list of chat messages forming the prompt or context. May include variable references to the `item` namespace, ie {{item.name}}.

One of the following:

EasyInputMessage object { content, role, phase, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

content: string or [ResponseInputMessageContentList](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_message_content_list%20%3E%20(schema)) { , ,  }

Text, image, or audio input to the model, used to generate a response.
Can also contain previous assistant responses.

One of the following:

TextInput = string

A text input to the model.

ResponseInputMessageContentList = array of [ResponseInputContent](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_content%20%3E%20(schema))

A list of one or many input items to the model, containing different content
types.

One of the following:

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

ResponseInputImage object { detail, type, file\_id, image\_url }

An image input to the model. Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

detail: "low" or "high" or "auto" or "original"

The detail level of the image to be sent to the model. One of `high`, `low`, `auto`, or `original`. Defaults to `auto`.

One of the following:

"low"

"high"

"auto"

"original"

type: "input\_image"

The type of the input item. Always `input_image`.

file\_id: optional string

The ID of the file to be sent to the model.

image\_url: optional string

The URL of the image to be sent to the model. A fully qualified URL or base64 encoded image in a data URL.

ResponseInputFile object { type, detail, file\_data, 3 more }

A file input to the model.

type: "input\_file"

The type of the input item. Always `input_file`.

detail: optional "low" or "high"

The detail level of the file to be sent to the model. Use `low` for the default rendering behavior, or `high` to render the file at higher quality. Defaults to `low`.

One of the following:

"low"

"high"

file\_data: optional string

The content of the file to be sent to the model.

file\_id: optional string

The ID of the file to be sent to the model.

file\_url: optional string

The URL of the file to be sent to the model.

filename: optional string

The name of the file to be sent to the model.

role: "user" or "assistant" or "system" or "developer"

The role of the message input. One of `user`, `assistant`, `system`, or
`developer`.

One of the following:

"user"

"assistant"

"system"

"developer"

phase: optional "commentary" or "final\_answer"

Labels an `assistant` message as intermediate commentary (`commentary`) or the final answer (`final_answer`).
For models like `gpt-5.3-codex` and beyond, when sending follow-up requests, preserve and resend
phase on all assistant messages — dropping it can degrade performance. Not used for user messages.

One of the following:

"commentary"

"final\_answer"

type: optional "message"

The type of the message input. Always `message`.

EvalMessageObject object { content, role, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

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

type: "template"

The type of input messages. Always `template`.

ItemReferenceInputMessages object { item\_reference, type }

item\_reference: string

A reference to a variable in the `item` namespace. Ie, “item.input\_trajectory”

type: "item\_reference"

The type of input messages. Always `item_reference`.

model: optional string

The name of the model to use for generating completions (e.g. “o3-mini”).

sampling\_params: optional object { max\_completion\_tokens, reasoning\_effort, response\_format, 4 more }

max\_completion\_tokens: optional number

The maximum number of tokens in the generated output.

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

response\_format: optional [ResponseFormatText](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_text%20%3E%20(schema)) { type }  or [ResponseFormatJSONSchema](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)) { json\_schema, type }  or [ResponseFormatJSONObject](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_object%20%3E%20(schema)) { type }

An object specifying the format that the model must output.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables
Structured Outputs which ensures the model will match your supplied JSON
schema. Learn more in the [Structured Outputs
guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables the older JSON mode, which
ensures the message the model generates is valid JSON. Using `json_schema`
is preferred for models that support it.

One of the following:

ResponseFormatText object { type }

Default response format. Used to generate text responses.

type: "text"

The type of response format being defined. Always `text`.

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

ResponseFormatJSONObject object { type }

JSON object response format. An older method of generating JSON responses.
Using `json_schema` is recommended for models that support it. Note that the
model will not generate JSON without a system or user message instructing it
to do so.

type: "json\_object"

The type of response format being defined. Always `json_object`.

seed: optional number

A seed value to initialize the randomness, during sampling.

temperature: optional number

A higher temperature increases randomness in the outputs.

tools: optional array of [ChatCompletionFunctionTool](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)) { function, type }

A list of tools the model may call. Currently, only functions are supported as a tool. Use this to provide a list of functions the model may generate JSON inputs for. A max of 128 functions are supported.

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of the tool. Currently, only `function` is supported.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

ResponsesRunDataSource object { source, type, input\_messages, 2 more }

A ResponsesRunDataSource object describing a model sampling configuration.

source: object { content, type }  or object { id, type }  or object { type, created\_after, created\_before, 8 more }

Determines what populates the `item` namespace in this run’s data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

EvalResponsesSource object { type, created\_after, created\_before, 8 more }

A EvalResponsesSource object describing a run data source configuration.

type: "responses"

The type of run data source. Always `responses`.

created\_after: optional number

Only include items created after this timestamp (inclusive). This is a query parameter used to select responses.

minimum0

created\_before: optional number

Only include items created before this timestamp (inclusive). This is a query parameter used to select responses.

minimum0

instructions\_search: optional string

Optional string to search the ‘instructions’ field. This is a query parameter used to select responses.

metadata: optional unknown

Metadata filter for the responses. This is a query parameter used to select responses.

model: optional string

The name of the model to find responses for. This is a query parameter used to select responses.

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

temperature: optional number

Sampling temperature. This is a query parameter used to select responses.

tools: optional array of string

List of tool names. This is a query parameter used to select responses.

top\_p: optional number

Nucleus sampling parameter. This is a query parameter used to select responses.

users: optional array of string

List of user identifiers. This is a query parameter used to select responses.

type: "responses"

The type of run data source. Always `responses`.

input\_messages: optional object { template, type }  or object { item\_reference, type }

Used when sampling from a model. Dictates the structure of the messages passed into the model. Can either be a reference to a prebuilt trajectory (ie, `item.input_trajectory`), or a template with variable references to the `item` namespace.

One of the following:

InputMessagesTemplate object { template, type }

template: array of object { content, role }  or object { content, role, type }

A list of chat messages forming the prompt or context. May include variable references to the `item` namespace, ie {{item.name}}.

One of the following:

ChatMessage object { content, role }

content: string

The content of the message.

role: string

The role of the message (e.g. “system”, “assistant”, “user”).

EvalMessageObject object { content, role, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

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

type: "template"

The type of input messages. Always `template`.

InputMessagesItemReference object { item\_reference, type }

item\_reference: string

A reference to a variable in the `item` namespace. Ie, “item.name”

type: "item\_reference"

The type of input messages. Always `item_reference`.

model: optional string

The name of the model to use for generating completions (e.g. “o3-mini”).

sampling\_params: optional object { max\_completion\_tokens, reasoning\_effort, seed, 4 more }

max\_completion\_tokens: optional number

The maximum number of tokens in the generated output.

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

text: optional object { format }

Configuration options for a text response from the model. Can be plain
text or structured JSON data. Learn more:

- [Text inputs and outputs](https://developers.openai.com/docs/guides/text)
- [Structured Outputs](https://developers.openai.com/docs/guides/structured-outputs)

format: optional [ResponseFormatTextConfig](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_format_text_config%20%3E%20(schema))

An object specifying the format that the model must output.

Configuring `{ "type": "json_schema" }` enables Structured Outputs,
which ensures the model will match your supplied JSON schema. Learn more in the
[Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

The default format is `{ "type": "text" }` with no additional options.

**Not recommended for gpt-4o and newer models:**

Setting to `{ "type": "json_object" }` enables the older JSON mode, which
ensures the message the model generates is valid JSON. Using `json_schema`
is preferred for models that support it.

tools: optional array of object { name, parameters, strict, 3 more }  or object { type, vector\_store\_ids, filters, 2 more }  or object { type }  or 12 more

An array of tools the model may call while generating a response. You
can specify which tool to use by setting the `tool_choice` parameter.

The two categories of tools you can provide the model are:

- **Built-in tools**: Tools that are provided by OpenAI that extend the
  model’s capabilities, like [web search](https://developers.openai.com/docs/guides/tools-web-search)
  or [file search](https://developers.openai.com/docs/guides/tools-file-search). Learn more about
  [built-in tools](https://developers.openai.com/docs/guides/tools).
- **Function calls (custom tools)**: Functions that are defined by you,
  enabling the model to call your own code. Learn more about
  [function calling](https://developers.openai.com/docs/guides/function-calling).

One of the following:

Function object { name, parameters, strict, 3 more }

Defines a function in your own code the model can choose to call. Learn more about [function calling](https://platform.openai.com/docs/guides/function-calling).

name: string

The name of the function to call.

parameters: map[unknown]

A JSON schema object describing the parameters of the function.

strict: boolean

Whether to enforce strict parameter validation. Default `true`.

type: "function"

The type of the function tool. Always `function`.

defer\_loading: optional boolean

Whether this function is deferred and loaded via tool search.

description: optional string

A description of the function. Used by the model to determine whether or not to call the function.

FileSearch object { type, vector\_store\_ids, filters, 2 more }

A tool that searches for relevant content from uploaded files. Learn more about the [file search tool](https://platform.openai.com/docs/guides/tools-file-search).

type: "file\_search"

The type of the file search tool. Always `file_search`.

vector\_store\_ids: array of string

The IDs of the vector stores to search.

filters: optional [ComparisonFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20comparison_filter%20%3E%20(schema)) { key, type, value }  or [CompoundFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20compound_filter%20%3E%20(schema)) { filters, type }

A filter to apply.

One of the following:

ComparisonFilter object { key, type, value }

A filter used to compare a specified attribute key to a given value using a defined comparison operation.

key: string

The key to compare against the value.

type: "eq" or "ne" or "gt" or 5 more

Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`.

- `eq`: equals
- `ne`: not equal
- `gt`: greater than
- `gte`: greater than or equal
- `lt`: less than
- `lte`: less than or equal
- `in`: in
- `nin`: not in

One of the following:

"eq"

"ne"

"gt"

"gte"

"lt"

"lte"

"in"

"nin"

value: string or number or boolean or array of string or number

The value to compare against the attribute key; supports string, number, or boolean types.

One of the following:

string

number

boolean

array of string or number

One of the following:

string

number

CompoundFilter object { filters, type }

Combine multiple filters using `and` or `or`.

filters: array of [ComparisonFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20comparison_filter%20%3E%20(schema)) { key, type, value }  or unknown

Array of filters to combine. Items can be `ComparisonFilter` or `CompoundFilter`.

One of the following:

ComparisonFilter object { key, type, value }

A filter used to compare a specified attribute key to a given value using a defined comparison operation.

key: string

The key to compare against the value.

type: "eq" or "ne" or "gt" or 5 more

Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`.

- `eq`: equals
- `ne`: not equal
- `gt`: greater than
- `gte`: greater than or equal
- `lt`: less than
- `lte`: less than or equal
- `in`: in
- `nin`: not in

One of the following:

"eq"

"ne"

"gt"

"gte"

"lt"

"lte"

"in"

"nin"

value: string or number or boolean or array of string or number

The value to compare against the attribute key; supports string, number, or boolean types.

One of the following:

string

number

boolean

array of string or number

One of the following:

string

number

unknown

type: "and" or "or"

Type of operation: `and` or `or`.

One of the following:

"and"

"or"

max\_num\_results: optional number

The maximum number of results to return. This number should be between 1 and 50 inclusive.

ranking\_options: optional object { hybrid\_search, ranker, score\_threshold }

Ranking options for search.

hybrid\_search: optional object { embedding\_weight, text\_weight }

Weights that control how reciprocal rank fusion balances semantic embedding matches versus sparse keyword matches when hybrid search is enabled.

embedding\_weight: number

The weight of the embedding in the reciprocal ranking fusion.

text\_weight: number

The weight of the text in the reciprocal ranking fusion.

ranker: optional "auto" or "default-2024-11-15"

The ranker to use for the file search.

One of the following:

"auto"

"default-2024-11-15"

score\_threshold: optional number

The score threshold for the file search, a number between 0 and 1. Numbers closer to 1 will attempt to return only the most relevant results, but may return fewer results.

Computer object { type }

A tool that controls a virtual computer. Learn more about the [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).

type: "computer"

The type of the computer tool. Always `computer`.

ComputerUsePreview object { display\_height, display\_width, environment, type }

A tool that controls a virtual computer. Learn more about the [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).

display\_height: number

The height of the computer display.

display\_width: number

The width of the computer display.

environment: "windows" or "mac" or "linux" or 2 more

The type of computer environment to control.

One of the following:

"windows"

"mac"

"linux"

"ubuntu"

"browser"

type: "computer\_use\_preview"

The type of the computer use tool. Always `computer_use_preview`.

WebSearch object { type, filters, search\_context\_size, user\_location }

Search the Internet for sources related to the prompt. Learn more about the
[web search tool](https://developers.openai.com/docs/guides/tools-web-search).

type: "web\_search" or "web\_search\_2025\_08\_26"

The type of the web search tool. One of `web_search` or `web_search_2025_08_26`.

One of the following:

"web\_search"

"web\_search\_2025\_08\_26"

filters: optional object { allowed\_domains }

Filters for the search.

allowed\_domains: optional array of string

Allowed domains for the search. If not provided, all domains are allowed.
Subdomains of the provided domains are allowed as well.

Example: `["pubmed.ncbi.nlm.nih.gov"]`

search\_context\_size: optional "low" or "medium" or "high"

High level guidance for the amount of context window space to use for the search. One of `low`, `medium`, or `high`. `medium` is the default.

One of the following:

"low"

"medium"

"high"

user\_location: optional object { city, country, region, 2 more }

The approximate location of the user.

city: optional string

Free text input for the city of the user, e.g. `San Francisco`.

country: optional string

The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of the user, e.g. `US`.

region: optional string

Free text input for the region of the user, e.g. `California`.

timezone: optional string

The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the user, e.g. `America/Los_Angeles`.

type: optional "approximate"

The type of location approximation. Always `approximate`.

Mcp object { server\_label, type, allowed\_tools, 7 more }

Give the model access to additional tools via remote Model Context Protocol
(MCP) servers. [Learn more about MCP](https://developers.openai.com/docs/guides/tools-remote-mcp).

server\_label: string

A label for this MCP server, used to identify it in tool calls.

type: "mcp"

The type of the MCP tool. Always `mcp`.

allowed\_tools: optional array of string or object { read\_only, tool\_names }

List of allowed tool names or a filter object.

One of the following:

McpAllowedTools = array of string

A string array of allowed tool names

McpToolFilter object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

authorization: optional string

An OAuth access token that can be used with a remote MCP server, either
with a custom MCP server URL or a service connector. Your application
must handle the OAuth authorization flow and provide the token here.

connector\_id: optional "connector\_dropbox" or "connector\_gmail" or "connector\_googlecalendar" or 5 more

Identifier for service connectors, like those available in ChatGPT. One of
`server_url` or `connector_id` must be provided. Learn more about service
connectors [here](https://developers.openai.com/docs/guides/tools-remote-mcp#connectors).

Currently supported `connector_id` values are:

- Dropbox: `connector_dropbox`
- Gmail: `connector_gmail`
- Google Calendar: `connector_googlecalendar`
- Google Drive: `connector_googledrive`
- Microsoft Teams: `connector_microsoftteams`
- Outlook Calendar: `connector_outlookcalendar`
- Outlook Email: `connector_outlookemail`
- SharePoint: `connector_sharepoint`

One of the following:

"connector\_dropbox"

"connector\_gmail"

"connector\_googlecalendar"

"connector\_googledrive"

"connector\_microsoftteams"

"connector\_outlookcalendar"

"connector\_outlookemail"

"connector\_sharepoint"

defer\_loading: optional boolean

Whether this MCP tool is deferred and discovered via tool search.

headers: optional map[string]

Optional HTTP headers to send to the MCP server. Use for authentication
or other purposes.

require\_approval: optional object { always, never }  or "always" or "never"

Specify which of the MCP server’s tools require approval.

One of the following:

McpToolApprovalFilter object { always, never }

Specify which of the MCP server’s tools require approval. Can be
`always`, `never`, or a filter object associated with tools
that require approval.

always: optional object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

never: optional object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

McpToolApprovalSetting = "always" or "never"

Specify a single approval policy for all tools. One of `always` or
`never`. When set to `always`, all tools will require approval. When
set to `never`, all tools will not require approval.

One of the following:

"always"

"never"

server\_description: optional string

Optional description of the MCP server, used to provide more context.

server\_url: optional string

The URL for the MCP server. One of `server_url` or `connector_id` must be
provided.

CodeInterpreter object { container, type }

A tool that runs Python code to help generate a response to a prompt.

container: string or object { type, file\_ids, memory\_limit, network\_policy }

The code interpreter container. Can be a container ID or an object that
specifies uploaded file IDs to make available to your code, along with an
optional `memory_limit` setting.

One of the following:

string

The container ID.

CodeInterpreterToolAuto object { type, file\_ids, memory\_limit, network\_policy }

Configuration for a code interpreter container. Optionally specify the IDs of the files to run the code on.

type: "auto"

Always `auto`.

file\_ids: optional array of string

An optional list of uploaded files to make available to your code.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit for the code interpreter container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional [ContainerNetworkPolicyDisabled](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_disabled%20%3E%20(schema)) { type }  or [ContainerNetworkPolicyAllowlist](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_allowlist%20%3E%20(schema)) { allowed\_domains, type, domain\_secrets }

Network access policy for the container.

One of the following:

ContainerNetworkPolicyDisabled object { type }

type: "disabled"

Disable outbound network access. Always `disabled`.

ContainerNetworkPolicyAllowlist object { allowed\_domains, type, domain\_secrets }

allowed\_domains: array of string

A list of allowed domains when type is `allowlist`.

type: "allowlist"

Allow outbound network access only to specified domains. Always `allowlist`.

domain\_secrets: optional array of [ContainerNetworkPolicyDomainSecret](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_domain_secret%20%3E%20(schema)) { domain, name, value }

Optional domain-scoped secrets for allowlisted domains.

domain: string

The domain associated with the secret.

minLength1

name: string

The name of the secret to inject for the domain.

minLength1

value: string

The secret value to inject for the domain.

maxLength10485760

minLength1

type: "code\_interpreter"

The type of the code interpreter tool. Always `code_interpreter`.

ImageGeneration object { type, action, background, 9 more }

A tool that generates images using the GPT image models.

type: "image\_generation"

The type of the image generation tool. Always `image_generation`.

action: optional "generate" or "edit" or "auto"

Whether to generate a new image or edit an existing image. Default: `auto`.

One of the following:

"generate"

"edit"

"auto"

background: optional "transparent" or "opaque" or "auto"

Background type for the generated image. One of `transparent`,
`opaque`, or `auto`. Default: `auto`.

One of the following:

"transparent"

"opaque"

"auto"

input\_fidelity: optional "high" or "low"

Control how much effort the model will exert to match the style and features, especially facial features, of input images. This parameter is only supported for `gpt-image-1` and `gpt-image-1.5` and later models, unsupported for `gpt-image-1-mini`. Supports `high` and `low`. Defaults to `low`.

One of the following:

"high"

"low"

input\_image\_mask: optional object { file\_id, image\_url }

Optional mask for inpainting. Contains `image_url`
(string, optional) and `file_id` (string, optional).

file\_id: optional string

File ID for the mask image.

image\_url: optional string

Base64-encoded mask image.

model: optional string or "gpt-image-1" or "gpt-image-1-mini" or "gpt-image-1.5"

The image generation model to use. Default: `gpt-image-1`.

One of the following:

string

"gpt-image-1" or "gpt-image-1-mini" or "gpt-image-1.5"

The image generation model to use. Default: `gpt-image-1`.

One of the following:

"gpt-image-1"

"gpt-image-1-mini"

"gpt-image-1.5"

moderation: optional "auto" or "low"

Moderation level for the generated image. Default: `auto`.

One of the following:

"auto"

"low"

output\_compression: optional number

Compression level for the output image. Default: 100.

minimum0

maximum100

output\_format: optional "png" or "webp" or "jpeg"

The output format of the generated image. One of `png`, `webp`, or
`jpeg`. Default: `png`.

One of the following:

"png"

"webp"

"jpeg"

partial\_images: optional number

Number of partial images to generate in streaming mode, from 0 (default value) to 3.

minimum0

maximum3

quality: optional "low" or "medium" or "high" or "auto"

The quality of the generated image. One of `low`, `medium`, `high`,
or `auto`. Default: `auto`.

One of the following:

"low"

"medium"

"high"

"auto"

size: optional "1024x1024" or "1024x1536" or "1536x1024" or "auto"

The size of the generated image. One of `1024x1024`, `1024x1536`,
`1536x1024`, or `auto`. Default: `auto`.

One of the following:

"1024x1024"

"1024x1536"

"1536x1024"

"auto"

LocalShell object { type }

A tool that allows the model to execute shell commands in a local environment.

type: "local\_shell"

The type of the local shell tool. Always `local_shell`.

Shell object { type, environment }

A tool that allows the model to execute shell commands.

type: "shell"

The type of the shell tool. Always `shell`.

environment: optional [ContainerAuto](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_auto%20%3E%20(schema)) { type, file\_ids, memory\_limit, 2 more }  or [LocalEnvironment](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20local_environment%20%3E%20(schema)) { type, skills }  or [ContainerReference](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_reference%20%3E%20(schema)) { container\_id, type }

One of the following:

ContainerAuto object { type, file\_ids, memory\_limit, 2 more }

type: "container\_auto"

Automatically creates a container for this request

file\_ids: optional array of string

An optional list of uploaded files to make available to your code.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit for the container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional [ContainerNetworkPolicyDisabled](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_disabled%20%3E%20(schema)) { type }  or [ContainerNetworkPolicyAllowlist](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_allowlist%20%3E%20(schema)) { allowed\_domains, type, domain\_secrets }

Network access policy for the container.

One of the following:

ContainerNetworkPolicyDisabled object { type }

type: "disabled"

Disable outbound network access. Always `disabled`.

ContainerNetworkPolicyAllowlist object { allowed\_domains, type, domain\_secrets }

allowed\_domains: array of string

A list of allowed domains when type is `allowlist`.

type: "allowlist"

Allow outbound network access only to specified domains. Always `allowlist`.

domain\_secrets: optional array of [ContainerNetworkPolicyDomainSecret](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_domain_secret%20%3E%20(schema)) { domain, name, value }

Optional domain-scoped secrets for allowlisted domains.

domain: string

The domain associated with the secret.

minLength1

name: string

The name of the secret to inject for the domain.

minLength1

value: string

The secret value to inject for the domain.

maxLength10485760

minLength1

skills: optional array of [SkillReference](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20skill_reference%20%3E%20(schema)) { skill\_id, type, version }  or [InlineSkill](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20inline_skill%20%3E%20(schema)) { description, name, source, type }

An optional list of skills referenced by id or inline data.

One of the following:

SkillReference object { skill\_id, type, version }

skill\_id: string

The ID of the referenced skill.

maxLength64

minLength1

type: "skill\_reference"

References a skill created with the /v1/skills endpoint.

version: optional string

Optional skill version. Use a positive integer or ‘latest’. Omit for default.

InlineSkill object { description, name, source, type }

description: string

The description of the skill.

name: string

The name of the skill.

source: [InlineSkillSource](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20inline_skill_source%20%3E%20(schema)) { data, media\_type, type }

Inline skill payload

type: "inline"

Defines an inline skill for this request.

LocalEnvironment object { type, skills }

type: "local"

Use a local computer environment.

skills: optional array of [LocalSkill](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20local_skill%20%3E%20(schema)) { description, name, path }

An optional list of skills.

description: string

The description of the skill.

name: string

The name of the skill.

path: string

The path to the directory containing the skill.

ContainerReference object { container\_id, type }

container\_id: string

The ID of the referenced container.

type: "container\_reference"

References a container created with the /v1/containers endpoint

Custom object { name, type, defer\_loading, 2 more }

A custom tool that processes input using a specified format. Learn more about [custom tools](https://developers.openai.com/docs/guides/function-calling#custom-tools)

name: string

The name of the custom tool, used to identify it in tool calls.

type: "custom"

The type of the custom tool. Always `custom`.

defer\_loading: optional boolean

Whether this tool should be deferred and discovered via tool search.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional [CustomToolInputFormat](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20custom_tool_input_format%20%3E%20(schema))

The input format for the custom tool. Default is unconstrained text.

Namespace object { description, name, tools, type }

Groups function/custom tools under a shared namespace.

description: string

A description of the namespace shown to the model.

minLength1

name: string

The namespace name used in tool calls (for example, `crm`).

minLength1

tools: array of object { name, type, defer\_loading, 3 more }  or object { name, type, defer\_loading, 2 more }

The function/custom tools available inside this namespace.

One of the following:

Function object { name, type, defer\_loading, 3 more }

name: string

maxLength128

minLength1

type: "function"

defer\_loading: optional boolean

Whether this function should be deferred and discovered via tool search.

description: optional string

parameters: optional unknown

strict: optional boolean

Custom object { name, type, defer\_loading, 2 more }

A custom tool that processes input using a specified format. Learn more about [custom tools](https://developers.openai.com/docs/guides/function-calling#custom-tools)

name: string

The name of the custom tool, used to identify it in tool calls.

type: "custom"

The type of the custom tool. Always `custom`.

defer\_loading: optional boolean

Whether this tool should be deferred and discovered via tool search.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional [CustomToolInputFormat](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20custom_tool_input_format%20%3E%20(schema))

The input format for the custom tool. Default is unconstrained text.

type: "namespace"

The type of the tool. Always `namespace`.

ToolSearch object { type, description, execution, parameters }

Hosted or BYOT tool search configuration for deferred tools.

type: "tool\_search"

The type of the tool. Always `tool_search`.

description: optional string

Description shown to the model for a client-executed tool search tool.

execution: optional "server" or "client"

Whether tool search is executed by the server or by the client.

One of the following:

"server"

"client"

parameters: optional unknown

Parameter schema for a client-executed tool search tool.

WebSearchPreview object { type, search\_content\_types, search\_context\_size, user\_location }

This tool searches the web for relevant results to use in a response. Learn more about the [web search tool](https://platform.openai.com/docs/guides/tools-web-search).

type: "web\_search\_preview" or "web\_search\_preview\_2025\_03\_11"

The type of the web search tool. One of `web_search_preview` or `web_search_preview_2025_03_11`.

One of the following:

"web\_search\_preview"

"web\_search\_preview\_2025\_03\_11"

search\_content\_types: optional array of "text" or "image"

One of the following:

"text"

"image"

search\_context\_size: optional "low" or "medium" or "high"

High level guidance for the amount of context window space to use for the search. One of `low`, `medium`, or `high`. `medium` is the default.

One of the following:

"low"

"medium"

"high"

user\_location: optional object { type, city, country, 2 more }

The user’s location.

type: "approximate"

The type of location approximation. Always `approximate`.

city: optional string

Free text input for the city of the user, e.g. `San Francisco`.

country: optional string

The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of the user, e.g. `US`.

region: optional string

Free text input for the region of the user, e.g. `California`.

timezone: optional string

The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the user, e.g. `America/Los_Angeles`.

ApplyPatch object { type }

Allows the assistant to create, delete, or update files using unified diffs.

type: "apply\_patch"

The type of the tool. Always `apply_patch`.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

error: [EvalAPIError](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20eval_api_error%20%3E%20(schema)) { code, message }

An object representing an error response from the Eval API.

eval\_id: string

The identifier of the associated evaluation.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: string

The model that is evaluated, if applicable.

name: string

The name of the evaluation run.

object: "eval.run"

The type of the object. Always “eval.run”.

per\_model\_usage: array of object { cached\_tokens, completion\_tokens, invocation\_count, 3 more }

Usage statistics for each model during the evaluation run.

cached\_tokens: number

The number of tokens retrieved from cache.

completion\_tokens: number

The number of completion tokens generated.

invocation\_count: number

The number of invocations.

model\_name: string

The name of the model.

prompt\_tokens: number

The number of prompt tokens used.

total\_tokens: number

The total number of tokens used.

per\_testing\_criteria\_results: array of object { failed, passed, testing\_criteria }

Results per testing criteria applied during the evaluation run.

failed: number

Number of tests failed for this criteria.

passed: number

Number of tests passed for this criteria.

testing\_criteria: string

A description of the testing criteria.

report\_url: string

The URL to the rendered evaluation run report on the UI dashboard.

result\_counts: object { errored, failed, passed, total }

Counters summarizing the outcomes of the evaluation run.

errored: number

Number of output items that resulted in an error.

failed: number

Number of output items that failed to pass the evaluation.

passed: number

Number of output items that passed the evaluation.

total: number

Total number of executed output items.

status: string

The status of the evaluation run.

RunRetrieveResponse object { id, created\_at, data\_source, 11 more }

A schema representing an evaluation run.

id: string

Unique identifier for the evaluation run.

created\_at: number

Unix timestamp (in seconds) when the evaluation run was created.

data\_source: [CreateEvalJSONLRunDataSource](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20create_eval_jsonl_run_data_source%20%3E%20(schema)) { source, type }  or [CreateEvalCompletionsRunDataSource](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20create_eval_completions_run_data_source%20%3E%20(schema)) { source, type, input\_messages, 2 more }  or object { source, type, input\_messages, 2 more }

Information about the run’s data source.

One of the following:

CreateEvalJSONLRunDataSource object { source, type }

A JsonlRunDataSource object with that specifies a JSONL file that matches the eval

source: object { content, type }  or object { id, type }

Determines what populates the `item` namespace in the data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

type: "jsonl"

The type of data source. Always `jsonl`.

CreateEvalCompletionsRunDataSource object { source, type, input\_messages, 2 more }

A CompletionsRunDataSource object describing a model sampling configuration.

source: object { content, type }  or object { id, type }  or object { type, created\_after, created\_before, 3 more }

Determines what populates the `item` namespace in this run’s data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

StoredCompletionsRunDataSource object { type, created\_after, created\_before, 3 more }

A StoredCompletionsRunDataSource configuration describing a set of filters

type: "stored\_completions"

The type of source. Always `stored_completions`.

created\_after: optional number

An optional Unix timestamp to filter items created after this time.

created\_before: optional number

An optional Unix timestamp to filter items created before this time.

limit: optional number

An optional maximum number of items to return.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: optional string

An optional model to filter by (e.g., ‘gpt-4o’).

type: "completions"

The type of run data source. Always `completions`.

input\_messages: optional object { template, type }  or object { item\_reference, type }

Used when sampling from a model. Dictates the structure of the messages passed into the model. Can either be a reference to a prebuilt trajectory (ie, `item.input_trajectory`), or a template with variable references to the `item` namespace.

One of the following:

TemplateInputMessages object { template, type }

template: array of [EasyInputMessage](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20easy_input_message%20%3E%20(schema)) { content, role, phase, type }  or object { content, role, type }

A list of chat messages forming the prompt or context. May include variable references to the `item` namespace, ie {{item.name}}.

One of the following:

EasyInputMessage object { content, role, phase, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

content: string or [ResponseInputMessageContentList](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_message_content_list%20%3E%20(schema)) { , ,  }

Text, image, or audio input to the model, used to generate a response.
Can also contain previous assistant responses.

One of the following:

TextInput = string

A text input to the model.

ResponseInputMessageContentList = array of [ResponseInputContent](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_content%20%3E%20(schema))

A list of one or many input items to the model, containing different content
types.

One of the following:

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

ResponseInputImage object { detail, type, file\_id, image\_url }

An image input to the model. Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

detail: "low" or "high" or "auto" or "original"

The detail level of the image to be sent to the model. One of `high`, `low`, `auto`, or `original`. Defaults to `auto`.

One of the following:

"low"

"high"

"auto"

"original"

type: "input\_image"

The type of the input item. Always `input_image`.

file\_id: optional string

The ID of the file to be sent to the model.

image\_url: optional string

The URL of the image to be sent to the model. A fully qualified URL or base64 encoded image in a data URL.

ResponseInputFile object { type, detail, file\_data, 3 more }

A file input to the model.

type: "input\_file"

The type of the input item. Always `input_file`.

detail: optional "low" or "high"

The detail level of the file to be sent to the model. Use `low` for the default rendering behavior, or `high` to render the file at higher quality. Defaults to `low`.

One of the following:

"low"

"high"

file\_data: optional string

The content of the file to be sent to the model.

file\_id: optional string

The ID of the file to be sent to the model.

file\_url: optional string

The URL of the file to be sent to the model.

filename: optional string

The name of the file to be sent to the model.

role: "user" or "assistant" or "system" or "developer"

The role of the message input. One of `user`, `assistant`, `system`, or
`developer`.

One of the following:

"user"

"assistant"

"system"

"developer"

phase: optional "commentary" or "final\_answer"

Labels an `assistant` message as intermediate commentary (`commentary`) or the final answer (`final_answer`).
For models like `gpt-5.3-codex` and beyond, when sending follow-up requests, preserve and resend
phase on all assistant messages — dropping it can degrade performance. Not used for user messages.

One of the following:

"commentary"

"final\_answer"

type: optional "message"

The type of the message input. Always `message`.

EvalMessageObject object { content, role, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

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

type: "template"

The type of input messages. Always `template`.

ItemReferenceInputMessages object { item\_reference, type }

item\_reference: string

A reference to a variable in the `item` namespace. Ie, “item.input\_trajectory”

type: "item\_reference"

The type of input messages. Always `item_reference`.

model: optional string

The name of the model to use for generating completions (e.g. “o3-mini”).

sampling\_params: optional object { max\_completion\_tokens, reasoning\_effort, response\_format, 4 more }

max\_completion\_tokens: optional number

The maximum number of tokens in the generated output.

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

response\_format: optional [ResponseFormatText](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_text%20%3E%20(schema)) { type }  or [ResponseFormatJSONSchema](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)) { json\_schema, type }  or [ResponseFormatJSONObject](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_object%20%3E%20(schema)) { type }

An object specifying the format that the model must output.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables
Structured Outputs which ensures the model will match your supplied JSON
schema. Learn more in the [Structured Outputs
guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables the older JSON mode, which
ensures the message the model generates is valid JSON. Using `json_schema`
is preferred for models that support it.

One of the following:

ResponseFormatText object { type }

Default response format. Used to generate text responses.

type: "text"

The type of response format being defined. Always `text`.

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

ResponseFormatJSONObject object { type }

JSON object response format. An older method of generating JSON responses.
Using `json_schema` is recommended for models that support it. Note that the
model will not generate JSON without a system or user message instructing it
to do so.

type: "json\_object"

The type of response format being defined. Always `json_object`.

seed: optional number

A seed value to initialize the randomness, during sampling.

temperature: optional number

A higher temperature increases randomness in the outputs.

tools: optional array of [ChatCompletionFunctionTool](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)) { function, type }

A list of tools the model may call. Currently, only functions are supported as a tool. Use this to provide a list of functions the model may generate JSON inputs for. A max of 128 functions are supported.

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of the tool. Currently, only `function` is supported.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

ResponsesRunDataSource object { source, type, input\_messages, 2 more }

A ResponsesRunDataSource object describing a model sampling configuration.

source: object { content, type }  or object { id, type }  or object { type, created\_after, created\_before, 8 more }

Determines what populates the `item` namespace in this run’s data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

EvalResponsesSource object { type, created\_after, created\_before, 8 more }

A EvalResponsesSource object describing a run data source configuration.

type: "responses"

The type of run data source. Always `responses`.

created\_after: optional number

Only include items created after this timestamp (inclusive). This is a query parameter used to select responses.

minimum0

created\_before: optional number

Only include items created before this timestamp (inclusive). This is a query parameter used to select responses.

minimum0

instructions\_search: optional string

Optional string to search the ‘instructions’ field. This is a query parameter used to select responses.

metadata: optional unknown

Metadata filter for the responses. This is a query parameter used to select responses.

model: optional string

The name of the model to find responses for. This is a query parameter used to select responses.

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

temperature: optional number

Sampling temperature. This is a query parameter used to select responses.

tools: optional array of string

List of tool names. This is a query parameter used to select responses.

top\_p: optional number

Nucleus sampling parameter. This is a query parameter used to select responses.

users: optional array of string

List of user identifiers. This is a query parameter used to select responses.

type: "responses"

The type of run data source. Always `responses`.

input\_messages: optional object { template, type }  or object { item\_reference, type }

Used when sampling from a model. Dictates the structure of the messages passed into the model. Can either be a reference to a prebuilt trajectory (ie, `item.input_trajectory`), or a template with variable references to the `item` namespace.

One of the following:

InputMessagesTemplate object { template, type }

template: array of object { content, role }  or object { content, role, type }

A list of chat messages forming the prompt or context. May include variable references to the `item` namespace, ie {{item.name}}.

One of the following:

ChatMessage object { content, role }

content: string

The content of the message.

role: string

The role of the message (e.g. “system”, “assistant”, “user”).

EvalMessageObject object { content, role, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

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

type: "template"

The type of input messages. Always `template`.

InputMessagesItemReference object { item\_reference, type }

item\_reference: string

A reference to a variable in the `item` namespace. Ie, “item.name”

type: "item\_reference"

The type of input messages. Always `item_reference`.

model: optional string

The name of the model to use for generating completions (e.g. “o3-mini”).

sampling\_params: optional object { max\_completion\_tokens, reasoning\_effort, seed, 4 more }

max\_completion\_tokens: optional number

The maximum number of tokens in the generated output.

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

text: optional object { format }

Configuration options for a text response from the model. Can be plain
text or structured JSON data. Learn more:

- [Text inputs and outputs](https://developers.openai.com/docs/guides/text)
- [Structured Outputs](https://developers.openai.com/docs/guides/structured-outputs)

format: optional [ResponseFormatTextConfig](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_format_text_config%20%3E%20(schema))

An object specifying the format that the model must output.

Configuring `{ "type": "json_schema" }` enables Structured Outputs,
which ensures the model will match your supplied JSON schema. Learn more in the
[Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

The default format is `{ "type": "text" }` with no additional options.

**Not recommended for gpt-4o and newer models:**

Setting to `{ "type": "json_object" }` enables the older JSON mode, which
ensures the message the model generates is valid JSON. Using `json_schema`
is preferred for models that support it.

tools: optional array of object { name, parameters, strict, 3 more }  or object { type, vector\_store\_ids, filters, 2 more }  or object { type }  or 12 more

An array of tools the model may call while generating a response. You
can specify which tool to use by setting the `tool_choice` parameter.

The two categories of tools you can provide the model are:

- **Built-in tools**: Tools that are provided by OpenAI that extend the
  model’s capabilities, like [web search](https://developers.openai.com/docs/guides/tools-web-search)
  or [file search](https://developers.openai.com/docs/guides/tools-file-search). Learn more about
  [built-in tools](https://developers.openai.com/docs/guides/tools).
- **Function calls (custom tools)**: Functions that are defined by you,
  enabling the model to call your own code. Learn more about
  [function calling](https://developers.openai.com/docs/guides/function-calling).

One of the following:

Function object { name, parameters, strict, 3 more }

Defines a function in your own code the model can choose to call. Learn more about [function calling](https://platform.openai.com/docs/guides/function-calling).

name: string

The name of the function to call.

parameters: map[unknown]

A JSON schema object describing the parameters of the function.

strict: boolean

Whether to enforce strict parameter validation. Default `true`.

type: "function"

The type of the function tool. Always `function`.

defer\_loading: optional boolean

Whether this function is deferred and loaded via tool search.

description: optional string

A description of the function. Used by the model to determine whether or not to call the function.

FileSearch object { type, vector\_store\_ids, filters, 2 more }

A tool that searches for relevant content from uploaded files. Learn more about the [file search tool](https://platform.openai.com/docs/guides/tools-file-search).

type: "file\_search"

The type of the file search tool. Always `file_search`.

vector\_store\_ids: array of string

The IDs of the vector stores to search.

filters: optional [ComparisonFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20comparison_filter%20%3E%20(schema)) { key, type, value }  or [CompoundFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20compound_filter%20%3E%20(schema)) { filters, type }

A filter to apply.

One of the following:

ComparisonFilter object { key, type, value }

A filter used to compare a specified attribute key to a given value using a defined comparison operation.

key: string

The key to compare against the value.

type: "eq" or "ne" or "gt" or 5 more

Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`.

- `eq`: equals
- `ne`: not equal
- `gt`: greater than
- `gte`: greater than or equal
- `lt`: less than
- `lte`: less than or equal
- `in`: in
- `nin`: not in

One of the following:

"eq"

"ne"

"gt"

"gte"

"lt"

"lte"

"in"

"nin"

value: string or number or boolean or array of string or number

The value to compare against the attribute key; supports string, number, or boolean types.

One of the following:

string

number

boolean

array of string or number

One of the following:

string

number

CompoundFilter object { filters, type }

Combine multiple filters using `and` or `or`.

filters: array of [ComparisonFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20comparison_filter%20%3E%20(schema)) { key, type, value }  or unknown

Array of filters to combine. Items can be `ComparisonFilter` or `CompoundFilter`.

One of the following:

ComparisonFilter object { key, type, value }

A filter used to compare a specified attribute key to a given value using a defined comparison operation.

key: string

The key to compare against the value.

type: "eq" or "ne" or "gt" or 5 more

Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`.

- `eq`: equals
- `ne`: not equal
- `gt`: greater than
- `gte`: greater than or equal
- `lt`: less than
- `lte`: less than or equal
- `in`: in
- `nin`: not in

One of the following:

"eq"

"ne"

"gt"

"gte"

"lt"

"lte"

"in"

"nin"

value: string or number or boolean or array of string or number

The value to compare against the attribute key; supports string, number, or boolean types.

One of the following:

string

number

boolean

array of string or number

One of the following:

string

number

unknown

type: "and" or "or"

Type of operation: `and` or `or`.

One of the following:

"and"

"or"

max\_num\_results: optional number

The maximum number of results to return. This number should be between 1 and 50 inclusive.

ranking\_options: optional object { hybrid\_search, ranker, score\_threshold }

Ranking options for search.

hybrid\_search: optional object { embedding\_weight, text\_weight }

Weights that control how reciprocal rank fusion balances semantic embedding matches versus sparse keyword matches when hybrid search is enabled.

embedding\_weight: number

The weight of the embedding in the reciprocal ranking fusion.

text\_weight: number

The weight of the text in the reciprocal ranking fusion.

ranker: optional "auto" or "default-2024-11-15"

The ranker to use for the file search.

One of the following:

"auto"

"default-2024-11-15"

score\_threshold: optional number

The score threshold for the file search, a number between 0 and 1. Numbers closer to 1 will attempt to return only the most relevant results, but may return fewer results.

Computer object { type }

A tool that controls a virtual computer. Learn more about the [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).

type: "computer"

The type of the computer tool. Always `computer`.

ComputerUsePreview object { display\_height, display\_width, environment, type }

A tool that controls a virtual computer. Learn more about the [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).

display\_height: number

The height of the computer display.

display\_width: number

The width of the computer display.

environment: "windows" or "mac" or "linux" or 2 more

The type of computer environment to control.

One of the following:

"windows"

"mac"

"linux"

"ubuntu"

"browser"

type: "computer\_use\_preview"

The type of the computer use tool. Always `computer_use_preview`.

WebSearch object { type, filters, search\_context\_size, user\_location }

Search the Internet for sources related to the prompt. Learn more about the
[web search tool](https://developers.openai.com/docs/guides/tools-web-search).

type: "web\_search" or "web\_search\_2025\_08\_26"

The type of the web search tool. One of `web_search` or `web_search_2025_08_26`.

One of the following:

"web\_search"

"web\_search\_2025\_08\_26"

filters: optional object { allowed\_domains }

Filters for the search.

allowed\_domains: optional array of string

Allowed domains for the search. If not provided, all domains are allowed.
Subdomains of the provided domains are allowed as well.

Example: `["pubmed.ncbi.nlm.nih.gov"]`

search\_context\_size: optional "low" or "medium" or "high"

High level guidance for the amount of context window space to use for the search. One of `low`, `medium`, or `high`. `medium` is the default.

One of the following:

"low"

"medium"

"high"

user\_location: optional object { city, country, region, 2 more }

The approximate location of the user.

city: optional string

Free text input for the city of the user, e.g. `San Francisco`.

country: optional string

The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of the user, e.g. `US`.

region: optional string

Free text input for the region of the user, e.g. `California`.

timezone: optional string

The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the user, e.g. `America/Los_Angeles`.

type: optional "approximate"

The type of location approximation. Always `approximate`.

Mcp object { server\_label, type, allowed\_tools, 7 more }

Give the model access to additional tools via remote Model Context Protocol
(MCP) servers. [Learn more about MCP](https://developers.openai.com/docs/guides/tools-remote-mcp).

server\_label: string

A label for this MCP server, used to identify it in tool calls.

type: "mcp"

The type of the MCP tool. Always `mcp`.

allowed\_tools: optional array of string or object { read\_only, tool\_names }

List of allowed tool names or a filter object.

One of the following:

McpAllowedTools = array of string

A string array of allowed tool names

McpToolFilter object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

authorization: optional string

An OAuth access token that can be used with a remote MCP server, either
with a custom MCP server URL or a service connector. Your application
must handle the OAuth authorization flow and provide the token here.

connector\_id: optional "connector\_dropbox" or "connector\_gmail" or "connector\_googlecalendar" or 5 more

Identifier for service connectors, like those available in ChatGPT. One of
`server_url` or `connector_id` must be provided. Learn more about service
connectors [here](https://developers.openai.com/docs/guides/tools-remote-mcp#connectors).

Currently supported `connector_id` values are:

- Dropbox: `connector_dropbox`
- Gmail: `connector_gmail`
- Google Calendar: `connector_googlecalendar`
- Google Drive: `connector_googledrive`
- Microsoft Teams: `connector_microsoftteams`
- Outlook Calendar: `connector_outlookcalendar`
- Outlook Email: `connector_outlookemail`
- SharePoint: `connector_sharepoint`

One of the following:

"connector\_dropbox"

"connector\_gmail"

"connector\_googlecalendar"

"connector\_googledrive"

"connector\_microsoftteams"

"connector\_outlookcalendar"

"connector\_outlookemail"

"connector\_sharepoint"

defer\_loading: optional boolean

Whether this MCP tool is deferred and discovered via tool search.

headers: optional map[string]

Optional HTTP headers to send to the MCP server. Use for authentication
or other purposes.

require\_approval: optional object { always, never }  or "always" or "never"

Specify which of the MCP server’s tools require approval.

One of the following:

McpToolApprovalFilter object { always, never }

Specify which of the MCP server’s tools require approval. Can be
`always`, `never`, or a filter object associated with tools
that require approval.

always: optional object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

never: optional object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

McpToolApprovalSetting = "always" or "never"

Specify a single approval policy for all tools. One of `always` or
`never`. When set to `always`, all tools will require approval. When
set to `never`, all tools will not require approval.

One of the following:

"always"

"never"

server\_description: optional string

Optional description of the MCP server, used to provide more context.

server\_url: optional string

The URL for the MCP server. One of `server_url` or `connector_id` must be
provided.

CodeInterpreter object { container, type }

A tool that runs Python code to help generate a response to a prompt.

container: string or object { type, file\_ids, memory\_limit, network\_policy }

The code interpreter container. Can be a container ID or an object that
specifies uploaded file IDs to make available to your code, along with an
optional `memory_limit` setting.

One of the following:

string

The container ID.

CodeInterpreterToolAuto object { type, file\_ids, memory\_limit, network\_policy }

Configuration for a code interpreter container. Optionally specify the IDs of the files to run the code on.

type: "auto"

Always `auto`.

file\_ids: optional array of string

An optional list of uploaded files to make available to your code.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit for the code interpreter container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional [ContainerNetworkPolicyDisabled](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_disabled%20%3E%20(schema)) { type }  or [ContainerNetworkPolicyAllowlist](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_allowlist%20%3E%20(schema)) { allowed\_domains, type, domain\_secrets }

Network access policy for the container.

One of the following:

ContainerNetworkPolicyDisabled object { type }

type: "disabled"

Disable outbound network access. Always `disabled`.

ContainerNetworkPolicyAllowlist object { allowed\_domains, type, domain\_secrets }

allowed\_domains: array of string

A list of allowed domains when type is `allowlist`.

type: "allowlist"

Allow outbound network access only to specified domains. Always `allowlist`.

domain\_secrets: optional array of [ContainerNetworkPolicyDomainSecret](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_domain_secret%20%3E%20(schema)) { domain, name, value }

Optional domain-scoped secrets for allowlisted domains.

domain: string

The domain associated with the secret.

minLength1

name: string

The name of the secret to inject for the domain.

minLength1

value: string

The secret value to inject for the domain.

maxLength10485760

minLength1

type: "code\_interpreter"

The type of the code interpreter tool. Always `code_interpreter`.

ImageGeneration object { type, action, background, 9 more }

A tool that generates images using the GPT image models.

type: "image\_generation"

The type of the image generation tool. Always `image_generation`.

action: optional "generate" or "edit" or "auto"

Whether to generate a new image or edit an existing image. Default: `auto`.

One of the following:

"generate"

"edit"

"auto"

background: optional "transparent" or "opaque" or "auto"

Background type for the generated image. One of `transparent`,
`opaque`, or `auto`. Default: `auto`.

One of the following:

"transparent"

"opaque"

"auto"

input\_fidelity: optional "high" or "low"

Control how much effort the model will exert to match the style and features, especially facial features, of input images. This parameter is only supported for `gpt-image-1` and `gpt-image-1.5` and later models, unsupported for `gpt-image-1-mini`. Supports `high` and `low`. Defaults to `low`.

One of the following:

"high"

"low"

input\_image\_mask: optional object { file\_id, image\_url }

Optional mask for inpainting. Contains `image_url`
(string, optional) and `file_id` (string, optional).

file\_id: optional string

File ID for the mask image.

image\_url: optional string

Base64-encoded mask image.

model: optional string or "gpt-image-1" or "gpt-image-1-mini" or "gpt-image-1.5"

The image generation model to use. Default: `gpt-image-1`.

One of the following:

string

"gpt-image-1" or "gpt-image-1-mini" or "gpt-image-1.5"

The image generation model to use. Default: `gpt-image-1`.

One of the following:

"gpt-image-1"

"gpt-image-1-mini"

"gpt-image-1.5"

moderation: optional "auto" or "low"

Moderation level for the generated image. Default: `auto`.

One of the following:

"auto"

"low"

output\_compression: optional number

Compression level for the output image. Default: 100.

minimum0

maximum100

output\_format: optional "png" or "webp" or "jpeg"

The output format of the generated image. One of `png`, `webp`, or
`jpeg`. Default: `png`.

One of the following:

"png"

"webp"

"jpeg"

partial\_images: optional number

Number of partial images to generate in streaming mode, from 0 (default value) to 3.

minimum0

maximum3

quality: optional "low" or "medium" or "high" or "auto"

The quality of the generated image. One of `low`, `medium`, `high`,
or `auto`. Default: `auto`.

One of the following:

"low"

"medium"

"high"

"auto"

size: optional "1024x1024" or "1024x1536" or "1536x1024" or "auto"

The size of the generated image. One of `1024x1024`, `1024x1536`,
`1536x1024`, or `auto`. Default: `auto`.

One of the following:

"1024x1024"

"1024x1536"

"1536x1024"

"auto"

LocalShell object { type }

A tool that allows the model to execute shell commands in a local environment.

type: "local\_shell"

The type of the local shell tool. Always `local_shell`.

Shell object { type, environment }

A tool that allows the model to execute shell commands.

type: "shell"

The type of the shell tool. Always `shell`.

environment: optional [ContainerAuto](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_auto%20%3E%20(schema)) { type, file\_ids, memory\_limit, 2 more }  or [LocalEnvironment](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20local_environment%20%3E%20(schema)) { type, skills }  or [ContainerReference](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_reference%20%3E%20(schema)) { container\_id, type }

One of the following:

ContainerAuto object { type, file\_ids, memory\_limit, 2 more }

type: "container\_auto"

Automatically creates a container for this request

file\_ids: optional array of string

An optional list of uploaded files to make available to your code.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit for the container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional [ContainerNetworkPolicyDisabled](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_disabled%20%3E%20(schema)) { type }  or [ContainerNetworkPolicyAllowlist](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_allowlist%20%3E%20(schema)) { allowed\_domains, type, domain\_secrets }

Network access policy for the container.

One of the following:

ContainerNetworkPolicyDisabled object { type }

type: "disabled"

Disable outbound network access. Always `disabled`.

ContainerNetworkPolicyAllowlist object { allowed\_domains, type, domain\_secrets }

allowed\_domains: array of string

A list of allowed domains when type is `allowlist`.

type: "allowlist"

Allow outbound network access only to specified domains. Always `allowlist`.

domain\_secrets: optional array of [ContainerNetworkPolicyDomainSecret](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_domain_secret%20%3E%20(schema)) { domain, name, value }

Optional domain-scoped secrets for allowlisted domains.

domain: string

The domain associated with the secret.

minLength1

name: string

The name of the secret to inject for the domain.

minLength1

value: string

The secret value to inject for the domain.

maxLength10485760

minLength1

skills: optional array of [SkillReference](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20skill_reference%20%3E%20(schema)) { skill\_id, type, version }  or [InlineSkill](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20inline_skill%20%3E%20(schema)) { description, name, source, type }

An optional list of skills referenced by id or inline data.

One of the following:

SkillReference object { skill\_id, type, version }

skill\_id: string

The ID of the referenced skill.

maxLength64

minLength1

type: "skill\_reference"

References a skill created with the /v1/skills endpoint.

version: optional string

Optional skill version. Use a positive integer or ‘latest’. Omit for default.

InlineSkill object { description, name, source, type }

description: string

The description of the skill.

name: string

The name of the skill.

source: [InlineSkillSource](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20inline_skill_source%20%3E%20(schema)) { data, media\_type, type }

Inline skill payload

type: "inline"

Defines an inline skill for this request.

LocalEnvironment object { type, skills }

type: "local"

Use a local computer environment.

skills: optional array of [LocalSkill](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20local_skill%20%3E%20(schema)) { description, name, path }

An optional list of skills.

description: string

The description of the skill.

name: string

The name of the skill.

path: string

The path to the directory containing the skill.

ContainerReference object { container\_id, type }

container\_id: string

The ID of the referenced container.

type: "container\_reference"

References a container created with the /v1/containers endpoint

Custom object { name, type, defer\_loading, 2 more }

A custom tool that processes input using a specified format. Learn more about [custom tools](https://developers.openai.com/docs/guides/function-calling#custom-tools)

name: string

The name of the custom tool, used to identify it in tool calls.

type: "custom"

The type of the custom tool. Always `custom`.

defer\_loading: optional boolean

Whether this tool should be deferred and discovered via tool search.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional [CustomToolInputFormat](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20custom_tool_input_format%20%3E%20(schema))

The input format for the custom tool. Default is unconstrained text.

Namespace object { description, name, tools, type }

Groups function/custom tools under a shared namespace.

description: string

A description of the namespace shown to the model.

minLength1

name: string

The namespace name used in tool calls (for example, `crm`).

minLength1

tools: array of object { name, type, defer\_loading, 3 more }  or object { name, type, defer\_loading, 2 more }

The function/custom tools available inside this namespace.

One of the following:

Function object { name, type, defer\_loading, 3 more }

name: string

maxLength128

minLength1

type: "function"

defer\_loading: optional boolean

Whether this function should be deferred and discovered via tool search.

description: optional string

parameters: optional unknown

strict: optional boolean

Custom object { name, type, defer\_loading, 2 more }

A custom tool that processes input using a specified format. Learn more about [custom tools](https://developers.openai.com/docs/guides/function-calling#custom-tools)

name: string

The name of the custom tool, used to identify it in tool calls.

type: "custom"

The type of the custom tool. Always `custom`.

defer\_loading: optional boolean

Whether this tool should be deferred and discovered via tool search.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional [CustomToolInputFormat](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20custom_tool_input_format%20%3E%20(schema))

The input format for the custom tool. Default is unconstrained text.

type: "namespace"

The type of the tool. Always `namespace`.

ToolSearch object { type, description, execution, parameters }

Hosted or BYOT tool search configuration for deferred tools.

type: "tool\_search"

The type of the tool. Always `tool_search`.

description: optional string

Description shown to the model for a client-executed tool search tool.

execution: optional "server" or "client"

Whether tool search is executed by the server or by the client.

One of the following:

"server"

"client"

parameters: optional unknown

Parameter schema for a client-executed tool search tool.

WebSearchPreview object { type, search\_content\_types, search\_context\_size, user\_location }

This tool searches the web for relevant results to use in a response. Learn more about the [web search tool](https://platform.openai.com/docs/guides/tools-web-search).

type: "web\_search\_preview" or "web\_search\_preview\_2025\_03\_11"

The type of the web search tool. One of `web_search_preview` or `web_search_preview_2025_03_11`.

One of the following:

"web\_search\_preview"

"web\_search\_preview\_2025\_03\_11"

search\_content\_types: optional array of "text" or "image"

One of the following:

"text"

"image"

search\_context\_size: optional "low" or "medium" or "high"

High level guidance for the amount of context window space to use for the search. One of `low`, `medium`, or `high`. `medium` is the default.

One of the following:

"low"

"medium"

"high"

user\_location: optional object { type, city, country, 2 more }

The user’s location.

type: "approximate"

The type of location approximation. Always `approximate`.

city: optional string

Free text input for the city of the user, e.g. `San Francisco`.

country: optional string

The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of the user, e.g. `US`.

region: optional string

Free text input for the region of the user, e.g. `California`.

timezone: optional string

The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the user, e.g. `America/Los_Angeles`.

ApplyPatch object { type }

Allows the assistant to create, delete, or update files using unified diffs.

type: "apply\_patch"

The type of the tool. Always `apply_patch`.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

error: [EvalAPIError](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20eval_api_error%20%3E%20(schema)) { code, message }

An object representing an error response from the Eval API.

eval\_id: string

The identifier of the associated evaluation.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: string

The model that is evaluated, if applicable.

name: string

The name of the evaluation run.

object: "eval.run"

The type of the object. Always “eval.run”.

per\_model\_usage: array of object { cached\_tokens, completion\_tokens, invocation\_count, 3 more }

Usage statistics for each model during the evaluation run.

cached\_tokens: number

The number of tokens retrieved from cache.

completion\_tokens: number

The number of completion tokens generated.

invocation\_count: number

The number of invocations.

model\_name: string

The name of the model.

prompt\_tokens: number

The number of prompt tokens used.

total\_tokens: number

The total number of tokens used.

per\_testing\_criteria\_results: array of object { failed, passed, testing\_criteria }

Results per testing criteria applied during the evaluation run.

failed: number

Number of tests failed for this criteria.

passed: number

Number of tests passed for this criteria.

testing\_criteria: string

A description of the testing criteria.

report\_url: string

The URL to the rendered evaluation run report on the UI dashboard.

result\_counts: object { errored, failed, passed, total }

Counters summarizing the outcomes of the evaluation run.

errored: number

Number of output items that resulted in an error.

failed: number

Number of output items that failed to pass the evaluation.

passed: number

Number of output items that passed the evaluation.

total: number

Total number of executed output items.

status: string

The status of the evaluation run.

RunCancelResponse object { id, created\_at, data\_source, 11 more }

A schema representing an evaluation run.

id: string

Unique identifier for the evaluation run.

created\_at: number

Unix timestamp (in seconds) when the evaluation run was created.

data\_source: [CreateEvalJSONLRunDataSource](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20create_eval_jsonl_run_data_source%20%3E%20(schema)) { source, type }  or [CreateEvalCompletionsRunDataSource](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20create_eval_completions_run_data_source%20%3E%20(schema)) { source, type, input\_messages, 2 more }  or object { source, type, input\_messages, 2 more }

Information about the run’s data source.

One of the following:

CreateEvalJSONLRunDataSource object { source, type }

A JsonlRunDataSource object with that specifies a JSONL file that matches the eval

source: object { content, type }  or object { id, type }

Determines what populates the `item` namespace in the data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

type: "jsonl"

The type of data source. Always `jsonl`.

CreateEvalCompletionsRunDataSource object { source, type, input\_messages, 2 more }

A CompletionsRunDataSource object describing a model sampling configuration.

source: object { content, type }  or object { id, type }  or object { type, created\_after, created\_before, 3 more }

Determines what populates the `item` namespace in this run’s data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

StoredCompletionsRunDataSource object { type, created\_after, created\_before, 3 more }

A StoredCompletionsRunDataSource configuration describing a set of filters

type: "stored\_completions"

The type of source. Always `stored_completions`.

created\_after: optional number

An optional Unix timestamp to filter items created after this time.

created\_before: optional number

An optional Unix timestamp to filter items created before this time.

limit: optional number

An optional maximum number of items to return.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: optional string

An optional model to filter by (e.g., ‘gpt-4o’).

type: "completions"

The type of run data source. Always `completions`.

input\_messages: optional object { template, type }  or object { item\_reference, type }

Used when sampling from a model. Dictates the structure of the messages passed into the model. Can either be a reference to a prebuilt trajectory (ie, `item.input_trajectory`), or a template with variable references to the `item` namespace.

One of the following:

TemplateInputMessages object { template, type }

template: array of [EasyInputMessage](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20easy_input_message%20%3E%20(schema)) { content, role, phase, type }  or object { content, role, type }

A list of chat messages forming the prompt or context. May include variable references to the `item` namespace, ie {{item.name}}.

One of the following:

EasyInputMessage object { content, role, phase, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

content: string or [ResponseInputMessageContentList](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_message_content_list%20%3E%20(schema)) { , ,  }

Text, image, or audio input to the model, used to generate a response.
Can also contain previous assistant responses.

One of the following:

TextInput = string

A text input to the model.

ResponseInputMessageContentList = array of [ResponseInputContent](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_input_content%20%3E%20(schema))

A list of one or many input items to the model, containing different content
types.

One of the following:

ResponseInputText object { text, type }

A text input to the model.

text: string

The text input to the model.

type: "input\_text"

The type of the input item. Always `input_text`.

ResponseInputImage object { detail, type, file\_id, image\_url }

An image input to the model. Learn about [image inputs](https://developers.openai.com/docs/guides/vision).

detail: "low" or "high" or "auto" or "original"

The detail level of the image to be sent to the model. One of `high`, `low`, `auto`, or `original`. Defaults to `auto`.

One of the following:

"low"

"high"

"auto"

"original"

type: "input\_image"

The type of the input item. Always `input_image`.

file\_id: optional string

The ID of the file to be sent to the model.

image\_url: optional string

The URL of the image to be sent to the model. A fully qualified URL or base64 encoded image in a data URL.

ResponseInputFile object { type, detail, file\_data, 3 more }

A file input to the model.

type: "input\_file"

The type of the input item. Always `input_file`.

detail: optional "low" or "high"

The detail level of the file to be sent to the model. Use `low` for the default rendering behavior, or `high` to render the file at higher quality. Defaults to `low`.

One of the following:

"low"

"high"

file\_data: optional string

The content of the file to be sent to the model.

file\_id: optional string

The ID of the file to be sent to the model.

file\_url: optional string

The URL of the file to be sent to the model.

filename: optional string

The name of the file to be sent to the model.

role: "user" or "assistant" or "system" or "developer"

The role of the message input. One of `user`, `assistant`, `system`, or
`developer`.

One of the following:

"user"

"assistant"

"system"

"developer"

phase: optional "commentary" or "final\_answer"

Labels an `assistant` message as intermediate commentary (`commentary`) or the final answer (`final_answer`).
For models like `gpt-5.3-codex` and beyond, when sending follow-up requests, preserve and resend
phase on all assistant messages — dropping it can degrade performance. Not used for user messages.

One of the following:

"commentary"

"final\_answer"

type: optional "message"

The type of the message input. Always `message`.

EvalMessageObject object { content, role, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

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

type: "template"

The type of input messages. Always `template`.

ItemReferenceInputMessages object { item\_reference, type }

item\_reference: string

A reference to a variable in the `item` namespace. Ie, “item.input\_trajectory”

type: "item\_reference"

The type of input messages. Always `item_reference`.

model: optional string

The name of the model to use for generating completions (e.g. “o3-mini”).

sampling\_params: optional object { max\_completion\_tokens, reasoning\_effort, response\_format, 4 more }

max\_completion\_tokens: optional number

The maximum number of tokens in the generated output.

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

response\_format: optional [ResponseFormatText](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_text%20%3E%20(schema)) { type }  or [ResponseFormatJSONSchema](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_schema%20%3E%20(schema)) { json\_schema, type }  or [ResponseFormatJSONObject](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20response_format_json_object%20%3E%20(schema)) { type }

An object specifying the format that the model must output.

Setting to `{ "type": "json_schema", "json_schema": {...} }` enables
Structured Outputs which ensures the model will match your supplied JSON
schema. Learn more in the [Structured Outputs
guide](https://developers.openai.com/docs/guides/structured-outputs).

Setting to `{ "type": "json_object" }` enables the older JSON mode, which
ensures the message the model generates is valid JSON. Using `json_schema`
is preferred for models that support it.

One of the following:

ResponseFormatText object { type }

Default response format. Used to generate text responses.

type: "text"

The type of response format being defined. Always `text`.

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

ResponseFormatJSONObject object { type }

JSON object response format. An older method of generating JSON responses.
Using `json_schema` is recommended for models that support it. Note that the
model will not generate JSON without a system or user message instructing it
to do so.

type: "json\_object"

The type of response format being defined. Always `json_object`.

seed: optional number

A seed value to initialize the randomness, during sampling.

temperature: optional number

A higher temperature increases randomness in the outputs.

tools: optional array of [ChatCompletionFunctionTool](https://developers.openai.com/api/reference/resources/chat#(resource)%20chat.completions%20%3E%20(model)%20chat_completion_function_tool%20%3E%20(schema)) { function, type }

A list of tools the model may call. Currently, only functions are supported as a tool. Use this to provide a list of functions the model may generate JSON inputs for. A max of 128 functions are supported.

function: [FunctionDefinition](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20function_definition%20%3E%20(schema)) { name, description, parameters, strict }

type: "function"

The type of the tool. Currently, only `function` is supported.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

ResponsesRunDataSource object { source, type, input\_messages, 2 more }

A ResponsesRunDataSource object describing a model sampling configuration.

source: object { content, type }  or object { id, type }  or object { type, created\_after, created\_before, 8 more }

Determines what populates the `item` namespace in this run’s data source.

One of the following:

EvalJSONLFileContentSource object { content, type }

content: array of object { item, sample }

The content of the jsonl file.

item: map[unknown]

sample: optional map[unknown]

type: "file\_content"

The type of jsonl source. Always `file_content`.

EvalJSONLFileIDSource object { id, type }

id: string

The identifier of the file.

type: "file\_id"

The type of jsonl source. Always `file_id`.

EvalResponsesSource object { type, created\_after, created\_before, 8 more }

A EvalResponsesSource object describing a run data source configuration.

type: "responses"

The type of run data source. Always `responses`.

created\_after: optional number

Only include items created after this timestamp (inclusive). This is a query parameter used to select responses.

minimum0

created\_before: optional number

Only include items created before this timestamp (inclusive). This is a query parameter used to select responses.

minimum0

instructions\_search: optional string

Optional string to search the ‘instructions’ field. This is a query parameter used to select responses.

metadata: optional unknown

Metadata filter for the responses. This is a query parameter used to select responses.

model: optional string

The name of the model to find responses for. This is a query parameter used to select responses.

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

temperature: optional number

Sampling temperature. This is a query parameter used to select responses.

tools: optional array of string

List of tool names. This is a query parameter used to select responses.

top\_p: optional number

Nucleus sampling parameter. This is a query parameter used to select responses.

users: optional array of string

List of user identifiers. This is a query parameter used to select responses.

type: "responses"

The type of run data source. Always `responses`.

input\_messages: optional object { template, type }  or object { item\_reference, type }

Used when sampling from a model. Dictates the structure of the messages passed into the model. Can either be a reference to a prebuilt trajectory (ie, `item.input_trajectory`), or a template with variable references to the `item` namespace.

One of the following:

InputMessagesTemplate object { template, type }

template: array of object { content, role }  or object { content, role, type }

A list of chat messages forming the prompt or context. May include variable references to the `item` namespace, ie {{item.name}}.

One of the following:

ChatMessage object { content, role }

content: string

The content of the message.

role: string

The role of the message (e.g. “system”, “assistant”, “user”).

EvalMessageObject object { content, role, type }

A message input to the model with a role indicating instruction following
hierarchy. Instructions given with the `developer` or `system` role take
precedence over instructions given with the `user` role. Messages with the
`assistant` role are presumed to have been generated by the model in previous
interactions.

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

type: "template"

The type of input messages. Always `template`.

InputMessagesItemReference object { item\_reference, type }

item\_reference: string

A reference to a variable in the `item` namespace. Ie, “item.name”

type: "item\_reference"

The type of input messages. Always `item_reference`.

model: optional string

The name of the model to use for generating completions (e.g. “o3-mini”).

sampling\_params: optional object { max\_completion\_tokens, reasoning\_effort, seed, 4 more }

max\_completion\_tokens: optional number

The maximum number of tokens in the generated output.

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

text: optional object { format }

Configuration options for a text response from the model. Can be plain
text or structured JSON data. Learn more:

- [Text inputs and outputs](https://developers.openai.com/docs/guides/text)
- [Structured Outputs](https://developers.openai.com/docs/guides/structured-outputs)

format: optional [ResponseFormatTextConfig](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20response_format_text_config%20%3E%20(schema))

An object specifying the format that the model must output.

Configuring `{ "type": "json_schema" }` enables Structured Outputs,
which ensures the model will match your supplied JSON schema. Learn more in the
[Structured Outputs guide](https://developers.openai.com/docs/guides/structured-outputs).

The default format is `{ "type": "text" }` with no additional options.

**Not recommended for gpt-4o and newer models:**

Setting to `{ "type": "json_object" }` enables the older JSON mode, which
ensures the message the model generates is valid JSON. Using `json_schema`
is preferred for models that support it.

tools: optional array of object { name, parameters, strict, 3 more }  or object { type, vector\_store\_ids, filters, 2 more }  or object { type }  or 12 more

An array of tools the model may call while generating a response. You
can specify which tool to use by setting the `tool_choice` parameter.

The two categories of tools you can provide the model are:

- **Built-in tools**: Tools that are provided by OpenAI that extend the
  model’s capabilities, like [web search](https://developers.openai.com/docs/guides/tools-web-search)
  or [file search](https://developers.openai.com/docs/guides/tools-file-search). Learn more about
  [built-in tools](https://developers.openai.com/docs/guides/tools).
- **Function calls (custom tools)**: Functions that are defined by you,
  enabling the model to call your own code. Learn more about
  [function calling](https://developers.openai.com/docs/guides/function-calling).

One of the following:

Function object { name, parameters, strict, 3 more }

Defines a function in your own code the model can choose to call. Learn more about [function calling](https://platform.openai.com/docs/guides/function-calling).

name: string

The name of the function to call.

parameters: map[unknown]

A JSON schema object describing the parameters of the function.

strict: boolean

Whether to enforce strict parameter validation. Default `true`.

type: "function"

The type of the function tool. Always `function`.

defer\_loading: optional boolean

Whether this function is deferred and loaded via tool search.

description: optional string

A description of the function. Used by the model to determine whether or not to call the function.

FileSearch object { type, vector\_store\_ids, filters, 2 more }

A tool that searches for relevant content from uploaded files. Learn more about the [file search tool](https://platform.openai.com/docs/guides/tools-file-search).

type: "file\_search"

The type of the file search tool. Always `file_search`.

vector\_store\_ids: array of string

The IDs of the vector stores to search.

filters: optional [ComparisonFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20comparison_filter%20%3E%20(schema)) { key, type, value }  or [CompoundFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20compound_filter%20%3E%20(schema)) { filters, type }

A filter to apply.

One of the following:

ComparisonFilter object { key, type, value }

A filter used to compare a specified attribute key to a given value using a defined comparison operation.

key: string

The key to compare against the value.

type: "eq" or "ne" or "gt" or 5 more

Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`.

- `eq`: equals
- `ne`: not equal
- `gt`: greater than
- `gte`: greater than or equal
- `lt`: less than
- `lte`: less than or equal
- `in`: in
- `nin`: not in

One of the following:

"eq"

"ne"

"gt"

"gte"

"lt"

"lte"

"in"

"nin"

value: string or number or boolean or array of string or number

The value to compare against the attribute key; supports string, number, or boolean types.

One of the following:

string

number

boolean

array of string or number

One of the following:

string

number

CompoundFilter object { filters, type }

Combine multiple filters using `and` or `or`.

filters: array of [ComparisonFilter](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20comparison_filter%20%3E%20(schema)) { key, type, value }  or unknown

Array of filters to combine. Items can be `ComparisonFilter` or `CompoundFilter`.

One of the following:

ComparisonFilter object { key, type, value }

A filter used to compare a specified attribute key to a given value using a defined comparison operation.

key: string

The key to compare against the value.

type: "eq" or "ne" or "gt" or 5 more

Specifies the comparison operator: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`.

- `eq`: equals
- `ne`: not equal
- `gt`: greater than
- `gte`: greater than or equal
- `lt`: less than
- `lte`: less than or equal
- `in`: in
- `nin`: not in

One of the following:

"eq"

"ne"

"gt"

"gte"

"lt"

"lte"

"in"

"nin"

value: string or number or boolean or array of string or number

The value to compare against the attribute key; supports string, number, or boolean types.

One of the following:

string

number

boolean

array of string or number

One of the following:

string

number

unknown

type: "and" or "or"

Type of operation: `and` or `or`.

One of the following:

"and"

"or"

max\_num\_results: optional number

The maximum number of results to return. This number should be between 1 and 50 inclusive.

ranking\_options: optional object { hybrid\_search, ranker, score\_threshold }

Ranking options for search.

hybrid\_search: optional object { embedding\_weight, text\_weight }

Weights that control how reciprocal rank fusion balances semantic embedding matches versus sparse keyword matches when hybrid search is enabled.

embedding\_weight: number

The weight of the embedding in the reciprocal ranking fusion.

text\_weight: number

The weight of the text in the reciprocal ranking fusion.

ranker: optional "auto" or "default-2024-11-15"

The ranker to use for the file search.

One of the following:

"auto"

"default-2024-11-15"

score\_threshold: optional number

The score threshold for the file search, a number between 0 and 1. Numbers closer to 1 will attempt to return only the most relevant results, but may return fewer results.

Computer object { type }

A tool that controls a virtual computer. Learn more about the [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).

type: "computer"

The type of the computer tool. Always `computer`.

ComputerUsePreview object { display\_height, display\_width, environment, type }

A tool that controls a virtual computer. Learn more about the [computer tool](https://platform.openai.com/docs/guides/tools-computer-use).

display\_height: number

The height of the computer display.

display\_width: number

The width of the computer display.

environment: "windows" or "mac" or "linux" or 2 more

The type of computer environment to control.

One of the following:

"windows"

"mac"

"linux"

"ubuntu"

"browser"

type: "computer\_use\_preview"

The type of the computer use tool. Always `computer_use_preview`.

WebSearch object { type, filters, search\_context\_size, user\_location }

Search the Internet for sources related to the prompt. Learn more about the
[web search tool](https://developers.openai.com/docs/guides/tools-web-search).

type: "web\_search" or "web\_search\_2025\_08\_26"

The type of the web search tool. One of `web_search` or `web_search_2025_08_26`.

One of the following:

"web\_search"

"web\_search\_2025\_08\_26"

filters: optional object { allowed\_domains }

Filters for the search.

allowed\_domains: optional array of string

Allowed domains for the search. If not provided, all domains are allowed.
Subdomains of the provided domains are allowed as well.

Example: `["pubmed.ncbi.nlm.nih.gov"]`

search\_context\_size: optional "low" or "medium" or "high"

High level guidance for the amount of context window space to use for the search. One of `low`, `medium`, or `high`. `medium` is the default.

One of the following:

"low"

"medium"

"high"

user\_location: optional object { city, country, region, 2 more }

The approximate location of the user.

city: optional string

Free text input for the city of the user, e.g. `San Francisco`.

country: optional string

The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of the user, e.g. `US`.

region: optional string

Free text input for the region of the user, e.g. `California`.

timezone: optional string

The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the user, e.g. `America/Los_Angeles`.

type: optional "approximate"

The type of location approximation. Always `approximate`.

Mcp object { server\_label, type, allowed\_tools, 7 more }

Give the model access to additional tools via remote Model Context Protocol
(MCP) servers. [Learn more about MCP](https://developers.openai.com/docs/guides/tools-remote-mcp).

server\_label: string

A label for this MCP server, used to identify it in tool calls.

type: "mcp"

The type of the MCP tool. Always `mcp`.

allowed\_tools: optional array of string or object { read\_only, tool\_names }

List of allowed tool names or a filter object.

One of the following:

McpAllowedTools = array of string

A string array of allowed tool names

McpToolFilter object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

authorization: optional string

An OAuth access token that can be used with a remote MCP server, either
with a custom MCP server URL or a service connector. Your application
must handle the OAuth authorization flow and provide the token here.

connector\_id: optional "connector\_dropbox" or "connector\_gmail" or "connector\_googlecalendar" or 5 more

Identifier for service connectors, like those available in ChatGPT. One of
`server_url` or `connector_id` must be provided. Learn more about service
connectors [here](https://developers.openai.com/docs/guides/tools-remote-mcp#connectors).

Currently supported `connector_id` values are:

- Dropbox: `connector_dropbox`
- Gmail: `connector_gmail`
- Google Calendar: `connector_googlecalendar`
- Google Drive: `connector_googledrive`
- Microsoft Teams: `connector_microsoftteams`
- Outlook Calendar: `connector_outlookcalendar`
- Outlook Email: `connector_outlookemail`
- SharePoint: `connector_sharepoint`

One of the following:

"connector\_dropbox"

"connector\_gmail"

"connector\_googlecalendar"

"connector\_googledrive"

"connector\_microsoftteams"

"connector\_outlookcalendar"

"connector\_outlookemail"

"connector\_sharepoint"

defer\_loading: optional boolean

Whether this MCP tool is deferred and discovered via tool search.

headers: optional map[string]

Optional HTTP headers to send to the MCP server. Use for authentication
or other purposes.

require\_approval: optional object { always, never }  or "always" or "never"

Specify which of the MCP server’s tools require approval.

One of the following:

McpToolApprovalFilter object { always, never }

Specify which of the MCP server’s tools require approval. Can be
`always`, `never`, or a filter object associated with tools
that require approval.

always: optional object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

never: optional object { read\_only, tool\_names }

A filter object to specify which tools are allowed.

read\_only: optional boolean

Indicates whether or not a tool modifies data or is read-only. If an
MCP server is [annotated with `readOnlyHint`](https://modelcontextprotocol.io/specification/2025-06-18/schema#toolannotations-readonlyhint),
it will match this filter.

tool\_names: optional array of string

List of allowed tool names.

McpToolApprovalSetting = "always" or "never"

Specify a single approval policy for all tools. One of `always` or
`never`. When set to `always`, all tools will require approval. When
set to `never`, all tools will not require approval.

One of the following:

"always"

"never"

server\_description: optional string

Optional description of the MCP server, used to provide more context.

server\_url: optional string

The URL for the MCP server. One of `server_url` or `connector_id` must be
provided.

CodeInterpreter object { container, type }

A tool that runs Python code to help generate a response to a prompt.

container: string or object { type, file\_ids, memory\_limit, network\_policy }

The code interpreter container. Can be a container ID or an object that
specifies uploaded file IDs to make available to your code, along with an
optional `memory_limit` setting.

One of the following:

string

The container ID.

CodeInterpreterToolAuto object { type, file\_ids, memory\_limit, network\_policy }

Configuration for a code interpreter container. Optionally specify the IDs of the files to run the code on.

type: "auto"

Always `auto`.

file\_ids: optional array of string

An optional list of uploaded files to make available to your code.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit for the code interpreter container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional [ContainerNetworkPolicyDisabled](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_disabled%20%3E%20(schema)) { type }  or [ContainerNetworkPolicyAllowlist](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_allowlist%20%3E%20(schema)) { allowed\_domains, type, domain\_secrets }

Network access policy for the container.

One of the following:

ContainerNetworkPolicyDisabled object { type }

type: "disabled"

Disable outbound network access. Always `disabled`.

ContainerNetworkPolicyAllowlist object { allowed\_domains, type, domain\_secrets }

allowed\_domains: array of string

A list of allowed domains when type is `allowlist`.

type: "allowlist"

Allow outbound network access only to specified domains. Always `allowlist`.

domain\_secrets: optional array of [ContainerNetworkPolicyDomainSecret](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_domain_secret%20%3E%20(schema)) { domain, name, value }

Optional domain-scoped secrets for allowlisted domains.

domain: string

The domain associated with the secret.

minLength1

name: string

The name of the secret to inject for the domain.

minLength1

value: string

The secret value to inject for the domain.

maxLength10485760

minLength1

type: "code\_interpreter"

The type of the code interpreter tool. Always `code_interpreter`.

ImageGeneration object { type, action, background, 9 more }

A tool that generates images using the GPT image models.

type: "image\_generation"

The type of the image generation tool. Always `image_generation`.

action: optional "generate" or "edit" or "auto"

Whether to generate a new image or edit an existing image. Default: `auto`.

One of the following:

"generate"

"edit"

"auto"

background: optional "transparent" or "opaque" or "auto"

Background type for the generated image. One of `transparent`,
`opaque`, or `auto`. Default: `auto`.

One of the following:

"transparent"

"opaque"

"auto"

input\_fidelity: optional "high" or "low"

Control how much effort the model will exert to match the style and features, especially facial features, of input images. This parameter is only supported for `gpt-image-1` and `gpt-image-1.5` and later models, unsupported for `gpt-image-1-mini`. Supports `high` and `low`. Defaults to `low`.

One of the following:

"high"

"low"

input\_image\_mask: optional object { file\_id, image\_url }

Optional mask for inpainting. Contains `image_url`
(string, optional) and `file_id` (string, optional).

file\_id: optional string

File ID for the mask image.

image\_url: optional string

Base64-encoded mask image.

model: optional string or "gpt-image-1" or "gpt-image-1-mini" or "gpt-image-1.5"

The image generation model to use. Default: `gpt-image-1`.

One of the following:

string

"gpt-image-1" or "gpt-image-1-mini" or "gpt-image-1.5"

The image generation model to use. Default: `gpt-image-1`.

One of the following:

"gpt-image-1"

"gpt-image-1-mini"

"gpt-image-1.5"

moderation: optional "auto" or "low"

Moderation level for the generated image. Default: `auto`.

One of the following:

"auto"

"low"

output\_compression: optional number

Compression level for the output image. Default: 100.

minimum0

maximum100

output\_format: optional "png" or "webp" or "jpeg"

The output format of the generated image. One of `png`, `webp`, or
`jpeg`. Default: `png`.

One of the following:

"png"

"webp"

"jpeg"

partial\_images: optional number

Number of partial images to generate in streaming mode, from 0 (default value) to 3.

minimum0

maximum3

quality: optional "low" or "medium" or "high" or "auto"

The quality of the generated image. One of `low`, `medium`, `high`,
or `auto`. Default: `auto`.

One of the following:

"low"

"medium"

"high"

"auto"

size: optional "1024x1024" or "1024x1536" or "1536x1024" or "auto"

The size of the generated image. One of `1024x1024`, `1024x1536`,
`1536x1024`, or `auto`. Default: `auto`.

One of the following:

"1024x1024"

"1024x1536"

"1536x1024"

"auto"

LocalShell object { type }

A tool that allows the model to execute shell commands in a local environment.

type: "local\_shell"

The type of the local shell tool. Always `local_shell`.

Shell object { type, environment }

A tool that allows the model to execute shell commands.

type: "shell"

The type of the shell tool. Always `shell`.

environment: optional [ContainerAuto](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_auto%20%3E%20(schema)) { type, file\_ids, memory\_limit, 2 more }  or [LocalEnvironment](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20local_environment%20%3E%20(schema)) { type, skills }  or [ContainerReference](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_reference%20%3E%20(schema)) { container\_id, type }

One of the following:

ContainerAuto object { type, file\_ids, memory\_limit, 2 more }

type: "container\_auto"

Automatically creates a container for this request

file\_ids: optional array of string

An optional list of uploaded files to make available to your code.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit for the container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional [ContainerNetworkPolicyDisabled](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_disabled%20%3E%20(schema)) { type }  or [ContainerNetworkPolicyAllowlist](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_allowlist%20%3E%20(schema)) { allowed\_domains, type, domain\_secrets }

Network access policy for the container.

One of the following:

ContainerNetworkPolicyDisabled object { type }

type: "disabled"

Disable outbound network access. Always `disabled`.

ContainerNetworkPolicyAllowlist object { allowed\_domains, type, domain\_secrets }

allowed\_domains: array of string

A list of allowed domains when type is `allowlist`.

type: "allowlist"

Allow outbound network access only to specified domains. Always `allowlist`.

domain\_secrets: optional array of [ContainerNetworkPolicyDomainSecret](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20container_network_policy_domain_secret%20%3E%20(schema)) { domain, name, value }

Optional domain-scoped secrets for allowlisted domains.

domain: string

The domain associated with the secret.

minLength1

name: string

The name of the secret to inject for the domain.

minLength1

value: string

The secret value to inject for the domain.

maxLength10485760

minLength1

skills: optional array of [SkillReference](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20skill_reference%20%3E%20(schema)) { skill\_id, type, version }  or [InlineSkill](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20inline_skill%20%3E%20(schema)) { description, name, source, type }

An optional list of skills referenced by id or inline data.

One of the following:

SkillReference object { skill\_id, type, version }

skill\_id: string

The ID of the referenced skill.

maxLength64

minLength1

type: "skill\_reference"

References a skill created with the /v1/skills endpoint.

version: optional string

Optional skill version. Use a positive integer or ‘latest’. Omit for default.

InlineSkill object { description, name, source, type }

description: string

The description of the skill.

name: string

The name of the skill.

source: [InlineSkillSource](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20inline_skill_source%20%3E%20(schema)) { data, media\_type, type }

Inline skill payload

type: "inline"

Defines an inline skill for this request.

LocalEnvironment object { type, skills }

type: "local"

Use a local computer environment.

skills: optional array of [LocalSkill](https://developers.openai.com/api/reference/resources/responses#(resource)%20responses%20%3E%20(model)%20local_skill%20%3E%20(schema)) { description, name, path }

An optional list of skills.

description: string

The description of the skill.

name: string

The name of the skill.

path: string

The path to the directory containing the skill.

ContainerReference object { container\_id, type }

container\_id: string

The ID of the referenced container.

type: "container\_reference"

References a container created with the /v1/containers endpoint

Custom object { name, type, defer\_loading, 2 more }

A custom tool that processes input using a specified format. Learn more about [custom tools](https://developers.openai.com/docs/guides/function-calling#custom-tools)

name: string

The name of the custom tool, used to identify it in tool calls.

type: "custom"

The type of the custom tool. Always `custom`.

defer\_loading: optional boolean

Whether this tool should be deferred and discovered via tool search.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional [CustomToolInputFormat](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20custom_tool_input_format%20%3E%20(schema))

The input format for the custom tool. Default is unconstrained text.

Namespace object { description, name, tools, type }

Groups function/custom tools under a shared namespace.

description: string

A description of the namespace shown to the model.

minLength1

name: string

The namespace name used in tool calls (for example, `crm`).

minLength1

tools: array of object { name, type, defer\_loading, 3 more }  or object { name, type, defer\_loading, 2 more }

The function/custom tools available inside this namespace.

One of the following:

Function object { name, type, defer\_loading, 3 more }

name: string

maxLength128

minLength1

type: "function"

defer\_loading: optional boolean

Whether this function should be deferred and discovered via tool search.

description: optional string

parameters: optional unknown

strict: optional boolean

Custom object { name, type, defer\_loading, 2 more }

A custom tool that processes input using a specified format. Learn more about [custom tools](https://developers.openai.com/docs/guides/function-calling#custom-tools)

name: string

The name of the custom tool, used to identify it in tool calls.

type: "custom"

The type of the custom tool. Always `custom`.

defer\_loading: optional boolean

Whether this tool should be deferred and discovered via tool search.

description: optional string

Optional description of the custom tool, used to provide more context.

format: optional [CustomToolInputFormat](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20custom_tool_input_format%20%3E%20(schema))

The input format for the custom tool. Default is unconstrained text.

type: "namespace"

The type of the tool. Always `namespace`.

ToolSearch object { type, description, execution, parameters }

Hosted or BYOT tool search configuration for deferred tools.

type: "tool\_search"

The type of the tool. Always `tool_search`.

description: optional string

Description shown to the model for a client-executed tool search tool.

execution: optional "server" or "client"

Whether tool search is executed by the server or by the client.

One of the following:

"server"

"client"

parameters: optional unknown

Parameter schema for a client-executed tool search tool.

WebSearchPreview object { type, search\_content\_types, search\_context\_size, user\_location }

This tool searches the web for relevant results to use in a response. Learn more about the [web search tool](https://platform.openai.com/docs/guides/tools-web-search).

type: "web\_search\_preview" or "web\_search\_preview\_2025\_03\_11"

The type of the web search tool. One of `web_search_preview` or `web_search_preview_2025_03_11`.

One of the following:

"web\_search\_preview"

"web\_search\_preview\_2025\_03\_11"

search\_content\_types: optional array of "text" or "image"

One of the following:

"text"

"image"

search\_context\_size: optional "low" or "medium" or "high"

High level guidance for the amount of context window space to use for the search. One of `low`, `medium`, or `high`. `medium` is the default.

One of the following:

"low"

"medium"

"high"

user\_location: optional object { type, city, country, 2 more }

The user’s location.

type: "approximate"

The type of location approximation. Always `approximate`.

city: optional string

Free text input for the city of the user, e.g. `San Francisco`.

country: optional string

The two-letter [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1) of the user, e.g. `US`.

region: optional string

Free text input for the region of the user, e.g. `California`.

timezone: optional string

The [IANA timezone](https://timeapi.io/documentation/iana-timezones) of the user, e.g. `America/Los_Angeles`.

ApplyPatch object { type }

Allows the assistant to create, delete, or update files using unified diffs.

type: "apply\_patch"

The type of the tool. Always `apply_patch`.

top\_p: optional number

An alternative to temperature for nucleus sampling; 1.0 includes all tokens.

error: [EvalAPIError](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20eval_api_error%20%3E%20(schema)) { code, message }

An object representing an error response from the Eval API.

eval\_id: string

The identifier of the associated evaluation.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

model: string

The model that is evaluated, if applicable.

name: string

The name of the evaluation run.

object: "eval.run"

The type of the object. Always “eval.run”.

per\_model\_usage: array of object { cached\_tokens, completion\_tokens, invocation\_count, 3 more }

Usage statistics for each model during the evaluation run.

cached\_tokens: number

The number of tokens retrieved from cache.

completion\_tokens: number

The number of completion tokens generated.

invocation\_count: number

The number of invocations.

model\_name: string

The name of the model.

prompt\_tokens: number

The number of prompt tokens used.

total\_tokens: number

The total number of tokens used.

per\_testing\_criteria\_results: array of object { failed, passed, testing\_criteria }

Results per testing criteria applied during the evaluation run.

failed: number

Number of tests failed for this criteria.

passed: number

Number of tests passed for this criteria.

testing\_criteria: string

A description of the testing criteria.

report\_url: string

The URL to the rendered evaluation run report on the UI dashboard.

result\_counts: object { errored, failed, passed, total }

Counters summarizing the outcomes of the evaluation run.

errored: number

Number of output items that resulted in an error.

failed: number

Number of output items that failed to pass the evaluation.

passed: number

Number of output items that passed the evaluation.

total: number

Total number of executed output items.

status: string

The status of the evaluation run.

RunDeleteResponse object { deleted, object, run\_id }

deleted: optional boolean

object: optional string

run\_id: optional string

#### EvalsRunsOutput Items

Manage and run evals in the OpenAI platform.

##### [Get eval run output items](https://developers.openai.com/api/reference/resources/evals/subresources/runs/subresources/output_items/methods/list)

GET/evals/{eval\_id}/runs/{run\_id}/output\_items

##### [Get an output item of an eval run](https://developers.openai.com/api/reference/resources/evals/subresources/runs/subresources/output_items/methods/retrieve)

GET/evals/{eval\_id}/runs/{run\_id}/output\_items/{output\_item\_id}

##### Models

OutputItemListResponse object { id, created\_at, datasource\_item, 7 more }

A schema representing an evaluation run output item.

id: string

Unique identifier for the evaluation run output item.

created\_at: number

Unix timestamp (in seconds) when the evaluation run was created.

datasource\_item: map[unknown]

Details of the input data source item.

datasource\_item\_id: number

The identifier for the data source item.

eval\_id: string

The identifier of the evaluation group.

object: "eval.run.output\_item"

The type of the object. Always “eval.run.output\_item”.

results: array of object { name, passed, score, 2 more }

A list of grader results for this output item.

name: string

The name of the grader.

passed: boolean

Whether the grader considered the output a pass.

score: number

The numeric score produced by the grader.

sample: optional map[unknown]

Optional sample or intermediate data produced by the grader.

type: optional string

The grader type (for example, “string-check-grader”).

run\_id: string

The identifier of the evaluation run associated with this output item.

sample: object { error, finish\_reason, input, 7 more }

A sample containing the input and output of the evaluation run.

error: [EvalAPIError](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20eval_api_error%20%3E%20(schema)) { code, message }

An object representing an error response from the Eval API.

finish\_reason: string

The reason why the sample generation was finished.

input: array of object { content, role }

An array of input messages.

content: string

The content of the message.

role: string

The role of the message sender (e.g., system, user, developer).

max\_completion\_tokens: number

The maximum number of tokens allowed for completion.

model: string

The model used for generating the sample.

output: array of object { content, role }

An array of output messages.

content: optional string

The content of the message.

role: optional string

The role of the message (e.g. “system”, “assistant”, “user”).

seed: number

The seed used for generating the sample.

temperature: number

The sampling temperature used.

top\_p: number

The top\_p value used for sampling.

usage: object { cached\_tokens, completion\_tokens, prompt\_tokens, total\_tokens }

Token usage details for the sample.

cached\_tokens: number

The number of tokens retrieved from cache.

completion\_tokens: number

The number of completion tokens generated.

prompt\_tokens: number

The number of prompt tokens used.

total\_tokens: number

The total number of tokens used.

status: string

The status of the evaluation run.

OutputItemRetrieveResponse object { id, created\_at, datasource\_item, 7 more }

A schema representing an evaluation run output item.

id: string

Unique identifier for the evaluation run output item.

created\_at: number

Unix timestamp (in seconds) when the evaluation run was created.

datasource\_item: map[unknown]

Details of the input data source item.

datasource\_item\_id: number

The identifier for the data source item.

eval\_id: string

The identifier of the evaluation group.

object: "eval.run.output\_item"

The type of the object. Always “eval.run.output\_item”.

results: array of object { name, passed, score, 2 more }

A list of grader results for this output item.

name: string

The name of the grader.

passed: boolean

Whether the grader considered the output a pass.

score: number

The numeric score produced by the grader.

sample: optional map[unknown]

Optional sample or intermediate data produced by the grader.

type: optional string

The grader type (for example, “string-check-grader”).

run\_id: string

The identifier of the evaluation run associated with this output item.

sample: object { error, finish\_reason, input, 7 more }

A sample containing the input and output of the evaluation run.

error: [EvalAPIError](https://developers.openai.com/api/reference/resources/evals#(resource)%20evals.runs%20%3E%20(model)%20eval_api_error%20%3E%20(schema)) { code, message }

An object representing an error response from the Eval API.

finish\_reason: string

The reason why the sample generation was finished.

input: array of object { content, role }

An array of input messages.

content: string

The content of the message.

role: string

The role of the message sender (e.g., system, user, developer).

max\_completion\_tokens: number

The maximum number of tokens allowed for completion.

model: string

The model used for generating the sample.

output: array of object { content, role }

An array of output messages.

content: optional string

The content of the message.

role: optional string

The role of the message (e.g. “system”, “assistant”, “user”).

seed: number

The seed used for generating the sample.

temperature: number

The sampling temperature used.

top\_p: number

The top\_p value used for sampling.

usage: object { cached\_tokens, completion\_tokens, prompt\_tokens, total\_tokens }

Token usage details for the sample.

cached\_tokens: number

The number of tokens retrieved from cache.

completion\_tokens: number

The number of completion tokens generated.

prompt\_tokens: number

The number of prompt tokens used.

total\_tokens: number

The total number of tokens used.

status: string

The status of the evaluation run.
