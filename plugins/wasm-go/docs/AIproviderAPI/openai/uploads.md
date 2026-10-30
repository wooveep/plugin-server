# Uploads

> Source: https://developers.openai.com/api/reference/resources/uploads
> Fetched: 2026-04-23

Use Uploads to upload large files in multiple parts.

##### [Create upload](https://developers.openai.com/api/reference/resources/uploads/methods/create)

POST/uploads

##### [Complete upload](https://developers.openai.com/api/reference/resources/uploads/methods/complete)

POST/uploads/{upload\_id}/complete

##### [Cancel upload](https://developers.openai.com/api/reference/resources/uploads/methods/cancel)

POST/uploads/{upload\_id}/cancel

##### Models

Upload object { id, bytes, created\_at, 6 more }

The Upload object can accept byte chunks in the form of Parts.

id: string

The Upload unique identifier, which can be referenced in API endpoints.

bytes: number

The intended number of bytes to be uploaded.

created\_at: number

The Unix timestamp (in seconds) for when the Upload was created.

expires\_at: number

The Unix timestamp (in seconds) for when the Upload will expire.

filename: string

The name of the file to be uploaded.

purpose: string

The intended purpose of the file. [Please refer here](https://developers.openai.com/docs/api-reference/files/object#files/object-purpose) for acceptable values.

status: "pending" or "completed" or "cancelled" or "expired"

The status of the Upload.

One of the following:

"pending"

"completed"

"cancelled"

"expired"

file: optional [FileObject](https://developers.openai.com/api/reference/resources/files#(resource)%20files%20%3E%20(model)%20file_object%20%3E%20(schema)) { id, bytes, created\_at, 6 more }

The `File` object represents a document that has been uploaded to OpenAI.

object: optional "upload"

The object type, which is always “upload”.

#### UploadsParts

Use Uploads to upload large files in multiple parts.

##### [Add upload part](https://developers.openai.com/api/reference/resources/uploads/subresources/parts/methods/create)

POST/uploads/{upload\_id}/parts

##### Models

UploadPart object { id, created\_at, object, upload\_id }

The upload Part represents a chunk of bytes we can add to an Upload object.

id: string

The upload Part unique identifier, which can be referenced in API endpoints.

created\_at: number

The Unix timestamp (in seconds) for when the Part was created.

object: "upload.part"

The object type, which is always `upload.part`.

upload\_id: string

The ID of the Upload object that this Part was added to.
