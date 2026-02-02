package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDevLogger tests the development logger's pretty JSON output
func TestDevLogger(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create a custom handler that writes to our buffer
	handler := &PrettyJSONHandler{
		JSONHandler: slog.NewJSONHandler(&buf, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
		writer: &buf,
	}

	// Create the logger with our custom handler
	devLogger := slog.New(handler)

	// Test basic logging
	devLogger.Info("test message", "key", "value")
	output := buf.String()

	// Print the output for debugging
	t.Logf("Raw output: %q", output)

	// Verify the output is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v\nOutput was: %s", err, output)
		return
	}

	// Verify the expected fields
	if result["msg"] != "test message" {
		t.Errorf("Expected message 'test message', got '%v'", result["msg"])
	}
	if result["key"] != "value" {
		t.Errorf("Expected key 'value', got '%v'", result["key"])
	}
	if result["level"] != "INFO" {
		t.Errorf("Expected level 'INFO', got '%v'", result["level"])
	}
}

// TestProdLogger tests the production logger's JSON output
func TestProdLogger(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer
	prodLogger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Test basic logging
	prodLogger.Info("test message", "key", "value")
	output := buf.String()

	// Verify the output is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify the expected fields
	if result["msg"] != "test message" {
		t.Errorf("Expected message 'test message', got '%v'", result["msg"])
	}
	if result["key"] != "value" {
		t.Errorf("Expected key 'value', got '%v'", result["key"])
	}
	if result["level"] != "INFO" {
		t.Errorf("Expected level 'INFO', got '%v'", result["level"])
	}
}

// TestDecorateMiddleware tests the HTTP middleware logging functionality
func TestDecorateMiddleware(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create test cases
	tests := []struct {
		name       string
		path       string
		method     string
		ignoreList []string
		userID     string
		shouldLog  bool
	}{
		{
			name:       "Normal request",
			path:       "/test",
			method:     "GET",
			ignoreList: []string{},
			userID:     "user123",
			shouldLog:  true,
		},
		{
			name:       "Ignored path",
			path:       "/health",
			method:     "GET",
			ignoreList: []string{"/health"},
			userID:     "user123",
			shouldLog:  false,
		},
		{
			name:       "No user ID",
			path:       "/test",
			method:     "POST",
			ignoreList: []string{},
			userID:     "",
			shouldLog:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			// Create request with context
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.userID != "" {
				req = req.WithContext(context.WithValue(req.Context(), UserIDKey, tt.userID))
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create decorated handler
			decorated := Decorate(tt.ignoreList, logger, handler)

			// Serve request
			decorated.ServeHTTP(rr, req)

			// Check if logging occurred
			output := buf.String()
			if tt.shouldLog {
				if output == "" {
					t.Error("Expected logging output, got none")
				}

				// Verify both request_started and request_completed logs
				if !strings.Contains(output, "request_started") {
					t.Error("Expected request_started log, not found")
				}
				if !strings.Contains(output, "request_completed") {
					t.Error("Expected request_completed log, not found")
				}

				// Verify request details
				if !strings.Contains(output, tt.path) {
					t.Errorf("Expected path %s in logs, not found", tt.path)
				}
				if !strings.Contains(output, tt.method) {
					t.Errorf("Expected method %s in logs, not found", tt.method)
				}

				// Verify user ID if present
				if tt.userID != "" && !strings.Contains(output, tt.userID) {
					t.Errorf("Expected user ID %s in logs, not found", tt.userID)
				}
			} else if output != "" {
				t.Error("Expected no logging output, got some")
			}
		})
	}
}

// TestRequestIDUniqueness tests that each request gets a unique ID
func TestRequestIDUniqueness(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	decorated := Decorate([]string{}, logger, handler)

	// Make multiple requests
	requestIDs := make(map[string]bool)
	for i := 0; i < 100; i++ {
		buf.Reset()
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		decorated.ServeHTTP(rr, req)

		// Split the output into individual log entries
		logEntries := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(logEntries) != 2 {
			t.Fatalf("Expected 2 log entries, got %d", len(logEntries))
		}

		// Parse the first log entry (request_started)
		var logEntry map[string]interface{}
		if err := json.Unmarshal([]byte(logEntries[0]), &logEntry); err != nil {
			t.Fatalf("Failed to parse log output: %v", err)
		}

		requestID, ok := logEntry["request_id"].(string)
		if !ok {
			t.Fatal("request_id not found in log output")
		}

		// Check for duplicates
		if requestIDs[requestID] {
			t.Errorf("Duplicate request ID found: %s", requestID)
		}
		requestIDs[requestID] = true
	}
}

// TestDurationLogging tests that request duration is properly logged
func TestDurationLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	// Create a handler that takes some time to execute
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	decorated := Decorate([]string{}, logger, handler)

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	decorated.ServeHTTP(rr, req)

	// Split the output into individual log entries
	logEntries := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(logEntries) != 2 {
		t.Fatalf("Expected 2 log entries, got %d", len(logEntries))
	}

	// Parse the second log entry (request_completed) which contains the duration
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logEntries[1]), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify duration is present and reasonable
	duration, ok := logEntry["duration_ms"].(float64)
	if !ok {
		t.Fatal("duration_ms not found in log output")
	}

	// Duration should be at least 100ms (our sleep time)
	if duration < 100 {
		t.Errorf("Expected duration >= 100ms, got %v", duration)
	}
}
