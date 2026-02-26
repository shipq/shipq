// Package llmtest is a standalone diagnostic program for testing LLM provider
// wire protocols directly. It uses net/http with no SDK dependencies so the
// exact request/response JSON is visible and auditable.
//
// Run with:
//
//	go run ./internal/llmtest/
//
// Required environment variables:
//
//	OPENAI_API_KEY
//	ANTHROPIC_API_KEY
//
// Optional:
//
//	OPENAI_MODEL     (default: gpt-4.1)
//	ANTHROPIC_MODEL  (default: claude-sonnet-4-20250514)
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ── Configuration ─────────────────────────────────────────────────────────────

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var (
	openaiKey      = os.Getenv("OPENAI_API_KEY")
	anthropicKey   = os.Getenv("ANTHROPIC_API_KEY")
	openaiModel    = envOr("OPENAI_MODEL", "gpt-4.1")
	anthropicModel = envOr("ANTHROPIC_MODEL", "claude-sonnet-4-20250514")
)

// ── Logging helpers ────────────────────────────────────────────────────────────

func section(title string) {
	fmt.Printf("\n%s\n%s\n", strings.Repeat("=", 70), title)
	fmt.Println(strings.Repeat("=", 70))
}

func logJSON(label string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("\n[%s]\n%s\n", label, string(b))
}

func logRaw(label string, b []byte) {
	// Pretty-print if valid JSON, otherwise raw.
	var tmp any
	if json.Unmarshal(b, &tmp) == nil {
		nice, _ := json.MarshalIndent(tmp, "", "  ")
		fmt.Printf("\n[%s]\n%s\n", label, string(nice))
	} else {
		fmt.Printf("\n[%s]\n%s\n", label, string(b))
	}
}

// ── HTTP helper ────────────────────────────────────────────────────────────────

func doRequest(method, url string, headers map[string]string, body any) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	logRaw("REQUEST BODY", buf.Bytes())

	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	logRaw("RESPONSE BODY", respBytes)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}
	return respBytes, nil
}

// doStreamRequest sends a request and returns the raw SSE body as a ReadCloser.
func doStreamRequest(method, url string, headers map[string]string, body any) (io.ReadCloser, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	logRaw("REQUEST BODY (stream)", buf.Bytes())

	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return resp.Body, nil
}

// ── Weather tool definition (shared literal, no codegen) ──────────────────────

var weatherToolSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"city": map[string]any{
			"type":        "string",
			"description": "The city name to get weather for",
		},
	},
	"required":             []string{"city"},
	"additionalProperties": false,
}

// simulateWeather pretends to look up weather and returns a result string.
func simulateWeather(city string) string {
	return fmt.Sprintf("The weather in %s is sunny and 22°C.", city)
}

// ── Test image for vision tests ───────────────────────────────────────────────

// A 32x32 orange square PNG encoded as base64.
// Generated with Python: make_png(32, 32, [255, 100, 50])
// Used for base64 vision tests with both providers.
const testImageB64 = "iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAIAAAD8GO2jAAAAKklEQVR4nGP4n2JEU8QwasGoBaMWjFowasGoBaMWjFowasGoBaMWDBULAL/eVFvglN15AAAAAElFTkSuQmCC"

// ══════════════════════════════════════════════════════════════════════════════
// OPENAI
// ══════════════════════════════════════════════════════════════════════════════

const openaiBaseURL = "https://api.openai.com/v1"

func openaiHeaders() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + openaiKey,
	}
}

// ── OpenAI: non-streaming tool call ──────────────────────────────────────────

func sendOpenAI() {
	section("OpenAI: Non-Streaming Tool Call (get_weather)")

	// 1. Initial request with tool definition.
	reqBody := map[string]any{
		"model": openaiModel,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo?"},
		},
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get the current weather for a city",
					"parameters":  weatherToolSchema,
					"strict":      true,
				},
			},
		},
		"tool_choice": "auto",
	}

	respBytes, err := doRequest("POST", openaiBaseURL+"/chat/completions", openaiHeaders(), reqBody)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	// 2. Parse tool calls from response.
	var resp1 struct {
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"` // NOTE: JSON string, requires second Unmarshal
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &resp1); err != nil {
		fmt.Printf("ERROR parsing response: %v\n", err)
		return
	}

	if len(resp1.Choices) == 0 || len(resp1.Choices[0].Message.ToolCalls) == 0 {
		fmt.Printf("No tool calls in response. Text: %s\n", resp1.Choices[0].Message.Content)
		return
	}

	tc := resp1.Choices[0].Message.ToolCalls[0]
	fmt.Printf("\n→ Model requested tool: %s\n", tc.Function.Name)
	fmt.Printf("→ Raw arguments string: %s\n", tc.Function.Arguments)

	// Second json.Unmarshal because Arguments is a JSON STRING.
	var args map[string]string
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		fmt.Printf("ERROR parsing arguments: %v\n", err)
		return
	}
	fmt.Printf("→ Parsed city: %s\n", args["city"])

	weatherResult := simulateWeather(args["city"])
	fmt.Printf("→ Tool result: %s\n", weatherResult)

	// 3. Send tool result back.
	// NOTE: OpenAI uses role:"tool" + tool_call_id
	reqBody2 := map[string]any{
		"model": openaiModel,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo?"},
			{
				"role":       resp1.Choices[0].Message.Role,
				"content":    nil,
				"tool_calls": resp1.Choices[0].Message.ToolCalls,
			},
			{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      weatherResult,
			},
		},
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get the current weather for a city",
					"parameters":  weatherToolSchema,
					"strict":      true,
				},
			},
		},
	}

	respBytes2, err := doRequest("POST", openaiBaseURL+"/chat/completions", openaiHeaders(), reqBody2)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	var resp2 struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes2, &resp2); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	fmt.Printf("\n✓ Final response: %s\n", resp2.Choices[0].Message.Content)
	_ = resp1.Usage
}

// ── OpenAI: SSE streaming tool call ──────────────────────────────────────────

func sendOpenAIStream() {
	section("OpenAI: SSE Streaming Tool Call (get_weather)")

	reqBody := map[string]any{
		"model": openaiModel,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo?"},
		},
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "get_weather",
					"description": "Get the current weather for a city",
					"parameters":  weatherToolSchema,
					"strict":      true,
				},
			},
		},
		"tool_choice": "auto",
		"stream":      true,
	}

	body, err := doStreamRequest("POST", openaiBaseURL+"/chat/completions", openaiHeaders(), reqBody)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	defer body.Close()

	// Accumulate streamed tool call data.
	type streamedToolCall struct {
		id        string
		name      string
		arguments strings.Builder
	}
	var toolCalls []streamedToolCall
	var textContent strings.Builder

	fmt.Printf("\n→ Streaming events:\n")
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			fmt.Printf("  [DONE]\n")
			break
		}

		// OpenAI SSE: data: {"choices":[{"delta":{...}}]}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Role      string `json:"role"`
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			fmt.Printf("  WARN: parse chunk: %v\n", err)
			continue
		}

		for _, choice := range chunk.Choices {
			d := choice.Delta
			if d.Content != "" {
				fmt.Printf("  [text_delta] %q\n", d.Content)
				textContent.WriteString(d.Content)
			}
			for _, tc := range d.ToolCalls {
				// Grow toolCalls slice as needed.
				for len(toolCalls) <= tc.Index {
					toolCalls = append(toolCalls, streamedToolCall{})
				}
				if tc.ID != "" {
					toolCalls[tc.Index].id = tc.ID
				}
				if tc.Function.Name != "" {
					toolCalls[tc.Index].name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					toolCalls[tc.Index].arguments.WriteString(tc.Function.Arguments)
				}
				fmt.Printf("  [tool_call_delta] index=%d name=%q args_so_far=%q\n",
					tc.Index, tc.Function.Name, tc.Function.Arguments)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("ERROR scanning: %v\n", err)
		return
	}

	if len(toolCalls) == 0 {
		fmt.Printf("\n✓ Streamed text response: %s\n", textContent.String())
		return
	}

	tc := toolCalls[0]
	fmt.Printf("\n→ Assembled tool call: id=%s name=%s args=%s\n", tc.id, tc.name, tc.arguments.String())

	var args map[string]string
	if err := json.Unmarshal([]byte(tc.arguments.String()), &args); err != nil {
		fmt.Printf("ERROR parsing streamed arguments: %v\n", err)
		return
	}
	weatherResult := simulateWeather(args["city"])
	fmt.Printf("→ Tool result: %s\n", weatherResult)

	// Send tool result back (non-streaming for simplicity).
	reqBody2 := map[string]any{
		"model": openaiModel,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo?"},
			{
				"role":    "assistant",
				"content": nil,
				"tool_calls": []map[string]any{
					{
						"id":   tc.id,
						"type": "function",
						"function": map[string]any{
							"name":      tc.name,
							"arguments": tc.arguments.String(),
						},
					},
				},
			},
			{
				"role":         "tool",
				"tool_call_id": tc.id,
				"content":      weatherResult,
			},
		},
	}

	respBytes2, err := doRequest("POST", openaiBaseURL+"/chat/completions", openaiHeaders(), reqBody2)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	var resp2 struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes2, &resp2); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Printf("\n✓ Final response after streaming tool call: %s\n", resp2.Choices[0].Message.Content)
}

// ── OpenAI: Vision ────────────────────────────────────────────────────────────

func sendOpenAIVision() {
	section("OpenAI: Vision (describe image)")

	reqBody := map[string]any{
		"model": openaiModel,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "Describe this image in one sentence."},
					{
						"type": "image_url",
						"image_url": map[string]any{
							"url":    "data:image/png;base64," + testImageB64,
							"detail": "low",
						},
					},
				},
			},
		},
		"max_tokens": 100,
	}

	respBytes, err := doRequest("POST", openaiBaseURL+"/chat/completions", openaiHeaders(), reqBody)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Printf("\n✓ Vision response: %s\n", resp.Choices[0].Message.Content)
}

// ══════════════════════════════════════════════════════════════════════════════
// ANTHROPIC
// ══════════════════════════════════════════════════════════════════════════════

const anthropicBaseURL = "https://api.anthropic.com/v1"

func anthropicHeaders() map[string]string {
	return map[string]string{
		"x-api-key":         anthropicKey,
		"anthropic-version": "2023-06-01",
	}
}

// ── Anthropic: non-streaming tool call ───────────────────────────────────────

func sendAnthropic() {
	section("Anthropic: Non-Streaming Tool Call (get_weather)")

	// 1. Initial request with tool definition.
	// NOTE: Anthropic uses "input_schema" (not "parameters") and
	// "tools" is a top-level array (same as OpenAI structurally but different field names).
	reqBody := map[string]any{
		"model":      anthropicModel,
		"max_tokens": 1024,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo?"},
		},
		"tools": []map[string]any{
			{
				"name":         "get_weather",
				"description":  "Get the current weather for a city",
				"input_schema": weatherToolSchema,
			},
		},
	}

	respBytes, err := doRequest("POST", anthropicBaseURL+"/messages", anthropicHeaders(), reqBody)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	// 2. Parse response.
	// NOTE: Anthropic returns content as an array of blocks.
	// tool_use blocks have "input" as a PARSED JSON OBJECT (not a string!).
	var resp1 struct {
		ID      string `json:"id"`
		Role    string `json:"role"`
		Content []struct {
			Type  string `json:"type"`
			Text  string `json:"text"`  // for type="text"
			ID    string `json:"id"`    // for type="tool_use"
			Name  string `json:"name"`  // for type="tool_use"
			Input any    `json:"input"` // for type="tool_use" — parsed JSON object, NOT a string
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &resp1); err != nil {
		fmt.Printf("ERROR parsing response: %v\n", err)
		return
	}

	// Find tool_use block.
	var toolUseID, toolUseName string
	var toolInput map[string]any
	var textContent string
	for _, block := range resp1.Content {
		switch block.Type {
		case "text":
			textContent = block.Text
		case "tool_use":
			toolUseID = block.ID
			toolUseName = block.Name
			// Input is already a parsed map — no second Unmarshal needed.
			if m, ok := block.Input.(map[string]any); ok {
				toolInput = m
			}
		}
	}

	if toolUseName == "" {
		fmt.Printf("No tool calls. Text: %s\n", textContent)
		return
	}

	fmt.Printf("\n→ Model requested tool: %s (id=%s)\n", toolUseName, toolUseID)
	logJSON("Tool input (already parsed, no double-unmarshal needed)", toolInput)

	city, _ := toolInput["city"].(string)
	weatherResult := simulateWeather(city)
	fmt.Printf("→ Tool result: %s\n", weatherResult)

	// 3. Send tool result back.
	// NOTE: Anthropic uses role:"user" + content block of type "tool_result" referencing tool_use_id.
	// This is the critical difference from OpenAI's role:"tool" + tool_call_id approach.
	//
	// IMPORTANT: We must reconstruct clean per-type content blocks — we cannot pass
	// the parsed struct back directly because unmarshalling populates all fields
	// on every block, causing Anthropic to reject extra fields (e.g. "id" on text blocks).
	assistantContent := make([]map[string]any, 0, len(resp1.Content))
	for _, b := range resp1.Content {
		switch b.Type {
		case "text":
			assistantContent = append(assistantContent, map[string]any{
				"type": "text",
				"text": b.Text,
			})
		case "tool_use":
			assistantContent = append(assistantContent, map[string]any{
				"type":  "tool_use",
				"id":    b.ID,
				"name":  b.Name,
				"input": b.Input,
			})
		}
	}

	reqBody2 := map[string]any{
		"model":      anthropicModel,
		"max_tokens": 1024,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo?"},
			// Include the assistant's message with clean per-type content blocks.
			{
				"role":    "assistant",
				"content": assistantContent,
			},
			// Tool result goes as a user message with tool_result content block.
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": toolUseID, // matches the tool_use block above
						"content":     weatherResult,
					},
				},
			},
		},
		"tools": []map[string]any{
			{
				"name":         "get_weather",
				"description":  "Get the current weather for a city",
				"input_schema": weatherToolSchema,
			},
		},
	}

	respBytes2, err := doRequest("POST", anthropicBaseURL+"/messages", anthropicHeaders(), reqBody2)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	var resp2 struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes2, &resp2); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	for _, b := range resp2.Content {
		if b.Type == "text" {
			fmt.Printf("\n✓ Final response: %s\n", b.Text)
		}
	}
}

// ── Anthropic: SSE streaming tool call ───────────────────────────────────────

func sendAnthropicStream() {
	section("Anthropic: SSE Streaming Tool Call (get_weather)")

	// NOTE: Anthropic SSE uses event: lines to distinguish event types.
	// Each SSE event has an "event:" line followed by a "data:" line.
	// Event types: message_start, content_block_start, ping,
	//              content_block_delta, content_block_stop, message_delta, message_stop

	reqBody := map[string]any{
		"model":      anthropicModel,
		"max_tokens": 1024,
		"stream":     true,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo?"},
		},
		"tools": []map[string]any{
			{
				"name":         "get_weather",
				"description":  "Get the current weather for a city",
				"input_schema": weatherToolSchema,
			},
		},
	}

	body, err := doStreamRequest("POST", anthropicBaseURL+"/messages", anthropicHeaders(), reqBody)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	defer body.Close()

	// Accumulate streamed content blocks.
	type contentBlock struct {
		blockType string // "text" or "tool_use"
		id        string
		name      string
		textBuf   strings.Builder
		inputBuf  strings.Builder
	}
	var blocks []contentBlock
	var currentBlockIdx int = -1
	var textContent strings.Builder

	fmt.Printf("\n→ Streaming events:\n")
	scanner := bufio.NewScanner(body)

	var currentEvent string
	for scanner.Scan() {
		line := scanner.Text()

		// Anthropic SSE: event: <type> followed by data: <json>
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			fmt.Printf("  [event: %s]\n", currentEvent)
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		switch currentEvent {
		case "content_block_start":
			var e struct {
				Index        int `json:"index"`
				ContentBlock struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
					Text string `json:"text"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal([]byte(data), &e); err != nil {
				fmt.Printf("  WARN: %v\n", err)
				continue
			}
			// Grow blocks slice.
			for len(blocks) <= e.Index {
				blocks = append(blocks, contentBlock{})
			}
			blocks[e.Index].blockType = e.ContentBlock.Type
			blocks[e.Index].id = e.ContentBlock.ID
			blocks[e.Index].name = e.ContentBlock.Name
			if e.ContentBlock.Text != "" {
				blocks[e.Index].textBuf.WriteString(e.ContentBlock.Text)
			}
			currentBlockIdx = e.Index
			fmt.Printf("  [content_block_start] index=%d type=%s\n", e.Index, e.ContentBlock.Type)

		case "content_block_delta":
			var e struct {
				Index int `json:"index"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`         // text_delta
					PartialJSON string `json:"partial_json"` // input_json_delta
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &e); err != nil {
				fmt.Printf("  WARN: %v\n", err)
				continue
			}
			if e.Index >= 0 && e.Index < len(blocks) {
				switch e.Delta.Type {
				case "text_delta":
					blocks[e.Index].textBuf.WriteString(e.Delta.Text)
					textContent.WriteString(e.Delta.Text)
					fmt.Printf("  [text_delta] %q\n", e.Delta.Text)
				case "input_json_delta":
					blocks[e.Index].inputBuf.WriteString(e.Delta.PartialJSON)
					fmt.Printf("  [input_json_delta] %q\n", e.Delta.PartialJSON)
				}
			}

		case "message_start":
			fmt.Printf("  [message_start] data: %s\n", data)

		case "message_stop":
			fmt.Printf("  [message_stop]\n")
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("ERROR scanning: %v\n", err)
		return
	}
	_ = currentBlockIdx

	// Find tool_use block.
	var toolBlock *contentBlock
	for i := range blocks {
		if blocks[i].blockType == "tool_use" {
			toolBlock = &blocks[i]
			break
		}
	}

	if toolBlock == nil {
		fmt.Printf("\n✓ Streamed text: %s\n", textContent.String())
		return
	}

	fmt.Printf("\n→ Assembled tool_use: id=%s name=%s input=%s\n",
		toolBlock.id, toolBlock.name, toolBlock.inputBuf.String())

	var args map[string]string
	if err := json.Unmarshal([]byte(toolBlock.inputBuf.String()), &args); err != nil {
		fmt.Printf("ERROR parsing streamed input: %v\n", err)
		return
	}
	weatherResult := simulateWeather(args["city"])
	fmt.Printf("→ Tool result: %s\n", weatherResult)

	// Send tool result back.
	assistantContent := make([]map[string]any, 0, len(blocks))
	for _, b := range blocks {
		switch b.blockType {
		case "text":
			if b.textBuf.Len() > 0 {
				assistantContent = append(assistantContent, map[string]any{
					"type": "text",
					"text": b.textBuf.String(),
				})
			}
		case "tool_use":
			var inputObj any
			_ = json.Unmarshal([]byte(b.inputBuf.String()), &inputObj)
			assistantContent = append(assistantContent, map[string]any{
				"type":  "tool_use",
				"id":    b.id,
				"name":  b.name,
				"input": inputObj,
			})
		}
	}

	reqBody2 := map[string]any{
		"model":      anthropicModel,
		"max_tokens": 1024,
		"messages": []map[string]any{
			{"role": "user", "content": "What's the weather in Tokyo?"},
			{"role": "assistant", "content": assistantContent},
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": toolBlock.id,
						"content":     weatherResult,
					},
				},
			},
		},
		"tools": []map[string]any{
			{
				"name":         "get_weather",
				"description":  "Get the current weather for a city",
				"input_schema": weatherToolSchema,
			},
		},
	}

	respBytes2, err := doRequest("POST", anthropicBaseURL+"/messages", anthropicHeaders(), reqBody2)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	var resp2 struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes2, &resp2); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	for _, b := range resp2.Content {
		if b.Type == "text" {
			fmt.Printf("\n✓ Final response after streaming tool call: %s\n", b.Text)
		}
	}
}

// ── Anthropic: Vision ─────────────────────────────────────────────────────────

func sendAnthropicVision() {
	section("Anthropic: Vision (describe image)")

	// Anthropic vision: image content block with URL source.
	reqBody := map[string]any{
		"model":      anthropicModel,
		"max_tokens": 100,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": "image/png",
							"data":       testImageB64,
						},
					},
					{"type": "text", "text": "Describe this image in one sentence."},
				},
			},
		},
	}

	respBytes, err := doRequest("POST", anthropicBaseURL+"/messages", anthropicHeaders(), reqBody)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	for _, b := range resp.Content {
		if b.Type == "text" {
			fmt.Printf("\n✓ Vision response: %s\n", b.Text)
		}
	}
}

// ── Anthropic: Web Search ─────────────────────────────────────────────────────

func sendAnthropicWebSearch() {
	section("Anthropic: Web Search (web_search_20250305)")

	reqBody := map[string]any{
		"model":      anthropicModel,
		"max_tokens": 1024,
		"messages": []map[string]any{
			{"role": "user", "content": "What is the latest version of Go as of today?"},
		},
		"tools": []map[string]any{
			{
				"type": "web_search_20250305",
				"name": "web_search",
			},
		},
	}

	respBytes, err := doRequest("POST", anthropicBaseURL+"/messages", anthropicHeaders(), reqBody)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	var resp struct {
		Content []struct {
			Type   string `json:"type"`
			Text   string `json:"text"`
			Source *struct {
				Type  string `json:"type"`
				URL   string `json:"url"`
				Title string `json:"title"`
			} `json:"source"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	for _, b := range resp.Content {
		if b.Type == "text" {
			fmt.Printf("\n✓ Web search response: %s\n", b.Text)
		}
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// Main
// ══════════════════════════════════════════════════════════════════════════════

func main() {
	start := time.Now()

	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              LLM Provider E2E Plumbing Test                         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Printf("OpenAI model:    %s\n", openaiModel)
	fmt.Printf("Anthropic model: %s\n", anthropicModel)

	if openaiKey == "" {
		fmt.Println("\nWARN: OPENAI_API_KEY not set — skipping OpenAI tests")
	} else {
		sendOpenAI()
		sendOpenAIStream()
		sendOpenAIVision()
	}

	if anthropicKey == "" {
		fmt.Println("\nWARN: ANTHROPIC_API_KEY not set — skipping Anthropic tests")
	} else {
		sendAnthropic()
		sendAnthropicStream()
		sendAnthropicVision()
		sendAnthropicWebSearch()
	}

	fmt.Printf("\n\nTotal elapsed: %s\n", time.Since(start).Round(time.Millisecond))
}
