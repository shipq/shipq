package channel

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMockQueue_ImplementsTaskQueue(t *testing.T) {
	// Compile-time check is already in mock_queue.go via:
	//   var _ TaskQueue = (*MockQueue)(nil)
	// This test verifies the constructor returns a usable instance.
	mq := NewMockQueue()
	if mq == nil {
		t.Fatal("NewMockQueue returned nil")
	}
}

func TestMockQueue_RegisterAndSend(t *testing.T) {
	mq := NewMockQueue()

	var received string
	err := mq.RegisterTask("greet", func(payload string) error {
		received = payload
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	err = mq.SendTask("greet", `{"name":"Alice"}`, TaskOptions{})
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}

	if received != `{"name":"Alice"}` {
		t.Errorf("handler received %q, want %q", received, `{"name":"Alice"}`)
	}
}

func TestMockQueue_UnregisteredTask_Errors(t *testing.T) {
	mq := NewMockQueue()

	err := mq.SendTask("nonexistent", `{}`, TaskOptions{})
	if err == nil {
		t.Fatal("expected error for unregistered task, got nil")
	}
}

func TestMockQueue_SendTask_PropagatesHandlerError(t *testing.T) {
	mq := NewMockQueue()

	handlerErr := errors.New("something went wrong")
	_ = mq.RegisterTask("failing", func(payload string) error {
		return handlerErr
	})

	err := mq.SendTask("failing", `{}`, TaskOptions{})
	if err == nil {
		t.Fatal("expected error from handler, got nil")
	}
	if !errors.Is(err, handlerErr) {
		t.Errorf("expected handler error, got %v", err)
	}
}

func TestMockQueue_RegisterTask_OverwritesExisting(t *testing.T) {
	mq := NewMockQueue()

	var callCount int
	_ = mq.RegisterTask("task", func(payload string) error {
		callCount = 1
		return nil
	})
	_ = mq.RegisterTask("task", func(payload string) error {
		callCount = 2
		return nil
	})

	_ = mq.SendTask("task", `{}`, TaskOptions{})
	if callCount != 2 {
		t.Errorf("expected second handler to be called (callCount=2), got callCount=%d", callCount)
	}
}

func TestMockQueue_SendTask_MultipleCallsExecuteSequentially(t *testing.T) {
	mq := NewMockQueue()

	var order []int
	_ = mq.RegisterTask("append", func(payload string) error {
		switch payload {
		case "1":
			order = append(order, 1)
		case "2":
			order = append(order, 2)
		case "3":
			order = append(order, 3)
		}
		return nil
	})

	for _, p := range []string{"1", "2", "3"} {
		if err := mq.SendTask("append", p, TaskOptions{}); err != nil {
			t.Fatalf("SendTask(%q): %v", p, err)
		}
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(order))
	}
	for i, v := range order {
		if v != i+1 {
			t.Errorf("order[%d] = %d, want %d", i, v, i+1)
		}
	}
}

func TestMockQueue_StartWorker_BlocksUntilContextDone(t *testing.T) {
	mq := NewMockQueue()

	ctx, cancel := context.WithCancel(context.Background())

	var done int32
	go func() {
		_ = mq.StartWorker(ctx, "test-worker", 1)
		atomic.StoreInt32(&done, 1)
	}()

	// Give the goroutine a moment to start.
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&done) != 0 {
		t.Error("StartWorker should block until context is cancelled")
	}

	cancel()

	// Wait for the goroutine to finish.
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&done) != 1 {
		t.Error("StartWorker should have returned after context cancellation")
	}
}

func TestMockQueue_StopWorker_IsNoop(t *testing.T) {
	mq := NewMockQueue()
	err := mq.StopWorker()
	if err != nil {
		t.Errorf("StopWorker: expected nil error, got %v", err)
	}
}

func TestMockQueue_SendTask_IgnoresTaskOptions(t *testing.T) {
	mq := NewMockQueue()

	var called bool
	_ = mq.RegisterTask("task", func(payload string) error {
		called = true
		return nil
	})

	// TaskOptions should be accepted but ignored by MockQueue.
	err := mq.SendTask("task", `{}`, TaskOptions{
		RetryCount:    5,
		RetryTimeoutS: 30,
	})
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}
	if !called {
		t.Error("handler should have been called")
	}
}

func TestMockQueue_MultipleTaskTypes(t *testing.T) {
	mq := NewMockQueue()

	var taskACalled, taskBCalled bool

	_ = mq.RegisterTask("taskA", func(payload string) error {
		taskACalled = true
		return nil
	})
	_ = mq.RegisterTask("taskB", func(payload string) error {
		taskBCalled = true
		return nil
	})

	if err := mq.SendTask("taskA", `{}`, TaskOptions{}); err != nil {
		t.Fatalf("SendTask A: %v", err)
	}
	if !taskACalled {
		t.Error("taskA handler should have been called")
	}
	if taskBCalled {
		t.Error("taskB handler should NOT have been called")
	}

	if err := mq.SendTask("taskB", `{}`, TaskOptions{}); err != nil {
		t.Fatalf("SendTask B: %v", err)
	}
	if !taskBCalled {
		t.Error("taskB handler should have been called")
	}
}
