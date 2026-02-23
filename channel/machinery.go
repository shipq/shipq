package channel

import (
	"context"
	"fmt"
	"strings"
	"sync"

	machinery "github.com/RichardKnop/machinery/v2"
	redisbackend "github.com/RichardKnop/machinery/v2/backends/redis"
	redisbroker "github.com/RichardKnop/machinery/v2/brokers/redis"
	"github.com/RichardKnop/machinery/v2/config"
	eagerlock "github.com/RichardKnop/machinery/v2/locks/eager"
	"github.com/RichardKnop/machinery/v2/tasks"
)

// MachineryQueue is the default concrete implementation of TaskQueue for Machinery v2.
// It is instantiated by generated startup code (cmd/worker/main.go); user handler code
// never references it directly -- it only sees the TaskQueue interface.
//
// [L5] go.mod bloat note: Running `go get github.com/RichardKnop/machinery/v2` adds
// indirect go.mod/go.sum entries for GCP PubSub, AWS SQS/DynamoDB, MongoDB, AMQP, etc.
// This is cosmetic — we only import the Redis broker/backend packages
// (v2/brokers/redis, v2/backends/redis), so the Go compiler never compiles any
// AMQP/SQS/GCP/MongoDB code into the binary. The extra go.mod entries exist because
// all broker/backend packages live in the same Go module. No action needed.
type MachineryQueue struct {
	server *machinery.Server
	worker *machinery.Worker
	mu     sync.Mutex
}

// Compile-time interface check.
var _ TaskQueue = (*MachineryQueue)(nil)

// NewMachineryQueue creates a new MachineryQueue backed by Redis at the given address.
//
// [L2] config.Config has Broker and ResultBackend string fields. They are used for
// logging only (e.g., "worker.go:59 - Broker: redis://localhost:6379"). The actual
// Redis connection is established by redisbroker.NewGR(cnf, addrs, db). We set these
// fields for readable log output.
func NewMachineryQueue(redisAddr string) (*MachineryQueue, error) {
	// Normalize: accept both "redis://host:port" and bare "host:port".
	// Strip the scheme so NewGR always receives a bare address, then
	// re-add it for the Config fields (used for logging only).
	redisAddr = strings.TrimPrefix(redisAddr, "redis://")

	cnf := &config.Config{
		Broker:          "redis://" + redisAddr, // used for logging only; actual connection via NewGR
		ResultBackend:   "redis://" + redisAddr, // used for logging only; actual connection via NewGR
		DefaultQueue:    "shipq_tasks",
		ResultsExpireIn: 86400,
		NoUnixSignals:   true, // important: disables built-in SIGINT/SIGTERM so we control shutdown via context
		Redis: &config.RedisConfig{
			MaxIdle:                3,
			IdleTimeout:            240,
			ReadTimeout:            15,
			WriteTimeout:           15,
			ConnectTimeout:         15,
			NormalTasksPollPeriod:  1000,
			DelayedTasksPollPeriod: 500,
		},
	}

	broker := redisbroker.NewGR(cnf, []string{redisAddr}, 0)
	backend := redisbackend.NewGR(cnf, []string{redisAddr}, 0)
	lock := eagerlock.New()
	server := machinery.NewServer(cnf, broker, backend, lock)

	return &MachineryQueue{
		server: server,
	}, nil
}

// Config returns the internal Machinery config for inspection (e.g., in tests).
func (mq *MachineryQueue) Config() *config.Config {
	return mq.server.GetConfig()
}

// RegisterTask registers a named task handler function. Machinery validates the
// function signature via reflection at registration time.
func (mq *MachineryQueue) RegisterTask(name string, handler func(string) error) error {
	return mq.server.RegisterTask(name, handler)
}

// SendTask enqueues a task for async execution. Machinery's built-in Fibonacci
// backoff handles retry scheduling. The AsyncResult is discarded because task
// outcomes are tracked in job_results, not via Machinery's result backend.
func (mq *MachineryQueue) SendTask(name string, payload string, opts TaskOptions) error {
	sig := &tasks.Signature{
		Name: name,
		Args: []tasks.Arg{
			{Type: "string", Value: payload},
		},
		RetryCount:   opts.RetryCount,
		RetryTimeout: opts.RetryTimeoutS,
	}

	_, err := mq.server.SendTask(sig)
	if err != nil {
		return fmt.Errorf("machinery send task %q: %w", name, err)
	}
	// [L4] asyncResult.Get() returns 0 values for func(string) error handlers.
	// We don't need the result since task outcomes are tracked in job_results.
	return nil
}

// StartWorker begins consuming tasks. It blocks until ctx is cancelled or
// StopWorker is called. The tag identifies this worker instance. Concurrency
// controls the number of parallel task goroutines.
//
// NoUnixSignals is set to true in the config so we control shutdown via
// context cancellation, not OS signals.
func (mq *MachineryQueue) StartWorker(ctx context.Context, tag string, concurrency int) error {
	mq.mu.Lock()
	w := mq.server.NewWorker(tag, concurrency)
	mq.worker = w
	mq.mu.Unlock()

	// LaunchAsync starts consuming in background goroutines and returns
	// a channel that receives any fatal errors.
	errorsChan := make(chan error, 1)
	w.LaunchAsync(errorsChan)

	// Wait for either context cancellation or a fatal worker error.
	select {
	case <-ctx.Done():
		w.Quit()
		return ctx.Err()
	case err := <-errorsChan:
		return fmt.Errorf("machinery worker error: %w", err)
	}
}

// StopWorker signals the worker to finish in-flight tasks and stop consuming.
// Calls worker.Quit() which internally calls broker.StopConsuming() and waits
// for in-flight tasks to complete.
func (mq *MachineryQueue) StopWorker() error {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if mq.worker != nil {
		mq.worker.Quit()
		mq.worker = nil
	}
	return nil
}
