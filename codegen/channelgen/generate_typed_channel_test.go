package channelgen

import (
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

// makeUnidirectionalChannel creates an email-like channel with 1 FromClient (dispatch) and 1 FromServer.
func makeUnidirectionalChannel() codegen.SerializedChannelInfo {
	return codegen.SerializedChannelInfo{
		Name:        "email",
		Visibility:  "frontend",
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
				TypeName:   "EmailProgress",
				IsDispatch: false,
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Percent", Type: "int", JSONName: "percent", Required: true},
					{Name: "Status", Type: "string", JSONName: "status", Required: true},
				},
			},
		},
	}
}

// makeBidirectionalChannel creates a chatbot-like channel with 2 FromClient and 4 FromServer types.
func makeBidirectionalChannel() codegen.SerializedChannelInfo {
	return codegen.SerializedChannelInfo{
		Name:        "chatbot",
		Visibility:  "frontend",
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

// makeMultiMidStreamChannel creates a channel with 3 mid-stream FromClient types
// (plus 1 dispatch, so 4 total FromClient) and 1 FromServer.
func makeMultiMidStreamChannel() codegen.SerializedChannelInfo {
	return codegen.SerializedChannelInfo{
		Name:        "workflow",
		Visibility:  "frontend",
		PackagePath: "myapp/channels/workflow",
		PackageName: "workflow",
		Messages: []codegen.SerializedMessageInfo{
			{
				Direction:   "client_to_server",
				TypeName:    "StartWorkflow",
				IsDispatch:  true,
				HandlerName: "HandleStartWorkflow",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "WorkflowID", Type: "string", JSONName: "workflow_id", Required: true},
				},
			},
			{
				Direction:   "client_to_server",
				TypeName:    "ApproveStep",
				IsDispatch:  false,
				HandlerName: "HandleApproveStep",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "StepID", Type: "string", JSONName: "step_id", Required: true},
				},
			},
			{
				Direction:   "client_to_server",
				TypeName:    "ProvideInput",
				IsDispatch:  false,
				HandlerName: "HandleProvideInput",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "InputKey", Type: "string", JSONName: "input_key", Required: true},
					{Name: "InputValue", Type: "string", JSONName: "input_value", Required: true},
				},
			},
			{
				Direction:   "client_to_server",
				TypeName:    "CancelStep",
				IsDispatch:  false,
				HandlerName: "HandleCancelStep",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "StepID", Type: "string", JSONName: "step_id", Required: true},
					{Name: "Reason", Type: "string", JSONName: "reason", Required: true},
				},
			},
			{
				Direction:  "server_to_client",
				TypeName:   "WorkflowUpdate",
				IsDispatch: false,
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Status", Type: "string", JSONName: "status", Required: true},
					{Name: "StepID", Type: "string", JSONName: "step_id", Required: true},
				},
			},
		},
	}
}

func TestGenerateTypedChannel_Unidirectional(t *testing.T) {
	ch := makeUnidirectionalChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	src := string(code)

	// ServerMessage interface exists with marker method
	if !strings.Contains(src, "type ServerMessage interface") {
		t.Error("expected ServerMessage interface")
	}
	if !strings.Contains(src, "serverMessage()") {
		t.Error("expected serverMessage() marker method")
	}

	// ServerMessage implementation for EmailProgress
	if !strings.Contains(src, "func (*EmailProgress) serverMessage()") {
		t.Error("expected serverMessage() implementation for EmailProgress")
	}
	if !strings.Contains(src, `func (*EmailProgress) TypeName() string { return "EmailProgress" }`) {
		t.Error("expected TypeName() implementation for EmailProgress")
	}

	// Send(ctx, ServerMessage) method exists
	if !strings.Contains(src, "func (tc *TypedChannel) Send(ctx context.Context, msg ServerMessage) error") {
		t.Error("expected Send(ctx, ServerMessage) method")
	}

	// Convenience SendEmailProgress
	if !strings.Contains(src, "func (tc *TypedChannel) SendEmailProgress(ctx context.Context, msg *EmailProgress) error") {
		t.Error("expected SendEmailProgress convenience method")
	}

	// No ClientMessage interface (no mid-stream types)
	if strings.Contains(src, "type ClientMessage interface") {
		t.Error("did NOT expect ClientMessage interface for unidirectional channel")
	}

	// No ReceiveAny
	if strings.Contains(src, "func (tc *TypedChannel) ReceiveAny") {
		t.Error("did NOT expect ReceiveAny for unidirectional channel")
	}

	// No ClientMessageHandler
	if strings.Contains(src, "type ClientMessageHandler struct") {
		t.Error("did NOT expect ClientMessageHandler for unidirectional channel")
	}

	// TypedChannel struct exists
	if !strings.Contains(src, "type TypedChannel struct") {
		t.Error("expected TypedChannel struct")
	}

	// TypedChannelFromContext
	if !strings.Contains(src, "func TypedChannelFromContext(ctx context.Context) *TypedChannel") {
		t.Error("expected TypedChannelFromContext function")
	}
}

func TestGenerateTypedChannel_Bidirectional(t *testing.T) {
	ch := makeBidirectionalChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	src := string(code)

	// Both ServerMessage and ClientMessage interfaces
	if !strings.Contains(src, "type ServerMessage interface") {
		t.Error("expected ServerMessage interface")
	}
	if !strings.Contains(src, "type ClientMessage interface") {
		t.Error("expected ClientMessage interface")
	}
	if !strings.Contains(src, "clientMessage()") {
		t.Error("expected clientMessage() marker method")
	}

	// ServerMessage implementations for all 4 FromServer types
	for _, typeName := range []string{"BotMessage", "ToolCallRequest", "StreamingToken", "ChatFinished"} {
		if !strings.Contains(src, "func (*"+typeName+") serverMessage()") {
			t.Errorf("expected serverMessage() implementation for %s", typeName)
		}
	}

	// Send(ServerMessage) with 4 convenience methods
	if !strings.Contains(src, "func (tc *TypedChannel) Send(ctx context.Context, msg ServerMessage) error") {
		t.Error("expected Send(ctx, ServerMessage) method")
	}
	for _, typeName := range []string{"BotMessage", "ToolCallRequest", "StreamingToken", "ChatFinished"} {
		if !strings.Contains(src, "func (tc *TypedChannel) Send"+typeName+"(ctx context.Context, msg *"+typeName+") error") {
			t.Errorf("expected Send%s convenience method", typeName)
		}
	}

	// ReceiveToolCallApproval method (type-specific)
	if !strings.Contains(src, "func (tc *TypedChannel) ReceiveToolCallApproval(ctx context.Context) (*ToolCallApproval, error)") {
		t.Error("expected ReceiveToolCallApproval method")
	}

	// ClientMessage implementation for ToolCallApproval (mid-stream)
	if !strings.Contains(src, "func (*ToolCallApproval) clientMessage()") {
		t.Error("expected clientMessage() implementation for ToolCallApproval")
	}

	// StartChat (dispatch) should NOT implement ClientMessage
	if strings.Contains(src, "func (*StartChat) clientMessage()") {
		t.Error("dispatch type StartChat should NOT implement ClientMessage")
	}

	// ClientMessageHandler struct with 1 field (only ToolCallApproval is mid-stream)
	if !strings.Contains(src, "type ClientMessageHandler struct") {
		t.Error("expected ClientMessageHandler struct")
	}
	if !strings.Contains(src, "OnToolCallApproval func(ctx context.Context, msg *ToolCallApproval) error") {
		t.Error("expected OnToolCallApproval field in ClientMessageHandler")
	}

	// ReceiveAny method with exhaustive switch
	if !strings.Contains(src, "func (tc *TypedChannel) ReceiveAny(ctx context.Context, h ClientMessageHandler) error") {
		t.Error("expected ReceiveAny method")
	}
	if !strings.Contains(src, `case "ToolCallApproval":`) {
		t.Error("expected ToolCallApproval case in ReceiveAny switch")
	}
}

func TestGenerateTypedChannel_MultiMidStream(t *testing.T) {
	ch := makeMultiMidStreamChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	src := string(code)

	// ClientMessageHandler should have 3 fields (ApproveStep, ProvideInput, CancelStep)
	if !strings.Contains(src, "type ClientMessageHandler struct") {
		t.Fatal("expected ClientMessageHandler struct")
	}

	// gofmt aligns struct fields with extra whitespace, so use regex to match
	expectedFieldPatterns := []string{
		`OnApproveStep\s+func\(ctx context\.Context, msg \*ApproveStep\) error`,
		`OnProvideInput\s+func\(ctx context\.Context, msg \*ProvideInput\) error`,
		`OnCancelStep\s+func\(ctx context\.Context, msg \*CancelStep\) error`,
	}
	for _, pattern := range expectedFieldPatterns {
		matched, err := regexp.MatchString(pattern, src)
		if err != nil {
			t.Fatalf("invalid regex %q: %v", pattern, err)
		}
		if !matched {
			t.Errorf("expected ClientMessageHandler field matching: %s", pattern)
		}
	}

	// ReceiveAny switch should have 3 cases
	expectedCases := []string{
		`case "ApproveStep":`,
		`case "ProvideInput":`,
		`case "CancelStep":`,
	}
	for _, c := range expectedCases {
		if !strings.Contains(src, c) {
			t.Errorf("expected ReceiveAny case: %s", c)
		}
	}

	_ = strings.Contains // ensure strings is used

	// validate() method should check all 3 fields
	expectedValidations := []string{
		"h.OnApproveStep == nil",
		"h.OnProvideInput == nil",
		"h.OnCancelStep == nil",
	}
	for _, v := range expectedValidations {
		if !strings.Contains(src, v) {
			t.Errorf("expected validate() check: %s", v)
		}
	}

	// Type-specific Receive methods for all 3 mid-stream types
	expectedReceive := []string{
		"func (tc *TypedChannel) ReceiveApproveStep(ctx context.Context) (*ApproveStep, error)",
		"func (tc *TypedChannel) ReceiveProvideInput(ctx context.Context) (*ProvideInput, error)",
		"func (tc *TypedChannel) ReceiveCancelStep(ctx context.Context) (*CancelStep, error)",
	}
	for _, r := range expectedReceive {
		if !strings.Contains(src, r) {
			t.Errorf("expected Receive method: %s", r)
		}
	}

	// StartWorkflow (dispatch) should NOT have a Receive method
	if strings.Contains(src, "ReceiveStartWorkflow") {
		t.Error("dispatch type StartWorkflow should NOT have a Receive method")
	}

	// StartWorkflow should NOT implement ClientMessage
	if strings.Contains(src, "func (*StartWorkflow) clientMessage()") {
		t.Error("dispatch type StartWorkflow should NOT implement ClientMessage")
	}
}

func TestGeneratedCode_Compiles_Unidirectional(t *testing.T) {
	ch := makeUnidirectionalChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "zz_generated_channel.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n\nCode:\n%s", err, string(code))
	}
}

func TestGeneratedCode_Compiles_Bidirectional(t *testing.T) {
	ch := makeBidirectionalChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "zz_generated_channel.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n\nCode:\n%s", err, string(code))
	}
}

func TestGeneratedCode_Compiles_MultiMidStream(t *testing.T) {
	ch := makeMultiMidStreamChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "zz_generated_channel.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v\n\nCode:\n%s", err, string(code))
	}
}

func TestGenerateTypedChannel_PackageDeclaration(t *testing.T) {
	ch := makeUnidirectionalChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	src := string(code)
	if !strings.Contains(src, "package email") {
		t.Errorf("expected package declaration 'package email', got:\n%s", src[:200])
	}

	ch2 := makeBidirectionalChannel()
	code2, err := GenerateTypedChannel(ch2, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	src2 := string(code2)
	if !strings.Contains(src2, "package chatbot") {
		t.Errorf("expected package declaration 'package chatbot', got:\n%s", src2[:200])
	}
}

func TestGenerateTypedChannel_Imports(t *testing.T) {
	ch := makeUnidirectionalChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	src := string(code)

	expectedImports := []string{
		`"context"`,
		`"encoding/json"`,
		`"fmt"`,
		`"myapp/shipq/lib/channel"`,
	}

	for _, imp := range expectedImports {
		if !strings.Contains(src, imp) {
			t.Errorf("expected import %s", imp)
		}
	}
}

func TestGenerateTypedChannel_CodeGenHeader(t *testing.T) {
	ch := makeUnidirectionalChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	src := string(code)
	if !strings.HasPrefix(src, "// Code generated by shipq. DO NOT EDIT.") {
		t.Error("expected DO NOT EDIT header")
	}
}

func TestGenerateTypedChannel_DifferentModulePath(t *testing.T) {
	ch := makeUnidirectionalChannel()
	ch.PackagePath = "github.com/company/project/channels/email"

	code, err := GenerateTypedChannel(ch, "github.com/company/project")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	src := string(code)
	if !strings.Contains(src, `"github.com/company/project/shipq/lib/channel"`) {
		t.Error("expected channel import path to use the given module path")
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "zz_generated_channel.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated code is not valid Go: %v", err)
	}
}

func TestGenerateTypedChannel_RawChannelUsed(t *testing.T) {
	ch := makeBidirectionalChannel()
	code, err := GenerateTypedChannel(ch, "myapp")
	if err != nil {
		t.Fatalf("GenerateTypedChannel failed: %v", err)
	}

	src := string(code)

	// TypedChannel wraps raw *channel.Channel
	if !strings.Contains(src, "raw *channel.Channel") {
		t.Error("expected TypedChannel to wrap raw *channel.Channel")
	}

	// FromContext delegates to channel.FromContext
	if !strings.Contains(src, "channel.FromContext(ctx)") {
		t.Error("expected TypedChannelFromContext to delegate to channel.FromContext")
	}

	// Send delegates to raw.Send
	if !strings.Contains(src, "tc.raw.Send(ctx,") {
		t.Error("expected Send to delegate to raw.Send")
	}

	// Receive delegates to raw.Receive
	if !strings.Contains(src, "tc.raw.Receive(ctx,") {
		t.Error("expected Receive methods to delegate to raw.Receive")
	}

	// ReceiveAny delegates to raw.ReceiveAny
	if !strings.Contains(src, "tc.raw.ReceiveAny(ctx)") {
		t.Error("expected ReceiveAny to delegate to raw.ReceiveAny")
	}
}
