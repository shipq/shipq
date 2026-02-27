package channelgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shipq/shipq/codegen"
)

// TestGenerateAllTypedChannels_MonorepoLayout verifies that in a monorepo
// layout (go.mod at a parent directory, shipq.ini in a subdirectory), the
// generated zz_generated_channel.go files are written relative to shipqRoot
// (the directory containing shipq.ini), NOT relative to goModRoot (the
// directory containing go.mod).
//
// This is a regression test for a bug where the generated files were placed
// at the go.mod root instead of under the shipq project subdirectory.
func TestGenerateAllTypedChannels_MonorepoLayout(t *testing.T) {
	// Set up a monorepo-like directory structure:
	//   <tmp>/                     ← goModRoot (contains go.mod)
	//   <tmp>/services/myservice/  ← shipqRoot (contains shipq.ini)
	//   channels live under shipqRoot: services/myservice/channels/echo/
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "services", "myservice")
	if err := os.MkdirAll(shipqRoot, 0755); err != nil {
		t.Fatalf("failed to create shipqRoot: %v", err)
	}

	// The channel package directory must exist for EnsureDir to work.
	channelDir := filepath.Join(shipqRoot, "channels", "echo")
	if err := os.MkdirAll(channelDir, 0755); err != nil {
		t.Fatalf("failed to create channel dir: %v", err)
	}

	modulePath := "github.com/company/monorepo"
	// importPrefix includes the subpath: github.com/company/monorepo/services/myservice
	importPrefix := modulePath + "/services/myservice"

	// Build a minimal channel definition. PackagePath is the full import path
	// to the channel package, as set by the compile program.
	channels := []codegen.SerializedChannelInfo{
		{
			Name:        "echo",
			Visibility:  "frontend",
			PackagePath: importPrefix + "/channels/echo",
			PackageName: "echo",
			Messages: []codegen.SerializedMessageInfo{
				{
					Direction:   "client_to_server",
					TypeName:    "EchoRequest",
					IsDispatch:  true,
					HandlerName: "HandleEchoRequest",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Message", Type: "string", JSONName: "message"},
					},
				},
				{
					Direction:  "server_to_client",
					TypeName:   "EchoResponse",
					IsDispatch: false,
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Reply", Type: "string", JSONName: "reply"},
					},
				},
			},
		},
	}

	// Run the generator
	err := GenerateAllTypedChannels(channels, goModRoot, shipqRoot, importPrefix)
	if err != nil {
		t.Fatalf("GenerateAllTypedChannels failed: %v", err)
	}

	// The generated file MUST be under shipqRoot, NOT goModRoot.
	correctPath := filepath.Join(shipqRoot, "channels", "echo", "zz_generated_channel.go")
	if _, err := os.Stat(correctPath); os.IsNotExist(err) {
		t.Errorf("expected generated file at %s but it does not exist", correctPath)
	}

	// The file must NOT exist at the wrong location (goModRoot/channels/echo/).
	wrongPath := filepath.Join(goModRoot, "channels", "echo", "zz_generated_channel.go")
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("generated file was placed at WRONG path %s (go.mod root) instead of under shipq root", wrongPath)
	}
}

// TestGenerateAllTypedChannels_StandardLayout verifies that in a standard
// (non-monorepo) layout where goModRoot == shipqRoot, files are written
// correctly.
func TestGenerateAllTypedChannels_StandardLayout(t *testing.T) {
	projectRoot := t.TempDir()

	channelDir := filepath.Join(projectRoot, "channels", "chat")
	if err := os.MkdirAll(channelDir, 0755); err != nil {
		t.Fatalf("failed to create channel dir: %v", err)
	}

	modulePath := "myapp"

	channels := []codegen.SerializedChannelInfo{
		{
			Name:        "chat",
			Visibility:  "frontend",
			PackagePath: modulePath + "/channels/chat",
			PackageName: "chat",
			Messages: []codegen.SerializedMessageInfo{
				{
					Direction:   "client_to_server",
					TypeName:    "ChatMessage",
					IsDispatch:  true,
					HandlerName: "HandleChatMessage",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Text", Type: "string", JSONName: "text"},
					},
				},
				{
					Direction:  "server_to_client",
					TypeName:   "ChatReply",
					IsDispatch: false,
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Reply", Type: "string", JSONName: "reply"},
					},
				},
			},
		},
	}

	// goModRoot == shipqRoot in standard layout
	err := GenerateAllTypedChannels(channels, projectRoot, projectRoot, modulePath)
	if err != nil {
		t.Fatalf("GenerateAllTypedChannels failed: %v", err)
	}

	expectedPath := filepath.Join(projectRoot, "channels", "chat", "zz_generated_channel.go")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected generated file at %s but it does not exist", expectedPath)
	}
}

// TestGenerateAllTypedChannels_MonorepoMultipleChannels verifies correct
// placement of generated files when multiple channels exist in a monorepo.
func TestGenerateAllTypedChannels_MonorepoMultipleChannels(t *testing.T) {
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "apps", "backend")
	modulePath := "github.com/org/repo"
	importPrefix := modulePath + "/apps/backend"

	channelNames := []string{"email", "notifications", "chatbot"}

	for _, name := range channelNames {
		dir := filepath.Join(shipqRoot, "channels", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create channel dir for %s: %v", name, err)
		}
	}

	var channels []codegen.SerializedChannelInfo
	for _, name := range channelNames {
		channels = append(channels, codegen.SerializedChannelInfo{
			Name:        name,
			Visibility:  "frontend",
			PackagePath: importPrefix + "/channels/" + name,
			PackageName: name,
			Messages: []codegen.SerializedMessageInfo{
				{
					Direction:   "client_to_server",
					TypeName:    "Request",
					IsDispatch:  true,
					HandlerName: "HandleRequest",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Data", Type: "string", JSONName: "data"},
					},
				},
				{
					Direction:  "server_to_client",
					TypeName:   "Response",
					IsDispatch: false,
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Result", Type: "string", JSONName: "result"},
					},
				},
			},
		})
	}

	err := GenerateAllTypedChannels(channels, goModRoot, shipqRoot, importPrefix)
	if err != nil {
		t.Fatalf("GenerateAllTypedChannels failed: %v", err)
	}

	for _, name := range channelNames {
		correctPath := filepath.Join(shipqRoot, "channels", name, "zz_generated_channel.go")
		if _, err := os.Stat(correctPath); os.IsNotExist(err) {
			t.Errorf("channel %q: expected generated file at %s but it does not exist", name, correctPath)
		}

		wrongPath := filepath.Join(goModRoot, "channels", name, "zz_generated_channel.go")
		if _, err := os.Stat(wrongPath); err == nil {
			t.Errorf("channel %q: generated file at WRONG path %s (go.mod root)", name, wrongPath)
		}
	}
}

// TestGenerateAllTypedChannels_MonorepoDeeplyNested verifies correct behavior
// when the shipq root is several levels deep inside the monorepo.
func TestGenerateAllTypedChannels_MonorepoDeeplyNested(t *testing.T) {
	goModRoot := t.TempDir()
	shipqRoot := filepath.Join(goModRoot, "packages", "services", "api", "backend")
	modulePath := "github.com/bigcorp/platform"
	importPrefix := modulePath + "/packages/services/api/backend"

	channelDir := filepath.Join(shipqRoot, "channels", "events")
	if err := os.MkdirAll(channelDir, 0755); err != nil {
		t.Fatalf("failed to create channel dir: %v", err)
	}

	channels := []codegen.SerializedChannelInfo{
		{
			Name:        "events",
			Visibility:  "backend",
			PackagePath: importPrefix + "/channels/events",
			PackageName: "events",
			Messages: []codegen.SerializedMessageInfo{
				{
					Direction:   "client_to_server",
					TypeName:    "EventPayload",
					IsDispatch:  true,
					HandlerName: "HandleEventPayload",
					Fields: []codegen.SerializedFieldInfo{
						{Name: "Kind", Type: "string", JSONName: "kind"},
					},
				},
				{
					Direction:  "server_to_client",
					TypeName:   "EventAck",
					IsDispatch: false,
					Fields: []codegen.SerializedFieldInfo{
						{Name: "OK", Type: "bool", JSONName: "ok"},
					},
				},
			},
		},
	}

	err := GenerateAllTypedChannels(channels, goModRoot, shipqRoot, importPrefix)
	if err != nil {
		t.Fatalf("GenerateAllTypedChannels failed: %v", err)
	}

	correctPath := filepath.Join(shipqRoot, "channels", "events", "zz_generated_channel.go")
	if _, err := os.Stat(correctPath); os.IsNotExist(err) {
		t.Errorf("expected generated file at %s but it does not exist", correctPath)
	}

	// Ensure no file was created relative to goModRoot
	wrongPath := filepath.Join(goModRoot, "channels", "events", "zz_generated_channel.go")
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("generated file at WRONG path %s", wrongPath)
	}
}
