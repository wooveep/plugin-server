# Containers

> Source: https://developers.openai.com/api/reference/resources/containers
> Fetched: 2026-04-23

##### [List containers](https://developers.openai.com/api/reference/resources/containers/methods/list)

GET/containers

##### [Create container](https://developers.openai.com/api/reference/resources/containers/methods/create)

POST/containers

##### [Retrieve container](https://developers.openai.com/api/reference/resources/containers/methods/retrieve)

GET/containers/{container\_id}

##### [Delete a container](https://developers.openai.com/api/reference/resources/containers/methods/delete)

DELETE/containers/{container\_id}

##### Models

ContainerListResponse object { id, created\_at, name, 6 more }

id: string

Unique identifier for the container.

created\_at: number

Unix timestamp (in seconds) when the container was created.

name: string

Name of the container.

object: string

The type of this object.

status: string

Status of the container (e.g., active, deleted).

expires\_after: optional object { anchor, minutes }

The container will expire after this time period.
The anchor is the reference point for the expiration.
The minutes is the number of minutes after the anchor before the container expires.

anchor: optional "last\_active\_at"

The reference point for the expiration.

minutes: optional number

The number of minutes after the anchor before the container expires.

last\_active\_at: optional number

Unix timestamp (in seconds) when the container was last active.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit configured for the container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional object { type, allowed\_domains }

Network access policy for the container.

type: "allowlist" or "disabled"

The network policy mode.

One of the following:

"allowlist"

"disabled"

allowed\_domains: optional array of string

Allowed outbound domains when `type` is `allowlist`.

ContainerCreateResponse object { id, created\_at, name, 6 more }

id: string

Unique identifier for the container.

created\_at: number

Unix timestamp (in seconds) when the container was created.

name: string

Name of the container.

object: string

The type of this object.

status: string

Status of the container (e.g., active, deleted).

expires\_after: optional object { anchor, minutes }

The container will expire after this time period.
The anchor is the reference point for the expiration.
The minutes is the number of minutes after the anchor before the container expires.

anchor: optional "last\_active\_at"

The reference point for the expiration.

minutes: optional number

The number of minutes after the anchor before the container expires.

last\_active\_at: optional number

Unix timestamp (in seconds) when the container was last active.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit configured for the container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional object { type, allowed\_domains }

Network access policy for the container.

type: "allowlist" or "disabled"

The network policy mode.

One of the following:

"allowlist"

"disabled"

allowed\_domains: optional array of string

Allowed outbound domains when `type` is `allowlist`.

ContainerRetrieveResponse object { id, created\_at, name, 6 more }

id: string

Unique identifier for the container.

created\_at: number

Unix timestamp (in seconds) when the container was created.

name: string

Name of the container.

object: string

The type of this object.

status: string

Status of the container (e.g., active, deleted).

expires\_after: optional object { anchor, minutes }

The container will expire after this time period.
The anchor is the reference point for the expiration.
The minutes is the number of minutes after the anchor before the container expires.

anchor: optional "last\_active\_at"

The reference point for the expiration.

minutes: optional number

The number of minutes after the anchor before the container expires.

last\_active\_at: optional number

Unix timestamp (in seconds) when the container was last active.

memory\_limit: optional "1g" or "4g" or "16g" or "64g"

The memory limit configured for the container.

One of the following:

"1g"

"4g"

"16g"

"64g"

network\_policy: optional object { type, allowed\_domains }

Network access policy for the container.

type: "allowlist" or "disabled"

The network policy mode.

One of the following:

"allowlist"

"disabled"

allowed\_domains: optional array of string

Allowed outbound domains when `type` is `allowlist`.

#### ContainersFiles

##### [List container files](https://developers.openai.com/api/reference/resources/containers/subresources/files/methods/list)

GET/containers/{container\_id}/files

##### [Create container file](https://developers.openai.com/api/reference/resources/containers/subresources/files/methods/create)

POST/containers/{container\_id}/files

##### [Retrieve container file](https://developers.openai.com/api/reference/resources/containers/subresources/files/methods/retrieve)

GET/containers/{container\_id}/files/{file\_id}

##### [Delete a container file](https://developers.openai.com/api/reference/resources/containers/subresources/files/methods/delete)

DELETE/containers/{container\_id}/files/{file\_id}

##### Models

FileListResponse object { id, bytes, container\_id, 4 more }

id: string

Unique identifier for the file.

bytes: number

Size of the file in bytes.

container\_id: string

The container this file belongs to.

created\_at: number

Unix timestamp (in seconds) when the file was created.

object: string

The type of this object (`container.file`).

path: string

Path of the file in the container.

source: string

Source of the file (e.g., `user`, `assistant`).

FileCreateResponse object { id, bytes, container\_id, 4 more }

id: string

Unique identifier for the file.

bytes: number

Size of the file in bytes.

container\_id: string

The container this file belongs to.

created\_at: number

Unix timestamp (in seconds) when the file was created.

object: string

The type of this object (`container.file`).

path: string

Path of the file in the container.

source: string

Source of the file (e.g., `user`, `assistant`).

FileRetrieveResponse object { id, bytes, container\_id, 4 more }

id: string

Unique identifier for the file.

bytes: number

Size of the file in bytes.

container\_id: string

The container this file belongs to.

created\_at: number

Unix timestamp (in seconds) when the file was created.

object: string

The type of this object (`container.file`).

path: string

Path of the file in the container.

source: string

Source of the file (e.g., `user`, `assistant`).

#### ContainersFilesContent

##### [Retrieve container file content](https://developers.openai.com/api/reference/resources/containers/subresources/files/subresources/content/methods/retrieve)

GET/containers/{container\_id}/files/{file\_id}/content
