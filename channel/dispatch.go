package channel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// UpdateJobFunc is a function that updates the status of a job result row.
// The generated worker code passes in a closure that calls runner.UpdateJobStatus(...),
// keeping the channel runtime library free of raw SQL while preserving its
// independence from codegen output.
//
// Parameters:
//   - publicID: the job's public_id (nanoid)
//   - status: "running", "completed", or "failed"
//   - startedAt: non-nil on transition to "running"
//   - completedAt: non-nil on transition to "completed" or "failed"
//   - errorMessage: non-nil on "failed"
//   - resultPayload: non-nil when the handler returns a result
//   - retryCount: number of retries so far
type UpdateJobFunc func(publicID, status string, startedAt, completedAt, errorMessage, resultPayload *string, retryCount int) error

// sqlDatetimeFormat is the Go time format string for MySQL DATETIME columns.
// MySQL rejects RFC 3339 timestamps ("2006-01-02T15:04:05Z"); it requires
// the space-separated format without a trailing timezone indicator.
const sqlDatetimeFormat = "2006-01-02 15:04:05"

// DispatchOption is a functional option for WrapDispatchHandler.
type DispatchOption func(*dispatchOptions)

// dispatchOptions holds optional configuration for WrapDispatchHandler.
type dispatchOptions struct {
	// SetupFunc, when non-nil, is called before each handler invocation to
	// enrich the context with dependencies (e.g., API clients). This is the
	// convention-based Setup function detected by static analysis.
	SetupFunc func(context.Context) context.Context

	// DB, when non-nil, is injected into the handler context via WithDB.
	// This allows WrapDispatchHandlerWithUpdater callers to provide a *sql.DB
	// without changing the function signature (the db parameter is nil in that path).
	DB *sql.DB
}

// WithSetup returns a DispatchOption that installs a Setup function. The Setup
// function is called before each handler invocation; it receives the context
// (which already contains the Channel) and returns an enriched context.
//
// Usage in generated worker code:
//
//	queue.RegisterTask("llmchat", channel.WrapDispatchHandler(
//	    llmchat.HandleChatRequest, transport, db, "llmchat",
//	    channel.WithSetup(llmchat.Setup),
//	))
func WithSetup(fn func(context.Context) context.Context) DispatchOption {
	return func(o *dispatchOptions) {
		o.SetupFunc = fn
	}
}

// WithDispatchDB returns a DispatchOption that provides a *sql.DB to inject
// into the handler context. This is primarily used with WrapDispatchHandlerWithUpdater,
// which passes nil for the db parameter internally — without this option,
// DBFromContext(ctx) would return nil inside Setup and handler functions.
//
// Usage in generated worker code:
//
//	channel.WrapDispatchHandlerWithUpdater(
//	    handler, transport, updateJob, "assistant",
//	    channel.WithSetup(assistant.Setup),
//	    channel.WithDispatchDB(db),
//	)
func WithDispatchDB(db *sql.DB) DispatchOption {
	return func(o *dispatchOptions) {
		o.DB = db
	}
}

// DispatchPayload is the wire format for the JSON payload that the TaskQueue
// delivers to a registered task handler. It contains the job ID, the request
// payload (as raw JSON), and scoping information so the worker can reconstruct
// the correct channel name and ownership context.
type DispatchPayload struct {
	JobID       string          `json:"job_id"`
	ChannelName string          `json:"channel_name"`
	AccountID   int64           `json:"account_id"`
	OrgID       int64           `json:"org_id"`
	IsPublic    bool            `json:"is_public"`
	Request     json.RawMessage `json:"request"`
}

// WrapDispatchHandler wraps a user-defined handler function so it can be
// registered with a TaskQueue via queue.RegisterTask(name, wrappedFn).
//
// Parameters:
//   - handler: the user's handler function. Its signature must be
//     func(context.Context, *DispatchType) error
//     where DispatchType is the channel's dispatch (first FromClient) message type.
//   - transport: a RealtimeTransport (interface) for pub/sub. NOT a concrete type.
//   - db: the *sql.DB connection for updating job_results.
//   - channelName: the registered channel name (e.g., "chatbot").
//
// The returned func(string) error is what gets registered with the TaskQueue:
//
//	queue.RegisterTask("chatbot", channel.WrapDispatchHandler(handler, transport, db, "chatbot"))
//
// Lifecycle of the returned function when invoked by the worker:
//  1. Deserializes the JSON payload (contains job_id, request, scoping info).
//  2. Updates job_results status to "running", sets started_at.
//  3. Computes the scoped channel name (must match token endpoint exactly — see [L3]).
//  4. Creates a Channel struct with a transport subscription.
//  5. Calls the user's handler via reflection.
//  6. On success: updates job_results to "completed", sets completed_at.
//  7. On failure: updates job_results to "failed", sets error_message.
//  8. Calls cleanup to tear down the transport subscription ([L7]: client.Close()).
//
// [L1] Echo handling: The worker's own Send calls are echoed back on the
// incoming channel by the transport (e.g., Centrifugo delivers all publications
// to all subscribers). This is handled by per-type buffering in Channel.Receive —
// the worker only waits for FromClient types, and FromServer echoes are buffered
// as non-matching.
//
// [L4] Retry: WrapDispatchHandler does NOT implement its own retry logic. It
// returns an error, and the TaskQueue re-enqueues the task if configured
// (e.g., Machinery Fibonacci backoff via TaskOptions.RetryCount).
// WrapDispatchHandlerWithUpdater is the preferred version of WrapDispatchHandler
// that accepts an UpdateJobFunc instead of using raw SQL for job status updates.
// This makes the channel runtime library portable across database dialects.
//
// The generated worker code should use this function, passing a closure that
// calls runner.UpdateJobStatus(...) from the generated query runner.
func WrapDispatchHandlerWithUpdater(handler any, transport RealtimeTransport, updateJob UpdateJobFunc, channelName string, opts ...DispatchOption) func(string) error {
	return wrapDispatchHandlerInternal(handler, transport, nil, updateJob, channelName, opts...)
}

// WrapDispatchHandler wraps a user-defined handler function so it can be
// registered with a TaskQueue via queue.RegisterTask(name, wrappedFn).
//
// Deprecated: Use WrapDispatchHandlerWithUpdater instead. This function uses
// raw SQL with ? placeholders which only works on SQLite and MySQL, not PostgreSQL.
func WrapDispatchHandler(handler any, transport RealtimeTransport, db *sql.DB, channelName string, opts ...DispatchOption) func(string) error {
	return wrapDispatchHandlerInternal(handler, transport, db, nil, channelName, opts...)
}

func wrapDispatchHandlerInternal(handler any, transport RealtimeTransport, db *sql.DB, updateJob UpdateJobFunc, channelName string, opts ...DispatchOption) func(string) error {
	// Apply functional options.
	var options dispatchOptions
	for _, o := range opts {
		o(&options)
	}

	// Validate handler signature at registration time (fail fast).
	handlerVal, reqType := validateHandlerSignature(handler, channelName)

	return func(payload string) error {
		// 1. Deserialize the dispatch payload.
		var dp DispatchPayload
		if err := json.Unmarshal([]byte(payload), &dp); err != nil {
			return fmt.Errorf("channel.WrapDispatchHandler(%s): unmarshal payload: %w", channelName, err)
		}

		// Build the update function: prefer the injected UpdateJobFunc; fall
		// back to the legacy raw-SQL helper when db is provided.
		doUpdate := updateJob
		if doUpdate == nil && db != nil {
			doUpdate = func(publicID, status string, startedAt, completedAt, errorMessage, resultPayload *string, retryCount int) error {
				return updateJobStatus(db, publicID, status, startedAt, completedAt, errorMessage, resultPayload, retryCount)
			}
		}
		if doUpdate == nil {
			return fmt.Errorf("channel.WrapDispatchHandler(%s): no update function or db provided", channelName)
		}

		// 2. Update job_results status to "running", set started_at.
		now := time.Now().UTC()
		nowStr := now.Format(sqlDatetimeFormat)
		if err := doUpdate(dp.JobID, "running", &nowStr, nil, nil, nil, 0); err != nil {
			return fmt.Errorf("channel.WrapDispatchHandler(%s): update job to running: %w", channelName, err)
		}

		// 3. Compute the scoped channel name.
		// This MUST exactly match what the token endpoint produces ([L3]).
		scopedName := computeChannelID(dp.ChannelName, dp.AccountID, dp.JobID, dp.IsPublic)

		// 4. Subscribe to the channel via the transport.
		// [L6]: Subscribe uses a buffered channel (capacity 64+) with non-blocking
		// sends in OnPublication to avoid deadlocking the transport read loop.
		workerSubID := fmt.Sprintf("worker_%s_%s", channelName, dp.JobID)
		incoming, cleanup, err := transport.Subscribe(scopedName, workerSubID)
		if err != nil {
			// Mark job as failed if we can't even subscribe.
			errMsg := fmt.Sprintf("subscribe failed: %v", err)
			_ = doUpdate(dp.JobID, "failed", nil, nil, &errMsg, nil, 0)
			return fmt.Errorf("channel.WrapDispatchHandler(%s): subscribe: %w", channelName, err)
		}
		// [L7]: cleanup calls client.Close() (terminal state, no reconnection).
		defer cleanup()

		// 5. Create the Channel struct and inject into context.
		ch := NewChannel(
			dp.ChannelName,
			dp.JobID,
			dp.AccountID,
			dp.OrgID,
			dp.IsPublic,
			transport,
			incoming,
			cleanup,
		)

		// Prefer DB from options (set via WithDispatchDB) over the db parameter.
		// WrapDispatchHandlerWithUpdater passes nil for db, so options.DB is the
		// only way to get a valid *sql.DB into the context in that path.
		actualDB := db
		if options.DB != nil {
			actualDB = options.DB
		}

		ctx := context.Background()
		ctx = WithChannel(ctx, ch)
		ctx = WithDB(ctx, actualDB)
		ctx = WithAccountID(ctx, dp.AccountID)
		ctx = WithOrgID(ctx, dp.OrgID)

		// If a Setup function was provided, call it to enrich the context
		// with handler dependencies (e.g., API clients, DB connections).
		if options.SetupFunc != nil {
			ctx = options.SetupFunc(ctx)
		}

		// 6. Deserialize the request into the handler's expected type.
		reqPtr := reflect.New(reqType)
		if err := json.Unmarshal(dp.Request, reqPtr.Interface()); err != nil {
			errMsg := fmt.Sprintf("unmarshal request: %v", err)
			_ = doUpdate(dp.JobID, "failed", nil, nil, &errMsg, nil, 0)
			return fmt.Errorf("channel.WrapDispatchHandler(%s): unmarshal request: %w", channelName, err)
		}

		// 7. Call the user's handler function via reflection.
		results := handlerVal.Call([]reflect.Value{
			reflect.ValueOf(ctx),
			reqPtr,
		})

		// 8. Process the result.
		completedAt := time.Now().UTC().Format(sqlDatetimeFormat)
		errIface := results[0].Interface()
		if errIface != nil {
			handlerErr := errIface.(error)
			errMsg := handlerErr.Error()
			_ = doUpdate(dp.JobID, "failed", nil, &completedAt, &errMsg, nil, 0)
			return handlerErr
		}

		// Success: update job_results to "completed".
		if err := doUpdate(dp.JobID, "completed", nil, &completedAt, nil, nil, 0); err != nil {
			return fmt.Errorf("channel.WrapDispatchHandler(%s): update job to completed: %w", channelName, err)
		}

		return nil
	}
}

// ComputeChannelID computes the transport-level channel name from the channel
// name, account ID, job ID, and public flag. This is exported so the HTTP
// token endpoint can use the exact same logic ([L3] critical).
//
// Format matches Channel.channelID() in context.go:
//   - Public:  "<name>_public_<jobID>"
//   - Scoped:  "<name>_<accountID>_<jobID>"
func ComputeChannelID(name string, accountID int64, jobID string, isPublic bool) string {
	return computeChannelID(name, accountID, jobID, isPublic)
}

// computeChannelID is the internal implementation.
func computeChannelID(name string, accountID int64, jobID string, isPublic bool) string {
	if isPublic {
		return name + "_public_" + jobID
	}
	return fmt.Sprintf("%s_%d_%s", name, accountID, jobID)
}

// validateHandlerSignature checks that handler has the signature
// func(context.Context, *T) error and returns the reflect.Value of the handler
// and the reflect.Type of T (not *T). Panics on invalid signatures so
// registration-time errors are caught immediately.
func validateHandlerSignature(handler any, channelName string) (reflect.Value, reflect.Type) {
	handlerVal := reflect.ValueOf(handler)
	handlerType := handlerVal.Type()

	if handlerType.Kind() != reflect.Func {
		panic(fmt.Sprintf("channel.WrapDispatchHandler(%s): handler must be a function, got %s", channelName, handlerType.Kind()))
	}

	if handlerType.NumIn() != 2 {
		panic(fmt.Sprintf("channel.WrapDispatchHandler(%s): handler must take 2 arguments (context.Context, *T), got %d", channelName, handlerType.NumIn()))
	}

	// First arg must be context.Context.
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	if !handlerType.In(0).Implements(ctxType) {
		panic(fmt.Sprintf("channel.WrapDispatchHandler(%s): first argument must implement context.Context, got %s", channelName, handlerType.In(0)))
	}

	// Second arg must be a pointer to a struct.
	reqArgType := handlerType.In(1)
	if reqArgType.Kind() != reflect.Ptr || reqArgType.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("channel.WrapDispatchHandler(%s): second argument must be a pointer to a struct (*T), got %s", channelName, reqArgType))
	}

	if handlerType.NumOut() != 1 {
		panic(fmt.Sprintf("channel.WrapDispatchHandler(%s): handler must return exactly 1 value (error), got %d", channelName, handlerType.NumOut()))
	}

	errType := reflect.TypeOf((*error)(nil)).Elem()
	if !handlerType.Out(0).Implements(errType) {
		panic(fmt.Sprintf("channel.WrapDispatchHandler(%s): handler must return error, got %s", channelName, handlerType.Out(0)))
	}

	return handlerVal, reqArgType.Elem()
}

// updateJobStatus updates the job_results row identified by publicID (job_id).
// It uses raw SQL to avoid depending on the generated query runner, since the
// channel runtime library must work independently of codegen output.
//
// Any parameter set to nil is written as NULL. The started_at parameter is only
// written when non-nil (i.e., on the transition to "running").
func updateJobStatus(db *sql.DB, publicID, status string, startedAt, completedAt, errorMessage, resultPayload *string, retryCount int) error {
	query := `UPDATE job_results SET status = ?, started_at = COALESCE(?, started_at), completed_at = ?, error_message = ?, result_payload = ?, retry_count = ?, updated_at = CURRENT_TIMESTAMP WHERE public_id = ?`
	_, err := db.Exec(query, status, startedAt, completedAt, errorMessage, resultPayload, retryCount, publicID)
	if err != nil {
		return fmt.Errorf("updateJobStatus(%s, %s): %w", publicID, status, err)
	}
	return nil
}
