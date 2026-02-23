package channelgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestGenerateIntegrationTests_UnidirectionalChannel(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateIntegrationTestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should have package spec
	if !strings.Contains(codeStr, "package spec") {
		t.Error("expected 'package spec' in generated code")
	}

	// Should have the test function
	if !strings.Contains(codeStr, "TestIntegration_Email_Dispatch") {
		t.Error("expected TestIntegration_Email_Dispatch function")
	}

	// Should create TestRecorder
	if !strings.Contains(codeStr, "channel.NewTestRecorder()") {
		t.Error("expected NewTestRecorder() call")
	}

	// Should create MockQueue
	if !strings.Contains(codeStr, "channel.NewMockQueue()") {
		t.Error("expected NewMockQueue() call")
	}

	// Should register the dispatch handler
	if !strings.Contains(codeStr, `queue.RegisterTask("email"`) {
		t.Error("expected RegisterTask call for email")
	}

	// Should reference the handler function
	if !strings.Contains(codeStr, "email.HandleSendEmailRequest") {
		t.Error("expected email.HandleSendEmailRequest reference")
	}

	// Should dispatch via SendTask
	if !strings.Contains(codeStr, `queue.SendTask("email"`) {
		t.Error("expected SendTask call")
	}

	// Should verify EmailProgress was sent
	if !strings.Contains(codeStr, `recorder.HasSent("EmailProgress")`) {
		t.Error("expected HasSent check for EmailProgress")
	}

	// Should import the channel package
	if !strings.Contains(codeStr, `"myapp/channels/email"`) {
		t.Error("expected email channel package import")
	}

	// Should import the channel runtime
	if !strings.Contains(codeStr, `"myapp/shipq/lib/channel"`) {
		t.Error("expected channel runtime import")
	}

	// Should use DispatchPayload
	if !strings.Contains(codeStr, "channel.DispatchPayload") {
		t.Error("expected DispatchPayload usage")
	}

	// Should use ComputeChannelID
	if !strings.Contains(codeStr, "channel.ComputeChannelID") {
		t.Error("expected ComputeChannelID usage")
	}

	// Should use NewChannel
	if !strings.Contains(codeStr, "channel.NewChannel") {
		t.Error("expected NewChannel usage")
	}

	// Should use WithChannel
	if !strings.Contains(codeStr, "channel.WithChannel") {
		t.Error("expected WithChannel usage")
	}
}

func TestGenerateIntegrationTests_BidirectionalChannel(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeBidirectionalChannel(),
	}

	code, err := GenerateIntegrationTestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should have test function for chatbot
	if !strings.Contains(codeStr, "TestIntegration_Chatbot_Dispatch") {
		t.Error("expected TestIntegration_Chatbot_Dispatch function")
	}

	// Should reference StartChat handler
	if !strings.Contains(codeStr, "chatbot.HandleStartChat") {
		t.Error("expected chatbot.HandleStartChat reference")
	}

	// Should pre-queue ToolCallApproval (mid-stream client message)
	if !strings.Contains(codeStr, `recorder.EnqueueIncoming("ToolCallApproval"`) {
		t.Error("expected EnqueueIncoming call for ToolCallApproval")
	}

	// Should verify server messages were sent
	if !strings.Contains(codeStr, `recorder.HasSent("BotMessage")`) {
		t.Error("expected HasSent check for BotMessage")
	}
	if !strings.Contains(codeStr, `recorder.HasSent("ToolCallRequest")`) {
		t.Error("expected HasSent check for ToolCallRequest")
	}
	if !strings.Contains(codeStr, `recorder.HasSent("StreamingToken")`) {
		t.Error("expected HasSent check for StreamingToken")
	}
	if !strings.Contains(codeStr, `recorder.HasSent("ChatFinished")`) {
		t.Error("expected HasSent check for ChatFinished")
	}

	// Should import chatbot channel package
	if !strings.Contains(codeStr, `"myapp/channels/chatbot"`) {
		t.Error("expected chatbot channel package import")
	}
}

func TestGenerateIntegrationTests_PublicChannel(t *testing.T) {
	ch := makeUnidirectionalChannel()
	ch.IsPublic = true
	ch.Visibility = "frontend"
	channels := []codegen.SerializedChannelInfo{ch}

	code, err := GenerateIntegrationTestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should verify no account scoping for public channels
	if !strings.Contains(codeStr, "AccountID") {
		t.Error("expected AccountID reference for public channel check")
	}

	// Should set IsPublic true
	if !strings.Contains(codeStr, "IsPublic:    true") {
		t.Error("expected IsPublic: true in dispatch payload")
	}
}

func TestGenerateIntegrationTests_ValidGo(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
		makeBidirectionalChannel(),
	}

	code, err := GenerateIntegrationTestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "zz_generated_integration_test.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated integration test is not valid Go: %v\ncode:\n%s", err, string(code))
	}
}

func TestGenerateIntegrationTests_MultipleChannels(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
		makeBidirectionalChannel(),
	}

	code, err := GenerateIntegrationTestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should have test functions for both channels
	if !strings.Contains(codeStr, "TestIntegration_Email_Dispatch") {
		t.Error("expected TestIntegration_Email_Dispatch")
	}
	if !strings.Contains(codeStr, "TestIntegration_Chatbot_Dispatch") {
		t.Error("expected TestIntegration_Chatbot_Dispatch")
	}
}

func TestGenerateIntegrationTests_EmptyChannels(t *testing.T) {
	code, err := GenerateIntegrationTestCode(nil, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	if !strings.Contains(codeStr, "package spec") {
		t.Error("expected 'package spec' even for empty channels")
	}
}

func TestGenerateIntegrationTests_NoDispatchMessage(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name:        "broken",
			PackagePath: "myapp/channels/broken",
			PackageName: "broken",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "server_to_client", TypeName: "SomeResponse"},
			},
		},
	}

	code, err := GenerateIntegrationTestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should not generate a test for a channel without a dispatch message
	if strings.Contains(codeStr, "TestIntegration_Broken_Dispatch") {
		t.Error("should not generate test for channel without dispatch message")
	}
}

func TestGenerateIntegrationTests_CodeGenComment(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateIntegrationTestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	if !strings.Contains(codeStr, "Code generated by shipq. DO NOT EDIT.") {
		t.Error("expected code generation header comment")
	}
}

func TestGenerateIntegrationTests_HasBuildTag(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateIntegrationTestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	// The integration build tag must be present so tests don't run during regular `go test ./...`
	if !strings.Contains(codeStr, "//go:build integration") {
		t.Error("expected //go:build integration build tag")
	}
}

func TestGenerateIntegrationTests_EmptyChannels_HasBuildTag(t *testing.T) {
	code, err := GenerateIntegrationTestCode(nil, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	if !strings.Contains(codeStr, "//go:build integration") {
		t.Error("expected //go:build integration build tag even for empty channels")
	}
}

func TestGenerateIntegrationTests_TestFieldValues(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateIntegrationTestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateIntegrationTestCode() error = %v", err)
	}
	codeStr := string(code)

	// The SendEmailRequest has string fields (To, Subject, Body)
	// Test values should be "test" strings
	if !strings.Contains(codeStr, `"test"`) {
		t.Error("expected test values for string fields")
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"email", "Email"},
		{"chatbot", "Chatbot"},
		{"", ""},
		{"A", "A"},
		{"abc", "Abc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := capitalizeFirst(tt.input)
			if got != tt.expected {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIntegrationTestName(t *testing.T) {
	tests := []struct {
		channelName string
		expected    string
	}{
		{"email", "TestIntegration_Email_Dispatch"},
		{"chatbot", "TestIntegration_Chatbot_Dispatch"},
		{"user_notifications", "TestIntegration_User_notifications_Dispatch"},
	}

	for _, tt := range tests {
		t.Run(tt.channelName, func(t *testing.T) {
			got := integrationTestName(tt.channelName)
			if got != tt.expected {
				t.Errorf("integrationTestName(%q) = %q, want %q", tt.channelName, got, tt.expected)
			}
		})
	}
}

func TestTestZeroValue(t *testing.T) {
	tests := []struct {
		goType   string
		expected string
	}{
		{"string", `"test"`},
		{"int", "1"},
		{"int64", "1"},
		{"bool", "true"},
		{"float64", "1.0"},
		{"[]string", "nil"},
		{"*int", "nil"},
		{"map[string]int", "nil"},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			got := testZeroValue(tt.goType)
			if got != tt.expected {
				t.Errorf("testZeroValue(%q) = %q, want %q", tt.goType, got, tt.expected)
			}
		})
	}
}
