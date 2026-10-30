# Vector Stores

> Source: https://developers.openai.com/api/reference/resources/vector_stores
> Fetched: 2026-04-23

##### [List vector stores](https://developers.openai.com/api/reference/resources/vector_stores/methods/list)

GET/vector\_stores

##### [Create vector store](https://developers.openai.com/api/reference/resources/vector_stores/methods/create)

POST/vector\_stores

##### [Retrieve vector store](https://developers.openai.com/api/reference/resources/vector_stores/methods/retrieve)

GET/vector\_stores/{vector\_store\_id}

##### [Modify vector store](https://developers.openai.com/api/reference/resources/vector_stores/methods/update)

POST/vector\_stores/{vector\_store\_id}

##### [Delete vector store](https://developers.openai.com/api/reference/resources/vector_stores/methods/delete)

DELETE/vector\_stores/{vector\_store\_id}

##### [Search vector store](https://developers.openai.com/api/reference/resources/vector_stores/methods/search)

POST/vector\_stores/{vector\_store\_id}/search

##### Models

AutoFileChunkingStrategyParam object { type }

The default strategy. This strategy currently uses a `max_chunk_size_tokens` of `800` and `chunk_overlap_tokens` of `400`.

type: "auto"

Always `auto`.

FileChunkingStrategyParam = [AutoFileChunkingStrategyParam](https://developers.openai.com/api/reference/resources/vector_stores#(resource)%20vector_stores%20%3E%20(model)%20auto_file_chunking_strategy_param%20%3E%20(schema)) { type }  or [StaticFileChunkingStrategyObjectParam](https://developers.openai.com/api/reference/resources/vector_stores#(resource)%20vector_stores%20%3E%20(model)%20static_file_chunking_strategy_object_param%20%3E%20(schema)) { static, type }

The chunking strategy used to chunk the file(s). If not set, will use the `auto` strategy.

One of the following:

AutoFileChunkingStrategyParam object { type }

The default strategy. This strategy currently uses a `max_chunk_size_tokens` of `800` and `chunk_overlap_tokens` of `400`.

type: "auto"

Always `auto`.

StaticFileChunkingStrategyObjectParam object { static, type }

Customize your own chunking strategy by setting chunk size and chunk overlap.

static: [StaticFileChunkingStrategy](https://developers.openai.com/api/reference/resources/vector_stores#(resource)%20vector_stores%20%3E%20(model)%20static_file_chunking_strategy%20%3E%20(schema)) { chunk\_overlap\_tokens, max\_chunk\_size\_tokens }

type: "static"

Always `static`.

OtherFileChunkingStrategyObject object { type }

This is returned when the chunking strategy is unknown. Typically, this is because the file was indexed before the `chunking_strategy` concept was introduced in the API.

type: "other"

Always `other`.

StaticFileChunkingStrategy object { chunk\_overlap\_tokens, max\_chunk\_size\_tokens }

chunk\_overlap\_tokens: number

The number of tokens that overlap between chunks. The default value is `400`.

Note that the overlap must not exceed half of `max_chunk_size_tokens`.

max\_chunk\_size\_tokens: number

The maximum number of tokens in each chunk. The default value is `800`. The minimum value is `100` and the maximum value is `4096`.

minimum100

maximum4096

StaticFileChunkingStrategyObject object { static, type }

static: [StaticFileChunkingStrategy](https://developers.openai.com/api/reference/resources/vector_stores#(resource)%20vector_stores%20%3E%20(model)%20static_file_chunking_strategy%20%3E%20(schema)) { chunk\_overlap\_tokens, max\_chunk\_size\_tokens }

type: "static"

Always `static`.

StaticFileChunkingStrategyObjectParam object { static, type }

Customize your own chunking strategy by setting chunk size and chunk overlap.

static: [StaticFileChunkingStrategy](https://developers.openai.com/api/reference/resources/vector_stores#(resource)%20vector_stores%20%3E%20(model)%20static_file_chunking_strategy%20%3E%20(schema)) { chunk\_overlap\_tokens, max\_chunk\_size\_tokens }

type: "static"

Always `static`.

VectorStore object { id, created\_at, file\_counts, 8 more }

A vector store is a collection of processed files can be used by the `file_search` tool.

id: string

The identifier, which can be referenced in API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the vector store was created.

file\_counts: object { cancelled, completed, failed, 2 more }

cancelled: number

The number of files that were cancelled.

completed: number

The number of files that have been successfully processed.

failed: number

The number of files that have failed to process.

in\_progress: number

The number of files that are currently being processed.

total: number

The total number of files.

last\_active\_at: number

The Unix timestamp (in seconds) for when the vector store was last active.

metadata: [Metadata](https://developers.openai.com/api/reference/resources/$shared#(resource)%20%24shared%20%3E%20(model)%20metadata%20%3E%20(schema))

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard.

Keys are strings with a maximum length of 64 characters. Values are strings
with a maximum length of 512 characters.

name: string

The name of the vector store.

object: "vector\_store"

The object type, which is always `vector_store`.

status: "expired" or "in\_progress" or "completed"

The status of the vector store, which can be either `expired`, `in_progress`, or `completed`. A status of `completed` indicates that the vector store is ready for use.

One of the following:

"expired"

"in\_progress"

"completed"

usage\_bytes: number

The total number of bytes used by the files in the vector store.

expires\_after: optional object { anchor, days }

The expiration policy for a vector store.

anchor: "last\_active\_at"

Anchor timestamp after which the expiration policy applies. Supported anchors: `last_active_at`.

days: number

The number of days after the anchor time that the vector store will expire.

minimum1

maximum365

expires\_at: optional number

The Unix timestamp (in seconds) for when the vector store will expire.

VectorStoreDeleted object { id, deleted, object }

id: string

deleted: boolean

object: "vector\_store.deleted"

VectorStoreSearchResponse object { attributes, content, file\_id, 2 more }

attributes: map[string or number or boolean]

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard. Keys are strings
with a maximum length of 64 characters. Values are strings with a maximum
length of 512 characters, booleans, or numbers.

One of the following:

string

number

boolean

content: array of object { text, type }

Content chunks from the file.

text: string

The text content returned from search.

type: "text"

The type of content.

file\_id: string

The ID of the vector store file.

filename: string

The name of the vector store file.

score: number

The similarity score for the result.

minimum0

maximum1

#### Vector StoresFiles

##### [List vector store files](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/list)

GET/vector\_stores/{vector\_store\_id}/files

##### [Create vector store file](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/create)

POST/vector\_stores/{vector\_store\_id}/files

##### [Update vector store file attributes](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/update)

POST/vector\_stores/{vector\_store\_id}/files/{file\_id}

##### [Retrieve vector store file](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/retrieve)

GET/vector\_stores/{vector\_store\_id}/files/{file\_id}

##### [Delete vector store file](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/delete)

DELETE/vector\_stores/{vector\_store\_id}/files/{file\_id}

##### [Retrieve vector store file content](https://developers.openai.com/api/reference/resources/vector_stores/subresources/files/methods/content)

GET/vector\_stores/{vector\_store\_id}/files/{file\_id}/content

##### Models

VectorStoreFile object { id, created\_at, last\_error, 6 more }

A list of files attached to a vector store.

id: string

The identifier, which can be referenced in API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the vector store file was created.

last\_error: object { code, message }

The last error associated with this vector store file. Will be `null` if there are no errors.

code: "server\_error" or "unsupported\_file" or "invalid\_file"

One of `server_error`, `unsupported_file`, or `invalid_file`.

One of the following:

"server\_error"

"unsupported\_file"

"invalid\_file"

message: string

A human-readable description of the error.

object: "vector\_store.file"

The object type, which is always `vector_store.file`.

status: "in\_progress" or "completed" or "cancelled" or "failed"

The status of the vector store file, which can be either `in_progress`, `completed`, `cancelled`, or `failed`. The status `completed` indicates that the vector store file is ready for use.

One of the following:

"in\_progress"

"completed"

"cancelled"

"failed"

usage\_bytes: number

The total vector store usage in bytes. Note that this may be different from the original file size.

vector\_store\_id: string

The ID of the [vector store](https://developers.openai.com/docs/api-reference/vector-stores/object) that the [File](https://developers.openai.com/docs/api-reference/files) is attached to.

attributes: optional map[string or number or boolean]

Set of 16 key-value pairs that can be attached to an object. This can be
useful for storing additional information about the object in a structured
format, and querying for objects via API or the dashboard. Keys are strings
with a maximum length of 64 characters. Values are strings with a maximum
length of 512 characters, booleans, or numbers.

One of the following:

string

number

boolean

chunking\_strategy: optional [StaticFileChunkingStrategyObject](https://developers.openai.com/api/reference/resources/vector_stores#(resource)%20vector_stores%20%3E%20(model)%20static_file_chunking_strategy_object%20%3E%20(schema)) { static, type }  or [OtherFileChunkingStrategyObject](https://developers.openai.com/api/reference/resources/vector_stores#(resource)%20vector_stores%20%3E%20(model)%20other_file_chunking_strategy_object%20%3E%20(schema)) { type }

The strategy used to chunk the file.

One of the following:

StaticFileChunkingStrategyObject object { static, type }

static: [StaticFileChunkingStrategy](https://developers.openai.com/api/reference/resources/vector_stores#(resource)%20vector_stores%20%3E%20(model)%20static_file_chunking_strategy%20%3E%20(schema)) { chunk\_overlap\_tokens, max\_chunk\_size\_tokens }

type: "static"

Always `static`.

OtherFileChunkingStrategyObject object { type }

This is returned when the chunking strategy is unknown. Typically, this is because the file was indexed before the `chunking_strategy` concept was introduced in the API.

type: "other"

Always `other`.

VectorStoreFileDeleted object { id, deleted, object }

id: string

deleted: boolean

object: "vector\_store.file.deleted"

FileContentResponse object { text, type }

text: optional string

The text content

type: optional string

The content type (currently only `"text"`)

#### Vector StoresFile Batches

##### [Create vector store file batch](https://developers.openai.com/api/reference/resources/vector_stores/subresources/file_batches/methods/create)

POST/vector\_stores/{vector\_store\_id}/file\_batches

##### [Retrieve vector store file batch](https://developers.openai.com/api/reference/resources/vector_stores/subresources/file_batches/methods/retrieve)

GET/vector\_stores/{vector\_store\_id}/file\_batches/{batch\_id}

##### [Cancel vector store file batch](https://developers.openai.com/api/reference/resources/vector_stores/subresources/file_batches/methods/cancel)

POST/vector\_stores/{vector\_store\_id}/file\_batches/{batch\_id}/cancel

##### [List vector store files in a batch](https://developers.openai.com/api/reference/resources/vector_stores/subresources/file_batches/methods/list_files)

GET/vector\_stores/{vector\_store\_id}/file\_batches/{batch\_id}/files

##### Models

VectorStoreFileBatch object { id, created\_at, file\_counts, 3 more }

A batch of files attached to a vector store.

id: string

The identifier, which can be referenced in API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the vector store files batch was created.

file\_counts: object { cancelled, completed, failed, 2 more }

cancelled: number

The number of files that where cancelled.

completed: number

The number of files that have been processed.

failed: number

The number of files that have failed to process.

in\_progress: number

The number of files that are currently being processed.

total: number

The total number of files.

object: "vector\_store.files\_batch"

The object type, which is always `vector_store.file_batch`.

status: "in\_progress" or "completed" or "cancelled" or "failed"

The status of the vector store files batch, which can be either `in_progress`, `completed`, `cancelled` or `failed`.

One of the following:

"in\_progress"

"completed"

"cancelled"

"failed"

vector\_store\_id: string

The ID of the [vector store](https://developers.openai.com/docs/api-reference/vector-stores/object) that the [File](https://developers.openai.com/docs/api-reference/files) is attached to.
