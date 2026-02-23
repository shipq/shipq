package channelgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestGenerateE2ETest_HasBuildTag(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// The very first line must be the build tag
	lines := strings.Split(codeStr, "\n")
	if len(lines) == 0 {
		t.Fatal("generated code is empty")
	}
	if lines[0] != "//go:build e2e" {
		t.Errorf("expected first line to be '//go:build e2e', got %q", lines[0])
	}
}

func TestGenerateE2ETest_SkipsGracefully(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should have t.Skipf for Redis
	if !strings.Contains(codeStr, "t.Skipf(\"Redis not reachable") {
		t.Error("expected t.Skipf for Redis not reachable")
	}

	// Should have t.Skipf for Centrifugo
	if !strings.Contains(codeStr, "t.Skipf(\"Centrifugo not reachable") {
		t.Error("expected t.Skipf for Centrifugo not reachable")
	}
}

func TestGenerateE2ETest_BidirectionalEchoHandling(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeBidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// [L1]: Must have echo filtering logic — isFromClientType
	if !strings.Contains(codeStr, "isFromClientType") {
		t.Error("expected isFromClientType helper for echo filtering")
	}

	// The isFromClientType function should include the FromClient types
	if !strings.Contains(codeStr, `"StartChat"`) {
		t.Error("expected StartChat in isFromClientType switch")
	}
	if !strings.Contains(codeStr, `"ToolCallApproval"`) {
		t.Error("expected ToolCallApproval in isFromClientType switch")
	}

	// Should have sub.Publish for sending mid-stream messages
	if !strings.Contains(codeStr, "sub.Publish(") {
		t.Error("expected sub.Publish call for bidirectional mid-stream messages")
	}

	// [L1]: Should have echo skip comments
	if !strings.Contains(codeStr, "Skip echoed FromClient messages") || !strings.Contains(codeStr, "[L1]") {
		t.Error("expected [L1] echo filtering comments")
	}
}

func TestGenerateE2ETest_NonBlockingOnPublication(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// [L6]: Should use a buffered channel with capacity >= 64
	if !strings.Contains(codeStr, "make(chan []byte, 128)") {
		t.Error("expected buffered channel with capacity 128 in connectAndSubscribe")
	}

	// [L6]: Should have non-blocking send pattern (select with default)
	if !strings.Contains(codeStr, "case msgCh <- e.Data:") {
		t.Error("expected non-blocking send to msgCh in OnPublication")
	}
	if !strings.Contains(codeStr, "default:") {
		t.Error("expected default case in select for non-blocking send")
	}

	// Should reference [L6] in comments
	if !strings.Contains(codeStr, "[L6]") {
		t.Error("expected [L6] annotation about non-blocking OnPublication")
	}
}

func TestGenerateE2ETest_UsesApiInfoForHealthCheck(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// [L8]: Must use POST /api/info, NOT /health
	if !strings.Contains(codeStr, `"/info"`) {
		t.Error("expected /info path in pingCentrifugo")
	}

	// Must NOT use /health as an actual endpoint path (comments mentioning it are OK)
	if strings.Contains(codeStr, `"/health"`) || strings.Contains(codeStr, `+ "/health"`) {
		t.Error("must NOT use /health endpoint — Centrifugo v6 has no /health")
	}

	// Should use POST method
	if !strings.Contains(codeStr, `"POST"`) {
		t.Error("expected POST method in pingCentrifugo")
	}

	// Should set X-API-Key header
	if !strings.Contains(codeStr, `"X-API-Key"`) {
		t.Error("expected X-API-Key header in pingCentrifugo")
	}

	// Should reference [L8] in comments
	if !strings.Contains(codeStr, "[L8]") {
		t.Error("expected [L8] annotation about Centrifugo health check")
	}
}

func TestGenerateE2ETest_UsesClientClose(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// [L7]: Must use client.Close() for cleanup, not Disconnect()
	if !strings.Contains(codeStr, "client.Close()") {
		t.Error("expected client.Close() for cleanup")
	}

	// Must NOT use Disconnect()
	if strings.Contains(codeStr, "client.Disconnect()") {
		t.Error("must NOT use client.Disconnect() — use Close() for terminal state")
	}

	// Should reference [L7] in comments
	if !strings.Contains(codeStr, "[L7]") {
		t.Error("expected [L7] annotation about Close vs Disconnect")
	}
}

func TestGenerateE2ETest_ValidGo(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
		makeBidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "zz_generated_e2e_test.go", code, parser.AllErrors)
	if err != nil {
		t.Errorf("generated e2e test is not valid Go: %v\ncode:\n%s", err, string(code))
	}
}

func TestGenerateE2ETest_EmptyChannels(t *testing.T) {
	code, err := GenerateE2ETestCode(nil, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// Even empty channels should have the build tag
	if !strings.Contains(codeStr, "//go:build e2e") {
		t.Error("expected '//go:build e2e' even for empty channels")
	}

	if !strings.Contains(codeStr, "package spec") {
		t.Error("expected 'package spec' even for empty channels")
	}
}

func TestGenerateE2ETest_PingRedisUsesRawTCP(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should use raw TCP dial for Redis ping
	if !strings.Contains(codeStr, "net.DialTimeout") {
		t.Error("expected net.DialTimeout in pingRedis")
	}

	// Should send PING and expect PONG
	if !strings.Contains(codeStr, `PING\r\n`) {
		t.Error("expected PING command in pingRedis")
	}
	if !strings.Contains(codeStr, `+PONG\r\n`) {
		t.Error("expected +PONG response check in pingRedis")
	}
}

func TestGenerateE2ETest_ConnectAndSubscribe(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should use centrifuge.NewJsonClient
	if !strings.Contains(codeStr, "centrifuge.NewJsonClient") {
		t.Error("expected centrifuge.NewJsonClient in connectAndSubscribe")
	}

	// Should set up GetToken callback for subscription
	if !strings.Contains(codeStr, "GetToken") {
		t.Error("expected GetToken callback in subscription config")
	}

	// Should call sub.Subscribe()
	if !strings.Contains(codeStr, "sub.Subscribe()") {
		t.Error("expected sub.Subscribe() call")
	}

	// Should set up OnPublication handler
	if !strings.Contains(codeStr, "sub.OnPublication") {
		t.Error("expected sub.OnPublication handler")
	}
}

func TestGenerateE2ETest_MainTestFunction(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
		makeBidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should have the main test function
	if !strings.Contains(codeStr, "func TestE2E_WorkerPipeline(t *testing.T)") {
		t.Error("expected TestE2E_WorkerPipeline function")
	}

	// Should have subtests for each channel
	if !strings.Contains(codeStr, `t.Run("email"`) {
		t.Error("expected t.Run subtest for email channel")
	}
	if !strings.Contains(codeStr, `t.Run("chatbot"`) {
		t.Error("expected t.Run subtest for chatbot channel")
	}
}

func TestGenerateE2ETest_DispatchAndTokenHelpers(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should have dispatchJob helper
	if !strings.Contains(codeStr, "func dispatchJob(") {
		t.Error("expected dispatchJob helper function")
	}

	// Should have getTokens helper
	if !strings.Contains(codeStr, "func getTokens(") {
		t.Error("expected getTokens helper function")
	}

	// dispatchJob should POST to /channels/<name>/dispatch
	if !strings.Contains(codeStr, "/channels/%s/dispatch") {
		t.Error("expected dispatch URL pattern in dispatchJob")
	}

	// getTokens should GET from /channels/<name>/tokens
	if !strings.Contains(codeStr, "/channels/%s/tokens") {
		t.Error("expected tokens URL pattern in getTokens")
	}
}

func TestGenerateE2ETest_UnidirectionalCollectsMessages(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should verify EmailProgress message was received
	if !strings.Contains(codeStr, `"EmailProgress"`) {
		t.Error("expected EmailProgress type assertion in E2E test")
	}

	// Should have a timeout for collecting messages
	if !strings.Contains(codeStr, "30 * time.Second") {
		t.Error("expected 30-second timeout for message collection")
	}
}

func TestGenerateE2ETest_BidirectionalSendsClientMessages(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeBidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should create ToolCallApproval message for mid-stream send
	if !strings.Contains(codeStr, "chatbot.ToolCallApproval{") {
		t.Error("expected chatbot.ToolCallApproval struct literal in bidirectional test")
	}

	// Should use sub.Publish to send mid-stream client messages
	if !strings.Contains(codeStr, "sub.Publish(context.Background()") {
		t.Error("expected sub.Publish(context.Background(), ...) for mid-stream messages")
	}

	// Should wrap in envelope before publishing
	if !strings.Contains(codeStr, "e2eEnvelope{Type:") {
		t.Error("expected envelope wrapping for published messages")
	}
}

func TestGenerateE2ETest_ImportsConfigPackage(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	// Should import config for Settings
	if !strings.Contains(codeStr, `"myapp/config"`) {
		t.Error("expected config package import")
	}

	// Should reference config.Settings for REDIS_URL
	if !strings.Contains(codeStr, "config.Settings.REDIS_URL") {
		t.Error("expected config.Settings.REDIS_URL reference")
	}

	// Should reference config.Settings for CENTRIFUGO_API_URL
	if !strings.Contains(codeStr, "config.Settings.CENTRIFUGO_API_URL") {
		t.Error("expected config.Settings.CENTRIFUGO_API_URL reference")
	}
}

func TestGenerateE2ETest_CodeGenHeader(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeUnidirectionalChannel(),
	}

	code, err := GenerateE2ETestCode(channels, "myapp")
	if err != nil {
		t.Fatalf("GenerateE2ETestCode() error = %v", err)
	}
	codeStr := string(code)

	if !strings.Contains(codeStr, "Code generated by shipq. DO NOT EDIT.") {
		t.Error("expected code generation header comment")
	}
}
