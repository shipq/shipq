package channelgen

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// ── Test helpers ─────────────────────────────────────────────────────────────

func makeUnidirectionalEmailChannel() codegen.SerializedChannelInfo {
	return codegen.SerializedChannelInfo{
		Name:        "email_notification",
		Visibility:  "frontend",
		IsPublic:    false,
		PackagePath: "myapp/channels/email",
		PackageName: "email",
		Messages: []codegen.SerializedMessageInfo{
			{
				Direction:   "client_to_server",
				TypeName:    "SendEmailRequest",
				IsDispatch:  true,
				HandlerName: "HandleSendEmailRequest",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "To", Type: "string", JSONName: "to", Required: true},
					{Name: "Subject", Type: "string", JSONName: "subject", Required: true},
					{Name: "Body", Type: "string", JSONName: "body", Required: true},
				},
			},
			{
				Direction:  "server_to_client",
				TypeName:   "SendEmailResult",
				IsDispatch: false,
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Success", Type: "bool", JSONName: "success", Required: true},
					{Name: "ErrorMessage", Type: "string", JSONName: "error_message", Required: false},
				},
			},
		},
	}
}

func makeBidirectionalChatbotChannel() codegen.SerializedChannelInfo {
	return codegen.SerializedChannelInfo{
		Name:        "chatbot",
		Visibility:  "frontend",
		IsPublic:    false,
		PackagePath: "myapp/channels/chatbot",
		PackageName: "chatbot",
		Messages: []codegen.SerializedMessageInfo{
			{
				Direction:   "client_to_server",
				TypeName:    "StartChat",
				IsDispatch:  true,
				HandlerName: "HandleStartChat",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Prompt", Type: "string", JSONName: "prompt", Required: true},
				},
			},
			{
				Direction:   "client_to_server",
				TypeName:    "ToolCallApproval",
				IsDispatch:  false,
				HandlerName: "HandleToolCallApproval",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "CallID", Type: "string", JSONName: "call_id", Required: true},
					{Name: "Approved", Type: "bool", JSONName: "approved", Required: true},
				},
			},
			{
				Direction:  "server_to_client",
				TypeName:   "BotMessage",
				IsDispatch: false,
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Text", Type: "string", JSONName: "text", Required: true},
				},
			},
			{
				Direction:  "server_to_client",
				TypeName:   "ToolCallRequest",
				IsDispatch: false,
				Fields: []codegen.SerializedFieldInfo{
					{Name: "CallID", Type: "string", JSONName: "call_id", Required: true},
					{Name: "ToolName", Type: "string", JSONName: "tool_name", Required: true},
					{Name: "Args", Type: "map[string]any", JSONName: "args", Required: true},
				},
			},
			{
				Direction:  "server_to_client",
				TypeName:   "StreamingToken",
				IsDispatch: false,
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Token", Type: "string", JSONName: "token", Required: true},
				},
			},
			{
				Direction:  "server_to_client",
				TypeName:   "ChatFinished",
				IsDispatch: false,
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Summary", Type: "string", JSONName: "summary", Required: true},
				},
			},
		},
	}
}

func makePublicAssistantChannel() codegen.SerializedChannelInfo {
	return codegen.SerializedChannelInfo{
		Name:        "assistant",
		Visibility:  "frontend",
		IsPublic:    true,
		PackagePath: "myapp/channels/assistant",
		PackageName: "assistant",
		Messages: []codegen.SerializedMessageInfo{
			{
				Direction:   "client_to_server",
				TypeName:    "AssistantQuery",
				IsDispatch:  true,
				HandlerName: "HandleAssistantQuery",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Question", Type: "string", JSONName: "question", Required: true},
				},
			},
			{
				Direction:  "server_to_client",
				TypeName:   "AssistantAnswer",
				IsDispatch: false,
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Answer", Type: "string", JSONName: "answer", Required: true},
					{Name: "Sources", Type: "[]string", JSONName: "sources", Required: false},
				},
			},
		},
	}
}

func makeBackendBillingChannel() codegen.SerializedChannelInfo {
	return codegen.SerializedChannelInfo{
		Name:        "billing_sync",
		Visibility:  "backend",
		IsPublic:    false,
		PackagePath: "myapp/channels/billing",
		PackageName: "billing",
		Messages: []codegen.SerializedMessageInfo{
			{
				Direction:   "client_to_server",
				TypeName:    "SyncBilling",
				IsDispatch:  true,
				HandlerName: "HandleSyncBilling",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "AccountID", Type: "int64", JSONName: "account_id", Required: true},
				},
			},
			{
				Direction:  "server_to_client",
				TypeName:   "BillingResult",
				IsDispatch: false,
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Status", Type: "string", JSONName: "status", Required: true},
				},
			},
		},
	}
}

// ── Unit tests ───────────────────────────────────────────────────────────────

func TestGenerateTS_SkipsBackendChannels(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeBackendBillingChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Should only contain the "no frontend channels" comment
	if !strings.Contains(code, "No frontend channels") {
		t.Errorf("expected 'No frontend channels' message for backend-only channels, got:\n%s", code)
	}

	// Should NOT contain any billing types
	if strings.Contains(code, "SyncBilling") {
		t.Error("backend channel type SyncBilling should not appear in output")
	}
	if strings.Contains(code, "BillingResult") {
		t.Error("backend channel type BillingResult should not appear in output")
	}
	if strings.Contains(code, "billing_sync") {
		t.Error("backend channel name billing_sync should not appear in output")
	}
}

func TestGenerateTS_UnidirectionalChannel(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Verify SendEmailRequest interface generated
	if !strings.Contains(code, "export interface SendEmailRequest") {
		t.Error("expected SendEmailRequest interface")
	}

	// Verify SendEmailResult interface generated
	if !strings.Contains(code, "export interface SendEmailResult") {
		t.Error("expected SendEmailResult interface")
	}

	// Verify EmailNotificationChannel interface
	if !strings.Contains(code, "export interface EmailNotificationChannel") {
		t.Error("expected EmailNotificationChannel interface")
	}

	// Verify onSendEmailResult handler
	if !strings.Contains(code, "onSendEmailResult(handler: (msg: SendEmailResult) => void): void") {
		t.Error("expected onSendEmailResult handler in channel interface")
	}

	// Verify unsubscribe
	if !strings.Contains(code, "unsubscribe(): void") {
		t.Error("expected unsubscribe in channel interface")
	}

	// Verify dispatch function
	if !strings.Contains(code, "export async function dispatchEmailNotification(request: SendEmailRequest): Promise<EmailNotificationChannel>") {
		t.Error("expected dispatchEmailNotification function")
	}

	// Verify NO send methods (unidirectional — no mid-stream client messages)
	if strings.Contains(code, "send") && strings.Contains(code, "sub.publish") {
		// More targeted check: there should be no sendXxx methods on the channel interface
		lines := strings.Split(code, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "send") && strings.Contains(trimmed, "(msg:") && !strings.Contains(trimmed, "function") {
				t.Errorf("unidirectional channel should not have send methods, found: %s", trimmed)
			}
		}
	}
}

func TestGenerateTS_BidirectionalChannel(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeBidirectionalChatbotChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Verify all message type interfaces generated
	for _, typeName := range []string{"StartChat", "ToolCallApproval", "BotMessage", "ToolCallRequest", "StreamingToken", "ChatFinished"} {
		if !strings.Contains(code, "export interface "+typeName) {
			t.Errorf("expected interface for %s", typeName)
		}
	}

	// Verify ChatbotChannel has on* handlers for each FromServer type
	for _, typeName := range []string{"BotMessage", "ToolCallRequest", "StreamingToken", "ChatFinished"} {
		handler := "on" + typeName + "(handler: (msg: " + typeName + ") => void): void"
		if !strings.Contains(code, handler) {
			t.Errorf("expected handler %s in ChatbotChannel interface", handler)
		}
	}

	// Verify sendToolCallApproval method for mid-stream FromClient type
	if !strings.Contains(code, "sendToolCallApproval(msg: ToolCallApproval): void") {
		t.Error("expected sendToolCallApproval method in ChatbotChannel interface")
	}

	// [L1] Verify echo filtering: FromClient type names are in the ignore set
	if !strings.Contains(code, "fromClientTypes") {
		t.Error("expected fromClientTypes set for echo filtering [L1]")
	}
	if !strings.Contains(code, `"ToolCallApproval"`) {
		t.Error("expected ToolCallApproval in fromClientTypes set [L1]")
	}

	// [L1] Verify the demultiplexer skips echoed FromClient messages
	if !strings.Contains(code, "fromClientTypes.has(msgType)") {
		t.Error("expected echo filter check: fromClientTypes.has(msgType) [L1]")
	}

	// Verify dispatch function
	if !strings.Contains(code, "export async function dispatchChatbot(request: StartChat): Promise<ChatbotChannel>") {
		t.Error("expected dispatchChatbot function")
	}

	// Verify sub.publish for send methods
	if !strings.Contains(code, `sub.publish({ type: "ToolCallApproval", data: msg })`) {
		t.Error("expected sub.publish call in sendToolCallApproval")
	}
}

func TestGenerateTS_PublicChannel_NoCredentials(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makePublicAssistantChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Public channel should NOT include credentials: "include"
	if strings.Contains(code, `credentials: "include"`) {
		t.Error("public channel should not include credentials: \"include\"")
	}

	// Verify dispatch function exists
	if !strings.Contains(code, "export async function dispatchAssistant(") {
		t.Error("expected dispatchAssistant function")
	}
}

func TestGenerateTS_AuthChannel_HasCredentials(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Authenticated channel should include credentials: "include"
	if !strings.Contains(code, `credentials: "include"`) {
		t.Error("authenticated channel should include credentials: \"include\"")
	}
}

func TestGoToTypeScriptConversion(t *testing.T) {
	tests := []struct {
		goType   string
		expected string
	}{
		{"string", "string"},
		{"int", "number"},
		{"int8", "number"},
		{"int16", "number"},
		{"int32", "number"},
		{"int64", "number"},
		{"uint", "number"},
		{"uint8", "number"},
		{"uint16", "number"},
		{"uint32", "number"},
		{"uint64", "number"},
		{"float32", "number"},
		{"float64", "number"},
		{"bool", "boolean"},
		{"[]string", "string[]"},
		{"[]int", "number[]"},
		{"[]bool", "boolean[]"},
		{"map[string]any", "Record<string, any>"},
		{"map[string]string", "Record<string, string>"},
		{"map[string]int", "Record<string, number>"},
		{"any", "any"},
		{"interface{}", "any"},
		{"*string", "string"},
		{"*int", "number"},
		{"[][]string", "string[][]"},
	}

	for _, tt := range tests {
		t.Run(tt.goType+"->"+tt.expected, func(t *testing.T) {
			result := goTypeStringToTS(tt.goType)
			if result != tt.expected {
				t.Errorf("goTypeStringToTS(%q) = %q, want %q", tt.goType, result, tt.expected)
			}
		})
	}
}

func TestGoToTypeScript_NestedStruct(t *testing.T) {
	field := codegen.SerializedFieldInfo{
		Name:     "Metadata",
		Type:     "Metadata",
		JSONName: "metadata",
		Required: true,
		StructFields: &codegen.SerializedStructInfo{
			Name: "Metadata",
			Fields: []codegen.SerializedFieldInfo{
				{Name: "Key", Type: "string", JSONName: "key", Required: true},
				{Name: "Value", Type: "string", JSONName: "value", Required: true},
			},
		},
	}

	result := goTypeToTS(field)
	if !strings.Contains(result, "key: string;") {
		t.Errorf("expected inline struct to contain 'key: string;', got: %s", result)
	}
	if !strings.Contains(result, "value: string;") {
		t.Errorf("expected inline struct to contain 'value: string;', got: %s", result)
	}
}

func TestGenerateTS_OptionalFields(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name:        "test",
			Visibility:  "frontend",
			IsPublic:    false,
			PackagePath: "myapp/channels/test",
			PackageName: "test",
			Messages: []codegen.SerializedMessageInfo{
				{
					Direction:   "client_to_server",
					TypeName:    "TestRequest",
					IsDispatch:  true,
					HandlerName: "HandleTestRequest",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Required", Type: "string", JSONName: "required", Required: true},
						{Name: "Optional", Type: "string", JSONName: "optional", Required: false},
					},
				},
				{
					Direction:  "server_to_client",
					TypeName:   "TestResult",
					IsDispatch: false,
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Data", Type: "string", JSONName: "data", Required: true},
					},
				},
			},
		},
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Required field should NOT have ?
	if !strings.Contains(code, "required: string;") {
		t.Error("expected 'required: string;' for required field")
	}

	// Optional field should have ?
	if !strings.Contains(code, "optional?: string;") {
		t.Error("expected 'optional?: string;' for optional field")
	}
}

func TestGenerateTS_OmittedFields(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name:        "test",
			Visibility:  "frontend",
			IsPublic:    false,
			PackagePath: "myapp/channels/test",
			PackageName: "test",
			Messages: []codegen.SerializedMessageInfo{
				{
					Direction:   "client_to_server",
					TypeName:    "TestRequest",
					IsDispatch:  true,
					HandlerName: "HandleTestRequest",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Visible", Type: "string", JSONName: "visible", Required: true},
						{Name: "Hidden", Type: "string", JSONName: "-", JSONOmit: true, Required: true},
					},
				},
				{
					Direction:  "server_to_client",
					TypeName:   "TestResult",
					IsDispatch: false,
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Data", Type: "string", JSONName: "data", Required: true},
					},
				},
			},
		},
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	if !strings.Contains(code, "visible: string;") {
		t.Error("expected visible field in output")
	}

	// Hidden field (json:"-") should not appear
	if strings.Contains(code, "Hidden") {
		t.Error("json-omitted field 'Hidden' should not appear in output")
	}
}

func TestGenerateTS_PerChannelCentrifugeClient(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// [L3] Verify each dispatch creates its own Centrifuge client with getToken for refresh
	if !strings.Contains(code, "new Centrifuge(ws_url, {") {
		t.Error("expected per-channel Centrifuge client creation [L3]")
	}
	if !strings.Contains(code, "token: connection_token,") {
		t.Error("expected initial connection token in Centrifuge constructor [L3]")
	}
	if !strings.Contains(code, "getToken: async () => {") {
		t.Error("expected getToken callback for token refresh [Bug 11]")
	}
	if !strings.Contains(code, "refreshTokens()") {
		t.Error("expected refreshTokens() call in getToken callback [Bug 11]")
	}

	// Verify unsubscribe disconnects the client
	if !strings.Contains(code, "client.disconnect()") {
		t.Error("expected client.disconnect() in unsubscribe [L3]")
	}
}

func TestGenerateTS_ConfigureFunction(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	if !strings.Contains(code, "export interface ChannelConfig") {
		t.Error("expected ChannelConfig interface")
	}
	if !strings.Contains(code, "baseURL: string;") {
		t.Error("expected baseURL in ChannelConfig")
	}
	if !strings.Contains(code, "centrifugoURL: string;") {
		t.Error("expected centrifugoURL in ChannelConfig")
	}
	if !strings.Contains(code, "export function configure(cfg: ChannelConfig): void") {
		t.Error("expected configure function")
	}
}

func TestGenerateTS_ImportsAndHeader(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	if !strings.Contains(code, "// Code generated by shipq. DO NOT EDIT.") {
		t.Error("expected codegen header comment")
	}
	if !strings.Contains(code, `import { Centrifuge } from "centrifuge";`) {
		t.Error("expected centrifuge import")
	}
}

func TestGenerateTS_MapField(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeBidirectionalChatbotChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// ToolCallRequest has Args field of type map[string]any
	if !strings.Contains(code, "args: Record<string, any>;") {
		t.Error("expected 'args: Record<string, any>;' for map[string]any field")
	}
}

func TestGenerateTS_SliceField(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makePublicAssistantChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// AssistantAnswer has Sources field of type []string (optional)
	if !strings.Contains(code, "sources?: string[];") {
		t.Error("expected 'sources?: string[];' for optional []string field")
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"email", "Email"},
		{"chatbot", "Chatbot"},
		{"email_notification", "EmailNotification"},
		{"my-channel", "MyChannel"},
		{"a_b_c", "ABC"},
		{"already_pascal", "AlreadyPascal"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateTS_UnidirectionalHasNoEchoFilter(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Unidirectional channels have no mid-stream client messages,
	// so there should be no fromClientTypes set
	if strings.Contains(code, "fromClientTypes") {
		t.Error("unidirectional channel should not have fromClientTypes echo filter set")
	}
}

func TestGenerateTS_MultipleChannels(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),
		makeBidirectionalChatbotChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Both channels should appear
	if !strings.Contains(code, "EmailNotificationChannel") {
		t.Error("expected EmailNotificationChannel")
	}
	if !strings.Contains(code, "ChatbotChannel") {
		t.Error("expected ChatbotChannel")
	}

	// Both dispatch functions
	if !strings.Contains(code, "dispatchEmailNotification") {
		t.Error("expected dispatchEmailNotification")
	}
	if !strings.Contains(code, "dispatchChatbot") {
		t.Error("expected dispatchChatbot")
	}
}

// ── Golden file test ─────────────────────────────────────────────────────────

func TestGenerateTS_Golden_MixedChannels(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),  // unidirectional, authenticated
		makeBidirectionalChatbotChannel(), // bidirectional, authenticated
		makePublicAssistantChannel(),      // unidirectional, public
		makeBackendBillingChannel(),       // backend-only, should be excluded
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	goldenPath := filepath.Join("testdata", "golden", "shipq-channels.ts")

	if *updateGolden {
		// Update the golden file
		dir := filepath.Dir(goldenPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, output, 0644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Log("updated golden file")
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file %s (run with -update to create): %v", goldenPath, err)
	}

	if string(output) != string(golden) {
		t.Errorf("output does not match golden file %s\n\nGot:\n%s\n\nWant:\n%s", goldenPath, string(output), string(golden))
	}
}

// ── LLM type injection tests ────────────────────────────────────────────────

func makeLLMConfig(channelPkgPath string) *LLMConfig {
	return &LLMConfig{
		LLMChannelPkgs: map[string]bool{
			channelPkgPath: true,
		},
	}
}

func TestGenerateTS_LLMChannel_HasLLMStreamTypes(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makePublicAssistantChannel(),
	}
	llmCfg := makeLLMConfig("myapp/channels/assistant")

	output, err := GenerateTypeScriptChannelClient(channels, llmCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// LLM stream type definitions should be prepended
	mustContain := []string{
		"export interface LLMTextDelta",
		"export interface LLMToolCallStart",
		"export interface LLMToolCallResult",
		"export interface LLMDone",
	}
	for _, s := range mustContain {
		if !strings.Contains(code, s) {
			t.Errorf("LLM channel output should contain %q", s)
		}
	}
}

func TestGenerateTS_LLMChannel_HasOnLLMHandlers(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makePublicAssistantChannel(),
	}
	llmCfg := makeLLMConfig("myapp/channels/assistant")

	output, err := GenerateTypeScriptChannelClient(channels, llmCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Channel interface should include LLM handlers
	mustContain := []string{
		"onLLMTextDelta(handler: (msg: LLMTextDelta) => void): void;",
		"onLLMToolCallStart(handler: (msg: LLMToolCallStart) => void): void;",
		"onLLMToolCallResult(handler: (msg: LLMToolCallResult) => void): void;",
		"onLLMDone(handler: (msg: LLMDone) => void): void;",
	}
	for _, s := range mustContain {
		if !strings.Contains(code, s) {
			t.Errorf("LLM channel interface should contain %q", s)
		}
	}
}

func TestGenerateTS_LLMChannel_HasDemultiplexerCases(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makePublicAssistantChannel(),
	}
	llmCfg := makeLLMConfig("myapp/channels/assistant")

	output, err := GenerateTypeScriptChannelClient(channels, llmCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Demultiplexer should include LLM type cases
	mustContain := []string{
		`case "LLMTextDelta":`,
		`case "LLMToolCallStart":`,
		`case "LLMToolCallResult":`,
		`case "LLMDone":`,
	}
	for _, s := range mustContain {
		if !strings.Contains(code, s) {
			t.Errorf("LLM channel demultiplexer should contain %q", s)
		}
	}
}

func TestGenerateTS_NonLLMChannel_NoLLMTypes(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),
	}
	// LLM config exists but does NOT include the email channel
	llmCfg := makeLLMConfig("myapp/channels/some_other_channel")

	output, err := GenerateTypeScriptChannelClient(channels, llmCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// Should NOT have LLM type definitions or handlers
	if strings.Contains(code, "LLMTextDelta") {
		t.Error("non-LLM channel should not contain LLMTextDelta")
	}
	if strings.Contains(code, "onLLMDone") {
		t.Error("non-LLM channel should not contain onLLMDone handler")
	}
}

func TestGenerateTS_NilLLMConfig_NoLLMTypes(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makePublicAssistantChannel(),
	}

	output, err := GenerateTypeScriptChannelClient(channels, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	if strings.Contains(code, "LLMTextDelta") {
		t.Error("nil LLM config should not produce LLM types")
	}
}

func TestGenerateTS_MixedChannels_OnlyLLMChannelGetsHandlers(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalEmailChannel(),
		makePublicAssistantChannel(),
	}
	llmCfg := makeLLMConfig("myapp/channels/assistant")

	output, err := GenerateTypeScriptChannelClient(channels, llmCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code := string(output)

	// LLM types should be prepended (because at least one channel is LLM-enabled)
	if !strings.Contains(code, "export interface LLMTextDelta") {
		t.Error("expected LLM types to be prepended")
	}

	// Find the assistant channel interface — it should have LLM handlers
	assistantIdx := strings.Index(code, "export interface AssistantChannel")
	if assistantIdx < 0 {
		t.Fatal("AssistantChannel interface not found")
	}
	assistantSection := code[assistantIdx:]
	if !strings.Contains(assistantSection, "onLLMTextDelta") {
		t.Error("AssistantChannel should have onLLMTextDelta handler")
	}

	// Find the email channel interface — it should NOT have LLM handlers
	emailIdx := strings.Index(code, "export interface EmailNotificationChannel")
	if emailIdx < 0 {
		t.Fatal("EmailNotificationChannel interface not found")
	}
	// Get the section between EmailNotificationChannel and AssistantChannel
	emailEnd := strings.Index(code[emailIdx:], "export interface AssistantChannel")
	if emailEnd < 0 {
		emailEnd = len(code) - emailIdx
	}
	emailSection := code[emailIdx : emailIdx+emailEnd]
	if strings.Contains(emailSection, "onLLMTextDelta") {
		t.Error("EmailNotificationChannel should NOT have onLLMTextDelta handler")
	}
}
