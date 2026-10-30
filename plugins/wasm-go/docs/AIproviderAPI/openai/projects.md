# Projects

> Source: https://developers.openai.com/api/reference/resources/projects
> Fetched: 2026-04-23

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

#### ProjectsUsers

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

#### ProjectsUsersRoles

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

#### ProjectsService Accounts

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

#### ProjectsAPI Keys

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

#### ProjectsRate Limits

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

#### ProjectsGroups

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

#### ProjectsGroupsRoles

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

#### ProjectsRoles

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

#### ProjectsCertificates

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
