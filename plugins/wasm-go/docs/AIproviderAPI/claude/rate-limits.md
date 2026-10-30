# Claude API Limits And Availability

> Sources:
> - https://platform.claude.com/docs/en/api/rate-limits
> - https://platform.claude.com/docs/en/api/service-tiers
> - https://platform.claude.com/docs/en/api/ip-addresses
> - https://platform.claude.com/docs/en/api/supported-regions
> Fetched: 2026-04-23

## Rate limits

To mitigate misuse and manage capacity on the API, limits are in place on how much an organization can use the Claude API.

---

There are two types of limits:

1. **Spend limits** set a maximum monthly cost an organization can incur for API usage.
2. **Rate limits** set the maximum number of API requests an organization can make over a defined period of time.

The API enforces service-configured limits at the organization level, but you may also set user-configurable limits for your organization's workspaces.

These limits apply to both Standard and Priority Tier usage. For more information about Priority Tier, which offers enhanced service levels in exchange for committed spend, see [Service Tiers](https://platform.claude.com/docs/en/api/service-tiers).

### About rate limits

* Limits are designed to prevent API abuse, while minimizing impact on common customer usage patterns.
* Limits are defined by **usage tier**, where each tier is associated with a different set of spend and rate limits.
* Your organization will increase tiers automatically as you reach certain thresholds while using the API.
  Limits are set at the organization level. You can see your organization's limits on the [Limits](https://platform.claude.com/settings/limits) page in the [Claude Console](/).
* You may hit rate limits over shorter time intervals. For instance, a rate of 60 requests per minute (RPM) may be enforced as 1 request per second. Short bursts of requests can exceed the limit and trigger rate limit errors.
* The limits outlined below are the standard tier limits. If you're seeking higher, custom limits or Priority Tier for enhanced service levels, contact sales on the [Limits](https://platform.claude.com/settings/limits) page.
* The API uses the [token bucket algorithm](https://en.wikipedia.org/wiki/Token_bucket) to do rate limiting. This means that your capacity is continuously replenished up to your maximum limit, rather than being reset at fixed intervals.
* All limits described here represent maximum allowed usage, not guaranteed minimums. These limits are intended to reduce unintentional overspend and ensure fair distribution of resources among users.

### Spend limits

Each usage tier has a limit on how much you can spend on the API each calendar month. Once you reach the spend limit of your tier, until you qualify for the next tier, you will have to wait until the next month to be able to use the API again.

To qualify for the next tier, you must meet a deposit requirement. To minimize the risk of overfunding your account, you cannot deposit more than your monthly spend limit.

#### Requirements to advance tier
<table>
  <thead>
    <tr>
      <th>Usage Tier</th>
      <th>Credit Purchase</th>
      <th>Max Credit Purchase</th>
      <th>Monthly Spend Limit</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Tier 1</td>
      <td>\$5</td>
      <td>\$100</td>
      <td>\$100</td>
    </tr>
    <tr>
      <td>Tier 2</td>
      <td>\$40</td>
      <td>\$500</td>
      <td>\$500</td>
    </tr>
    <tr>
      <td>Tier 3</td>
      <td>\$200</td>
      <td>\$1,000</td>
      <td>\$1,000</td>
    </tr>
    <tr>
      <td>Tier 4</td>
      <td>\$400</td>
      <td>\$200,000</td>
      <td>\$200,000</td>
    </tr>
    <tr>
      <td>Monthly Invoicing</td>
      <td>N/A</td>
      <td>N/A</td>
      <td>No limit</td>
    </tr>
  </tbody>
</table>

> **Credit Purchase** shows the cumulative credit purchases (excluding tax) required to advance to that tier. You advance immediately upon reaching the threshold.
>
> **Max Credit Purchase** limits the maximum amount you can add to your account in a single transaction to prevent account overfunding.
>
> **Monthly Spend Limit** is the maximum you can spend on the API each calendar month at that tier.

### Increasing your spend limits

Your organization has two kinds of spend limits: a customer-set limit you control directly, and a tier-enforced ceiling set by your usage tier. Each has a different process for increasing it.

#### Customer-set spend limits

You can set a spend limit lower than your tier's ceiling to control costs. To adjust it:

<Steps>
  <Step title="Navigate to the Limits page">
    Go to [Settings > Limits](https://platform.claude.com/settings/limits) in the Claude Console.
  </Step>
  <Step title="Open the spend limit editor">
    In the **Spend limits** section, click **Change Limit** (or **Set spend limit** if no limit is currently set).
  </Step>
  <Step title="Adjust your spend limit">
    Enter a new value. Your customer-set limit cannot exceed your current tier's limit.
  </Step>
</Steps>

#### Tier-enforced spend limits

When you need a limit higher than your tier's ceiling (Tier 4's ceiling is $200,000 per month), click **Contact Sales** on the [Limits](https://platform.claude.com/settings/limits) page. This opens the contact form in a new tab, and a member of the sales team will follow up by email when your organization is upgraded.

Monthly Invoicing removes the monthly spend cap entirely and uses Net-30 payment terms by default.

> Support can also raise tier-enforced limits. For urgent needs, contact [support](https://support.anthropic.com).

### Rate limits

The rate limits for the Messages API are measured in requests per minute (RPM), input tokens per minute (ITPM), and output tokens per minute (OTPM) for each model class.
If you exceed any of the rate limits you will get a [429 error](https://platform.claude.com/docs/en/api/errors) describing which rate limit was exceeded, along with a `retry-after` header indicating how long to wait.

> You might also encounter 429 errors due to acceleration limits on the API if your organization has a sharp increase in usage. To avoid hitting acceleration limits, ramp up your traffic gradually and maintain consistent usage patterns.

#### Cache-aware ITPM

Many API providers use a combined "tokens per minute" (TPM) limit that may include all tokens, both cached and uncached, input and output. **For most Claude models, only uncached input tokens count towards your ITPM rate limits.** This is a key advantage that makes the rate limits effectively higher than they might initially appear.

ITPM rate limits are estimated at the beginning of each request, and the estimate is adjusted during the request to reflect the actual number of input tokens used.

Here's what counts towards ITPM:
- `input_tokens` (tokens after the last cache breakpoint) ✓ **Count towards ITPM**
- `cache_creation_input_tokens` (tokens being written to cache) ✓ **Count towards ITPM**
- `cache_read_input_tokens` (tokens read from cache) ✗ **Do NOT count towards ITPM** for most models

> The `input_tokens` field only represents tokens that appear **after your last cache breakpoint**, not all input tokens in your request. To calculate total input tokens:
>
> ```text
> total_input_tokens = cache_read_input_tokens + cache_creation_input_tokens + input_tokens
> ```
>
> This means when you have cached content, `input_tokens` will typically be much smaller than your total input. For example, with a 200k token cached document and a 50 token user question, you'd see `input_tokens: 50` even though the total input is 200,050 tokens.
>
> For rate limit purposes on most models, only `input_tokens` + `cache_creation_input_tokens` count toward your ITPM limit, making [prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) an effective way to increase your effective throughput.

**Example**: With a 2,000,000 ITPM limit and an 80% cache hit rate, you could effectively process 10,000,000 total input tokens per minute (2M uncached + 8M cached), since cached tokens don't count towards your rate limit.

> Some older models (marked with † in the rate limit tables below) also count `cache_read_input_tokens` towards ITPM rate limits.
>
> For all models without the † marker, cached input tokens do not count towards rate limits and are billed at a reduced rate (10% of base input token price). This means you can achieve significantly higher effective throughput by using [prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching).

> **Maximize your rate limits with prompt caching**
>
> To get the most out of your rate limits, use [prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) for repeated content like:
> - System instructions and prompts
> - Large context documents
> - Tool definitions
> - Conversation history
>
> With effective caching, you can dramatically increase your actual throughput without increasing your rate limits. Monitor your cache hit rate on the [Usage page](https://platform.claude.com/usage) to optimize your caching strategy.

OTPM rate limits are evaluated in real time as output tokens are produced, counting only the actual tokens generated. The `max_tokens` parameter does not factor into OTPM rate limit calculations, so there is no rate limit downside to setting a higher `max_tokens` value.

Rate limits are applied separately for each model; therefore you can use different models up to their respective limits simultaneously.
You can check your current rate limits and behavior in the [Claude Console](https://platform.claude.com/settings/limits).

> Rate limits are currently shared across all `inference_geo` values. Requests with `inference_geo: "us"` and `inference_geo: "global"` draw from the same rate limit pool.

<Tabs>
<Tab title="Tier 1">
| Model                                                                                        | Maximum requests per minute (RPM) | Maximum input tokens per minute (ITPM) | Maximum output tokens per minute (OTPM) |
| -------------------------------------------------------------------------------------------- | --------------------------------- | -------------------------------------- | --------------------------------------- |
| Claude Sonnet 4.x<sup>**</sup>                                                               | 50                                | 30,000                                 | 8,000                                   |
| Claude Sonnet 3.7 ([deprecated](https://platform.claude.com/docs/en/about-claude/model-deprecations))                   | 50                                | 20,000                                 | 8,000                                   |
| Claude Haiku 4.5                                                                             | 50                                | 50,000                                 | 10,000                                  |
| Claude Haiku 3.5 ([deprecated](https://platform.claude.com/docs/en/about-claude/model-deprecations))                    | 50                                | 50,000<sup>†</sup>                     | 10,000                                  |
| Claude Opus 4.x<sup>*</sup>                                                                  | 50                                | 30,000                                 | 8,000                                   |

</Tab>
<Tab title="Tier 2">
| Model                                                                                        | Maximum requests per minute (RPM) | Maximum input tokens per minute (ITPM) | Maximum output tokens per minute (OTPM) |
| -------------------------------------------------------------------------------------------- | --------------------------------- | -------------------------------------- | --------------------------------------- |
| Claude Sonnet 4.x<sup>**</sup>                                                               | 1,000                             | 450,000                                | 90,000                                  |
| Claude Sonnet 3.7 ([deprecated](https://platform.claude.com/docs/en/about-claude/model-deprecations))                   | 1,000                             | 40,000                                 | 16,000                                  |
| Claude Haiku 4.5                                                                             | 1,000                             | 450,000                                | 90,000                                  |
| Claude Haiku 3.5 ([deprecated](https://platform.claude.com/docs/en/about-claude/model-deprecations))                    | 1,000                             | 100,000<sup>†</sup>                    | 20,000                                  |
| Claude Opus 4.x<sup>*</sup>                                                                  | 1,000                             | 450,000                                | 90,000                                  |

</Tab>
<Tab title="Tier 3">
| Model                                                                                        | Maximum requests per minute (RPM) | Maximum input tokens per minute (ITPM) | Maximum output tokens per minute (OTPM) |
| -------------------------------------------------------------------------------------------- | --------------------------------- | -------------------------------------- | --------------------------------------- |
| Claude Sonnet 4.x<sup>**</sup>                                                               | 2,000                             | 800,000                                | 160,000                                 |
| Claude Sonnet 3.7 ([deprecated](https://platform.claude.com/docs/en/about-claude/model-deprecations))                   | 2,000                             | 80,000                                 | 32,000                                  |
| Claude Haiku 4.5                                                                             | 2,000                             | 1,000,000                              | 200,000                                 |
| Claude Haiku 3.5 ([deprecated](https://platform.claude.com/docs/en/about-claude/model-deprecations))                    | 2,000                             | 200,000<sup>†</sup>                    | 40,000                                  |
| Claude Opus 4.x<sup>*</sup>                                                                  | 2,000                             | 800,000                                | 160,000                                 |

</Tab>
<Tab title="Tier 4">
| Model                                                                                        | Maximum requests per minute (RPM) | Maximum input tokens per minute (ITPM) | Maximum output tokens per minute (OTPM) |
| -------------------------------------------------------------------------------------------- | --------------------------------- | -------------------------------------- | --------------------------------------- |
| Claude Sonnet 4.x<sup>**</sup>                                                               | 4,000                             | 2,000,000                              | 400,000                                 |
| Claude Sonnet 3.7 ([deprecated](https://platform.claude.com/docs/en/about-claude/model-deprecations))                   | 4,000                             | 200,000                                | 80,000                                  |
| Claude Haiku 4.5                                                                             | 4,000                             | 4,000,000                              | 800,000                                 |
| Claude Haiku 3.5 ([deprecated](https://platform.claude.com/docs/en/about-claude/model-deprecations))                    | 4,000                             | 400,000<sup>†</sup>                    | 80,000                                  |
| Claude Opus 4.x<sup>*</sup>                                                                  | 4,000                             | 2,000,000                              | 400,000                                 |

</Tab>
<Tab title="Custom">
If you're seeking higher limits for an Enterprise use case, contact sales through the [Claude Console](https://platform.claude.com/settings/limits).
</Tab>
</Tabs>

_<sup>* - Opus rate limit is a total limit that applies to combined traffic across Opus 4.7, Opus 4.6, Opus 4.5, Opus 4.1, and Opus 4.</sup>_

_<sup>** - Sonnet 4.x rate limit is a total limit that applies to combined traffic across Sonnet 4.6, Sonnet 4.5, and Sonnet 4.</sup>_

_<sup>† - Limit counts `cache_read_input_tokens` towards ITPM usage.</sup>_

#### Message Batches API

The Message Batches API has its own set of rate limits which are shared across all models. These include a requests per minute (RPM) limit to all API endpoints and a limit on the number of batch requests that can be in the processing queue at the same time. A "batch request" here refers to part of a Message Batch. You may create a Message Batch containing thousands of batch requests, each of which count towards this limit. A batch request is considered part of the processing queue when it has yet to be successfully processed by the model.

<Tabs>
<Tab title="Tier 1">
| Maximum requests per minute (RPM) | Maximum batch requests in processing queue | Maximum batch requests per batch |
| --------------------------------- | ------------------------------------------ | -------------------------------- |
| 50                                | 100,000                                    | 100,000                          |
</Tab>
<Tab title="Tier 2">
| Maximum requests per minute (RPM) | Maximum batch requests in processing queue | Maximum batch requests per batch |
| --------------------------------- | ------------------------------------------ | -------------------------------- |
| 1,000                             | 200,000                                    | 100,000                          |
</Tab>
<Tab title="Tier 3">
| Maximum requests per minute (RPM) | Maximum batch requests in processing queue | Maximum batch requests per batch |
| --------------------------------- | ------------------------------------------ | -------------------------------- |
| 2,000                             | 300,000                                    | 100,000                          |
</Tab>
<Tab title="Tier 4">
| Maximum requests per minute (RPM) | Maximum batch requests in processing queue | Maximum batch requests per batch |
| --------------------------------- | ------------------------------------------ | -------------------------------- |
| 4,000                             | 500,000                                    | 100,000                          |
</Tab>
<Tab title="Custom">
If you're seeking higher limits for an Enterprise use case, contact sales through the [Claude Console](https://platform.claude.com/settings/limits).
</Tab>
</Tabs>

#### Managed Agents

[Claude Managed Agents](https://platform.claude.com/docs/en/managed-agents/overview) endpoints are rate-limited per organization. These limits are separate from the Messages API rate limits above.

| Operation | Limit |
| --- | --- |
| Create endpoints (agents, sessions, environments, etc.) | 60 requests per minute |
| Read endpoints (retrieve, list, stream, etc.) | 600 requests per minute |

#### Fast mode rate limits

When using [fast mode](https://platform.claude.com/docs/en/build-with-claude/fast-mode) (beta: research preview) with `speed: "fast"` on Opus 4.6, dedicated rate limits apply that are separate from standard Opus rate limits. When fast mode rate limits are exceeded, the API returns a `429` error with a `retry-after` header.

The response includes `anthropic-fast-*` headers that indicate your fast mode rate limit status. See the [fast mode documentation](https://platform.claude.com/docs/en/build-with-claude/fast-mode#rate-limits) for details on these headers.

#### Monitoring your rate limits in the Console

You can monitor your rate limit usage on the [Usage](https://platform.claude.com/usage) page of the [Claude Console](/).

In addition to providing token and request charts, the Usage page provides two separate rate limit charts. Use these charts to see what headroom you have to grow, when you may be hitting peak use, better understand what rate limits to request, or how you can improve your caching rates. The charts visualize a number of metrics for a given rate limit (e.g. per model):

- The **Rate Limit - Input Tokens** chart includes:
  - Hourly maximum uncached input tokens per minute
  - Your current input tokens per minute rate limit
  - The cache rate for your input tokens (i.e. the percentage of input tokens read from the cache)
- The **Rate Limit - Output Tokens** chart includes:
  - Hourly maximum output tokens per minute
  - Your current output tokens per minute rate limit

### Setting lower limits for Workspaces

For more about workspaces, see [Workspaces](https://platform.claude.com/docs/en/build-with-claude/workspaces).

In order to protect Workspaces in your Organization from potential overuse, you can set custom spend and rate limits per Workspace.

Example: If your Organization's limit is 40,000 input tokens per minute and 8,000 output tokens per minute, you might limit one Workspace to 30,000 total tokens per minute. This protects other Workspaces from potential overuse and ensures a more equitable distribution of resources across your Organization. The remaining unused tokens per minute (or more, if that Workspace doesn't use the limit) are then available for other Workspaces to use.

Note:
- You can't set limits on the default Workspace.
- If not set, Workspace limits match the Organization's limit.
- Organization-wide limits always apply, even if Workspace limits add up to more.
- Support for input and output token limits will be added to Workspaces in the future.

### Response headers

The API response includes headers that show you the rate limit enforced, current usage, and when the limit will be reset.

The following headers are returned:

| Header                                        | Description                                                                                                                                     |
| --------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `retry-after`                                 | The number of seconds to wait until you can retry the request. Earlier retries will fail.                                                      |
| `anthropic-ratelimit-requests-limit`          | The maximum number of requests allowed within any rate limit period.                                                                            |
| `anthropic-ratelimit-requests-remaining`      | The number of requests remaining before being rate limited.                                                                                     |
| `anthropic-ratelimit-requests-reset`          | The time when the request rate limit will be fully replenished, provided in RFC 3339 format.                                                    |
| `anthropic-ratelimit-tokens-limit`            | The maximum number of tokens allowed within any rate limit period.                                                                              |
| `anthropic-ratelimit-tokens-remaining`        | The number of tokens remaining (rounded to the nearest thousand) before being rate limited.                                                     |
| `anthropic-ratelimit-tokens-reset`            | The time when the token rate limit will be fully replenished, provided in RFC 3339 format.                                                      |
| `anthropic-ratelimit-input-tokens-limit`      | The maximum number of input tokens allowed within any rate limit period.                                                                        |
| `anthropic-ratelimit-input-tokens-remaining`  | The number of input tokens remaining (rounded to the nearest thousand) before being rate limited.                                               |
| `anthropic-ratelimit-input-tokens-reset`      | The time when the input token rate limit will be fully replenished, provided in RFC 3339 format.                                                |
| `anthropic-ratelimit-output-tokens-limit`     | The maximum number of output tokens allowed within any rate limit period.                                                                       |
| `anthropic-ratelimit-output-tokens-remaining` | The number of output tokens remaining (rounded to the nearest thousand) before being rate limited.                                              |
| `anthropic-ratelimit-output-tokens-reset`     | The time when the output token rate limit will be fully replenished, provided in RFC 3339 format.                                               |
| `anthropic-priority-input-tokens-limit`       | The maximum number of Priority Tier input tokens allowed within any rate limit period. (Priority Tier only)                                     |
| `anthropic-priority-input-tokens-remaining`   | The number of Priority Tier input tokens remaining (rounded to the nearest thousand) before being rate limited. (Priority Tier only)            |
| `anthropic-priority-input-tokens-reset`       | The time when the Priority Tier input token rate limit will be fully replenished, provided in RFC 3339 format. (Priority Tier only)             |
| `anthropic-priority-output-tokens-limit`      | The maximum number of Priority Tier output tokens allowed within any rate limit period. (Priority Tier only)                                    |
| `anthropic-priority-output-tokens-remaining`  | The number of Priority Tier output tokens remaining (rounded to the nearest thousand) before being rate limited. (Priority Tier only)           |
| `anthropic-priority-output-tokens-reset`      | The time when the Priority Tier output token rate limit will be fully replenished, provided in RFC 3339 format. (Priority Tier only)            |

The `anthropic-ratelimit-tokens-*` headers display the values for the most restrictive limit currently in effect. For instance, if you have exceeded the Workspace per-minute token limit, the headers will contain the Workspace per-minute token rate limit values. If Workspace limits do not apply, the headers will return the total tokens remaining, where total is the sum of input and output tokens. This approach ensures that you have visibility into the most relevant constraint on your current API usage.

---

## Service tiers

Different tiers of service allow you to balance availability, performance, and predictable costs based on your application's needs.

---

Anthropic offers three service tiers:
- **Priority Tier:** Best for workflows deployed in production where time, availability, and predictable pricing are important
- **Standard:** Default tier for both piloting and scaling everyday use cases
- **Batch:** Best for asynchronous workflows which can wait or benefit from being outside your normal capacity

### Standard Tier

The standard tier is the default service tier for all API requests. The API prioritizes these requests alongside all other requests with best-effort availability.

### Priority Tier

The API prioritizes requests in this tier over all other requests. This prioritization helps minimize ["server overloaded" errors](https://platform.claude.com/docs/en/api/errors#http-errors), even during peak times.

For more information, see [Get started with Priority Tier](#get-started-with-priority-tier)

### How requests get assigned tiers

When handling a request, Anthropic decides to assign a request to Priority Tier in the following scenarios:
- Your organization has sufficient priority tier capacity **input** tokens per minute
- Your organization has sufficient priority tier capacity **output** tokens per minute

Anthropic counts usage against Priority Tier capacity as follows:

**Input Tokens**
- Cache reads as 0.1 tokens per token read from the cache
- Cache writes as 1.25 tokens per token written to the cache with a 5 minute TTL
- Cache writes as 2.00 tokens per token written to the cache with a 1 hour TTL
- For [US-only inference](https://platform.claude.com/docs/en/build-with-claude/data-residency) (`inference_geo: "us"`) requests on Claude Opus 4.7, Claude Opus 4.6, and newer models, input tokens are 1.1 tokens per token
- All other input tokens are 1 token per token

**Output Tokens**
- For [US-only inference](https://platform.claude.com/docs/en/build-with-claude/data-residency) (`inference_geo: "us"`) requests on Claude Opus 4.7, Claude Opus 4.6, and newer models, output tokens are 1.1 tokens per token
- All other output tokens are 1 token per token

Otherwise, requests proceed at standard tier.

> These burndown rates reflect the relative pricing of each token type. For example, US-only inference is priced at 1.1x on Opus 4.7, Opus 4.6, and newer models, so each token consumed with `inference_geo: "us"` draws down 1.1 tokens from your Priority Tier capacity.

> Requests assigned Priority Tier pull from both the Priority Tier capacity and the regular rate limits.
> If servicing the request would exceed the rate limits, the request is declined.

### Using service tiers

You can control which service tiers can be used for a request by setting the `service_tier` parameter:

```python Python
message = client.messages.create(
    model="claude-opus-4-7",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello, Claude!"}],
    service_tier="auto",  # Automatically use Priority Tier when available, fallback to standard
)
```

The `service_tier` parameter accepts the following values:

- `"auto"` (default) - Uses the Priority Tier capacity if available, falling back to your other capacity if not
- `"standard_only"` - Only use standard tier capacity, useful if you don't want to use your Priority Tier capacity

The response `usage` object also includes the service tier assigned to the request:

```json
{
  "usage": {
    "input_tokens": 410,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0,
    "output_tokens": 585,
    "service_tier": "priority"
  }
}
```
This allows you to determine which service tier was assigned to the request.

When requesting `service_tier="auto"` with a model with a Priority Tier commitment, these response headers provide insights:
```text
anthropic-priority-input-tokens-limit: 10000
anthropic-priority-input-tokens-remaining: 9618
anthropic-priority-input-tokens-reset: 2025-01-12T23:11:59Z
anthropic-priority-output-tokens-limit: 10000
anthropic-priority-output-tokens-remaining: 6000
anthropic-priority-output-tokens-reset: 2025-01-12T23:12:21Z
```
You can use the presence of these headers to detect if your request was eligible for Priority Tier, even if it was over the limit.

### Get started with Priority Tier

You may want to commit to Priority Tier capacity if you are interested in:
- **Higher availability**: Target 99.5% uptime with prioritized computational resources
- **Cost Control**: Predictable spend and discounts for longer commitments
- **Flexible overflow**: Automatically falls back to standard tier when you exceed your committed capacity

Committing to Priority Tier will involve deciding:
- A number of input tokens per minute
- A number of output tokens per minute
- A commitment duration (1, 3, 6, or 12 months)
- A specific model version

> The ratio of input to output tokens you purchase matters. Sizing your Priority Tier capacity to align with your actual traffic patterns helps you maximize utilization of your purchased tokens.

#### Supported models

Priority Tier is supported on all available Claude models (including Claude Opus 4.7) except [Claude Mythos Preview](https://anthropic.com/glasswing).

Check the [model overview page](https://platform.claude.com/docs/en/about-claude/models/overview) for more details on available models.

#### How to access Priority Tier

To begin using Priority Tier:

1. [Contact sales](https://claude.com/contact-sales/priority-tier) to complete provisioning
2. (Optional) Update your API requests to optionally set the `service_tier` parameter to `auto`
3. Monitor your usage through response headers and the Claude Console

---

## IP addresses

Anthropic services use fixed IP addresses for both inbound and outbound connections. You can use these addresses to configure your firewall rules for secure access to the Claude API and Console. These addresses will not change without notice.

---

### Inbound IP addresses

These are the IP addresses where Anthropic services receive incoming connections.

#### IPv4

`160.79.104.0/23`

#### IPv6

`2607:6bc0::/48`

### Outbound IP addresses

These are the stable IP addresses that Anthropic uses for outbound requests (for example, when making MCP tool calls to external servers).

#### IPv4

`160.79.104.0/21`

#### Phased out IP addresses

The following IP addresses are no longer in use by Anthropic. If you have previously allowlisted these addresses, you should remove them from your firewall rules.

```text
34.162.46.92/32
34.162.102.82/32
34.162.136.91/32
34.162.142.92/32
34.162.183.95/32
```

---

## Supported regions

Here are the countries, regions, and territories we can currently support access from:

---

* Albania
* Algeria
* Andorra
* Angola
* Antigua and Barbuda
* Argentina
* Armenia
* Australia
* Austria
* Azerbaijan
* Bahamas
* Bahrain
* Bangladesh
* Barbados
* Belgium
* Belize
* Benin
* Bhutan
* Bolivia
* Bosnia and Herzegovina
* Botswana
* Brazil
* Brunei
* Bulgaria
* Burkina Faso
* Burundi
* Cabo Verde
* Cambodia
* Cameroon
* Canada
* Chad
* Chile
* Colombia
* Comoros
* Congo, Republic of the
* Costa Rica
* Côte d'Ivoire
* Croatia
* Cyprus
* Czechia (Czech Republic)
* Denmark
* Djibouti
* Dominica
* Dominican Republic
* Ecuador
* Egypt
* El Salvador
* Equatorial Guinea
* Estonia
* Eswatini
* Fiji
* Finland
* France
* Gabon
* Gambia
* Georgia
* Germany
* Ghana
* Greece
* Grenada
* Guatemala
* Guinea
* Guinea-Bissau
* Guyana
* Haiti
* Holy See (Vatican City)
* Honduras
* Hungary
* Iceland
* India
* Indonesia
* Iraq
* Ireland
* Israel
* Italy
* Jamaica
* Japan
* Jordan
* Kazakhstan
* Kenya
* Kiribati
* Kuwait
* Kyrgyzstan
* Laos
* Latvia
* Lebanon
* Lesotho
* Liberia
* Liechtenstein
* Lithuania
* Luxembourg
* Madagascar
* Malawi
* Malaysia
* Maldives
* Malta
* Marshall Islands
* Mauritania
* Mauritius
* Mexico
* Micronesia
* Moldova
* Monaco
* Mongolia
* Montenegro
* Morocco
* Mozambique
* Namibia
* Nauru
* Nepal
* Netherlands
* New Zealand
* Niger
* Nigeria
* North Macedonia
* Norway
* Oman
* Pakistan
* Palau
* Palestine
* Panama
* Papua New Guinea
* Paraguay
* Peru
* Philippines
* Poland
* Portugal
* Qatar
* Romania
* Rwanda
* Saint Kitts and Nevis
* Saint Lucia
* Saint Vincent and the Grenadines
* Samoa
* San Marino
* Sao Tome and Principe
* Saudi Arabia
* Senegal
* Serbia
* Seychelles
* Sierra Leone
* Singapore
* Slovakia
* Slovenia
* Solomon Islands
* South Africa
* South Korea
* Spain
* Sri Lanka
* Suriname
* Sweden
* Switzerland
* Taiwan
* Tajikistan
* Tanzania
* Thailand
* Timor-Leste, Democratic Republic of
* Togo
* Tonga
* Trinidad and Tobago
* Tunisia
* Turkey
* Turkmenistan
* Tuvalu
* Uganda
* Ukraine (except Crimea, Donetsk, and Luhansk regions)
* United Arab Emirates
* United Kingdom
* United States of America
* Uruguay
* Uzbekistan
* Vanuatu
* Vietnam
* Zambia
* Zimbabwe
