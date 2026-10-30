# Organization

> Source: https://developers.openai.com/api/reference/resources/organization
> Fetched: 2026-04-23

#### OrganizationAudit Logs

##### [List audit logs](https://developers.openai.com/api/reference/resources/organization/subresources/audit_logs/methods/list)

GET/organization/audit\_logs

##### [Costs](https://developers.openai.com/api/reference/resources/organization/subresources/audit_logs/methods/get_costs)

GET/organization/costs

##### Models

AuditLogListResponse object { id, actor, effective\_at, 49 more }

A log of a user action or configuration change within this organization.

id: string

The ID of this log.

actor: object { api\_key, session, type }

The actor who performed the audit logged action.

api\_key: optional object { id, service\_account, type, user }

The API Key used to perform the audit logged action.

id: optional string

The tracking id of the API key.

service\_account: optional object { id }

The service account that performed the audit logged action.

id: optional string

The service account id.

type: optional "user" or "service\_account"

The type of API key. Can be either `user` or `service_account`.

One of the following:

"user"

"service\_account"

user: optional object { id, email }

The user who performed the audit logged action.

id: optional string

The user id.

email: optional string

The user email.

session: optional object { ip\_address, user }

The session in which the audit logged action was performed.

ip\_address: optional string

The IP address from which the action was performed.

user: optional object { id, email }

The user who performed the audit logged action.

id: optional string

The user id.

email: optional string

The user email.

type: optional "session" or "api\_key"

The type of actor. Is either `session` or `api_key`.

One of the following:

"session"

"api\_key"

effective\_at: number

The Unix timestamp (in seconds) of the event.

type: "api\_key.created" or "api\_key.updated" or "api\_key.deleted" or 48 more

The event type.

One of the following:

"api\_key.created"

"api\_key.updated"

"api\_key.deleted"

"certificate.created"

"certificate.updated"

"certificate.deleted"

"certificates.activated"

"certificates.deactivated"

"checkpoint.permission.created"

"checkpoint.permission.deleted"

"external\_key.registered"

"external\_key.removed"

"group.created"

"group.updated"

"group.deleted"

"invite.sent"

"invite.accepted"

"invite.deleted"

"ip\_allowlist.created"

"ip\_allowlist.updated"

"ip\_allowlist.deleted"

"ip\_allowlist.config.activated"

"ip\_allowlist.config.deactivated"

"login.succeeded"

"login.failed"

"logout.succeeded"

"logout.failed"

"organization.updated"

"project.created"

"project.updated"

"project.archived"

"project.deleted"

"rate\_limit.updated"

"rate\_limit.deleted"

"resource.deleted"

"tunnel.created"

"tunnel.updated"

"tunnel.deleted"

"role.created"

"role.updated"

"role.deleted"

"role.assignment.created"

"role.assignment.deleted"

"scim.enabled"

"scim.disabled"

"service\_account.created"

"service\_account.updated"

"service\_account.deleted"

"user.added"

"user.updated"

"user.deleted"

"api\_key.created": optional object { id, data }

The details for events with this `type`.

id: optional string

The tracking ID of the API key.

data: optional object { scopes }

The payload used to create the API key.

scopes: optional array of string

A list of scopes allowed for the API key, e.g. `["api.model.request"]`

"api\_key.deleted": optional object { id }

The details for events with this `type`.

id: optional string

The tracking ID of the API key.

"api\_key.updated": optional object { id, changes\_requested }

The details for events with this `type`.

id: optional string

The tracking ID of the API key.

changes\_requested: optional object { scopes }

The payload used to update the API key.

scopes: optional array of string

A list of scopes allowed for the API key, e.g. `["api.model.request"]`

"certificate.created": optional object { id, name }

The details for events with this `type`.

id: optional string

The certificate ID.

name: optional string

The name of the certificate.

"certificate.deleted": optional object { id, certificate, name }

The details for events with this `type`.

id: optional string

The certificate ID.

certificate: optional string

The certificate content in PEM format.

name: optional string

The name of the certificate.

"certificate.updated": optional object { id, name }

The details for events with this `type`.

id: optional string

The certificate ID.

name: optional string

The name of the certificate.

"certificates.activated": optional object { certificates }

The details for events with this `type`.

certificates: optional array of object { id, name }

id: optional string

The certificate ID.

name: optional string

The name of the certificate.

"certificates.deactivated": optional object { certificates }

The details for events with this `type`.

certificates: optional array of object { id, name }

id: optional string

The certificate ID.

name: optional string

The name of the certificate.

"checkpoint.permission.created": optional object { id, data }

The project and fine-tuned model checkpoint that the checkpoint permission was created for.

id: optional string

The ID of the checkpoint permission.

data: optional object { fine\_tuned\_model\_checkpoint, project\_id }

The payload used to create the checkpoint permission.

fine\_tuned\_model\_checkpoint: optional string

The ID of the fine-tuned model checkpoint.

project\_id: optional string

The ID of the project that the checkpoint permission was created for.

"checkpoint.permission.deleted": optional object { id }

The details for events with this `type`.

id: optional string

The ID of the checkpoint permission.

"external\_key.registered": optional object { id, data }

The details for events with this `type`.

id: optional string

The ID of the external key configuration.

data: optional unknown

The configuration for the external key.

"external\_key.removed": optional object { id }

The details for events with this `type`.

id: optional string

The ID of the external key configuration.

"group.created": optional object { id, data }

The details for events with this `type`.

id: optional string

The ID of the group.

data: optional object { group\_name }

Information about the created group.

group\_name: optional string

The group name.

"group.deleted": optional object { id }

The details for events with this `type`.

id: optional string

The ID of the group.

"group.updated": optional object { id, changes\_requested }

The details for events with this `type`.

id: optional string

The ID of the group.

changes\_requested: optional object { group\_name }

The payload used to update the group.

group\_name: optional string

The updated group name.

"invite.accepted": optional object { id }

The details for events with this `type`.

id: optional string

The ID of the invite.

"invite.deleted": optional object { id }

The details for events with this `type`.

id: optional string

The ID of the invite.

"invite.sent": optional object { id, data }

The details for events with this `type`.

id: optional string

The ID of the invite.

data: optional object { email, role }

The payload used to create the invite.

email: optional string

The email invited to the organization.

role: optional string

The role the email was invited to be. Is either `owner` or `member`.

"ip\_allowlist.config.activated": optional object { configs }

The details for events with this `type`.

configs: optional array of object { id, name }

The configurations that were activated.

id: optional string

The ID of the IP allowlist configuration.

name: optional string

The name of the IP allowlist configuration.

"ip\_allowlist.config.deactivated": optional object { configs }

The details for events with this `type`.

configs: optional array of object { id, name }

The configurations that were deactivated.

id: optional string

The ID of the IP allowlist configuration.

name: optional string

The name of the IP allowlist configuration.

"ip\_allowlist.created": optional object { id, allowed\_ips, name }

The details for events with this `type`.

id: optional string

The ID of the IP allowlist configuration.

allowed\_ips: optional array of string

The IP addresses or CIDR ranges included in the configuration.

name: optional string

The name of the IP allowlist configuration.

"ip\_allowlist.deleted": optional object { id, allowed\_ips, name }

The details for events with this `type`.

id: optional string

The ID of the IP allowlist configuration.

allowed\_ips: optional array of string

The IP addresses or CIDR ranges that were in the configuration.

name: optional string

The name of the IP allowlist configuration.

"ip\_allowlist.updated": optional object { id, allowed\_ips }

The details for events with this `type`.

id: optional string

The ID of the IP allowlist configuration.

allowed\_ips: optional array of string

The updated set of IP addresses or CIDR ranges in the configuration.

"login.failed": optional object { error\_code, error\_message }

The details for events with this `type`.

error\_code: optional string

The error code of the failure.

error\_message: optional string

The error message of the failure.

"login.succeeded": optional unknown

This event has no additional fields beyond the standard audit log attributes.

"logout.failed": optional object { error\_code, error\_message }

The details for events with this `type`.

error\_code: optional string

The error code of the failure.

error\_message: optional string

The error message of the failure.

"logout.succeeded": optional unknown

This event has no additional fields beyond the standard audit log attributes.

"organization.updated": optional object { id, changes\_requested }

The details for events with this `type`.

id: optional string

The organization ID.

changes\_requested: optional object { api\_call\_logging, api\_call\_logging\_project\_ids, description, 4 more }

The payload used to update the organization settings.

api\_call\_logging: optional string

How your organization logs data from supported API calls. One of `disabled`, `enabled_per_call`, `enabled_for_all_projects`, or `enabled_for_selected_projects`

api\_call\_logging\_project\_ids: optional string

The list of project ids if api\_call\_logging is set to `enabled_for_selected_projects`

description: optional string

The organization description.

name: optional string

The organization name.

threads\_ui\_visibility: optional string

Visibility of the threads page which shows messages created with the Assistants API and Playground. One of `ANY_ROLE`, `OWNERS`, or `NONE`.

title: optional string

The organization title.

usage\_dashboard\_visibility: optional string

Visibility of the usage dashboard which shows activity and costs for your organization. One of `ANY_ROLE` or `OWNERS`.

project: optional object { id, name }

The project that the action was scoped to. Absent for actions not scoped to projects. Note that any admin actions taken via Admin API keys are associated with the default project.

id: optional string

The project ID.

name: optional string

The project title.

"project.archived": optional object { id }

The details for events with this `type`.

id: optional string

The project ID.

"project.created": optional object { id, data }

The details for events with this `type`.

id: optional string

The project ID.

data: optional object { name, title }

The payload used to create the project.

name: optional string

The project name.

title: optional string

The title of the project as seen on the dashboard.

"project.deleted": optional object { id }

The details for events with this `type`.

id: optional string

The project ID.

"project.updated": optional object { id, changes\_requested }

The details for events with this `type`.

id: optional string

The project ID.

changes\_requested: optional object { title }

The payload used to update the project.

title: optional string

The title of the project as seen on the dashboard.

"rate\_limit.deleted": optional object { id }

The details for events with this `type`.

id: optional string

The rate limit ID

"rate\_limit.updated": optional object { id, changes\_requested }

The details for events with this `type`.

id: optional string

The rate limit ID

changes\_requested: optional object { batch\_1\_day\_max\_input\_tokens, max\_audio\_megabytes\_per\_1\_minute, max\_images\_per\_1\_minute, 3 more }

The payload used to update the rate limits.

batch\_1\_day\_max\_input\_tokens: optional number

The maximum batch input tokens per day. Only relevant for certain models.

max\_audio\_megabytes\_per\_1\_minute: optional number

The maximum audio megabytes per minute. Only relevant for certain models.

max\_images\_per\_1\_minute: optional number

The maximum images per minute. Only relevant for certain models.

max\_requests\_per\_1\_day: optional number

The maximum requests per day. Only relevant for certain models.

max\_requests\_per\_1\_minute: optional number

The maximum requests per minute.

max\_tokens\_per\_1\_minute: optional number

The maximum tokens per minute.

"role.assignment.created": optional object { id, principal\_id, principal\_type, 2 more }

The details for events with this `type`.

id: optional string

The identifier of the role assignment.

principal\_id: optional string

The principal (user or group) that received the role.

principal\_type: optional string

The type of principal (user or group) that received the role.

resource\_id: optional string

The resource the role assignment is scoped to.

resource\_type: optional string

The type of resource the role assignment is scoped to.

"role.assignment.deleted": optional object { id, principal\_id, principal\_type, 2 more }

The details for events with this `type`.

id: optional string

The identifier of the role assignment.

principal\_id: optional string

The principal (user or group) that had the role removed.

principal\_type: optional string

The type of principal (user or group) that had the role removed.

resource\_id: optional string

The resource the role assignment was scoped to.

resource\_type: optional string

The type of resource the role assignment was scoped to.

"role.created": optional object { id, permissions, resource\_id, 2 more }

The details for events with this `type`.

id: optional string

The role ID.

permissions: optional array of string

The permissions granted by the role.

resource\_id: optional string

The resource the role is scoped to.

resource\_type: optional string

The type of resource the role belongs to.

role\_name: optional string

The name of the role.

"role.deleted": optional object { id }

The details for events with this `type`.

id: optional string

The role ID.

"role.updated": optional object { id, changes\_requested }

The details for events with this `type`.

id: optional string

The role ID.

changes\_requested: optional object { description, metadata, permissions\_added, 4 more }

The payload used to update the role.

description: optional string

The updated role description, when provided.

metadata: optional unknown

Additional metadata stored on the role.

permissions\_added: optional array of string

The permissions added to the role.

permissions\_removed: optional array of string

The permissions removed from the role.

resource\_id: optional string

The resource the role is scoped to.

resource\_type: optional string

The type of resource the role belongs to.

role\_name: optional string

The updated role name, when provided.

"scim.disabled": optional object { id }

The details for events with this `type`.

id: optional string

The ID of the SCIM was disabled for.

"scim.enabled": optional object { id }

The details for events with this `type`.

id: optional string

The ID of the SCIM was enabled for.

"service\_account.created": optional object { id, data }

The details for events with this `type`.

id: optional string

The service account ID.

data: optional object { role }

The payload used to create the service account.

role: optional string

The role of the service account. Is either `owner` or `member`.

"service\_account.deleted": optional object { id }

The details for events with this `type`.

id: optional string

The service account ID.

"service\_account.updated": optional object { id, changes\_requested }

The details for events with this `type`.

id: optional string

The service account ID.

changes\_requested: optional object { role }

The payload used to updated the service account.

role: optional string

The role of the service account. Is either `owner` or `member`.

"user.added": optional object { id, data }

The details for events with this `type`.

id: optional string

The user ID.

data: optional object { role }

The payload used to add the user to the project.

role: optional string

The role of the user. Is either `owner` or `member`.

"user.deleted": optional object { id }

The details for events with this `type`.

id: optional string

The user ID.

"user.updated": optional object { id, changes\_requested }

The details for events with this `type`.

id: optional string

The project ID.

changes\_requested: optional object { role }

The payload used to update the user.

role: optional string

The role of the user. Is either `owner` or `member`.

AuditLogGetCostsResponse object { data, has\_more, next\_page, object }

data: array of object { end\_time, object, result, start\_time }

end\_time: number

object: "bucket"

result: array of object { input\_tokens, num\_model\_requests, object, 10 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or 6 more

One of the following:

UsageCompletionsResult object { input\_tokens, num\_model\_requests, object, 10 more }

The aggregated completions usage details of the specific time bucket.

input\_tokens: number

The aggregated number of text input tokens used, including cached tokens. For customers subscribe to scale tier, this includes scale tier tokens.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.completions.result"

output\_tokens: number

The aggregated number of text output tokens used. For customers subscribe to scale tier, this includes scale tier tokens.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

batch: optional boolean

When `group_by=batch`, this field tells whether the grouped usage result is batch or not.

input\_audio\_tokens: optional number

The aggregated number of audio input tokens used, including cached tokens.

input\_cached\_tokens: optional number

The aggregated number of text input tokens that has been cached from previous requests. For customers subscribe to scale tier, this includes scale tier tokens.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

output\_audio\_tokens: optional number

The aggregated number of audio output tokens used.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

service\_tier: optional string

When `group_by=service_tier`, this field provides the service tier of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageEmbeddingsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated embeddings usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.embeddings.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageModerationsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated moderations usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.moderations.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageImagesResult object { images, num\_model\_requests, object, 6 more }

The aggregated images usage details of the specific time bucket.

images: number

The number of images processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.images.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

size: optional string

When `group_by=size`, this field provides the image size of the grouped usage result.

source: optional string

When `group_by=source`, this field provides the source of the grouped usage result, possible values are `image.generation`, `image.edit`, `image.variation`.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioSpeechesResult object { characters, num\_model\_requests, object, 4 more }

The aggregated audio speeches usage details of the specific time bucket.

characters: number

The number of characters processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_speeches.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioTranscriptionsResult object { num\_model\_requests, object, seconds, 4 more }

The aggregated audio transcriptions usage details of the specific time bucket.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_transcriptions.result"

seconds: number

The number of seconds processed.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageVectorStoresResult object { object, usage\_bytes, project\_id }

The aggregated vector stores usage details of the specific time bucket.

object: "organization.usage.vector\_stores.result"

usage\_bytes: number

The vector stores usage in bytes.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

UsageCodeInterpreterSessionsResult object { object, num\_sessions, project\_id }

The aggregated code interpreter sessions usage details of the specific time bucket.

object: "organization.usage.code\_interpreter\_sessions.result"

num\_sessions: optional number

The number of code interpreter sessions.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

CostsResult object { object, amount, line\_item, project\_id }

The aggregated costs details of the specific time bucket.

object: "organization.costs.result"

amount: optional object { currency, value }

The monetary value in its associated currency.

currency: optional string

Lowercase ISO-4217 currency e.g. “usd”

value: optional number

The numeric value of the cost.

line\_item: optional string

When `group_by=line_item`, this field provides the line item of the grouped costs result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped costs result.

start\_time: number

has\_more: boolean

next\_page: string

object: "page"

#### OrganizationAdmin API Keys

##### [List all organization and project API keys.](https://developers.openai.com/api/reference/resources/organization/subresources/admin_api_keys/methods/list)

GET/organization/admin\_api\_keys

##### [Create admin API key](https://developers.openai.com/api/reference/resources/organization/subresources/admin_api_keys/methods/create)

POST/organization/admin\_api\_keys

##### [Retrieve admin API key](https://developers.openai.com/api/reference/resources/organization/subresources/admin_api_keys/methods/retrieve)

GET/organization/admin\_api\_keys/{key\_id}

##### [Delete admin API key](https://developers.openai.com/api/reference/resources/organization/subresources/admin_api_keys/methods/delete)

DELETE/organization/admin\_api\_keys/{key\_id}

##### Models

AdminAPIKeyListResponse object { id, created\_at, last\_used\_at, 5 more }

Represents an individual Admin API key in an org.

id: string

The identifier, which can be referenced in API endpoints

created\_at: number

The Unix timestamp (in seconds) of when the API key was created

formatint64

last\_used\_at: number

The Unix timestamp (in seconds) of when the API key was last used

formatint64

name: string

The name of the API key

object: string

The object type, which is always `organization.admin_api_key`

owner: object { id, created\_at, name, 3 more }

id: optional string

The identifier, which can be referenced in API endpoints

created\_at: optional number

The Unix timestamp (in seconds) of when the user was created

formatint64

name: optional string

The name of the user

object: optional string

The object type, which is always organization.user

role: optional string

Always `owner`

type: optional string

Always `user`

redacted\_value: string

The redacted value of the API key

value: optional string

The value of the API key. Only shown on create.

AdminAPIKeyCreateResponse object { id, created\_at, last\_used\_at, 5 more }

Represents an individual Admin API key in an org.

id: string

The identifier, which can be referenced in API endpoints

created\_at: number

The Unix timestamp (in seconds) of when the API key was created

formatint64

last\_used\_at: number

The Unix timestamp (in seconds) of when the API key was last used

formatint64

name: string

The name of the API key

object: string

The object type, which is always `organization.admin_api_key`

owner: object { id, created\_at, name, 3 more }

id: optional string

The identifier, which can be referenced in API endpoints

created\_at: optional number

The Unix timestamp (in seconds) of when the user was created

formatint64

name: optional string

The name of the user

object: optional string

The object type, which is always organization.user

role: optional string

Always `owner`

type: optional string

Always `user`

redacted\_value: string

The redacted value of the API key

value: optional string

The value of the API key. Only shown on create.

AdminAPIKeyRetrieveResponse object { id, created\_at, last\_used\_at, 5 more }

Represents an individual Admin API key in an org.

id: string

The identifier, which can be referenced in API endpoints

created\_at: number

The Unix timestamp (in seconds) of when the API key was created

formatint64

last\_used\_at: number

The Unix timestamp (in seconds) of when the API key was last used

formatint64

name: string

The name of the API key

object: string

The object type, which is always `organization.admin_api_key`

owner: object { id, created\_at, name, 3 more }

id: optional string

The identifier, which can be referenced in API endpoints

created\_at: optional number

The Unix timestamp (in seconds) of when the user was created

formatint64

name: optional string

The name of the user

object: optional string

The object type, which is always organization.user

role: optional string

Always `owner`

type: optional string

Always `user`

redacted\_value: string

The redacted value of the API key

value: optional string

The value of the API key. Only shown on create.

AdminAPIKeyDeleteResponse object { id, deleted, object }

id: optional string

deleted: optional boolean

object: optional string

#### OrganizationUsage

##### [Audio speeches](https://developers.openai.com/api/reference/resources/organization/subresources/usage/methods/get_audio_speeches)

GET/organization/usage/audio\_speeches

##### [Audio transcriptions](https://developers.openai.com/api/reference/resources/organization/subresources/usage/methods/get_audio_transcriptions)

GET/organization/usage/audio\_transcriptions

##### [Code interpreter sessions](https://developers.openai.com/api/reference/resources/organization/subresources/usage/methods/get_code_interpreter_sessions)

GET/organization/usage/code\_interpreter\_sessions

##### [Completions](https://developers.openai.com/api/reference/resources/organization/subresources/usage/methods/get_completions)

GET/organization/usage/completions

##### [Embeddings](https://developers.openai.com/api/reference/resources/organization/subresources/usage/methods/get_embeddings)

GET/organization/usage/embeddings

##### [Images](https://developers.openai.com/api/reference/resources/organization/subresources/usage/methods/get_images)

GET/organization/usage/images

##### [Moderations](https://developers.openai.com/api/reference/resources/organization/subresources/usage/methods/get_moderations)

GET/organization/usage/moderations

##### [Vector stores](https://developers.openai.com/api/reference/resources/organization/subresources/usage/methods/get_vector_stores)

GET/organization/usage/vector\_stores

##### Models

UsageGetAudioSpeechesResponse object { data, has\_more, next\_page, object }

data: array of object { end\_time, object, result, start\_time }

end\_time: number

object: "bucket"

result: array of object { input\_tokens, num\_model\_requests, object, 10 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or 6 more

One of the following:

UsageCompletionsResult object { input\_tokens, num\_model\_requests, object, 10 more }

The aggregated completions usage details of the specific time bucket.

input\_tokens: number

The aggregated number of text input tokens used, including cached tokens. For customers subscribe to scale tier, this includes scale tier tokens.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.completions.result"

output\_tokens: number

The aggregated number of text output tokens used. For customers subscribe to scale tier, this includes scale tier tokens.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

batch: optional boolean

When `group_by=batch`, this field tells whether the grouped usage result is batch or not.

input\_audio\_tokens: optional number

The aggregated number of audio input tokens used, including cached tokens.

input\_cached\_tokens: optional number

The aggregated number of text input tokens that has been cached from previous requests. For customers subscribe to scale tier, this includes scale tier tokens.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

output\_audio\_tokens: optional number

The aggregated number of audio output tokens used.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

service\_tier: optional string

When `group_by=service_tier`, this field provides the service tier of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageEmbeddingsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated embeddings usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.embeddings.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageModerationsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated moderations usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.moderations.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageImagesResult object { images, num\_model\_requests, object, 6 more }

The aggregated images usage details of the specific time bucket.

images: number

The number of images processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.images.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

size: optional string

When `group_by=size`, this field provides the image size of the grouped usage result.

source: optional string

When `group_by=source`, this field provides the source of the grouped usage result, possible values are `image.generation`, `image.edit`, `image.variation`.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioSpeechesResult object { characters, num\_model\_requests, object, 4 more }

The aggregated audio speeches usage details of the specific time bucket.

characters: number

The number of characters processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_speeches.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioTranscriptionsResult object { num\_model\_requests, object, seconds, 4 more }

The aggregated audio transcriptions usage details of the specific time bucket.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_transcriptions.result"

seconds: number

The number of seconds processed.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageVectorStoresResult object { object, usage\_bytes, project\_id }

The aggregated vector stores usage details of the specific time bucket.

object: "organization.usage.vector\_stores.result"

usage\_bytes: number

The vector stores usage in bytes.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

UsageCodeInterpreterSessionsResult object { object, num\_sessions, project\_id }

The aggregated code interpreter sessions usage details of the specific time bucket.

object: "organization.usage.code\_interpreter\_sessions.result"

num\_sessions: optional number

The number of code interpreter sessions.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

CostsResult object { object, amount, line\_item, project\_id }

The aggregated costs details of the specific time bucket.

object: "organization.costs.result"

amount: optional object { currency, value }

The monetary value in its associated currency.

currency: optional string

Lowercase ISO-4217 currency e.g. “usd”

value: optional number

The numeric value of the cost.

line\_item: optional string

When `group_by=line_item`, this field provides the line item of the grouped costs result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped costs result.

start\_time: number

has\_more: boolean

next\_page: string

object: "page"

UsageGetAudioTranscriptionsResponse object { data, has\_more, next\_page, object }

data: array of object { end\_time, object, result, start\_time }

end\_time: number

object: "bucket"

result: array of object { input\_tokens, num\_model\_requests, object, 10 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or 6 more

One of the following:

UsageCompletionsResult object { input\_tokens, num\_model\_requests, object, 10 more }

The aggregated completions usage details of the specific time bucket.

input\_tokens: number

The aggregated number of text input tokens used, including cached tokens. For customers subscribe to scale tier, this includes scale tier tokens.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.completions.result"

output\_tokens: number

The aggregated number of text output tokens used. For customers subscribe to scale tier, this includes scale tier tokens.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

batch: optional boolean

When `group_by=batch`, this field tells whether the grouped usage result is batch or not.

input\_audio\_tokens: optional number

The aggregated number of audio input tokens used, including cached tokens.

input\_cached\_tokens: optional number

The aggregated number of text input tokens that has been cached from previous requests. For customers subscribe to scale tier, this includes scale tier tokens.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

output\_audio\_tokens: optional number

The aggregated number of audio output tokens used.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

service\_tier: optional string

When `group_by=service_tier`, this field provides the service tier of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageEmbeddingsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated embeddings usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.embeddings.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageModerationsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated moderations usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.moderations.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageImagesResult object { images, num\_model\_requests, object, 6 more }

The aggregated images usage details of the specific time bucket.

images: number

The number of images processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.images.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

size: optional string

When `group_by=size`, this field provides the image size of the grouped usage result.

source: optional string

When `group_by=source`, this field provides the source of the grouped usage result, possible values are `image.generation`, `image.edit`, `image.variation`.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioSpeechesResult object { characters, num\_model\_requests, object, 4 more }

The aggregated audio speeches usage details of the specific time bucket.

characters: number

The number of characters processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_speeches.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioTranscriptionsResult object { num\_model\_requests, object, seconds, 4 more }

The aggregated audio transcriptions usage details of the specific time bucket.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_transcriptions.result"

seconds: number

The number of seconds processed.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageVectorStoresResult object { object, usage\_bytes, project\_id }

The aggregated vector stores usage details of the specific time bucket.

object: "organization.usage.vector\_stores.result"

usage\_bytes: number

The vector stores usage in bytes.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

UsageCodeInterpreterSessionsResult object { object, num\_sessions, project\_id }

The aggregated code interpreter sessions usage details of the specific time bucket.

object: "organization.usage.code\_interpreter\_sessions.result"

num\_sessions: optional number

The number of code interpreter sessions.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

CostsResult object { object, amount, line\_item, project\_id }

The aggregated costs details of the specific time bucket.

object: "organization.costs.result"

amount: optional object { currency, value }

The monetary value in its associated currency.

currency: optional string

Lowercase ISO-4217 currency e.g. “usd”

value: optional number

The numeric value of the cost.

line\_item: optional string

When `group_by=line_item`, this field provides the line item of the grouped costs result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped costs result.

start\_time: number

has\_more: boolean

next\_page: string

object: "page"

UsageGetCodeInterpreterSessionsResponse object { data, has\_more, next\_page, object }

data: array of object { end\_time, object, result, start\_time }

end\_time: number

object: "bucket"

result: array of object { input\_tokens, num\_model\_requests, object, 10 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or 6 more

One of the following:

UsageCompletionsResult object { input\_tokens, num\_model\_requests, object, 10 more }

The aggregated completions usage details of the specific time bucket.

input\_tokens: number

The aggregated number of text input tokens used, including cached tokens. For customers subscribe to scale tier, this includes scale tier tokens.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.completions.result"

output\_tokens: number

The aggregated number of text output tokens used. For customers subscribe to scale tier, this includes scale tier tokens.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

batch: optional boolean

When `group_by=batch`, this field tells whether the grouped usage result is batch or not.

input\_audio\_tokens: optional number

The aggregated number of audio input tokens used, including cached tokens.

input\_cached\_tokens: optional number

The aggregated number of text input tokens that has been cached from previous requests. For customers subscribe to scale tier, this includes scale tier tokens.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

output\_audio\_tokens: optional number

The aggregated number of audio output tokens used.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

service\_tier: optional string

When `group_by=service_tier`, this field provides the service tier of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageEmbeddingsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated embeddings usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.embeddings.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageModerationsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated moderations usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.moderations.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageImagesResult object { images, num\_model\_requests, object, 6 more }

The aggregated images usage details of the specific time bucket.

images: number

The number of images processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.images.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

size: optional string

When `group_by=size`, this field provides the image size of the grouped usage result.

source: optional string

When `group_by=source`, this field provides the source of the grouped usage result, possible values are `image.generation`, `image.edit`, `image.variation`.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioSpeechesResult object { characters, num\_model\_requests, object, 4 more }

The aggregated audio speeches usage details of the specific time bucket.

characters: number

The number of characters processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_speeches.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioTranscriptionsResult object { num\_model\_requests, object, seconds, 4 more }

The aggregated audio transcriptions usage details of the specific time bucket.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_transcriptions.result"

seconds: number

The number of seconds processed.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageVectorStoresResult object { object, usage\_bytes, project\_id }

The aggregated vector stores usage details of the specific time bucket.

object: "organization.usage.vector\_stores.result"

usage\_bytes: number

The vector stores usage in bytes.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

UsageCodeInterpreterSessionsResult object { object, num\_sessions, project\_id }

The aggregated code interpreter sessions usage details of the specific time bucket.

object: "organization.usage.code\_interpreter\_sessions.result"

num\_sessions: optional number

The number of code interpreter sessions.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

CostsResult object { object, amount, line\_item, project\_id }

The aggregated costs details of the specific time bucket.

object: "organization.costs.result"

amount: optional object { currency, value }

The monetary value in its associated currency.

currency: optional string

Lowercase ISO-4217 currency e.g. “usd”

value: optional number

The numeric value of the cost.

line\_item: optional string

When `group_by=line_item`, this field provides the line item of the grouped costs result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped costs result.

start\_time: number

has\_more: boolean

next\_page: string

object: "page"

UsageGetCompletionsResponse object { data, has\_more, next\_page, object }

data: array of object { end\_time, object, result, start\_time }

end\_time: number

object: "bucket"

result: array of object { input\_tokens, num\_model\_requests, object, 10 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or 6 more

One of the following:

UsageCompletionsResult object { input\_tokens, num\_model\_requests, object, 10 more }

The aggregated completions usage details of the specific time bucket.

input\_tokens: number

The aggregated number of text input tokens used, including cached tokens. For customers subscribe to scale tier, this includes scale tier tokens.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.completions.result"

output\_tokens: number

The aggregated number of text output tokens used. For customers subscribe to scale tier, this includes scale tier tokens.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

batch: optional boolean

When `group_by=batch`, this field tells whether the grouped usage result is batch or not.

input\_audio\_tokens: optional number

The aggregated number of audio input tokens used, including cached tokens.

input\_cached\_tokens: optional number

The aggregated number of text input tokens that has been cached from previous requests. For customers subscribe to scale tier, this includes scale tier tokens.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

output\_audio\_tokens: optional number

The aggregated number of audio output tokens used.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

service\_tier: optional string

When `group_by=service_tier`, this field provides the service tier of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageEmbeddingsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated embeddings usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.embeddings.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageModerationsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated moderations usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.moderations.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageImagesResult object { images, num\_model\_requests, object, 6 more }

The aggregated images usage details of the specific time bucket.

images: number

The number of images processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.images.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

size: optional string

When `group_by=size`, this field provides the image size of the grouped usage result.

source: optional string

When `group_by=source`, this field provides the source of the grouped usage result, possible values are `image.generation`, `image.edit`, `image.variation`.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioSpeechesResult object { characters, num\_model\_requests, object, 4 more }

The aggregated audio speeches usage details of the specific time bucket.

characters: number

The number of characters processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_speeches.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioTranscriptionsResult object { num\_model\_requests, object, seconds, 4 more }

The aggregated audio transcriptions usage details of the specific time bucket.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_transcriptions.result"

seconds: number

The number of seconds processed.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageVectorStoresResult object { object, usage\_bytes, project\_id }

The aggregated vector stores usage details of the specific time bucket.

object: "organization.usage.vector\_stores.result"

usage\_bytes: number

The vector stores usage in bytes.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

UsageCodeInterpreterSessionsResult object { object, num\_sessions, project\_id }

The aggregated code interpreter sessions usage details of the specific time bucket.

object: "organization.usage.code\_interpreter\_sessions.result"

num\_sessions: optional number

The number of code interpreter sessions.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

CostsResult object { object, amount, line\_item, project\_id }

The aggregated costs details of the specific time bucket.

object: "organization.costs.result"

amount: optional object { currency, value }

The monetary value in its associated currency.

currency: optional string

Lowercase ISO-4217 currency e.g. “usd”

value: optional number

The numeric value of the cost.

line\_item: optional string

When `group_by=line_item`, this field provides the line item of the grouped costs result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped costs result.

start\_time: number

has\_more: boolean

next\_page: string

object: "page"

UsageGetEmbeddingsResponse object { data, has\_more, next\_page, object }

data: array of object { end\_time, object, result, start\_time }

end\_time: number

object: "bucket"

result: array of object { input\_tokens, num\_model\_requests, object, 10 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or 6 more

One of the following:

UsageCompletionsResult object { input\_tokens, num\_model\_requests, object, 10 more }

The aggregated completions usage details of the specific time bucket.

input\_tokens: number

The aggregated number of text input tokens used, including cached tokens. For customers subscribe to scale tier, this includes scale tier tokens.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.completions.result"

output\_tokens: number

The aggregated number of text output tokens used. For customers subscribe to scale tier, this includes scale tier tokens.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

batch: optional boolean

When `group_by=batch`, this field tells whether the grouped usage result is batch or not.

input\_audio\_tokens: optional number

The aggregated number of audio input tokens used, including cached tokens.

input\_cached\_tokens: optional number

The aggregated number of text input tokens that has been cached from previous requests. For customers subscribe to scale tier, this includes scale tier tokens.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

output\_audio\_tokens: optional number

The aggregated number of audio output tokens used.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

service\_tier: optional string

When `group_by=service_tier`, this field provides the service tier of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageEmbeddingsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated embeddings usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.embeddings.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageModerationsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated moderations usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.moderations.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageImagesResult object { images, num\_model\_requests, object, 6 more }

The aggregated images usage details of the specific time bucket.

images: number

The number of images processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.images.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

size: optional string

When `group_by=size`, this field provides the image size of the grouped usage result.

source: optional string

When `group_by=source`, this field provides the source of the grouped usage result, possible values are `image.generation`, `image.edit`, `image.variation`.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioSpeechesResult object { characters, num\_model\_requests, object, 4 more }

The aggregated audio speeches usage details of the specific time bucket.

characters: number

The number of characters processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_speeches.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioTranscriptionsResult object { num\_model\_requests, object, seconds, 4 more }

The aggregated audio transcriptions usage details of the specific time bucket.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_transcriptions.result"

seconds: number

The number of seconds processed.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageVectorStoresResult object { object, usage\_bytes, project\_id }

The aggregated vector stores usage details of the specific time bucket.

object: "organization.usage.vector\_stores.result"

usage\_bytes: number

The vector stores usage in bytes.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

UsageCodeInterpreterSessionsResult object { object, num\_sessions, project\_id }

The aggregated code interpreter sessions usage details of the specific time bucket.

object: "organization.usage.code\_interpreter\_sessions.result"

num\_sessions: optional number

The number of code interpreter sessions.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

CostsResult object { object, amount, line\_item, project\_id }

The aggregated costs details of the specific time bucket.

object: "organization.costs.result"

amount: optional object { currency, value }

The monetary value in its associated currency.

currency: optional string

Lowercase ISO-4217 currency e.g. “usd”

value: optional number

The numeric value of the cost.

line\_item: optional string

When `group_by=line_item`, this field provides the line item of the grouped costs result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped costs result.

start\_time: number

has\_more: boolean

next\_page: string

object: "page"

UsageGetImagesResponse object { data, has\_more, next\_page, object }

data: array of object { end\_time, object, result, start\_time }

end\_time: number

object: "bucket"

result: array of object { input\_tokens, num\_model\_requests, object, 10 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or 6 more

One of the following:

UsageCompletionsResult object { input\_tokens, num\_model\_requests, object, 10 more }

The aggregated completions usage details of the specific time bucket.

input\_tokens: number

The aggregated number of text input tokens used, including cached tokens. For customers subscribe to scale tier, this includes scale tier tokens.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.completions.result"

output\_tokens: number

The aggregated number of text output tokens used. For customers subscribe to scale tier, this includes scale tier tokens.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

batch: optional boolean

When `group_by=batch`, this field tells whether the grouped usage result is batch or not.

input\_audio\_tokens: optional number

The aggregated number of audio input tokens used, including cached tokens.

input\_cached\_tokens: optional number

The aggregated number of text input tokens that has been cached from previous requests. For customers subscribe to scale tier, this includes scale tier tokens.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

output\_audio\_tokens: optional number

The aggregated number of audio output tokens used.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

service\_tier: optional string

When `group_by=service_tier`, this field provides the service tier of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageEmbeddingsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated embeddings usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.embeddings.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageModerationsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated moderations usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.moderations.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageImagesResult object { images, num\_model\_requests, object, 6 more }

The aggregated images usage details of the specific time bucket.

images: number

The number of images processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.images.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

size: optional string

When `group_by=size`, this field provides the image size of the grouped usage result.

source: optional string

When `group_by=source`, this field provides the source of the grouped usage result, possible values are `image.generation`, `image.edit`, `image.variation`.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioSpeechesResult object { characters, num\_model\_requests, object, 4 more }

The aggregated audio speeches usage details of the specific time bucket.

characters: number

The number of characters processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_speeches.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioTranscriptionsResult object { num\_model\_requests, object, seconds, 4 more }

The aggregated audio transcriptions usage details of the specific time bucket.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_transcriptions.result"

seconds: number

The number of seconds processed.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageVectorStoresResult object { object, usage\_bytes, project\_id }

The aggregated vector stores usage details of the specific time bucket.

object: "organization.usage.vector\_stores.result"

usage\_bytes: number

The vector stores usage in bytes.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

UsageCodeInterpreterSessionsResult object { object, num\_sessions, project\_id }

The aggregated code interpreter sessions usage details of the specific time bucket.

object: "organization.usage.code\_interpreter\_sessions.result"

num\_sessions: optional number

The number of code interpreter sessions.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

CostsResult object { object, amount, line\_item, project\_id }

The aggregated costs details of the specific time bucket.

object: "organization.costs.result"

amount: optional object { currency, value }

The monetary value in its associated currency.

currency: optional string

Lowercase ISO-4217 currency e.g. “usd”

value: optional number

The numeric value of the cost.

line\_item: optional string

When `group_by=line_item`, this field provides the line item of the grouped costs result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped costs result.

start\_time: number

has\_more: boolean

next\_page: string

object: "page"

UsageGetModerationsResponse object { data, has\_more, next\_page, object }

data: array of object { end\_time, object, result, start\_time }

end\_time: number

object: "bucket"

result: array of object { input\_tokens, num\_model\_requests, object, 10 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or 6 more

One of the following:

UsageCompletionsResult object { input\_tokens, num\_model\_requests, object, 10 more }

The aggregated completions usage details of the specific time bucket.

input\_tokens: number

The aggregated number of text input tokens used, including cached tokens. For customers subscribe to scale tier, this includes scale tier tokens.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.completions.result"

output\_tokens: number

The aggregated number of text output tokens used. For customers subscribe to scale tier, this includes scale tier tokens.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

batch: optional boolean

When `group_by=batch`, this field tells whether the grouped usage result is batch or not.

input\_audio\_tokens: optional number

The aggregated number of audio input tokens used, including cached tokens.

input\_cached\_tokens: optional number

The aggregated number of text input tokens that has been cached from previous requests. For customers subscribe to scale tier, this includes scale tier tokens.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

output\_audio\_tokens: optional number

The aggregated number of audio output tokens used.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

service\_tier: optional string

When `group_by=service_tier`, this field provides the service tier of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageEmbeddingsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated embeddings usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.embeddings.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageModerationsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated moderations usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.moderations.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageImagesResult object { images, num\_model\_requests, object, 6 more }

The aggregated images usage details of the specific time bucket.

images: number

The number of images processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.images.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

size: optional string

When `group_by=size`, this field provides the image size of the grouped usage result.

source: optional string

When `group_by=source`, this field provides the source of the grouped usage result, possible values are `image.generation`, `image.edit`, `image.variation`.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioSpeechesResult object { characters, num\_model\_requests, object, 4 more }

The aggregated audio speeches usage details of the specific time bucket.

characters: number

The number of characters processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_speeches.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioTranscriptionsResult object { num\_model\_requests, object, seconds, 4 more }

The aggregated audio transcriptions usage details of the specific time bucket.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_transcriptions.result"

seconds: number

The number of seconds processed.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageVectorStoresResult object { object, usage\_bytes, project\_id }

The aggregated vector stores usage details of the specific time bucket.

object: "organization.usage.vector\_stores.result"

usage\_bytes: number

The vector stores usage in bytes.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

UsageCodeInterpreterSessionsResult object { object, num\_sessions, project\_id }

The aggregated code interpreter sessions usage details of the specific time bucket.

object: "organization.usage.code\_interpreter\_sessions.result"

num\_sessions: optional number

The number of code interpreter sessions.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

CostsResult object { object, amount, line\_item, project\_id }

The aggregated costs details of the specific time bucket.

object: "organization.costs.result"

amount: optional object { currency, value }

The monetary value in its associated currency.

currency: optional string

Lowercase ISO-4217 currency e.g. “usd”

value: optional number

The numeric value of the cost.

line\_item: optional string

When `group_by=line_item`, this field provides the line item of the grouped costs result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped costs result.

start\_time: number

has\_more: boolean

next\_page: string

object: "page"

UsageGetVectorStoresResponse object { data, has\_more, next\_page, object }

data: array of object { end\_time, object, result, start\_time }

end\_time: number

object: "bucket"

result: array of object { input\_tokens, num\_model\_requests, object, 10 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or object { input\_tokens, num\_model\_requests, object, 4 more }  or 6 more

One of the following:

UsageCompletionsResult object { input\_tokens, num\_model\_requests, object, 10 more }

The aggregated completions usage details of the specific time bucket.

input\_tokens: number

The aggregated number of text input tokens used, including cached tokens. For customers subscribe to scale tier, this includes scale tier tokens.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.completions.result"

output\_tokens: number

The aggregated number of text output tokens used. For customers subscribe to scale tier, this includes scale tier tokens.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

batch: optional boolean

When `group_by=batch`, this field tells whether the grouped usage result is batch or not.

input\_audio\_tokens: optional number

The aggregated number of audio input tokens used, including cached tokens.

input\_cached\_tokens: optional number

The aggregated number of text input tokens that has been cached from previous requests. For customers subscribe to scale tier, this includes scale tier tokens.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

output\_audio\_tokens: optional number

The aggregated number of audio output tokens used.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

service\_tier: optional string

When `group_by=service_tier`, this field provides the service tier of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageEmbeddingsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated embeddings usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.embeddings.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageModerationsResult object { input\_tokens, num\_model\_requests, object, 4 more }

The aggregated moderations usage details of the specific time bucket.

input\_tokens: number

The aggregated number of input tokens used.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.moderations.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageImagesResult object { images, num\_model\_requests, object, 6 more }

The aggregated images usage details of the specific time bucket.

images: number

The number of images processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.images.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

size: optional string

When `group_by=size`, this field provides the image size of the grouped usage result.

source: optional string

When `group_by=source`, this field provides the source of the grouped usage result, possible values are `image.generation`, `image.edit`, `image.variation`.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioSpeechesResult object { characters, num\_model\_requests, object, 4 more }

The aggregated audio speeches usage details of the specific time bucket.

characters: number

The number of characters processed.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_speeches.result"

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageAudioTranscriptionsResult object { num\_model\_requests, object, seconds, 4 more }

The aggregated audio transcriptions usage details of the specific time bucket.

num\_model\_requests: number

The count of requests made to the model.

object: "organization.usage.audio\_transcriptions.result"

seconds: number

The number of seconds processed.

api\_key\_id: optional string

When `group_by=api_key_id`, this field provides the API key ID of the grouped usage result.

model: optional string

When `group_by=model`, this field provides the model name of the grouped usage result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

user\_id: optional string

When `group_by=user_id`, this field provides the user ID of the grouped usage result.

UsageVectorStoresResult object { object, usage\_bytes, project\_id }

The aggregated vector stores usage details of the specific time bucket.

object: "organization.usage.vector\_stores.result"

usage\_bytes: number

The vector stores usage in bytes.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

UsageCodeInterpreterSessionsResult object { object, num\_sessions, project\_id }

The aggregated code interpreter sessions usage details of the specific time bucket.

object: "organization.usage.code\_interpreter\_sessions.result"

num\_sessions: optional number

The number of code interpreter sessions.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped usage result.

CostsResult object { object, amount, line\_item, project\_id }

The aggregated costs details of the specific time bucket.

object: "organization.costs.result"

amount: optional object { currency, value }

The monetary value in its associated currency.

currency: optional string

Lowercase ISO-4217 currency e.g. “usd”

value: optional number

The numeric value of the cost.

line\_item: optional string

When `group_by=line_item`, this field provides the line item of the grouped costs result.

project\_id: optional string

When `group_by=project_id`, this field provides the project ID of the grouped costs result.

start\_time: number

has\_more: boolean

next\_page: string

object: "page"

#### OrganizationInvites

##### [List invites](https://developers.openai.com/api/reference/resources/organization/subresources/invites/methods/list)

GET/organization/invites

##### [Create invite](https://developers.openai.com/api/reference/resources/organization/subresources/invites/methods/create)

POST/organization/invites

##### [Retrieve invite](https://developers.openai.com/api/reference/resources/organization/subresources/invites/methods/retrieve)

GET/organization/invites/{invite\_id}

##### [Delete invite](https://developers.openai.com/api/reference/resources/organization/subresources/invites/methods/delete)

DELETE/organization/invites/{invite\_id}

##### Models

Invite object { id, email, expires\_at, 6 more }

Represents an individual `invite` to the organization.

id: string

The identifier, which can be referenced in API endpoints

email: string

The email address of the individual to whom the invite was sent

expires\_at: number

The Unix timestamp (in seconds) of when the invite expires.

invited\_at: number

The Unix timestamp (in seconds) of when the invite was sent.

object: "organization.invite"

The object type, which is always `organization.invite`

role: "owner" or "reader"

`owner` or `reader`

One of the following:

"owner"

"reader"

status: "accepted" or "expired" or "pending"

`accepted`,`expired`, or `pending`

One of the following:

"accepted"

"expired"

"pending"

accepted\_at: optional number

The Unix timestamp (in seconds) of when the invite was accepted.

projects: optional array of object { id, role }

The projects that were granted membership upon acceptance of the invite.

id: optional string

Project’s public ID

role: optional "member" or "owner"

Project membership role

One of the following:

"member"

"owner"

InviteDeleteResponse object { id, deleted, object }

id: string

deleted: boolean

object: "organization.invite.deleted"

The object type, which is always `organization.invite.deleted`

#### OrganizationUsers

##### [List users](https://developers.openai.com/api/reference/resources/organization/subresources/users/methods/list)

GET/organization/users

##### [Retrieve user](https://developers.openai.com/api/reference/resources/organization/subresources/users/methods/retrieve)

GET/organization/users/{user\_id}

##### [Modify user](https://developers.openai.com/api/reference/resources/organization/subresources/users/methods/update)

POST/organization/users/{user\_id}

##### [Delete user](https://developers.openai.com/api/reference/resources/organization/subresources/users/methods/delete)

DELETE/organization/users/{user\_id}

##### Models

User object { id, added\_at, email, 3 more }

Represents an individual `user` within an organization.

id: string

The identifier, which can be referenced in API endpoints

added\_at: number

The Unix timestamp (in seconds) of when the user was added.

email: string

The email address of the user

name: string

The name of the user

object: "organization.user"

The object type, which is always `organization.user`

role: "owner" or "reader"

`owner` or `reader`

One of the following:

"owner"

"reader"

UserDeleteResponse object { id, deleted, object }

id: string

deleted: boolean

object: "organization.user.deleted"

#### OrganizationUsersRoles

##### [List user organization role assignments](https://developers.openai.com/api/reference/resources/organization/subresources/users/subresources/roles/methods/list)

GET/organization/users/{user\_id}/roles

##### [Assign organization role to user](https://developers.openai.com/api/reference/resources/organization/subresources/users/subresources/roles/methods/create)

POST/organization/users/{user\_id}/roles

##### [Unassign organization role from user](https://developers.openai.com/api/reference/resources/organization/subresources/users/subresources/roles/methods/delete)

DELETE/organization/users/{user\_id}/roles/{role\_id}

##### Models

RoleListResponse object { id, created\_at, created\_by, 8 more }

Detailed information about a role assignment entry returned when listing assignments.

id: string

Identifier for the role.

created\_at: number

When the role was created.

formatint64

created\_by: string

Identifier of the actor who created the role.

created\_by\_user\_obj: map[unknown]

User details for the actor that created the role, when available.

description: string

Description of the role.

metadata: map[unknown]

Arbitrary metadata stored on the role.

name: string

Name of the role.

permissions: array of string

Permissions associated with the role.

predefined\_role: boolean

Whether the role is predefined by OpenAI.

resource\_type: string

Resource type the role applies to.

updated\_at: number

When the role was last updated.

formatint64

RoleCreateResponse object { object, role, user }

Role assignment linking a user to a role.

object: "user.role"

Always `user.role`.

role: object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

user: [User](https://developers.openai.com/api/reference/resources/organization#(resource)%20organization.users%20%3E%20(model)%20user%20%3E%20(schema)) { id, added\_at, email, 3 more }

Represents an individual `user` within an organization.

RoleDeleteResponse object { deleted, object }

Confirmation payload returned after unassigning a role.

deleted: boolean

Whether the assignment was removed.

object: string

Identifier for the deleted assignment, such as `group.role.deleted` or `user.role.deleted`.

#### OrganizationGroups

##### [List groups](https://developers.openai.com/api/reference/resources/organization/subresources/groups/methods/list)

GET/organization/groups

##### [Create group](https://developers.openai.com/api/reference/resources/organization/subresources/groups/methods/create)

POST/organization/groups

##### [Update group](https://developers.openai.com/api/reference/resources/organization/subresources/groups/methods/update)

POST/organization/groups/{group\_id}

##### [Delete group](https://developers.openai.com/api/reference/resources/organization/subresources/groups/methods/delete)

DELETE/organization/groups/{group\_id}

##### Models

GroupListResponse object { id, created\_at, is\_scim\_managed, name }

Details about an organization group.

id: string

Identifier for the group.

created\_at: number

Unix timestamp (in seconds) when the group was created.

formatint64

is\_scim\_managed: boolean

Whether the group is managed through SCIM and controlled by your identity provider.

name: string

Display name of the group.

GroupCreateResponse object { id, created\_at, is\_scim\_managed, name }

Details about an organization group.

id: string

Identifier for the group.

created\_at: number

Unix timestamp (in seconds) when the group was created.

formatint64

is\_scim\_managed: boolean

Whether the group is managed through SCIM and controlled by your identity provider.

name: string

Display name of the group.

GroupUpdateResponse object { id, created\_at, is\_scim\_managed, name }

Response returned after updating a group.

id: string

Identifier for the group.

created\_at: number

Unix timestamp (in seconds) when the group was created.

formatint64

is\_scim\_managed: boolean

Whether the group is managed through SCIM and controlled by your identity provider.

name: string

Updated display name for the group.

GroupDeleteResponse object { id, deleted, object }

Confirmation payload returned after deleting a group.

id: string

Identifier of the deleted group.

deleted: boolean

Whether the group was deleted.

object: "group.deleted"

Always `group.deleted`.

#### OrganizationGroupsUsers

##### [List group users](https://developers.openai.com/api/reference/resources/organization/subresources/groups/subresources/users/methods/list)

GET/organization/groups/{group\_id}/users

##### [Add group user](https://developers.openai.com/api/reference/resources/organization/subresources/groups/subresources/users/methods/create)

POST/organization/groups/{group\_id}/users

##### [Remove group user](https://developers.openai.com/api/reference/resources/organization/subresources/groups/subresources/users/methods/delete)

DELETE/organization/groups/{group\_id}/users/{user\_id}

##### Models

UserCreateResponse object { group\_id, object, user\_id }

Confirmation payload returned after adding a user to a group.

group\_id: string

Identifier of the group the user was added to.

object: "group.user"

Always `group.user`.

user\_id: string

Identifier of the user that was added.

UserDeleteResponse object { deleted, object }

Confirmation payload returned after removing a user from a group.

deleted: boolean

Whether the group membership was removed.

object: "group.user.deleted"

Always `group.user.deleted`.

#### OrganizationGroupsRoles

##### [List group organization role assignments](https://developers.openai.com/api/reference/resources/organization/subresources/groups/subresources/roles/methods/list)

GET/organization/groups/{group\_id}/roles

##### [Assign organization role to group](https://developers.openai.com/api/reference/resources/organization/subresources/groups/subresources/roles/methods/create)

POST/organization/groups/{group\_id}/roles

##### [Unassign organization role from group](https://developers.openai.com/api/reference/resources/organization/subresources/groups/subresources/roles/methods/delete)

DELETE/organization/groups/{group\_id}/roles/{role\_id}

##### Models

RoleListResponse object { id, created\_at, created\_by, 8 more }

Detailed information about a role assignment entry returned when listing assignments.

id: string

Identifier for the role.

created\_at: number

When the role was created.

formatint64

created\_by: string

Identifier of the actor who created the role.

created\_by\_user\_obj: map[unknown]

User details for the actor that created the role, when available.

description: string

Description of the role.

metadata: map[unknown]

Arbitrary metadata stored on the role.

name: string

Name of the role.

permissions: array of string

Permissions associated with the role.

predefined\_role: boolean

Whether the role is predefined by OpenAI.

resource\_type: string

Resource type the role applies to.

updated\_at: number

When the role was last updated.

formatint64

RoleCreateResponse object { group, object, role }

Role assignment linking a group to a role.

group: object { id, created\_at, name, 2 more }

Summary information about a group returned in role assignment responses.

id: string

Identifier for the group.

created\_at: number

Unix timestamp (in seconds) when the group was created.

formatint64

name: string

Display name of the group.

object: "group"

Always `group`.

scim\_managed: boolean

Whether the group is managed through SCIM.

object: "group.role"

Always `group.role`.

role: object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

RoleDeleteResponse object { deleted, object }

Confirmation payload returned after unassigning a role.

deleted: boolean

Whether the assignment was removed.

object: string

Identifier for the deleted assignment, such as `group.role.deleted` or `user.role.deleted`.

#### OrganizationRoles

##### [List organization roles](https://developers.openai.com/api/reference/resources/organization/subresources/roles/methods/list)

GET/organization/roles

##### [Create organization role](https://developers.openai.com/api/reference/resources/organization/subresources/roles/methods/create)

POST/organization/roles

##### [Update organization role](https://developers.openai.com/api/reference/resources/organization/subresources/roles/methods/update)

POST/organization/roles/{role\_id}

##### [Delete organization role](https://developers.openai.com/api/reference/resources/organization/subresources/roles/methods/delete)

DELETE/organization/roles/{role\_id}

##### Models

RoleListResponse object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

RoleCreateResponse object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

RoleUpdateResponse object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

RoleDeleteResponse object { id, deleted, object }

Confirmation payload returned after deleting a role.

id: string

Identifier of the deleted role.

deleted: boolean

Whether the role was deleted.

object: "role.deleted"

Always `role.deleted`.

#### OrganizationCertificates

##### [List organization certificates](https://developers.openai.com/api/reference/resources/organization/subresources/certificates/methods/list)

GET/organization/certificates

##### [Upload certificate](https://developers.openai.com/api/reference/resources/organization/subresources/certificates/methods/create)

POST/organization/certificates

##### [Get certificate](https://developers.openai.com/api/reference/resources/organization/subresources/certificates/methods/retrieve)

GET/organization/certificates/{certificate\_id}

##### [Modify certificate](https://developers.openai.com/api/reference/resources/organization/subresources/certificates/methods/update)

POST/organization/certificates/{certificate\_id}

##### [Delete certificate](https://developers.openai.com/api/reference/resources/organization/subresources/certificates/methods/delete)

DELETE/organization/certificates/{certificate\_id}

##### [Activate certificates for organization](https://developers.openai.com/api/reference/resources/organization/subresources/certificates/methods/activate)

POST/organization/certificates/activate

##### [Deactivate certificates for organization](https://developers.openai.com/api/reference/resources/organization/subresources/certificates/methods/deactivate)

POST/organization/certificates/deactivate

##### Models

CertificateListResponse object { id, certificate\_details, created\_at, 3 more }

Represents an individual `certificate` uploaded to the organization.

id: string

The identifier, which can be referenced in API endpoints

certificate\_details: object { content, expires\_at, valid\_at }

content: optional string

The content of the certificate in PEM format.

expires\_at: optional number

The Unix timestamp (in seconds) of when the certificate expires.

valid\_at: optional number

The Unix timestamp (in seconds) of when the certificate becomes valid.

created\_at: number

The Unix timestamp (in seconds) of when the certificate was uploaded.

name: string

The name of the certificate.

object: "certificate" or "organization.certificate" or "organization.project.certificate"

The object type.

- If creating, updating, or getting a specific certificate, the object type is `certificate`.
- If listing, activating, or deactivating certificates for the organization, the object type is `organization.certificate`.
- If listing, activating, or deactivating certificates for a project, the object type is `organization.project.certificate`.

One of the following:

"certificate"

"organization.certificate"

"organization.project.certificate"

active: optional boolean

Whether the certificate is currently active at the specified scope. Not returned when getting details for a specific certificate.

CertificateCreateResponse object { id, certificate\_details, created\_at, 3 more }

Represents an individual `certificate` uploaded to the organization.

id: string

The identifier, which can be referenced in API endpoints

certificate\_details: object { content, expires\_at, valid\_at }

content: optional string

The content of the certificate in PEM format.

expires\_at: optional number

The Unix timestamp (in seconds) of when the certificate expires.

valid\_at: optional number

The Unix timestamp (in seconds) of when the certificate becomes valid.

created\_at: number

The Unix timestamp (in seconds) of when the certificate was uploaded.

name: string

The name of the certificate.

object: "certificate" or "organization.certificate" or "organization.project.certificate"

The object type.

- If creating, updating, or getting a specific certificate, the object type is `certificate`.
- If listing, activating, or deactivating certificates for the organization, the object type is `organization.certificate`.
- If listing, activating, or deactivating certificates for a project, the object type is `organization.project.certificate`.

One of the following:

"certificate"

"organization.certificate"

"organization.project.certificate"

active: optional boolean

Whether the certificate is currently active at the specified scope. Not returned when getting details for a specific certificate.

CertificateRetrieveResponse object { id, certificate\_details, created\_at, 3 more }

Represents an individual `certificate` uploaded to the organization.

id: string

The identifier, which can be referenced in API endpoints

certificate\_details: object { content, expires\_at, valid\_at }

content: optional string

The content of the certificate in PEM format.

expires\_at: optional number

The Unix timestamp (in seconds) of when the certificate expires.

valid\_at: optional number

The Unix timestamp (in seconds) of when the certificate becomes valid.

created\_at: number

The Unix timestamp (in seconds) of when the certificate was uploaded.

name: string

The name of the certificate.

object: "certificate" or "organization.certificate" or "organization.project.certificate"

The object type.

- If creating, updating, or getting a specific certificate, the object type is `certificate`.
- If listing, activating, or deactivating certificates for the organization, the object type is `organization.certificate`.
- If listing, activating, or deactivating certificates for a project, the object type is `organization.project.certificate`.

One of the following:

"certificate"

"organization.certificate"

"organization.project.certificate"

active: optional boolean

Whether the certificate is currently active at the specified scope. Not returned when getting details for a specific certificate.

CertificateUpdateResponse object { id, certificate\_details, created\_at, 3 more }

Represents an individual `certificate` uploaded to the organization.

id: string

The identifier, which can be referenced in API endpoints

certificate\_details: object { content, expires\_at, valid\_at }

content: optional string

The content of the certificate in PEM format.

expires\_at: optional number

The Unix timestamp (in seconds) of when the certificate expires.

valid\_at: optional number

The Unix timestamp (in seconds) of when the certificate becomes valid.

created\_at: number

The Unix timestamp (in seconds) of when the certificate was uploaded.

name: string

The name of the certificate.

object: "certificate" or "organization.certificate" or "organization.project.certificate"

The object type.

- If creating, updating, or getting a specific certificate, the object type is `certificate`.
- If listing, activating, or deactivating certificates for the organization, the object type is `organization.certificate`.
- If listing, activating, or deactivating certificates for a project, the object type is `organization.project.certificate`.

One of the following:

"certificate"

"organization.certificate"

"organization.project.certificate"

active: optional boolean

Whether the certificate is currently active at the specified scope. Not returned when getting details for a specific certificate.

CertificateDeleteResponse object { id, object }

id: string

The ID of the certificate that was deleted.

object: "certificate.deleted"

The object type, must be `certificate.deleted`.

CertificateActivateResponse object { id, certificate\_details, created\_at, 3 more }

Represents an individual `certificate` uploaded to the organization.

id: string

The identifier, which can be referenced in API endpoints

certificate\_details: object { content, expires\_at, valid\_at }

content: optional string

The content of the certificate in PEM format.

expires\_at: optional number

The Unix timestamp (in seconds) of when the certificate expires.

valid\_at: optional number

The Unix timestamp (in seconds) of when the certificate becomes valid.

created\_at: number

The Unix timestamp (in seconds) of when the certificate was uploaded.

name: string

The name of the certificate.

object: "certificate" or "organization.certificate" or "organization.project.certificate"

The object type.

- If creating, updating, or getting a specific certificate, the object type is `certificate`.
- If listing, activating, or deactivating certificates for the organization, the object type is `organization.certificate`.
- If listing, activating, or deactivating certificates for a project, the object type is `organization.project.certificate`.

One of the following:

"certificate"

"organization.certificate"

"organization.project.certificate"

active: optional boolean

Whether the certificate is currently active at the specified scope. Not returned when getting details for a specific certificate.

CertificateDeactivateResponse object { id, certificate\_details, created\_at, 3 more }

Represents an individual `certificate` uploaded to the organization.

id: string

The identifier, which can be referenced in API endpoints

certificate\_details: object { content, expires\_at, valid\_at }

content: optional string

The content of the certificate in PEM format.

expires\_at: optional number

The Unix timestamp (in seconds) of when the certificate expires.

valid\_at: optional number

The Unix timestamp (in seconds) of when the certificate becomes valid.

created\_at: number

The Unix timestamp (in seconds) of when the certificate was uploaded.

name: string

The name of the certificate.

object: "certificate" or "organization.certificate" or "organization.project.certificate"

The object type.

- If creating, updating, or getting a specific certificate, the object type is `certificate`.
- If listing, activating, or deactivating certificates for the organization, the object type is `organization.certificate`.
- If listing, activating, or deactivating certificates for a project, the object type is `organization.project.certificate`.

One of the following:

"certificate"

"organization.certificate"

"organization.project.certificate"

active: optional boolean

Whether the certificate is currently active at the specified scope. Not returned when getting details for a specific certificate.

#### OrganizationProjects

##### [List projects](https://developers.openai.com/api/reference/resources/organization/subresources/projects/methods/list)

GET/organization/projects

##### [Create project](https://developers.openai.com/api/reference/resources/organization/subresources/projects/methods/create)

POST/organization/projects

##### [Retrieve project](https://developers.openai.com/api/reference/resources/organization/subresources/projects/methods/retrieve)

GET/organization/projects/{project\_id}

##### [Modify project](https://developers.openai.com/api/reference/resources/organization/subresources/projects/methods/update)

POST/organization/projects/{project\_id}

##### [Archive project](https://developers.openai.com/api/reference/resources/organization/subresources/projects/methods/archive)

POST/organization/projects/{project\_id}/archive

##### Models

Project object { id, created\_at, name, 3 more }

Represents an individual project.

id: string

The identifier, which can be referenced in API endpoints

created\_at: number

The Unix timestamp (in seconds) of when the project was created.

name: string

The name of the project. This appears in reporting.

object: "organization.project"

The object type, which is always `organization.project`

status: "active" or "archived"

`active` or `archived`

One of the following:

"active"

"archived"

archived\_at: optional number

The Unix timestamp (in seconds) of when the project was archived or `null`.

#### OrganizationProjectsUsers

##### [List project users](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/users/methods/list)

GET/organization/projects/{project\_id}/users

##### [Create project user](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/users/methods/create)

POST/organization/projects/{project\_id}/users

##### [Retrieve project user](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/users/methods/retrieve)

GET/organization/projects/{project\_id}/users/{user\_id}

##### [Modify project user](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/users/methods/update)

POST/organization/projects/{project\_id}/users/{user\_id}

##### [Delete project user](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/users/methods/delete)

DELETE/organization/projects/{project\_id}/users/{user\_id}

##### Models

ProjectUser object { id, added\_at, email, 3 more }

Represents an individual user in a project.

id: string

The identifier, which can be referenced in API endpoints

added\_at: number

The Unix timestamp (in seconds) of when the project was added.

email: string

The email address of the user

name: string

The name of the user

object: "organization.project.user"

The object type, which is always `organization.project.user`

role: "owner" or "member"

`owner` or `member`

One of the following:

"owner"

"member"

UserDeleteResponse object { id, deleted, object }

id: string

deleted: boolean

object: "organization.project.user.deleted"

#### OrganizationProjectsUsersRoles

##### [List project user role assignments](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/users/subresources/roles/methods/list)

GET/projects/{project\_id}/users/{user\_id}/roles

##### [Assign project role to user](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/users/subresources/roles/methods/create)

POST/projects/{project\_id}/users/{user\_id}/roles

##### [Unassign project role from user](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/users/subresources/roles/methods/delete)

DELETE/projects/{project\_id}/users/{user\_id}/roles/{role\_id}

##### Models

RoleListResponse object { id, created\_at, created\_by, 8 more }

Detailed information about a role assignment entry returned when listing assignments.

id: string

Identifier for the role.

created\_at: number

When the role was created.

formatint64

created\_by: string

Identifier of the actor who created the role.

created\_by\_user\_obj: map[unknown]

User details for the actor that created the role, when available.

description: string

Description of the role.

metadata: map[unknown]

Arbitrary metadata stored on the role.

name: string

Name of the role.

permissions: array of string

Permissions associated with the role.

predefined\_role: boolean

Whether the role is predefined by OpenAI.

resource\_type: string

Resource type the role applies to.

updated\_at: number

When the role was last updated.

formatint64

RoleCreateResponse object { object, role, user }

Role assignment linking a user to a role.

object: "user.role"

Always `user.role`.

role: object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

user: [User](https://developers.openai.com/api/reference/resources/organization#(resource)%20organization.users%20%3E%20(model)%20user%20%3E%20(schema)) { id, added\_at, email, 3 more }

Represents an individual `user` within an organization.

RoleDeleteResponse object { deleted, object }

Confirmation payload returned after unassigning a role.

deleted: boolean

Whether the assignment was removed.

object: string

Identifier for the deleted assignment, such as `group.role.deleted` or `user.role.deleted`.

#### OrganizationProjectsService Accounts

##### [List project service accounts](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/service_accounts/methods/list)

GET/organization/projects/{project\_id}/service\_accounts

##### [Create project service account](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/service_accounts/methods/create)

POST/organization/projects/{project\_id}/service\_accounts

##### [Retrieve project service account](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/service_accounts/methods/retrieve)

GET/organization/projects/{project\_id}/service\_accounts/{service\_account\_id}

##### [Delete project service account](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/service_accounts/methods/delete)

DELETE/organization/projects/{project\_id}/service\_accounts/{service\_account\_id}

##### Models

ProjectServiceAccount object { id, created\_at, name, 2 more }

Represents an individual service account in a project.

id: string

The identifier, which can be referenced in API endpoints

created\_at: number

The Unix timestamp (in seconds) of when the service account was created

name: string

The name of the service account

object: "organization.project.service\_account"

The object type, which is always `organization.project.service_account`

role: "owner" or "member"

`owner` or `member`

One of the following:

"owner"

"member"

ServiceAccountCreateResponse object { id, api\_key, created\_at, 3 more }

id: string

api\_key: object { id, created\_at, name, 2 more }

id: string

created\_at: number

name: string

object: "organization.project.service\_account.api\_key"

The object type, which is always `organization.project.service_account.api_key`

value: string

created\_at: number

name: string

object: "organization.project.service\_account"

role: "member"

Service accounts can only have one role of type `member`

ServiceAccountDeleteResponse object { id, deleted, object }

id: string

deleted: boolean

object: "organization.project.service\_account.deleted"

#### OrganizationProjectsAPI Keys

##### [List project API keys](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/api_keys/methods/list)

GET/organization/projects/{project\_id}/api\_keys

##### [Retrieve project API key](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/api_keys/methods/retrieve)

GET/organization/projects/{project\_id}/api\_keys/{key\_id}

##### [Delete project API key](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/api_keys/methods/delete)

DELETE/organization/projects/{project\_id}/api\_keys/{key\_id}

##### Models

ProjectAPIEy object { id, created\_at, last\_used\_at, 4 more }

Represents an individual API key in a project.

id: string

The identifier, which can be referenced in API endpoints

created\_at: number

The Unix timestamp (in seconds) of when the API key was created

last\_used\_at: number

The Unix timestamp (in seconds) of when the API key was last used.

name: string

The name of the API key

object: "organization.project.api\_key"

The object type, which is always `organization.project.api_key`

owner: object { service\_account, type, user }

service\_account: optional [ProjectServiceAccount](https://developers.openai.com/api/reference/resources/organization#(resource)%20organization.projects.service_accounts%20%3E%20(model)%20project_service_account%20%3E%20(schema)) { id, created\_at, name, 2 more }

Represents an individual service account in a project.

type: optional "user" or "service\_account"

`user` or `service_account`

One of the following:

"user"

"service\_account"

user: optional [ProjectUser](https://developers.openai.com/api/reference/resources/organization#(resource)%20organization.projects.users%20%3E%20(model)%20project_user%20%3E%20(schema)) { id, added\_at, email, 3 more }

Represents an individual user in a project.

redacted\_value: string

The redacted value of the API key

APIKeyDeleteResponse object { id, deleted, object }

id: string

deleted: boolean

object: "organization.project.api\_key.deleted"

#### OrganizationProjectsRate Limits

##### [List project rate limits](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/rate_limits/methods/list_rate_limits)

GET/organization/projects/{project\_id}/rate\_limits

##### [Modify project rate limit](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/rate_limits/methods/update_rate_limit)

POST/organization/projects/{project\_id}/rate\_limits/{rate\_limit\_id}

##### Models

RateLimitListRateLimitsResponse object { id, max\_requests\_per\_1\_minute, max\_tokens\_per\_1\_minute, 6 more }

Represents a project rate limit config.

id: string

The identifier, which can be referenced in API endpoints.

max\_requests\_per\_1\_minute: number

The maximum requests per minute.

max\_tokens\_per\_1\_minute: number

The maximum tokens per minute.

model: string

The model this rate limit applies to.

object: "project.rate\_limit"

The object type, which is always `project.rate_limit`

batch\_1\_day\_max\_input\_tokens: optional number

The maximum batch input tokens per day. Only present for relevant models.

max\_audio\_megabytes\_per\_1\_minute: optional number

The maximum audio megabytes per minute. Only present for relevant models.

max\_images\_per\_1\_minute: optional number

The maximum images per minute. Only present for relevant models.

max\_requests\_per\_1\_day: optional number

The maximum requests per day. Only present for relevant models.

RateLimitUpdateRateLimitResponse object { id, max\_requests\_per\_1\_minute, max\_tokens\_per\_1\_minute, 6 more }

Represents a project rate limit config.

id: string

The identifier, which can be referenced in API endpoints.

max\_requests\_per\_1\_minute: number

The maximum requests per minute.

max\_tokens\_per\_1\_minute: number

The maximum tokens per minute.

model: string

The model this rate limit applies to.

object: "project.rate\_limit"

The object type, which is always `project.rate_limit`

batch\_1\_day\_max\_input\_tokens: optional number

The maximum batch input tokens per day. Only present for relevant models.

max\_audio\_megabytes\_per\_1\_minute: optional number

The maximum audio megabytes per minute. Only present for relevant models.

max\_images\_per\_1\_minute: optional number

The maximum images per minute. Only present for relevant models.

max\_requests\_per\_1\_day: optional number

The maximum requests per day. Only present for relevant models.

#### OrganizationProjectsGroups

##### [List project groups](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/groups/methods/list)

GET/organization/projects/{project\_id}/groups

##### [Add project group](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/groups/methods/create)

POST/organization/projects/{project\_id}/groups

##### [Remove project group](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/groups/methods/delete)

DELETE/organization/projects/{project\_id}/groups/{group\_id}

##### Models

GroupListResponse object { created\_at, group\_id, group\_name, 2 more }

Details about a group’s membership in a project.

created\_at: number

Unix timestamp (in seconds) when the group was granted project access.

formatint64

group\_id: string

Identifier of the group that has access to the project.

group\_name: string

Display name of the group.

object: "project.group"

Always `project.group`.

project\_id: string

Identifier of the project.

GroupCreateResponse object { created\_at, group\_id, group\_name, 2 more }

Details about a group’s membership in a project.

created\_at: number

Unix timestamp (in seconds) when the group was granted project access.

formatint64

group\_id: string

Identifier of the group that has access to the project.

group\_name: string

Display name of the group.

object: "project.group"

Always `project.group`.

project\_id: string

Identifier of the project.

GroupDeleteResponse object { deleted, object }

Confirmation payload returned after removing a group from a project.

deleted: boolean

Whether the group membership in the project was removed.

object: "project.group.deleted"

Always `project.group.deleted`.

#### OrganizationProjectsGroupsRoles

##### [List project group role assignments](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/groups/subresources/roles/methods/list)

GET/projects/{project\_id}/groups/{group\_id}/roles

##### [Assign project role to group](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/groups/subresources/roles/methods/create)

POST/projects/{project\_id}/groups/{group\_id}/roles

##### [Unassign project role from group](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/groups/subresources/roles/methods/delete)

DELETE/projects/{project\_id}/groups/{group\_id}/roles/{role\_id}

##### Models

RoleListResponse object { id, created\_at, created\_by, 8 more }

Detailed information about a role assignment entry returned when listing assignments.

id: string

Identifier for the role.

created\_at: number

When the role was created.

formatint64

created\_by: string

Identifier of the actor who created the role.

created\_by\_user\_obj: map[unknown]

User details for the actor that created the role, when available.

description: string

Description of the role.

metadata: map[unknown]

Arbitrary metadata stored on the role.

name: string

Name of the role.

permissions: array of string

Permissions associated with the role.

predefined\_role: boolean

Whether the role is predefined by OpenAI.

resource\_type: string

Resource type the role applies to.

updated\_at: number

When the role was last updated.

formatint64

RoleCreateResponse object { group, object, role }

Role assignment linking a group to a role.

group: object { id, created\_at, name, 2 more }

Summary information about a group returned in role assignment responses.

id: string

Identifier for the group.

created\_at: number

Unix timestamp (in seconds) when the group was created.

formatint64

name: string

Display name of the group.

object: "group"

Always `group`.

scim\_managed: boolean

Whether the group is managed through SCIM.

object: "group.role"

Always `group.role`.

role: object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

RoleDeleteResponse object { deleted, object }

Confirmation payload returned after unassigning a role.

deleted: boolean

Whether the assignment was removed.

object: string

Identifier for the deleted assignment, such as `group.role.deleted` or `user.role.deleted`.

#### OrganizationProjectsRoles

##### [List project roles](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/roles/methods/list)

GET/projects/{project\_id}/roles

##### [Create project role](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/roles/methods/create)

POST/projects/{project\_id}/roles

##### [Update project role](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/roles/methods/update)

POST/projects/{project\_id}/roles/{role\_id}

##### [Delete project role](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/roles/methods/delete)

DELETE/projects/{project\_id}/roles/{role\_id}

##### Models

RoleListResponse object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

RoleCreateResponse object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

RoleUpdateResponse object { id, description, name, 4 more }

Details about a role that can be assigned through the public Roles API.

id: string

Identifier for the role.

description: string

Optional description of the role.

name: string

Unique name for the role.

object: "role"

Always `role`.

permissions: array of string

Permissions granted by the role.

predefined\_role: boolean

Whether the role is predefined and managed by OpenAI.

resource\_type: string

Resource type the role is bound to (for example `api.organization` or `api.project`).

RoleDeleteResponse object { id, deleted, object }

Confirmation payload returned after deleting a role.

id: string

Identifier of the deleted role.

deleted: boolean

Whether the role was deleted.

object: "role.deleted"

Always `role.deleted`.

#### OrganizationProjectsCertificates

##### [List project certificates](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/certificates/methods/list)

GET/organization/projects/{project\_id}/certificates

##### [Activate certificates for project](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/certificates/methods/activate)

POST/organization/projects/{project\_id}/certificates/activate

##### [Deactivate certificates for project](https://developers.openai.com/api/reference/resources/organization/subresources/projects/subresources/certificates/methods/deactivate)

POST/organization/projects/{project\_id}/certificates/deactivate

##### Models

CertificateListResponse object { id, certificate\_details, created\_at, 3 more }

Represents an individual `certificate` uploaded to the organization.

id: string

The identifier, which can be referenced in API endpoints

certificate\_details: object { content, expires\_at, valid\_at }

content: optional string

The content of the certificate in PEM format.

expires\_at: optional number

The Unix timestamp (in seconds) of when the certificate expires.

valid\_at: optional number

The Unix timestamp (in seconds) of when the certificate becomes valid.

created\_at: number

The Unix timestamp (in seconds) of when the certificate was uploaded.

name: string

The name of the certificate.

object: "certificate" or "organization.certificate" or "organization.project.certificate"

The object type.

- If creating, updating, or getting a specific certificate, the object type is `certificate`.
- If listing, activating, or deactivating certificates for the organization, the object type is `organization.certificate`.
- If listing, activating, or deactivating certificates for a project, the object type is `organization.project.certificate`.

One of the following:

"certificate"

"organization.certificate"

"organization.project.certificate"

active: optional boolean

Whether the certificate is currently active at the specified scope. Not returned when getting details for a specific certificate.

CertificateActivateResponse object { id, certificate\_details, created\_at, 3 more }

Represents an individual `certificate` uploaded to the organization.

id: string

The identifier, which can be referenced in API endpoints

certificate\_details: object { content, expires\_at, valid\_at }

content: optional string

The content of the certificate in PEM format.

expires\_at: optional number

The Unix timestamp (in seconds) of when the certificate expires.

valid\_at: optional number

The Unix timestamp (in seconds) of when the certificate becomes valid.

created\_at: number

The Unix timestamp (in seconds) of when the certificate was uploaded.

name: string

The name of the certificate.

object: "certificate" or "organization.certificate" or "organization.project.certificate"

The object type.

- If creating, updating, or getting a specific certificate, the object type is `certificate`.
- If listing, activating, or deactivating certificates for the organization, the object type is `organization.certificate`.
- If listing, activating, or deactivating certificates for a project, the object type is `organization.project.certificate`.

One of the following:

"certificate"

"organization.certificate"

"organization.project.certificate"

active: optional boolean

Whether the certificate is currently active at the specified scope. Not returned when getting details for a specific certificate.

CertificateDeactivateResponse object { id, certificate\_details, created\_at, 3 more }

Represents an individual `certificate` uploaded to the organization.

id: string

The identifier, which can be referenced in API endpoints

certificate\_details: object { content, expires\_at, valid\_at }

content: optional string

The content of the certificate in PEM format.

expires\_at: optional number

The Unix timestamp (in seconds) of when the certificate expires.

valid\_at: optional number

The Unix timestamp (in seconds) of when the certificate becomes valid.

created\_at: number

The Unix timestamp (in seconds) of when the certificate was uploaded.

name: string

The name of the certificate.

object: "certificate" or "organization.certificate" or "organization.project.certificate"

The object type.

- If creating, updating, or getting a specific certificate, the object type is `certificate`.
- If listing, activating, or deactivating certificates for the organization, the object type is `organization.certificate`.
- If listing, activating, or deactivating certificates for a project, the object type is `organization.project.certificate`.

One of the following:

"certificate"

"organization.certificate"

"organization.project.certificate"

active: optional boolean

Whether the certificate is currently active at the specified scope. Not returned when getting details for a specific certificate.
