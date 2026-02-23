package channelgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestGenerateWorkerMain_SingleChannel(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "chatbot",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				Messages: []codegen.SerializedMessageInfo{
					{
						Direction:   "client_to_server",
						TypeName:    "ChatRequest",
						IsDispatch:  true,
						HandlerName: "HandleChatRequest",
					},
					{
						Direction: "server_to_client",
						TypeName:  "ChatResponse",
					},
				},
				MaxRetries:     3,
				BackoffSeconds: 5,
			},
		},
		ModulePath:           "example.com/myapp",
		DBDialect:            "postgres",
		RedisAddr:            "localhost:6379",
		CentrifugoAPIURL:     "http://localhost:8100/api",
		CentrifugoAPIKey:     "test-api-key",
		CentrifugoHMACSecret: "test-hmac-secret",
		CentrifugoWSURL:      "ws://localhost:8100/connection/websocket",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	// Should have package main
	if !strings.Contains(codeStr, "package main") {
		t.Error("expected 'package main' in generated code")
	}

	// Should import channel package
	if !strings.Contains(codeStr, `"example.com/myapp/channels/chatbot"`) {
		t.Error("expected chatbot channel package import")
	}

	// Should import channel runtime library
	if !strings.Contains(codeStr, `"example.com/myapp/shipq/lib/channel"`) {
		t.Error("expected channel runtime library import")
	}

	// Should import config package
	if !strings.Contains(codeStr, `"example.com/myapp/config"`) {
		t.Error("expected config package import")
	}

	// Should register the task
	if !strings.Contains(codeStr, `queue.RegisterTask("chatbot"`) {
		t.Error("expected RegisterTask call for chatbot")
	}

	// Should use WrapDispatchHandler
	if !strings.Contains(codeStr, "channel.WrapDispatchHandler") {
		t.Error("expected WrapDispatchHandler call")
	}

	// Should reference the handler function
	if !strings.Contains(codeStr, "chatbot.HandleChatRequest") {
		t.Error("expected chatbot.HandleChatRequest reference")
	}

	// Should create CentrifugoTransport
	if !strings.Contains(codeStr, "channel.NewCentrifugoTransport") {
		t.Error("expected NewCentrifugoTransport call")
	}

	// Should create MachineryQueue
	if !strings.Contains(codeStr, "channel.NewMachineryQueue") {
		t.Error("expected NewMachineryQueue call")
	}

	// Should use config.Settings for Centrifugo values
	if !strings.Contains(codeStr, "config.Settings.CENTRIFUGO_API_URL") {
		t.Error("expected config.Settings.CENTRIFUGO_API_URL reference")
	}

	// Should start the worker
	if !strings.Contains(codeStr, "queue.StartWorker") {
		t.Error("expected StartWorker call")
	}

	// Should use signal.NotifyContext for graceful shutdown
	if !strings.Contains(codeStr, "signal.NotifyContext") {
		t.Error("expected signal.NotifyContext for graceful shutdown")
	}
}

func TestGenerateWorkerMain_MultiChannel(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "chatbot",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
					{Direction: "server_to_client", TypeName: "ChatResponse"},
				},
			},
			{
				Name:        "email",
				PackagePath: "example.com/myapp/channels/email",
				PackageName: "email",
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "EmailRequest", IsDispatch: true, HandlerName: "HandleEmailRequest"},
					{Direction: "server_to_client", TypeName: "EmailStatus"},
				},
			},
			{
				Name:        "summarizer",
				PackagePath: "example.com/myapp/channels/summarizer",
				PackageName: "summarizer",
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "SummarizeRequest", IsDispatch: true, HandlerName: "HandleSummarizeRequest"},
					{Direction: "server_to_client", TypeName: "SummaryResult"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	// All three channels should be registered
	if !strings.Contains(codeStr, `queue.RegisterTask("chatbot"`) {
		t.Error("expected RegisterTask call for chatbot")
	}
	if !strings.Contains(codeStr, `queue.RegisterTask("email"`) {
		t.Error("expected RegisterTask call for email")
	}
	if !strings.Contains(codeStr, `queue.RegisterTask("summarizer"`) {
		t.Error("expected RegisterTask call for summarizer")
	}

	// All three package imports should be present
	if !strings.Contains(codeStr, `"example.com/myapp/channels/chatbot"`) {
		t.Error("expected chatbot package import")
	}
	if !strings.Contains(codeStr, `"example.com/myapp/channels/email"`) {
		t.Error("expected email package import")
	}
	if !strings.Contains(codeStr, `"example.com/myapp/channels/summarizer"`) {
		t.Error("expected summarizer package import")
	}

	// All three handler references
	if !strings.Contains(codeStr, "chatbot.HandleChatRequest") {
		t.Error("expected chatbot.HandleChatRequest reference")
	}
	if !strings.Contains(codeStr, "email.HandleEmailRequest") {
		t.Error("expected email.HandleEmailRequest reference")
	}
	if !strings.Contains(codeStr, "summarizer.HandleSummarizeRequest") {
		t.Error("expected summarizer.HandleSummarizeRequest reference")
	}
}

func TestGenerateWorkerMain_ValidGo(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "chatbot",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
					{Direction: "server_to_client", TypeName: "ChatResponse"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "main.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated worker main is not valid Go: %v\ncode:\n%s", err, string(code))
	}
}

func TestGenerateWorkerMain_DialectImports(t *testing.T) {
	dialects := []struct {
		dialect        string
		expectedImport string
	}{
		{"postgres", "github.com/jackc/pgx/v5/stdlib"},
		{"mysql", "github.com/go-sql-driver/mysql"},
		{"sqlite", "modernc.org/sqlite"},
	}

	for _, tt := range dialects {
		t.Run(tt.dialect, func(t *testing.T) {
			cfg := WorkerGenConfig{
				Channels: []codegen.SerializedChannelInfo{
					{
						Name:        "chatbot",
						PackagePath: "example.com/myapp/channels/chatbot",
						PackageName: "chatbot",
						Messages: []codegen.SerializedMessageInfo{
							{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
						},
					},
				},
				ModulePath: "example.com/myapp",
				DBDialect:  tt.dialect,
			}

			code, err := GenerateWorkerMain(cfg)
			if err != nil {
				t.Fatalf("GenerateWorkerMain() error = %v", err)
			}
			codeStr := string(code)

			if !strings.Contains(codeStr, tt.expectedImport) {
				t.Errorf("expected driver import %q for dialect %q", tt.expectedImport, tt.dialect)
			}
		})
	}
}

func TestGenerateWorkerMain_GeneratedComment(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "chatbot",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	if !strings.Contains(codeStr, "Code generated by shipq") {
		t.Error("expected generated file comment")
	}
}

func TestGenerateWorkerMain_InterfaceWiring(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "chatbot",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	// Should declare transport as RealtimeTransport interface
	if !strings.Contains(codeStr, "var transport channel.RealtimeTransport") {
		t.Error("expected 'var transport channel.RealtimeTransport' declaration")
	}

	// Should have comment about being the only coupling point
	if !strings.Contains(codeStr, "ONLY coupling") {
		t.Error("expected comment about interface coupling")
	}
}

func TestGenerateWorkerMain_DBConnection(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "chatbot",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	// Should use config.ParseDatabaseURL
	if !strings.Contains(codeStr, "config.ParseDatabaseURL") {
		t.Error("expected config.ParseDatabaseURL call")
	}

	// Should use sql.Open
	if !strings.Contains(codeStr, "sql.Open") {
		t.Error("expected sql.Open call")
	}

	// Should ping the DB
	if !strings.Contains(codeStr, "db.Ping()") {
		t.Error("expected db.Ping() call")
	}

	// Should defer db.Close()
	if !strings.Contains(codeStr, "defer db.Close()") {
		t.Error("expected defer db.Close()")
	}
}

func TestGenerateWorkerMain_NoDispatchMessage_SkipsChannel(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "chatbot",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				Messages: []codegen.SerializedMessageInfo{
					// No dispatch message (IsDispatch is false for all)
					{Direction: "server_to_client", TypeName: "ChatResponse"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	// Should NOT register a task for a channel without a dispatch message
	if strings.Contains(codeStr, `queue.RegisterTask("chatbot"`) {
		t.Error("expected no RegisterTask call for channel without dispatch message")
	}
}

func TestGenerateWorkerMain_WithSetup_EmitsWithSetupOption(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "llmchat",
				PackagePath: "example.com/myapp/channels/llmchat",
				PackageName: "llmchat",
				HasSetup:    true,
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
					{Direction: "server_to_client", TypeName: "ChatResponse"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	// Should include channel.WithSetup(llmchat.Setup) in the RegisterTask call
	if !strings.Contains(codeStr, "channel.WithSetup(llmchat.Setup)") {
		t.Error("expected channel.WithSetup(llmchat.Setup) option when HasSetup is true")
	}
}

func TestGenerateWorkerMain_WithSetup_ValidGo(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "llmchat",
				PackagePath: "example.com/myapp/channels/llmchat",
				PackageName: "llmchat",
				HasSetup:    true,
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
					{Direction: "server_to_client", TypeName: "ChatResponse"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "main.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated worker main with HasSetup is not valid Go: %v\ncode:\n%s", err, string(code))
	}
}

func TestGenerateWorkerMain_WithoutSetup_NoWithSetupOption(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "chatbot",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				HasSetup:    false,
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
					{Direction: "server_to_client", TypeName: "ChatResponse"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	// Should NOT include WithSetup when HasSetup is false
	if strings.Contains(codeStr, "WithSetup") {
		t.Error("expected no WithSetup option when HasSetup is false")
	}
}

func TestGenerateWorkerMain_MixedSetup_OnlyAppliesWherePresent(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "llmchat",
				PackagePath: "example.com/myapp/channels/llmchat",
				PackageName: "llmchat",
				HasSetup:    true,
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
					{Direction: "server_to_client", TypeName: "ChatResponse"},
				},
			},
			{
				Name:        "email",
				PackagePath: "example.com/myapp/channels/email",
				PackageName: "email",
				HasSetup:    false,
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "EmailRequest", IsDispatch: true, HandlerName: "HandleEmailRequest"},
					{Direction: "server_to_client", TypeName: "EmailStatus"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	// llmchat should have WithSetup
	if !strings.Contains(codeStr, "channel.WithSetup(llmchat.Setup)") {
		t.Error("expected channel.WithSetup(llmchat.Setup) for llmchat channel")
	}

	// email should NOT have WithSetup
	if strings.Contains(codeStr, "channel.WithSetup(email.Setup)") {
		t.Error("expected no WithSetup for email channel (HasSetup is false)")
	}

	// Both tasks should be registered
	if !strings.Contains(codeStr, `queue.RegisterTask("llmchat"`) {
		t.Error("expected RegisterTask for llmchat")
	}
	if !strings.Contains(codeStr, `queue.RegisterTask("email"`) {
		t.Error("expected RegisterTask for email")
	}
}

func TestGenerateWorkerMain_DuplicatePackagePaths_ImportedOnce(t *testing.T) {
	cfg := WorkerGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			{
				Name:        "chatbot",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequest", IsDispatch: true, HandlerName: "HandleChatRequest"},
				},
			},
			{
				Name:        "chatbot_v2",
				PackagePath: "example.com/myapp/channels/chatbot",
				PackageName: "chatbot",
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "ChatRequestV2", IsDispatch: true, HandlerName: "HandleChatRequestV2"},
				},
			},
		},
		ModulePath: "example.com/myapp",
		DBDialect:  "postgres",
	}

	code, err := GenerateWorkerMain(cfg)
	if err != nil {
		t.Fatalf("GenerateWorkerMain() error = %v", err)
	}
	codeStr := string(code)

	// The chatbot package import should appear exactly once
	count := strings.Count(codeStr, `"example.com/myapp/channels/chatbot"`)
	if count != 1 {
		t.Errorf("expected exactly 1 import of chatbot package, got %d", count)
	}
}
