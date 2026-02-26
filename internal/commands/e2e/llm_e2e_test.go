package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePersistenceRoundtripTest writes a Go test file into the generated project
// that exercises InsertConversation + InsertMessage against a real SQLite DB.
// This test is picked up by `go test ./...` in the E2E scenario.
func writePersistenceRoundtripTest(t *testing.T, projectDir string) {
	t.Helper()
	mod := readModulePath(t, projectDir)
	dir := filepath.Join(projectDir, "llmpersist_test")
	mustMkdirAll(t, dir)
	mustWriteFile(t, filepath.Join(dir, "persist_roundtrip_test.go"), `package llmpersist_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"`+mod+`/shipq/lib/llm"
	"`+mod+`/shipq/lib/llmpersist"
	dbrunner "`+mod+`/shipq/queries/sqlite"

	_ "modernc.org/sqlite"
)

func TestPersistenceRoundtrip(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	// Run migrations via the schema embedded in the migration files.
	// We apply the DDL directly since we know the SQLite schema.
	for _, ddl := range []string{
		`+"`"+`CREATE TABLE "llm_conversations" (
			"id" INTEGER PRIMARY KEY,
			"public_id" TEXT NOT NULL UNIQUE,
			"job_id" TEXT NOT NULL,
			"channel_name" TEXT NOT NULL,
			"account_id" INTEGER,
			"provider" TEXT NOT NULL,
			"model" TEXT NOT NULL,
			"system_prompt" TEXT,
			"total_input_tokens" INTEGER NOT NULL DEFAULT 0,
			"total_output_tokens" INTEGER NOT NULL DEFAULT 0,
			"tool_call_count" INTEGER NOT NULL DEFAULT 0,
			"status" TEXT NOT NULL DEFAULT 'running',
			"error_message" TEXT,
			"started_at" TEXT NOT NULL,
			"completed_at" TEXT,
			"created_at" TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			"updated_at" TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`+"`"+`,
		`+"`"+`CREATE TABLE "llm_messages" (
			"id" INTEGER PRIMARY KEY,
			"public_id" TEXT NOT NULL UNIQUE,
			"conversation_id" INTEGER NOT NULL,
			"role" TEXT NOT NULL,
			"content" TEXT,
			"tool_name" TEXT,
			"tool_call_id" TEXT,
			"tool_input" TEXT,
			"tool_output" TEXT,
			"tool_error" TEXT,
			"tool_duration_ms" INTEGER,
			"input_tokens" INTEGER NOT NULL DEFAULT 0,
			"output_tokens" INTEGER NOT NULL DEFAULT 0,
			"created_at" TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
			"updated_at" TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		)`+"`"+`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("exec DDL: %v", err)
		}
	}

	runner := dbrunner.NewQueryRunner(db)
	persister := llmpersist.New(runner)
	ctx := context.Background()

	// 1. InsertConversation
	row, err := persister.InsertConversation(ctx, llm.InsertConversationParams{
		PublicID:    "conv-roundtrip-1",
		JobID:       "job-1",
		ChannelName: "chatbot",
		AccountID:   0,
		Provider:    "test-provider",
		Model:       "test-model",
		Status:      llm.StatusRunning,
		StartedAt:   time.Now(),
	})
	if err != nil {
		t.Fatalf("InsertConversation: %v", err)
	}
	if row.ID == 0 {
		t.Fatal("InsertConversation returned zero ID")
	}
	if row.PublicID != "conv-roundtrip-1" {
		t.Fatalf("InsertConversation PublicID = %q, want %q", row.PublicID, "conv-roundtrip-1")
	}

	// 2. InsertMessage (user)
	err = persister.InsertMessage(ctx, llm.InsertMessageParams{
		PublicID:       "msg-1",
		ConversationID: row.ID,
		Role:           llm.RoleUser,
		Content:        "Hello, world!",
	})
	if err != nil {
		t.Fatalf("InsertMessage (user): %v", err)
	}

	// 3. InsertMessage (assistant)
	err = persister.InsertMessage(ctx, llm.InsertMessageParams{
		PublicID:       "msg-2",
		ConversationID: row.ID,
		Role:           llm.RoleAssistant,
		Content:        "Hi there!",
	})
	if err != nil {
		t.Fatalf("InsertMessage (assistant): %v", err)
	}

	// 4. Verify messages exist with correct roles, ordered by created_at
	rows, err := db.Query(
		"SELECT role FROM llm_messages WHERE conversation_id = ? ORDER BY created_at ASC",
		row.ID,
	)
	if err != nil {
		t.Fatalf("query messages: %v", err)
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			t.Fatalf("scan role: %v", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}

	if len(roles) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(roles))
	}
	if roles[0] != "user" {
		t.Errorf("message[0] role = %q, want %q", roles[0], "user")
	}
	if roles[1] != "assistant" {
		t.Errorf("message[1] role = %q, want %q", roles[1], "assistant")
	}
}
`)
}

// ── Helper functions ─────────────────────────────────────────────────────────

// readModulePath reads the Go module path from go.mod in projectDir.
func readModulePath(t *testing.T, projectDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}
	t.Fatal("could not find 'module' line in go.mod")
	return ""
}

// mustMkdirAll creates a directory (and all parents), failing the test on error.
func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

// mustWriteFile writes content to path, failing the test on error.
func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ── Tool package writers ─────────────────────────────────────────────────────

func writeWeatherToolPkg(t *testing.T, projectDir string) {
	t.Helper()
	mod := readModulePath(t, projectDir)
	dir := filepath.Join(projectDir, "tools", "weather")
	mustMkdirAll(t, dir)
	mustWriteFile(t, filepath.Join(dir, "weather.go"), `package weather

import (
    "context"
    "fmt"

    "`+mod+`/shipq/lib/llm"
)

// WeatherInput is the input to the get_weather tool.
type WeatherInput struct {
    City    string `+"`"+`json:"city"    desc:"The city to get weather for"`+"`"+`
    Country string `+"`"+`json:"country" desc:"ISO 3166-1 alpha-2 country code, e.g. US"`+"`"+`
}

// WeatherOutput is the result of the get_weather tool.
type WeatherOutput struct {
    TempC       float64 `+"`"+`json:"temp_c"`+"`"+`
    Humidity    int     `+"`"+`json:"humidity"`+"`"+`
    Description string  `+"`"+`json:"description"`+"`"+`
}

// GetWeather returns stub weather data for any city.
// Replace this with a real weather API call in production.
func GetWeather(ctx context.Context, input *WeatherInput) (*WeatherOutput, error) {
    return &WeatherOutput{
        TempC:       22.5,
        Humidity:    65,
        Description: fmt.Sprintf("Sunny with light clouds in %s, %s", input.City, input.Country),
    }, nil
}

// Register wires the weather tools into an llm.App.
func Register(app *llm.App) {
    app.Tool("get_weather", "Get the current weather for a city", GetWeather)
}
`)
}

func writeCalculatorToolPkg(t *testing.T, projectDir string) {
	t.Helper()
	mod := readModulePath(t, projectDir)
	dir := filepath.Join(projectDir, "tools", "calculator")
	mustMkdirAll(t, dir)
	mustWriteFile(t, filepath.Join(dir, "calculator.go"), `package calculator

import (
    "context"
    "fmt"
    "strconv"
    "strings"

    "`+mod+`/shipq/lib/llm"
)

// CalculateInput is the input to the calculate tool.
type CalculateInput struct {
    Expression string `+"`"+`json:"expression" desc:"A simple math expression: two numbers separated by +, -, *, or /. Example: 12 * 34"`+"`"+`
}

// CalculateOutput is the result of the calculate tool.
type CalculateOutput struct {
    Result float64 `+"`"+`json:"result"`+"`"+`
    Error  string  `+"`"+`json:"error,omitempty"`+"`"+`
}

// Calculate evaluates a simple two-operand expression.
func Calculate(ctx context.Context, input *CalculateInput) (*CalculateOutput, error) {
    parts := strings.Fields(input.Expression)
    if len(parts) != 3 {
        return &CalculateOutput{Error: "expected format: <number> <op> <number>"}, nil
    }
    a, err := strconv.ParseFloat(parts[0], 64)
    if err != nil {
        return nil, fmt.Errorf("invalid number %q: %w", parts[0], err)
    }
    b, err := strconv.ParseFloat(parts[2], 64)
    if err != nil {
        return nil, fmt.Errorf("invalid number %q: %w", parts[2], err)
    }
    switch parts[1] {
    case "+":
        return &CalculateOutput{Result: a + b}, nil
    case "-":
        return &CalculateOutput{Result: a - b}, nil
    case "*":
        return &CalculateOutput{Result: a * b}, nil
    case "/":
        if b == 0 {
            return &CalculateOutput{Error: "division by zero"}, nil
        }
        return &CalculateOutput{Result: a / b}, nil
    default:
        return &CalculateOutput{Error: fmt.Sprintf("unknown operator %q", parts[1])}, nil
    }
}

// Register wires the calculator tools into an llm.App.
func Register(app *llm.App) {
    app.Tool("calculate", "Evaluate a simple math expression (number op number)", Calculate)
}
`)
}

// ── Chatbot channel writer ───────────────────────────────────────────────────

func writeChatbotChannel(t *testing.T, projectDir string) {
	t.Helper()
	mod := readModulePath(t, projectDir)
	dir := filepath.Join(projectDir, "channels", "chatbot")
	mustMkdirAll(t, dir)

	// register.go — declares the channel's message types and visibility
	mustWriteFile(t, filepath.Join(dir, "register.go"), `package chatbot

import "`+mod+`/shipq/lib/channel"

// ChatRequest is dispatched by the client to start a conversation.
type ChatRequest struct {
    Message string `+"`"+`json:"message"`+"`"+`
}

// ChatResponse carries the assistant's final answer back to the client.
// LLM stream events (LLMTextDelta, LLMToolCallStart, etc.) are sent
// automatically by the llm.Client before this message is sent.
type ChatResponse struct {
    Text string `+"`"+`json:"text"`+"`"+`
}

func Register(app *channel.App) {
    app.DefineChannel("chatbot",
        channel.FromClient(ChatRequest{}),
        channel.FromServer(ChatResponse{}),
    ).Public(channel.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 10})
}
`)

	// handler.go — the channel handler that drives the LLM conversation
	mustWriteFile(t, filepath.Join(dir, "handler.go"), `package chatbot

import (
    "context"
    "encoding/json"

    "`+mod+`/shipq/lib/channel"
    "`+mod+`/shipq/lib/llm"
)

// HandleChatRequest is the dispatch handler for the chatbot channel.
// It retrieves the LLM client wired by Setup, runs the conversation
// (which streams LLM events to the channel automatically), then sends
// the final ChatResponse.
func HandleChatRequest(ctx context.Context, req *ChatRequest) error {
    ch := channel.FromContext(ctx)
    client := llm.ClientFromContext(ctx)

    resp, err := client.Chat(ctx, req.Message)
    if err != nil {
        return err
    }

    data, err := json.Marshal(ChatResponse{Text: resp.Text})
    if err != nil {
        return err
    }
    return ch.Send(ctx, "ChatResponse", data)
}
`)

	// setup.go — wires the LLM client (provider + tools + persistence) into context.
	// Setup is called by WrapDispatchHandler before each handler invocation.
	// It reads ANTHROPIC_API_KEY from the environment; if not set, a placeholder
	// is used so the project compiles and unit-tests pass without a real key.
	mustWriteFile(t, filepath.Join(dir, "setup.go"), `package chatbot

import (
    "context"
    "os"

    "`+mod+`/shipq/lib/channel"
    "`+mod+`/shipq/lib/llm"
    "`+mod+`/shipq/lib/llm/anthropic"
    "`+mod+`/shipq/lib/llmpersist"
    dbrunner "`+mod+`/shipq/queries/sqlite"
    "`+mod+`/tools/calculator"
    "`+mod+`/tools/weather"
)

// Setup is called before each HandleChatRequest invocation. It builds the
// llm.Client from the channel context (transport) and DB context (persistence)
// and stores it in context for the handler to retrieve.
func Setup(ctx context.Context) context.Context {
    ch := channel.FromContext(ctx)
    db := channel.DBFromContext(ctx)

    app := llm.NewApp()
    weather.Register(app)
    calculator.Register(app)

    opts := []llm.Option{
        llm.WithTools(app.Registry()),
        llm.WithChannel(ch),
        llm.WithSystem("You are a helpful assistant. You have access to weather and calculator tools. Use them when the user's request requires it."),
    }
    if db != nil {
        opts = append(opts, llm.WithPersister(llmpersist.New(dbrunner.NewQueryRunner(db))))
    }

    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        apiKey = "sk-ant-placeholder-for-tests"
    }

    client := llm.NewClient(
        anthropic.New(apiKey, "claude-sonnet-4-20250514"),
        opts...,
    )
    return llm.WithClient(ctx, client)
}
`)
}

// ── Configuration helpers ────────────────────────────────────────────────────

func appendLLMConfig(t *testing.T, projectDir string) {
	t.Helper()
	mod := readModulePath(t, projectDir)
	iniPath := filepath.Join(projectDir, "shipq.ini")
	existing, err := os.ReadFile(iniPath)
	if err != nil {
		t.Fatalf("read shipq.ini: %v", err)
	}
	section := fmt.Sprintf(
		"\n[llm]\ntool_pkgs = %s/tools/weather, %s/tools/calculator\n",
		mod, mod,
	)
	if err := os.WriteFile(iniPath, append(existing, []byte(section)...), 0o644); err != nil {
		t.Fatalf("write shipq.ini: %v", err)
	}
}

// ── React frontend writer ────────────────────────────────────────────────────

func writeReactFrontend(t *testing.T, projectDir string) {
	t.Helper()
	frontendDir := filepath.Join(projectDir, "frontend")
	mustMkdirAll(t, frontendDir)
	mustMkdirAll(t, filepath.Join(frontendDir, "src"))

	// package.json — Vite + React + centrifuge-js
	mustWriteFile(t, filepath.Join(frontendDir, "package.json"), `{
  "name": "shipq-llm-chat",
  "private": true,
  "version": "0.0.1",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "centrifuge": "^5.5.3",
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@types/react": "^18.3.3",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.1",
    "typescript": "^5.5.3",
    "vite": "^5.3.4"
  }
}
`)

	// tsconfig.json
	mustWriteFile(t, filepath.Join(frontendDir, "tsconfig.json"), `{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": false,
    "noUnusedParameters": false,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src"]
}
`)

	// frontend/.gitignore
	mustWriteFile(t, filepath.Join(frontendDir, ".gitignore"), `node_modules
`)

	// vite.config.ts — proxy API and Centrifugo WS to local services
	mustWriteFile(t, filepath.Join(frontendDir, "vite.config.ts"), `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
});
`)

	// index.html
	mustWriteFile(t, filepath.Join(frontendDir, "index.html"), `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>ShipQ LLM Chat</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
`)

	// src/main.tsx
	mustWriteFile(t, filepath.Join(frontendDir, "src", "main.tsx"), `import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
`)

	// src/App.tsx — the chat UI with streaming LLM events
	mustWriteFile(t, filepath.Join(frontendDir, "src", "App.tsx"), `import React, { useState, useRef, useEffect } from "react";
import { configure, dispatchChatbot, type ChatbotChannel } from "./shipq-channels";

// Configure the channel client to talk to the local server.
// The Vite proxy forwards /api/* to localhost:8080 with path rewrite.
// Centrifugo WS is accessed directly (no proxy needed for WS).
configure({
  baseURL: "/api",
  centrifugoURL: "ws://localhost:8000/connection/websocket",
});

// ─── Types for display ──────────────────────────────────────────────────────

interface StreamEvent {
  kind: "text" | "tool_start" | "tool_result" | "done" | "response" | "error";
  text?: string;
  toolCallId?: string;
  toolName?: string;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  durationMs?: number;
  inputTokens?: number;
  outputTokens?: number;
  toolCallCount?: number;
}

// ─── App Component ──────────────────────────────────────────────────────────

export default function App() {
  const [message, setMessage] = useState("");
  const [events, setEvents] = useState<StreamEvent[]>([]);
  const [streamingText, setStreamingText] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const channelRef = useRef<ChatbotChannel | null>(null);
  const eventsEndRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when events change
  useEffect(() => {
    eventsEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [events, streamingText]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!message.trim() || isStreaming) return;

    const userMessage = message.trim();
    setMessage("");
    setEvents([]);
    setStreamingText("");
    setError(null);
    setIsStreaming(true);

    // Disconnect previous channel if any
    if (channelRef.current) {
      channelRef.current.unsubscribe();
      channelRef.current = null;
    }

    try {
      const ch = await dispatchChatbot({ message: userMessage });
      channelRef.current = ch;

      // LLM text streaming — accumulate deltas
      ch.onLLMTextDelta((msg) => {
        setStreamingText((prev) => prev + msg.text);
      });

      // Tool call started
      ch.onLLMToolCallStart((msg) => {
        // Flush any accumulated streaming text first
        setStreamingText((prev) => {
          if (prev) {
            setEvents((evts) => [...evts, { kind: "text", text: prev }]);
          }
          return "";
        });
        setEvents((evts) => [
          ...evts,
          {
            kind: "tool_start",
            toolCallId: msg.tool_call_id,
            toolName: msg.tool_name,
            input: msg.input,
          },
        ]);
      });

      // Tool call result
      ch.onLLMToolCallResult((msg) => {
        setEvents((evts) => [
          ...evts,
          {
            kind: "tool_result",
            toolCallId: msg.tool_call_id,
            toolName: msg.tool_name,
            output: msg.output,
            error: msg.error,
            durationMs: msg.duration_ms,
          },
        ]);
      });

      // LLM done — token usage summary
      ch.onLLMDone((msg) => {
        // Flush remaining streaming text
        setStreamingText((prev) => {
          if (prev) {
            setEvents((evts) => [...evts, { kind: "text", text: prev }]);
          }
          return "";
        });
        setEvents((evts) => [
          ...evts,
          {
            kind: "done",
            text: msg.text,
            inputTokens: msg.input_tokens,
            outputTokens: msg.output_tokens,
            toolCallCount: msg.tool_call_count,
          },
        ]);
      });

      // Final chat response
      ch.onChatResponse((msg) => {
        setEvents((evts) => [
          ...evts,
          { kind: "response", text: msg.text },
        ]);
        setIsStreaming(false);
      });
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      setIsStreaming(false);
    }
  };

  return (
    <div style={styles.container}>
      <h1 style={styles.title}>ShipQ LLM Chat</h1>
      <p style={styles.subtitle}>
        Weather &amp; Calculator tools &middot; Streaming via Centrifugo
      </p>

      {/* Event stream */}
      <div style={styles.eventStream}>
        {events.map((evt, i) => (
          <EventCard key={i} event={evt} />
        ))}

        {/* Live streaming text */}
        {streamingText && (
          <div style={{ ...styles.card, ...styles.textCard }}>
            <span style={styles.badge}>streaming</span>
            <pre style={styles.pre}>{streamingText}<span style={styles.cursor}>▊</span></pre>
          </div>
        )}

        {error && (
          <div style={{ ...styles.card, ...styles.errorCard }}>
            <strong>Error:</strong> {error}
          </div>
        )}

        <div ref={eventsEndRef} />
      </div>

      {/* Input form */}
      <form onSubmit={handleSubmit} style={styles.form}>
        <input
          type="text"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Ask about weather or math..."
          style={styles.input}
          disabled={isStreaming}
        />
        <button type="submit" style={styles.button} disabled={isStreaming}>
          {isStreaming ? "Streaming..." : "Send"}
        </button>
      </form>

      <div style={styles.hints}>
        <strong>Try:</strong>
        <ul>
          <li>"What's the weather in Tokyo?"</li>
          <li>"Calculate 42 * 17"</li>
          <li>"What's the weather in Paris and what is 100 / 7?"</li>
        </ul>
      </div>
    </div>
  );
}

// ─── Event Card ─────────────────────────────────────────────────────────────

function EventCard({ event }: { event: StreamEvent }) {
  switch (event.kind) {
    case "text":
      return (
        <div style={{ ...styles.card, ...styles.textCard }}>
          <span style={styles.badge}>text</span>
          <pre style={styles.pre}>{event.text}</pre>
        </div>
      );
    case "tool_start":
      return (
        <div style={{ ...styles.card, ...styles.toolStartCard }}>
          <span style={styles.badge}>🔧 tool call</span>
          <strong>{event.toolName}</strong>
          <pre style={styles.pre}>{JSON.stringify(event.input, null, 2)}</pre>
        </div>
      );
    case "tool_result":
      return (
        <div style={{ ...styles.card, ...(event.error ? styles.errorCard : styles.toolResultCard) }}>
          <span style={styles.badge}>
            {event.error ? "❌ tool error" : "✅ tool result"}
          </span>
          <strong>{event.toolName}</strong>
          {event.error ? (
            <pre style={styles.pre}>{event.error}</pre>
          ) : (
            <pre style={styles.pre}>{JSON.stringify(event.output, null, 2)}</pre>
          )}
          <small style={styles.meta}>{event.durationMs}ms</small>
        </div>
      );
    case "done":
      return (
        <div style={{ ...styles.card, ...styles.doneCard }}>
          <span style={styles.badge}>📊 done</span>
          <small style={styles.meta}>
            {event.inputTokens} input tokens &middot;{" "}
            {event.outputTokens} output tokens &middot;{" "}
            {event.toolCallCount} tool calls
          </small>
        </div>
      );
    case "response":
      return (
        <div style={{ ...styles.card, ...styles.responseCard }}>
          <span style={styles.badge}>💬 response</span>
          <pre style={styles.pre}>{event.text}</pre>
        </div>
      );
    case "error":
      return (
        <div style={{ ...styles.card, ...styles.errorCard }}>
          <span style={styles.badge}>error</span>
          <pre style={styles.pre}>{event.text}</pre>
        </div>
      );
    default:
      return null;
  }
}

// ─── Styles ─────────────────────────────────────────────────────────────────

const styles: Record<string, React.CSSProperties> = {
  container: {
    maxWidth: 720,
    margin: "0 auto",
    padding: "2rem 1rem",
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
  },
  title: { margin: 0, fontSize: "1.8rem" },
  subtitle: { color: "#666", marginTop: 4, marginBottom: 24 },
  eventStream: {
    border: "1px solid #ddd",
    borderRadius: 8,
    padding: 16,
    minHeight: 200,
    maxHeight: 500,
    overflowY: "auto" as const,
    background: "#fafafa",
    marginBottom: 16,
  },
  card: {
    borderRadius: 6,
    padding: "10px 14px",
    marginBottom: 8,
    fontSize: 14,
  },
  textCard: { background: "#fff", border: "1px solid #e0e0e0" },
  toolStartCard: { background: "#fff8e1", border: "1px solid #ffe082" },
  toolResultCard: { background: "#e8f5e9", border: "1px solid #a5d6a7" },
  doneCard: { background: "#e3f2fd", border: "1px solid #90caf9" },
  responseCard: { background: "#f3e5f5", border: "1px solid #ce93d8" },
  errorCard: { background: "#ffebee", border: "1px solid #ef9a9a" },
  badge: {
    display: "inline-block",
    fontSize: 11,
    fontWeight: 600,
    textTransform: "uppercase" as const,
    letterSpacing: "0.05em",
    color: "#555",
    marginBottom: 4,
  },
  pre: {
    margin: "4px 0 0",
    whiteSpace: "pre-wrap" as const,
    wordBreak: "break-word" as const,
    fontFamily: "'SF Mono', 'Fira Code', monospace",
    fontSize: 13,
  },
  cursor: { animation: "blink 1s step-end infinite", opacity: 0.7 },
  meta: { display: "block", color: "#888", marginTop: 4, fontSize: 12 },
  form: { display: "flex", gap: 8, marginBottom: 16 },
  input: {
    flex: 1,
    padding: "10px 14px",
    fontSize: 15,
    border: "1px solid #ccc",
    borderRadius: 6,
    outline: "none",
  },
  button: {
    padding: "10px 24px",
    fontSize: 15,
    fontWeight: 600,
    background: "#1976d2",
    color: "#fff",
    border: "none",
    borderRadius: 6,
    cursor: "pointer",
  },
  hints: { fontSize: 13, color: "#888" },
};
`)
}

// ── File assertion helpers ───────────────────────────────────────────────────

func assertLLMFilesExist(t *testing.T, projectDir string) {
	t.Helper()

	required := []string{
		"tools/weather/zz_generated_registry.go",
		"tools/calculator/zz_generated_registry.go",
		"shipq/lib/llmpersist/zz_generated_persister.go",
		"querydefs/llm/queries.go",
		"cmd/worker/main.go",
		"cmd/server/main.go",
	}
	for _, rel := range required {
		if _, err := os.Stat(filepath.Join(projectDir, rel)); os.IsNotExist(err) {
			t.Errorf("expected generated file to exist: %s", rel)
		}
	}

	// The LLM migration has a timestamp in the name; check via glob.
	matches, err := filepath.Glob(filepath.Join(projectDir, "migrations", "*_llm_tables.go"))
	if err != nil {
		t.Fatalf("glob for LLM migration: %v", err)
	}
	if len(matches) == 0 {
		t.Error("expected a migration file matching migrations/*_llm_tables.go")
	}
}

// ── Scenario ─────────────────────────────────────────────────────────────────

func scenarioLLMBasic(t *testing.T, shipq string, db dbConfig) {
	t.Helper()

	proj := setupProject(t, shipq, "shipq-e2e-llm", db)
	dbEnv := []string{"DATABASE_URL=" + proj.DatabaseURL}
	tEnv := testEnvForProject(t, proj.CleanDir, db)

	// Step 1: Auth (required before workers)
	t.Log("Generating auth...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "auth")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Step 2: Workers (creates channels scaffold, Redis/Centrifugo config, job_results migration)
	t.Log("Running shipq workers...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "workers")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Step 3: Write tool packages
	t.Log("Writing tool packages...")
	writeWeatherToolPkg(t, proj.CleanDir)
	writeCalculatorToolPkg(t, proj.CleanDir)

	// Step 4: Write the LLM-enabled chatbot channel
	t.Log("Writing chatbot channel...")
	writeChatbotChannel(t, proj.CleanDir)

	// Step 5: Add [llm] section to shipq.ini
	t.Log("Adding [llm] config to shipq.ini...")
	appendLLMConfig(t, proj.CleanDir)

	// Step 6: Run shipq llm compile — generates registry, persister, migration, querydefs
	t.Log("Running shipq llm compile...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "llm", "compile")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Step 7: Apply the generated LLM migration
	t.Log("Running shipq migrate up...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "migrate", "up")

	// Step 7b: Re-run db compile to restore user queries in types.go.
	// migrate up regenerates shipq/queries/types.go with an empty Runner
	// (UserQueries: nil). We need db compile to rediscover all querydefs
	// (auth, llm, CRUD, etc.) and rebuild the full Runner interface.
	t.Log("Running shipq db compile...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "db", "compile")

	// Step 8: Re-run handler compile so the LLM stream types are wired into
	// the typed channel, TypeScript types, and generated integration tests.
	t.Log("Running shipq handler compile...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "handler", "compile")

	// Step 8b: Re-run workers compile to regenerate the TypeScript channel
	// client and React hooks with the chatbot channel + LLM stream types.
	// The initial `shipq workers` ran before the chatbot channel existed,
	// so shipq-channels.ts and react/shipq-channels.ts need regeneration.
	t.Log("Running shipq workers compile...")
	runWithEnv(t, proj.CleanDir, dbEnv, shipq, "workers", "compile")
	run(t, proj.CleanDir, "go", "mod", "tidy")

	// Step 8c: Write the React frontend for interactive LLM testing.
	t.Log("Writing React frontend...")
	writeReactFrontend(t, proj.CleanDir)

	// Step 9: Verify expected generated files are on disk
	t.Log("Verifying generated files...")
	assertLLMFilesExist(t, proj.CleanDir)

	// Step 10: Verify the full project compiles
	t.Log("Compiling cmd/server...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "build", "./cmd/server")
	t.Log("Compiling cmd/worker...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "build", "./cmd/worker")

	// Step 10b: Write a persistence round-trip test into the generated project.
	// This test exercises InsertConversation + InsertMessage against a real SQLite DB,
	// catching bugs like the NOT NULL constraint on sequence that the mock tests miss.
	t.Log("Writing persistence round-trip test...")
	writePersistenceRoundtripTest(t, proj.CleanDir)

	// Step 11: Run generated tests — these use mock infrastructure and a
	// MockProvider; no real API keys or running services required.
	// The persistence round-trip test is also included via go test ./...
	t.Log("Running go test ./... (mock provider, no real API keys)...")
	runWithEnv(t, proj.CleanDir, tEnv, "go", "test", "./...", "-v", "-count=1")

	// Verify the generated TypeScript channel files exist (from workers compile).
	// The default typescript_channel_output is "." (project root), so the files
	// live at the project root unless overridden in shipq.ini.
	tsFiles := []string{
		"shipq-channels.ts",
		"react/shipq-channels.ts",
	}
	for _, rel := range tsFiles {
		if _, err := os.Stat(filepath.Join(proj.CleanDir, rel)); os.IsNotExist(err) {
			t.Errorf("expected generated TypeScript file to exist: %s", rel)
		}
	}

	// Verify the React frontend files exist.
	frontendFiles := []string{
		"frontend/package.json",
		"frontend/vite.config.ts",
		"frontend/index.html",
		"frontend/src/main.tsx",
		"frontend/src/App.tsx",
	}
	for _, rel := range frontendFiles {
		if _, err := os.Stat(filepath.Join(proj.CleanDir, rel)); os.IsNotExist(err) {
			t.Errorf("expected frontend file to exist: %s", rel)
		}
	}

	// Leave the project on disk so the developer can cd in and run manually.
	t.Logf("============================================================")
	t.Logf("LLM E2E scenario passed.")
	t.Logf("Project left at: %s", proj.CleanDir)
	t.Logf("")
	t.Logf("To test manually with real API keys:")
	t.Logf("  cd %s", proj.CleanDir)
	t.Logf("  # Terminal 1:")
	t.Logf("  shipq start redis")
	t.Logf("  # Terminal 2:")
	t.Logf("  shipq start centrifugo")
	t.Logf("  # Terminal 3:")
	t.Logf("  ANTHROPIC_API_KEY=sk-ant-... shipq start worker")
	t.Logf("  # Terminal 4:")
	t.Logf("  shipq start server")
	t.Logf("  # Terminal 5 — React frontend:")
	t.Logf("  cd frontend && npm install && npm run dev")
	t.Logf("  # Then open http://localhost:3000")
	t.Logf("")
	t.Logf("The React frontend streams LLM events live:")
	t.Logf("  - LLMTextDelta: real-time text chunks")
	t.Logf("  - LLMToolCallStart/Result: tool invocations")
	t.Logf("  - LLMDone: token usage summary")
	t.Logf("  - ChatResponse: final response")
	t.Logf("============================================================")
}

// ── Test entry point ─────────────────────────────────────────────────────────

func TestEndToEnd_LLM(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping end-to-end test in short mode")
	}

	repoRoot := shipqRepoRoot(t)
	shipq := buildShipq(t, repoRoot)

	// SQLite requires no external server, so it always runs.
	// Expand to allDBConfigs(t) once the full pipeline is stable.
	t.Run("sqlite", func(t *testing.T) {
		scenarioLLMBasic(t, shipq, dbConfig{Name: "sqlite", BaseURL: ""})
	})
}
