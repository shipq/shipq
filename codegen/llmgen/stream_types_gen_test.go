package llmgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectLLMChannels_WithClient(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	channelPkg := modulePath + "/channels/chatbot"

	pkgDir := filepath.Join(tmpDir, "channels", "chatbot")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package chatbot

import (
	"context"
	"myapp/shipq/lib/llm"
)

func Setup(ctx context.Context) context.Context {
	client := llm.WithClient(ctx, provider)
	_ = client
	return ctx
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "setup.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write setup.go: %v", err)
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 LLM channel, got %d", len(result))
	}
	if result[0] != channelPkg {
		t.Errorf("expected %q, got %q", channelPkg, result[0])
	}
}

func TestDetectLLMChannels_WithNamedClient(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	channelPkg := modulePath + "/channels/assistant"

	pkgDir := filepath.Join(tmpDir, "channels", "assistant")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package assistant

import (
	"context"
	"myapp/shipq/lib/llm"
)

func Setup(ctx context.Context) context.Context {
	client := llm.WithNamedClient(ctx, "gpt4", provider)
	_ = client
	return ctx
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "setup.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write setup.go: %v", err)
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 LLM channel, got %d", len(result))
	}
	if result[0] != channelPkg {
		t.Errorf("expected %q, got %q", channelPkg, result[0])
	}
}

func TestDetectLLMChannels_NoLLMImport(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	channelPkg := modulePath + "/channels/plain"

	pkgDir := filepath.Join(tmpDir, "channels", "plain")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package plain

import "context"

func HandleRequest(ctx context.Context, req *Request) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "handler.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write handler.go: %v", err)
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 LLM channels for package without llm import, got %d", len(result))
	}
}

func TestDetectLLMChannels_LLMImportButNoWithClient(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	channelPkg := modulePath + "/channels/partial"

	pkgDir := filepath.Join(tmpDir, "channels", "partial")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package partial

import (
	"myapp/shipq/lib/llm"
)

func DoSomething() {
	_ = llm.NewApp()
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "stuff.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write stuff.go: %v", err)
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 LLM channels when llm is imported but WithClient is not called, got %d", len(result))
	}
}

func TestDetectLLMChannels_SkipsTestFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	channelPkg := modulePath + "/channels/testonly"

	pkgDir := filepath.Join(tmpDir, "channels", "testonly")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// Put WithClient call only in a test file.
	testContent := `package testonly

import (
	"context"
	"myapp/shipq/lib/llm"
	"testing"
)

func TestSomething(t *testing.T) {
	ctx := context.Background()
	_ = llm.WithClient(ctx, nil)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "stuff_test.go"), []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Non-test file without llm usage.
	normalContent := `package testonly

func Helper() {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "helper.go"), []byte(normalContent), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 LLM channels (test files should be skipped), got %d", len(result))
	}
}

func TestDetectLLMChannels_SkipsGeneratedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	channelPkg := modulePath + "/channels/genonly"

	pkgDir := filepath.Join(tmpDir, "channels", "genonly")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	// Put WithClient call only in a generated file.
	genContent := `package genonly

import (
	"context"
	"myapp/shipq/lib/llm"
)

func GeneratedSetup(ctx context.Context) context.Context {
	_ = llm.WithClient(ctx, nil)
	return ctx
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "zz_generated_channel.go"), []byte(genContent), 0644); err != nil {
		t.Fatalf("failed to write generated file: %v", err)
	}

	// Non-generated file without llm usage.
	normalContent := `package genonly

func Helper() {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "helper.go"), []byte(normalContent), 0644); err != nil {
		t.Fatalf("failed to write helper file: %v", err)
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 LLM channels (generated files should be skipped), got %d", len(result))
	}
}

func TestDetectLLMChannels_MultipleChannels(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"

	// Channel 1: LLM-enabled.
	llmPkgDir := filepath.Join(tmpDir, "channels", "chat")
	if err := os.MkdirAll(llmPkgDir, 0755); err != nil {
		t.Fatalf("failed to create chat dir: %v", err)
	}
	llmContent := `package chat

import (
	"context"
	"myapp/shipq/lib/llm"
)

func Setup(ctx context.Context) context.Context {
	return llm.WithClient(ctx, nil)
}
`
	if err := os.WriteFile(filepath.Join(llmPkgDir, "setup.go"), []byte(llmContent), 0644); err != nil {
		t.Fatalf("failed to write chat setup.go: %v", err)
	}

	// Channel 2: Not LLM-enabled.
	plainPkgDir := filepath.Join(tmpDir, "channels", "notify")
	if err := os.MkdirAll(plainPkgDir, 0755); err != nil {
		t.Fatalf("failed to create notify dir: %v", err)
	}
	plainContent := `package notify

func HandleNotification() {}
`
	if err := os.WriteFile(filepath.Join(plainPkgDir, "handler.go"), []byte(plainContent), 0644); err != nil {
		t.Fatalf("failed to write notify handler.go: %v", err)
	}

	// Channel 3: Also LLM-enabled.
	llmPkgDir2 := filepath.Join(tmpDir, "channels", "assistant")
	if err := os.MkdirAll(llmPkgDir2, 0755); err != nil {
		t.Fatalf("failed to create assistant dir: %v", err)
	}
	llmContent2 := `package assistant

import (
	"context"
	"myapp/shipq/lib/llm"
)

func Init(ctx context.Context) {
	llm.WithNamedClient(ctx, "claude", nil)
}
`
	if err := os.WriteFile(filepath.Join(llmPkgDir2, "init.go"), []byte(llmContent2), 0644); err != nil {
		t.Fatalf("failed to write assistant init.go: %v", err)
	}

	channelPkgs := []string{
		modulePath + "/channels/chat",
		modulePath + "/channels/notify",
		modulePath + "/channels/assistant",
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, channelPkgs)
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 LLM channels, got %d: %v", len(result), result)
	}

	// Check that the right channels are detected.
	found := map[string]bool{}
	for _, pkg := range result {
		found[pkg] = true
	}
	if !found[modulePath+"/channels/chat"] {
		t.Error("expected channels/chat to be detected as LLM-enabled")
	}
	if !found[modulePath+"/channels/assistant"] {
		t.Error("expected channels/assistant to be detected as LLM-enabled")
	}
	if found[modulePath+"/channels/notify"] {
		t.Error("channels/notify should NOT be detected as LLM-enabled")
	}
}

func TestDetectLLMChannels_AliasedImport(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	channelPkg := modulePath + "/channels/aliased"

	pkgDir := filepath.Join(tmpDir, "channels", "aliased")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package aliased

import (
	"context"
	myllm "myapp/shipq/lib/llm"
)

func Setup(ctx context.Context) context.Context {
	return myllm.WithClient(ctx, nil)
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "setup.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write setup.go: %v", err)
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 LLM channel with aliased import, got %d", len(result))
	}
}

func TestDetectLLMChannels_EmptyList(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := DetectLLMChannels(tmpDir, "myapp", []string{})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 LLM channels for empty input, got %d", len(result))
	}
}

func TestDetectLLMChannels_NonexistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := DetectLLMChannels(tmpDir, "myapp", []string{"myapp/channels/nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent channel directory")
	}
}

// ── TypeScript generation tests ──────────────────────────────────────────────

func TestGenerateLLMStreamTypeScript_ContainsAllInterfaces(t *testing.T) {
	ts := GenerateLLMStreamTypeScript()

	interfaces := []string{
		"export interface LLMTextDelta",
		"export interface LLMToolCallStart",
		"export interface LLMToolCallResult",
		"export interface LLMDone",
	}

	for _, iface := range interfaces {
		if !strings.Contains(ts, iface) {
			t.Errorf("expected TypeScript to contain %q", iface)
		}
	}
}

func TestGenerateLLMStreamTypeScript_LLMTextDeltaFields(t *testing.T) {
	ts := GenerateLLMStreamTypeScript()

	if !strings.Contains(ts, "text: string;") {
		t.Error("LLMTextDelta: expected 'text: string;' field")
	}
}

func TestGenerateLLMStreamTypeScript_LLMToolCallStartFields(t *testing.T) {
	ts := GenerateLLMStreamTypeScript()

	expectedFields := []string{
		"tool_call_id: string;",
		"tool_name: string;",
		"input: Record<string, unknown>;",
	}

	for _, field := range expectedFields {
		if !strings.Contains(ts, field) {
			t.Errorf("LLMToolCallStart: expected field %q", field)
		}
	}
}

func TestGenerateLLMStreamTypeScript_LLMToolCallResultFields(t *testing.T) {
	ts := GenerateLLMStreamTypeScript()

	expectedFields := []string{
		"tool_call_id: string;",
		"tool_name: string;",
		"output?: Record<string, unknown>;",
		"error?: string;",
		"duration_ms: number;",
	}

	for _, field := range expectedFields {
		if !strings.Contains(ts, field) {
			t.Errorf("LLMToolCallResult: expected field %q", field)
		}
	}
}

func TestGenerateLLMStreamTypeScript_LLMDoneFields(t *testing.T) {
	ts := GenerateLLMStreamTypeScript()

	expectedFields := []string{
		"text: string;",
		"input_tokens: number;",
		"output_tokens: number;",
		"tool_call_count: number;",
	}

	for _, field := range expectedFields {
		if !strings.Contains(ts, field) {
			t.Errorf("LLMDone: expected field %q", field)
		}
	}
}

func TestGenerateLLMStreamTypeScript_HasAutoInjectedComment(t *testing.T) {
	ts := GenerateLLMStreamTypeScript()

	if !strings.Contains(ts, "auto-injected by shipq llm compile") {
		t.Error("expected auto-injected comment in TypeScript output")
	}
}

// ── Union member tests ───────────────────────────────────────────────────────

func TestLLMFromServerUnionMembers_ReturnsFourMembers(t *testing.T) {
	members := LLMFromServerUnionMembers()

	if len(members) != 4 {
		t.Fatalf("expected 4 union members, got %d", len(members))
	}
}

func TestLLMFromServerUnionMembers_ContainsAllTypes(t *testing.T) {
	members := LLMFromServerUnionMembers()

	expectedTypes := []string{
		"LLMTextDelta",
		"LLMToolCallStart",
		"LLMToolCallResult",
		"LLMDone",
	}

	joined := strings.Join(members, "\n")
	for _, typeName := range expectedTypes {
		if !strings.Contains(joined, typeName) {
			t.Errorf("expected union members to contain %q", typeName)
		}
	}
}

func TestLLMFromServerUnionMembers_HasCorrectFormat(t *testing.T) {
	members := LLMFromServerUnionMembers()

	for _, member := range members {
		if !strings.HasPrefix(member, `{ type: "`) {
			t.Errorf("expected member to start with '{ type: \"', got %q", member)
		}
		if !strings.HasSuffix(member, " }") {
			t.Errorf("expected member to end with ' }', got %q", member)
		}
		if !strings.Contains(member, "; data: ") {
			t.Errorf("expected member to contain '; data: ', got %q", member)
		}
	}
}

// ── Marker file tests ────────────────────────────────────────────────────────

func TestWriteAndReadLLMChannelsMarker(t *testing.T) {
	tmpDir := t.TempDir()

	channels := []string{
		"myapp/channels/chat",
		"myapp/channels/assistant",
	}

	err := WriteLLMChannelsMarker(tmpDir, channels)
	if err != nil {
		t.Fatalf("WriteLLMChannelsMarker failed: %v", err)
	}

	// Verify the file was created.
	markerPath := filepath.Join(tmpDir, ".shipq", "llm_channels.json")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Fatal("expected llm_channels.json to be created")
	}

	// Read it back.
	result, err := ReadLLMChannelsMarker(tmpDir)
	if err != nil {
		t.Fatalf("ReadLLMChannelsMarker failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 channels from marker, got %d: %v", len(result), result)
	}

	if result[0] != "myapp/channels/chat" {
		t.Errorf("expected first channel 'myapp/channels/chat', got %q", result[0])
	}
	if result[1] != "myapp/channels/assistant" {
		t.Errorf("expected second channel 'myapp/channels/assistant', got %q", result[1])
	}
}

func TestWriteAndReadLLMChannelsMarker_EmptyList(t *testing.T) {
	tmpDir := t.TempDir()

	err := WriteLLMChannelsMarker(tmpDir, []string{})
	if err != nil {
		t.Fatalf("WriteLLMChannelsMarker failed: %v", err)
	}

	result, err := ReadLLMChannelsMarker(tmpDir)
	if err != nil {
		t.Fatalf("ReadLLMChannelsMarker failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 channels from empty marker, got %d: %v", len(result), result)
	}
}

func TestWriteAndReadLLMChannelsMarker_NilList(t *testing.T) {
	tmpDir := t.TempDir()

	err := WriteLLMChannelsMarker(tmpDir, nil)
	if err != nil {
		t.Fatalf("WriteLLMChannelsMarker failed: %v", err)
	}

	result, err := ReadLLMChannelsMarker(tmpDir)
	if err != nil {
		t.Fatalf("ReadLLMChannelsMarker failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 channels from nil marker, got %d: %v", len(result), result)
	}
}

func TestReadLLMChannelsMarker_NoFile(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := ReadLLMChannelsMarker(tmpDir)
	if err != nil {
		t.Fatalf("ReadLLMChannelsMarker failed: %v", err)
	}

	if result != nil {
		t.Errorf("expected nil for missing marker file, got %v", result)
	}
}

func TestWriteLLMChannelsMarker_CreatesShipqDir(t *testing.T) {
	tmpDir := t.TempDir()

	// .shipq directory should not exist yet.
	shipqDir := filepath.Join(tmpDir, ".shipq")
	if _, err := os.Stat(shipqDir); !os.IsNotExist(err) {
		t.Fatal("expected .shipq directory to not exist initially")
	}

	err := WriteLLMChannelsMarker(tmpDir, []string{"myapp/channels/chat"})
	if err != nil {
		t.Fatalf("WriteLLMChannelsMarker failed: %v", err)
	}

	if _, err := os.Stat(shipqDir); os.IsNotExist(err) {
		t.Error("expected .shipq directory to be created")
	}
}

func TestWriteLLMChannelsMarker_SingleChannel(t *testing.T) {
	tmpDir := t.TempDir()

	channels := []string{"myapp/channels/solo"}

	err := WriteLLMChannelsMarker(tmpDir, channels)
	if err != nil {
		t.Fatalf("WriteLLMChannelsMarker failed: %v", err)
	}

	result, err := ReadLLMChannelsMarker(tmpDir)
	if err != nil {
		t.Fatalf("ReadLLMChannelsMarker failed: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 channel, got %d: %v", len(result), result)
	}
	if result[0] != "myapp/channels/solo" {
		t.Errorf("expected 'myapp/channels/solo', got %q", result[0])
	}
}

func TestWriteLLMChannelsMarker_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Write initial marker.
	err := WriteLLMChannelsMarker(tmpDir, []string{"myapp/channels/old"})
	if err != nil {
		t.Fatalf("first WriteLLMChannelsMarker failed: %v", err)
	}

	// Overwrite with new data.
	err = WriteLLMChannelsMarker(tmpDir, []string{"myapp/channels/new1", "myapp/channels/new2"})
	if err != nil {
		t.Fatalf("second WriteLLMChannelsMarker failed: %v", err)
	}

	result, err := ReadLLMChannelsMarker(tmpDir)
	if err != nil {
		t.Fatalf("ReadLLMChannelsMarker failed: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 channels after overwrite, got %d: %v", len(result), result)
	}
	if result[0] != "myapp/channels/new1" {
		t.Errorf("expected 'myapp/channels/new1', got %q", result[0])
	}
	if result[1] != "myapp/channels/new2" {
		t.Errorf("expected 'myapp/channels/new2', got %q", result[1])
	}
}

func TestWriteAndReadLLMChannelsMarker_ManyChannels(t *testing.T) {
	tmpDir := t.TempDir()

	channels := []string{
		"myapp/channels/a",
		"myapp/channels/b",
		"myapp/channels/c",
		"myapp/channels/d",
		"myapp/channels/e",
	}

	err := WriteLLMChannelsMarker(tmpDir, channels)
	if err != nil {
		t.Fatalf("WriteLLMChannelsMarker failed: %v", err)
	}

	result, err := ReadLLMChannelsMarker(tmpDir)
	if err != nil {
		t.Fatalf("ReadLLMChannelsMarker failed: %v", err)
	}

	if len(result) != 5 {
		t.Fatalf("expected 5 channels, got %d: %v", len(result), result)
	}

	for i, ch := range channels {
		if result[i] != ch {
			t.Errorf("channel %d: expected %q, got %q", i, ch, result[i])
		}
	}
}

func TestWriteLLMChannelsMarker_FileContent(t *testing.T) {
	tmpDir := t.TempDir()

	channels := []string{"myapp/channels/chat"}

	err := WriteLLMChannelsMarker(tmpDir, channels)
	if err != nil {
		t.Fatalf("WriteLLMChannelsMarker failed: %v", err)
	}

	markerPath := filepath.Join(tmpDir, ".shipq", "llm_channels.json")
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("failed to read marker file: %v", err)
	}

	content := string(data)

	// Should be valid JSON-ish (array of strings).
	if !strings.Contains(content, "[") {
		t.Error("expected '[' in marker file")
	}
	if !strings.Contains(content, "]") {
		t.Error("expected ']' in marker file")
	}
	if !strings.Contains(content, `"myapp/channels/chat"`) {
		t.Error("expected channel import path in marker file")
	}
}

func TestGenerateLLMStreamTypeScript_Deterministic(t *testing.T) {
	ts1 := GenerateLLMStreamTypeScript()
	ts2 := GenerateLLMStreamTypeScript()

	if ts1 != ts2 {
		t.Error("expected deterministic TypeScript output")
	}
}

func TestLLMFromServerUnionMembers_Deterministic(t *testing.T) {
	m1 := LLMFromServerUnionMembers()
	m2 := LLMFromServerUnionMembers()

	if len(m1) != len(m2) {
		t.Fatal("expected same number of union members")
	}

	for i := range m1 {
		if m1[i] != m2[i] {
			t.Errorf("member %d: expected %q, got %q", i, m1[i], m2[i])
		}
	}
}

func TestDetectLLMChannels_BlankImportNotDetected(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	channelPkg := modulePath + "/channels/blank"

	pkgDir := filepath.Join(tmpDir, "channels", "blank")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package blank

import (
	_ "myapp/shipq/lib/llm"
)

func DoStuff() {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "stuff.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write stuff.go: %v", err)
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 LLM channels for blank import, got %d", len(result))
	}
}

func TestDetectLLMChannels_DotImportNotDetected(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "myapp"
	channelPkg := modulePath + "/channels/dotimport"

	pkgDir := filepath.Join(tmpDir, "channels", "dotimport")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create package dir: %v", err)
	}

	content := `package dotimport

import (
	. "myapp/shipq/lib/llm"
)

func DoStuff() {}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "stuff.go"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write stuff.go: %v", err)
	}

	result, err := DetectLLMChannels(tmpDir, modulePath, []string{channelPkg})
	if err != nil {
		t.Fatalf("DetectLLMChannels failed: %v", err)
	}

	// Dot imports are not supported for detection — we skip them.
	if len(result) != 0 {
		t.Errorf("expected 0 LLM channels for dot import (not supported), got %d", len(result))
	}
}
