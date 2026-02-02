package logging

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/shipq/shipq/nanoid"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
)

// PrettyJSONHandler is a custom handler that pretty prints JSON in development
type PrettyJSONHandler struct {
	*slog.JSONHandler
	writer io.Writer
}

func (h *PrettyJSONHandler) Handle(ctx context.Context, r slog.Record) error {
	// Convert the record to a map
	attrs := make(map[string]interface{})
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	// Add time and level
	attrs["time"] = r.Time.Format(time.RFC3339)
	attrs["level"] = r.Level.String()
	attrs["msg"] = r.Message

	// Marshal with indentation
	prettyJSON, err := json.MarshalIndent(attrs, "", "  ")
	if err != nil {
		return err
	}

	// Write to the handler's writer with newline
	_, err = h.writer.Write(append(prettyJSON, '\n'))
	return err
}

// NewPrettyJSONHandler creates a new pretty JSON handler
func newPrettyJSONHandler() *PrettyJSONHandler {
	return &PrettyJSONHandler{
		JSONHandler: slog.NewJSONHandler(os.Stdout, nil),
		writer:      os.Stdout,
	}
}

var ProdLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

var DevLogger = slog.New(newPrettyJSONHandler())

// Decorate wraps an HTTP handler and adds tasteful JSON logging to all requests.
// It ignores requests to the paths in the ignoreList.
func Decorate(ignoreList []string, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slices.Contains(ignoreList, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		requestID := nanoid.New()
		startTime := time.Now()

		// Get user ID from context, will be nil if not present
		var userID *string
		if id, ok := r.Context().Value(UserIDKey).(string); ok {
			userID = &id
		}

		logger.Info("request_started",
			"path", r.URL.Path,
			"method", r.Method,
			"request_id", requestID,
			"user_id", userID,
			"timestamp", startTime,
		)

		next.ServeHTTP(w, r)

		logger.Info("request_completed",
			"path", r.URL.Path,
			"method", r.Method,
			"request_id", requestID,
			"user_id", userID,
			"duration_ms", float64(time.Since(startTime).Nanoseconds())/1e6,
			"timestamp", time.Now(),
		)
	})
}
