# Models

> Source: https://developers.openai.com/api/reference/resources/models
> Fetched: 2026-04-23

List and describe the various models available in the API.

##### [List models](https://developers.openai.com/api/reference/resources/models/methods/list)

GET/models

##### [Retrieve model](https://developers.openai.com/api/reference/resources/models/methods/retrieve)

GET/models/{model}

##### [Delete a fine-tuned model](https://developers.openai.com/api/reference/resources/models/methods/delete)

DELETE/models/{model}

##### Models

Model object { id, created, object, owned\_by }

Describes an OpenAI model offering that can be used with the API.

id: string

The model identifier, which can be referenced in the API endpoints.

created: number

The Unix timestamp (in seconds) when the model was created.

object: "model"

The object type, which is always “model”.

owned\_by: string

The organization that owns the model.

ModelDeleted object { id, deleted, object }

id: string

deleted: boolean

object: string
