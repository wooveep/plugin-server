# Files

> Source: https://developers.openai.com/api/reference/resources/files
> Fetched: 2026-04-23

Files are used to upload documents that can be used with features like Assistants and Fine-tuning.

##### [List files](https://developers.openai.com/api/reference/resources/files/methods/list)

GET/files

##### [Upload file](https://developers.openai.com/api/reference/resources/files/methods/create)

POST/files

##### [Delete file](https://developers.openai.com/api/reference/resources/files/methods/delete)

DELETE/files/{file\_id}

##### [Retrieve file](https://developers.openai.com/api/reference/resources/files/methods/retrieve)

GET/files/{file\_id}

##### [Retrieve file content](https://developers.openai.com/api/reference/resources/files/methods/content)

GET/files/{file\_id}/content

##### Models

FileContent = string

FileDeleted object { id, deleted, object }

id: string

deleted: boolean

object: "file"

FileObject object { id, bytes, created\_at, 6 more }

The `File` object represents a document that has been uploaded to OpenAI.

id: string

The file identifier, which can be referenced in the API endpoints.

bytes: number

The size of the file, in bytes.

created\_at: number

The Unix timestamp (in seconds) for when the file was created.

filename: string

The name of the file.

object: "file"

The object type, which is always `file`.

purpose: "assistants" or "assistants\_output" or "batch" or 5 more

The intended purpose of the file. Supported values are `assistants`, `assistants_output`, `batch`, `batch_output`, `fine-tune`, `fine-tune-results`, `vision`, and `user_data`.

One of the following:

"assistants"

"assistants\_output"

"batch"

"batch\_output"

"fine-tune"

"fine-tune-results"

"vision"

"user\_data"

Deprecatedstatus: "uploaded" or "processed" or "error"

Deprecated. The current status of the file, which can be either `uploaded`, `processed`, or `error`.

One of the following:

"uploaded"

"processed"

"error"

expires\_at: optional number

The Unix timestamp (in seconds) for when the file will expire.

Deprecatedstatus\_details: optional string

Deprecated. For details on why a fine-tuning training file failed validation, see the `error` field on `fine_tuning.job`.
