package channel

import (
	"testing"
)

// Test message types used across tests.

type ApprovalRequest struct {
	Amount int    `json:"amount"`
	Reason string `json:"reason"`
}

type ApprovalFollowUp struct {
	Note string `json:"note"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type DoneResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type NestedInner struct {
	Value int `json:"value"`
}

type NestedOuter struct {
	Name  string      `json:"name"`
	Inner NestedInner `json:"inner"`
}

type PointerAndSliceFields struct {
	Name     string   `json:"name"`
	Tags     []string `json:"tags"`
	Optional *int     `json:"optional,omitempty"`
}

func TestFromClient_TagsDirection(t *testing.T) {
	defs := FromClient(ApprovalRequest{})
	if len(defs) != 1 {
		t.Fatalf("expected 1 MessageDef, got %d", len(defs))
	}
	if defs[0].Direction != ClientToServer {
		t.Errorf("expected Direction=ClientToServer, got %d", defs[0].Direction)
	}
}

func TestFromServer_TagsDirection(t *testing.T) {
	defs := FromServer(TokenResponse{})
	if len(defs) != 1 {
		t.Fatalf("expected 1 MessageDef, got %d", len(defs))
	}
	if defs[0].Direction != ServerToClient {
		t.Errorf("expected Direction=ServerToClient, got %d", defs[0].Direction)
	}
}

func TestFromClient_MultipleTypes(t *testing.T) {
	defs := FromClient(ApprovalRequest{}, ApprovalFollowUp{})
	if len(defs) != 2 {
		t.Fatalf("expected 2 MessageDefs, got %d", len(defs))
	}
	if defs[0].Direction != ClientToServer {
		t.Errorf("defs[0]: expected Direction=ClientToServer, got %d", defs[0].Direction)
	}
	if defs[1].Direction != ClientToServer {
		t.Errorf("defs[1]: expected Direction=ClientToServer, got %d", defs[1].Direction)
	}
}

func TestFromServer_MultipleTypes(t *testing.T) {
	defs := FromServer(TokenResponse{}, DoneResponse{})
	if len(defs) != 2 {
		t.Fatalf("expected 2 MessageDefs, got %d", len(defs))
	}
	for i, d := range defs {
		if d.Direction != ServerToClient {
			t.Errorf("defs[%d]: expected Direction=ServerToClient, got %d", i, d.Direction)
		}
	}
}

func TestDefineChannel_CapturesMetadata(t *testing.T) {
	app := NewApp()
	app.DefineChannel("approval",
		FromClient(ApprovalRequest{}, ApprovalFollowUp{}),
		FromServer(TokenResponse{}, DoneResponse{}),
	)

	if len(app.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(app.Channels))
	}

	ch := app.Channels[0]
	if ch.Name != "approval" {
		t.Errorf("expected name %q, got %q", "approval", ch.Name)
	}
	if ch.Visibility != Frontend {
		t.Errorf("expected Visibility=Frontend, got %d", ch.Visibility)
	}

	// 2 client + 2 server = 4 messages
	if len(ch.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(ch.Messages))
	}

	// Client messages
	if ch.Messages[0].Direction != ClientToServer {
		t.Errorf("msg[0]: expected ClientToServer, got %d", ch.Messages[0].Direction)
	}
	if ch.Messages[0].TypeName != "ApprovalRequest" {
		t.Errorf("msg[0]: expected TypeName %q, got %q", "ApprovalRequest", ch.Messages[0].TypeName)
	}
	if ch.Messages[1].Direction != ClientToServer {
		t.Errorf("msg[1]: expected ClientToServer, got %d", ch.Messages[1].Direction)
	}
	if ch.Messages[1].TypeName != "ApprovalFollowUp" {
		t.Errorf("msg[1]: expected TypeName %q, got %q", "ApprovalFollowUp", ch.Messages[1].TypeName)
	}

	// Server messages
	if ch.Messages[2].Direction != ServerToClient {
		t.Errorf("msg[2]: expected ServerToClient, got %d", ch.Messages[2].Direction)
	}
	if ch.Messages[2].TypeName != "TokenResponse" {
		t.Errorf("msg[2]: expected TypeName %q, got %q", "TokenResponse", ch.Messages[2].TypeName)
	}
	if ch.Messages[3].Direction != ServerToClient {
		t.Errorf("msg[3]: expected ServerToClient, got %d", ch.Messages[3].Direction)
	}
	if ch.Messages[3].TypeName != "DoneResponse" {
		t.Errorf("msg[3]: expected TypeName %q, got %q", "DoneResponse", ch.Messages[3].TypeName)
	}
}

func TestDefineBackendChannel_SetsBackendVisibility(t *testing.T) {
	app := NewApp()
	app.DefineBackendChannel("internal-sync",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	)

	if len(app.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(app.Channels))
	}
	if app.Channels[0].Visibility != Backend {
		t.Errorf("expected Visibility=Backend, got %d", app.Channels[0].Visibility)
	}
}

func TestChannelBuilder_Retries(t *testing.T) {
	app := NewApp()
	app.DefineChannel("approval",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	).Retries(3)

	if app.Channels[0].MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", app.Channels[0].MaxRetries)
	}
}

func TestChannelBuilder_BackoffSeconds(t *testing.T) {
	app := NewApp()
	app.DefineChannel("approval",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	).BackoffSeconds(10)

	if app.Channels[0].BackoffSeconds != 10 {
		t.Errorf("expected BackoffSeconds=10, got %d", app.Channels[0].BackoffSeconds)
	}
}

func TestChannelBuilder_TimeoutSeconds(t *testing.T) {
	app := NewApp()
	app.DefineChannel("approval",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	).TimeoutSeconds(60)

	if app.Channels[0].TimeoutSeconds != 60 {
		t.Errorf("expected TimeoutSeconds=60, got %d", app.Channels[0].TimeoutSeconds)
	}
}

func TestChannelBuilder_Public(t *testing.T) {
	app := NewApp()
	app.DefineChannel("public-channel",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	).Public(RateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         10,
	})

	ch := app.Channels[0]
	if !ch.IsPublic {
		t.Error("expected IsPublic=true")
	}
	if ch.RateLimit == nil {
		t.Fatal("expected RateLimit to be set")
	}
	if ch.RateLimit.RequestsPerMinute != 60 {
		t.Errorf("expected RequestsPerMinute=60, got %d", ch.RateLimit.RequestsPerMinute)
	}
	if ch.RateLimit.BurstSize != 10 {
		t.Errorf("expected BurstSize=10, got %d", ch.RateLimit.BurstSize)
	}
}

func TestChannelBuilder_RequireRole(t *testing.T) {
	app := NewApp()
	app.DefineChannel("admin-channel",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	).RequireRole("admin")

	if app.Channels[0].RequiredRole != "admin" {
		t.Errorf("expected RequiredRole=%q, got %q", "admin", app.Channels[0].RequiredRole)
	}
}

func TestChannelBuilder_Chaining(t *testing.T) {
	app := NewApp()
	app.DefineChannel("complex",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	).Retries(5).BackoffSeconds(30).TimeoutSeconds(120).RequireRole("editor")

	ch := app.Channels[0]
	if ch.MaxRetries != 5 {
		t.Errorf("MaxRetries: got %d, want 5", ch.MaxRetries)
	}
	if ch.BackoffSeconds != 30 {
		t.Errorf("BackoffSeconds: got %d, want 30", ch.BackoffSeconds)
	}
	if ch.TimeoutSeconds != 120 {
		t.Errorf("TimeoutSeconds: got %d, want 120", ch.TimeoutSeconds)
	}
	if ch.RequiredRole != "editor" {
		t.Errorf("RequiredRole: got %q, want %q", ch.RequiredRole, "editor")
	}
}

func TestExtractMessageInfo_Fields(t *testing.T) {
	app := NewApp()
	app.DefineChannel("test",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	)

	// Check ApprovalRequest fields
	msg := app.Channels[0].Messages[0]
	if msg.TypeName != "ApprovalRequest" {
		t.Fatalf("expected TypeName %q, got %q", "ApprovalRequest", msg.TypeName)
	}
	if len(msg.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(msg.Fields))
	}

	// Field: Amount
	amount := msg.Fields[0]
	if amount.Name != "Amount" {
		t.Errorf("field[0] Name: got %q, want %q", amount.Name, "Amount")
	}
	if amount.Type != "int" {
		t.Errorf("field[0] Type: got %q, want %q", amount.Type, "int")
	}
	if amount.JSONName != "amount" {
		t.Errorf("field[0] JSONName: got %q, want %q", amount.JSONName, "amount")
	}
	if !amount.Required {
		t.Error("field[0] Amount should be Required")
	}

	// Field: Reason
	reason := msg.Fields[1]
	if reason.Name != "Reason" {
		t.Errorf("field[1] Name: got %q, want %q", reason.Name, "Reason")
	}
	if reason.Type != "string" {
		t.Errorf("field[1] Type: got %q, want %q", reason.Type, "string")
	}
	if reason.JSONName != "reason" {
		t.Errorf("field[1] JSONName: got %q, want %q", reason.JSONName, "reason")
	}
	if !reason.Required {
		t.Error("field[1] Reason should be Required")
	}

	// Check DoneResponse fields (has omitempty)
	done := app.Channels[0].Messages[1]
	if done.TypeName != "DoneResponse" {
		t.Fatalf("expected TypeName %q, got %q", "DoneResponse", done.TypeName)
	}
	if len(done.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(done.Fields))
	}

	// Field: Success (required)
	success := done.Fields[0]
	if success.JSONName != "success" {
		t.Errorf("Success JSONName: got %q, want %q", success.JSONName, "success")
	}
	if !success.Required {
		t.Error("Success should be Required")
	}

	// Field: Message (omitempty -> not required)
	message := done.Fields[1]
	if message.JSONName != "message" {
		t.Errorf("Message JSONName: got %q, want %q", message.JSONName, "message")
	}
	if message.Required {
		t.Error("Message should NOT be Required (has omitempty)")
	}
	if !message.JSONOmit {
		t.Error("Message should have JSONOmit=true (has omitempty)")
	}
}

func TestFirstFromClientIsDispatch(t *testing.T) {
	app := NewApp()
	app.DefineChannel("approval",
		FromClient(ApprovalRequest{}, ApprovalFollowUp{}),
		FromServer(TokenResponse{}, DoneResponse{}),
	)

	msgs := app.Channels[0].Messages

	// First client message should be dispatch.
	if !msgs[0].IsDispatch {
		t.Error("first FromClient type should have IsDispatch=true")
	}
	if msgs[0].HandlerName != "HandleApprovalRequest" {
		t.Errorf("expected HandlerName %q, got %q", "HandleApprovalRequest", msgs[0].HandlerName)
	}

	// Second client message should NOT be dispatch.
	if msgs[1].IsDispatch {
		t.Error("second FromClient type should have IsDispatch=false")
	}
	if msgs[1].HandlerName != "" {
		t.Errorf("expected empty HandlerName for non-dispatch, got %q", msgs[1].HandlerName)
	}

	// Server messages should NOT be dispatch.
	if msgs[2].IsDispatch {
		t.Error("server message should have IsDispatch=false")
	}
	if msgs[3].IsDispatch {
		t.Error("server message should have IsDispatch=false")
	}
}

func TestExtractMessageInfo_PointerAndSliceFields(t *testing.T) {
	app := NewApp()
	app.DefineChannel("test",
		FromClient(PointerAndSliceFields{}),
		FromServer(DoneResponse{}),
	)

	msg := app.Channels[0].Messages[0]
	if len(msg.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(msg.Fields))
	}

	// Name: string, required
	name := msg.Fields[0]
	if name.Type != "string" {
		t.Errorf("Name Type: got %q, want %q", name.Type, "string")
	}
	if !name.Required {
		t.Error("Name should be Required")
	}

	// Tags: []string, not required (slice)
	tags := msg.Fields[1]
	if tags.Type != "[]string" {
		t.Errorf("Tags Type: got %q, want %q", tags.Type, "[]string")
	}
	if tags.Required {
		t.Error("Tags should NOT be Required (slice type)")
	}

	// Optional: *int, not required (pointer + omitempty)
	optional := msg.Fields[2]
	if optional.Type != "*int" {
		t.Errorf("Optional Type: got %q, want %q", optional.Type, "*int")
	}
	if optional.Required {
		t.Error("Optional should NOT be Required (pointer + omitempty)")
	}
}

func TestExtractMessageInfo_NestedStruct(t *testing.T) {
	app := NewApp()
	app.DefineChannel("test",
		FromClient(NestedOuter{}),
		FromServer(DoneResponse{}),
	)

	msg := app.Channels[0].Messages[0]
	if len(msg.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(msg.Fields))
	}

	inner := msg.Fields[1]
	if inner.Name != "Inner" {
		t.Errorf("expected field name %q, got %q", "Inner", inner.Name)
	}
	if inner.StructFields == nil {
		t.Fatal("expected StructFields to be non-nil for nested struct")
	}
	if inner.StructFields.Name != "NestedInner" {
		t.Errorf("expected nested struct name %q, got %q", "NestedInner", inner.StructFields.Name)
	}
	if len(inner.StructFields.Fields) != 1 {
		t.Fatalf("expected 1 field in nested struct, got %d", len(inner.StructFields.Fields))
	}
	if inner.StructFields.Fields[0].Name != "Value" {
		t.Errorf("expected nested field name %q, got %q", "Value", inner.StructFields.Fields[0].Name)
	}
}

func TestDefineChannel_DefaultValues(t *testing.T) {
	app := NewApp()
	app.DefineChannel("simple",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	)

	ch := app.Channels[0]
	if ch.IsPublic {
		t.Error("default IsPublic should be false")
	}
	if ch.RateLimit != nil {
		t.Error("default RateLimit should be nil")
	}
	if ch.MaxRetries != 0 {
		t.Errorf("default MaxRetries should be 0, got %d", ch.MaxRetries)
	}
	if ch.BackoffSeconds != 0 {
		t.Errorf("default BackoffSeconds should be 0, got %d", ch.BackoffSeconds)
	}
	if ch.TimeoutSeconds != 0 {
		t.Errorf("default TimeoutSeconds should be 0, got %d", ch.TimeoutSeconds)
	}
	if ch.RequiredRole != "" {
		t.Errorf("default RequiredRole should be empty, got %q", ch.RequiredRole)
	}
}

func TestMultipleChannels(t *testing.T) {
	app := NewApp()
	app.DefineChannel("channel1",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	)
	app.DefineBackendChannel("channel2",
		FromClient(ApprovalFollowUp{}),
		FromServer(TokenResponse{}),
	)

	if len(app.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(app.Channels))
	}
	if app.Channels[0].Name != "channel1" {
		t.Errorf("channel[0] name: got %q, want %q", app.Channels[0].Name, "channel1")
	}
	if app.Channels[0].Visibility != Frontend {
		t.Errorf("channel[0] visibility: got %d, want Frontend", app.Channels[0].Visibility)
	}
	if app.Channels[1].Name != "channel2" {
		t.Errorf("channel[1] name: got %q, want %q", app.Channels[1].Name, "channel2")
	}
	if app.Channels[1].Visibility != Backend {
		t.Errorf("channel[1] visibility: got %d, want Backend", app.Channels[1].Visibility)
	}
}

func TestNewApp_EmptyChannels(t *testing.T) {
	app := NewApp()
	if app.Channels == nil {
		t.Fatal("Channels should be initialized, not nil")
	}
	if len(app.Channels) != 0 {
		t.Errorf("expected 0 channels, got %d", len(app.Channels))
	}
}

func TestExtractMessageInfo_TagsParsing(t *testing.T) {
	app := NewApp()
	app.DefineChannel("test",
		FromClient(ApprovalRequest{}),
		FromServer(DoneResponse{}),
	)

	msg := app.Channels[0].Messages[0]
	// Check that the json tag was captured in Tags map
	amountField := msg.Fields[0]
	if jsonTag, ok := amountField.Tags["json"]; !ok {
		t.Error("expected 'json' key in Tags map")
	} else if jsonTag != "amount" {
		t.Errorf("expected json tag %q, got %q", "amount", jsonTag)
	}
}
