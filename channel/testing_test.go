package channel

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestTestRecorder_ImplementsRealtimeTransport(t *testing.T) {
	// Compile-time check is already in testing.go via:
	//   var _ RealtimeTransport = (*TestRecorder)(nil)
	// This test verifies all methods work at runtime.
	recorder := NewTestRecorder()

	// Publish
	env := Envelope{Type: "Ping", Data: json.RawMessage(`{}`)}
	if err := recorder.Publish("ch", env.Marshal()); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// Subscribe
	incoming, cleanup, err := recorder.Subscribe("ch", "sub-1")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cleanup()
	if incoming == nil {
		t.Fatal("Subscribe returned nil incoming channel")
	}

	// GenerateConnectionToken
	connToken, err := recorder.GenerateConnectionToken("user-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("GenerateConnectionToken: %v", err)
	}
	if connToken != "test-conn-token-user-1" {
		t.Errorf("GenerateConnectionToken: got %q, want %q", connToken, "test-conn-token-user-1")
	}

	// GenerateSubscriptionToken
	subToken, err := recorder.GenerateSubscriptionToken("user-1", "my-channel", 5*time.Minute)
	if err != nil {
		t.Fatalf("GenerateSubscriptionToken: %v", err)
	}
	if subToken != "test-sub-token-my-channel" {
		t.Errorf("GenerateSubscriptionToken: got %q, want %q", subToken, "test-sub-token-my-channel")
	}

	// ConnectionURL
	url := recorder.ConnectionURL()
	if url != "ws://test/connection/websocket" {
		t.Errorf("ConnectionURL: got %q, want %q", url, "ws://test/connection/websocket")
	}
}

func TestTestRecorder_Send_CapturesMessages(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	data, _ := json.Marshal(map[string]string{"token": "xyz"})
	if err := ch.Send(ctx, "Token", data); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if len(recorder.Sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(recorder.Sent))
	}
	if recorder.Sent[0].Type != "Token" {
		t.Errorf("sent type: got %q, want %q", recorder.Sent[0].Type, "Token")
	}

	var payload map[string]string
	if err := json.Unmarshal(recorder.Sent[0].Data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["token"] != "xyz" {
		t.Errorf("sent data token: got %q, want %q", payload["token"], "xyz")
	}
}

func TestTestRecorder_Receive_ReturnsPreQueued(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	// Enqueue a message before Receive.
	recorder.EnqueueIncoming("Approval", map[string]bool{"ok": true})

	data, err := ch.Receive(ctx, "Approval")
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	var payload map[string]bool
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !payload["ok"] {
		t.Error("expected ok=true")
	}
}

func TestTestRecorder_HasSent(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	// Nothing sent yet.
	if recorder.HasSent("Token") {
		t.Error("HasSent should be false before any sends")
	}
	if recorder.HasSent("Done") {
		t.Error("HasSent should be false before any sends")
	}

	// Send a Token message.
	data, _ := json.Marshal(map[string]string{"t": "abc"})
	if err := ch.Send(ctx, "Token", data); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if !recorder.HasSent("Token") {
		t.Error("HasSent('Token') should be true after sending Token")
	}
	if recorder.HasSent("Done") {
		t.Error("HasSent('Done') should be false -- only Token was sent")
	}

	// Send a Done message.
	doneData, _ := json.Marshal(map[string]bool{"success": true})
	if err := ch.Send(ctx, "Done", doneData); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if !recorder.HasSent("Done") {
		t.Error("HasSent('Done') should be true after sending Done")
	}
}

func TestTestRecorder_SentOfType(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	// Send 3 Token messages and 1 Done message.
	for i := 0; i < 3; i++ {
		data, _ := json.Marshal(map[string]int{"i": i})
		if err := ch.Send(ctx, "Token", data); err != nil {
			t.Fatalf("Send Token %d: %v", i, err)
		}
	}
	doneData, _ := json.Marshal(map[string]bool{"ok": true})
	if err := ch.Send(ctx, "Done", doneData); err != nil {
		t.Fatalf("Send Done: %v", err)
	}

	tokens := recorder.SentOfType("Token")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 Token messages, got %d", len(tokens))
	}
	for _, env := range tokens {
		if env.Type != "Token" {
			t.Errorf("expected type Token, got %q", env.Type)
		}
	}

	dones := recorder.SentOfType("Done")
	if len(dones) != 1 {
		t.Fatalf("expected 1 Done message, got %d", len(dones))
	}

	nones := recorder.SentOfType("NonExistent")
	if len(nones) != 0 {
		t.Errorf("expected 0 messages for non-existent type, got %d", len(nones))
	}
}

func TestTestRecorder_ReceiveAny_ReturnsAny(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	// Enqueue messages of different types.
	recorder.EnqueueIncoming("TypeA", map[string]string{"a": "1"})
	recorder.EnqueueIncoming("TypeB", map[string]string{"b": "2"})

	typeName, data, err := ch.ReceiveAny(ctx)
	if err != nil {
		t.Fatalf("ReceiveAny: %v", err)
	}

	// Should get the first message enqueued.
	if typeName != "TypeA" {
		t.Errorf("expected type %q, got %q", "TypeA", typeName)
	}

	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["a"] != "1" {
		t.Errorf("expected a=%q, got %q", "1", payload["a"])
	}

	// Second call should return TypeB.
	typeName2, data2, err := ch.ReceiveAny(ctx)
	if err != nil {
		t.Fatalf("ReceiveAny 2: %v", err)
	}
	if typeName2 != "TypeB" {
		t.Errorf("expected type %q, got %q", "TypeB", typeName2)
	}

	var payload2 map[string]string
	if err := json.Unmarshal(data2, &payload2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload2["b"] != "2" {
		t.Errorf("expected b=%q, got %q", "2", payload2["b"])
	}
}

func TestWithTestChannel_CreatesWorkingChannel(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	// Verify the channel was created with sensible defaults.
	if ch.Name() != "test" {
		t.Errorf("expected name %q, got %q", "test", ch.Name())
	}
	if ch.JobID() != "test-job" {
		t.Errorf("expected jobID %q, got %q", "test-job", ch.JobID())
	}
}

func TestTestRecorder_EnqueueIncoming_MultipleTypes(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	// Enqueue several types and retrieve them in specific order via Receive.
	recorder.EnqueueIncoming("A", "alpha")
	recorder.EnqueueIncoming("B", "beta")
	recorder.EnqueueIncoming("C", "gamma")

	// Ask for C first -- A and B should be buffered.
	dataC, err := ch.Receive(ctx, "C")
	if err != nil {
		t.Fatalf("Receive C: %v", err)
	}
	var c string
	json.Unmarshal(dataC, &c)
	if c != "gamma" {
		t.Errorf("expected %q, got %q", "gamma", c)
	}

	// Now A should come from buffer.
	dataA, err := ch.Receive(ctx, "A")
	if err != nil {
		t.Fatalf("Receive A: %v", err)
	}
	var a string
	json.Unmarshal(dataA, &a)
	if a != "alpha" {
		t.Errorf("expected %q, got %q", "alpha", a)
	}

	// Now B should come from buffer.
	dataB, err := ch.Receive(ctx, "B")
	if err != nil {
		t.Fatalf("Receive B: %v", err)
	}
	var b string
	json.Unmarshal(dataB, &b)
	if b != "beta" {
		t.Errorf("expected %q, got %q", "beta", b)
	}
}

func TestTestRecorder_ConnectionToken_Format(t *testing.T) {
	recorder := NewTestRecorder()
	token, err := recorder.GenerateConnectionToken("user-42", time.Hour)
	if err != nil {
		t.Fatalf("GenerateConnectionToken: %v", err)
	}
	if token != "test-conn-token-user-42" {
		t.Errorf("got %q, want %q", token, "test-conn-token-user-42")
	}
}

func TestTestRecorder_SubscriptionToken_Format(t *testing.T) {
	recorder := NewTestRecorder()
	token, err := recorder.GenerateSubscriptionToken("user-42", "approval_100_job-1", time.Hour)
	if err != nil {
		t.Fatalf("GenerateSubscriptionToken: %v", err)
	}
	if token != "test-sub-token-approval_100_job-1" {
		t.Errorf("got %q, want %q", token, "test-sub-token-approval_100_job-1")
	}
}

func TestTestRecorder_SentOfType_EmptyWhenNothingSent(t *testing.T) {
	recorder := NewTestRecorder()
	result := recorder.SentOfType("Anything")
	if result != nil {
		t.Errorf("expected nil for unsent type, got %v", result)
	}
}

func TestTestRecorder_HasSent_FalseWhenEmpty(t *testing.T) {
	recorder := NewTestRecorder()
	if recorder.HasSent("Anything") {
		t.Error("HasSent should be false on fresh recorder")
	}
}
