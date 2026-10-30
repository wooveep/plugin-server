# Webhook Events

> Source: https://developers.openai.com/api/reference/resources/webhooks
> Fetched: 2026-04-23

Sent when a background response has been completed.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the model response was completed.

data: object { id }

Event data payload.

id: string

The unique ID of the model response.

type: "response.completed"

The type of the event. Always `response.completed`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### response.completed

```
{
  "id": "evt_abc123",
  "type": "response.completed",
  "created_at": 1719168000,
  "data": {
    "id": "resp_abc123"
  }
}
```

Sent when a background response has been cancelled.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the model response was cancelled.

data: object { id }

Event data payload.

id: string

The unique ID of the model response.

type: "response.cancelled"

The type of the event. Always `response.cancelled`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### response.cancelled

```
{
  "id": "evt_abc123",
  "type": "response.cancelled",
  "created_at": 1719168000,
  "data": {
    "id": "resp_abc123"
  }
}
```

Sent when a background response has failed.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the model response failed.

data: object { id }

Event data payload.

id: string

The unique ID of the model response.

type: "response.failed"

The type of the event. Always `response.failed`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### response.failed

```
{
  "id": "evt_abc123",
  "type": "response.failed",
  "created_at": 1719168000,
  "data": {
    "id": "resp_abc123"
  }
}
```

Sent when a background response has been interrupted.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the model response was interrupted.

data: object { id }

Event data payload.

id: string

The unique ID of the model response.

type: "response.incomplete"

The type of the event. Always `response.incomplete`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### response.incomplete

```
{
  "id": "evt_abc123",
  "type": "response.incomplete",
  "created_at": 1719168000,
  "data": {
    "id": "resp_abc123"
  }
}
```

Sent when a batch API request has been completed.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the batch API request was completed.

data: object { id }

Event data payload.

id: string

The unique ID of the batch API request.

type: "batch.completed"

The type of the event. Always `batch.completed`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### batch.completed

```
{
  "id": "evt_abc123",
  "type": "batch.completed",
  "created_at": 1719168000,
  "data": {
    "id": "batch_abc123"
  }
}
```

Sent when a batch API request has been cancelled.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the batch API request was cancelled.

data: object { id }

Event data payload.

id: string

The unique ID of the batch API request.

type: "batch.cancelled"

The type of the event. Always `batch.cancelled`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### batch.cancelled

```
{
  "id": "evt_abc123",
  "type": "batch.cancelled",
  "created_at": 1719168000,
  "data": {
    "id": "batch_abc123"
  }
}
```

Sent when a batch API request has expired.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the batch API request expired.

data: object { id }

Event data payload.

id: string

The unique ID of the batch API request.

type: "batch.expired"

The type of the event. Always `batch.expired`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### batch.expired

```
{
  "id": "evt_abc123",
  "type": "batch.expired",
  "created_at": 1719168000,
  "data": {
    "id": "batch_abc123"
  }
}
```

Sent when a batch API request has failed.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the batch API request failed.

data: object { id }

Event data payload.

id: string

The unique ID of the batch API request.

type: "batch.failed"

The type of the event. Always `batch.failed`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### batch.failed

```
{
  "id": "evt_abc123",
  "type": "batch.failed",
  "created_at": 1719168000,
  "data": {
    "id": "batch_abc123"
  }
}
```

Sent when a fine-tuning job has succeeded.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the fine-tuning job succeeded.

data: object { id }

Event data payload.

id: string

The unique ID of the fine-tuning job.

type: "fine\_tuning.job.succeeded"

The type of the event. Always `fine_tuning.job.succeeded`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### fine\_tuning.job.succeeded

```
{
  "id": "evt_abc123",
  "type": "fine_tuning.job.succeeded",
  "created_at": 1719168000,
  "data": {
    "id": "ftjob_abc123"
  }
}
```

Sent when a fine-tuning job has failed.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the fine-tuning job failed.

data: object { id }

Event data payload.

id: string

The unique ID of the fine-tuning job.

type: "fine\_tuning.job.failed"

The type of the event. Always `fine_tuning.job.failed`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### fine\_tuning.job.failed

```
{
  "id": "evt_abc123",
  "type": "fine_tuning.job.failed",
  "created_at": 1719168000,
  "data": {
    "id": "ftjob_abc123"
  }
}
```

Sent when a fine-tuning job has been cancelled.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the fine-tuning job was cancelled.

data: object { id }

Event data payload.

id: string

The unique ID of the fine-tuning job.

type: "fine\_tuning.job.cancelled"

The type of the event. Always `fine_tuning.job.cancelled`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### fine\_tuning.job.cancelled

```
{
  "id": "evt_abc123",
  "type": "fine_tuning.job.cancelled",
  "created_at": 1719168000,
  "data": {
    "id": "ftjob_abc123"
  }
}
```

Sent when an eval run has succeeded.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the eval run succeeded.

data: object { id }

Event data payload.

id: string

The unique ID of the eval run.

type: "eval.run.succeeded"

The type of the event. Always `eval.run.succeeded`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### eval.run.succeeded

```
{
  "id": "evt_abc123",
  "type": "eval.run.succeeded",
  "created_at": 1719168000,
  "data": {
    "id": "evalrun_abc123"
  }
}
```

Sent when an eval run has failed.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the eval run failed.

data: object { id }

Event data payload.

id: string

The unique ID of the eval run.

type: "eval.run.failed"

The type of the event. Always `eval.run.failed`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### eval.run.failed

```
{
  "id": "evt_abc123",
  "type": "eval.run.failed",
  "created_at": 1719168000,
  "data": {
    "id": "evalrun_abc123"
  }
}
```

Sent when an eval run has been canceled.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the eval run was canceled.

data: object { id }

Event data payload.

id: string

The unique ID of the eval run.

type: "eval.run.canceled"

The type of the event. Always `eval.run.canceled`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### eval.run.canceled

```
{
  "id": "evt_abc123",
  "type": "eval.run.canceled",
  "created_at": 1719168000,
  "data": {
    "id": "evalrun_abc123"
  }
}
```

Sent when Realtime API Receives a incoming SIP call.

id: string

The unique ID of the event.

created\_at: number

The Unix timestamp (in seconds) of when the model response was completed.

data: object { call\_id, sip\_headers }

Event data payload.

call\_id: string

The unique ID of this call.

sip\_headers: array of object { name, value }

Headers from the SIP Invite.

name: string

Name of the SIP Header.

value: string

Value of the SIP Header.

type: "realtime.call.incoming"

The type of the event. Always `realtime.call.incoming`.

object: optional "event"

The object of the event. Always `event`.

OBJECT

### realtime.call.incoming

```
{
  "id": "evt_abc123",
  "type": "realtime.call.incoming",
  "created_at": 1719168000,
  "data": {
    "call_id": "rtc_479a275623b54bdb9b6fbae2f7cbd408",
    "sip_headers": [
      {"name": "Max-Forwards", "value": "63"},
      {"name": "CSeq", "value": "851287 INVITE"},
      {"name": "Content-Type", "value": "application/sdp"},
    ]
  }
}
```
