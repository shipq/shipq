package channel

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestFromContext_Panics_WhenMissing(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when FromContext called on bare context")
		}
	}()
	FromContext(context.Background())
}

func TestWithChannel_RoundTrips(t *testing.T) {
	recorder := NewTestRecorder()
	incoming, cleanup, _ := recorder.Subscribe("test-ch", "sub-1")
	defer cleanup()

	ch := NewChannel("test", "job-1", 100, 200, false, recorder, incoming, cleanup)
	ctx := WithChannel(context.Background(), ch)

	got := FromContext(ctx)
	if got != ch {
		t.Errorf("expected same pointer from FromContext, got different")
	}
}

func TestChannel_Send_PublishesEnvelope(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	data, _ := json.Marshal(map[string]string{"token": "abc123"})
	if err := ch.Send(ctx, "Token", data); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if len(recorder.Sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(recorder.Sent))
	}
	if recorder.Sent[0].Type != "Token" {
		t.Errorf("expected type %q, got %q", "Token", recorder.Sent[0].Type)
	}

	var payload map[string]string
	if err := json.Unmarshal(recorder.Sent[0].Data, &payload); err != nil {
		t.Fatalf("unmarshal sent data: %v", err)
	}
	if payload["token"] != "abc123" {
		t.Errorf("expected token %q, got %q", "abc123", payload["token"])
	}
}

func TestChannel_Receive_ReturnsMatchingType(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	// Enqueue a message of the type we're waiting for.
	recorder.EnqueueIncoming("Approval", map[string]bool{"approved": true})

	data, err := ch.Receive(ctx, "Approval")
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	var payload map[string]bool
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal received data: %v", err)
	}
	if !payload["approved"] {
		t.Error("expected approved=true")
	}
}

func TestChannel_Receive_BuffersNonMatchingMessages(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	// Enqueue 3 different types. We want "type3".
	recorder.EnqueueIncoming("type1", map[string]int{"v": 1})
	recorder.EnqueueIncoming("type2", map[string]int{"v": 2})
	recorder.EnqueueIncoming("type3", map[string]int{"v": 3})

	data, err := ch.Receive(ctx, "type3")
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	var payload map[string]int
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["v"] != 3 {
		t.Errorf("expected v=3, got v=%d", payload["v"])
	}

	// Now type1 and type2 should be buffered. Receive them in order.
	data1, err := ch.Receive(ctx, "type1")
	if err != nil {
		t.Fatalf("Receive type1 failed: %v", err)
	}
	var p1 map[string]int
	json.Unmarshal(data1, &p1)
	if p1["v"] != 1 {
		t.Errorf("expected buffered type1 v=1, got v=%d", p1["v"])
	}

	data2, err := ch.Receive(ctx, "type2")
	if err != nil {
		t.Fatalf("Receive type2 failed: %v", err)
	}
	var p2 map[string]int
	json.Unmarshal(data2, &p2)
	if p2["v"] != 2 {
		t.Errorf("expected buffered type2 v=2, got v=%d", p2["v"])
	}
}

func TestChannel_Receive_ChecksBufferFirst(t *testing.T) {
	recorder := NewTestRecorder()
	incoming, cleanup, _ := recorder.Subscribe("test-ch", "sub-1")
	defer cleanup()

	ch := NewChannel("test", "job-1", 0, 0, false, recorder, incoming, cleanup)
	ctx := WithChannel(context.Background(), ch)

	// Pre-populate the buffer directly.
	bufferedData, _ := json.Marshal(map[string]string{"msg": "from-buffer"})
	ch.mu.Lock()
	ch.buffers["foo"] = []Envelope{
		{Type: "foo", Data: json.RawMessage(bufferedData)},
	}
	ch.mu.Unlock()

	// Receive should return the buffered message without touching incoming.
	data, err := ch.Receive(ctx, "foo")
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["msg"] != "from-buffer" {
		t.Errorf("expected msg=%q, got %q", "from-buffer", payload["msg"])
	}

	// Buffer should now be empty for "foo".
	ch.mu.Lock()
	remaining := len(ch.buffers["foo"])
	ch.mu.Unlock()
	if remaining != 0 {
		t.Errorf("expected buffer to be empty after pop, got %d items", remaining)
	}
}

func TestChannel_Receive_ContextCancellation(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	// Create a cancellable context.
	cancelCtx, cancel := context.WithCancel(ctx)

	// Cancel immediately.
	cancel()

	_, err := ch.Receive(cancelCtx, "WillNeverArrive")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestChannel_Receive_ContextTimeout(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_, err := ch.Receive(timeoutCtx, "WillNeverArrive")
	if err == nil {
		t.Fatal("expected error from timed out context, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestChannel_ReceiveAny_ReturnsFirstMessage(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	recorder.EnqueueIncoming("Greeting", map[string]string{"hello": "world"})

	typeName, data, err := ch.ReceiveAny(ctx)
	if err != nil {
		t.Fatalf("ReceiveAny failed: %v", err)
	}
	if typeName != "Greeting" {
		t.Errorf("expected type %q, got %q", "Greeting", typeName)
	}

	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["hello"] != "world" {
		t.Errorf("expected hello=%q, got %q", "world", payload["hello"])
	}
}

func TestChannel_ReceiveAny_ReturnsBufferedFirst(t *testing.T) {
	recorder := NewTestRecorder()
	incoming, cleanup, _ := recorder.Subscribe("test-ch", "sub-1")
	defer cleanup()

	ch := NewChannel("test", "job-1", 0, 0, false, recorder, incoming, cleanup)
	ctx := WithChannel(context.Background(), ch)

	// Pre-populate the buffer.
	bufferedData, _ := json.Marshal(map[string]string{"src": "buffer"})
	ch.mu.Lock()
	ch.buffers["buffered-type"] = []Envelope{
		{Type: "buffered-type", Data: json.RawMessage(bufferedData)},
	}
	ch.mu.Unlock()

	typeName, data, err := ch.ReceiveAny(ctx)
	if err != nil {
		t.Fatalf("ReceiveAny failed: %v", err)
	}
	if typeName != "buffered-type" {
		t.Errorf("expected type %q, got %q", "buffered-type", typeName)
	}

	var payload map[string]string
	json.Unmarshal(data, &payload)
	if payload["src"] != "buffer" {
		t.Errorf("expected src=%q, got %q", "buffer", payload["src"])
	}
}

func TestChannel_ReceiveAny_ContextCancellation(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	_, _, err := ch.ReceiveAny(cancelCtx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestChannel_channelID_Scoped(t *testing.T) {
	recorder := NewTestRecorder()
	incoming, cleanup, _ := recorder.Subscribe("ch", "sub")
	defer cleanup()

	ch := NewChannel("approval", "job-42", 100, 200, false, recorder, incoming, cleanup)
	got := ch.channelID()
	want := "approval_100_job-42"
	if got != want {
		t.Errorf("channelID: got %q, want %q", got, want)
	}
}

func TestChannel_channelID_Public(t *testing.T) {
	recorder := NewTestRecorder()
	incoming, cleanup, _ := recorder.Subscribe("ch", "sub")
	defer cleanup()

	ch := NewChannel("demo", "job-99", 0, 0, true, recorder, incoming, cleanup)
	got := ch.channelID()
	want := "demo_public_job-99"
	if got != want {
		t.Errorf("channelID: got %q, want %q", got, want)
	}
}

func TestChannel_Cleanup(t *testing.T) {
	cleanupCalled := false
	recorder := NewTestRecorder()
	incoming, _, _ := recorder.Subscribe("ch", "sub")

	ch := NewChannel("test", "job-1", 0, 0, false, recorder, incoming, func() {
		cleanupCalled = true
	})

	ch.Cleanup()
	if !cleanupCalled {
		t.Error("expected cleanup function to be called")
	}
}

func TestChannel_Name(t *testing.T) {
	recorder := NewTestRecorder()
	incoming, cleanup, _ := recorder.Subscribe("ch", "sub")
	defer cleanup()

	ch := NewChannel("my-channel", "job-1", 0, 0, false, recorder, incoming, cleanup)
	if ch.Name() != "my-channel" {
		t.Errorf("Name: got %q, want %q", ch.Name(), "my-channel")
	}
}

func TestChannel_JobID(t *testing.T) {
	recorder := NewTestRecorder()
	incoming, cleanup, _ := recorder.Subscribe("ch", "sub")
	defer cleanup()

	ch := NewChannel("test", "job-xyz", 0, 0, false, recorder, incoming, cleanup)
	if ch.JobID() != "job-xyz" {
		t.Errorf("JobID: got %q, want %q", ch.JobID(), "job-xyz")
	}
}

func TestChannel_Send_MultipleTimes(t *testing.T) {
	recorder := NewTestRecorder()
	ctx := WithTestChannel(context.Background(), recorder)
	ch := FromContext(ctx)

	for i := 0; i < 5; i++ {
		data, _ := json.Marshal(map[string]int{"i": i})
		if err := ch.Send(ctx, "Counter", data); err != nil {
			t.Fatalf("Send %d failed: %v", i, err)
		}
	}

	if len(recorder.Sent) != 5 {
		t.Fatalf("expected 5 sent messages, got %d", len(recorder.Sent))
	}
	for i, env := range recorder.Sent {
		if env.Type != "Counter" {
			t.Errorf("sent[%d] type: got %q, want %q", i, env.Type, "Counter")
		}
	}
}

func TestChannel_Receive_MultipleBuffered(t *testing.T) {
	recorder := NewTestRecorder()
	incoming, cleanup, _ := recorder.Subscribe("ch", "sub")
	defer cleanup()

	ch := NewChannel("test", "job-1", 0, 0, false, recorder, incoming, cleanup)
	ctx := WithChannel(context.Background(), ch)

	// Pre-populate the buffer with multiple messages of the same type.
	for i := 0; i < 3; i++ {
		data, _ := json.Marshal(map[string]int{"i": i})
		ch.mu.Lock()
		ch.buffers["multi"] = append(ch.buffers["multi"], Envelope{
			Type: "multi",
			Data: json.RawMessage(data),
		})
		ch.mu.Unlock()
	}

	// Each Receive should pop the next one in FIFO order.
	for i := 0; i < 3; i++ {
		data, err := ch.Receive(ctx, "multi")
		if err != nil {
			t.Fatalf("Receive %d failed: %v", i, err)
		}
		var payload map[string]int
		json.Unmarshal(data, &payload)
		if payload["i"] != i {
			t.Errorf("Receive %d: expected i=%d, got i=%d", i, i, payload["i"])
		}
	}
}

func TestChannel_Receive_ClosedIncoming(t *testing.T) {
	recorder := NewTestRecorder()
	// Create our own incoming channel so we can close it.
	incoming := make(chan []byte)
	close(incoming)

	ch := NewChannel("test", "job-1", 0, 0, false, recorder, incoming, func() {})
	ctx := WithChannel(context.Background(), ch)

	_, err := ch.Receive(ctx, "SomeType")
	if err == nil {
		t.Fatal("expected error when incoming channel is closed")
	}
}
