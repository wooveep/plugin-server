# Fine Tuning

> Source: https://developers.openai.com/api/reference/resources/fine_tuning
> Fetched: 2026-04-23

#### Fine TuningMethods

##### Models

DpoHyperparameters object { batch\_size, beta, learning\_rate\_multiplier, n\_epochs }

The hyperparameters used for the DPO fine-tuning job.

batch\_size: optional "auto" or number

Number of examples in each batch. A larger batch size means that model parameters are updated less frequently, but with lower variance.

One of the following:

"auto"

number

beta: optional "auto" or number

The beta value for the DPO method. A higher beta value will increase the weight of the penalty between the policy and reference model.

One of the following:

"auto"

number

learning\_rate\_multiplier: optional "auto" or number

Scaling factor for the learning rate. A smaller learning rate may be useful to avoid overfitting.

One of the following:

"auto"

number

n\_epochs: optional "auto" or number

The number of epochs to train the model for. An epoch refers to one full cycle through the training dataset.

One of the following:

"auto"

number

DpoMethod object { hyperparameters }

Configuration for the DPO fine-tuning method.

hyperparameters: optional [DpoHyperparameters](https://developers.openai.com/api/reference/resources/fine_tuning#(resource)%20fine_tuning.methods%20%3E%20(model)%20dpo_hyperparameters%20%3E%20(schema)) { batch\_size, beta, learning\_rate\_multiplier, n\_epochs }

The hyperparameters used for the DPO fine-tuning job.

ReinforcementHyperparameters object { batch\_size, compute\_multiplier, eval\_interval, 4 more }

The hyperparameters used for the reinforcement fine-tuning job.

batch\_size: optional "auto" or number

Number of examples in each batch. A larger batch size means that model parameters are updated less frequently, but with lower variance.

One of the following:

"auto"

number

compute\_multiplier: optional "auto" or number

Multiplier on amount of compute used for exploring search space during training.

One of the following:

"auto"

number

eval\_interval: optional "auto" or number

The number of training steps between evaluation runs.

One of the following:

"auto"

number

eval\_samples: optional "auto" or number

Number of evaluation samples to generate per training step.

One of the following:

"auto"

number

learning\_rate\_multiplier: optional "auto" or number

Scaling factor for the learning rate. A smaller learning rate may be useful to avoid overfitting.

One of the following:

"auto"

number

n\_epochs: optional "auto" or number

The number of epochs to train the model for. An epoch refers to one full cycle through the training dataset.

One of the following:

"auto"

number

reasoning\_effort: optional "default" or "low" or "medium" or "high"

Level of reasoning effort.

One of the following:

"default"

"low"

"medium"

"high"

ReinforcementMethod object { grader, hyperparameters }

Configuration for the reinforcement fine-tuning method.

grader: [StringCheckGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20string_check_grader%20%3E%20(schema)) { input, name, operation, 2 more }  or [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }  or [PythonGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20python_grader%20%3E%20(schema)) { name, source, type, image\_tag }  or 2 more

The grader used for the fine-tuning job.

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

hyperparameters: optional [ReinforcementHyperparameters](https://developers.openai.com/api/reference/resources/fine_tuning#(resource)%20fine_tuning.methods%20%3E%20(model)%20reinforcement_hyperparameters%20%3E%20(schema)) { batch\_size, compute\_multiplier, eval\_interval, 4 more }

The hyperparameters used for the reinforcement fine-tuning job.

SupervisedHyperparameters object { batch\_size, learning\_rate\_multiplier, n\_epochs }

The hyperparameters used for the fine-tuning job.

batch\_size: optional "auto" or number

Number of examples in each batch. A larger batch size means that model parameters are updated less frequently, but with lower variance.

One of the following:

"auto"

number

learning\_rate\_multiplier: optional "auto" or number

Scaling factor for the learning rate. A smaller learning rate may be useful to avoid overfitting.

One of the following:

"auto"

number

n\_epochs: optional "auto" or number

The number of epochs to train the model for. An epoch refers to one full cycle through the training dataset.

One of the following:

"auto"

number

SupervisedMethod object { hyperparameters }

Configuration for the supervised fine-tuning method.

hyperparameters: optional [SupervisedHyperparameters](https://developers.openai.com/api/reference/resources/fine_tuning#(resource)%20fine_tuning.methods%20%3E%20(model)%20supervised_hyperparameters%20%3E%20(schema)) { batch\_size, learning\_rate\_multiplier, n\_epochs }

The hyperparameters used for the fine-tuning job.

#### Fine TuningJobs

Manage fine-tuning jobs to tailor a model to your specific training data.

##### [Create fine-tuning job](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/create)

POST/fine\_tuning/jobs

##### [List fine-tuning jobs](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/list)

GET/fine\_tuning/jobs

##### [Retrieve fine-tuning job](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/retrieve)

GET/fine\_tuning/jobs/{fine\_tuning\_job\_id}

##### [List fine-tuning events](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/list_events)

GET/fine\_tuning/jobs/{fine\_tuning\_job\_id}/events

##### [Cancel fine-tuning](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/cancel)

POST/fine\_tuning/jobs/{fine\_tuning\_job\_id}/cancel

##### [Pause fine-tuning](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/pause)

POST/fine\_tuning/jobs/{fine\_tuning\_job\_id}/pause

##### [Resume fine-tuning](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/methods/resume)

POST/fine\_tuning/jobs/{fine\_tuning\_job\_id}/resume

##### Models

FineTuningJob object { id, created\_at, error, 16 more }

The `fine_tuning.job` object represents a fine-tuning job that has been created through the API.

id: string

The object identifier, which can be referenced in the API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the fine-tuning job was created.

error: object { code, message, param }

For fine-tuning jobs that have `failed`, this will contain more information on the cause of the failure.

code: string

A machine-readable error code.

message: string

A human-readable error message.

param: string

The parameter that was invalid, usually `training_file` or `validation_file`. This field will be null if the failure was not parameter-specific.

fine\_tuned\_model: string

The name of the fine-tuned model that is being created. The value will be null if the fine-tuning job is still running.

finished\_at: number

The Unix timestamp (in seconds) for when the fine-tuning job was finished. The value will be null if the fine-tuning job is still running.

hyperparameters: object { batch\_size, learning\_rate\_multiplier, n\_epochs }

The hyperparameters used for the fine-tuning job. This value will only be returned when running `supervised` jobs.

batch\_size: optional "auto" or number

Number of examples in each batch. A larger batch size means that model parameters
are updated less frequently, but with lower variance.

One of the following:

"auto"

number

learning\_rate\_multiplier: optional "auto" or number

Scaling factor for the learning rate. A smaller learning rate may be useful to avoid
overfitting.

One of the following:

"auto"

number

n\_epochs: optional "auto" or number

The number of epochs to train the model for. An epoch refers to one full cycle
through the training dataset.

One of the following:

"auto"

number

model: string

The base model that is being fine-tuned.

object: "fine\_tuning.job"

The object type, which is always “fine\_tuning.job”.

organization\_id: string

The organization that owns the fine-tuning job.

result\_files: array of string

The compiled results file ID(s) for the fine-tuning job. You can retrieve the results with the [Files API](https://developers.openai.com/docs/api-reference/files/retrieve-contents).

seed: number

The seed used for the fine-tuning job.

status: "validating\_files" or "queued" or "running" or 3 more

The current status of the fine-tuning job, which can be either `validating_files`, `queued`, `running`, `succeeded`, `failed`, or `cancelled`.

One of the following:

"validating\_files"

"queued"

"running"

"succeeded"

"failed"

"cancelled"

trained\_tokens: number

The total number of billable tokens processed by this fine-tuning job. The value will be null if the fine-tuning job is still running.

training\_file: string

The file ID used for training. You can retrieve the training data with the [Files API](https://developers.openai.com/docs/api-reference/files/retrieve-contents).

validation\_file: string

The file ID used for validation. You can retrieve the validation results with the [Files API](https://developers.openai.com/docs/api-reference/files/retrieve-contents).

estimated\_finish: optional number

The Unix timestamp (in seconds) for when the fine-tuning job is estimated to finish. The value will be null if the fine-tuning job is not running.

integrations: optional array of [FineTuningJobWandbIntegrationObject](https://developers.openai.com/api/reference/resources/fine_tuning#(resource)%20fine_tuning.jobs%20%3E%20(model)%20fine_tuning_job_wandb_integration_object%20%3E%20(schema)) { type, wandb }

A list of integrations to enable for this fine-tuning job.

type: "wandb"

The type of the integration being enabled for the fine-tuning job

wandb: [FineTuningJobWandbIntegration](https://developers.openai.com/api/reference/resources/fine_tuning#(resource)%20fine_tuning.jobs%20%3E%20(model)%20fine_tuning_job_wandb_integration%20%3E%20(schema)) { project, entity, name, tags }

The settings for your integration with Weights and Biases. This payload specifies the project that
metrics will be sent to. Optionally, you can set an explicit display name for your run, add tags
to your run, and set a default entity (team, username, etc) to be associated with your run.

metadata: optional [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

method: optional object { type, dpo, reinforcement, supervised }

The method used for fine-tuning.

type: "supervised" or "dpo" or "reinforcement"

The type of method. Is either `supervised`, `dpo`, or `reinforcement`.

One of the following:

"supervised"

"dpo"

"reinforcement"

dpo: optional [DpoMethod](https://developers.openai.com/api/reference/resources/fine_tuning#(resource)%20fine_tuning.methods%20%3E%20(model)%20dpo_method%20%3E%20(schema)) { hyperparameters }

Configuration for the DPO fine-tuning method.

reinforcement: optional [ReinforcementMethod](https://developers.openai.com/api/reference/resources/fine_tuning#(resource)%20fine_tuning.methods%20%3E%20(model)%20reinforcement_method%20%3E%20(schema)) { grader, hyperparameters }

Configuration for the reinforcement fine-tuning method.

supervised: optional [SupervisedMethod](https://developers.openai.com/api/reference/resources/fine_tuning#(resource)%20fine_tuning.methods%20%3E%20(model)%20supervised_method%20%3E%20(schema)) { hyperparameters }

Configuration for the supervised fine-tuning method.

FineTuningJobEvent object { id, created\_at, level, 4 more }

Fine-tuning job event object

id: string

The object identifier.

created\_at: number

The Unix timestamp (in seconds) for when the fine-tuning job was created.

level: "info" or "warn" or "error"

The log level of the event.

One of the following:

"info"

"warn"

"error"

message: string

The message of the event.

object: "fine\_tuning.job.event"

The object type, which is always “fine\_tuning.job.event”.

data: optional unknown

The data associated with the event.

type: optional "message" or "metrics"

The type of event.

One of the following:

"message"

"metrics"

FineTuningJobWandbIntegration object { project, entity, name, tags }

The settings for your integration with Weights and Biases. This payload specifies the project that
metrics will be sent to. Optionally, you can set an explicit display name for your run, add tags
to your run, and set a default entity (team, username, etc) to be associated with your run.

project: string

The name of the project that the new run will be created under.

entity: optional string

The entity to use for the run. This allows you to set the team or username of the WandB user that you would
like associated with the run. If not set, the default entity for the registered WandB API key is used.

name: optional string

A display name to set for the run. If not set, we will use the Job ID as the name.

tags: optional array of string

A list of tags to be attached to the newly created run. These tags are passed through directly to WandB. Some
default tags are generated by OpenAI: “openai/finetune”, “openai/{base-model}”, “openai/{ftjob-abcdef}”.

FineTuningJobWandbIntegrationObject object { type, wandb }

type: "wandb"

The type of the integration being enabled for the fine-tuning job

wandb: [FineTuningJobWandbIntegration](https://developers.openai.com/api/reference/resources/fine_tuning#(resource)%20fine_tuning.jobs%20%3E%20(model)%20fine_tuning_job_wandb_integration%20%3E%20(schema)) { project, entity, name, tags }

The settings for your integration with Weights and Biases. This payload specifies the project that
metrics will be sent to. Optionally, you can set an explicit display name for your run, add tags
to your run, and set a default entity (team, username, etc) to be associated with your run.

#### Fine TuningJobsCheckpoints

Manage fine-tuning jobs to tailor a model to your specific training data.

##### [List fine-tuning checkpoints](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/jobs/subresources/checkpoints/methods/list)

GET/fine\_tuning/jobs/{fine\_tuning\_job\_id}/checkpoints

##### Models

FineTuningJobCheckpoint object { id, created\_at, fine\_tuned\_model\_checkpoint, 4 more }

The `fine_tuning.job.checkpoint` object represents a model checkpoint for a fine-tuning job that is ready to use.

id: string

The checkpoint identifier, which can be referenced in the API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the checkpoint was created.

fine\_tuned\_model\_checkpoint: string

The name of the fine-tuned checkpoint model that is created.

fine\_tuning\_job\_id: string

The name of the fine-tuning job that this checkpoint was created from.

metrics: object { full\_valid\_loss, full\_valid\_mean\_token\_accuracy, step, 4 more }

Metrics at the step number during the fine-tuning job.

full\_valid\_loss: optional number

full\_valid\_mean\_token\_accuracy: optional number

step: optional number

train\_loss: optional number

train\_mean\_token\_accuracy: optional number

valid\_loss: optional number

valid\_mean\_token\_accuracy: optional number

object: "fine\_tuning.job.checkpoint"

The object type, which is always “fine\_tuning.job.checkpoint”.

step\_number: number

The step number that the checkpoint was created at.

#### Fine TuningCheckpoints

#### Fine TuningCheckpointsPermissions

Manage fine-tuning jobs to tailor a model to your specific training data.

##### [List checkpoint permissions](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/checkpoints/subresources/permissions/methods/retrieve)

Deprecated

GET/fine\_tuning/checkpoints/{fine\_tuned\_model\_checkpoint}/permissions

##### [List checkpoint permissions](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/checkpoints/subresources/permissions/methods/list)

GET/fine\_tuning/checkpoints/{fine\_tuned\_model\_checkpoint}/permissions

##### [Create checkpoint permissions](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/checkpoints/subresources/permissions/methods/create)

POST/fine\_tuning/checkpoints/{fine\_tuned\_model\_checkpoint}/permissions

##### [Delete checkpoint permission](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/checkpoints/subresources/permissions/methods/delete)

DELETE/fine\_tuning/checkpoints/{fine\_tuned\_model\_checkpoint}/permissions/{permission\_id}

##### Models

PermissionRetrieveResponse object { data, has\_more, object, 2 more }

data: array of object { id, created\_at, object, project\_id }

id: string

The permission identifier, which can be referenced in the API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the permission was created.

object: "checkpoint.permission"

The object type, which is always “checkpoint.permission”.

project\_id: string

The project identifier that the permission is for.

has\_more: boolean

object: "list"

first\_id: optional string

last\_id: optional string

PermissionListResponse object { id, created\_at, object, project\_id }

The `checkpoint.permission` object represents a permission for a fine-tuned model checkpoint.

id: string

The permission identifier, which can be referenced in the API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the permission was created.

object: "checkpoint.permission"

The object type, which is always “checkpoint.permission”.

project\_id: string

The project identifier that the permission is for.

PermissionCreateResponse object { id, created\_at, object, project\_id }

The `checkpoint.permission` object represents a permission for a fine-tuned model checkpoint.

id: string

The permission identifier, which can be referenced in the API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the permission was created.

object: "checkpoint.permission"

The object type, which is always “checkpoint.permission”.

project\_id: string

The project identifier that the permission is for.

PermissionDeleteResponse object { id, deleted, object }

id: string

The ID of the fine-tuned model checkpoint permission that was deleted.

deleted: boolean

Whether the fine-tuned model checkpoint permission was successfully deleted.

object: "checkpoint.permission"

The object type, which is always “checkpoint.permission”.

#### Fine TuningAlpha

#### Fine TuningAlphaGraders

Manage fine-tuning jobs to tailor a model to your specific training data.

##### [Run grader](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/alpha/subresources/graders/methods/run)

POST/fine\_tuning/alpha/graders/run

##### [Validate grader](https://developers.openai.com/api/reference/resources/fine_tuning/subresources/alpha/subresources/graders/methods/validate)

POST/fine\_tuning/alpha/graders/validate

##### Models

GraderRunResponse object { metadata, model\_grader\_token\_usage\_per\_model, reward, sub\_rewards }

metadata: object { errors, execution\_time, name, 4 more }

errors: object { formula\_parse\_error, invalid\_variable\_error, model\_grader\_parse\_error, 11 more }

formula\_parse\_error: boolean

invalid\_variable\_error: boolean

model\_grader\_parse\_error: boolean

model\_grader\_refusal\_error: boolean

model\_grader\_server\_error: boolean

model\_grader\_server\_error\_details: string

other\_error: boolean

python\_grader\_runtime\_error: boolean

python\_grader\_runtime\_error\_details: string

python\_grader\_server\_error: boolean

python\_grader\_server\_error\_type: string

sample\_parse\_error: boolean

truncated\_observation\_error: boolean

unresponsive\_reward\_error: boolean

execution\_time: number

name: string

sampled\_model\_name: string

scores: map[unknown]

token\_usage: number

type: string

model\_grader\_token\_usage\_per\_model: map[unknown]

reward: number

sub\_rewards: map[unknown]

GraderValidateResponse object { grader }

grader: optional [StringCheckGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20string_check_grader%20%3E%20(schema)) { input, name, operation, 2 more }  or [TextSimilarityGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20text_similarity_grader%20%3E%20(schema)) { evaluation\_metric, input, name, 2 more }  or [PythonGrader](https://developers.openai.com/api/reference/resources/graders#(resource)%20graders.grader_models%20%3E%20(model)%20python_grader%20%3E%20(schema)) { name, source, type, image\_tag }  or 2 more

The grader used for the fine-tuning job.

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
