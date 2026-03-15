---
title: LLM Tools
description: Add LLM-powered conversations with tool calling, streaming, and persistence to your ShipQ application.
---

ShipQ includes an LLM integration layer that lets you expose plain Go functions as LLM tools, wire them to OpenAI or Anthropic (or both), and get automatic streaming over Centrifugo and automatic persistence to your database — all through the existing channel/worker infrastructure.

## What You Get

- **Tool calling from plain Go functions** — write a function, register it, and the LLM can call it
- **Provider-agnostic** — OpenAI and Anthropic work out of the box; swap providers with one line of Go
- **Streaming** — text deltas and tool call events are published to Centrifugo in real time via your existing channel
- **Persistence** — every message (user prompt, assistant response, tool call, tool result) is written to `llm_conversations` and `llm_messages` tables automatically
- **Codegen** — `shipq llm compile` generates tool registries, JSON Schemas, typed dispatchers, database migrations, querydefs, and stream types
- **Testing utilities** — mock providers and assertion helpers for unit testing without real API keys

## Prerequisites

Before adding LLM support, you need:

- **Workers & channels** set up (`shipq workers` has been run)
- **At least one channel package** defined in `[channels]`
- An **API key** for OpenAI (`OPENAI_API_KEY`) and/or Anthropic (`ANTHROPIC_API_KEY`) as environment variables

Auth is **not** required. Public channels work fine for LLM — the `account_id` columns in the LLM tables are nullable.

## Step 1: Define a Tool

A tool is a plain Go function with a struct input and a struct output. Create a package for your tools:

```go
// tools/weather/weather.go
package weather

import "context"

type WeatherInput struct {
    City    string `json:"city"    desc:"The city to get weather for"`
    Country string `json:"country" desc:"ISO country code, e.g. US"`
}

type WeatherOutput struct {
    TempC       float64 `json:"temp_c"`
    Description string  `json:"description"`
}

func GetWeather(ctx context.Context, input *WeatherInput) (*WeatherOutput, error) {
    // Your real implementation here — call a weather API, query a database, etc.
    return &WeatherOutput{
        TempC:       22.5,
        Description: "Partly cloudy",
    }, nil
}
```

### Struct tags

| Tag | Purpose | Example |
|-----|---------|---------|
| `json` | Property name in the JSON Schema sent to the LLM | `json:"city"` |
| `desc` | Human-readable description included in the schema (helps the model understand the parameter) | `desc:"The city to get weather for"` |

### Function signature rules

Tool functions must match one of these signatures:

```go
func(context.Context, *InputStruct) (*OutputStruct, error)
func(*InputStruct) (*OutputStruct, error)
```

Both `InputStruct` and `OutputStruct` must be structs. The context parameter is optional but recommended — it carries cancellation, deadlines, and any values you injected via `Setup`.

## Step 2: Register Tools

Each tool package exports a `Register` function that registers tools on an `llm.App`:

```go
// tools/weather/register.go
package weather

import "myapp/shipq/lib/llm"

func Register(app *llm.App) {
    app.Tool("get_weather", "Get the current weather for a city", GetWeather)
}
```

You can register multiple tools in one package:

```go
func Register(app *llm.App) {
    app.Tool("get_weather", "Get the current weather for a city", GetWeather)
    app.Tool("get_forecast", "Get a 5-day weather forecast", GetForecast)
}
```

The `Register` function follows the same convention as `handler.App` and `channel.App` — it's a registration shim that captures metadata for code generation.

## Step 3: Configure `shipq.ini`

Add an `[llm]` section listing your tool packages:

```ini
[llm]
tool_pkgs = myapp/tools/weather
```

Multiple packages are comma-separated:

```ini
[llm]
tool_pkgs = myapp/tools/weather, myapp/tools/calendar
```

That's the only LLM-specific config. Provider, model, and system prompt are all set in Go code (see Step 5).

## Step 4: Compile

```sh
shipq llm compile
```

This generates:

| Artifact | Location | Description |
|----------|----------|-------------|
| Tool registry | `tools/weather/zz_generated_registry.go` | `Registry()` function returning typed tool dispatchers + JSON Schemas |
| Persister adapter | `shipq/lib/llmpersist/zz_generated_persister.go` | Wraps `queries.Runner` to satisfy `llm.Persister` |
| Migration | `migrations/` | `llm_conversations` + `llm_messages` tables |
| Querydefs | `querydefs/` | Insert/update/list queries for LLM persistence |
| Stream types (TypeScript only) | Generated TypeScript channel client | `LLMTextDelta`, `LLMToolCallStart`, `LLMToolCallResult`, `LLMDone` auto-injected into the TypeScript client for LLM-enabled channels |

:::note
LLM stream types (`LLMTextDelta`, `LLMToolCallStart`, `LLMToolCallResult`, `LLMDone`) are **not** added to `FromServer(...)` in your Go channel registration. The LLM library publishes them automatically via the raw `channel.Channel` internally. They are only auto-injected into the **TypeScript** client so the frontend can handle them with typed `on<Type>` callbacks.

A channel is detected as "LLM-enabled" when any Go file in its package imports the `llm` package and calls `llm.WithClient` or `llm.WithNamedClient`. This detection is used only for TypeScript codegen, not for Go.
:::

After compiling, run migrations and tidy dependencies:

```sh
shipq migrate up
go mod tidy
```

## Step 5: Write the Setup Function

In your channel package, write a `Setup` function that wires the provider, tools, and persistence together. This is ordinary Go code — not generated — so you have full control:

```go
// channels/chatbot/setup.go
package chatbot

import (
    "context"
    "os"

    "myapp/shipq/lib/channel"
    "myapp/shipq/lib/llm"
    "myapp/shipq/lib/llm/anthropic"
    "myapp/shipq/lib/llmpersist"
    "myapp/shipq/lib/db/dbrunner"
    "myapp/tools/weather"
)

func Setup(ctx context.Context) context.Context {
    ch := channel.FromContext(ctx)
    db := channel.DBFromContext(ctx)
    persister := llmpersist.New(dbrunner.NewQueryRunner(db))

    client := llm.NewClient(
        anthropic.New(os.Getenv("ANTHROPIC_API_KEY"), "claude-sonnet-4-20250514"),
        llm.WithTools(weather.Registry()),
        llm.WithChannel(ch),
        llm.WithPersister(persister),
        llm.WithSystem("You are a helpful weather assistant."),
    )
    return llm.WithClient(ctx, client)
}
```

### Why Setup is user-written

- **Provider choice is a runtime decision.** Env vars, feature flags, A/B tests — these need Go code, not INI config.
- **Mixing providers requires code.** One channel can use Anthropic for conversation and OpenAI for summaries.
- **The boilerplate is small** — ~15 lines of straightforward Go.
- **It matches the existing pattern.** Many channels already have Setup functions for injecting dependencies.

## Step 6: Write the Channel Handler

Your channel handler uses the client from context:

```go
// channels/chatbot/handler.go
package chatbot

import (
    "context"

    "myapp/shipq/lib/llm"
)

type ChatRequest struct {
    Message string `json:"message"`
}

func HandleChatRequest(ctx context.Context, req *ChatRequest) error {
    client := llm.ClientFromContext(ctx)
    resp, err := client.Chat(ctx, req.Message)
    if err != nil {
        return err
    }
    // By this point:
    // - Every text token was streamed to the frontend via Centrifugo
    // - Every tool call and result was persisted to llm_messages
    // - The conversation metadata was written to llm_conversations
    _ = resp
    return nil
}
```

Register the channel as usual:

```go
// channels/chatbot/register.go
package chatbot

import "myapp/shipq/lib/channel"

func Register(app *channel.App) {
    app.DefineChannel("chatbot",
        channel.FromClient(ChatRequest{}),
        channel.FromServer(ChatResponse{}),
        // LLM stream types (LLMTextDelta, LLMToolCallStart, etc.) are NOT
        // registered here. The LLM client publishes them automatically via
        // the raw channel. They are auto-injected into the TypeScript client
        // by `shipq workers compile`.
    ).Public(channel.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 10})
}
```

:::tip[LLM-only channels]
If the **only** server→client messages are LLM stream events (no custom response type), you can omit `FromServer` arguments entirely:

```go
func Register(app *channel.App) {
    app.DefineChannel("assistant",
        channel.FromClient(ChatRequest{}),
        channel.FromServer(), // No custom server types — all server messages
        // are LLM stream events (LLMTextDelta, LLMToolCallStart, etc.)
        // published automatically by the LLM library.
    ).Public(channel.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 10})
}
```
:::

## Client Options

`llm.NewClient` accepts the following options:

| Option | Description | Default |
|--------|-------------|---------|
| `WithTools(r)` | Tool registry for function calling | No tools |
| `WithChannel(ch)` | Enables real-time streaming over Centrifugo | No streaming |
| `WithPersister(p)` | Enables database persistence | No persistence |
| `WithSystem(prompt)` | System prompt for the conversation | Empty |
| `WithMaxIterations(n)` | Maximum tool-calling round-trips | 10 |
| `WithMaxTokens(n)` | Maximum output tokens per provider call | Provider default |
| `WithTemperature(t)` | Sampling temperature | Provider default |
| `WithWebSearch(cfg)` | Enable web search (provider-dependent) | Disabled |
| `WithErrorStrategy(s)` | What to do when a tool returns an error | `SendErrorToModel` |
| `WithSequentialToolCalls()` | Execute parallel tool calls sequentially | Parallel |
| `WithTaskDAG(g)` | Attach a task dependency graph to control tool ordering | No DAG (all tools always available) |
| `WithLogger(l)` | Structured logger for retry/diagnostic messages | `slog.Default()` |

### Error strategies

| Strategy | Behavior |
|----------|----------|
| `SendErrorToModel` | Sends the error message back to the model so it can retry or explain. This is the default — many tool errors are recoverable ("city not found", etc.). |
| `AbortOnToolError` | Stops the conversation loop and returns the error to the caller. |

## Using Multiple Providers

A single channel can use multiple LLM providers. Register named clients in Setup:

```go
func Setup(ctx context.Context) context.Context {
    ch := channel.FromContext(ctx)
    db := channel.DBFromContext(ctx)
    persister := llmpersist.New(dbrunner.NewQueryRunner(db))
    tools := weather.Registry()

    // Default client: Anthropic for conversation
    ctx = llm.WithClient(ctx, llm.NewClient(
        anthropic.New(os.Getenv("ANTHROPIC_API_KEY"), "claude-sonnet-4-20250514"),
        llm.WithTools(tools),
        llm.WithChannel(ch),
        llm.WithPersister(persister),
        llm.WithSystem("You are a helpful assistant."),
    ))

    // Named client: OpenAI for summaries
    ctx = llm.WithNamedClient(ctx, "summary", llm.NewClient(
        openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1"),
        llm.WithTools(tools),
        llm.WithChannel(ch),
        llm.WithPersister(persister),
        llm.WithSystem("Summarize the conversation so far."),
    ))

    return ctx
}
```

Then in your handler:

```go
func HandleChatRequest(ctx context.Context, req *ChatRequest) error {
    // Default client (Anthropic)
    main := llm.ClientFromContext(ctx)
    resp, err := main.Chat(ctx, req.Message)
    if err != nil {
        return err
    }

    // Named client (OpenAI)
    summary := llm.NamedClientFromContext(ctx, "summary")
    _, _ = summary.Chat(ctx, "Summarize: " + resp.Text)

    return nil
}
```

## Streaming

When the client has a `channel.Channel` (which it always does inside a channel handler), it publishes progress events as typed envelope messages over Centrifugo. These use the same envelope format as every other ShipQ channel — `{type: "...", data: {...}}` — so existing frontend hooks work out of the box.

| Envelope type | Data shape | Description |
|---------------|------------|-------------|
| `LLMTextDelta` | `{ "text": "..." }` | A chunk of streamed text from the model |
| `LLMToolCallStart` | `{ "tool_call_id": "...", "tool_name": "...", "input": {...} }` | The model is invoking a tool |
| `LLMToolCallResult` | `{ "tool_call_id": "...", "tool_name": "...", "output": {...}, "error": "...", "duration_ms": 123 }` | Tool execution finished |
| `LLMDone` | `{ "text": "...", "input_tokens": 100, "output_tokens": 50, "tool_call_count": 2 }` | Conversation complete |

Both OpenAI and Anthropic support SSE streaming. `LLMTextDelta` maps directly to streamed token chunks. If a provider doesn't support streaming, the client falls back to publishing the complete text as a single `LLMTextDelta` followed by `LLMDone`.

### Frontend usage

The generated TypeScript types include the LLM stream message types. Use them with the existing channel hooks:

```tsx
const { dispatch } = useChatbot({
    onLLMTextDelta: (data) => {
        // Append streaming token to the UI
        setResponse(prev => prev + data.text);
    },
    onLLMToolCallStart: (data) => {
        // Show tool invocation in progress
        setToolCalls(prev => [...prev, { name: data.tool_name, status: "running" }]);
    },
    onLLMToolCallResult: (data) => {
        // Update tool call status
        updateToolCall(data.tool_call_id, { status: "done", output: data.output });
    },
    onLLMDone: (data) => {
        // Conversation complete
        setIsStreaming(false);
    },
});

// Start a conversation
dispatch({ message: "What's the weather in Tokyo?" });
```

## Database Persistence

`shipq llm compile` generates two tables:

### `llm_conversations`

One row per call to `client.Chat()`. Links back to the `job_results` row via `job_id`.

| Column | Type | Description |
|--------|------|-------------|
| `id` | BIGINT PK | Internal primary key |
| `public_id` | VARCHAR UNIQUE | Nanoid for external reference |
| `job_id` | VARCHAR | References `job_results.public_id` |
| `channel_name` | VARCHAR | Channel that initiated this conversation |
| `account_id` | BIGINT (nullable) | Account that owns the conversation |
| `provider` | VARCHAR | Provider name (e.g., `openai`, `anthropic`) |
| `model` | VARCHAR | Model identifier (e.g., `claude-sonnet-4-20250514`) |
| `system_prompt` | TEXT (nullable) | System prompt used |
| `total_input_tokens` | INT | Sum of input tokens across all round-trips |
| `total_output_tokens` | INT | Sum of output tokens across all round-trips |
| `tool_call_count` | INT | Total tool calls made |
| `status` | VARCHAR | `running`, `completed`, or `failed` |
| `error_message` | TEXT (nullable) | Error message if failed |
| `started_at` | DATETIME | When the conversation started |
| `completed_at` | DATETIME (nullable) | When the conversation finished |

### `llm_messages`

One row per logical message. Tool calls and tool results are separate rows for a full audit trail.

| Column | Type | Description |
|--------|------|-------------|
| `id` | BIGINT PK | Internal primary key |
| `conversation_id` | BIGINT | FK → `llm_conversations.id` |
| `sequence` | INT | Ordering within the conversation |
| `role` | VARCHAR | `user`, `assistant`, `tool_call`, or `tool_result` |
| `content` | TEXT (nullable) | Text content for user/assistant messages |
| `tool_name` | VARCHAR (nullable) | Which tool (for `tool_call` / `tool_result`) |
| `tool_call_id` | VARCHAR (nullable) | Provider-assigned call ID for correlation |
| `tool_input` | JSON (nullable) | JSON arguments sent to the tool |
| `tool_output` | JSON (nullable) | JSON result returned by the tool |
| `tool_error` | TEXT (nullable) | Error message if the tool failed |
| `tool_duration_ms` | INT (nullable) | Tool execution time in milliseconds |
| `input_tokens` | INT | Input tokens for this round-trip |
| `output_tokens` | INT | Output tokens for this round-trip |

The `role` values are intentionally different from provider wire formats. Providers use `assistant` for both text responses and tool call requests; the database splits them into `assistant` and `tool_call` for clarity.

## Task DAGs (Tool Ordering)

Real-world agents have multi-step workflows where certain actions only make sense after prerequisite actions have completed. `WithTaskDAG` lets you declare these ordering constraints so the conversation loop enforces them automatically — filtering tools, guarding against hallucinated calls, and remembering progress across turns.

```go
g, _ := dag.New([]dag.Node[string]{
    {ID: "search_docs",  Description: "Search documentation"},
    {ID: "write_code",   Description: "Write code",
        HardDeps: []string{"search_docs"}},
    {ID: "run_tests",    Description: "Run tests",
        HardDeps: []string{"write_code"}},
})

client := llm.NewClient(provider,
    llm.WithTools(registry),
    llm.WithTaskDAG(g),
)
```

With a DAG configured, the loop automatically:
- **Filters tools** — only tools whose hard dependencies are satisfied appear in the request
- **Injects DAG context** into the system prompt so the model can plan ahead
- **Guards against hallucinated calls** — sends an error back if the model calls a blocked tool
- **Publishes `LLMToolsAvailable` events** with available/completed/blocked lists for UI rendering
- **Remembers progress across turns** when persistence is enabled (via `ListCompletedTools`)

Tools in the registry but not in the DAG are always available. See the full [Task DAGs](/guides/task-dags/) guide for details, examples, and the cross-turn persistence mechanism.

## The Conversation Loop

When you call `client.Chat(ctx, message)`, the following happens automatically:

1. **Create** an `llm_conversations` row (status = `running`)
2. **Persist** the user message → `llm_messages` (role = `user`)
3. **Loop** (up to `maxIterations`):
   - Build the provider request from conversation history + tools (filtered by DAG if configured)
   - Call the provider (streaming if supported)
   - Stream `LLMTextDelta` events to the channel as tokens arrive
   - Persist the assistant message → `llm_messages`
   - If the model is done (no tool calls) → exit loop
   - For each tool call:
     - Persist → `llm_messages` (role = `tool_call`)
     - Publish `LLMToolCallStart` to channel
     - Execute the tool function
     - Persist → `llm_messages` (role = `tool_result`)
     - Publish `LLMToolCallResult` to channel
   - Feed tool results back to the model for the next round-trip
4. **Finalize**: update `llm_conversations` (status = `completed`, token totals), publish `LLMDone`
5. **Return** `Response` to the caller

### Parallel tool calls

When the model requests multiple tool calls in one turn, they execute concurrently by default (using `errgroup`). Both OpenAI and Anthropic emit parallel tool calls. Use `WithSequentialToolCalls()` if your tools have side effects that must be ordered.

### DAG progression

When a task DAG is configured, completed tools are tracked after each tool-call round. After marking completions, the loop publishes an `LLMToolsAvailable` event and recomputes the available tool set for the next iteration — so tools can unlock mid-turn as their dependencies complete.

## JSON Schema Generation

ShipQ generates JSON Schema from your Go struct types automatically. The mapping is:

| Go type | JSON Schema |
|---------|-------------|
| `string` | `{"type": "string"}` |
| `int`, `int32`, `int64` | `{"type": "integer"}` |
| `float32`, `float64` | `{"type": "number"}` |
| `bool` | `{"type": "boolean"}` |
| `*T` | Schema of T, with `"null"` added to type |
| `[]T` | `{"type": "array", "items": <schema of T>}` |
| `struct{...}` | `{"type": "object", "properties": {...}, "required": [...]}` |
| `map[string]T` | `{"type": "object", "additionalProperties": <schema of T>}` |
| `time.Time` | `{"type": "string", "format": "date-time"}` |

For strict mode compatibility (both OpenAI and Anthropic):
- All non-pointer fields go into `required`
- Pointer fields get `["type", "null"]` and still go into `required`
- `additionalProperties: false` is set on all objects

## Providers

### OpenAI

```go
import "myapp/shipq/lib/llm/openai"

provider := openai.New(os.Getenv("OPENAI_API_KEY"), "gpt-4.1")
```

Supports: tool calling, vision (images), streaming (SSE).

### Anthropic

```go
import "myapp/shipq/lib/llm/anthropic"

provider := anthropic.New(os.Getenv("ANTHROPIC_API_KEY"), "claude-sonnet-4-20250514")
```

Supports: tool calling, vision (images), streaming (SSE), web search.

### Custom providers

Implement the `llm.Provider` interface to use any LLM backend:

```go
type Provider interface {
    Name() string
    Send(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
    SendStream(ctx context.Context, req *ProviderRequest) (<-chan StreamEvent, error)
    ModelName() string
}
```

## Testing

The `llm/llmtest` package provides test utilities that don't require real API keys:

### Mock provider

Script responses for deterministic tests:

```go
import "myapp/shipq/lib/llm/llmtest"

mock := llmtest.NewMockProvider()
mock.EnqueueResponse(&llm.ProviderResponse{
    Text: "The weather in Tokyo is 22°C and partly cloudy.",
    Done: true,
    Usage: llm.Usage{InputTokens: 50, OutputTokens: 20},
})

client := llm.NewClient(mock,
    llm.WithTools(weather.Registry()),
    llm.WithSystem("You are a weather assistant."),
)

resp, err := client.Chat(ctx, "What's the weather in Tokyo?")
```

### Recording provider

Wrap a real provider to record requests and responses for snapshot testing:

```go
recorder := llmtest.NewRecordingProvider(realProvider)
client := llm.NewClient(recorder, ...)

// After the test, inspect recorded interactions
for _, interaction := range recorder.Interactions() {
    fmt.Println(interaction.Request, interaction.Response)
}
```

### Assertion helpers

```go
llmtest.AssertToolCalled(t, resp, "get_weather")
llmtest.AssertToolNotCalled(t, resp, "get_forecast")
llmtest.AssertResponseContains(t, resp, "Tokyo")
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | Required if using the OpenAI provider |
| `ANTHROPIC_API_KEY` | Required if using the Anthropic provider |

These are read by your `Setup` function — not by the framework. You decide which env var to read and which provider to construct.

## Recompiling After Changes

When you add, remove, or change tool functions:

```sh
shipq llm compile
```

When you also changed channel definitions:

```sh
shipq workers compile
```

Or use the umbrella compile to run everything:

```sh
shipq compile
```

## Architecture Overview

LLM conversations are a specialization of channels. The existing infrastructure handles:
- **Real-time streaming** — `channel.Channel` publishes typed envelopes over Centrifugo
- **Job lifecycle** — `WrapDispatchHandler` manages the `job_results` row
- **Worker execution** — the task queue dispatches to the handler with a `context.Context`

The LLM layer adds three things on top:
1. **Tool registry** — compile-time schema generation + runtime dispatch
2. **Conversation loop** — the multi-turn request → tool_call → tool_result cycle
3. **Automatic persistence + streaming** — the loop writes every message to the DB and publishes every event to the channel

```
┌──────────────────────────────────────────────┐
│                User Code                      │
│  Tool functions + Register() + Setup()        │
│  + Channel handler calling client.Chat()      │
└──────────────────┬───────────────────────────┘
                   │
      shipq llm compile (build-time)
                   │
                   ▼
┌──────────────────────────────────────────────┐
│             Generated Code                    │
│  JSON Schemas, typed dispatchers, persister,  │
│  migrations, querydefs, stream types          │
└──────────────────┬───────────────────────────┘
                   │
              runtime (library)
                   │
                   ▼
┌──────────────────────────────────────────────┐
│              llm.Client                       │
│                                               │
│  ┌─────────────┐ ┌────────┐ ┌────────────┐  │
│  │  Channel     │ │ DB     │ │ Tool       │  │
│  │  (streaming) │ │ (save) │ │ Registry   │  │
│  └─────────────┘ └────────┘ └────────────┘  │
│                                               │
│  Provider adapters:                           │
│  ┌────────┐  ┌───────────┐  ┌────────┐      │
│  │ OpenAI │  │ Anthropic │  │ Custom │      │
│  └────────┘  └───────────┘  └────────┘      │
└──────────────────────────────────────────────┘
```

## File Ownership

| File pattern | Owner | Editable? |
|-------------|-------|-----------|
| `tools/*/register.go` | You | Yes — add/remove tool registrations |
| `tools/*/*.go` (tool functions) | You | Yes — your business logic |
| `channels/*/setup.go` | You | Yes — provider wiring |
| `tools/*/zz_generated_*.go` | ShipQ | No — regenerated by `shipq llm compile` |
| `shipq/lib/llmpersist/zz_generated_*.go` | ShipQ | No — regenerated |
| `migrations/*_llm_tables.go` | ShipQ | No — generated once |

## Next Steps

- [Task DAGs](/guides/task-dags/) — enforce tool ordering constraints in multi-step agent workflows
- [Workers & Channels](/guides/workers/) — set up the channel infrastructure that LLM tools build on
- [Configuration](/concepts/configuration/) — understand `shipq.ini` and the `[llm]` section
- [CLI Commands](/reference/cli/) — full reference for `shipq llm compile`
- [The Compiler Chain](/concepts/compiler-chain/) — how the LLM compiler fits into the build pipeline
