# Claude API Client SDKs

> Sources:
> - https://platform.claude.com/docs/en/api/client-sdks
> - https://platform.claude.com/docs/en/api/sdks/cli
> - https://platform.claude.com/docs/en/api/sdks/python
> - https://platform.claude.com/docs/en/api/sdks/typescript
> - https://platform.claude.com/docs/en/api/sdks/java
> - https://platform.claude.com/docs/en/api/sdks/go
> - https://platform.claude.com/docs/en/api/sdks/ruby
> - https://platform.claude.com/docs/en/api/sdks/csharp
> - https://platform.claude.com/docs/en/api/sdks/php
> - https://platform.claude.com/docs/en/api/openai-sdk
> Fetched: 2026-04-23

## Client SDKs

Official SDKs for building with the Claude API in Python, TypeScript, Java, Go, Ruby, C#, PHP, and the command line.

---

Anthropic provides official client SDKs in multiple languages to make it easier to work with the Claude API. Each SDK provides idiomatic interfaces, type safety, and built-in support for features like streaming, retries, and error handling.

- [CLI](https://platform.claude.com/docs/en/api/sdks/cli): Shell scripting, typed flags, response transforms
  - [Python](https://platform.claude.com/docs/en/api/sdks/python): Sync and async clients, Pydantic models
  - [TypeScript](https://platform.claude.com/docs/en/api/sdks/typescript): Node.js, Deno, Bun, and browser support
  - [Java](https://platform.claude.com/docs/en/api/sdks/java): Builder pattern, CompletableFuture async
  - [Go](https://platform.claude.com/docs/en/api/sdks/go): Context-based cancellation, functional options
  - [Ruby](https://platform.claude.com/docs/en/api/sdks/ruby): Sorbet types, streaming helpers
  - [C#](https://platform.claude.com/docs/en/api/sdks/csharp): .NET Standard 2.0+, IChatClient integration
  - [PHP](https://platform.claude.com/docs/en/api/sdks/php): Value objects, builder pattern

### Quick installation

<Tabs>
<Tab title="CLI">
```bash
brew install anthropics/tap/ant
```
</Tab>
<Tab title="Python">
```bash
pip install anthropic
```
</Tab>
<Tab title="TypeScript">
```bash
npm install @anthropic-ai/sdk
```
</Tab>
<Tab title="C#">
```bash
dotnet add package Anthropic
```
</Tab>
<Tab title="Go">
```bash
go get github.com/anthropics/anthropic-sdk-go
```
</Tab>
<Tab title="Java">
<CodeGroup>
```groovy Gradle
implementation("com.anthropic:anthropic-java:2.20.0")
```

```xml Maven
<dependency>
    <groupId>com.anthropic</groupId>
    <artifactId>anthropic-java</artifactId>
    <version>2.20.0</version>
</dependency>
```
</CodeGroup>
</Tab>
<Tab title="PHP">
```bash
composer require anthropic-ai/sdk
```
</Tab>
<Tab title="Ruby">
```bash
bundler add anthropic
```
</Tab>
</Tabs>

### Quick start

<CodeGroup>
```bash CLI
ant messages create \
  --model claude-opus-4-7 \
  --max-tokens 1024 \
  --message '{role: user, content: "Hello, Claude"}' \
  --transform content
```

```python Python
import anthropic

client = anthropic.Anthropic()

message = client.messages.create(
    model="claude-opus-4-7",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello, Claude"}],
)
print(message.content)
```

```typescript TypeScript
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic();

const message = await client.messages.create({
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: [{ role: "user", content: "Hello, Claude" }]
});
console.log(message.content);
```

```csharp C# hidelines={2}
using Anthropic;
using Anthropic.Models.Messages;

var client = new AnthropicClient();

var message = await client.Messages.Create(new MessageCreateParams
{
    Model = "claude-opus-4-7",
    MaxTokens = 1024,
    Messages = [new() { Role = Role.User, Content = "Hello, Claude" }]
});
Console.WriteLine(message.Content);
```

```go Go hidelines={1..2,10..11,-1}
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/anthropics/anthropic-sdk-go"
)

func main() {
	client := anthropic.NewClient()

	message, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_7,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello, Claude")),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(message.Content)
}
```

```java Java hidelines={6..8,-2..}
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;
import com.anthropic.models.messages.Message;
import com.anthropic.models.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

public class Main {
    public static void main(String[] args) {
        AnthropicClient client = AnthropicOkHttpClient.fromEnv();

        MessageCreateParams params = MessageCreateParams.builder()
            .model(Model.CLAUDE_OPUS_4_7)
            .maxTokens(1024L)
            .addUserMessage("Hello, Claude")
            .build();

        Message message = client.messages().create(params);
        System.out.println(message.content());
    }
}
```

```php PHP hidelines={1}
<?php
use Anthropic\Client;

$client = new Client(apiKey: getenv('ANTHROPIC_API_KEY'));

$message = $client->messages->create(
    model: 'claude-opus-4-7',
    maxTokens: 1024,
    messages: [
        ['role' => 'user', 'content' => 'Hello, Claude']
    ],
);
echo $message->content[0]->text;
```

```ruby Ruby
client = Anthropic::Client.new

message = client.messages.create(
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: [
    { role: "user", content: "Hello, Claude" }
  ]
)
puts message.content
```
</CodeGroup>

### Platform support

All SDKs support multiple deployment options:

| Platform | Description |
|----------|-------------|
| Claude API | Connect directly to Claude API endpoints |
| [Amazon Bedrock](https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock) | Use Claude through AWS |
| [Google Vertex AI](https://platform.claude.com/docs/en/build-with-claude/claude-on-vertex-ai) | Use Claude through Google Cloud |
| [Microsoft Foundry](https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry) | Use Claude through Microsoft Azure |

See individual SDK pages for platform-specific setup instructions.

### Beta features

Access beta features using the `beta` namespace in any SDK:

<CodeGroup>

```bash CLI nocheck
ant beta:messages create \
  --model claude-opus-4-7 \
  --max-tokens 1024 \
  --message '{role: user, content: "Hello"}' \
  --beta feature-name
```

```python Python nocheck
message = client.beta.messages.create(
    model="claude-opus-4-7",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello"}],
    betas=["feature-name"],
)
```

```typescript TypeScript nocheck
const message = await client.beta.messages.create({
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: [{ role: "user", content: "Hello" }],
  betas: ["feature-name"]
});
```

```csharp C# nocheck
var message = await client.Beta.Messages.Create(new MessageCreateParams
{
    Model = "claude-opus-4-7",
    MaxTokens = 1024,
    Messages = [new() { Role = Role.User, Content = "Hello" }],
    Betas = ["feature-name"],
});
```

```go Go nocheck hidelines={9}
message, _ := client.Beta.Messages.New(context.Background(), anthropic.BetaMessageNewParams{
	Model:     anthropic.ModelClaudeOpus4_7,
	MaxTokens: 1024,
	Messages: []anthropic.BetaMessageParam{
		anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock("Hello")),
	},
	Betas: []anthropic.AnthropicBeta{anthropic.AnthropicBeta("feature-name")},
})
fmt.Println(message)
```

```java Java nocheck
import com.anthropic.models.beta.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

MessageCreateParams params = MessageCreateParams.builder()
    .model(Model.CLAUDE_OPUS_4_7)
    .maxTokens(1024L)
    .addUserMessage("Hello")
    .addBeta("feature-name")
    .build();

client.beta().messages().create(params);
```

```php PHP nocheck
$message = $client->beta->messages->create(
    model: 'claude-opus-4-7',
    maxTokens: 1024,
    messages: [['role' => 'user', 'content' => 'Hello']],
    betas: ['feature-name'],
);
```

```ruby Ruby nocheck
message = client.beta.messages.create(
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: [{ role: "user", content: "Hello" }],
  betas: ["feature-name"]
)
```
</CodeGroup>

See [Beta headers](https://platform.claude.com/docs/en/api/beta-headers) for available beta features.

### Requirements

| SDK | Minimum Version |
|-----|-----------------|
| Python | 3.9+ |
| TypeScript | 4.9+ (Node.js 20+) |
| Java | 8+ |
| Go | 1.23+ |
| Ruby | 3.2.0+ |
| C# | .NET Standard 2.0 |
| PHP | 8.1.0+ |

### GitHub repositories

- [anthropic-sdk-python](https://github.com/anthropics/anthropic-sdk-python)
- [anthropic-sdk-typescript](https://github.com/anthropics/anthropic-sdk-typescript)
- [anthropic-sdk-java](https://github.com/anthropics/anthropic-sdk-java)
- [anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go)
- [anthropic-sdk-ruby](https://github.com/anthropics/anthropic-sdk-ruby)
- [anthropic-sdk-csharp](https://github.com/anthropics/anthropic-sdk-csharp)
- [anthropic-sdk-php](https://github.com/anthropics/anthropic-sdk-php)

---

## CLI

Interact with the Claude API directly from your terminal with the ant command-line tool

---

The `ant` CLI provides access to the Claude API from your terminal. Every API resource is exposed as a subcommand, with output formatting, response filtering, and support for YAML or JSON file input that make it practical for both interactive exploration and automation.

Compared to calling the API with `curl`, `ant` lets you build request bodies from typed flags or piped YAML rather than hand-written JSON, inline file contents into string fields with an `@path` reference, and extract fields from the response with a built-in `--transform` query — no separate JSON tooling required. List endpoints paginate automatically. Claude Code understands how to use `ant` natively.

> For endpoint-specific parameters and response schemas, see the [API reference](https://platform.claude.com/docs/en/api/cli/messages/create). This page covers CLI-specific features and workflows that apply across all endpoints.

### Installation

<Tabs>
<Tab title="Homebrew (macOS)">

```bash
brew install anthropics/tap/ant
```

On macOS, unquarantine the binary:

```bash
xattr -d com.apple.quarantine "$(brew --prefix)/bin/ant"
```

</Tab>
<Tab title="curl (Linux/WSL)">

For Linux environments, download the release binary directly.

```bash
VERSION=1.0.0
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
curl -fsSL "https://github.com/anthropics/anthropic-cli/releases/download/v${VERSION}/ant_${VERSION}_${OS}_${ARCH}.tar.gz" \
  | sudo tar -xz -C /usr/local/bin ant
```

You can find all releases on the [GitHub releases page](https://github.com/anthropics/anthropic-cli/releases).

</Tab>
<Tab title="Go">

You may also install the CLI from source using `go install`. Requires Go 1.22 or later.

```bash
go install github.com/anthropics/anthropic-cli/cmd/ant@latest
```

The binary is placed in `$(go env GOPATH)/bin`. Add it to your `PATH` if it isn't already:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

</Tab>
</Tabs>

Check the installation:

```bash
ant --version
```

### Authentication

The CLI reads your API key from the `ANTHROPIC_API_KEY` environment variable.

<Tabs>
<Tab title="zsh">

```bash
echo 'export ANTHROPIC_API_KEY=sk-ant-api03-...' >> ~/.zshrc
source ~/.zshrc
```

</Tab>
<Tab title="bash">

```bash
echo 'export ANTHROPIC_API_KEY=sk-ant-api03-...' >> ~/.bashrc
source ~/.bashrc
```

</Tab>
<Tab title="Windows">

```powershell
setx ANTHROPIC_API_KEY "sk-ant-api03-..."
```

Open a new terminal for the change to take effect.

</Tab>
</Tabs>

Get a key from the [Claude Console](https://platform.claude.com/settings/keys). To point at a different API host, set `ANTHROPIC_BASE_URL` or pass `--base-url` on any command.

### Send your first request

With the binary installed and `ANTHROPIC_API_KEY` set, call the [Messages API](https://platform.claude.com/docs/en/api/cli/messages/create):

```bash
ant messages create \
  --model claude-opus-4-7 \
  --max-tokens 1024 \
  --message '{role: user, content: "Hello, Claude"}'
```

```json Output
{
  "model": "claude-opus-4-7",
  "id": "msg_01YMmR5XodC5nTqMxLZMKaq6",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How are you doing today? Is there something I can help you with?"
    }
  ],
  "stop_reason": "end_turn",
  "usage": { "input_tokens": 27, "output_tokens": 20 /*, ... */ }
}
```

The response is the full API object, pretty-printed because stdout is a terminal. The rest of this page covers how to reshape that output, pass complex request bodies, and chain commands together.

### Command structure

Commands follow a `resource action` pattern. Nested resources use colons:

```text
ant <resource>[:<subresource>] <action> [flags]
```

Run `ant --help` for the full resource list, or append `--help` to any subcommand for its flags.

Resources currently in beta — including agents, sessions, deployments, environments, and skills — live under the `beta:` prefix. Commands in this namespace automatically send the appropriate `anthropic-beta` header for that resource, so you don't need to pass it yourself. Use `--beta <header>` only to override the default — for example, to opt into a different schema version.

```bash
ant models list
ant messages create --model claude-opus-4-7 --max-tokens 1024 ...
ant beta:agents retrieve --agent-id agent_01...
ant beta:sessions:events list --session-id session_01...
```

#### Global flags

| Flag | Description |
| --- | --- |
| `--format` | Output format: `auto`, `json`, `jsonl`, `yaml`, `pretty`, `raw`, `explore` |
| `--transform` | Filter or reshape the response with a [GJSON path](#transform-output-with-gjson) |
| `--base-url` | Override the API base URL |
| `--debug` | Print full HTTP request and response to stderr |
| `--format-error`, `--transform-error` | Same as `--format` and `--transform` but applied to [error responses](#inspect-errors) |

### Output formats

The default `auto` format pretty-prints JSON when writing to a terminal and emits compact JSON when piped. Override it with `--format`:

```bash
ant models retrieve --model-id claude-opus-4-7 --format yaml
```

```yaml Output
type: model
id: claude-opus-4-7
display_name: Claude Opus 4.7
created_at: "2026-02-04T00:00:00Z"
...
```

List endpoints auto-paginate. In the default formats each item is written separately (one compact JSON object per line in `jsonl` mode, a stream of YAML documents in `yaml` mode), which streams cleanly into `head`, `grep`, and `--transform` filters.

#### Interactive explorer

When connected to a terminal, `--format explore` opens a fold-and-search TUI for browsing large responses. Arrow keys expand and collapse nodes, `/` searches, `q` exits.

```bash
ant models list --format explore
```

### Transform output with GJSON

Use `--transform` to reshape responses before printing. The expression is a [GJSON path](https://github.com/tidwall/gjson/blob/master/SYNTAX.md). For list endpoints the transform runs against each item individually, not the envelope:

```bash
ant beta:agents list \
  --transform "{id,name,model}" \
  --format jsonl
```

```jsonl Output
{"id": "agent_011CYm1BLqPX...", "name": "Docs CLI Test Agent", "model": "claude-sonnet-4-6"}
{"id": "agent_011CYkVwfaEt...", "name": "Coffee Making Assistant", "model": "claude-sonnet-4-6"}
{"id": "agent_011CYixHhtUP...", "name": "Coding Assistant", "model": "claude-opus-4-5"}
```

#### Extract a scalar

To capture a single field as an unquoted string — for example, the ID of a newly created resource — pair `--transform` with `--format yaml`. YAML emits scalar values without quotes, so the result is ready to assign to a shell variable:

```bash
AGENT_ID=$(ant beta:agents create \
  --name "My Agent" \
  --model '{id: claude-sonnet-4-6}' \
  --transform id --format yaml)

printf '%s\n' "$AGENT_ID"
```

```text Output
agent_011CYm1BLqPXpQRk5khsSXrs
```

> `--transform` is not applied when `--format raw` is set. Use `--format yaml` for unquoted scalars, or `--format jsonl` to keep the result as structured data for further processing.

### Passing request bodies

The right input mechanism depends on the shape of the data: use **flags** for scalar fields and short structured values, pipe a **stdin** document for nested or multi-line bodies, and use **`@file` references** to pull file contents into any string or binary field.

#### Flags

Scalar fields map directly to flags. Structured fields accept a relaxed YAML-like syntax (unquoted keys, optional quotes around strings) or strict JSON:

```bash
ant beta:sessions create \
  --agent '{type: agent, id: agent_011CYm1BLqPXpQRk5khsSXrs, version: 1}' \
  --environment-id env_01595EKxaaTTGwwY3kyXdtbs \
  --title "CLI docs test session"
```

Repeatable flags build arrays. Each `--tool` or `--event` appends one element:

```bash
ant beta:agents create \
  --name "Research Agent" \
  --model '{id: claude-opus-4-7}' \
  --tool '{type: agent_toolset_20260401}' \
  --tool '{type: custom, name: search_docs, input_schema: {type: object, properties: {query: {type: string}}}}'
```

#### Stdin

Pipe a JSON or YAML document to stdin to supply the full request body. Fields from stdin are merged with flags, with flags taking precedence. Here `version` is the optimistic-locking token returned by an earlier `retrieve`, and `$AGENT_ID` was captured as in [Extract a scalar](#extract-a-scalar):

```bash
echo '{"description": "Updated test agent.", "version": 1}' | \
  ant beta:agents update --agent-id "$AGENT_ID"
```

Heredocs work the same way and are convenient for multi-line YAML. Quote the delimiter (as in `<<'YAML'`) to disable variable expansion inside the body.

```bash
ant beta:agents create <<'YAML'
name: Research Agent
model: claude-opus-4-7
system: |
  You are a research assistant. Cite sources for every claim.
tools:
  - type: agent_toolset_20260401
YAML
```

#### File references

Flags that take a file path, such as `--file` on the upload command, accept a bare path:

```bash
ant beta:files upload --file ./report.pdf
```

To inline a file's contents into a string-valued field, prefix the path with `@`:

```bash
ant beta:agents create \
  --name "Researcher" --model '{id: claude-sonnet-4-6}' \
  --system @./prompts/researcher.txt
```

Inside structured flag values, wrap the path in quotes. To send a PDF to the Messages API:

```bash
ant messages create \
  --model claude-opus-4-7 \
  --max-tokens 1024 \
  --message '{role: user, content: [
    {type: document, source: {type: base64, media_type: application/pdf, data: "@./scan.pdf"}},
    {type: text, text: "Extract the text from this scanned document."}
  ]}' \
  --transform 'content.0.text' --format yaml
```

The CLI detects the file type and encodes binary files as base64 automatically. To force a specific encoding use `@file://` for plain text or `@data://` for base64. Escape a literal leading `@` with a backslash (`\@username`).

### Version-controlling API resources

You can use the CLI to version control API resources such as skills, agents, environments, or deployments as YAML files in your repository and keep them in sync with the Claude API.

> For more information on these resources, see [Managed Agents](https://platform.claude.com/docs/en/managed-agents/overview).

<Steps>
<Step title="Define your agent">

Write the agent definition to `summarizer.agent.yaml`:

```yaml summarizer.agent.yaml
name: Summarizer
model: claude-sonnet-4-6
system: |
  You are a helpful assistant that writes concise summaries.
tools:
  - type: agent_toolset_20260401
```

</Step>
<Step title="Create the agent">

```bash
ant beta:agents create < summarizer.agent.yaml
```

```json Output
{
  "id": "agent_011CYm1BLqPXpQRk5khsSXrs",
  "version": 1,
  "name": "Summarizer",
  "model": "claude-sonnet-4-6"
  /* ... */
}
```

Note the `id` from the response — you'll pass it to the session create command below.

> Check `summarizer.agent.yaml` into your repository and keep it in sync with the API in your CI pipeline. The update command needs the agent ID and current version as flags:
>
> ```bash CLI nocheck
> ant beta:agents update --agent-id agent_011CYm1BLqPXpQRk5khsSXrs --version 1 < summarizer.agent.yaml
> ```

</Step>
<Step title="Define the environment">

A session runs in an [environment](https://platform.claude.com/docs/en/api/cli/beta/environments), which defines the container it executes in. Write the environment definition to `summarizer.environment.yaml`:

```yaml summarizer.environment.yaml
name: summarizer-env
config:
  type: cloud
  networking:
    type: unrestricted
```

</Step>
<Step title="Create the environment">

```bash
ant beta:environments create < summarizer.environment.yaml
```

```json Output
{
  "id": "env_01595EKxaaTTGwwY3kyXdtbs",
  "name": "summarizer-env"
  /* ... */
}
```

Note the `id` from the response — you'll pass it to the session create command below.

> Check `summarizer.environment.yaml` into your repository and keep it in sync with the API in your CI pipeline. The update command needs the environment ID as a flag:
>
> ```bash CLI nocheck
> ant beta:environments update --environment-id env_01595EKxaaTTGwwY3kyXdtbs < summarizer.environment.yaml
> ```

</Step>
<Step title="Start a session">

Paste the agent `id` and environment `id` from the previous outputs into the session create command:

```bash highlight={2..3}
ant beta:sessions create \
  --agent agent_011CYm1BLqPXpQRk5khsSXrs \
  --environment-id env_01595EKxaaTTGwwY3kyXdtbs \
  --title "Summarization task"
```

```json Output
{
  "id": "session_01JZCh78XvmxJjiXVy3oSi7K",
  "status": "running"
  /* ... */
}
```

</Step>
<Step title="Send a user message">

Copy the session `id` from the previous output into `--session-id`:

```bash highlight={2}
ant beta:sessions:events send \
  --session-id session_01JZCh78XvmxJjiXVy3oSi7K \
  --event '{type: user.message, content: [{type: text, text: "Summarize the benefits of type safety in one sentence."}]}'
```

</Step>
<Step title="Read the conversation">

`--transform` runs against each listed event, so this prints the text of every message in order:

```bash highlight={2}
ant beta:sessions:events list \
  --session-id session_01JZCh78XvmxJjiXVy3oSi7K \
  --transform 'content.0.text' --format yaml
```

```text Output
Summarize the benefits of type safety in one sentence.
Type safety catches errors at compile time rather than runtime, reducing bugs, improving code clarity, enabling better tooling support, and making codebases easier to maintain and refactor with confidence.
```

> To watch a session as it runs, use `ant beta:sessions stream --session-id session_01JZCh78XvmxJjiXVy3oSi7K`. Events are written to stdout as they arrive.

</Step>
</Steps>

### Scripting patterns

The CLI is designed to compose with standard shell tooling.

#### Chain list output into a second command

`--transform id --format yaml` on a list endpoint emits one bare ID per line, so standard tools such as `head` and `xargs` apply directly. Capture the first result, then pass it to a follow-up command:

```bash
FIRST_AGENT=$(ant beta:agents list \
  --transform id --format yaml | head -1)

ant beta:agents:versions list \
  --agent-id "$FIRST_AGENT" \
  --transform "{version,created_at}" --format jsonl
```

#### Inspect errors

The `--transform-error` and `--format-error` flags mirror their success-path counterparts and follow the same rule — pair with `yaml`, not `raw`, to apply the transform. Extract only the error message:

```bash
ant beta:agents retrieve --agent-id bogus \
  --transform-error error.message --format-error yaml 2>&1
```

```text Output
GET "https://api.anthropic.com/v1/agents/bogus?beta=true": 404 Not Found
Agent not found.
```

### Use the CLI from Claude Code

[Claude Code](https://docs.claude.com/en/docs/claude-code/overview) knows out of the box how to use the `ant` CLI. With the CLI installed and `ANTHROPIC_API_KEY` set, you can ask Claude Code to operate on your API resources directly — for example:

- "List my recent agent sessions and summarize which ones errored."
- "Upload every PDF in `./reports` to the Files API and print the resulting IDs."
- "Pull the events for session `session_01...` and tell me where the agent got stuck."

Claude Code shells out to `ant`, parses the structured output, and reasons over the results — no custom integration code required.

### Debugging

Add `--debug` to any command to print the exact HTTP request and response (headers and body) to stderr. API keys are redacted.

```bash
ant --debug beta:agents list
```

```text Output
GET /v1/agents?beta=true HTTP/1.1
Host: api.anthropic.com
Anthropic-Beta: managed-agents-2026-04-01
Anthropic-Version: 2023-06-01
X-Api-Key: <REDACTED>
...
```

### Shell completion

The CLI ships completion scripts for bash, zsh, fish, and PowerShell. Generate and install one for your shell:

<Tabs>
<Tab title="zsh">

```bash
ant @completion zsh > "${fpath[1]}/_ant"
## Restart your shell or run: autoload -U compinit && compinit
```

</Tab>
<Tab title="bash">

```bash
ant @completion bash > /etc/bash_completion.d/ant
```

</Tab>
<Tab title="fish">

```bash
ant @completion fish > ~/.config/fish/completions/ant.fish
```

</Tab>
<Tab title="PowerShell">

```powershell
ant @completion powershell | Out-String | Invoke-Expression
## To persist across sessions:
## ant @completion powershell >> $PROFILE
```

</Tab>
</Tabs>

### Available resources

Every API resource the CLI exposes is documented in the [API reference](https://platform.claude.com/docs/en/api/cli/messages/create). For a local listing, run `ant --help`, and append `--help` to any subcommand for its flags and parameters.

---

## Python SDK

Install and configure the Anthropic Python SDK with sync and async client support

---

The Anthropic Python SDK provides convenient access to the Anthropic REST API from Python applications. It supports both synchronous and asynchronous operations, streaming, and integrations with AWS Bedrock and Google Vertex AI.

> For API feature documentation with code examples, see the [API reference](https://platform.claude.com/docs/en/api/overview). This page covers Python-specific SDK features and configuration.

### Installation

```bash
pip install anthropic
```

For platform-specific integrations, install with extras:

```bash
## For AWS Bedrock support
pip install anthropic[bedrock]

## For Google Vertex AI support
pip install anthropic[vertex]

## For improved async performance with aiohttp
pip install anthropic[aiohttp]
```

### Requirements

Python 3.9 or later is required.

### Usage

```python
import os
from anthropic import Anthropic

client = Anthropic(
    # This is the default and can be omitted
    api_key=os.environ.get("ANTHROPIC_API_KEY"),
)

message = client.messages.create(
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": "Hello, Claude",
        }
    ],
    model="claude-opus-4-7",
)
print(message.content)
```

> Consider using [python-dotenv](https://pypi.org/project/python-dotenv/) to add `ANTHROPIC_API_KEY="my-anthropic-api-key"` to your `.env` file so that your API key isn't stored in source control.

### Async usage

```python
import os
import asyncio
from anthropic import AsyncAnthropic

client = AsyncAnthropic(
    api_key=os.environ.get("ANTHROPIC_API_KEY"),
)

async def main() -> None:
    message = await client.messages.create(
        max_tokens=1024,
        messages=[
            {
                "role": "user",
                "content": "Hello, Claude",
            }
        ],
        model="claude-opus-4-7",
    )
    print(message.content)

asyncio.run(main())
```

#### Using aiohttp for better concurrency

For improved async performance, you can use the `aiohttp` HTTP backend instead of the default `httpx`:

```python nocheck
import os
import asyncio
from anthropic import AsyncAnthropic, DefaultAioHttpClient

async def main() -> None:
    async with AsyncAnthropic(
        api_key=os.environ.get("ANTHROPIC_API_KEY"),
        http_client=DefaultAioHttpClient(),
    ) as client:
        message = await client.messages.create(
            max_tokens=1024,
            messages=[
                {
                    "role": "user",
                    "content": "Hello, Claude",
                }
            ],
            model="claude-opus-4-7",
        )
        print(message.content)

asyncio.run(main())
```

### Streaming responses

The SDK provides support for streaming responses using Server-Sent Events (SSE).

```python hidelines={1..2}
from anthropic import Anthropic

client = Anthropic()

stream = client.messages.create(
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": "Hello, Claude",
        }
    ],
    model="claude-opus-4-7",
    stream=True,
)
for event in stream:
    print(event.type)
```

The async client uses the exact same interface:

```python hidelines={1..2}
from anthropic import AsyncAnthropic

client = AsyncAnthropic()

stream = await client.messages.create(
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": "Hello, Claude",
        }
    ],
    model="claude-opus-4-7",
    stream=True,
)
async for event in stream:
    print(event.type)
```

#### Streaming helpers

The SDK also provides streaming helpers that use context managers and provide access to the accumulated text and the final message:

```python hidelines={1..6}
import asyncio
from anthropic import AsyncAnthropic

client = AsyncAnthropic()

async def main() -> None:
    async with client.messages.stream(
        max_tokens=1024,
        messages=[
            {
                "role": "user",
                "content": "Say hello there!",
            }
        ],
        model="claude-opus-4-7",
    ) as stream:
        async for text in stream.text_stream:
            print(text, end="", flush=True)
        print()

        message = await stream.get_final_message()
        print(message.to_json())

asyncio.run(main())
```

Streaming with `client.messages.stream(...)` exposes various helpers including accumulation and SDK-specific events.

Alternatively, you can use `client.messages.create(..., stream=True)` which only returns an async iterable of the events in the stream and uses less memory (it doesn't build up a final message object for you).

### Token counting

You can see the exact usage for a given request through the `usage` response property:

```python nocheck
message = client.messages.create(...)
print(message.usage)
## Usage(input_tokens=25, output_tokens=13)
```

You can also count tokens before making a request:

```python
count = client.messages.count_tokens(
    model="claude-opus-4-7", messages=[{"role": "user", "content": "Hello, world"}]
)
print(count.input_tokens)  # 10
```

### Tool use

This SDK provides support for tool use, also known as function calling. More details can be found in the [tool use overview](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview).

#### Tool helpers

The SDK provides helpers for defining and running tools as pure Python functions. You can use the `@beta_tool` decorator for more control:

```python
import json
from anthropic import Anthropic, beta_tool

client = Anthropic()

@beta_tool
def get_weather(location: str) -> str:
    """Get the weather for a given location.

    Args:
        location: The city and state, e.g. San Francisco, CA
    Returns:
        A dictionary containing the location, temperature, and weather condition.
    """
    return json.dumps(
        {
            "location": location,
            "temperature": "68°F",
            "condition": "Sunny",
        }
    )

## Use the tool_runner to automatically handle tool calls
runner = client.beta.messages.tool_runner(
    max_tokens=1024,
    model="claude-opus-4-7",
    tools=[get_weather],
    messages=[
        {"role": "user", "content": "What is the weather in SF?"},
    ],
)
for message in runner:
    print(message)
```

On every iteration, an API request is made. If Claude wants to call one of the given tools, it's automatically called, and the result is returned directly to the model in the next iteration.

### Message batches

This SDK provides support for the [Message Batches API](https://platform.claude.com/docs/en/build-with-claude/batch-processing) under `client.messages.batches`.

#### Creating a batch

Message Batches takes an array of requests, where each object has a `custom_id` identifier and the same request `params` as the standard Messages API:

```python
client.messages.batches.create(
    requests=[
        {
            "custom_id": "my-first-request",
            "params": {
                "model": "claude-opus-4-7",
                "max_tokens": 1024,
                "messages": [{"role": "user", "content": "Hello, world"}],
            },
        },
        {
            "custom_id": "my-second-request",
            "params": {
                "model": "claude-opus-4-7",
                "max_tokens": 1024,
                "messages": [{"role": "user", "content": "Hi again, friend"}],
            },
        },
    ]
)
```

#### Getting results from a batch

Once a Message Batch has been processed, indicated by `.processing_status == 'ended'`, you can access the results with `.batches.results()`:

```python nocheck hidelines={1..2}
import anthropic

client = anthropic.Anthropic()
batch_id = "batch_abc123"
result_stream = client.messages.batches.results(batch_id)
for entry in result_stream:
    if entry.result.type == "succeeded":
        print(entry.result.message.content)
```

### File uploads

Request parameters that correspond to file uploads can be passed in many different forms:

- A `PathLike` object (e.g., `pathlib.Path`)
- A tuple of `(filename, content, content_type)`
- A `BinaryIO` file-like object
- The return value of the `toFile` helper

```python nocheck
from pathlib import Path
from anthropic import Anthropic

client = Anthropic()

## Upload using a file path
client.beta.files.upload(
    file=Path("/path/to/file"),
    betas=["files-api-2025-04-14"],
)

## Upload using bytes
client.beta.files.upload(
    file=("file.txt", b"my bytes", "text/plain"),
    betas=["files-api-2025-04-14"],
)
```

The async client uses the exact same interface. If you pass a `PathLike` instance, the file contents are read asynchronously automatically.

### Handling errors

When the library is unable to connect to the API, or if the API returns a non-success status code (i.e., 4xx or 5xx response), a subclass of `APIError` is raised:

```python hidelines={2..5}
import anthropic
from anthropic import Anthropic

client = Anthropic()

try:
    message = client.messages.create(
        max_tokens=1024,
        messages=[
            {
                "role": "user",
                "content": "Hello, Claude",
            }
        ],
        model="claude-opus-4-7",
    )
except anthropic.APIConnectionError as e:
    print("The server could not be reached")
    print(e.__cause__)  # an underlying Exception, likely raised within httpx
except anthropic.RateLimitError as e:
    print("A 429 status code was received; we should back off a bit.")
except anthropic.APIStatusError as e:
    print("Another non-200-range status code was received")
    print(e.status_code)
    print(e.response)
```

Error codes are as follows:

| Status Code | Error Type |
|-------------|-----------|
| 400 | `BadRequestError` |
| 401 | `AuthenticationError` |
| 403 | `PermissionDeniedError` |
| 404 | `NotFoundError` |
| 422 | `UnprocessableEntityError` |
| 429 | `RateLimitError` |
| >=500 | `InternalServerError` |
| N/A | `APIConnectionError` |

### Request IDs

> For more information on debugging requests, see the [errors documentation](https://platform.claude.com/docs/en/api/errors#request-id).

All object responses in the SDK provide a `_request_id` property which is added from the `request-id` response header so that you can quickly log failing requests and report them back to Anthropic.

```python
message = client.messages.create(
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello, Claude"}],
    model="claude-opus-4-7",
)
print(message._request_id)  # e.g., req_018EeWyXxfu5pfWkrYcMdjWG
```

> Unlike other properties that use an `_` prefix, the `_request_id` property is public. Unless documented otherwise, all other `_` prefix properties, methods, and modules are private.

### Retries

Certain errors are automatically retried 2 times by default, with a short exponential backoff. Connection errors (for example, due to a network connectivity problem), 408 Request Timeout, 409 Conflict, 429 Rate Limit, and >=500 Internal errors are all retried by default.

You can use the `max_retries` option to configure or disable this:

```python hidelines={1..2}
from anthropic import Anthropic

## Configure the default for all requests:
client = Anthropic(
    max_retries=0,  # default is 2
)

## Or, configure per-request:
client.with_options(max_retries=5).messages.create(
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello, Claude"}],
    model="claude-opus-4-7",
)
```

### Timeouts

By default requests time out after 10 minutes. You can configure this with a `timeout` option, which accepts a float or an `httpx.Timeout` object:

```python
import httpx
from anthropic import Anthropic

## Configure the default for all requests:
client = Anthropic(
    timeout=20.0,  # 20 seconds (default is 10 minutes)
)

## More granular control:
client = Anthropic(
    timeout=httpx.Timeout(60.0, read=5.0, write=10.0, connect=2.0),
)

## Override per-request:
client.with_options(timeout=5.0).messages.create(
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello, Claude"}],
    model="claude-opus-4-7",
)
```

On timeout, an `APITimeoutError` is thrown.

Note that requests which time out will be [retried twice by default](#retries).

### Long requests

> Consider using the streaming [Messages API](#streaming-responses) for longer running requests.

Avoid setting a large `max_tokens` value without using streaming. Some networks may drop idle connections after a certain period of time, which can cause the request to fail or [timeout](#timeouts) without receiving a response from Anthropic.

The SDK will throw a `ValueError` if a non-streaming request is expected to take longer than approximately 10 minutes. Passing `stream=True` or overriding the `timeout` option at the client or request level disables this error.

An expected request latency longer than the [timeout](#timeouts) for a non-streaming request will result in the client terminating the connection and retrying without receiving a response.

The SDK sets a [TCP socket keep-alive](https://tldp.org/HOWTO/TCP-Keepalive-HOWTO/overview.html) option to reduce the impact of idle connection timeouts on some networks. This can be overridden by passing a custom `http_client` option to the client.

### Auto-pagination

List methods in the Claude API are paginated. You can use the `for` syntax to iterate through items across all pages:

```python hidelines={1..2}
from anthropic import Anthropic

client = Anthropic()

all_batches = []
## Automatically fetches more pages as needed.
for batch in client.messages.batches.list(limit=20):
    all_batches.append(batch)
print(all_batches)
```

For async iteration:

```python hidelines={1..6}
import asyncio
from anthropic import AsyncAnthropic

client = AsyncAnthropic()

async def main() -> None:
    all_batches = []
    async for batch in client.messages.batches.list(limit=20):
        all_batches.append(batch)
    print(all_batches)

asyncio.run(main())
```

Alternatively, you can use the `.has_next_page()`, `.next_page_info()`, or `.get_next_page()` methods for more granular control working with pages:

```python nocheck
first_page = await client.messages.batches.list(limit=20)

if first_page.has_next_page():
    print(f"will fetch next page using these details: {first_page.next_page_info()}")
    next_page = await first_page.get_next_page()
    print(f"number of items we just fetched: {len(next_page.data)}")

## Remove `await` for non-async usage.
```

Or work directly with the returned data:

```python nocheck
first_page = await client.messages.batches.list(limit=20)

print(f"next page cursor: {first_page.last_id}")
for batch in first_page.data:
    print(batch.id)

## Remove `await` for non-async usage.
```

### Default headers

The SDK automatically sends the `anthropic-version` header set to `2023-06-01`.

If you need to, you can override it by setting default headers on the client object or per-request.

> Overriding default headers may result in incorrect types and other unexpected or undefined behavior in the SDK.

```python nocheck hidelines={1..2}
from anthropic import Anthropic

## Set default headers for all requests on the client
client = Anthropic(
    default_headers={"anthropic-version": "My-Custom-Value"},
)

## Or override per-request
client.messages.with_raw_response.create(
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello, Claude"}],
    model="claude-opus-4-7",
    extra_headers={"anthropic-version": "My-Custom-Value"},
)
```

### Type system

#### Request parameters

Nested request parameters are [TypedDicts](https://docs.python.org/3/library/typing.html#typing.TypedDict). Responses are [Pydantic models](https://docs.pydantic.dev) which also have helper methods for things like serializing back into JSON ([`v1`](https://docs.pydantic.dev/1.10/usage/models/), [`v2`](https://docs.pydantic.dev/latest/concepts/serialization/)).

Typed requests and responses provide autocomplete and documentation within your editor. If you'd like to see type errors in VS Code to help catch bugs earlier, set `python.analysis.typeCheckingMode` to `basic`.

#### Response models

To convert a Pydantic model to a dictionary, use the helper methods:

```python nocheck
message = client.messages.create(...)

## Convert to JSON string
json_str = message.to_json()

## Convert to dictionary
data = message.to_dict()
```

#### Handling null vs missing fields

In responses, you can distinguish between fields that are explicitly `null` versus fields that were not returned (missing):

```python nocheck
response = client.messages.create(
    model="claude-opus-4-7",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello"}],
)
if response.my_field is None:
    if "my_field" not in response.model_fields_set:
        print("field was not in the response")
    else:
        print("field was null")
```

### Advanced usage

#### Accessing raw response data (e.g., headers)

The "raw" `Response` returned by `httpx` can be accessed via the `.with_raw_response` property on the client. This is useful for accessing response headers or other metadata:

```python hidelines={1..2}
from anthropic import Anthropic

client = Anthropic()

response = client.messages.with_raw_response.create(
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello, Claude"}],
    model="claude-opus-4-7",
)

print(response.headers.get("x-request-id"))
message = (
    response.parse()
)  # get the object that `messages.create()` would have returned
print(message.content)
```

These methods return an `APIResponse` object.

#### Streaming response body

The `.with_raw_response` approach above eagerly reads the full response body when you make the request. To stream the response body instead, use `.with_streaming_response`, which requires a context manager and only reads the response body once you call `.read()`, `.text()`, `.json()`, `.iter_bytes()`, `.iter_text()`, `.iter_lines()`, or `.parse()`. In the async client, these are async methods.

```python
with client.messages.with_streaming_response.create(
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello, Claude"}],
    model="claude-opus-4-7",
) as response:
    print(response.headers.get("x-request-id"))

    for line in response.iter_lines():
        print(line)
```

The context manager is required so that the response will reliably be closed.

#### Logging

The SDK uses the standard library `logging` module.

You can enable logging by setting the environment variable `ANTHROPIC_LOG` to one of `debug`, `info`, `warn`, or `off`:

```bash
export ANTHROPIC_LOG=debug
```

#### Making custom/undocumented requests

This library is typed for convenient access to the documented API. If you need to access undocumented endpoints, params, or response properties, the library can still be used.

##### Undocumented endpoints

To make requests to undocumented endpoints, you can use `client.get`, `client.post`, and other HTTP verbs. Options on the client, such as retries, will be respected when making these requests.

```python nocheck
import httpx

response = client.post(
    "/foo",
    cast_to=httpx.Response,
    body={"my_param": True},
)

print(response.json())
```

##### Undocumented request params

If you want to explicitly send an extra param, you can do so with the `extra_query`, `extra_body`, and `extra_headers` request options.

> The `extra_` parameters override documented parameters of the same name. For security reasons, ensure these methods are only used with trusted input data.

##### Undocumented response properties

To access undocumented response properties, you can access the extra fields like `response.unknown_prop`. You can also get all extra fields on the Pydantic model as a dict with `response.model_extra`.

#### Configuring the HTTP client

You can directly override the [httpx client](https://www.python-httpx.org/api/#client) to customize it for your use case, including support for proxies and transports:

```python
import httpx
from anthropic import Anthropic, DefaultHttpxClient

client = Anthropic(
    # Or use the `ANTHROPIC_BASE_URL` env var
    base_url="http://my.test.server.example.com:8083",
    http_client=DefaultHttpxClient(
        proxy="http://my.test.proxy.example.com",
        transport=httpx.HTTPTransport(local_address="0.0.0.0"),
    ),
)
```

You can also customize the client on a per-request basis by using `with_options()`:

```python
client.with_options(http_client=DefaultHttpxClient(...))
```

> Use `DefaultHttpxClient` and `DefaultAsyncHttpxClient` instead of raw `httpx.Client` and `httpx.AsyncClient` to ensure the SDK's default configuration (timeouts, connection limits, etc.) is preserved.

#### Managing HTTP resources

By default the library closes underlying HTTP connections whenever the client is [garbage collected](https://docs.python.org/3/reference/datamodel.html#object.__del__). You can manually close the client using the `.close()` method if desired, or with a context manager that closes when exiting.

```python nocheck hidelines={1..2}
from anthropic import Anthropic

with Anthropic() as client:
    message = client.messages.create(...)

## HTTP client is automatically closed
```

### Beta features

Beta features are available before general release to get early feedback and test new functionality. You can check the availability of all of Claude's capabilities and tools in the [build with Claude overview](https://platform.claude.com/docs/en/build-with-claude/overview).

You can access most beta API features through the `beta` property of the client. To enable a particular beta feature, you need to add the appropriate [beta header](https://platform.claude.com/docs/en/api/beta-headers) to the `betas` field when creating a message.

For example, to use the [Files API](https://platform.claude.com/docs/en/build-with-claude/files):

```python nocheck hidelines={1..2}
from anthropic import Anthropic

client = Anthropic()

response = client.beta.messages.create(
    model="claude-opus-4-7",
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": [
                {"type": "text", "text": "Please summarize this document for me."},
                {
                    "type": "document",
                    "source": {
                        "type": "file",
                        "file_id": "file_abc123",
                    },
                },
            ],
        },
    ],
    betas=["files-api-2025-04-14"],
)
```

### Platform integrations

> For detailed platform setup guides with code examples, see:
> - [Amazon Bedrock](https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock)
> - [Amazon Bedrock (legacy)](https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock)
> - [Google Vertex AI](https://platform.claude.com/docs/en/build-with-claude/claude-on-vertex-ai)
> - [Microsoft Foundry](https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry)

All four client classes are included in the base `anthropic` package:

| Provider | Client | Extra dependencies |
|-----------|--------|-------------------|
| Bedrock | `from anthropic import AnthropicBedrockMantle` | `pip install anthropic[bedrock]` |
| Bedrock (`bedrock-runtime` path) | `from anthropic import AnthropicBedrock` | `pip install anthropic[bedrock]` |
| Vertex AI | `from anthropic import AnthropicVertex` | `pip install anthropic[vertex]` |
| Foundry | `from anthropic import AnthropicFoundry` | None |

Use `AnthropicBedrockMantle` for new projects; `AnthropicBedrock` remains for existing applications using the Bedrock `InvokeModel` API.

### Semantic versioning

This package generally follows [SemVer](https://semver.org/spec/v2.0.0.html) conventions, though certain backwards-incompatible changes may be released as minor versions:

1. Changes that only affect static types, without breaking runtime behavior.
2. Changes to library internals which are technically public but not intended or documented for external use.
3. Changes that aren't expected to impact the vast majority of users in practice.

#### Determining the installed version

If you've upgraded to the latest version but aren't seeing new features you were expecting, your Python environment is likely still using an older version. You can determine the version being used at runtime with:

```python hidelines={1..2}
import anthropic

print(anthropic.__version__)
```

### Additional resources

- [GitHub repository](https://github.com/anthropics/anthropic-sdk-python)
- [API reference](https://platform.claude.com/docs/en/api/overview)
- [Streaming guide](https://platform.claude.com/docs/en/build-with-claude/streaming)
- [Tool use guide](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview)

---

## TypeScript SDK

Install and configure the Anthropic TypeScript SDK for Node.js, Deno, Bun, and browser environments

---

This library provides convenient access to the Anthropic REST API from server-side TypeScript or JavaScript.

> For API feature documentation with code examples, see the [API reference](https://platform.claude.com/docs/en/api/overview). This page covers TypeScript-specific SDK features and configuration.

### Installation

```bash
npm install @anthropic-ai/sdk
```

### Requirements

TypeScript >= 4.9 is supported.

The following runtimes are supported:

- Node.js 20 LTS or later ([non-EOL](https://endoflife.date/nodejs)) versions.
- Deno v1.28.0 or higher.
- Bun 1.0 or later.
- Cloudflare Workers.
- Vercel Edge Runtime.
- Jest 28 or greater with the `"node"` environment (`"jsdom"` is not supported at this time).
- Nitro v2.6 or greater.
- Web browsers: disabled by default to avoid exposing your secret API credentials (see [API key best practices](https://support.anthropic.com/en/articles/9767949-api-key-best-practices-keeping-your-keys-safe-and-secure)). Enable browser support by explicitly setting `dangerouslyAllowBrowser` to `true`.

Note that React Native is not supported at this time.

If you are interested in other runtime environments, please open or upvote an issue on [GitHub](https://github.com/anthropics/anthropic-sdk-typescript).

### Usage

```typescript hidelines={1..2}
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  apiKey: process.env["ANTHROPIC_API_KEY"] // This is the default and can be omitted
});

const message = await client.messages.create({
  max_tokens: 1024,
  messages: [{ role: "user", content: "Hello, Claude" }],
  model: "claude-opus-4-7"
});

console.log(message.content);
```

### Request and response types

This library includes TypeScript definitions for all request params and response fields. You may import and use them like so:

```typescript hidelines={1..2}
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  apiKey: process.env["ANTHROPIC_API_KEY"] // This is the default and can be omitted
});

const params: Anthropic.MessageCreateParams = {
  max_tokens: 1024,
  messages: [{ role: "user", content: "Hello, Claude" }],
  model: "claude-opus-4-7"
};
const message: Anthropic.Message = await client.messages.create(params);
```

Documentation for each method, request param, and response field are available in docstrings and will appear on hover in most modern editors.

### Counting tokens

You can see the exact usage for a given request through the `usage` response property, e.g.

```typescript
const message = await client.messages.create(/* ... */);
console.log(message.usage);
// { input_tokens: 25, output_tokens: 13 }
```

### Streaming responses

The SDK provides support for streaming responses using Server Sent Events (SSE).

```typescript hidelines={1..2}
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic();

const stream = await client.messages.create({
  max_tokens: 1024,
  messages: [{ role: "user", content: "Hello, Claude" }],
  model: "claude-opus-4-7",
  stream: true
});
for await (const messageStreamEvent of stream) {
  console.log(messageStreamEvent.type);
}
```

If you need to cancel a stream, you can `break` from the loop or call `stream.controller.abort()`.

### Streaming helpers

This library provides several conveniences for streaming messages, for example:

```typescript hidelines={1..5,-3..-1}
import Anthropic from "@anthropic-ai/sdk";

const anthropic = new Anthropic();

async function main() {
  const stream = anthropic.messages
    .stream({
      model: "claude-opus-4-7",
      max_tokens: 1024,
      messages: [
        {
          role: "user",
          content: "Say hello there!"
        }
      ]
    })
    .on("text", (text) => {
      console.log(text);
    });

  const message = await stream.finalMessage();
  console.log(message);
}

main();
```

Streaming with `client.messages.stream(...)` exposes various helpers for your convenience including event handlers and accumulation.

Alternatively, you can use `client.messages.create({ ..., stream: true })` which only returns an async iterable of the events in the stream and thus uses less memory (it does not build up a final message object for you).

### Tool helpers

This SDK provides helpers for making it easy to create and run tools in the Messages API. You can use Zod schemas or JSON Schemas to describe the input to a tool. You can then run those tools using the `client.messages.toolRunner()` method. This method will handle passing the inputs generated by the chosen model into the right tool and passing the result back to the model.

For more details on tool use, see the [tool use overview](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview).

```typescript hidelines={1..2}
import Anthropic from "@anthropic-ai/sdk";

import { betaZodTool } from "@anthropic-ai/sdk/helpers/beta/zod";
import { z } from "zod";

const anthropic = new Anthropic();

const weatherTool = betaZodTool({
  name: "get_weather",
  inputSchema: z.object({
    location: z.string()
  }),
  description: "Get the current weather in a given location",
  run: (input) => {
    return `The weather in ${input.location} is foggy and 60°F`;
  }
});

const finalMessage = await anthropic.beta.messages.toolRunner({
  model: "claude-opus-4-7",
  max_tokens: 1000,
  messages: [{ role: "user", content: "What is the weather in San Francisco?" }],
  tools: [weatherTool]
});
```

#### Tool errors

To report an error from a tool back to the model, throw a `ToolError` from the `run` function. Unlike a plain `Error`, `ToolError` accepts content blocks, allowing you to include images or other structured content in the error response:

```typescript nocheck
import { ToolError } from "@anthropic-ai/sdk/lib/tools/BetaRunnableTool";

const screenshotTool = betaZodTool({
  name: "take_screenshot",
  inputSchema: z.object({ url: z.string() }),
  run: async (input) => {
    if (!isValidUrl(input.url)) {
      throw new ToolError(`Invalid URL: ${input.url}`);
    }
    const result = await takeScreenshot(input.url);
    if (result.error) {
      // Include the error screenshot so the model can see what went wrong
      throw new ToolError([
        { type: "text", text: `Failed to load page: ${result.error}` },
        {
          type: "image",
          source: { type: "base64", data: result.screenshot, media_type: "image/png" }
        }
      ]);
    }
    return {
      type: "image",
      source: { type: "base64", data: result.screenshot, media_type: "image/png" }
    };
  }
});
```

If a plain `Error` is thrown, the message will be converted to a text content block.

### Tool use

This SDK provides support for tool use, aka function calling. More details can be found in the [tool use overview](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview).

### MCP helpers

This SDK provides helpers for integrating with [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) servers. These helpers convert MCP types to Claude API types, reducing boilerplate when working with MCP tools, prompts, and resources.

> The Claude API also supports an [`mcp_servers` parameter](https://platform.claude.com/docs/en/agents-and-tools/mcp) that lets Claude connect directly to remote MCP servers. Use `mcp_servers` when you have remote servers accessible via URL and only need tool support. Use the MCP helpers when you need local MCP servers, prompts, resources, or more control over the MCP connection.

For the Claude API's built-in remote MCP server support, see [MCP Connector](https://platform.claude.com/docs/en/agents-and-tools/mcp-connector).

```typescript nocheck hidelines={1}
import Anthropic from "@anthropic-ai/sdk";
import {
  mcpTools,
  mcpMessages,
  mcpResourceToContent,
  mcpResourceToFile
} from "@anthropic-ai/sdk/helpers/beta/mcp";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

const anthropic = new Anthropic();

// Connect to an MCP server
const transport = new StdioClientTransport({ command: "mcp-server", args: [] });
const mcpClient = new Client({ name: "my-client", version: "1.0.0" });
await mcpClient.connect(transport);

// Use MCP prompts
const { messages } = await mcpClient.getPrompt({ name: "my-prompt" });
const response = await anthropic.beta.messages.create({
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: mcpMessages(messages)
});

// Use MCP tools with toolRunner
const { tools } = await mcpClient.listTools();
const runner = await anthropic.beta.messages.toolRunner({
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: [{ role: "user", content: "Use the available tools" }],
  tools: mcpTools(tools, mcpClient)
});

// Use MCP resources as content
const resource = await mcpClient.readResource({ uri: "file:///path/to/doc.txt" });
await anthropic.beta.messages.create({
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: [
    {
      role: "user",
      content: [
        mcpResourceToContent(resource),
        { type: "text", text: "Summarize this document" }
      ]
    }
  ]
});

// Upload MCP resources as files
const fileResource = await mcpClient.readResource({ uri: "file:///path/to/data.json" });
await anthropic.beta.files.upload({ file: mcpResourceToFile(fileResource) });
```

#### MCP error handling

The conversion functions throw `UnsupportedMCPValueError` if an MCP value isn't supported by the Claude API (e.g., unsupported content type, unsupported MIME type, non-http/https resource link).

### Message batches

This SDK provides support for the [Message Batches API](https://platform.claude.com/docs/en/build-with-claude/batch-processing) under the `client.messages.batches` namespace.

#### Creating a batch

Message Batches takes an array of requests, where each object has a `custom_id` identifier, and the exact same request `params` as the standard Messages API:

```typescript
await client.messages.batches.create({
  requests: [
    {
      custom_id: "my-first-request",
      params: {
        model: "claude-opus-4-7",
        max_tokens: 1024,
        messages: [{ role: "user", content: "Hello, world" }]
      }
    },
    {
      custom_id: "my-second-request",
      params: {
        model: "claude-opus-4-7",
        max_tokens: 1024,
        messages: [{ role: "user", content: "Hi again, friend" }]
      }
    }
  ]
});
```

#### Getting results from a batch

Once a Message Batch has been processed, indicated by `.processing_status === 'ended'`, you can access the results with `.batches.results()`

```typescript nocheck
const results = await client.messages.batches.results(batch_id);
for await (const entry of results) {
  if (entry.result.type === "succeeded") {
    console.log(entry.result.message.content);
  }
}
```

### File uploads

Request parameters that correspond to file uploads can be passed in many different forms:

- `File` (or an object with the same structure)
- a `fetch` `Response` (or an object with the same structure)
- an `fs.ReadStream`
- the return value of the `toFile` helper

Set the content-type explicitly as the files API will not infer it for you:

```typescript nocheck
import fs from "fs";
import Anthropic, { toFile } from "@anthropic-ai/sdk";

const client = new Anthropic();

// If you have access to Node `fs` we recommend using `fs.createReadStream()`:
await client.beta.files.upload({
  file: await toFile(fs.createReadStream("/path/to/file"), undefined, {
    type: "application/json"
  }),
  betas: ["files-api-2025-04-14"]
});

// Or if you have the web `File` API you can pass a `File` instance:
await client.beta.files.upload({
  file: new File(["my bytes"], "file.txt", { type: "text/plain" }),
  betas: ["files-api-2025-04-14"]
});
// You can also pass a `fetch` `Response`:
await client.beta.files.upload({
  file: await fetch("https://somesite/file"),
  betas: ["files-api-2025-04-14"]
});

// Or a `Buffer` / `Uint8Array`
await client.beta.files.upload({
  file: await toFile(Buffer.from("my bytes"), "file", { type: "text/plain" }),
  betas: ["files-api-2025-04-14"]
});
await client.beta.files.upload({
  file: await toFile(new Uint8Array([0, 1, 2]), "file", { type: "text/plain" }),
  betas: ["files-api-2025-04-14"]
});
```

### Handling errors

When the library is unable to connect to the API,
or if the API returns a non-success status code (i.e., 4xx or 5xx response),
a subclass of `APIError` will be thrown:

```typescript
const message = await client.messages
  .create({
    max_tokens: 1024,
    messages: [{ role: "user", content: "Hello, Claude" }],
    model: "claude-opus-4-7"
  })
  .catch(async (err) => {
    if (err instanceof Anthropic.APIError) {
      console.log(err.status); // 400
      console.log(err.name); // BadRequestError
      console.log(err.headers); // {server: 'nginx', ...}
    } else {
      throw err;
    }
  });
```

Error codes are as follows:

| Status Code | Error Type                 |
| ----------- | -------------------------- |
| 400         | `BadRequestError`          |
| 401         | `AuthenticationError`      |
| 403         | `PermissionDeniedError`    |
| 404         | `NotFoundError`            |
| 422         | `UnprocessableEntityError` |
| 429         | `RateLimitError`           |
| >=500       | `InternalServerError`      |
| N/A         | `APIConnectionError`       |

### Request IDs

> For more information on debugging requests, see [these docs](https://platform.claude.com/docs/en/api/errors#request-id)

All object responses in the SDK provide a `_request_id` property which is added from the `request-id` response header so that you can quickly log failing requests and report them back to Anthropic.

```typescript
const message = await client.messages.create({
  max_tokens: 1024,
  messages: [{ role: "user", content: "Hello, Claude" }],
  model: "claude-opus-4-7"
});
console.log(message._request_id); // req_018EeWyXxfu5pfWkrYcMdjWG
```

### Retries

Certain errors will be automatically retried 2 times by default, with a short exponential backoff.
Connection errors (for example, due to a network connectivity problem), 408 Request Timeout, 409 Conflict,
429 Rate Limit, and >=500 Internal errors will all be retried by default.

You can use the `maxRetries` option to configure or disable this:

```typescript
// Configure the default for all requests:
const client = new Anthropic({
  maxRetries: 0 // default is 2
});

// Or, configure per-request:
await client.messages.create(
  {
    max_tokens: 1024,
    messages: [{ role: "user", content: "Hello, Claude" }],
    model: "claude-opus-4-7"
  },
  { maxRetries: 5 }
);
```

### Timeouts

By default requests time out after 10 minutes. However if you have specified a large `max_tokens` value and are
_not_ streaming, the default timeout will be calculated dynamically using the formula:

```typescript nocheck
const minimum = 10 * 60;
const calculated = (60 * 60 * maxTokens) / 128_000;
return calculated < minimum ? minimum * 1000 : calculated * 1000;
```

which will result in a timeout up to 60 minutes, scaled by the `max_tokens` parameter, unless overridden at the request or client level.

You can configure this with a `timeout` option:

```typescript
// Configure the default for all requests:
const client = new Anthropic({
  timeout: 20 * 1000 // 20 seconds (default is 10 minutes)
});

// Override per-request:
await client.messages.create(
  {
    max_tokens: 1024,
    messages: [{ role: "user", content: "Hello, Claude" }],
    model: "claude-opus-4-7"
  },
  { timeout: 5 * 1000 }
);
```

On timeout, an `APIConnectionTimeoutError` is thrown.

Note that requests which time out will be [retried twice by default](#retries).

### Long requests

> Consider using the streaming [Messages API](#streaming-responses) for longer running requests.

Avoid setting a large `max_tokens` value without using streaming.
Some networks may drop idle connections after a certain period of time, which
can cause the request to fail or [timeout](#timeouts) without receiving a response from Anthropic.

This SDK will also throw an error if a non-streaming request is expected to be above roughly 10 minutes long.
Passing `stream: true` or [overriding](#timeouts) the `timeout` option at the client or request level disables this error.

An expected request latency longer than the [timeout](#timeouts) for a non-streaming request
will result in the client terminating the connection and retrying without receiving a response.

When supported by the `fetch` implementation, the SDK sets a [TCP socket keep-alive](https://tldp.org/HOWTO/TCP-Keepalive-HOWTO/overview.html) option in order
to reduce the impact of idle connection timeouts on some networks.
This can be [overridden](#configuring-proxies) by configuring a custom proxy.

### Auto-pagination

List methods in the Claude API are paginated.
You can use the `for await ... of` syntax to iterate through items across all pages:

```typescript
async function fetchAllMessageBatches(params: Record<string, unknown>) {
  const allMessageBatches = [];
  // Automatically fetches more pages as needed.
  for await (const messageBatch of client.messages.batches.list({ limit: 20 })) {
    allMessageBatches.push(messageBatch);
  }
  return allMessageBatches;
}
```

Alternatively, you can request a single page at a time:

```typescript
let page = await client.messages.batches.list({ limit: 20 });
for (const messageBatch of page.data) {
  console.log(messageBatch);
}

// Convenience methods are provided for manually paginating:
while (page.hasNextPage()) {
  page = await page.getNextPage();
  // ...
}
```

### Default headers

The SDK automatically sends the `anthropic-version` header set to `2023-06-01`.

If you need to, you can override it by setting default headers on a per-request basis.

Be aware that doing so may result in incorrect types and other unexpected or undefined behavior in the SDK.

```typescript nocheck hidelines={1..2}
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic();

const message = await client.messages.create(
  {
    max_tokens: 1024,
    messages: [{ role: "user", content: "Hello, Claude" }],
    model: "claude-opus-4-7"
  },
  { headers: { "anthropic-version": "My-Custom-Value" } }
);
```

### Advanced usage

#### Accessing raw Response data (e.g., headers)

The "raw" `Response` returned by `fetch()` can be accessed through the `.asResponse()` method on the `APIPromise` type that all methods return.
This method returns as soon as the headers for a successful response are received and does not consume the response body, so you are free to write custom parsing or streaming logic.

You can also use the `.withResponse()` method to get the raw `Response` along with the parsed data.
Unlike `.asResponse()` this method consumes the body, returning once it is parsed.

```typescript
const client = new Anthropic();

const response = await client.messages
  .create({
    max_tokens: 1024,
    messages: [{ role: "user", content: "Hello, Claude" }],
    model: "claude-opus-4-7"
  })
  .asResponse();
console.log(response.headers.get("X-My-Header"));
console.log(response.statusText); // access the underlying Response object

const { data: message, response: raw } = await client.messages
  .create({
    max_tokens: 1024,
    messages: [{ role: "user", content: "Hello, Claude" }],
    model: "claude-opus-4-7"
  })
  .withResponse();
console.log(raw.headers.get("X-My-Header"));
console.log(message.content);
```

#### Logging

> All log messages are intended for debugging only. The format and content of log messages
> may change between releases.

##### Log levels

The log level can be configured in two ways:

1. Via the `ANTHROPIC_LOG` environment variable
2. Using the `logLevel` client option (overrides the environment variable if set)

```typescript hidelines={1..2}
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  logLevel: "debug" // Show all log messages
});
```

Available log levels, from most to least verbose:

- `'debug'` - Show debug messages, info, warnings, and errors
- `'info'` - Show info messages, warnings, and errors
- `'warn'` - Show warnings and errors (default)
- `'error'` - Show only errors
- `'off'` - Disable all logging

At the `'debug'` level, all HTTP requests and responses are logged, including headers and bodies.
Some authentication-related headers are redacted, but sensitive data in request and response bodies
may still be visible.

##### Custom logger

By default, this library logs to `globalThis.console`. You can also provide a custom logger.
Most logging libraries are supported, including [pino](https://www.npmjs.com/package/pino), [winston](https://www.npmjs.com/package/winston), [bunyan](https://www.npmjs.com/package/bunyan), [consola](https://www.npmjs.com/package/consola), [signale](https://www.npmjs.com/package/signale), and [@std/log](https://jsr.io/@std/log). If your logger doesn't work, please open an issue.

When providing a custom logger, the `logLevel` option still controls which messages are emitted, messages
below the configured level will not be sent to your logger.

```typescript nocheck hidelines={1}
import Anthropic from "@anthropic-ai/sdk";
import pino from "pino";

const logger = pino();

const client = new Anthropic({
  logger: logger.child({ name: "Anthropic" }),
  logLevel: "debug" // Send all messages to pino, allowing it to filter
});
```

#### Making custom/undocumented requests

This library is typed for convenient access to the documented API. If you need to access undocumented
endpoints, params, or response properties, the library can still be used.

##### Undocumented endpoints

To make requests to undocumented endpoints, you can use `client.get`, `client.post`, and other HTTP verbs.
Options on the client, such as retries, will be respected when making these requests.

```typescript nocheck
await client.post("/some/path", {
  body: { some_prop: "foo" },
  query: { some_query_arg: "bar" }
});
```

##### Undocumented request params

To make requests using undocumented parameters, you may use `// @ts-expect-error` on the undocumented
parameter. This library doesn't validate at runtime that the request matches the type, so any extra values you
send will be sent as-is.

```typescript
client.messages.create({
  // ...
  // @ts-expect-error baz is not yet public
  baz: "undocumented option"
});
```

For requests with the `GET` verb, any extra params will be in the query, all other requests will send the
extra param in the body.

If you want to explicitly send an extra argument, you can do so with the `query`, `body`, and `headers` request
options.

##### Undocumented response properties

To access undocumented response properties, you may access the response object with `// @ts-expect-error` on
the response object, or cast the response object to the requisite type. Like the request params, the SDK does not
validate or strip extra properties from the response from the API.

#### Customizing the fetch client

By default, this library expects a global `fetch` function is defined.

If you want to use a different `fetch` function, you can either polyfill the global:

```typescript nocheck
import fetch from "my-fetch";

globalThis.fetch = fetch;
```

Or pass it to the client:

```typescript nocheck hidelines={1}
import Anthropic from "@anthropic-ai/sdk";
import fetch from "my-fetch";

const client = new Anthropic({ fetch });
```

#### Fetch options

If you want to set custom `fetch` options without overriding the `fetch` function, you can provide a `fetchOptions` object when instantiating the client or making a request. (Request-specific options override client options.)

```typescript hidelines={1..2}
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  fetchOptions: {
    // `RequestInit` options
  }
});
```

#### Configuring proxies

To modify proxy behavior, you can provide custom `fetchOptions` that add runtime-specific proxy
options to requests:

<Tabs>
<Tab title="Node.js">

```typescript nocheck hidelines={1}
import Anthropic from "@anthropic-ai/sdk";
import * as undici from "undici";

const proxyAgent = new undici.ProxyAgent("http://localhost:8888");
const client = new Anthropic({
  fetchOptions: {
    dispatcher: proxyAgent
  }
});
```
</Tab>
<Tab title="Bun">

```typescript nocheck hidelines={1..2}
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  fetchOptions: {
    proxy: "http://localhost:8888"
  }
});
```
</Tab>
<Tab title="Deno">

```typescript nocheck
import Anthropic from "npm:@anthropic-ai/sdk";

const httpClient = Deno.createHttpClient({ proxy: { url: "http://localhost:8888" } });
const client = new Anthropic({
  fetchOptions: {
    client: httpClient
  }
});
```
</Tab>
</Tabs>

### Beta features

Beta features are available before general release to get early feedback and test new functionality. You can check the availability of all of Claude's capabilities and tools in the [build with Claude overview](https://platform.claude.com/docs/en/build-with-claude/overview).

You can access most beta API features through the beta property of the client. To enable a particular beta feature, you need to add the appropriate [beta header](https://platform.claude.com/docs/en/api/beta-headers) to the `betas` field when creating a message.

For example, to use the [Files API](https://platform.claude.com/docs/en/build-with-claude/files):

```typescript nocheck hidelines={1..2}
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic();
const response = await client.beta.messages.create({
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: [
    {
      role: "user",
      content: [
        { type: "text", text: "Please summarize this document for me." },
        {
          type: "document",
          source: {
            type: "file",
            file_id: "file_abc123"
          }
        }
      ]
    }
  ],
  betas: ["files-api-2025-04-14"]
});
```

### Runtime support

<section title="Browser usage">

Enabling the `dangerouslyAllowBrowser` option can be dangerous because it exposes your secret API credentials in the client-side code. Web browsers are inherently less secure than server environments, any user with access to the browser can potentially inspect, extract, and misuse these credentials. This could lead to unauthorized access using your credentials and potentially compromise sensitive data or functionality.

**When might this not be dangerous?**

In certain scenarios where enabling browser support might not pose significant risks:

- **Internal Tools:** If the application is used solely within a controlled internal environment where the users are trusted, the risk of credential exposure can be mitigated.
- **Development or debugging purpose:** Enabling this feature temporarily might be acceptable, provided the credentials are short-lived, aren't also used in production environments, or are frequently rotated.

</section>

### Platform integrations

> For detailed platform setup guides with code examples, see:
> - [Amazon Bedrock](https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock)
> - [Amazon Bedrock (legacy)](https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock)
> - [Google Vertex AI](https://platform.claude.com/docs/en/build-with-claude/claude-on-vertex-ai)
> - [Microsoft Foundry](https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry)

The TypeScript SDK supports Bedrock, Vertex AI, and Foundry through separate npm packages:

- **Bedrock:** `npm install @anthropic-ai/bedrock-sdk`: Provides `AnthropicBedrockMantle` client, and `AnthropicBedrock` for the `bedrock-runtime` path
- **Vertex AI:** `npm install @anthropic-ai/vertex-sdk`: Provides `AnthropicVertex` client
- **Foundry:** `npm install @anthropic-ai/foundry-sdk`: Provides `AnthropicFoundry` client

Use `AnthropicBedrockMantle` for new projects; `AnthropicBedrock` remains for existing applications using the Bedrock `InvokeModel` API.

### Semantic versioning

This package generally follows [SemVer](https://semver.org/spec/v2.0.0.html) conventions, though certain backwards-incompatible changes may be released as minor versions:

1. Changes that only affect static types, without breaking runtime behavior.
2. Changes to library internals which are technically public but not intended or documented for external use. _(Please open a GitHub issue to let the maintainers know if you are relying on such internals.)_
3. Changes that aren't expected to impact the vast majority of users in practice.

Backwards-compatibility is taken seriously to ensure you can rely on a smooth upgrade experience.

### Frequently asked questions

See the [GitHub repository](https://github.com/anthropics/anthropic-sdk-typescript) for FAQs, issues, and community support.

### Additional resources

- [GitHub repository](https://github.com/anthropics/anthropic-sdk-typescript)
- [API reference](https://platform.claude.com/docs/en/api/overview)
- [Streaming guide](https://platform.claude.com/docs/en/build-with-claude/streaming)
- [Tool use guide](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview)

---

## Java SDK

Install and configure the Anthropic Java SDK with builder patterns and async support

---

The Anthropic Java SDK provides convenient access to the Anthropic REST API from applications written in Java. It uses the builder pattern for creating requests and supports both synchronous and asynchronous operations.

> For API feature documentation with code examples, see the [API reference](https://platform.claude.com/docs/en/api/overview). This page covers Java-specific SDK features and configuration.

### Installation

<Tabs>
<Tab title="Gradle">
```kotlin
implementation("com.anthropic:anthropic-java:2.22.0")
```
</Tab>
<Tab title="Maven">
```xml
<dependency>
    <groupId>com.anthropic</groupId>
    <artifactId>anthropic-java</artifactId>
    <version>2.22.0</version>
</dependency>
```
</Tab>
</Tabs>

### Requirements

This library requires Java 8 or later.

> The SDK supports Java 8 and later. Code examples in this documentation are written as [JDK 25 compact source files](https://openjdk.org/jeps/512), using a bare `void main()` entry point and `IO.println()` for output. The API calls themselves are identical on every supported JDK; to compile an example on an earlier version, replace `IO.println(...)` with `System.out.println(...)` and place the body inside `public static void main(String[] args)` within a class.

### Quick start

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;
import com.anthropic.models.messages.Message;
import com.anthropic.models.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

// Configures using the `anthropic.apiKey`, `anthropic.authToken` and `anthropic.baseUrl` system properties
// Or configures using the `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` and `ANTHROPIC_BASE_URL` environment variables
AnthropicClient client = AnthropicOkHttpClient.fromEnv();

MessageCreateParams params = MessageCreateParams.builder()
  .maxTokens(1024L)
  .addUserMessage("Hello, Claude")
  .model(Model.CLAUDE_OPUS_4_7)
  .build();

Message message = client.messages().create(params);
```

### Client configuration

#### API key setup

Configure the client using system properties or environment variables:

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;

// Configures using the `anthropic.apiKey`, `anthropic.authToken` and `anthropic.baseUrl` system properties
// Or configures using the `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` and `ANTHROPIC_BASE_URL` environment variables
AnthropicClient client = AnthropicOkHttpClient.fromEnv();
```

Or configure manually:

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;

AnthropicClient client = AnthropicOkHttpClient.builder()
  .apiKey("my-anthropic-api-key")
  .build();
```

Or use a combination of both approaches:

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;

AnthropicClient client = AnthropicOkHttpClient.builder()
  // Configures using system properties or environment variables
  .fromEnv()
  .apiKey("my-anthropic-api-key")
  .build();
```

#### Configuration options

| Setter      | System property       | Environment variable   | Required | Default value                 |
| ----------- | --------------------- | ---------------------- | -------- | ----------------------------- |
| `apiKey`    | `anthropic.apiKey`    | `ANTHROPIC_API_KEY`    | false    | -                             |
| `authToken` | `anthropic.authToken` | `ANTHROPIC_AUTH_TOKEN` | false    | -                             |
| `baseUrl`   | `anthropic.baseUrl`   | `ANTHROPIC_BASE_URL`   | true     | `"https://api.anthropic.com"` |

System properties take precedence over environment variables.

> Don't create more than one client in the same application. Each client has a connection pool and thread pools, which are more efficient to share between requests.

#### Modifying configuration

To temporarily use a modified client configuration while reusing the same connection and thread pools, call `withOptions()` on any client or service:

```java nocheck
import com.anthropic.client.AnthropicClient;

AnthropicClient clientWithOptions = client.withOptions(optionsBuilder -> {
  optionsBuilder.baseUrl("https://example.com");
  optionsBuilder.maxRetries(42);
});
```

The `withOptions()` method does not affect the original client or service.

### Async usage

The default client is synchronous. To switch to asynchronous execution, call the `async()` method:

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;
import com.anthropic.models.messages.Message;
import com.anthropic.models.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

AnthropicClient client = AnthropicOkHttpClient.fromEnv();

MessageCreateParams params = MessageCreateParams.builder()
  .maxTokens(1024L)
  .addUserMessage("Hello, Claude")
  .model(Model.CLAUDE_OPUS_4_7)
  .build();

CompletableFuture<Message> message = client.async().messages().create(params);
```

Or create an asynchronous client from the beginning:

```java nocheck
import com.anthropic.client.AnthropicClientAsync;
import com.anthropic.client.okhttp.AnthropicOkHttpClientAsync;
import com.anthropic.models.messages.Message;
import com.anthropic.models.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

AnthropicClientAsync client = AnthropicOkHttpClientAsync.fromEnv();

MessageCreateParams params = MessageCreateParams.builder()
  .maxTokens(1024L)
  .addUserMessage("Hello, Claude")
  .model(Model.CLAUDE_OPUS_4_7)
  .build();

CompletableFuture<Message> message = client.messages().create(params);
```

The asynchronous client supports the same options as the synchronous one, except most methods return `CompletableFuture`s.

### Streaming

The SDK defines methods that return response "chunk" streams, where each chunk can be individually processed as soon as it arrives instead of waiting on the full response.

#### Synchronous streaming

These streaming methods return `StreamResponse` for synchronous clients:

```java nocheck
import com.anthropic.core.http.StreamResponse;
import com.anthropic.models.messages.RawMessageStreamEvent;

try (StreamResponse<RawMessageStreamEvent> streamResponse = client.messages().createStreaming(params)) {
    streamResponse.stream().forEach(chunk -> {
        IO.println(chunk);
    });
    IO.println("No more chunks!");
}
```

#### Asynchronous streaming

For asynchronous clients, the method returns `AsyncStreamResponse`:

```java nocheck
import com.anthropic.core.http.AsyncStreamResponse;
import com.anthropic.models.messages.RawMessageStreamEvent;

client.async().messages().createStreaming(params).subscribe(chunk -> {
    IO.println(chunk);
});

// If you need to handle errors or completion of the stream
client.async().messages().createStreaming(params).subscribe(new AsyncStreamResponse.Handler<>() {
    @Override
    public void onNext(RawMessageStreamEvent chunk) {
        IO.println(chunk);
    }

    @Override
    public void onComplete(Optional<Throwable> error) {
        if (error.isPresent()) {
            IO.println("Something went wrong!");
            throw new RuntimeException(error.get());
        } else {
            IO.println("No more chunks!");
        }
    }
});

// Or use futures
client.async().messages().createStreaming(params)
    .subscribe(chunk -> {
        IO.println(chunk);
    })
    .onCompleteFuture()
    .whenComplete((unused, error) -> {
        if (error != null) {
            IO.println("Something went wrong!");
            throw new RuntimeException(error);
        } else {
            IO.println("No more chunks!");
        }
    });
```

Async streaming uses a dedicated per-client cached thread pool `Executor` to stream without blocking the current thread. To use a different `Executor`:

```java nocheck
Executor executor = Executors.newFixedThreadPool(4);
client.async().messages().createStreaming(params).subscribe(
    chunk -> IO.println(chunk), executor
);
```

Or configure the client globally using the `streamHandlerExecutor` method:

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;

AnthropicClient client = AnthropicOkHttpClient.builder()
  .fromEnv()
  .streamHandlerExecutor(Executors.newFixedThreadPool(4))
  .build();
```

#### Streaming with message accumulator

A `MessageAccumulator` can record the stream of events in the response as they are processed and accumulate a `Message` object similar to what would have been returned by the non-streaming API.

For a synchronous response, add a `Stream.peek()` call to the stream pipeline to accumulate each event:

```java nocheck
import com.anthropic.core.http.StreamResponse;
import com.anthropic.helpers.MessageAccumulator;
import com.anthropic.models.messages.Message;
import com.anthropic.models.messages.RawMessageStreamEvent;

MessageAccumulator messageAccumulator = MessageAccumulator.create();

try (StreamResponse<RawMessageStreamEvent> streamResponse =
         client.messages().createStreaming(createParams)) {
    streamResponse.stream()
            .peek(messageAccumulator::accumulate)
            .flatMap(event -> event.contentBlockDelta().stream())
            .flatMap(deltaEvent -> deltaEvent.delta().text().stream())
            .forEach(textDelta -> IO.print(textDelta.text()));
}

Message message = messageAccumulator.message();
```

For an asynchronous response, add the `MessageAccumulator` to the `subscribe()` call:

```java nocheck
import com.anthropic.helpers.MessageAccumulator;
import com.anthropic.models.messages.Message;

MessageAccumulator messageAccumulator = MessageAccumulator.create();

client.async().messages()
        .createStreaming(createParams)
        .subscribe(event -> messageAccumulator.accumulate(event).contentBlockDelta().stream()
                .flatMap(deltaEvent -> deltaEvent.delta().text().stream())
                .forEach(textDelta -> IO.print(textDelta.text())))
        .onCompleteFuture()
        .join();

Message message = messageAccumulator.message();
```

A `BetaMessageAccumulator` is also available for the accumulation of a `BetaMessage` object. It is used in the same manner as the `MessageAccumulator`.

### Structured outputs

For complete structured outputs documentation including Java examples, see [Structured Outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs).

### Tool use

[Tool Use](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview) lets you integrate external tools and functions directly into the AI model's responses. Instead of producing plain text, the model can output instructions (with parameters) for invoking a tool or calling a function when appropriate. You define JSON schemas for tools, and the model uses the schemas to decide when and how to use these tools.

The tool use feature supports a "strict" mode (beta) that guarantees that the JSON output from the AI model will conform to the JSON schema you provide in the input parameters.

The SDK can derive a tool and its parameters automatically from the structure of an arbitrary Java class: the class's name (converted to snake case) provides the tool name, and the class's fields define the tool's parameters.

> Declare your tool classes as top-level classes or `static` nested classes. This requirement comes from the Jackson Databind library (`com.fasterxml.jackson.databind`), which the SDK uses to deserialize tool inputs into your class instances and cannot instantiate non-static inner classes.

#### Defining tools with annotations

```java nocheck
import com.fasterxml.jackson.annotation.JsonClassDescription;
import com.fasterxml.jackson.annotation.JsonPropertyDescription;

enum Unit {
  CELSIUS,
  FAHRENHEIT;

  public String toString() {
    switch (this) {
      case CELSIUS:
        return "C";
      case FAHRENHEIT:
      default:
        return "F";
    }
  }

  public double fromKelvin(double temperatureK) {
    switch (this) {
      case CELSIUS:
        return temperatureK - 273.15;
      case FAHRENHEIT:
      default:
        return (temperatureK - 273.15) * 1.8 + 32.0;
    }
  }
}

@JsonClassDescription("Get the weather in a given location")
static class GetWeather {

  @JsonPropertyDescription("The city and state, e.g. San Francisco, CA")
  public String location;

  @JsonPropertyDescription("The unit of temperature")
  public Unit unit;

  public Weather execute() {
    double temperatureK;
    switch (location) {
      case "San Francisco, CA":
        temperatureK = 300.0;
        break;
      case "New York, NY":
        temperatureK = 310.0;
        break;
      case "Dallas, TX":
        temperatureK = 305.0;
        break;
      default:
        temperatureK = 295;
        break;
    }
    return new Weather(String.format("%.0f%s", unit.fromKelvin(temperatureK), unit));
  }
}

static class Weather {

  public String temperature;

  public Weather(String temperature) {
    this.temperature = temperature;
  }
}
```

#### Calling tools

When your tool classes are defined, add them to the message parameters using `MessageCreateParams.addTool(Class<T>)` and then call them if requested to do so in the AI model's response. `BetaToolUseBlock.input(Class<T>)` can be used to parse a tool's parameters in JSON form to an instance of your tool-defining class.

After invoking the tool, use `BetaToolResultBlockParam.Builder.contentAsJson(Object)` to pass the tool's result back to the AI model:

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;
import com.anthropic.models.beta.messages.*;
import com.anthropic.models.messages.Model;

AnthropicClient client = AnthropicOkHttpClient.fromEnv();

MessageCreateParams.Builder createParamsBuilder = MessageCreateParams.builder()
        .model(Model.CLAUDE_OPUS_4_7)
        .maxTokens(2048)
        .addTool(GetWeather.class)
        .addUserMessage("What's the temperature in New York?");

client.beta().messages().create(createParamsBuilder.build()).content().stream()
        .flatMap(contentBlock -> contentBlock.toolUse().stream())
        .forEach(toolUseBlock -> createParamsBuilder
              // Add a message indicating that the tool use was requested.
              .addAssistantMessageOfBetaContentBlockParams(
                      List.of(BetaContentBlockParam.ofToolUse(BetaToolUseBlockParam.builder()
                              .name(toolUseBlock.name())
                              .id(toolUseBlock.id())
                              .input(toolUseBlock._input())
                              .build())))
              // Add a message with the result of the requested tool use.
              .addUserMessageOfBetaContentBlockParams(
                      List.of(BetaContentBlockParam.ofToolResult(BetaToolResultBlockParam.builder()
                              .toolUseId(toolUseBlock.id())
                              .contentAsJson(callTool(toolUseBlock))
                              .build()))));

client.beta().messages().create(createParamsBuilder.build()).content().stream()
        .flatMap(contentBlock -> contentBlock.text().stream())
        .forEach(textBlock -> IO.println(textBlock.text()));

private static Object callTool(BetaToolUseBlock toolUseBlock) {
  if (!"get_weather".equals(toolUseBlock.name())) {
    throw new IllegalArgumentException("Unknown tool: " + toolUseBlock.name());
  }

  GetWeather tool = toolUseBlock.input(GetWeather.class);
  return tool != null ? tool.execute() : new Weather("unknown");
}
```

#### Tool name conversion

Tool names are derived from the camel case tool class names (e.g., `GetWeather`) and converted to snake case (e.g., `get_weather`). Word boundaries begin where the current character is not the first character, is upper-case, and either the preceding character is lower-case, or the following character is lower-case. For example, `MyJSONParser` becomes `my_json_parser` and `ParseJSON` becomes `parse_json`. This conversion can be overridden using the `@JsonTypeName` annotation.

#### Local tool JSON schema validation

Like for structured outputs, you can perform local validation to check that the JSON schema derived from your tool class respects Anthropic's restrictions. Local validation is enabled by default, but it can be disabled:

```java nocheck
MessageCreateParams.Builder createParamsBuilder = MessageCreateParams.builder()
  .model(Model.CLAUDE_OPUS_4_7)
  .maxTokens(2048)
  .addTool(GetWeather.class, JsonSchemaLocalValidation.NO)
  .addUserMessage("What's the temperature in New York?");
```

#### Annotating tool classes

You can use annotations to add further information about tools to the JSON schemas:

- `@JsonClassDescription` - Add a description to a tool class detailing when and how to use that tool.
- `@JsonTypeName` - Set the tool name to something other than the simple name of the class converted to snake case.
- `@JsonPropertyDescription` - Add a detailed description to a tool parameter.
- `@JsonIgnore` - Exclude a `public` field or getter method from the generated JSON schema for a tool's parameters.
- `@JsonProperty` - Include a non-`public` field or getter method in the generated JSON schema for a tool's parameters.

### Message batches

The SDK provides support for the [Message Batches API](https://platform.claude.com/docs/en/build-with-claude/batch-processing) under the `client.messages().batches()` namespace. See the [pagination section](#pagination) for how to iterate through batch results.

### File uploads

The SDK defines methods that accept files through the `MultipartField` class:

```java nocheck
import com.anthropic.core.MultipartField;
import com.anthropic.models.beta.AnthropicBeta;
import com.anthropic.models.beta.files.FileMetadata;
import com.anthropic.models.beta.files.FileUploadParams;

FileUploadParams params = FileUploadParams.builder()
  .file(
    MultipartField.<InputStream>builder()
      .value(Files.newInputStream(Paths.get("/path/to/file.pdf")))
      .contentType("application/pdf")
      .build()
  )
  .addBeta(AnthropicBeta.FILES_API_2025_04_14)
  .build();

FileMetadata fileMetadata = client.beta().files().upload(params);
```

Or from an `InputStream`:

```java nocheck
import com.anthropic.core.MultipartField;
import com.anthropic.models.beta.AnthropicBeta;
import com.anthropic.models.beta.files.FileMetadata;
import com.anthropic.models.beta.files.FileUploadParams;

FileUploadParams params = FileUploadParams.builder()
  .file(
    MultipartField.<InputStream>builder()
      .value(new URL("https://example.com/path/to/file").openStream())
      .filename("document.pdf")
      .contentType("application/pdf")
      .build()
  )
  .addBeta(AnthropicBeta.FILES_API_2025_04_14)
  .build();

FileMetadata fileMetadata = client.beta().files().upload(params);
```

Or from in-memory bytes:

```java nocheck
import com.anthropic.core.MultipartField;
import com.anthropic.models.beta.AnthropicBeta;
import com.anthropic.models.beta.files.FileMetadata;
import com.anthropic.models.beta.files.FileUploadParams;

FileUploadParams params = FileUploadParams.builder()
  .file(
    MultipartField.<InputStream>builder()
      .value(new ByteArrayInputStream("content".getBytes()))
      .filename("document.txt")
      .contentType("text/plain")
      .build()
  )
  .addBeta(AnthropicBeta.FILES_API_2025_04_14)
  .build();

FileMetadata fileMetadata = client.beta().files().upload(params);
```

#### Binary responses

The SDK defines methods that return binary responses for API responses that aren't necessarily parsed as JSON:

```java nocheck
import com.anthropic.core.http.HttpResponse;

HttpResponse response = client.beta().files().download("file_id");
```

To save the response content to a file:

```java nocheck
import com.anthropic.core.http.HttpResponse;

try (HttpResponse response = client.beta().files().download(params)) {
    Files.copy(
        response.body(),
        Paths.get(path),
        StandardCopyOption.REPLACE_EXISTING
    );
} catch (Exception e) {
    IO.println("Something went wrong!");
    throw new RuntimeException(e);
}
```

Or transfer the response content to any `OutputStream`:

```java nocheck
import com.anthropic.core.http.HttpResponse;

try (HttpResponse response = client.beta().files().download(params)) {
    response.body().transferTo(Files.newOutputStream(Paths.get(path)));
} catch (Exception e) {
    IO.println("Something went wrong!");
    throw new RuntimeException(e);
}
```

### Error handling

The SDK throws custom unchecked exception types:

- `AnthropicServiceException` - Base class for HTTP errors.
- `AnthropicIoException` - I/O networking errors.
- `AnthropicRetryableException` - Generic error indicating a failure that could be retried.
- `AnthropicInvalidDataException` - Failure to interpret successfully parsed data (e.g., when accessing a property that's supposed to be required, but the API unexpectedly omitted it).
- `AnthropicException` - Base class for all exceptions.

#### Status code mapping

| Status | Exception |
| ------ | --------- |
| 400    | `BadRequestException` |
| 401    | `UnauthorizedException` |
| 403    | `PermissionDeniedException` |
| 404    | `NotFoundException` |
| 422    | `UnprocessableEntityException` |
| 429    | `RateLimitException` |
| 5xx    | `InternalServerException` |
| others | `UnexpectedStatusCodeException` |

`SseException` is thrown for errors encountered during SSE streaming after a successful initial HTTP response.

```java nocheck
import com.anthropic.errors.*;

try {
    Message message = client.messages().create(params);
} catch (RateLimitException e) {
    IO.println("Rate limited, retry after: " + e.headers());
} catch (UnauthorizedException e) {
    IO.println("Invalid API key");
} catch (AnthropicServiceException e) {
    IO.println("API error: " + e.statusCode());
} catch (AnthropicIoException e) {
    IO.println("Network error: " + e.getMessage());
}
```

### Request IDs

When using raw responses, you can access the `request-id` response header using the `requestId()` method:

```java nocheck
import com.anthropic.core.http.HttpResponseFor;
import com.anthropic.models.messages.Message;

HttpResponseFor<Message> message = client.messages().withRawResponse().create(params);

Optional<String> requestId = message.requestId();
```

This can be used to quickly log failing requests and report them back to Anthropic. For more information on debugging requests, see the [API error documentation](https://platform.claude.com/docs/en/api/errors#request-id).

### Retries

The SDK automatically retries 2 times by default, with a short exponential backoff between requests.

Only the following error types are retried:

- Connection errors (for example, due to a network connectivity problem)
- 408 Request Timeout
- 409 Conflict
- 429 Rate Limit
- 5xx Internal

The API may also explicitly instruct the SDK to retry or not retry a request.

To set a custom number of retries, configure the client using the `maxRetries` method:

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;

AnthropicClient client = AnthropicOkHttpClient.builder().fromEnv().maxRetries(4).build();
```

### Timeouts

Requests time out after 10 minutes by default.

However, for methods that accept `maxTokens`, if you specify a large `maxTokens` value and are streaming, then the default timeout will be calculated dynamically using this formula:

```java nocheck
Duration.ofSeconds(
    Math.min(
        60 * 60, // 1 hour max
        Math.max(
            10 * 60, // 10 minute minimum
            60 * 60 * maxTokens / 128_000
        )
    )
)
```

This results in a timeout of up to 60 minutes, scaled by the `maxTokens` parameter, unless overridden.

For non-streaming requests, the dynamic timeout scales from a 30 second minimum up to a 10 minute maximum based on `maxTokens`.

To set a custom timeout per-request:

```java nocheck
import com.anthropic.models.messages.Message;

Message message = client
  .messages()
  .create(params, RequestOptions.builder().timeout(Duration.ofSeconds(30)).build());
```

Or configure the default for all method calls at the client level:

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;

AnthropicClient client = AnthropicOkHttpClient.builder()
  .fromEnv()
  .timeout(Duration.ofSeconds(30))
  .build();
```

### Long requests

> Consider using [streaming](#streaming) for longer running requests.

Avoid setting a large `maxTokens` value without using streaming. Some networks may drop idle connections after a certain period of time, which can cause the request to fail or [timeout](#timeouts) without receiving a response from Anthropic. The SDK periodically pings the API to keep the connection alive and reduce the impact of these networks.

The SDK throws an error if a non-streaming request is expected to take longer than 10 minutes. Using a [streaming method](#streaming) or [overriding the timeout](#timeouts) at the client or request level disables the error.

### Pagination

The SDK provides convenient ways to access paginated results either one page at a time or item-by-item across all pages.

#### Auto-pagination

To iterate through all results across all pages, use the `autoPager()` method, which automatically fetches more pages as needed.

```java nocheck
import com.anthropic.models.messages.batches.BatchListPage;
import com.anthropic.models.messages.batches.MessageBatch;

BatchListPage page = client.messages().batches().list();

// Process as an Iterable
for (MessageBatch batch : page.autoPager()) {
    IO.println(batch);
}

// Process as a Stream
page.autoPager()
    .stream()
    .limit(50)
    .forEach(batch -> IO.println(batch));
```

When using the asynchronous client, the method returns an `AsyncStreamResponse`:

```java nocheck
import com.anthropic.core.http.AsyncStreamResponse;
import com.anthropic.models.messages.batches.BatchListPageAsync;
import com.anthropic.models.messages.batches.MessageBatch;

CompletableFuture<BatchListPageAsync> pageFuture = client.async().messages().batches().list();

pageFuture.thenAccept(page -> page.autoPager().subscribe(batch -> {
    IO.println(batch);
}));

// If you need to handle errors or completion of the stream
pageFuture.thenAccept(page -> page.autoPager().subscribe(new AsyncStreamResponse.Handler<>() {
    @Override
    public void onNext(MessageBatch batch) {
        IO.println(batch);
    }

    @Override
    public void onComplete(Optional<Throwable> error) {
        if (error.isPresent()) {
            IO.println("Something went wrong!");
            throw new RuntimeException(error.get());
        } else {
            IO.println("No more!");
        }
    }
}));

// Or use futures
pageFuture.thenAccept(page -> page.autoPager()
    .subscribe(batch -> {
        IO.println(batch);
    })
    .onCompleteFuture()
    .whenComplete((unused, error) -> {
        if (error != null) {
            IO.println("Something went wrong!");
            throw new RuntimeException(error);
        } else {
            IO.println("No more!");
        }
    }));
```

#### Manual pagination

To access individual page items and manually request the next page:

```java nocheck
import com.anthropic.models.messages.batches.BatchListPage;
import com.anthropic.models.messages.batches.MessageBatch;

BatchListPage page = client.messages().batches().list();
while (true) {
    for (MessageBatch batch : page.items()) {
        IO.println(batch);
    }

    if (!page.hasNextPage()) {
        break;
    }

    page = page.nextPage();
}
```

### Type system

#### Immutability and builders

Each class in the SDK has an associated builder for constructing it. Each class is immutable once constructed. If the class has an associated builder, then it has a `toBuilder()` method, which can be used to convert it back to a builder for making a modified copy.

```java nocheck
MessageCreateParams params = MessageCreateParams.builder()
  .maxTokens(1024L)
  .addUserMessage("Hello, Claude")
  .model(Model.CLAUDE_OPUS_4_7)
  .build();

// Create a modified copy using toBuilder()
MessageCreateParams modified = params.toBuilder().maxTokens(2048L).build();
```

Because each class is immutable, builder modification will never affect already built class instances.

#### Requests and responses

To send a request to the Claude API, build an instance of some `Params` class and pass it to the corresponding client method. When the response is received, it is deserialized into an instance of a Java class.

For example, `client.messages().create(...)` should be called with an instance of `MessageCreateParams`, and it will return an instance of `Message`.

#### Undocumented parameters

To set undocumented parameters, call the `putAdditionalHeader`, `putAdditionalQueryParam`, or `putAdditionalBodyProperty` methods on any `Params` class:

```java nocheck
import com.anthropic.core.JsonValue;
import com.anthropic.models.messages.MessageCreateParams;

MessageCreateParams params = MessageCreateParams.builder()
  .putAdditionalHeader("Secret-Header", "42")
  .putAdditionalQueryParam("secret_query_param", "42")
  .putAdditionalBodyProperty("secretProperty", JsonValue.from("42"))
  .build();
```

These can be accessed on the built object later using the `_additionalHeaders()`, `_additionalQueryParams()`, and `_additionalBodyProperties()` methods.

> The values passed to these methods overwrite values passed to earlier methods. For security reasons, ensure these methods are only used with trusted input data.

To set undocumented parameters on nested headers, query params, or body classes:

```java nocheck
import com.anthropic.core.JsonValue;
import com.anthropic.models.messages.MessageCreateParams;
import com.anthropic.models.messages.Metadata;

MessageCreateParams params = MessageCreateParams.builder()
  .metadata(
    Metadata.builder().putAdditionalProperty("secretProperty", JsonValue.from("42")).build()
  )
  .build();
```

These properties can be accessed on the nested built object later using the `_additionalProperties()` method.

To set a documented parameter or property to an undocumented or not yet supported value, pass a `JsonValue` object to its setter:

```java nocheck
import com.anthropic.core.JsonValue;
import com.anthropic.models.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

MessageCreateParams params = MessageCreateParams.builder()
  .maxTokens(JsonValue.from(3.14))
  .addUserMessage("Hello, Claude")
  .model(Model.CLAUDE_OPUS_4_7)
  .build();
```

#### JsonValue creation

The most straightforward way to create a `JsonValue` is using its `from(...)` method:

```java nocheck
import com.anthropic.core.JsonValue;

// Create primitive JSON values
JsonValue nullValue = JsonValue.from(null);

JsonValue booleanValue = JsonValue.from(true);

JsonValue numberValue = JsonValue.from(42);

JsonValue stringValue = JsonValue.from("Hello World!");

// Create a JSON array value equivalent to `["Hello", "World"]`
JsonValue arrayValue = JsonValue.from(List.of("Hello", "World"));

// Create a JSON object value equivalent to `{ "a": 1, "b": 2 }`
JsonValue objectValue = JsonValue.from(Map.of("a", 1, "b", 2));

// Create an arbitrarily nested JSON equivalent to:
// { "a": [1, 2], "b": [3, 4] }
JsonValue complexValue = JsonValue.from(Map.of("a", List.of(1, 2), "b", List.of(3, 4)));
```

#### Forcibly omitting required parameters

Normally a `Builder` class's `build` method will throw `IllegalStateException` if any required parameter or property is unset. To forcibly omit a required parameter or property, pass `JsonMissing`:

```java nocheck
import com.anthropic.core.JsonMissing;
import com.anthropic.models.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

MessageCreateParams params = MessageCreateParams.builder()
  .addUserMessage("Hello, world")
  .model(Model.CLAUDE_OPUS_4_7)
  .maxTokens(JsonMissing.of())
  .build();
```

#### Response properties

To access undocumented response properties, call the `_additionalProperties()` method:

```java nocheck
import com.anthropic.core.JsonValue;

Map<String, JsonValue> additionalProperties = client
  .messages()
  .create(params)
  ._additionalProperties();

JsonValue secretPropertyValue = additionalProperties.get("secretProperty");

String result = secretPropertyValue.accept(new JsonValue.Visitor<>() {
    @Override
    public String visitNull() {
        return "It's null!";
    }

    @Override
    public String visitBoolean(boolean value) {
        return "It's a boolean!";
    }

    @Override
    public String visitNumber(Number value) {
        return "It's a number!";
    }

    // Other methods include `visitMissing`, `visitString`, `visitArray`, and `visitObject`
    // The default implementation of each unimplemented method delegates to `visitDefault`,
    // which throws by default, but can also be overridden
});
```

To access a property's raw JSON value, call its `_` prefixed method:

```java nocheck
import com.anthropic.core.JsonField;
import com.anthropic.models.messages.StopReason;

JsonField<StopReason> stopReason = client.messages().create(params)._stopReason();

if (stopReason.isMissing()) {
  // The property is absent from the JSON response
} else if (stopReason.isNull()) {
  // The property was set to literal null
} else {
  // Check if value was provided as a string
  // Other methods include `asNumber()`, `asBoolean()`, etc.
  Optional<String> jsonString = stopReason.asString();

  // Try to deserialize into a custom type
  MyClass myObject = stopReason.asUnknown().orElseThrow().convert(MyClass.class);
}
```

#### Response validation

By default, the SDK does not throw an exception when the API returns a response that doesn't match the expected type. It throws `AnthropicInvalidDataException` only if you directly access the property.

To check that the response is completely well-typed upfront, call `validate()`:

```java nocheck
import com.anthropic.models.messages.Message;

Message message = client.messages().create(params).validate();
```

Or configure per-request:

```java nocheck
import com.anthropic.models.messages.Message;

Message message = client
  .messages()
  .create(params, RequestOptions.builder().responseValidation(true).build());
```

Or configure the default for all method calls at the client level:

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;

AnthropicClient client = AnthropicOkHttpClient.builder()
  .fromEnv()
  .responseValidation(true)
  .build();
```

### HTTP client customization

#### Proxy configuration

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;
import java.net.Proxy;

AnthropicClient client = AnthropicOkHttpClient.builder()
  .fromEnv()
  .proxy(new Proxy(Proxy.Type.HTTP, new InetSocketAddress("https://example.com", 8080)))
  .build();
```

#### HTTPS / SSL configuration

> Most applications should not call these methods, and instead use the system defaults. The defaults include special optimizations that can be lost if the implementations are modified.

```java nocheck
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;

AnthropicClient client = AnthropicOkHttpClient.builder()
  .fromEnv()
  .sslSocketFactory(yourSSLSocketFactory)
  .trustManager(yourTrustManager)
  .hostnameVerifier(yourHostnameVerifier)
  .build();
```

#### Custom HTTP client

The SDK consists of three artifacts:

- `anthropic-java-core` - Contains core SDK logic, does not depend on OkHttp. Exposes `AnthropicClient`, `AnthropicClientAsync`, and their implementation classes, all of which can work with any HTTP client.
- `anthropic-java-client-okhttp` - Depends on OkHttp. Exposes `AnthropicOkHttpClient` and `AnthropicOkHttpClientAsync`.
- `anthropic-java` - Depends on and exposes the APIs of both `anthropic-java-core` and `anthropic-java-client-okhttp`. Does not have its own logic.

This structure allows replacing the SDK's default HTTP client without pulling in unnecessary dependencies.

##### Customized OkHttpClient

> Try the available [network options](#retries) before replacing the default client.

To use a customized `OkHttpClient`:

1. Replace your `anthropic-java` dependency with `anthropic-java-core`.
2. Copy `anthropic-java-client-okhttp`'s `OkHttpClient` class into your code and customize it.
3. Construct `AnthropicClientImpl` or `AnthropicClientAsyncImpl` using your customized client.

##### Completely custom HTTP client

To use a completely custom HTTP client:

1. Replace your `anthropic-java` dependency with `anthropic-java-core`.
2. Write a class that implements the `HttpClient` interface.
3. Construct `AnthropicClientImpl` or `AnthropicClientAsyncImpl` using your new client class.

### Platform integrations

> For detailed platform setup guides with code examples, see:
> - [Amazon Bedrock](https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock)
> - [Amazon Bedrock (legacy)](https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock)
> - [Google Vertex AI](https://platform.claude.com/docs/en/build-with-claude/claude-on-vertex-ai)
> - [Microsoft Foundry](https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry)

The Java SDK supports Bedrock, Vertex AI, and Foundry through separate dependencies that provide platform-specific `Backend` implementations:

- **Bedrock:** `com.anthropic:anthropic-java-bedrock`: Use `BedrockMantleBackend.fromEnv()` for the Messages-API Bedrock endpoint, or `BedrockBackend.fromEnv()` / `BedrockBackend.builder()` (`bedrock-runtime` path).
- **Vertex AI:** `com.anthropic:anthropic-java-vertex`: Use `VertexBackend.fromEnv()` or `VertexBackend.builder()`.
- **Foundry:** `com.anthropic:anthropic-java-foundry`: Use `FoundryBackend.fromEnv()` or `FoundryBackend.builder()`.

Use `BedrockMantleBackend` for new projects; `BedrockBackend` remains for existing applications using the Bedrock `InvokeModel` API.

Each backend is passed to the client via `.backend()` on `AnthropicOkHttpClient.builder()`. AWS, Google Cloud, and Azure classes are included as transitive dependencies of the respective library.

### Advanced usage

#### Raw response access

To access HTTP headers, status codes, and the raw response body, prefix any HTTP method call with `withRawResponse()`:

```java nocheck
import com.anthropic.core.http.Headers;
import com.anthropic.core.http.HttpResponseFor;
import com.anthropic.models.messages.Message;
import com.anthropic.models.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

MessageCreateParams params = MessageCreateParams.builder()
  .maxTokens(1024L)
  .addUserMessage("Hello, Claude")
  .model(Model.CLAUDE_OPUS_4_7)
  .build();

HttpResponseFor<Message> message = client.messages().withRawResponse().create(params);

int statusCode = message.statusCode();

Headers headers = message.headers();
```

You can still deserialize the response into an instance of a Java class if needed:

```java nocheck
import com.anthropic.models.messages.Message;

Message parsedMessage = message.parse();
```

#### Logging

The SDK uses the standard OkHttp logging interceptor.

Enable logging by setting the `ANTHROPIC_LOG` environment variable to `info`:

```bash
export ANTHROPIC_LOG=info
```

Or to `debug` for more verbose logging:

```bash
export ANTHROPIC_LOG=debug
```

<section title="Jackson compatibility">

The SDK depends on Jackson for JSON serialization/deserialization. It is compatible with version 2.13.4 or higher, but depends on version 2.18.2 by default.

The SDK throws an exception if it detects an incompatible Jackson version at runtime (e.g. if the default version was overridden in your Maven or Gradle config).

If the SDK threw an exception, but you're certain the version is compatible, then disable the version check using `checkJacksonVersionCompatibility` on `AnthropicOkHttpClient` or `AnthropicOkHttpClientAsync`.

> There is no guarantee that the SDK works correctly when the Jackson version check is disabled.

There are also bugs in older Jackson versions that can affect the SDK. The SDK doesn't work around all Jackson bugs and expects users to upgrade Jackson for those instead.

</section>

<section title="ProGuard/R8 configuration">

Although the SDK uses reflection, it is still usable with ProGuard and R8 because `anthropic-java-core` is published with a configuration file containing keep rules.

ProGuard and R8 should automatically detect and use the published rules, but you can also manually copy the keep rules if necessary.

</section>

#### Undocumented API functionality

The SDK is typed for convenient usage of the documented API. However, it also supports working with undocumented or not yet supported parts of the API.

##### Undocumented endpoints

To make requests to undocumented endpoints, you can use the `putAdditionalHeader`, `putAdditionalQueryParam`, or `putAdditionalBodyProperty` methods as described in [Undocumented parameters](#undocumented-parameters).

##### Undocumented response properties

To access undocumented response properties, use the `_additionalProperties()` method as described in [Response properties](#response-properties).

### Beta features

Beta features are available before general release to get early feedback and test new functionality. You can check the availability of all of Claude's capabilities and tools in the [build with Claude overview](https://platform.claude.com/docs/en/build-with-claude/overview).

You can access most beta API features through the `beta()` method on the client. To enable a particular beta feature, add the appropriate [beta header](https://platform.claude.com/docs/en/api/beta-headers) with `.addBeta()` when building the message params.

For example, to use the [Files API](https://platform.claude.com/docs/en/build-with-claude/files):

```java nocheck hidelines={1..2,9..10}
import com.anthropic.client.AnthropicClient;
import com.anthropic.client.okhttp.AnthropicOkHttpClient;
import com.anthropic.models.beta.AnthropicBeta;
import com.anthropic.models.beta.messages.BetaContentBlockParam;
import com.anthropic.models.beta.messages.BetaMessage;
import com.anthropic.models.beta.messages.BetaRequestDocumentBlock;
import com.anthropic.models.beta.messages.BetaTextBlockParam;
import com.anthropic.models.beta.messages.MessageCreateParams;
import com.anthropic.models.messages.Model;

void main() {
    AnthropicClient client = AnthropicOkHttpClient.fromEnv();

    BetaMessage message = client.beta().messages().create(
        MessageCreateParams.builder()
            .model(Model.CLAUDE_OPUS_4_7)
            .maxTokens(1024L)
            .addBeta(AnthropicBeta.FILES_API_2025_04_14)
            .addUserMessageOfBetaContentBlockParams(List.of(
                BetaContentBlockParam.ofText(
                    BetaTextBlockParam.builder()
                        .text("Please summarize this document for me.")
                        .build()),
                BetaContentBlockParam.ofDocument(
                    BetaRequestDocumentBlock.builder()
                        .fileSource("file_abc123")
                        .build())))
            .build());
}
```

### Frequently asked questions

<section title="Why doesn't the SDK use plain enum classes?">

Java `enum` classes are not trivially forwards compatible. Using them in the SDK could cause runtime exceptions if the API is updated to respond with a new enum value.

</section>

<section title="Why are fields represented using JsonField<T> instead of just plain T?">

Using `JsonField<T>` enables a few features:

- Allowing usage of undocumented API functionality
- Lazily validating the API response against the expected shape
- Representing absent vs explicitly null values

</section>

<section title="Why doesn't the SDK use data classes?">

It is not backwards compatible to add new fields to a data class, and the SDK avoids introducing a breaking change every time a field is added to a class.

</section>

<section title="Why doesn't the SDK use checked exceptions?">

Checked exceptions are widely considered a mistake in the Java programming language. In fact, they were omitted from Kotlin for this reason.

Checked exceptions:

- Are verbose to handle
- Encourage error handling at the wrong level of abstraction, where nothing can be done about the error
- Are tedious to propagate due to the function coloring problem
- Don't play well with lambdas (also due to the function coloring problem)

</section>

### Semantic versioning

This package generally follows [SemVer](https://semver.org/spec/v2.0.0.html) conventions, though certain backwards-incompatible changes may be released as minor versions:

1. Changes to library internals which are technically public but not intended or documented for external use.
2. Changes that aren't expected to impact the vast majority of users in practice.

### Additional resources

- [GitHub repository](https://github.com/anthropics/anthropic-sdk-java)
- [Javadocs](https://javadoc.io/doc/com.anthropic/anthropic-java)
- [API reference](https://platform.claude.com/docs/en/api/overview)
- [Streaming guide](https://platform.claude.com/docs/en/build-with-claude/streaming)
- [Tool use guide](https://platform.claude.com/docs/en/agents-and-tools/tool-use/overview)

---

## Go SDK

Install and configure the Anthropic Go SDK with context-based cancellation and functional options

---

The Anthropic Go library provides convenient access to the Anthropic REST API from applications written in Go.

> For API feature documentation with code examples, see the [API reference](https://platform.claude.com/docs/en/api/overview). This page covers Go-specific SDK features and configuration.

### Installation

```go nocheck
import (
	"github.com/anthropics/anthropic-sdk-go" // imported as anthropic
)
```

Or to pin the version:

```bash
go get -u 'github.com/anthropics/anthropic-sdk-go@v1.27.1'
```

### Requirements

This library requires Go 1.23+.

### Usage

```go nocheck
package main

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func main() {
	client := anthropic.NewClient(
		option.WithAPIKey("my-anthropic-api-key"), // defaults to os.LookupEnv("ANTHROPIC_API_KEY")
	)
	message, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What is a quaternion?")),
		},
		Model: anthropic.ModelClaudeOpus4_7,
	})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("%+v\n", message.Content)
}
```

<section title="Conversations">

```go
messages := []anthropic.MessageParam{
	anthropic.NewUserMessage(anthropic.NewTextBlock("What is my first name?")),
}

message, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
	Model:     anthropic.ModelClaudeOpus4_7,
	Messages:  messages,
	MaxTokens: 1024,
})
if err != nil {
	panic(err)
}

fmt.Printf("%+v\n", message.Content)

messages = append(messages, message.ToParam())
messages = append(messages, anthropic.NewUserMessage(
	anthropic.NewTextBlock("My full name is John Doe"),
))

message, err = client.Messages.New(context.TODO(), anthropic.MessageNewParams{
	Model:     anthropic.ModelClaudeOpus4_7,
	Messages:  messages,
	MaxTokens: 1024,
})

fmt.Printf("%+v\n", message.Content)
```

</section>
<section title="System prompts">

```go hidelines={1,10..11}
messages := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("Hello"))}
message, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
	Model:     anthropic.ModelClaudeOpus4_7,
	MaxTokens: 1024,
	System: []anthropic.TextBlockParam{
		{Text: "Be very serious at all times."},
	},
	Messages: messages,
})
_ = message
_ = err
```

</section>
<section title="Streaming">

```go
content := "What is a quaternion?"

stream := client.Messages.NewStreaming(context.TODO(), anthropic.MessageNewParams{
	Model:     anthropic.ModelClaudeOpus4_7,
	MaxTokens: 1024,
	Messages: []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(content)),
	},
})

message := anthropic.Message{}
for stream.Next() {
	event := stream.Current()
	err := message.Accumulate(event)
	if err != nil {
		panic(err)
	}

	switch eventVariant := event.AsAny().(type) {
	case anthropic.ContentBlockDeltaEvent:
		switch deltaVariant := eventVariant.Delta.AsAny().(type) {
		case anthropic.TextDelta:
			print(deltaVariant.Text)
		}

	}
}

if stream.Err() != nil {
	panic(stream.Err())
}
```

</section>
<section title="Tool calling">

```go hidelines={1..18,99..135}
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
)

func main() {
	client := anthropic.NewClient()

	content := "Where is San Francisco?"

	println("[user]: " + content)

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(content)),
	}

	toolParams := []anthropic.ToolParam{
		{
			Name:        "get_coordinates",
			Description: anthropic.String("Accepts a place as an address, then returns the latitude and longitude coordinates."),
			InputSchema: GetCoordinatesInputSchema,
		},
	}
	tools := make([]anthropic.ToolUnionParam, len(toolParams))
	for i, toolParam := range toolParams {
		tools[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
	}

	for {
		message, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
			Model:     anthropic.ModelClaudeOpus4_7,
			MaxTokens: 1024,
			Messages:  messages,
			Tools:     tools,
		})

		if err != nil {
			panic(err)
		}

		print(color("[assistant]: "))
		for _, block := range message.Content {
			switch block := block.AsAny().(type) {
			case anthropic.TextBlock:
				println(block.Text)
				println()
			case anthropic.ToolUseBlock:
				inputJSON, _ := json.Marshal(block.Input)
				println(block.Name + ": " + string(inputJSON))
				println()
			}
		}

		messages = append(messages, message.ToParam())
		toolResults := []anthropic.ContentBlockParamUnion{}

		for _, block := range message.Content {
			switch variant := block.AsAny().(type) {
			case anthropic.ToolUseBlock:
				print(color("[user (" + block.Name + ")]: "))

				var response interface{}
				switch block.Name {
				case "get_coordinates":
					var input struct {
						Location string `json:"location"`
					}

					err := json.Unmarshal([]byte(variant.JSON.Input.Raw()), &input)
					if err != nil {
						panic(err)
					}

					response = GetCoordinates(input.Location)
				}

				b, err := json.Marshal(response)
				if err != nil {
					panic(err)
				}

				println(string(b))

				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, string(b), false))
			}

		}
		if len(toolResults) == 0 {
			break
		}
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}
}

type GetCoordinatesInput struct {
	Location string `json:"location" jsonschema_description:"The location to look up."`
}

var GetCoordinatesInputSchema = GenerateSchema[GetCoordinatesInput]()

type GetCoordinateResponse struct {
	Long float64 `json:"long"`
	Lat  float64 `json:"lat"`
}

func GetCoordinates(location string) GetCoordinateResponse {
	return GetCoordinateResponse{
		Long: -122.4194,
		Lat:  37.7749,
	}
}

func GenerateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T

	schema := reflector.Reflect(v)

	return anthropic.ToolInputSchemaParam{
		Properties: schema.Properties,
	}
}

func color(s string) string {
	return fmt.Sprintf("\033[1;%sm%s\033[0m", "33", s)
}
```

</section>

### Request fields

The anthropic library uses the [`omitzero`](https://tip.golang.org/doc/go1.24#encodingjsonpkgencodingjson)
semantics from the Go 1.24+ `encoding/json` release for request fields.

Required primitive fields (`int64`, `string`, etc.) feature the tag `` `json:"...,required"` ``. These
fields are always serialized, even their zero values.

Optional primitive types are wrapped in a `param.Opt[T]`. These fields can be set with the provided constructors, `anthropic.String(string)`, `anthropic.Int(int64)`, etc.

Any `param.Opt[T]`, map, slice, struct or string enum uses the
tag `` `json:"...,omitzero"` ``. Its zero value is considered omitted.

The `param.IsOmitted(any)` function can confirm the presence of any `omitzero` field.

```go nocheck
p := anthropic.ExampleParams{
	ID:   "id_xxx",                // required property
	Name: anthropic.String("..."), // optional property

	Point: anthropic.Point{
		X: 0,                // required field will serialize as 0
		Y: anthropic.Int(1), // optional field will serialize as 1
		// ... omitted non-required fields will not be serialized
	},

	Origin: anthropic.Origin{}, // the zero value of [Origin] is considered omitted
}
```

To send `null` instead of a `param.Opt[T]`, use `param.Null[T]()`.
To send `null` instead of a struct `T`, use `param.NullStruct[T]()`.

```go nocheck
p.Name = param.Null[string]()       // 'null' instead of string
p.Point = param.NullStruct[Point]() // 'null' instead of struct

param.IsNull(p.Name)  // true
param.IsNull(p.Point) // true
```

Request structs contain a `.SetExtraFields(map[string]any)` method which can send non-conforming
fields in the request body. Extra fields overwrite any struct fields with a matching
key.

> For security reasons, only use `SetExtraFields` with trusted data.

To send a custom value instead of a struct, use the generic function `param.Override` (for example, `param.Override[anthropic.FooParams](12)`).

```go nocheck
// In cases where the API specifies a given type,
// but you want to send something else, use [SetExtraFields]:
p.SetExtraFields(map[string]any{
	"x": 0.01, // send "x" as a float instead of int
})

// Send a number instead of an object
custom := param.Override[anthropic.FooParams](12)
```

#### Request unions

Unions are represented as a struct with fields prefixed by "Of" for each of its variants,
only one field can be non-zero. The non-zero field will be serialized.

Sub-properties of the union can be accessed via methods on the union struct.
These methods return a mutable pointer to the underlying data, if present.

```go nocheck
// Only one field can be non-zero, use param.IsOmitted() to check if a field is set
type AnimalUnionParam struct {
	OfCat *Cat `json:",omitzero,inline`
	OfDog *Dog `json:",omitzero,inline`
}

animal := AnimalUnionParam{
	OfCat: &Cat{
		Name: "Whiskers",
		Owner: PersonParam{
			Address: AddressParam{Street: "3333 Coyote Hill Rd", Zip: 0},
		},
	},
}

// Mutating a field
if address := animal.GetOwner().GetAddress(); address != nil {
	address.ZipCode = 94304
}
```

#### Deserializing params

> `param.SetJSON` requires SDK v1.20.0 or later.

Param types (types ending in `Param`, such as `MessageNewParams` or `ToolUnionParam`) are designed for outgoing requests only. They marshal correctly to JSON but do not fully support round-trip deserialization. If you unmarshal raw JSON into a param struct, typed union fields like `OfBashTool20250124` will be nil even when the underlying JSON is valid.

If you need to reconstruct params from raw JSON (for example, from a database, middleware, or a previous request), call `UnmarshalJSON` to populate non-union fields, then use `param.SetJSON` to attach the raw bytes for correct re-serialization:

```go hidelines={1..24,44}
package main

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

func main() {
	original := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_7,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("hello")),
		},
		Tools: []anthropic.ToolUnionParam{{
			OfBashTool20250124: &anthropic.ToolBash20250124Param{
				Type: "bash_20250124",
				Name: "bash",
			},
		}},
	}
	// Serialize params (for example, for storage or forwarding)
	b, err := json.Marshal(original)
	if err != nil {
		panic(err)
	}

	// Later, reconstruct params from the stored JSON
	var params anthropic.MessageNewParams
	if err := params.UnmarshalJSON(b); err != nil {
		panic(err)
	}
	param.SetJSON(b, &params)

	// params.Model and other scalar fields are populated by UnmarshalJSON.
	// params.Tools[0].OfBashTool20250124 is nil (the union limitation),
	// but the raw JSON is preserved. When params is marshaled again
	// for the API call, the tools serialize correctly.
	b2, _ := json.Marshal(params)
	fmt.Println(string(b) == string(b2)) // true
}
```

For this use case, `param.SetJSON` (available since v1.20.0) is preferred over the more general `param.Override[T](any)` because it doesn't require spelling out the type parameter and makes the round-trip intent explicit.

### Response objects

All fields in response structs are ordinary value types (not pointers or wrappers).
Response structs also include a special `JSON` field containing metadata about
each property.

```go nocheck
type Animal struct {
	Name   string `json:"name,nullable"`
	Owners int    `json:"owners"`
	Age    int    `json:"age"`
	JSON   struct {
		Name        respjson.Field
		Owner       respjson.Field
		Age         respjson.Field
		ExtraFields map[string]respjson.Field
	} `json:"-"`
}
```

To handle optional data, use the `.Valid()` method on the JSON field.
`.Valid()` returns true if a field is not `null`, not present, or couldn't be marshaled.

If `.Valid()` is false, the corresponding field will be its zero value.

```go nocheck
raw := `{"owners": 1, "name": null}`

var res Animal
json.Unmarshal([]byte(raw), &res)

// Accessing regular fields

res.Owners // 1
res.Name   // ""
res.Age    // 0

// Optional field checks

res.JSON.Owners.Valid() // true
res.JSON.Name.Valid()   // false
res.JSON.Age.Valid()    // false

// Raw JSON values

res.JSON.Owners.Raw()                  // "1"
res.JSON.Name.Raw() == "null"          // true
res.JSON.Name.Raw() == respjson.Null   // true
res.JSON.Age.Raw() == ""               // true
res.JSON.Age.Raw() == respjson.Omitted // true
```

These `.JSON` structs also include an `ExtraFields` map containing
any properties in the json response that were not specified
in the struct. This can be useful for API features not yet
present in the SDK.

```go nocheck
body := res.JSON.ExtraFields["my_unexpected_field"].Raw()
```

#### Response unions

In responses, unions are represented by a flattened struct containing all possible fields from each of the
object variants.
To convert it to a variant use the `.AsFooVariant()` method or the `.AsAny()` method if present.

If a response value union contains primitive values, primitive fields will be alongside
the properties but prefixed with `Of` and feature the tag `json:"...,inline"`.

```go nocheck
type AnimalUnion struct {
	// From variants [Dog], [Cat]
	Owner Person `json:"owner"`
	// From variant [Dog]
	DogBreed string `json:"dog_breed"`
	// From variant [Cat]
	CatBreed string `json:"cat_breed"`
	// ...

	JSON struct {
		Owner respjson.Field
		// ...
	} `json:"-"`
}

// If animal variant
if animal.Owner.Address.ZipCode == "" {
	panic("missing zip code")
}

// Switch on the variant
switch variant := animal.AsAny().(type) {
case Dog:
case Cat:
default:
	panic("unexpected type")
}
```

### Error handling

When the API returns a non-success status code, the SDK returns an error with type
`*anthropic.Error`. This contains the `StatusCode`, `*http.Request`, and
`*http.Response` values of the request, as well as the JSON of the error body
(much like other response objects in the SDK). The error also includes the `RequestID`
from the response headers, which is useful for troubleshooting with Anthropic support.

To handle errors, use the `errors.As` pattern:

```go hidelines={1..11,33}
package main

import (
	"context"
	"errors"

	"github.com/anthropics/anthropic-sdk-go"
)

func main() {
	client := anthropic.NewClient()
	_, err := client.Messages.New(context.TODO(), anthropic.MessageNewParams{
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{{
			Content: []anthropic.ContentBlockParamUnion{{
				OfText: &anthropic.TextBlockParam{
					Text: "What is a quaternion?",
				},
			}},
			Role: anthropic.MessageParamRoleUser,
		}},
		Model: anthropic.ModelClaudeOpus4_7,
	})
	if err != nil {
		var apierr *anthropic.Error
		if errors.As(err, &apierr) {
			println("Request ID:", apierr.RequestID)
			println(string(apierr.DumpRequest(true)))  // Prints the serialized HTTP request
			println(string(apierr.DumpResponse(true))) // Prints the serialized HTTP response
		}
		panic(err.Error()) // GET "/v1/messages": 400 Bad Request (Request-ID: req_xxx) { ... }
	}
}
```

When other errors occur, they are returned unwrapped; for example,
if HTTP transport fails, you might receive `*url.Error` wrapping `*net.OpError`.

### Retries

Certain errors will be automatically retried 2 times by default, with a short exponential backoff.
The SDK retries by default all connection errors, 408 Request Timeout, 409 Conflict, 429 Rate Limit,
and >=500 Internal errors.

You can use the `WithMaxRetries` option to configure or disable this:

```go hidelines={1..10,17,34}
package main

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func main() {
	// Configure the default for all requests:
	client := anthropic.NewClient(
		option.WithMaxRetries(0), // default is 2
	)

	// Override per-request:
	_, _ =
		client.Messages.New(
			context.TODO(),
			anthropic.MessageNewParams{
				MaxTokens: 1024,
				Messages: []anthropic.MessageParam{{
					Content: []anthropic.ContentBlockParamUnion{{
						OfText: &anthropic.TextBlockParam{
							Text: "What is a quaternion?",
						},
					}},
					Role: anthropic.MessageParamRoleUser,
				}},
				Model: anthropic.ModelClaudeOpus4_7,
			},
			option.WithMaxRetries(5),
		)
}
```

### Timeouts

Requests do not time out by default; use context to configure a timeout for a request lifecycle.

Note that if a request is [retried](#retries), the context timeout does not start over.
To set a per-retry timeout, use `option.WithRequestTimeout()`.

```go hidelines={1..12,16,34}
package main

import (
	"context"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func main() {
	client := anthropic.NewClient()
	// This sets the timeout for the request, including all the retries.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	_, _ =
		client.Messages.New(
			ctx,
			anthropic.MessageNewParams{
				MaxTokens: 1024,
				Messages: []anthropic.MessageParam{{
					Content: []anthropic.ContentBlockParamUnion{{
						OfText: &anthropic.TextBlockParam{
							Text: "What is a quaternion?",
						},
					}},
					Role: anthropic.MessageParamRoleUser,
				}},
				Model: anthropic.ModelClaudeOpus4_7,
			},
			// This sets the per-retry timeout
			option.WithRequestTimeout(20*time.Second),
		)
}
```

### Long requests

> Consider using the streaming Messages API for longer running requests.

Avoid setting a large `MaxTokens` value without using streaming as some networks may drop idle connections after a certain period of time, which
can cause the request to fail or [timeout](#timeouts) without receiving a response from Anthropic.

This SDK will also return an error if a non-streaming request is expected to be above roughly 10 minutes long.
Calling `.Messages.NewStreaming()` or [setting a custom timeout](#timeouts) disables this error.

### File uploads

Request parameters that correspond to file uploads in multipart requests are typed as
`io.Reader`. The contents of the `io.Reader` will by default be sent as a multipart form
part with the file name of "anonymous_file" and content-type of "application/octet-stream", so the recommended approach is to specify a custom content-type with the `anthropic.File(reader io.Reader, filename string, contentType string)`
helper, which wraps any `io.Reader` with the appropriate file name and content type.

```go nocheck
// A file from the file system
file, err := os.Open("/path/to/file.json")
anthropic.BetaFileUploadParams{
	File:  anthropic.File(file, "custom-name.json", "application/json"),
	Betas: []anthropic.AnthropicBeta{anthropic.AnthropicBetaFilesAPI2025_04_14},
}

// A file from a string
anthropic.BetaFileUploadParams{
	File:  anthropic.File(strings.NewReader("my file contents"), "custom-name.json", "application/json"),
	Betas: []anthropic.AnthropicBeta{anthropic.AnthropicBetaFilesAPI2025_04_14},
}
```

The file name and content-type can also be customized by implementing `Name() string` or `ContentType()
string` on the run-time type of `io.Reader`. Note that `os.File` implements `Name() string`, so a
file returned by `os.Open` will be sent with the file name on disk.

### Pagination

This library provides some conveniences for working with paginated list endpoints.

You can use `.ListAutoPaging()` methods to iterate through items across all pages:

```go
iter := client.Messages.Batches.ListAutoPaging(context.TODO(), anthropic.MessageBatchListParams{
	Limit: anthropic.Int(20),
})
// Automatically fetches more pages as needed.
for iter.Next() {
	messageBatch := iter.Current()
	fmt.Printf("%+v\n", messageBatch)
}
if err := iter.Err(); err != nil {
	panic(err.Error())
}
```

Or you can use simple `.List()` methods to fetch a single page and receive a standard response object
with additional helper methods like `.GetNextPage()`:

```go
page, err := client.Messages.Batches.List(context.TODO(), anthropic.MessageBatchListParams{
	Limit: anthropic.Int(20),
})
for page != nil {
	for _, batch := range page.Data {
		fmt.Printf("%+v\n", batch)
	}
	page, err = page.GetNextPage()
}
if err != nil {
	panic(err.Error())
}
```

### RequestOptions

This library uses the functional options pattern. Functions defined in the
`option` package return a `RequestOption`, which is a closure that mutates a
`RequestConfig`. These options can be supplied to the client or at individual
requests. For example:

```go nocheck
client := anthropic.NewClient(
	// Adds a header to every request made by the client
	option.WithHeader("X-Some-Header", "custom_header_info"),
)

client.Messages.New(context.TODO(), // ...,
	// Override the header
	option.WithHeader("X-Some-Header", "some_other_custom_header_info"),
	// Add an undocumented field to the request body, using sjson syntax
	option.WithJSONSet("some.json.path", map[string]string{"my": "object"}),
)
```

The request option `option.WithDebugLog(nil)` may be helpful while debugging.

See the [full list of request options](https://pkg.go.dev/github.com/anthropics/anthropic-sdk-go/option).

### HTTP client customization

#### Middleware

The SDK provides `option.WithMiddleware`, which applies the given
middleware to requests.

```go hidelines={1..16,32..33}
package main

import (
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

var _ = anthropic.ModelClaudeOpus4_7

func LogReq(req *http.Request)                              {}
func LogRes(res *http.Response, err error, d time.Duration) {}

func main() {
	client := anthropic.NewClient(
		option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (res *http.Response, err error) {
			// Before the request
			start := time.Now()
			LogReq(req)

			// Forward the request to the next handler
			res, err = next(req)

			// Handle stuff after the request
			LogRes(res, err, time.Since(start))

			return res, err
		}),
	)
	_ = client
}
```

When multiple middlewares are provided as variadic arguments, the middlewares
are applied left to right. If `option.WithMiddleware` is given
multiple times, for example first in the client then the method, the
middleware in the client will run first and the middleware given in the method
will run next.

You may also replace the default `http.Client` with
`option.WithHTTPClient(client)`. Only one http client is
accepted (this overwrites any previous client) and receives requests after any
middleware has been applied.

### Platform integrations

> For detailed platform setup guides with code examples, see:
> - [Amazon Bedrock](https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock)
> - [Amazon Bedrock (legacy)](https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock)
> - [Google Vertex AI](https://platform.claude.com/docs/en/build-with-claude/claude-on-vertex-ai)

The Go SDK supports Amazon Bedrock and Google Vertex AI through subpackages:

- **Bedrock:** `import "github.com/anthropics/anthropic-sdk-go/bedrock"`. Use `bedrock.NewMantleClient` for the Messages-API Bedrock endpoint (streams over SSE), or `bedrock.WithLoadDefaultConfig(ctx)` / `bedrock.WithConfig(cfg)` (`bedrock-runtime` path). Importing the `bedrock` package globally registers a decoder for `application/vnd.amazon.eventstream` with the SDK's streaming layer (through package `init()`). This applies whether you use the `bedrock-runtime` `WithConfig`/`WithLoadDefaultConfig` path or `NewMantleClient`.
- **Vertex AI:** `import "github.com/anthropics/anthropic-sdk-go/vertex"`. Use `vertex.WithGoogleAuth(ctx, region, projectID)` or `vertex.WithCredentials(ctx, region, projectID, creds)`.

Use `bedrock.NewMantleClient` for new projects; `bedrock.WithLoadDefaultConfig`/`WithConfig` remain for existing applications using the Bedrock `InvokeModel` API.

### Advanced usage

#### Accessing raw response data (for example, response headers)

You can access the raw HTTP response data by using the `option.WithResponseInto()` request option. This is useful when
you need to examine response headers, status codes, or other details.

```go hidelines={1..13,39}
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func main() {
	client := anthropic.NewClient()
	// Create a variable to store the HTTP response
	var response *http.Response
	message, err := client.Messages.New(
		context.TODO(),
		anthropic.MessageNewParams{
			MaxTokens: 1024,
			Messages: []anthropic.MessageParam{{
				Content: []anthropic.ContentBlockParamUnion{{
					OfText: &anthropic.TextBlockParam{
						Text: "What is a quaternion?",
					},
				}},
				Role: anthropic.MessageParamRoleUser,
			}},
			Model: anthropic.ModelClaudeOpus4_7,
		},
		option.WithResponseInto(&response),
	)
	if err != nil {
		// handle error
	}
	fmt.Printf("%+v\n", message)

	fmt.Printf("Status Code: %d\n", response.StatusCode)
	fmt.Printf("Headers: %+#v\n", response.Header)
}
```

#### Making custom/undocumented requests

This library is typed for convenient access to the documented API. If you need to access undocumented
endpoints, params, or response properties, the library can still be used.

##### Undocumented endpoints

To make requests to undocumented endpoints, you can use `client.Get`, `client.Post`, and other HTTP verbs.
`RequestOptions` on the client, such as retries, will be respected when making these requests.

```go nocheck
var (
	// params can be an io.Reader, a []byte, an encoding/json serializable object,
	// or a "...Params" struct defined in this library.
	params map[string]any

	// result can be an []byte, *http.Response, a encoding/json deserializable object,
	// or a model defined in this library.
	result *http.Response
)
err := client.Post(context.Background(), "/unspecified", params, &result)
if err != nil {
	// ...
}
```

##### Undocumented request params

To make requests using undocumented parameters, you may use either the `option.WithQuerySet()`
or the `option.WithJSONSet()` methods.

```go nocheck
params := FooNewParams{
	ID: "id_xxxx",
	Data: FooNewParamsData{
		FirstName: anthropic.String("John"),
	},
}
client.Foo.New(context.Background(), params, option.WithJSONSet("data.last_name", "Doe"))
```

##### Undocumented response properties

To access undocumented response properties, you may either access the raw JSON of the response as a string
with `result.JSON.RawJSON()`, or get the raw JSON of a particular field on the result with
`result.JSON.Foo.Raw()`.

Any fields that are not present on the response struct will be saved and can be accessed by `result.JSON.ExtraFields()` which returns the extra fields as a `map[string]Field`.

### Semantic versioning

This package generally follows [SemVer](https://semver.org/spec/v2.0.0.html) conventions, though certain backwards-incompatible changes may be released as minor versions:

1. Changes to library internals which are technically public but not intended or documented for external use. _(Please open a GitHub issue to let the maintainers know if you're relying on such internals.)_
2. Changes that aren't expected to impact the vast majority of users in practice.

Backwards-compatibility is taken seriously to ensure you can rely on a smooth upgrade experience.

Your feedback is welcome; please open an [issue](https://www.github.com/anthropics/anthropic-sdk-go/issues) with questions, bugs, or suggestions.

### Additional resources

- [GitHub repository](https://github.com/anthropics/anthropic-sdk-go)
- [Go package documentation](https://pkg.go.dev/github.com/anthropics/anthropic-sdk-go)
- [API reference](https://platform.claude.com/docs/en/api/overview)
- [Streaming guide](https://platform.claude.com/docs/en/build-with-claude/streaming)

---

## Ruby SDK

Install and configure the Anthropic Ruby SDK with Sorbet types, streaming helpers, and connection pooling

---

The Anthropic Ruby library provides convenient access to the Anthropic REST API from any Ruby 3.2.0+ application. It ships with comprehensive types and docstrings in Yard, RBS, and RBI. The standard library's `net/http` is used as the HTTP transport, with connection pooling via the `connection_pool` gem.

> For API feature documentation with code examples, see the [API reference](https://platform.claude.com/docs/en/api/overview). This page covers Ruby-specific SDK features and configuration.

### Installation

Add the gem to your application's `Gemfile` with Bundler:

```bash
bundle add anthropic
```

### Requirements

Ruby 3.2.0 or higher.

### Usage

```ruby hidelines={1..2}
require "anthropic"

anthropic = Anthropic::Client.new(
  api_key: ENV["ANTHROPIC_API_KEY"] # This is the default and can be omitted
)

message = anthropic.messages.create(
  max_tokens: 1024,
  messages: [{role: "user", content: "Hello, Claude"}],
  model: :"claude-opus-4-7"
)

puts(message.content)
```

### Streaming

The SDK provides support for streaming responses using Server-Sent Events (SSE).

```ruby hidelines={1}
require "anthropic"
anthropic = Anthropic::Client.new
stream = anthropic.messages.stream(
  max_tokens: 1024,
  messages: [{role: "user", content: "Hello, Claude"}],
  model: :"claude-opus-4-7"
)

stream.each do |message|
  puts(message.type)
end
```

#### Streaming helpers

This library provides several conveniences for streaming messages, for example:

```ruby hidelines={1}
require "anthropic"
anthropic = Anthropic::Client.new
stream = anthropic.messages.stream(
  max_tokens: 1024,
  messages: [{role: :user, content: "Say hello there!"}],
  model: :"claude-opus-4-7"
)

stream.text.each do |text|
  print(text)
end
```

Streaming with `anthropic.messages.stream(...)` exposes various helpers including accumulation and SDK-specific events.

### Input schema and tool calling

The SDK provides helper mechanisms to define structured data classes for tools and let Claude automatically execute them. For detailed documentation on tool use patterns including the tool runner, see [Tool Runner (SDK)](https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-runner).

```ruby hidelines={1}
require "anthropic"
anthropic = Anthropic::Client.new
class CalculatorInput < Anthropic::BaseModel
  required :lhs, Float
  required :rhs, Float
  required :operator, Anthropic::InputSchema::EnumOf[:+, :-, :*, :/]
end

class Calculator < Anthropic::BaseTool
  input_schema CalculatorInput

  def call(expr)
    expr.lhs.public_send(expr.operator, expr.rhs)
  end
end

## Automatically handles tool execution loop
anthropic.beta.messages.tool_runner(
  model: "claude-opus-4-7",
  max_tokens: 1024,
  messages: [{role: "user", content: "What's 15 * 7?"}],
  tools: [Calculator.new]
).each_message { puts _1.content }
```

### Structured outputs

For complete structured outputs documentation including Ruby examples, see [Structured Outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs).

### Handling errors

When the library is unable to connect to the API, or if the API returns a non-success status code (i.e., 4xx or 5xx response), a subclass of `Anthropic::Errors::APIError` will be thrown:

```ruby hidelines={1}
require "anthropic"
anthropic = Anthropic::Client.new
begin
  message = anthropic.messages.create(
    max_tokens: 1024,
    messages: [{role: "user", content: "Hello, Claude"}],
    model: :"claude-opus-4-7"
  )
rescue Anthropic::Errors::APIConnectionError => e
  puts("The server could not be reached")
  puts(e.cause)  # an underlying Exception, likely raised within `net/http`
rescue Anthropic::Errors::RateLimitError => e
  puts("A 429 status code was received; we should back off a bit.")
rescue Anthropic::Errors::APIStatusError => e
  puts("Another non-200-range status code was received")
  puts(e.status)
end
```

Error codes are as follows:

| Cause            | Error Type                 |
| ---------------- | -------------------------- |
| HTTP 400         | `BadRequestError`          |
| HTTP 401         | `AuthenticationError`      |
| HTTP 403         | `PermissionDeniedError`    |
| HTTP 404         | `NotFoundError`            |
| HTTP 409         | `ConflictError`            |
| HTTP 422         | `UnprocessableEntityError` |
| HTTP 429         | `RateLimitError`           |
| HTTP >= 500      | `InternalServerError`      |
| Other HTTP error | `APIStatusError`           |
| Timeout          | `APITimeoutError`          |
| Network error    | `APIConnectionError`       |

### Retries

Certain errors will be automatically retried 2 times by default, with a short exponential backoff.

Connection errors (for example, due to a network connectivity problem), 408 Request Timeout, 409 Conflict, 429 Rate Limit, >=500 Internal errors, and timeouts will all be retried by default.

You can use the `max_retries` option to configure or disable this:

```ruby
## Configure the default for all requests:
anthropic = Anthropic::Client.new(
  max_retries: 0 # default is 2
)

## Or, configure per-request:
anthropic.messages.create(
  max_tokens: 1024,
  messages: [{role: "user", content: "Hello, Claude"}],
  model: :"claude-opus-4-7",
  request_options: {max_retries: 5}
)
```

### Timeouts

By default, requests will time out after 600 seconds. You can use the timeout option to configure or disable this:

```ruby
## Configure the default for all requests:
anthropic = Anthropic::Client.new(
  timeout: nil # default is 600
)

## Or, configure per-request:
anthropic.messages.create(
  max_tokens: 1024,
  messages: [{role: "user", content: "Hello, Claude"}],
  model: :"claude-opus-4-7",
  request_options: {timeout: 5}
)
```

On timeout, `Anthropic::Errors::APITimeoutError` is raised.

Note that requests that time out are retried by default.

### Pagination

List methods in the Claude API are paginated.

This library provides auto-paginating iterators with each list response, so you do not have to request successive pages manually:

```ruby hidelines={1}
require "anthropic"
anthropic = Anthropic::Client.new
page = anthropic.messages.batches.list(limit: 20)

## Fetch single item from page.
batch = page.data[0]
puts(batch.id)

## Automatically fetches more pages as needed.
page.auto_paging_each do |batch|
  puts(batch.id)
end
```

Alternatively, you can use the `#next_page?` and `#next_page` methods for more granular control working with pages.

```ruby hidelines={1}
require "anthropic"
anthropic = Anthropic::Client.new
page = anthropic.messages.batches.list(limit: 20)
while page.next_page?
  page = page.next_page
  page.data&.each { |batch| puts(batch.id) }
end
```

### File uploads

Request parameters that correspond to file uploads can be passed as raw contents, a [`Pathname`](https://rubyapi.org/3.2/o/pathname) instance, [`StringIO`](https://rubyapi.org/3.2/o/stringio), or more.

```ruby hidelines={1} nocheck
require "anthropic"
anthropic = Anthropic::Client.new
require "pathname"

## Use `Pathname` to send the filename and/or avoid paging a large file into memory:
file_metadata = anthropic.beta.files.upload(file: Pathname("/path/to/file"))

## Alternatively, pass file contents or a `StringIO` directly:
file_metadata = anthropic.beta.files.upload(file: File.read("/path/to/file"))

## Or, to control the filename and/or content type:
file = Anthropic::FilePart.new(File.read("/path/to/file"), filename: "/path/to/file", content_type: "...")
file_metadata = anthropic.beta.files.upload(file: file)

puts(file_metadata.id)
```

Note that you can also pass a raw `IO` descriptor, but this disables retries, as the library can't be sure if the descriptor is a file or pipe (which cannot be rewound).

### Sorbet

This library provides comprehensive [RBI](https://sorbet.org/docs/rbi) definitions, and has no dependency on sorbet-runtime.

You can provide typesafe request parameters like so:

```ruby hidelines={1}
require "anthropic"
anthropic = Anthropic::Client.new
anthropic.messages.create(
  max_tokens: 1024,
  messages: [Anthropic::MessageParam.new(role: "user", content: "Hello, Claude")],
  model: :"claude-opus-4-7"
)
```

Or, equivalently:

```ruby hidelines={1}
require "anthropic"
anthropic = Anthropic::Client.new
## Hashes work, but are not typesafe:
anthropic.messages.create(
  max_tokens: 1024,
  messages: [{role: "user", content: "Hello, Claude"}],
  model: :"claude-opus-4-7"
)

## You can also splat a full Params class:
params = Anthropic::MessageCreateParams.new(
  max_tokens: 1024,
  messages: [Anthropic::MessageParam.new(role: "user", content: "Hello, Claude")],
  model: :"claude-opus-4-7"
)
anthropic.messages.create(**params)
```

#### Enums

Since this library does not depend on `sorbet-runtime`, it cannot provide [`T::Enum`](https://sorbet.org/docs/tenum) instances. Instead, the SDK provides "tagged symbols", which is always a primitive at runtime:

```ruby nocheck
## :auto
puts(Anthropic::MessageCreateParams::ServiceTier::AUTO)

## Revealed type: `T.all(Anthropic::MessageCreateParams::ServiceTier, Symbol)`
T.reveal_type(Anthropic::MessageCreateParams::ServiceTier::AUTO)
```

Enum parameters have a "relaxed" type, so you can either pass in enum constants or their literal value:

```ruby
## Using the enum constants preserves the tagged type information:
anthropic.messages.create(
  service_tier: Anthropic::MessageCreateParams::ServiceTier::AUTO,
  # ...
)

## Literal values are also permissible:
anthropic.messages.create(
  service_tier: :auto,
  # ...
)
```

### BaseModel

All parameter and response objects inherit from `Anthropic::Internal::Type::BaseModel`, which provides several conveniences, including:

1. All fields, including unknown ones, are accessible with `obj[:prop]` syntax, and can be destructured with `obj => {prop: prop}` or pattern-matching syntax.

2. Structural equivalence for equality; if two API calls return the same values, comparing the responses with == will return true.

3. Both instances and the classes themselves can be pretty-printed.

4. Helpers such as `#to_h`, `#deep_to_h`, `#to_json`, and `#to_yaml`.

### Concurrency and connection pooling

The `Anthropic::Client` instances are threadsafe, but are only fork-safe when there are no in-flight HTTP requests.

Each instance of `Anthropic::Client` has its own HTTP connection pool with a default size of 99. As such, the recommendation is to instantiate the client once per application in most settings.

When all available connections from the pool are checked out, requests wait for a new connection to become available, with queue time counting towards the request timeout.

Unless otherwise specified, other classes in the SDK do not have locks protecting their underlying data structure.

### Making custom or undocumented requests

#### Undocumented properties

You can send undocumented parameters to any endpoint, and read undocumented response properties, like so:

> The `extra_` parameters of the same name override the documented parameters. For security reasons, ensure these methods are only used with trusted input data.

```ruby hidelines={1} nocheck
require "anthropic"
anthropic = Anthropic::Client.new
value = "example"
message =
  anthropic.messages.create(
    max_tokens: 1024,
    messages: [{role: "user", content: "Hello, Claude"}],
    model: :"claude-opus-4-7",
    request_options: {
      extra_query: {my_query_parameter: value},
      extra_body: {my_body_parameter: value},
      extra_headers: {"my-header": value}
    }
  )

puts(message[:my_undocumented_property])
```

#### Undocumented request params

If you want to explicitly send an extra param, you can do so with the `extra_query`, `extra_body`, and `extra_headers` under the `request_options:` parameter when making a request, as seen in the examples above.

#### Undocumented endpoints

To make requests to undocumented endpoints while retaining the benefit of auth, retries, and so on, you can make requests using `anthropic.request`, like so:

```ruby nocheck
response = anthropic.request(
  method: :post,
  path: '/undocumented/endpoint',
  query: {"dog": "woof"},
  headers: {"useful-header": "interesting-value"},
  body: {"hello": "world"}
)
```

### Platform integrations

> For detailed platform setup guides with code examples, see:
> - [Amazon Bedrock](https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock)
> - [Amazon Bedrock (legacy)](https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock)
> - [Google Vertex AI](https://platform.claude.com/docs/en/build-with-claude/claude-on-vertex-ai)

The Ruby SDK supports Bedrock and Vertex AI through dedicated client classes:

- **Bedrock:** `Anthropic::BedrockMantleClient`, or `Anthropic::BedrockClient` for the `bedrock-runtime` path. `Anthropic::BedrockMantleClient` requires the `aws-sdk-core` gem; `Anthropic::BedrockClient` requires the `aws-sdk-bedrockruntime` gem.
- **Vertex AI:** `Anthropic::VertexClient`. Requires the `googleauth` gem.

Use `Anthropic::BedrockMantleClient` for new projects; `Anthropic::BedrockClient` remains for existing applications using the Bedrock `InvokeModel` API.

### Semantic versioning

This package follows [SemVer](https://semver.org/spec/v2.0.0.html) conventions. As the library is in initial development and has a major version of `0`, APIs may change at any time.

This package considers improvements to the (non-runtime) `*.rbi` and `*.rbs` type definitions to be non-breaking changes.

### Additional resources

- [GitHub repository](https://github.com/anthropics/anthropic-sdk-ruby)
- [RubyDoc documentation](https://gemdocs.org/gems/anthropic)
- [API reference](https://platform.claude.com/docs/en/api/overview)
- [Streaming guide](https://platform.claude.com/docs/en/build-with-claude/streaming)

---

## C# SDK

Install and configure the Anthropic C# SDK for .NET applications with IChatClient integration

---

The Anthropic C# SDK provides convenient access to the Anthropic REST API from applications written in C#.

> The C# SDK is currently in beta. APIs may change between versions.

> For API feature documentation with code examples, see the [API reference](https://platform.claude.com/docs/en/api/overview). This page covers C#-specific SDK features and configuration.

> As of version 10+, the `Anthropic` package is now the official Anthropic SDK for C#. Package versions 3.X and below were previously used for the tryAGI community-built SDK, which has moved to [`tryAGI.Anthropic`](https://www.nuget.org/packages/tryagi.Anthropic/). If you need to continue using the former client in your project, update your package reference to `tryAGI.Anthropic`.

### Installation

Install the package from [NuGet](https://www.nuget.org/packages/Anthropic):

```bash
dotnet add package Anthropic
```

### Requirements

This library requires .NET Standard 2.0 or later.

### Usage

```csharp
using System;
using Anthropic;
using Anthropic.Models.Messages;

AnthropicClient client = new();

MessageCreateParams parameters = new()
{
    MaxTokens = 1024,
    Messages =
    [
        new()
        {
            Role = Role.User,
            Content = "Hello, Claude",
        },
    ],
    Model = "claude-opus-4-7",
};

var message = await client.Messages.Create(parameters);

Console.WriteLine(message);
```

### Client configuration

Configure the client using environment variables:

```csharp
using Anthropic;

// Configured using the ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN and ANTHROPIC_BASE_URL environment variables
AnthropicClient client = new();
```

Or manually:

```csharp
using Anthropic;

AnthropicClient client = new() { ApiKey = "my-anthropic-api-key" };
```

Or using a combination of the two approaches.

See this table for the available options:

| Property    | Environment variable   | Required | Default value                 |
| ----------- | ---------------------- | -------- | ----------------------------- |
| `ApiKey`    | `ANTHROPIC_API_KEY`    | false    | -                             |
| `AuthToken` | `ANTHROPIC_AUTH_TOKEN` | false    | -                             |
| `BaseUrl`   | `ANTHROPIC_BASE_URL`   | true     | `"https://api.anthropic.com"` |

#### Modifying configuration

To temporarily use a modified client configuration, while reusing the same connection and thread pools, call `WithOptions` on any client or service:

```csharp nocheck
using System;

var message = await client
    .WithOptions(options =>
        options with
        {
            BaseUrl = "https://example.com",
            Timeout = TimeSpan.FromSeconds(42),
        }
    )
    .Messages.Create(parameters);

Console.WriteLine(message);
```

Using a [`with` expression](https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/operators/with-expression) makes it easy to construct the modified options.

The `WithOptions` method does not affect the original client or service.

### Streaming

The SDK defines methods that return response "chunk" streams, where each chunk can be individually processed as soon as it arrives instead of waiting on the full response. Streaming methods generally correspond to [SSE](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events) or [JSONL](https://jsonlines.org) responses.

A streaming method always has a `Streaming` suffix in its name, even if it doesn't have a non-streaming variant.

These streaming methods return [`IAsyncEnumerable`](https://learn.microsoft.com/en-us/dotnet/api/system.collections.generic.iasyncenumerable-1):

```csharp nocheck
using System;
using Anthropic.Models.Messages;

MessageCreateParams parameters = new()
{
    MaxTokens = 1024,
    Messages =
    [
        new()
        {
            Role = Role.User,
            Content = "Hello, Claude",
        },
    ],
    Model = "claude-opus-4-7",
};

await foreach (var message in client.Messages.CreateStreaming(parameters))
{
    Console.WriteLine(message);
}
```

### Error handling

The SDK throws custom unchecked exception types:

- `AnthropicApiException`: Base class for API errors. See this table for which exception subclass is thrown for each HTTP status code:

| Status | Exception                                |
| ------ | ---------------------------------------- |
| 400    | `AnthropicBadRequestException`           |
| 401    | `AnthropicUnauthorizedException`         |
| 403    | `AnthropicForbiddenException`            |
| 404    | `AnthropicNotFoundException`             |
| 422    | `AnthropicUnprocessableEntityException`  |
| 429    | `AnthropicRateLimitException`            |
| 5xx    | `Anthropic5xxException`                  |
| others | `AnthropicUnexpectedStatusCodeException` |

Additionally, all 4xx errors inherit from `Anthropic4xxException`.

- `AnthropicSseException`: thrown for errors encountered during SSE streaming after a successful initial HTTP response.

- `AnthropicIOException`: I/O networking errors.

- `AnthropicInvalidDataException`: Failure to interpret successfully parsed data. For example, when accessing a property that's supposed to be required, but the API unexpectedly omitted it from the response.

- `AnthropicException`: Base class for all exceptions.

### Retries

The SDK automatically retries 2 times by default, with a short exponential backoff between requests.

Only the following error types are retried:

- Connection errors (for example, due to a network connectivity problem)
- 408 Request Timeout
- 409 Conflict
- 429 Rate Limit
- 5xx Internal

The API may also explicitly instruct the SDK to retry or not retry a request.

To set a custom number of retries, configure the client using the `MaxRetries` property:

```csharp
using Anthropic;

AnthropicClient client = new() { MaxRetries = 3 };
```

Or configure a single method call using `WithOptions`:

```csharp nocheck
using System;

var message = await client
    .WithOptions(options =>
        options with { MaxRetries = 3 }
    )
    .Messages.Create(parameters);

Console.WriteLine(message);
```

### Timeouts

Requests time out after 10 minutes by default.

To set a custom timeout, configure the client using the `Timeout` option:

```csharp
using System;
using Anthropic;

AnthropicClient client = new() { Timeout = TimeSpan.FromSeconds(42) };
```

Or configure a single method call using `WithOptions`:

```csharp nocheck
using System;

var message = await client
    .WithOptions(options =>
        options with { Timeout = TimeSpan.FromSeconds(42) }
    )
    .Messages.Create(parameters);

Console.WriteLine(message);
```

### Pagination

The SDK defines methods that return paginated lists of results. It provides convenient ways to access the results either one page at a time or item-by-item across all pages.

#### Auto-pagination

To iterate through all results across all pages, use the `Paginate` method, which automatically fetches more pages as needed. The method returns an [`IAsyncEnumerable`](https://learn.microsoft.com/en-us/dotnet/api/system.collections.generic.iasyncenumerable-1):

```csharp nocheck
using System;

var page = await client.Messages.Batches.List(parameters);
await foreach (var item in page.Paginate())
{
    Console.WriteLine(item);
}
```

#### Manual pagination

To access individual page items and manually request the next page, use the `Items` property, and `HasNext` and `Next` methods:

```csharp hidelines={1..5}
using Anthropic;
using System;

AnthropicClient client = new();

var page = await client.Messages.Batches.List();
while (true)
{
    foreach (var item in page.Items)
    {
        Console.WriteLine(item);
    }
    if (!page.HasNext())
    {
        break;
    }
    page = await page.Next();
}
```

### Response validation

In rare cases, the API may return a response that doesn't match the expected type. By default, the SDK does not throw an exception in this case. It throws `AnthropicInvalidDataException` only if you directly access the property.

If you would prefer to check that the response is completely well-typed upfront, then either call `Validate`:

```csharp nocheck
var message = await client.Messages.Create(parameters);
message.Validate();
```

Or configure the client using the `ResponseValidation` option:

```csharp
using Anthropic;

AnthropicClient client = new() { ResponseValidation = true };
```

Or configure a single method call using `WithOptions`:

```csharp nocheck
using System;

var message = await client
    .WithOptions(options =>
        options with { ResponseValidation = true }
    )
    .Messages.Create(parameters);

Console.WriteLine(message);
```

### IChatClient integration

The SDK provides an implementation of the `IChatClient` interface from the `Microsoft.Extensions.AI.Abstractions` library. This enables `AnthropicClient` (and `Anthropic.Services.IBetaService`) to be used with other libraries that integrate with these core abstractions. For example, tools in the MCP C# SDK (`ModelContextProtocol`) library can be used directly with an `AnthropicClient` exposed via `IChatClient`.

```csharp nocheck
using Anthropic;
using Microsoft.Extensions.AI;
using ModelContextProtocol.Client;

// Configured using the ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN and ANTHROPIC_BASE_URL environment variables
IChatClient chatClient = client.AsIChatClient("claude-opus-4-7")
    .AsBuilder()
    .UseFunctionInvocation()
    .Build();

// Using McpClient from the MCP C# SDK
McpClient learningServer = await McpClient.CreateAsync(
    new HttpClientTransport(new() { Endpoint = new("https://learn.microsoft.com/api/mcp") }));

ChatOptions options = new() { Tools = [.. await learningServer.ListToolsAsync()] };

Console.WriteLine(await chatClient.GetResponseAsync("Tell me about IChatClient", options));
```

### Requests and responses

To send a request to the Claude API, build an instance of a `Params` class and pass it to the corresponding client method. When the response is received, it's deserialized into an instance of a C# class.

For example, `client.Messages.Create` should be called with an instance of `MessageCreateParams`, and it will return an instance of `Task<Message>`.

### Advanced usage

#### Binary responses

The SDK defines methods that return binary responses, which are used for API responses that shouldn't necessarily be parsed, like non-JSON data.

These methods return `HttpResponse`:

```csharp nocheck
using System;
using Anthropic.Models.Beta.Files;

FileDownloadParams parameters = new() { FileID = "file_id" };

var response = await client.Beta.Files.Download(parameters);

Console.WriteLine(response);
```

To save the response content to a file, or any [`Stream`](https://learn.microsoft.com/en-us/dotnet/api/system.io.stream), use the [`CopyToAsync`](https://learn.microsoft.com/en-us/dotnet/api/system.io.stream.copytoasync) method:

```csharp nocheck
using System.IO;

using var response = await client.Beta.Files.Download(parameters);
using var contentStream = await response.ReadAsStream();
using var fileStream = File.Open(path, FileMode.OpenOrCreate);
await contentStream.CopyToAsync(fileStream); // Or any other Stream
```

#### Raw responses

The SDK defines methods that deserialize responses into instances of C# classes. To access response headers, status code, or the raw response body, prefix any HTTP method call on a client or service with `WithRawResponse`:

```csharp nocheck
var response = await client.WithRawResponse.Messages.Create(parameters);
var statusCode = response.StatusCode;
var headers = response.Headers;
```

The raw `HttpResponseMessage` can also be accessed through the `RawMessage` property.

For non-streaming responses, you can deserialize the response into an instance of a C# class if needed:

```csharp nocheck
using System;
using Anthropic.Models.Messages;

var response = await client.WithRawResponse.Messages.Create(parameters);
Message deserialized = await response.Deserialize();
Console.WriteLine(deserialized);
```

For streaming responses, you can deserialize the response to an `IAsyncEnumerable` if needed:

```csharp nocheck
using System;

var response = await client.WithRawResponse.Messages.CreateStreaming(parameters);
await foreach (var item in response.Enumerate())
{
    Console.WriteLine(item);
}
```

#### Logging

> All log messages are intended for debugging only. The format and content of log messages may change between releases.

Enable debug logging via environment variable:

```bash
export ANTHROPIC_LOG=debug
```

#### Undocumented API functionality

The SDK is typed for convenient usage of the documented API. However, it also supports working with undocumented or not yet supported parts of the API.

### Platform integrations

> For detailed platform setup guides with code examples, see:
> - [Amazon Bedrock](https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock)
> - [Amazon Bedrock (legacy)](https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock)
> - [Microsoft Foundry](https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry)

The C# SDK supports Bedrock and Foundry through separate NuGet packages:

- **Bedrock:** `Anthropic.Bedrock`. Use `AnthropicBedrockMantleClient` for the Messages-API Bedrock endpoint, or `AnthropicBedrockClient` (`bedrock-runtime` path). `AnthropicBedrockMantleClient` takes an optional `MantleAwsClientOptions` config object; `AnthropicBedrockClient` accepts `AnthropicBedrockCredentialsHelper.FromEnv()` or explicit credentials.
- **Foundry:** `Anthropic.Foundry`. Use `AnthropicFoundryClient` with `DefaultAnthropicFoundryCredentials.FromEnv()` or explicit credentials.

Use `AnthropicBedrockMantleClient` for new projects; `AnthropicBedrockClient` remains for existing applications using the Bedrock `InvokeModel` API.

### Semantic versioning

> While this package is versioned as 10+, it's currently in beta. During the beta period, breaking changes may occur in minor or patch releases. Once the library reaches stable release, SemVer conventions will be followed more strictly. Share feedback by [filing an issue](https://www.github.com/anthropics/anthropic-sdk-csharp/issues/new).

This package generally follows [SemVer](https://semver.org/spec/v2.0.0.html) conventions, though certain backwards-incompatible changes may be released as minor versions:

1. Changes to library internals which are technically public but not intended or documented for external use. _(Please open a GitHub issue to let the maintainers know if you're relying on such internals.)_
2. Changes that aren't expected to impact the vast majority of users in practice.

Backwards-compatibility is taken seriously to ensure you can rely on a smooth upgrade experience.

### Additional resources

- [GitHub repository](https://github.com/anthropics/anthropic-sdk-csharp)
- [NuGet package](https://www.nuget.org/packages/Anthropic)
- [API reference](https://platform.claude.com/docs/en/api/overview)
- [Streaming guide](https://platform.claude.com/docs/en/build-with-claude/streaming)

---

## PHP SDK

Install and configure the Anthropic PHP SDK with value objects and builder patterns

---

The Anthropic PHP library provides convenient access to the Anthropic REST API from any PHP 8.1.0+ application.

> The PHP SDK is currently in beta. APIs may change between versions.

> For API feature documentation with code examples, see the [API reference](https://platform.claude.com/docs/en/api/overview). This page covers PHP-specific SDK features and configuration.

### Installation

```bash
composer require "anthropic-ai/sdk"
```

### Requirements

PHP 8.1.0 or higher.

### Usage

This library uses named parameters to specify optional arguments. Parameters with a default value must be set by name.

```php hidelines={1..4}
<?php

use Anthropic\Client;

$client = new Client(
  apiKey: getenv("ANTHROPIC_API_KEY") ?: "my-anthropic-api-key"
);

$message = $client->messages->create(
  maxTokens: 1024,
  messages: [['role' => 'user', 'content' => 'Hello, Claude']],
  model: 'claude-opus-4-7',
);

var_dump($message->content);
```

### Value objects

It is recommended to use the static `with` constructor `Base64ImageSource::with(data: "U3RhaW5sZXNzIHJvY2tz", ...)` and named parameters to initialize value objects.

However, builders are also provided `(new Base64ImageSource)->withData("U3RhaW5sZXNzIHJvY2tz")`.

### Streaming

The SDK provides support for streaming responses using Server-Sent Events (SSE).

```php hidelines={1..4}
<?php

use Anthropic\Client;

$client = new Client(
  apiKey: getenv("ANTHROPIC_API_KEY") ?: "my-anthropic-api-key"
);

$stream = $client->messages->createStream(
  maxTokens: 1024,
  messages: [['role' => 'user', 'content' => 'Hello, Claude']],
  model: 'claude-opus-4-7',
);

foreach ($stream as $message) {
  var_dump($message);
}
```

### Error handling

When the library is unable to connect to the API, or if the API returns a non-success status code (i.e., 4xx or 5xx response), a subclass of `Anthropic\Core\Exceptions\APIException` is thrown:

```php hidelines={2..3,7..9}
<?php
use Anthropic\Client;

use Anthropic\Core\Exceptions\APIConnectionException;
use Anthropic\Core\Exceptions\APIStatusException;
use Anthropic\Core\Exceptions\RateLimitException;

$client = new Client();

try {
  $message = $client->messages->create(
    maxTokens: 1024,
    messages: [['role' => 'user', 'content' => 'Hello, Claude']],
    model: 'claude-opus-4-7',
  );
} catch (APIConnectionException $e) {
  echo "The server could not be reached", PHP_EOL;
  var_dump($e->getPrevious());
} catch (RateLimitException $_) {
  echo "A 429 status code was received; we should back off a bit.", PHP_EOL;
} catch (APIStatusException $e) {
  echo "Another non-200-range status code was received", PHP_EOL;
  echo $e->getMessage();
}
```

Error codes are as follows:

| Cause            | Error Type                     |
| ---------------- | ------------------------------ |
| HTTP 400         | `BadRequestException`          |
| HTTP 401         | `AuthenticationException`      |
| HTTP 403         | `PermissionDeniedException`    |
| HTTP 404         | `NotFoundException`            |
| HTTP 409         | `ConflictException`            |
| HTTP 422         | `UnprocessableEntityException` |
| HTTP 429         | `RateLimitException`           |
| HTTP >= 500      | `InternalServerException`      |
| Other HTTP error | `APIStatusException`           |
| Timeout          | `APITimeoutException`          |
| Network error    | `APIConnectionException`       |

### Retries

Certain errors are automatically retried 2 times by default, with a short exponential backoff.

Connection errors (for example, due to a network connectivity problem), 408 Request Timeout, 409 Conflict, 429 Rate Limit, >=500 Internal errors, and timeouts are all retried by default.

You can use the `maxRetries` option to configure or disable this:

```php hidelines={1..3,5}
<?php

use Anthropic\Client;
use Anthropic\RequestOptions;

// Configure the default for all requests:
$client = new Client(requestOptions: RequestOptions::with(maxRetries: 0));

// Or, configure per-request:
$result = $client->messages->create(
  maxTokens: 1024,
  messages: [['role' => 'user', 'content' => 'Hello, Claude']],
  model: 'claude-opus-4-7',
  requestOptions: RequestOptions::with(maxRetries: 5),
);
```

### Pagination

List methods in the Claude API are paginated.

This library provides auto-paginating iterators with each list response, so you do not have to request successive pages manually:

```php hidelines={1..4}
<?php

use Anthropic\Client;

$client = new Client(
  apiKey: getenv("ANTHROPIC_API_KEY") ?: "my-anthropic-api-key"
);

$page = $client->beta->messages->batches->list(limit: 20);

var_dump($page);

// fetch items from the current page
foreach ($page->getItems() as $item) {
  var_dump($item->id);
}
// make additional network requests to fetch items from all pages, including and after the current page
foreach ($page->pagingEachItem() as $item) {
  var_dump($item->id);
}
```

### Advanced usage

#### Undocumented properties

You can send undocumented parameters to any endpoint, and read undocumented response properties, like so:

> The `extra*` parameters of the same name override the documented parameters.

```php hidelines={2..3,5..7}
<?php
use Anthropic\Client;

use Anthropic\RequestOptions;

$client = new Client();

$message = $client->messages->create(
  maxTokens: 1024,
  messages: [['role' => 'user', 'content' => 'Hello, Claude']],
  model: 'claude-opus-4-7',
  requestOptions: RequestOptions::with(
    extraQueryParams: ['my_query_parameter' => 'value'],
    extraBodyParams: ['my_body_parameter' => 'value'],
    extraHeaders: ['my-header' => 'value'],
  ),
);
```

#### Undocumented request params

If you want to explicitly send an extra param, you can do so with the `extraQueryParams`, `extraBodyParams`, and `extraHeaders` options under `RequestOptions::with()` when making a request, as seen in the example above.

#### Undocumented endpoints

To make requests to undocumented endpoints while retaining the benefit of auth, retries, and so on, you can make requests using `client->request`, like so:

```php hidelines={1..2} nocheck
<?php
use Anthropic\Client;
$client = new Client();

$response = $client->request(
  method: "post",
  path: '/undocumented/endpoint',
  query: ['dog' => 'woof'],
  headers: ['useful-header' => 'interesting-value'],
  body: ['hello' => 'world']
);
```

### Semantic versioning

This package follows [SemVer](https://semver.org/spec/v2.0.0.html) conventions. As the library is in initial development and has a major version of `0`, APIs may change at any time.

This package considers improvements to the (non-runtime) PHPDoc type definitions to be non-breaking changes.

### Additional resources

- [GitHub repository](https://github.com/anthropics/anthropic-sdk-php)
- [Packagist](https://packagist.org/packages/anthropic-ai/sdk)
- [API reference](https://platform.claude.com/docs/en/api/overview)
- [Streaming guide](https://platform.claude.com/docs/en/build-with-claude/streaming)

---

## OpenAI SDK compatibility

Anthropic provides a compatibility layer that enables you to use the OpenAI SDK to test the Claude API. With a few code changes, you can quickly evaluate Anthropic model capabilities.

---

> This compatibility layer is primarily intended to test and compare model capabilities, and is not considered a long-term or production-ready solution for most use cases. While it is intended to remain fully functional and not have breaking changes, the priority is the reliability and effectiveness of the [Claude API](https://platform.claude.com/docs/en/api/overview).
>
> For more information on known compatibility limitations, see [Important OpenAI compatibility limitations](#important-openai-compatibility-limitations).
>
> If you encounter any issues with the OpenAI SDK compatibility feature, please share your feedback via this [compatibility feedback form](https://forms.gle/oQV4McQNiuuNbz9n8).

> For the best experience and access to Claude API full feature set ([PDF processing](https://platform.claude.com/docs/en/build-with-claude/pdf-support), [citations](https://platform.claude.com/docs/en/build-with-claude/citations), [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking), and [prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)), use the native [Claude API](https://platform.claude.com/docs/en/api/overview).

### Getting started with the OpenAI SDK

To use the OpenAI SDK compatibility feature, you'll need to:

1. Use an official OpenAI SDK
2. Change the following
   * Update your base URL to point to the Claude API
   * Replace your API key with a [Claude API key](https://platform.claude.com/settings/keys)
   * Update your model name to use a [Claude model](https://platform.claude.com/docs/en/about-claude/models/overview)
3. Review the documentation below for what features are supported

#### Quick start example

<CodeGroup>
    
    ```python Python nocheck
    import os

    from openai import OpenAI

    client = OpenAI(
        api_key=os.environ.get("ANTHROPIC_API_KEY"),  # Your Claude API key
        base_url="https://api.anthropic.com/v1/",  # the Claude API endpoint
    )

    response = client.chat.completions.create(
        model="claude-opus-4-7",  # Claude model name
        messages=[
            {"role": "system", "content": "You are a helpful assistant."},
            {"role": "user", "content": "Who are you?"},
        ],
    )

    print(response.choices[0].message.content)
    ```

    
    ```typescript TypeScript nocheck
    import OpenAI from "openai";

    const openai = new OpenAI({
      apiKey: "ANTHROPIC_API_KEY", // Your Claude API key
      baseURL: "https://api.anthropic.com/v1/" // Claude API endpoint
    });

    const response = await openai.chat.completions.create({
      messages: [{ role: "user", content: "Who are you?" }],
      model: "claude-opus-4-7" // Claude model name
    });

    console.log(response.choices[0].message.content);
    ```

</CodeGroup>

### Important OpenAI compatibility limitations

#### API behavior

Here are the most substantial differences from using OpenAI:

* The `strict` parameter for function calling is ignored, which means the tool use JSON is not guaranteed to follow the supplied schema. For guaranteed schema conformance, use the native [Claude API with Structured Outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs).
* Audio input is not supported; it will simply be ignored and stripped from input
* Prompt caching is not supported, but it is supported in [the Anthropic SDK](https://platform.claude.com/docs/en/api/client-sdks)
* System/developer messages are hoisted and concatenated to the beginning of the conversation, as Anthropic only supports a single initial system message.

Most unsupported fields are silently ignored rather than producing errors. These are all documented below.

#### Output quality considerations

If you’ve done lots of tweaking to your prompt, it’s likely to be well-tuned to OpenAI specifically. Consider using the [prompt improver in the Claude Console](https://platform.claude.com/dashboard) as a good starting point.

#### System / developer message hoisting

Most of the inputs to the OpenAI SDK clearly map directly to Anthropic’s API parameters, but one distinct difference is the handling of system / developer prompts. These two prompts can be put throughout a chat conversation via OpenAI. Since Anthropic only supports an initial system message, the API takes all system/developer messages and concatenates them together with a single newline (`\n`) in between them. This full string is then supplied as a single system message at the start of the messages.

#### Extended thinking support

You can enable [extended thinking](https://platform.claude.com/docs/en/build-with-claude/extended-thinking) capabilities by adding the `thinking` parameter. While this improves Claude's reasoning for complex tasks, the OpenAI SDK doesn't return Claude's detailed thought process. For full extended thinking features, including access to Claude's step-by-step reasoning output, use the native Claude API.

<CodeGroup>
    
    ```python Python nocheck hidelines={1..9}
    import os

    from openai import OpenAI

    client = OpenAI(
        api_key=os.environ.get("ANTHROPIC_API_KEY"),
        base_url="https://api.anthropic.com/v1/",
    )

    response = client.chat.completions.create(
        model="claude-sonnet-4-6",
        messages=[{"role": "user", "content": "Who are you?"}],
        extra_body={"thinking": {"type": "enabled", "budget_tokens": 2000}},
    )
    ```

    
    ```typescript TypeScript nocheck
    const response = await openai.chat.completions.create({
      messages: [{ role: "user", content: "Who are you?" }],
      model: "claude-sonnet-4-6",
      // @ts-expect-error
      thinking: { type: "enabled", budget_tokens: 2000 }
    });
    ```

</CodeGroup>

### Rate limits

Rate limits follow Anthropic's [standard limits](https://platform.claude.com/docs/en/api/rate-limits) for the `/v1/messages` endpoint.

### Detailed OpenAI compatible API support
#### Request fields
##### Simple fields
| Field | Support status |
|--------|----------------|
| `model` | Use Claude model names |
| `max_tokens` | Fully supported |
| `max_completion_tokens` | Fully supported |
| `stream` | Fully supported |
| `stream_options` | Fully supported |
| `top_p` | Fully supported |
| `parallel_tool_calls` | Fully supported |
| `stop` | All non-whitespace stop sequences work |
| `temperature` | Between 0 and 1 (inclusive). Values greater than 1 are capped at 1. |
| `n` | Must be exactly 1 |
| `logprobs` | Ignored |
| `metadata` | Ignored |
| `response_format` | Ignored. For JSON output, use [Structured Outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs) with the native Claude API |
| `prediction` | Ignored |
| `presence_penalty` | Ignored |
| `frequency_penalty` | Ignored |
| `seed` | Ignored |
| `service_tier` | Ignored |
| `audio` | Ignored |
| `logit_bias` | Ignored |
| `store` | Ignored |
| `user` | Ignored |
| `modalities` | Ignored |
| `top_logprobs` | Ignored |
| `reasoning_effort` | Ignored |

##### `tools` / `functions` fields
<section title="Show fields">

<Tabs>
<Tab title="Tools">
`tools[n].function` fields
| Field        | Support status         |
|--------------|-----------------|
| `name`       | Fully supported |
| `description`| Fully supported |
| `parameters` | Fully supported |
| `strict`     | Ignored. Use [Structured Outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs) with native Claude API for strict schema validation |
</Tab>
<Tab title="Functions">

`functions[n]` fields
> OpenAI has deprecated the `functions` field and suggests using `tools` instead.
| Field        | Support status         |
|--------------|-----------------|
| `name`       | Fully supported |
| `description`| Fully supported |
| `parameters` | Fully supported |
| `strict`     | Ignored. Use [Structured Outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs) with native Claude API for strict schema validation |
</Tab>
</Tabs>

</section>

##### `messages` array fields
<section title="Show fields">

<Tabs>
<Tab title="Developer role">
Fields for `messages[n].role == "developer"`
> Developer messages are hoisted to beginning of conversation as part of the initial system message
| Field | Support status |
|-------|---------|
| `content` | Fully supported, but hoisted |
| `name` | Ignored |

</Tab>
<Tab title="System role">
Fields for `messages[n].role == "system"`

> System messages are hoisted to beginning of conversation as part of the initial system message
| Field | Support status |
|-------|---------|
| `content` | Fully supported, but hoisted |
| `name` | Ignored |

</Tab>
<Tab title="User role">
Fields for `messages[n].role == "user"`

| Field | Variant | Sub-field | Support status |
|-------|---------|-----------|----------------|
| `content` | `string` | | Fully supported |
| | `array`, `type == "text"` | | Fully supported |
| | `array`, `type == "image_url"` | `url` | Fully supported |
| | | `detail` | Ignored |
| | `array`, `type == "input_audio"` | | Ignored |
| | `array`, `type == "file"` | | Ignored |
| `name` | | | Ignored |

</Tab>

<Tab title="Assistant role">
Fields for `messages[n].role == "assistant"`
| Field | Variant | Support status |
|-------|---------|----------------|
| `content` | `string` | Fully supported |
| | `array`, `type == "text"` | Fully supported |
| | `array`, `type == "refusal"` | Ignored |
| `tool_calls` | | Fully supported |
| `function_call` | | Fully supported |
| `audio` | | Ignored |
| `refusal` | | Ignored |

</Tab>

<Tab title="Tool role">
Fields for `messages[n].role == "tool"`
| Field | Variant | Support status |
|-------|---------|----------------|
| `content` | `string` | Fully supported |
| | `array`, `type == "text"` | Fully supported |
| `tool_call_id` | | Fully supported |
| `tool_choice` | | Fully supported |
| `name` | | Ignored |
</Tab>

<Tab title="Function role">
Fields for `messages[n].role == "function"`
| Field | Variant | Support status |
|-------|---------|----------------|
| `content` | `string` | Fully supported |
| | `array`, `type == "text"` | Fully supported |
| `tool_choice` | | Fully supported |
| `name` | | Ignored |
</Tab>
</Tabs>

</section>

#### Response fields

| Field | Support status |
|---------------------------|----------------|
| `id` | Fully supported |
| `choices[]` | Will always have a length of 1 |
| `choices[].finish_reason` | Fully supported |
| `choices[].index` | Fully supported |
| `choices[].message.role` | Fully supported |
| `choices[].message.content` | Fully supported |
| `choices[].message.tool_calls` | Fully supported |
| `object` | Fully supported |
| `created` | Fully supported |
| `model` | Fully supported |
| `finish_reason` | Fully supported |
| `content` | Fully supported |
| `usage.completion_tokens` | Fully supported |
| `usage.prompt_tokens` | Fully supported |
| `usage.total_tokens` | Fully supported |
| `usage.completion_tokens_details` | Always empty |
| `usage.prompt_tokens_details` | Always empty |
| `choices[].message.refusal` | Always empty |
| `choices[].message.audio` | Always empty |
| `logprobs` | Always empty |
| `service_tier` | Always empty |
| `system_fingerprint` | Always empty |

#### Error message compatibility

The compatibility layer maintains consistent error formats with the OpenAI API. However, the detailed error messages will not be equivalent. Only use the error messages for logging and debugging.

#### Header compatibility

While the OpenAI SDK automatically manages headers, here is the complete list of headers supported by the Claude API for developers who need to work with them directly.

| Header | Support Status |
|---------|----------------|
| `x-ratelimit-limit-requests` | Fully supported |
| `x-ratelimit-limit-tokens` | Fully supported |
| `x-ratelimit-remaining-requests` | Fully supported |
| `x-ratelimit-remaining-tokens` | Fully supported |
| `x-ratelimit-reset-requests` | Fully supported |
| `x-ratelimit-reset-tokens` | Fully supported |
| `retry-after` | Fully supported |
| `request-id` | Fully supported |
| `openai-version` | Always `2020-10-01` |
| `authorization` | Fully supported |
| `openai-processing-ms` | Always empty |
