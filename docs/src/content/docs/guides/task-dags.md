---
title: Task DAGs
description: Enforce tool ordering constraints in LLM agent workflows with dependency graphs.
---

ShipQ's Task DAG feature lets you declare ordering constraints between LLM tools — "the agent can call `search_docs` immediately, but `write_code` requires `search_docs` to complete first, and `submit_pr` requires both `write_code` and `run_tests`." The conversation loop automatically filters tools, guards against hallucinated calls, and remembers progress across turns.

## Why Task DAGs?

Without ordering constraints, models can call tools in any order. System prompts can _ask_ models to follow a sequence, but LLMs don't follow instructions reliably. Structured constraints are more trustworthy than prose:

- **Prevent bad sequences** — the model can't call `submit_pr` before running tests
- **Programmatic visibility** — the loop can answer "what can the agent do next?" at any point, useful for UI rendering and debugging
- **Automatic memory** — when persistence is enabled, DAG progress carries across conversation turns without any extra code

## Prerequisites

- **LLM tools** set up (see [LLM Tools](/guides/llm-tools/))
- Familiarity with the **`dag` package** — ShipQ ships a generic `dag` library that the Task DAG feature builds on

## Quick Start

```go
package assistant

import (
    "context"
    "os"

    "myapp/shipq/lib/dag"
    "myapp/shipq/lib/llm"
    "myapp/shipq/lib/llm/anthropic"
    "myapp/tools/coding"
)

func Setup(ctx context.Context) context.Context {
    provider := anthropic.New(
        os.Getenv("ANTHROPIC_API_KEY"),
        "claude-sonnet-4-20250514",
    )
    registry := coding.Registry()

    // Define the task DAG: what depends on what.
    g, err := dag.New([]dag.Node[string]{
        {ID: "search_docs",  Description: "Search documentation"},
        {ID: "list_files",   Description: "List repository files"},
        {ID: "write_code",   Description: "Write code",
            HardDeps: []string{"search_docs"}},
        {ID: "run_tests",    Description: "Run tests",
            HardDeps: []string{"write_code"}},
        {ID: "submit_pr",    Description: "Submit a pull request",
            HardDeps: []string{"write_code", "run_tests"}},
    })
    if err != nil {
        panic("invalid task DAG: " + err.Error())
    }

    client := llm.NewClient(provider,
        llm.WithTools(registry),
        llm.WithTaskDAG(g),
        llm.WithSystem("You are a coding assistant."),
    )

    return llm.WithClient(ctx, client)
}
```

That's it. The conversation loop handles everything else — filtering tools, injecting DAG context into the system prompt, guarding against hallucinated calls, and publishing progress events.

## How It Works

### Tool Filtering

On each iteration of the conversation loop, the client computes which tools are available:

1. **Root nodes** (no hard dependencies) are available immediately
2. **Dependent nodes** become available once all their hard dependencies have completed
3. **Completed tools remain available** — the model can retry or call them with different arguments
4. **Ungoverned tools** (registered in the registry but not in the DAG) are always available

Only available tools are included in the `ProviderRequest.Tools` slice sent to the model. Blocked tools are simply omitted — the model never sees them.

### System Prompt Injection

When a DAG is configured, the client automatically appends a section to the system prompt describing:

- Which tools have prerequisites and what those prerequisites are
- Which tools are currently available
- Which tools have soft dependencies (recommended but not required)

This helps the model understand why certain tools aren't available and plan ahead.

### Blocked Tool Guard

Even though blocked tools are omitted from the request, some models occasionally hallucinate tool calls for tools not in the current request. When this happens:

1. The client detects the unsatisfied dependencies
2. An error message is sent back to the model as the tool result (not a fatal error)
3. The model can then course-correct and call an available tool instead
4. The blocked call is logged with an error in the `Response.ToolCalls` slice

### Stream Events

When a DAG is configured, the client publishes `LLMToolsAvailable` events after each tool-call round. These events contain three lists:

| Field | Description |
|-------|-------------|
| `available` | Tool names that can be called right now |
| `completed` | Tool names that have already been called |
| `blocked` | Tool names waiting on unsatisfied dependencies |

Frontend clients can use these events to render progress indicators, enable/disable tool buttons, or show a visual workflow.

```typescript
// In your React hook options:
onLLMToolsAvailable: (msg) => {
    console.log("Available:", msg.available);
    console.log("Completed:", msg.completed);
    console.log("Blocked:", msg.blocked);
}
```

## DAG Construction

### Nodes

Each node in the DAG represents a tool, keyed by its name (matching `ToolDef.Name`):

```go
dag.Node[string]{
    ID:          "write_code",
    Description: "Write code to a file",
    HardDeps:    []string{"search_docs"},
    SoftDeps:    []string{"read_file"},
}
```

### Hard Dependencies

Hard dependencies (`HardDeps`) **must** be completed before the dependent tool becomes available. If `write_code` has a hard dependency on `search_docs`, the model cannot call `write_code` until `search_docs` has been successfully invoked.

### Soft Dependencies

Soft dependencies (`SoftDeps`) are informational — they appear in the system prompt as recommendations but do not block the tool. Use soft deps for "it would be better to do X first, but it's not required."

### Validation

`dag.New()` validates the graph at construction time:

- **No duplicate IDs** — panics with `DuplicateIDError`
- **No dangling references** — every dependency must reference an existing node (`DanglingDepError`)
- **No cycles** — the graph must be a DAG (`CycleError`)

Additionally, `llm.NewClient` validates that every DAG node ID matches a tool in the registry. This catches typos at wiring time:

```go
// This panics: "nonexistent_tool" is not in the registry.
llm.NewClient(provider,
    llm.WithTools(registry),
    llm.WithTaskDAG(dagWithNonexistentTool),
)
```

## Partial DAGs

Not every tool needs to be in the DAG. Tools registered in the registry but not mentioned in the DAG are **always available** — they have no ordering constraints:

```go
// Only 2 of 8 tools have ordering constraints.
g, _ := dag.New([]dag.Node[string]{
    {ID: "verify_identity", Description: "Verify customer identity"},
    {ID: "issue_refund",    Description: "Issue a refund",
        HardDeps: []string{"verify_identity"}},
})

client := llm.NewClient(provider,
    llm.WithTools(supportRegistry), // has 8 tools total
    llm.WithTaskDAG(g),             // only 2 are governed
)
```

In this case, 6 of the 8 tools are always available. Only `issue_refund` is gated behind `verify_identity`.

## Cross-Turn Persistence

When a persister and channel are both configured (the normal production case), the DAG automatically remembers which tools have been completed across conversation turns.

### How It Works

1. Every tool invocation is persisted as a `tool_call` message in `llm_messages` (this already happens as part of normal LLM persistence)
2. At the start of each `run()` call, the client calls `persister.ListCompletedTools(ctx, jobID)` to find all tool names from prior conversations on the same channel job
3. These prior completions are loaded into the `completedTools` set before the main loop begins
4. The DAG picks up where it left off — no code needed from the user

### Example: Multi-Turn Workflow

**Turn 1** — user sends "Research and write a report about photosynthesis":

- `completedTools = {}` (fresh start)
- Available: `lookup_info`
- Model calls `lookup_info` → completes
- Available: `lookup_info`, `write_report`
- Model calls `write_report` → completes

**Turn 2** — user sends "Actually, make the report longer" (same channel job):

- `completedTools = {lookup_info, write_report}` (hydrated from DB)
- Available: `lookup_info`, `write_report` (both already unlocked)
- Model calls `write_report` again with expanded content

No extra code needed. The persistence layer does all the bookkeeping.

### Graceful Degradation

The hydration step is resilient:

| Scenario | Behavior |
|----------|----------|
| No persister configured | `completedTools` starts empty (same as single-turn) |
| No channel configured | `completedTools` starts empty (no job ID to query) |
| Persister query fails | `completedTools` starts empty (non-fatal, logged) |
| Empty job ID | `completedTools` starts empty |

## Conversation Flow Example

Given this DAG:

```
  ┌─────────────┐     ┌────────────┐
  │ search_docs  │     │ list_files │
  └──────┬───────┘     └──────┬─────┘
         │                    │
         │              ┌─────┴──────┐
         │              │ read_file  │
         │              └─────┬──────┘
         │                    ╎ (soft dep)
         ▼                    ╎
  ┌──────────────┐◀╌╌╌╌╌╌╌╌╌╌┘
  │  write_code  │
  └──────┬───────┘
         │
         ▼
  ┌──────────────┐
  │  run_tests   │
  └──────┬───────┘
         │
         ▼
  ┌──────────────┐
  │  submit_pr   │
  └──────────────┘

  Legend:
    ──▶  hard dependency (must complete first)
    ╌╌▶  soft dependency (recommended but not required)
```

The conversation progresses like this:

| Turn | Completed | Available | Model Calls |
|------|-----------|-----------|-------------|
| 1 | ∅ | `search_docs`, `list_files` | `search_docs`, `list_files` |
| 2 | `search_docs`, `list_files` | + `read_file`, `write_code` | `read_file` |
| 3 | + `read_file` | (same + `write_code`) | `write_code` |
| 4 | + `write_code` | + `run_tests` | `run_tests` |
| 5 | + `run_tests` | + `submit_pr` | `submit_pr` |
| 6 | all | all | (final text response) |

## Opt-In Design

The DAG is entirely opt-in:

- Existing code that doesn't declare a DAG continues to work exactly as before — all tools are available on every turn
- `WithTaskDAG` is one additional option on `llm.NewClient`
- No codegen is involved — the DAG is built at runtime in your Setup function

## Client Options Reference

| Option | Description |
|--------|-------------|
| `WithTaskDAG(g)` | Attach a `*dag.Graph[string]` to control tool availability |

This is used in combination with the existing options documented in [LLM Tools — Client Options](/guides/llm-tools/#client-options).

## The `dag` Package

The `dag` package is a general-purpose dependency graph library. It's already embedded into user projects by `EmbedAllPackages`. Key API:

| Function/Method | Description |
|----------------|-------------|
| `dag.New(nodes)` | Construct and validate a graph |
| `g.Available(satisfied)` | "What can I do next?" — returns nodes whose hard deps are met |
| `g.CheckHardDeps(id, satisfied)` | Which hard deps are unsatisfied? |
| `g.Find(id)` | Look up a node by ID |
| `g.Nodes()` | Get all nodes |
| `g.TopologicalOrder()` | All node IDs in valid topological order |
| `g.TransitiveDeps(id)` | All transitive hard dependencies |
| `g.Dependents(id)` | Nodes that directly depend on a given node |

The graph is immutable after construction. All query methods are safe to call concurrently.

## What Task DAGs Do NOT Cover

- **DAG visualization in the admin UI** — the `LLMToolsAvailable` stream event gives the frontend enough data to build a progress indicator, but the UI component is out of scope
- **Cross-session memory** — DAG progress is scoped to a single channel job (`job_id`). Starting a new session resets the DAG
- **Conditional edges** — the DAG is static. Dynamic edges ("if tool A returns error, unlock tool D instead") would require a different abstraction
- **Execution ordering within a round** — the DAG controls which tools are _presented_, not the order of co-available tools within a single round-trip. Parallel tools still execute concurrently unless `WithSequentialToolCalls()` is set

## Next Steps

- [LLM Tools](/guides/llm-tools/) — the full LLM integration guide
- [Workers & Channels](/guides/workers/) — the channel infrastructure that LLM tools build on
