package channelcompile

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/codegen/embed"
)

// TestBuildAndRunChannelCompileProgram_EndToEnd tests the full channel compile
// pipeline: discovery -> program gen -> build -> run -> parse -> static analysis.
//
// It creates a temp project with go.mod, a channels/echo/register.go that
// defines a simple bidirectional channel, embeds the channel library, runs
// BuildAndRunChannelCompileProgram, and verifies the returned
// []SerializedChannelInfo has correct channel name, message types, and directions.
func TestBuildAndRunChannelCompileProgram_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp project directory
	tmpDir := t.TempDir()

	modulePath := "com.test-channel-compile"

	// Create go.mod
	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Embed the channel package into shipq/lib/channel (mirrors real project setup)
	if err := embed.EmbedAllPackages(tmpDir, modulePath); err != nil {
		t.Fatalf("failed to embed packages: %v", err)
	}

	// Create channels/echo directory
	channelDir := filepath.Join(tmpDir, "channels", "echo")
	if err := os.MkdirAll(channelDir, 0755); err != nil {
		t.Fatalf("failed to create channels/echo directory: %v", err)
	}

	channelImport := modulePath + "/shipq/lib/channel"

	// Create types.go with message types
	typesCode := `package echo

// EchoRequest is the dispatch message from the client.
type EchoRequest struct {
	Message string ` + "`json:\"message\"`" + `
}

// EchoReply is the response message from the server.
type EchoReply struct {
	Reply string ` + "`json:\"reply\"`" + `
}

// FollowUp is a mid-stream message from the client.
type FollowUp struct {
	Extra string ` + "`json:\"extra\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "types.go"), []byte(typesCode), 0644); err != nil {
		t.Fatalf("failed to create types.go: %v", err)
	}

	// Create handler.go with handler functions
	handlerCode := `package echo

import "context"

// HandleEchoRequest handles the dispatch message.
func HandleEchoRequest(ctx context.Context, req *EchoRequest) error {
	return nil
}

// HandleFollowUp handles the mid-stream follow-up message.
func HandleFollowUp(ctx context.Context, req *FollowUp) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "handler.go"), []byte(handlerCode), 0644); err != nil {
		t.Fatalf("failed to create handler.go: %v", err)
	}

	// Create register.go that registers the channel
	registerCode := `package echo

import "` + channelImport + `"

// Register registers the echo channel.
func Register(app *channel.App) {
	app.DefineChannel(
		"echo",
		channel.FromClient(EchoRequest{}, FollowUp{}),
		channel.FromServer(EchoReply{}),
	).Retries(3).TimeoutSeconds(30)
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "register.go"), []byte(registerCode), 0644); err != nil {
		t.Fatalf("failed to create register.go: %v", err)
	}

	// Run go mod tidy to resolve dependencies
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\noutput: %s", err, output)
	}

	// Run the channel compile pipeline
	// Note: BuildAndRunChannelCompileProgram does discovery internally,
	// so we pass shipqRoot == goModRoot for the standard layout.
	channels, err := BuildAndRunChannelCompileProgram(tmpDir, tmpDir, modulePath)
	if err != nil {
		t.Fatalf("BuildAndRunChannelCompileProgram failed: %v", err)
	}

	// Verify we got exactly one channel
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}

	ch := channels[0]

	// Verify channel name
	if ch.Name != "echo" {
		t.Errorf("expected channel name 'echo', got %q", ch.Name)
	}

	// Verify visibility (DefineChannel = frontend)
	if ch.Visibility != "frontend" {
		t.Errorf("expected visibility 'frontend', got %q", ch.Visibility)
	}

	// Verify retries and timeout
	if ch.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", ch.MaxRetries)
	}
	if ch.TimeoutSeconds != 30 {
		t.Errorf("expected TimeoutSeconds 30, got %d", ch.TimeoutSeconds)
	}

	// Verify PackagePath
	expectedPkgPath := modulePath + "/channels/echo"
	if ch.PackagePath != expectedPkgPath {
		t.Errorf("expected PackagePath %q, got %q", expectedPkgPath, ch.PackagePath)
	}

	// Verify PackageName
	if ch.PackageName != "echo" {
		t.Errorf("expected PackageName 'echo', got %q", ch.PackageName)
	}

	// Verify messages (should be 3: EchoRequest, FollowUp, EchoReply)
	if len(ch.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d: %+v", len(ch.Messages), ch.Messages)
	}

	// Message 0: EchoRequest (dispatch, client_to_server)
	msg0 := ch.Messages[0]
	if msg0.TypeName != "EchoRequest" {
		t.Errorf("msg[0]: expected TypeName 'EchoRequest', got %q", msg0.TypeName)
	}
	if msg0.Direction != "client_to_server" {
		t.Errorf("msg[0]: expected direction 'client_to_server', got %q", msg0.Direction)
	}
	if !msg0.IsDispatch {
		t.Error("msg[0]: expected IsDispatch=true")
	}
	// Static analysis should have filled in the handler name
	if msg0.HandlerName != "HandleEchoRequest" {
		t.Errorf("msg[0]: expected HandlerName 'HandleEchoRequest', got %q", msg0.HandlerName)
	}

	// Message 1: FollowUp (mid-stream, client_to_server)
	msg1 := ch.Messages[1]
	if msg1.TypeName != "FollowUp" {
		t.Errorf("msg[1]: expected TypeName 'FollowUp', got %q", msg1.TypeName)
	}
	if msg1.Direction != "client_to_server" {
		t.Errorf("msg[1]: expected direction 'client_to_server', got %q", msg1.Direction)
	}
	if msg1.IsDispatch {
		t.Error("msg[1]: expected IsDispatch=false for mid-stream type")
	}
	if msg1.HandlerName != "HandleFollowUp" {
		t.Errorf("msg[1]: expected HandlerName 'HandleFollowUp', got %q", msg1.HandlerName)
	}

	// Message 2: EchoReply (server_to_client)
	msg2 := ch.Messages[2]
	if msg2.TypeName != "EchoReply" {
		t.Errorf("msg[2]: expected TypeName 'EchoReply', got %q", msg2.TypeName)
	}
	if msg2.Direction != "server_to_client" {
		t.Errorf("msg[2]: expected direction 'server_to_client', got %q", msg2.Direction)
	}
	if msg2.IsDispatch {
		t.Error("msg[2]: expected IsDispatch=false for server_to_client type")
	}
	if msg2.HandlerName != "" {
		t.Errorf("msg[2]: expected empty HandlerName for server_to_client, got %q", msg2.HandlerName)
	}

	// Verify EchoRequest has fields
	if len(msg0.Fields) == 0 {
		t.Error("msg[0]: expected fields for EchoRequest, got none")
	} else {
		foundMessage := false
		for _, f := range msg0.Fields {
			if f.Name == "Message" {
				foundMessage = true
				if f.Type != "string" {
					t.Errorf("msg[0].Message: expected type 'string', got %q", f.Type)
				}
				if f.JSONName != "message" {
					t.Errorf("msg[0].Message: expected json name 'message', got %q", f.JSONName)
				}
			}
		}
		if !foundMessage {
			t.Error("msg[0]: expected to find 'Message' field")
		}
	}
}

// TestBuildAndRunChannelCompileProgram_NoChannels tests that the pipeline
// gracefully returns nil when no channel packages are found.
func TestBuildAndRunChannelCompileProgram_NoChannels(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	modulePath := "com.test-no-channels"

	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	channels, err := BuildAndRunChannelCompileProgram(tmpDir, tmpDir, modulePath)
	if err != nil {
		t.Fatalf("BuildAndRunChannelCompileProgram failed: %v", err)
	}

	if channels != nil {
		t.Errorf("expected nil channels for empty project, got %d channels", len(channels))
	}
}

// TestBuildAndRunChannelCompileProgram_PublicChannel tests that a public channel
// with rate limiting is properly captured by the compile pipeline.
func TestBuildAndRunChannelCompileProgram_PublicChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	modulePath := "com.test-public-channel"

	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	if err := embed.EmbedAllPackages(tmpDir, modulePath); err != nil {
		t.Fatalf("failed to embed packages: %v", err)
	}

	channelDir := filepath.Join(tmpDir, "channels", "public_chat")
	if err := os.MkdirAll(channelDir, 0755); err != nil {
		t.Fatalf("failed to create channels/public_chat directory: %v", err)
	}

	channelImport := modulePath + "/shipq/lib/channel"

	typesCode := `package public_chat

type ChatMsg struct {
	Text string ` + "`json:\"text\"`" + `
}

type ChatReply struct {
	Text string ` + "`json:\"text\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "types.go"), []byte(typesCode), 0644); err != nil {
		t.Fatalf("failed to create types.go: %v", err)
	}

	handlerCode := `package public_chat

import "context"

func HandleChatMsg(ctx context.Context, req *ChatMsg) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "handler.go"), []byte(handlerCode), 0644); err != nil {
		t.Fatalf("failed to create handler.go: %v", err)
	}

	registerCode := `package public_chat

import "` + channelImport + `"

func Register(app *channel.App) {
	app.DefineChannel(
		"public_chat",
		channel.FromClient(ChatMsg{}),
		channel.FromServer(ChatReply{}),
	).Public(channel.RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
	})
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "register.go"), []byte(registerCode), 0644); err != nil {
		t.Fatalf("failed to create register.go: %v", err)
	}

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\noutput: %s", err, output)
	}

	channels, err := BuildAndRunChannelCompileProgram(tmpDir, tmpDir, modulePath)
	if err != nil {
		t.Fatalf("BuildAndRunChannelCompileProgram failed: %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}

	ch := channels[0]

	if ch.Name != "public_chat" {
		t.Errorf("expected name 'public_chat', got %q", ch.Name)
	}
	if !ch.IsPublic {
		t.Error("expected IsPublic=true")
	}
	if ch.RateLimit == nil {
		t.Fatal("expected non-nil RateLimit")
	}
	if ch.RateLimit.RequestsPerMinute != 60 {
		t.Errorf("expected RequestsPerMinute 60, got %d", ch.RateLimit.RequestsPerMinute)
	}
	if ch.RateLimit.BurstSize != 10 {
		t.Errorf("expected BurstSize 10, got %d", ch.RateLimit.BurstSize)
	}

	// Verify handler name from static analysis
	if len(ch.Messages) < 1 {
		t.Fatal("expected at least 1 message")
	}
	if ch.Messages[0].HandlerName != "HandleChatMsg" {
		t.Errorf("expected HandlerName 'HandleChatMsg', got %q", ch.Messages[0].HandlerName)
	}
}

// TestBuildAndRunChannelCompileProgram_BackendChannel tests that a backend
// channel with a required role is properly captured.
func TestBuildAndRunChannelCompileProgram_BackendChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	modulePath := "com.test-backend-channel"

	goModContent := "module " + modulePath + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	if err := embed.EmbedAllPackages(tmpDir, modulePath); err != nil {
		t.Fatalf("failed to embed packages: %v", err)
	}

	channelDir := filepath.Join(tmpDir, "channels", "internal_sync")
	if err := os.MkdirAll(channelDir, 0755); err != nil {
		t.Fatalf("failed to create channels/internal_sync directory: %v", err)
	}

	channelImport := modulePath + "/shipq/lib/channel"

	typesCode := `package internal_sync

type SyncRequest struct {
	EntityID string ` + "`json:\"entity_id\"`" + `
}

type SyncProgress struct {
	Percent int ` + "`json:\"percent\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "types.go"), []byte(typesCode), 0644); err != nil {
		t.Fatalf("failed to create types.go: %v", err)
	}

	handlerCode := `package internal_sync

import "context"

func HandleSyncRequest(ctx context.Context, req *SyncRequest) error {
	return nil
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "handler.go"), []byte(handlerCode), 0644); err != nil {
		t.Fatalf("failed to create handler.go: %v", err)
	}

	registerCode := `package internal_sync

import "` + channelImport + `"

func Register(app *channel.App) {
	app.DefineBackendChannel(
		"internal_sync",
		channel.FromClient(SyncRequest{}),
		channel.FromServer(SyncProgress{}),
	).RequireRole("admin").Retries(5).BackoffSeconds(10)
}
`
	if err := os.WriteFile(filepath.Join(channelDir, "register.go"), []byte(registerCode), 0644); err != nil {
		t.Fatalf("failed to create register.go: %v", err)
	}

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\noutput: %s", err, output)
	}

	channels, err := BuildAndRunChannelCompileProgram(tmpDir, tmpDir, modulePath)
	if err != nil {
		t.Fatalf("BuildAndRunChannelCompileProgram failed: %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}

	ch := channels[0]

	if ch.Name != "internal_sync" {
		t.Errorf("expected name 'internal_sync', got %q", ch.Name)
	}
	if ch.Visibility != "backend" {
		t.Errorf("expected visibility 'backend', got %q", ch.Visibility)
	}
	if ch.RequiredRole != "admin" {
		t.Errorf("expected RequiredRole 'admin', got %q", ch.RequiredRole)
	}
	if ch.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", ch.MaxRetries)
	}
	if ch.BackoffSeconds != 10 {
		t.Errorf("expected BackoffSeconds 10, got %d", ch.BackoffSeconds)
	}
	if !ch.IsPublic == true {
		// DefineBackendChannel should not be public
	}
	if ch.IsPublic {
		t.Error("expected IsPublic=false for backend channel")
	}
}

// TestBuildAndRunChannelCompileProgram_ExampleChannelRegression is a regression
// test for a bug where `shipq workers` generated channels/example/register.go
// containing a HandleEchoRequest handler that called TypedChannelFromContext.
// That symbol lives in zz_generated_channel.go which is only produced AFTER
// channel compilation, so `go build` of the channel package failed with:
//
//	undefined: TypedChannelFromContext
//
// The fix splits the example into register.go (types + Register only) and
// handler.go (handler using TypedChannelFromContext), where handler.go is
// written only after typed channel generation.
//
// This test has two sub-tests:
//   - "handler_in_register" reproduces the old broken state to prove the
//     regression exists without the fix.
//   - "split_files" verifies the fixed layout compiles successfully.
func TestBuildAndRunChannelCompileProgram_ExampleChannelRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// --- Sub-test 1: the old broken layout (handler in register.go) ---
	t.Run("handler_in_register", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := "com.test-example-broken"

		goModContent := "module " + modulePath + "\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}

		if err := embed.EmbedAllPackages(tmpDir, modulePath); err != nil {
			t.Fatalf("failed to embed packages: %v", err)
		}

		channelDir := filepath.Join(tmpDir, "channels", "example")
		if err := os.MkdirAll(channelDir, 0755); err != nil {
			t.Fatalf("failed to create channels/example directory: %v", err)
		}

		channelImport := modulePath + "/shipq/lib/channel"

		// This is the OLD template: register.go includes the handler that
		// references TypedChannelFromContext, which doesn't exist yet.
		brokenCode := `package example

import (
	"context"

	"` + channelImport + `"
)

type EchoRequest struct {
	Message string ` + "`json:\"message\"`" + `
}

type EchoResponse struct {
	Echo string ` + "`json:\"echo\"`" + `
}

func Register(app *channel.App) {
	app.DefineChannel(
		"example",
		channel.FromClient(EchoRequest{}),
		channel.FromServer(EchoResponse{}),
	).Retries(3).TimeoutSeconds(30)
}

func HandleEchoRequest(ctx context.Context, req *EchoRequest) error {
	tc := TypedChannelFromContext(ctx)
	return tc.SendEchoResponse(ctx, &EchoResponse{
		Echo: "Echo: " + req.Message,
	})
}
`
		if err := os.WriteFile(filepath.Join(channelDir, "register.go"), []byte(brokenCode), 0644); err != nil {
			t.Fatalf("failed to write register.go: %v", err)
		}

		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = tmpDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go mod tidy failed: %v\noutput: %s", err, output)
		}

		// This MUST fail — the handler references TypedChannelFromContext
		// which doesn't exist yet. If it somehow passes, the regression
		// test is no longer meaningful.
		_, err := BuildAndRunChannelCompileProgram(tmpDir, tmpDir, modulePath)
		if err == nil {
			t.Fatal("expected compile failure when handler referencing " +
				"TypedChannelFromContext is in register.go, but got nil error")
		}
		t.Logf("confirmed regression: %v", err)
	})

	// --- Sub-test 2: the fixed layout (handler in separate file, written later) ---
	t.Run("split_files", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := "com.test-example-fixed"

		goModContent := "module " + modulePath + "\n\ngo 1.21\n"
		if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			t.Fatalf("failed to create go.mod: %v", err)
		}

		if err := embed.EmbedAllPackages(tmpDir, modulePath); err != nil {
			t.Fatalf("failed to embed packages: %v", err)
		}

		channelDir := filepath.Join(tmpDir, "channels", "example")
		if err := os.MkdirAll(channelDir, 0755); err != nil {
			t.Fatalf("failed to create channels/example directory: %v", err)
		}

		channelImport := modulePath + "/shipq/lib/channel"

		// This is the FIXED template: register.go has only types + Register.
		// No handler, no reference to TypedChannelFromContext.
		registerCode := `package example

import (
	"` + channelImport + `"
)

type EchoRequest struct {
	Message string ` + "`json:\"message\"`" + `
}

type EchoResponse struct {
	Echo string ` + "`json:\"echo\"`" + `
}

func Register(app *channel.App) {
	app.DefineChannel(
		"example",
		channel.FromClient(EchoRequest{}),
		channel.FromServer(EchoResponse{}),
	).Retries(3).TimeoutSeconds(30)
}
`
		if err := os.WriteFile(filepath.Join(channelDir, "register.go"), []byte(registerCode), 0644); err != nil {
			t.Fatalf("failed to write register.go: %v", err)
		}

		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = tmpDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go mod tidy failed: %v\noutput: %s", err, output)
		}

		// With the handler removed from register.go, channel compilation
		// must succeed because there is no reference to the not-yet-generated
		// TypedChannelFromContext.
		channels, err := BuildAndRunChannelCompileProgram(tmpDir, tmpDir, modulePath)
		if err != nil {
			t.Fatalf("BuildAndRunChannelCompileProgram failed on fixed layout: %v", err)
		}

		if len(channels) != 1 {
			t.Fatalf("expected 1 channel, got %d", len(channels))
		}

		ch := channels[0]
		if ch.Name != "example" {
			t.Errorf("expected channel name 'example', got %q", ch.Name)
		}
		if len(ch.Messages) != 2 {
			t.Fatalf("expected 2 messages (EchoRequest + EchoResponse), got %d", len(ch.Messages))
		}
	})
}
