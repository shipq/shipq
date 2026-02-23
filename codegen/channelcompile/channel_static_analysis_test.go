package channelcompile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestParseFileForHandlerFuncs_BasicHandler(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package email

import "context"

type SendEmailRequest struct {
	To      string
	Subject string
}

func HandleSendEmailRequest(ctx context.Context, req *SendEmailRequest) error {
	return nil
}
`
	filePath := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(handlers))
	}

	h := handlers[0]
	if h.FuncName != "HandleSendEmailRequest" {
		t.Errorf("expected FuncName 'HandleSendEmailRequest', got %q", h.FuncName)
	}
	if h.TypeName != "SendEmailRequest" {
		t.Errorf("expected TypeName 'SendEmailRequest', got %q", h.TypeName)
	}
	if h.Line == 0 {
		t.Error("expected non-zero line number")
	}
}

func TestParseFileForHandlerFuncs_MultipleHandlers(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package chatbot

import "context"

type StartChat struct {
	Prompt string
}

type ToolCallApproval struct {
	CallID   string
	Approved bool
}

type ProvideInput struct {
	Key   string
	Value string
}

func HandleStartChat(ctx context.Context, req *StartChat) error {
	return nil
}

func HandleToolCallApproval(ctx context.Context, req *ToolCallApproval) error {
	return nil
}

func HandleProvideInput(ctx context.Context, req *ProvideInput) error {
	return nil
}
`
	filePath := filepath.Join(tmpDir, "handlers.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 3 {
		t.Fatalf("expected 3 handlers, got %d", len(handlers))
	}

	expectedNames := []string{"HandleStartChat", "HandleToolCallApproval", "HandleProvideInput"}
	expectedTypes := []string{"StartChat", "ToolCallApproval", "ProvideInput"}

	for i, h := range handlers {
		if h.FuncName != expectedNames[i] {
			t.Errorf("handler %d: expected FuncName %q, got %q", i, expectedNames[i], h.FuncName)
		}
		if h.TypeName != expectedTypes[i] {
			t.Errorf("handler %d: expected TypeName %q, got %q", i, expectedTypes[i], h.TypeName)
		}
	}
}

func TestParseFileForHandlerFuncs_IgnoresNonHandlers(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package email

import "context"

type SendEmailRequest struct {
	To string
}

// This is a valid handler
func HandleSendEmailRequest(ctx context.Context, req *SendEmailRequest) error {
	return nil
}

// Not a handler — doesn't start with "Handle"
func ProcessEmail(ctx context.Context, req *SendEmailRequest) error {
	return nil
}

// Not a handler — "Handle" alone with no type name
func Handle() {
}

// Not a handler — wrong signature (no context param)
func HandleBadSig(req *SendEmailRequest) error {
	return nil
}

// Not a handler — wrong signature (returns two values)
func HandleMultiReturn(ctx context.Context, req *SendEmailRequest) (*SendEmailRequest, error) {
	return nil, nil
}

// Not a handler — method receiver (not a top-level function)
type MyService struct{}
func (s *MyService) HandleServiceRequest(ctx context.Context, req *SendEmailRequest) error {
	return nil
}

// Not a handler — second param not a pointer
func HandleValueParam(ctx context.Context, req SendEmailRequest) error {
	return nil
}

// Helper function
func helperFunc() string {
	return "helper"
}
`
	filePath := filepath.Join(tmpDir, "handlers.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	// Only HandleSendEmailRequest should match
	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler (only HandleSendEmailRequest), got %d: %+v", len(handlers), handlers)
	}

	if handlers[0].FuncName != "HandleSendEmailRequest" {
		t.Errorf("expected FuncName 'HandleSendEmailRequest', got %q", handlers[0].FuncName)
	}
}

func TestParseFileForHandlerFuncs_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package email
`
	filePath := filepath.Join(tmpDir, "empty.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 0 {
		t.Fatalf("expected 0 handlers, got %d", len(handlers))
	}
}

func TestParseFileForHandlerFuncs_InvalidGoFile(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "invalid.go")
	if err := os.WriteFile(filePath, []byte("this is not valid go"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := parseFileForHandlerFuncs(filePath)
	if err == nil {
		t.Error("expected error for invalid Go file")
	}
}

func TestFindChannelHandlerFuncs_AcrossFiles(t *testing.T) {
	tmpDir := t.TempDir()

	modulePath := "example.com/myapp"
	pkgDir := filepath.Join(tmpDir, "channels", "chatbot")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// handler1.go
	handler1 := `package chatbot

import "context"

type StartChat struct {
	Prompt string
}

func HandleStartChat(ctx context.Context, req *StartChat) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "handler1.go"), []byte(handler1), 0644); err != nil {
		t.Fatalf("failed to write handler1.go: %v", err)
	}

	// handler2.go
	handler2 := `package chatbot

import "context"

type ToolCallApproval struct {
	CallID   string
	Approved bool
}

func HandleToolCallApproval(ctx context.Context, req *ToolCallApproval) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "handler2.go"), []byte(handler2), 0644); err != nil {
		t.Fatalf("failed to write handler2.go: %v", err)
	}

	// types.go (no handlers)
	types := `package chatbot

type BotMessage struct {
	Text string
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "types.go"), []byte(types), 0644); err != nil {
		t.Fatalf("failed to write types.go: %v", err)
	}

	importPath := modulePath + "/channels/chatbot"
	handlers, err := findChannelHandlerFuncs(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("findChannelHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 2 {
		t.Fatalf("expected 2 handlers, got %d: %+v", len(handlers), handlers)
	}

	// Verify both are found (order depends on file system / sort order)
	foundNames := map[string]bool{}
	for _, h := range handlers {
		foundNames[h.FuncName] = true
		if h.PackagePath != importPath {
			t.Errorf("expected PackagePath %q, got %q", importPath, h.PackagePath)
		}
	}

	if !foundNames["HandleStartChat"] {
		t.Error("expected to find HandleStartChat")
	}
	if !foundNames["HandleToolCallApproval"] {
		t.Error("expected to find HandleToolCallApproval")
	}
}

func TestFindChannelHandlerFuncs_SkipsTestFiles(t *testing.T) {
	tmpDir := t.TempDir()

	modulePath := "example.com/myapp"
	pkgDir := filepath.Join(tmpDir, "channels", "email")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// handler.go with a real handler
	handler := `package email

import "context"

type SendEmailRequest struct {
	To string
}

func HandleSendEmailRequest(ctx context.Context, req *SendEmailRequest) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "handler.go"), []byte(handler), 0644); err != nil {
		t.Fatalf("failed to write handler.go: %v", err)
	}

	// handler_test.go with a handler-like function (should be skipped)
	testFile := `package email

import "context"

type TestRequest struct{}

func HandleTestRequest(ctx context.Context, req *TestRequest) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "handler_test.go"), []byte(testFile), 0644); err != nil {
		t.Fatalf("failed to write handler_test.go: %v", err)
	}

	importPath := modulePath + "/channels/email"
	handlers, err := findChannelHandlerFuncs(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("findChannelHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler (test file should be skipped), got %d", len(handlers))
	}

	if handlers[0].FuncName != "HandleSendEmailRequest" {
		t.Errorf("expected HandleSendEmailRequest, got %q", handlers[0].FuncName)
	}
}

func TestFindChannelHandlerFuncs_SkipsGeneratedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	modulePath := "example.com/myapp"
	pkgDir := filepath.Join(tmpDir, "channels", "email")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// handler.go with a real handler
	handler := `package email

import "context"

type SendEmailRequest struct {
	To string
}

func HandleSendEmailRequest(ctx context.Context, req *SendEmailRequest) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "handler.go"), []byte(handler), 0644); err != nil {
		t.Fatalf("failed to write handler.go: %v", err)
	}

	// zz_generated_channel.go (should be skipped)
	generated := `package email

import "context"

type GeneratedType struct{}

func HandleGeneratedType(ctx context.Context, req *GeneratedType) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "zz_generated_channel.go"), []byte(generated), 0644); err != nil {
		t.Fatalf("failed to write zz_generated_channel.go: %v", err)
	}

	importPath := modulePath + "/channels/email"
	handlers, err := findChannelHandlerFuncs(tmpDir, modulePath, importPath)
	if err != nil {
		t.Fatalf("findChannelHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 1 {
		t.Fatalf("expected 1 handler (generated file should be skipped), got %d", len(handlers))
	}
}

func TestMergeChannelStaticAnalysis_MatchesHandlers(t *testing.T) {
	tmpDir := t.TempDir()

	modulePath := "example.com/myapp"
	pkgDir := filepath.Join(tmpDir, "channels", "chatbot")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// register.go (needed so static analysis doesn't skip)
	register := `package chatbot

func Register(app interface{}) {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(register), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	// handler.go
	handler := `package chatbot

import "context"

type StartChat struct {
	Prompt string
}

type ToolCallApproval struct {
	CallID   string
	Approved bool
}

func HandleStartChat(ctx context.Context, req *StartChat) error {
	return nil
}

func HandleToolCallApproval(ctx context.Context, req *ToolCallApproval) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "handler.go"), []byte(handler), 0644); err != nil {
		t.Fatalf("failed to write handler.go: %v", err)
	}

	importPath := modulePath + "/channels/chatbot"
	channels := []codegen.SerializedChannelInfo{
		{
			Name:        "chatbot",
			PackagePath: importPath,
			PackageName: "chatbot",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "StartChat", IsDispatch: true},
				{Direction: "client_to_server", TypeName: "ToolCallApproval", IsDispatch: false},
				{Direction: "server_to_client", TypeName: "BotMessage"},
			},
		},
	}

	err := MergeChannelStaticAnalysis(tmpDir, modulePath, []string{importPath}, channels)
	if err != nil {
		t.Fatalf("MergeChannelStaticAnalysis failed: %v", err)
	}

	// Verify handler names are filled in
	msgs := channels[0].Messages
	if msgs[0].HandlerName != "HandleStartChat" {
		t.Errorf("expected HandlerName 'HandleStartChat', got %q", msgs[0].HandlerName)
	}
	if msgs[1].HandlerName != "HandleToolCallApproval" {
		t.Errorf("expected HandlerName 'HandleToolCallApproval', got %q", msgs[1].HandlerName)
	}

	// Server-to-client messages should not have handler names
	if msgs[2].HandlerName != "" {
		t.Errorf("expected empty HandlerName for server_to_client message, got %q", msgs[2].HandlerName)
	}
}

func TestMergeChannelStaticAnalysis_NoRegisterFile(t *testing.T) {
	tmpDir := t.TempDir()

	modulePath := "example.com/myapp"
	pkgDir := filepath.Join(tmpDir, "channels", "email")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// No register.go — static analysis should be skipped gracefully
	handler := `package email

import "context"

type SendEmailRequest struct{}

func HandleSendEmailRequest(ctx context.Context, req *SendEmailRequest) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "handler.go"), []byte(handler), 0644); err != nil {
		t.Fatalf("failed to write handler.go: %v", err)
	}

	importPath := modulePath + "/channels/email"
	channels := []codegen.SerializedChannelInfo{
		{
			Name:        "email",
			PackagePath: importPath,
			PackageName: "email",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "SendEmailRequest", IsDispatch: true},
				{Direction: "server_to_client", TypeName: "EmailProgress"},
			},
		},
	}

	err := MergeChannelStaticAnalysis(tmpDir, modulePath, []string{importPath}, channels)
	if err != nil {
		t.Fatalf("MergeChannelStaticAnalysis failed: %v", err)
	}

	// Without register.go, handler names should remain empty
	if channels[0].Messages[0].HandlerName != "" {
		t.Errorf("expected empty HandlerName when register.go is missing, got %q", channels[0].Messages[0].HandlerName)
	}
}

func TestMergeChannelStaticAnalysis_UnmatchedType(t *testing.T) {
	tmpDir := t.TempDir()

	modulePath := "example.com/myapp"
	pkgDir := filepath.Join(tmpDir, "channels", "email")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	register := `package email

func Register(app interface{}) {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "register.go"), []byte(register), 0644); err != nil {
		t.Fatalf("failed to write register.go: %v", err)
	}

	// Handler function that doesn't match any registered message type
	handler := `package email

import "context"

type SomethingElse struct{}

func HandleSomethingElse(ctx context.Context, req *SomethingElse) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "handler.go"), []byte(handler), 0644); err != nil {
		t.Fatalf("failed to write handler.go: %v", err)
	}

	importPath := modulePath + "/channels/email"
	channels := []codegen.SerializedChannelInfo{
		{
			Name:        "email",
			PackagePath: importPath,
			PackageName: "email",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "SendEmailRequest", IsDispatch: true},
				{Direction: "server_to_client", TypeName: "EmailProgress"},
			},
		},
	}

	// Should not error — unmatched types are ignored
	err := MergeChannelStaticAnalysis(tmpDir, modulePath, []string{importPath}, channels)
	if err != nil {
		t.Fatalf("MergeChannelStaticAnalysis failed: %v", err)
	}

	// SendEmailRequest has no matching handler, so HandlerName stays empty
	if channels[0].Messages[0].HandlerName != "" {
		t.Errorf("expected empty HandlerName for unmatched type, got %q", channels[0].Messages[0].HandlerName)
	}
}

func TestIsValidChannelHandlerSignature_ValidCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Various valid signatures
	content := `package test

import "context"

type Msg struct{}

func HandleMsg(ctx context.Context, req *Msg) error {
	return nil
}
`
	filePath := filepath.Join(tmpDir, "valid.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 1 {
		t.Fatalf("expected 1 valid handler, got %d", len(handlers))
	}
}

func TestIsValidChannelHandlerSignature_RejectsNoReturn(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package test

import "context"

type Msg struct{}

func HandleMsg(ctx context.Context, req *Msg) {
}
`
	filePath := filepath.Join(tmpDir, "noreturns.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 0 {
		t.Fatalf("expected 0 handlers (no return value), got %d", len(handlers))
	}
}

func TestIsValidChannelHandlerSignature_RejectsTwoReturns(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package test

import "context"

type Msg struct{}

func HandleMsg(ctx context.Context, req *Msg) (string, error) {
	return "", nil
}
`
	filePath := filepath.Join(tmpDir, "tworeturns.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 0 {
		t.Fatalf("expected 0 handlers (two return values), got %d", len(handlers))
	}
}

func TestIsValidChannelHandlerSignature_RejectsThreeParams(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package test

import "context"

type Msg struct{}

func HandleMsg(ctx context.Context, req *Msg, extra string) error {
	return nil
}
`
	filePath := filepath.Join(tmpDir, "threeparams.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 0 {
		t.Fatalf("expected 0 handlers (three params), got %d", len(handlers))
	}
}

func TestIsValidChannelHandlerSignature_RejectsNonPointerSecondParam(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package test

import "context"

type Msg struct{}

func HandleMsg(ctx context.Context, req Msg) error {
	return nil
}
`
	filePath := filepath.Join(tmpDir, "nonptr.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 0 {
		t.Fatalf("expected 0 handlers (non-pointer second param), got %d", len(handlers))
	}
}

func TestIsValidChannelHandlerSignature_RejectsNonErrorReturn(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package test

import "context"

type Msg struct{}

func HandleMsg(ctx context.Context, req *Msg) string {
	return ""
}
`
	filePath := filepath.Join(tmpDir, "nonerror.go")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	handlers, err := parseFileForHandlerFuncs(filePath)
	if err != nil {
		t.Fatalf("parseFileForHandlerFuncs failed: %v", err)
	}

	if len(handlers) != 0 {
		t.Fatalf("expected 0 handlers (non-error return), got %d", len(handlers))
	}
}

func TestImportPathToChannelRegisterFilePath(t *testing.T) {
	tests := []struct {
		name        string
		projectRoot string
		modulePath  string
		importPath  string
		wantPath    string
	}{
		{
			name:        "simple channel package",
			projectRoot: "/project",
			modulePath:  "com.myapp",
			importPath:  "com.myapp/channels/email",
			wantPath:    "/project/channels/email/register.go",
		},
		{
			name:        "nested channel package",
			projectRoot: "/project",
			modulePath:  "com.myapp",
			importPath:  "com.myapp/channels/admin/notifications",
			wantPath:    "/project/channels/admin/notifications/register.go",
		},
		{
			name:        "github module path",
			projectRoot: "/home/user/myproject",
			modulePath:  "github.com/user/myproject",
			importPath:  "github.com/user/myproject/channels/chatbot",
			wantPath:    "/home/user/myproject/channels/chatbot/register.go",
		},
		{
			name:        "import path equals module path",
			projectRoot: "/project",
			modulePath:  "com.myapp",
			importPath:  "com.myapp",
			wantPath:    "/project/register.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := importPathToChannelRegisterFilePath(tt.projectRoot, tt.modulePath, tt.importPath)
			if got != tt.wantPath {
				t.Errorf("importPathToChannelRegisterFilePath() = %q, want %q", got, tt.wantPath)
			}
		})
	}
}
