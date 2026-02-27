package channelgen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func makeAuthChannel(name string) codegen.SerializedChannelInfo {
	return codegen.SerializedChannelInfo{
		Name:        name,
		Visibility:  "frontend",
		IsPublic:    false,
		PackagePath: "example.com/myapp/channels/" + name,
		PackageName: name,
		Messages: []codegen.SerializedMessageInfo{
			{
				Direction:   "client_to_server",
				TypeName:    strings.Title(name) + "Request",
				IsDispatch:  true,
				HandlerName: name + "Handler",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Prompt", Type: "string", JSONName: "prompt", Required: true},
				},
			},
			{
				Direction: "server_to_client",
				TypeName:  strings.Title(name) + "Response",
			},
		},
		MaxRetries:     3,
		BackoffSeconds: 5,
	}
}

func makePublicChannel(name string, rpm, burst int) codegen.SerializedChannelInfo {
	return codegen.SerializedChannelInfo{
		Name:        name,
		Visibility:  "frontend",
		IsPublic:    true,
		PackagePath: "example.com/myapp/channels/" + name,
		PackageName: name,
		RateLimit: &codegen.SerializedRateLimitConfig{
			RequestsPerMinute: rpm,
			BurstSize:         burst,
		},
		Messages: []codegen.SerializedMessageInfo{
			{
				Direction:   "client_to_server",
				TypeName:    strings.Title(name) + "Request",
				IsDispatch:  true,
				HandlerName: name + "Handler",
				Fields: []codegen.SerializedFieldInfo{
					{Name: "Input", Type: "string", JSONName: "input", Required: true},
				},
			},
			{
				Direction: "server_to_client",
				TypeName:  strings.Title(name) + "Response",
			},
		},
	}
}

func makeRBACChannel(name, role string) codegen.SerializedChannelInfo {
	ch := makeAuthChannel(name)
	ch.RequiredRole = role
	return ch
}

func makeBackendChannel(name string) codegen.SerializedChannelInfo {
	ch := makeAuthChannel(name)
	ch.Visibility = "backend"
	return ch
}

func generateAndCheck(t *testing.T, channels []codegen.SerializedChannelInfo) string {
	t.Helper()
	cfg := ChannelHTTPGenConfig{
		Channels:   channels,
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
	}

	files, err := GenerateChannelHTTPRoutes(cfg)
	if err != nil {
		t.Fatalf("GenerateChannelHTTPRoutes() error = %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least one generated file")
	}

	return string(files[0].Content)
}

func TestGenerateDispatchRoute_AuthRequired(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Should have auth check in the dispatch handler
	if !strings.Contains(code, "checkAuth") {
		t.Error("expected checkAuth call for authenticated channel dispatch")
	}

	// Should have the dispatch route
	if !strings.Contains(code, `POST /channels/chatbot/dispatch`) {
		t.Error("expected POST /channels/chatbot/dispatch route")
	}

	// Should have 401 Unauthorized response
	if !strings.Contains(code, "Unauthorized") {
		t.Error("expected Unauthorized error for auth failure")
	}
}

func TestGenerateDispatchRoute_Public_NoAuth(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makePublicChannel("demo", 60, 10)})

	// Should have the dispatch route
	if !strings.Contains(code, `POST /channels/demo/dispatch`) {
		t.Error("expected POST /channels/demo/dispatch route")
	}

	// The public dispatch handler function definition should NOT call checkAuth.
	// Find the function definition (starts with "func handleDispatch_demo(")
	fnDef := "func handleDispatch_demo("
	dispatchIdx := strings.Index(code, fnDef)
	if dispatchIdx == -1 {
		t.Fatal("expected func handleDispatch_demo( definition")
	}
	dispatchSection := code[dispatchIdx:]

	// The function ends at the next top-level "func " or end of file
	nextFuncIdx := strings.Index(dispatchSection[len(fnDef):], "\nfunc ")
	if nextFuncIdx > 0 {
		dispatchSection = dispatchSection[:nextFuncIdx+len(fnDef)+1]
	}

	if strings.Contains(dispatchSection, "checkAuth(r)") {
		t.Error("expected no checkAuth call in public channel dispatch handler")
	}

	// Should set accountID to 0
	if !strings.Contains(dispatchSection, "accountID int64") || !strings.Contains(dispatchSection, "= 0") {
		t.Error("expected accountID = 0 for public channel")
	}
}

func TestGenerateDispatchRoute_Public_HasRateLimit(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makePublicChannel("demo", 120, 20)})

	// Should have rate limiter variable
	if !strings.Contains(code, "rateLimiter_demo") {
		t.Error("expected rateLimiter_demo variable for rate-limited public channel")
	}

	// Should instantiate with correct values
	if !strings.Contains(code, "newSimpleRateLimiter(120, 20)") {
		t.Error("expected newSimpleRateLimiter(120, 20) call")
	}

	// Should have rate limit check in dispatch handler
	if !strings.Contains(code, "rateLimiter_demo.Allow") {
		t.Error("expected rateLimiter_demo.Allow call in dispatch handler")
	}

	// Should have TooManyRequests error
	if !strings.Contains(code, "TooManyRequests") {
		t.Error("expected TooManyRequests error for rate limit exceeded")
	}

	// Should have the rate limiter types
	if !strings.Contains(code, "type simpleRateLimiter struct") {
		t.Error("expected simpleRateLimiter struct definition")
	}
}

func TestGenerateDispatchRoute_WithRBAC(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeRBACChannel("admin_chat", "admin")})

	// Should have checkAuth call
	if !strings.Contains(code, "checkAuth") {
		t.Error("expected checkAuth call for RBAC channel")
	}

	// Should have checkRBAC call
	if !strings.Contains(code, "checkRBAC") {
		t.Error("expected checkRBAC call for RBAC channel")
	}

	// Should reference the correct route path
	if !strings.Contains(code, `/channels/admin_chat/dispatch`) {
		t.Error("expected RBAC check against /channels/admin_chat/dispatch route path")
	}

	// Should have Forbidden error for insufficient permissions
	if !strings.Contains(code, "Forbidden") {
		t.Error("expected Forbidden error for insufficient permissions")
	}
}

func TestGenerateTokenRoute_ConnectionJWTClaims(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Should generate connection token
	if !strings.Contains(code, "transport.GenerateConnectionToken") {
		t.Error("expected GenerateConnectionToken call")
	}

	// Should pass the accountPublicID as sub
	if !strings.Contains(code, "GenerateConnectionToken(accountPublicID,") {
		t.Error("expected accountPublicID as sub for connection token")
	}

	// Should have 5*time.Minute TTL
	if !strings.Contains(code, "5*time.Minute") {
		t.Error("expected 5*time.Minute TTL for tokens")
	}
}

func TestGenerateTokenRoute_SubscriptionJWTClaims(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Should generate subscription token
	if !strings.Contains(code, "transport.GenerateSubscriptionToken") {
		t.Error("expected GenerateSubscriptionToken call")
	}

	// Should pass accountPublicID and channelName
	if !strings.Contains(code, "GenerateSubscriptionToken(accountPublicID, channelName,") {
		t.Error("expected accountPublicID and channelName in subscription token call")
	}

	// The sub in connection and subscription tokens must match.
	// Both should use accountPublicID.
	connCount := strings.Count(code, "accountPublicID")
	if connCount < 2 {
		t.Errorf("expected accountPublicID used in both connection and subscription tokens, found %d references", connCount)
	}
}

func TestGenerateTokenRoute_PublicChannel_AnonymousSub(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makePublicChannel("demo", 60, 10)})

	// Find the token handler function definition for this channel
	fnDef := "func handleToken_demo("
	tokenIdx := strings.Index(code, fnDef)
	if tokenIdx == -1 {
		t.Fatal("expected func handleToken_demo( definition")
	}
	tokenSection := code[tokenIdx:]
	nextFuncIdx := strings.Index(tokenSection[len(fnDef):], "\nfunc ")
	if nextFuncIdx > 0 {
		tokenSection = tokenSection[:nextFuncIdx+len(fnDef)+1]
	}

	// Should use empty string for anonymous sub in connection token
	if !strings.Contains(tokenSection, `GenerateConnectionToken("")`) && !strings.Contains(tokenSection, `GenerateConnectionToken("",`) {
		t.Error("expected empty string sub for anonymous connection token")
	}

	// Should use empty string for anonymous sub in subscription token
	if !strings.Contains(tokenSection, `GenerateSubscriptionToken("",`) && !strings.Contains(tokenSection, `GenerateSubscriptionToken("" ,`) {
		t.Error("expected empty string sub for anonymous subscription token")
	}
}

func TestGenerateTokenRoute_PublicChannel_UnscopedName(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makePublicChannel("demo", 60, 10)})

	// Find the token handler function definition for this channel
	fnDef := "func handleToken_demo("
	tokenIdx := strings.Index(code, fnDef)
	if tokenIdx == -1 {
		t.Fatal("expected func handleToken_demo( definition")
	}
	tokenSection := code[tokenIdx:]
	nextFuncIdx := strings.Index(tokenSection[len(fnDef):], "\nfunc ")
	if nextFuncIdx > 0 {
		tokenSection = tokenSection[:nextFuncIdx+len(fnDef)+1]
	}

	// Should use ComputeChannelID with "demo" and true (public)
	if !strings.Contains(tokenSection, `ComputeChannelID("demo"`) {
		t.Error("expected ComputeChannelID with channel name 'demo' for public channel")
	}
	if !strings.Contains(tokenSection, ", true)") {
		t.Error("expected isPublic=true in ComputeChannelID call for public channel")
	}
}

func TestGenerateTokenRoute_AuthChannel_ScopedName(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Find the token handler function definition for this channel
	fnDef := "func handleToken_chatbot("
	tokenIdx := strings.Index(code, fnDef)
	if tokenIdx == -1 {
		t.Fatal("expected func handleToken_chatbot( definition")
	}
	tokenSection := code[tokenIdx:]
	nextFuncIdx := strings.Index(tokenSection[len(fnDef):], "\nfunc ")
	if nextFuncIdx > 0 {
		tokenSection = tokenSection[:nextFuncIdx+len(fnDef)+1]
	}

	// Should use ComputeChannelID with "chatbot" and false (scoped)
	if !strings.Contains(tokenSection, `ComputeChannelID("chatbot"`) {
		t.Error("expected ComputeChannelID with channel name 'chatbot' for authenticated channel")
	}
	if !strings.Contains(tokenSection, ", false)") {
		t.Error("expected isPublic=false in ComputeChannelID call for authenticated channel")
	}
}

func TestGenerateTokenRoute_ReturnsChannelAndWSURL(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Response should include channel name
	if !strings.Contains(code, `"channel":`) {
		t.Error("expected 'channel' key in token response")
	}

	// Response should include ws_url
	if !strings.Contains(code, `"ws_url":`) {
		t.Error("expected 'ws_url' key in token response")
	}

	// Should use transport.ConnectionURL()
	if !strings.Contains(code, "transport.ConnectionURL()") {
		t.Error("expected transport.ConnectionURL() call")
	}

	// Response should include connection_token
	if !strings.Contains(code, `"connection_token":`) {
		t.Error("expected 'connection_token' key in token response")
	}

	// Response should include subscription_token
	if !strings.Contains(code, `"subscription_token":`) {
		t.Error("expected 'subscription_token' key in token response")
	}
}

func TestGenerateTokenRoute_L3_ChannelNameConsistency(t *testing.T) {
	// [L3] Critical: the channel name in the subscription token must exactly
	// match the channel the client subscribes to. Both the token endpoint and
	// the dispatch handler must use the same ComputeChannelID function.
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Token endpoint should use channel.ComputeChannelID
	if !strings.Contains(code, "channel.ComputeChannelID") {
		t.Error("expected channel.ComputeChannelID call for consistent channel name computation")
	}

	// Should have the L3 warning comment
	if !strings.Contains(code, "[L3] CRITICAL") {
		t.Error("expected [L3] CRITICAL comment about channel name matching")
	}
}

func TestGenerateJobStatusEndpoint(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Should have the job status route
	if !strings.Contains(code, `GET /channels/jobs/{id}`) {
		t.Error("expected GET /channels/jobs/{id} route")
	}

	// Should have the handleJobStatus function
	if !strings.Contains(code, "handleJobStatus") {
		t.Error("expected handleJobStatus function")
	}

	// Should use the query runner to look up job results
	if !strings.Contains(code, "runner.GetJobResult(") {
		t.Error("expected runner.GetJobResult() call in job status handler")
	}

	// Should check ownership for authenticated endpoints
	if !strings.Contains(code, "jobResult.AccountId != accountID") {
		t.Error("expected ownership check in job status handler")
	}
}

func TestGenerateChannelHTTPRoutes_ValidGo_AuthChannel(t *testing.T) {
	cfg := ChannelHTTPGenConfig{
		Channels:   []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")},
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
	}

	files, err := GenerateChannelHTTPRoutes(cfg)
	if err != nil {
		t.Fatalf("GenerateChannelHTTPRoutes() error = %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least one generated file")
	}

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "channels.go", files[0].Content, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated channel routes are not valid Go: %v\ncode:\n%s", parseErr, string(files[0].Content))
	}
}

func TestGenerateChannelHTTPRoutes_ValidGo_PublicChannel(t *testing.T) {
	cfg := ChannelHTTPGenConfig{
		Channels:   []codegen.SerializedChannelInfo{makePublicChannel("demo", 60, 10)},
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
	}

	files, err := GenerateChannelHTTPRoutes(cfg)
	if err != nil {
		t.Fatalf("GenerateChannelHTTPRoutes() error = %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least one generated file")
	}

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "channels.go", files[0].Content, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated channel routes are not valid Go: %v\ncode:\n%s", parseErr, string(files[0].Content))
	}
}

func TestGenerateChannelHTTPRoutes_ValidGo_MixedChannels(t *testing.T) {
	cfg := ChannelHTTPGenConfig{
		Channels: []codegen.SerializedChannelInfo{
			makeAuthChannel("chatbot"),
			makePublicChannel("demo", 60, 10),
			makeBackendChannel("processor"),
			makeRBACChannel("admin_chat", "admin"),
		},
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
	}

	files, err := GenerateChannelHTTPRoutes(cfg)
	if err != nil {
		t.Fatalf("GenerateChannelHTTPRoutes() error = %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least one generated file")
	}

	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "channels.go", files[0].Content, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("generated channel routes are not valid Go: %v\ncode:\n%s", parseErr, string(files[0].Content))
	}
}

func TestGenerateChannelHTTPRoutes_OutputRelPath(t *testing.T) {
	cfg := ChannelHTTPGenConfig{
		Channels:   []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")},
		ModulePath: "example.com/myapp",
		OutputPkg:  "api",
	}

	files, err := GenerateChannelHTTPRoutes(cfg)
	if err != nil {
		t.Fatalf("GenerateChannelHTTPRoutes() error = %v", err)
	}

	if files[0].RelPath != "api/zz_generated_channels.go" {
		t.Errorf("expected RelPath 'api/zz_generated_channels.go', got %q", files[0].RelPath)
	}
}

func TestGenerateChannelHTTPRoutes_RegisterFunction(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Should have RegisterChannelRoutes function
	if !strings.Contains(code, "func RegisterChannelRoutes(") {
		t.Error("expected RegisterChannelRoutes function")
	}

	// Should accept queue and transport as parameters
	if !strings.Contains(code, "queue channel.TaskQueue") {
		t.Error("expected TaskQueue parameter")
	}
	if !strings.Contains(code, "transport channel.RealtimeTransport") {
		t.Error("expected RealtimeTransport parameter")
	}

	// Should accept db parameter
	if !strings.Contains(code, "db *sql.DB") {
		t.Error("expected *sql.DB parameter")
	}

	// Should accept runner parameter
	if !strings.Contains(code, "runner queries.Runner") {
		t.Error("expected queries.Runner parameter")
	}
}

func TestGenerateChannelHTTPRoutes_UsesInterfacesOnly(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{
		makeAuthChannel("chatbot"),
		makePublicChannel("demo", 60, 10),
	})

	// Should NOT contain any direct Machinery references
	if strings.Contains(code, "MachineryQueue") {
		t.Error("generated HTTP code should not reference MachineryQueue directly")
	}
	if strings.Contains(code, "machinery") {
		t.Error("generated HTTP code should not reference machinery directly")
	}

	// Should NOT contain any direct Centrifugo references
	if strings.Contains(code, "CentrifugoTransport") {
		t.Error("generated HTTP code should not reference CentrifugoTransport directly")
	}
	if strings.Contains(code, "centrifugo") && !strings.Contains(code, "CENTRIFUGO") {
		// Allow CENTRIFUGO_* env var names, but not centrifugo package references
		t.Error("generated HTTP code should not reference centrifugo directly")
	}

	// Should use the interface methods
	if !strings.Contains(code, "queue.SendTask") {
		t.Error("expected queue.SendTask call via interface")
	}
	if !strings.Contains(code, "transport.GenerateConnectionToken") {
		t.Error("expected transport.GenerateConnectionToken call via interface")
	}
	if !strings.Contains(code, "transport.GenerateSubscriptionToken") {
		t.Error("expected transport.GenerateSubscriptionToken call via interface")
	}
	if !strings.Contains(code, "transport.ConnectionURL()") {
		t.Error("expected transport.ConnectionURL() call via interface")
	}
}

func TestGenerateChannelHTTPRoutes_DispatchReturnJobID(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Dispatch endpoint should return job_id
	if !strings.Contains(code, `"job_id"`) {
		t.Error("expected job_id in dispatch response")
	}

	// Should use http.StatusAccepted (202)
	if !strings.Contains(code, "http.StatusAccepted") {
		t.Error("expected http.StatusAccepted (202) for dispatch response")
	}
}

func TestGenerateChannelHTTPRoutes_DispatchInsertJobResults(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Should use runner.InsertJobResult instead of raw SQL
	if !strings.Contains(code, "runner.InsertJobResult(") {
		t.Error("expected runner.InsertJobResult call")
	}

	// Should NOT contain raw INSERT SQL (replaced by runner call)
	if strings.Contains(code, "INSERT INTO job_results") {
		t.Error("dispatch handler should use runner.InsertJobResult, not raw INSERT SQL")
	}

	// Should set status to pending
	if !strings.Contains(code, `"pending"`) {
		t.Error("expected pending status in insert")
	}
}

func TestGenerateChannelHTTPRoutes_DispatchUsesTaskOptions(t *testing.T) {
	ch := makeAuthChannel("chatbot")
	ch.MaxRetries = 5
	ch.BackoffSeconds = 10
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{ch})

	// Should pass TaskOptions with correct values
	if !strings.Contains(code, "RetryCount:    5") {
		t.Error("expected RetryCount: 5 in TaskOptions")
	}
	if !strings.Contains(code, "RetryTimeoutS: 10") {
		t.Error("expected RetryTimeoutS: 10 in TaskOptions")
	}
}

func TestGenerateChannelHTTPRoutes_DispatchPayloadStructure(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Should construct DispatchPayload
	if !strings.Contains(code, "channel.DispatchPayload") {
		t.Error("expected channel.DispatchPayload construction")
	}

	// Should set all required fields
	if !strings.Contains(code, "JobID:") {
		t.Error("expected JobID field in DispatchPayload")
	}
	if !strings.Contains(code, "ChannelName:") {
		t.Error("expected ChannelName field in DispatchPayload")
	}
	if !strings.Contains(code, "AccountID:") {
		t.Error("expected AccountID field in DispatchPayload")
	}
	if !strings.Contains(code, "OrgID:") {
		t.Error("expected OrgID field in DispatchPayload")
	}
	if !strings.Contains(code, "IsPublic:") {
		t.Error("expected IsPublic field in DispatchPayload")
	}
	if !strings.Contains(code, "Request:") {
		t.Error("expected Request field in DispatchPayload")
	}
}

func TestGenerateChannelHTTPRoutes_TokenOwnershipVerification(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// The token handler should contain ownership verification.
	// Look for the handleToken_chatbot function definition (it appears multiple times
	// in the code: once in the closure and once as the function itself)
	if !strings.Contains(code, "handleToken_chatbot") {
		t.Fatal("expected handleToken_chatbot function")
	}

	// Should verify ownership somewhere in the token code
	if !strings.Contains(code, "jobResult.AccountId != accountID") {
		t.Error("expected ownership check in token handler")
	}

	// Should return Forbidden for non-owners
	if !strings.Contains(code, "you do not own this job") {
		t.Error("expected Forbidden error for non-owner in token handler")
	}
}

func TestGenerateChannelHTTPRoutes_GeneratedComment(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	if !strings.Contains(code, "Code generated by shipq") {
		t.Error("expected generated file comment")
	}
}

func TestGenerateChannelHTTPRoutes_PackageName(t *testing.T) {
	cfg := ChannelHTTPGenConfig{
		Channels:   []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")},
		ModulePath: "example.com/myapp",
		OutputPkg:  "myapi",
	}

	files, err := GenerateChannelHTTPRoutes(cfg)
	if err != nil {
		t.Fatalf("GenerateChannelHTTPRoutes() error = %v", err)
	}

	code := string(files[0].Content)
	if !strings.Contains(code, "package myapi") {
		t.Error("expected 'package myapi' in generated code")
	}

	if files[0].RelPath != "myapi/zz_generated_channels.go" {
		t.Errorf("expected RelPath with custom output pkg, got %q", files[0].RelPath)
	}
}

func TestGenerateChannelHTTPRoutes_NanoidForJobID(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Should use nanoid for job ID generation
	if !strings.Contains(code, "nanoid.New()") {
		t.Error("expected nanoid.New() for job ID generation")
	}
}

// ---------------------------------------------------------------------------
// Bug 3: Generated channel handlers must not use raw SQL with ? placeholders.
// All database access should go through the query runner.
// ---------------------------------------------------------------------------

func TestGenerateChannelHTTPRoutes_NoRawSQL_AuthChannel(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// The generated code must NOT contain raw db.QueryRow or db.Exec calls.
	// All database access should go through the query runner (runner.GetJobResult, etc.).
	if strings.Contains(code, "db.QueryRow(") {
		t.Error("generated channel routes must not use raw db.QueryRow(); use runner.GetJobResult() instead")
	}
	if strings.Contains(code, "db.Exec(") {
		t.Error("generated channel routes must not use raw db.Exec(); use query runner methods instead")
	}
	if strings.Contains(code, "db.Query(") {
		t.Error("generated channel routes must not use raw db.Query(); use query runner methods instead")
	}
}

func TestGenerateChannelHTTPRoutes_NoRawSQL_PublicChannel(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makePublicChannel("demo", 60, 10)})

	if strings.Contains(code, "db.QueryRow(") {
		t.Error("generated public channel routes must not use raw db.QueryRow()")
	}
	if strings.Contains(code, "db.Exec(") {
		t.Error("generated public channel routes must not use raw db.Exec()")
	}
}

func TestGenerateChannelHTTPRoutes_NoRawSQL_MixedChannels(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		makeAuthChannel("chatbot"),
		makePublicChannel("demo", 60, 10),
	}
	code := generateAndCheck(t, channels)

	if strings.Contains(code, "db.QueryRow(") {
		t.Error("generated mixed channel routes must not use raw db.QueryRow()")
	}
	if strings.Contains(code, "db.Exec(") {
		t.Error("generated mixed channel routes must not use raw db.Exec()")
	}
}

func TestGenerateChannelHTTPRoutes_TokenHandler_UsesQueryRunner(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Token handler should use runner.GetJobResult for job lookup
	if !strings.Contains(code, "runner.GetJobResult(") {
		t.Error("token handler must use runner.GetJobResult() instead of raw SQL")
	}

	// Token handler should receive runner, not db
	if strings.Contains(code, "func handleToken_chatbot(\n\tw http.ResponseWriter,\n\tr *http.Request,\n\ttransport channel.RealtimeTransport,\n\tdb *sql.DB,") {
		t.Error("token handler must accept queries.Runner, not *sql.DB")
	}
	if !strings.Contains(code, "runner queries.Runner") {
		t.Error("token handler must accept runner queries.Runner parameter")
	}
}

func TestGenerateChannelHTTPRoutes_JobStatusHandler_UsesQueryRunner(t *testing.T) {
	code := generateAndCheck(t, []codegen.SerializedChannelInfo{makeAuthChannel("chatbot")})

	// Job status handler should reference jobResult struct fields, not local scan vars
	if !strings.Contains(code, "jobResult.PublicId") {
		t.Error("job status handler must use jobResult.PublicId from query runner result")
	}
	if !strings.Contains(code, "jobResult.Status") {
		t.Error("job status handler must use jobResult.Status from query runner result")
	}
	if !strings.Contains(code, "jobResult.ChannelName") {
		t.Error("job status handler must use jobResult.ChannelName from query runner result")
	}
}

func TestGenerateChannelHTTPRoutes_NoQuestionMarkPlaceholders(t *testing.T) {
	// Verify that the generated code contains zero raw SQL placeholders.
	// The ? placeholder syntax works on SQLite and MySQL but NOT PostgreSQL.
	channels := []codegen.SerializedChannelInfo{
		makeAuthChannel("chatbot"),
		makePublicChannel("demo", 60, 10),
	}
	code := generateAndCheck(t, channels)

	// Check for raw SQL patterns with ? placeholders
	if strings.Contains(code, "WHERE public_id = ?") {
		t.Error("generated code must not contain raw SQL with ? placeholders; use query runner instead")
	}
	if strings.Contains(code, "SELECT ") && strings.Contains(code, " FROM job_results") {
		t.Error("generated code must not contain raw SELECT FROM job_results; use runner.GetJobResult() instead")
	}
}
