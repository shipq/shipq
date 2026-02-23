package channel

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

const testHMACSecret = "test-hmac-secret-that-is-at-least-32-chars-long"

func TestCentrifugoTransport_ImplementsRealtimeTransport(t *testing.T) {
	// Compile-time check is already in centrifugo.go via:
	//   var _ RealtimeTransport = (*CentrifugoTransport)(nil)
	// This test verifies the constructor returns a non-nil value.
	ct := NewCentrifugoTransport("http://localhost:8100/api", "key", testHMACSecret, "ws://localhost:8100/connection/websocket")
	if ct == nil {
		t.Fatal("NewCentrifugoTransport returned nil")
	}
}

func TestGenerateConnectionToken_ValidJWT(t *testing.T) {
	ct := NewCentrifugoTransport("http://localhost:8100/api", "key", testHMACSecret, "ws://localhost:8100/connection/websocket")

	tokenStr, err := ct.GenerateConnectionToken("user-42", 5*time.Minute)
	if err != nil {
		t.Fatalf("GenerateConnectionToken: %v", err)
	}

	// Parse the token without validation first to inspect claims.
	token, err := jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
		return []byte(testHMACSecret), nil
	})
	if err != nil {
		t.Fatalf("jwt.Parse: %v", err)
	}
	if !token.Valid {
		t.Fatal("token is not valid")
	}

	claims, ok := token.Claims.(jwtv5.MapClaims)
	if !ok {
		t.Fatal("expected MapClaims")
	}

	// Check sub claim.
	sub, err := claims.GetSubject()
	if err != nil {
		t.Fatalf("GetSubject: %v", err)
	}
	if sub != "user-42" {
		t.Errorf("sub: got %q, want %q", sub, "user-42")
	}

	// Check exp claim exists and is in the future.
	exp, err := claims.GetExpirationTime()
	if err != nil {
		t.Fatalf("GetExpirationTime: %v", err)
	}
	if exp == nil {
		t.Fatal("exp claim is missing")
	}
	if exp.Time.Before(time.Now()) {
		t.Error("exp should be in the future")
	}
	// exp should be approximately 5 minutes from now.
	diff := time.Until(exp.Time)
	if diff < 4*time.Minute || diff > 6*time.Minute {
		t.Errorf("exp should be ~5 minutes from now, got %v", diff)
	}

	// Connection token should NOT have a channel claim.
	if _, exists := claims["channel"]; exists {
		t.Error("connection token should not have a channel claim")
	}
}

func TestGenerateSubscriptionToken_ValidJWT(t *testing.T) {
	ct := NewCentrifugoTransport("http://localhost:8100/api", "key", testHMACSecret, "ws://localhost:8100/connection/websocket")

	channelName := "approval_100_job-42"
	tokenStr, err := ct.GenerateSubscriptionToken("user-42", channelName, 10*time.Minute)
	if err != nil {
		t.Fatalf("GenerateSubscriptionToken: %v", err)
	}

	token, err := jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
		return []byte(testHMACSecret), nil
	})
	if err != nil {
		t.Fatalf("jwt.Parse: %v", err)
	}
	if !token.Valid {
		t.Fatal("token is not valid")
	}

	claims, ok := token.Claims.(jwtv5.MapClaims)
	if !ok {
		t.Fatal("expected MapClaims")
	}

	// Check sub claim.
	sub, err := claims.GetSubject()
	if err != nil {
		t.Fatalf("GetSubject: %v", err)
	}
	if sub != "user-42" {
		t.Errorf("sub: got %q, want %q", sub, "user-42")
	}

	// [L3] Check channel claim is the EXACT string passed in.
	// A mismatch disconnects the entire client connection.
	chClaim, ok := claims["channel"].(string)
	if !ok {
		t.Fatal("channel claim missing or not a string")
	}
	if chClaim != channelName {
		t.Errorf("channel claim: got %q, want %q", chClaim, channelName)
	}

	// Check exp claim.
	exp, err := claims.GetExpirationTime()
	if err != nil {
		t.Fatalf("GetExpirationTime: %v", err)
	}
	if exp == nil {
		t.Fatal("exp claim is missing")
	}
	diff := time.Until(exp.Time)
	if diff < 9*time.Minute || diff > 11*time.Minute {
		t.Errorf("exp should be ~10 minutes from now, got %v", diff)
	}
}

func TestGenerateToken_SignedWithHMAC(t *testing.T) {
	ct := NewCentrifugoTransport("http://localhost:8100/api", "key", testHMACSecret, "ws://localhost:8100/connection/websocket")

	tokenStr, err := ct.GenerateConnectionToken("user-1", time.Hour)
	if err != nil {
		t.Fatalf("GenerateConnectionToken: %v", err)
	}

	// Verify with the correct secret succeeds.
	token, err := jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
		// Verify the signing method is HMAC.
		if _, ok := token.Method.(*jwtv5.SigningMethodHMAC); !ok {
			t.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(testHMACSecret), nil
	})
	if err != nil {
		t.Fatalf("Parse with correct secret: %v", err)
	}
	if !token.Valid {
		t.Error("token should be valid with correct secret")
	}

	// Verify with the wrong secret fails.
	_, err = jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
		return []byte("wrong-secret-definitely-not-the-right-one"), nil
	})
	if err == nil {
		t.Error("Parse with wrong secret should fail")
	}
}

func TestGenerateSubscriptionToken_SignedWithHMAC(t *testing.T) {
	ct := NewCentrifugoTransport("http://localhost:8100/api", "key", testHMACSecret, "ws://localhost:8100/connection/websocket")

	tokenStr, err := ct.GenerateSubscriptionToken("user-1", "my-channel", time.Hour)
	if err != nil {
		t.Fatalf("GenerateSubscriptionToken: %v", err)
	}

	// Verify with correct secret.
	token, err := jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwtv5.SigningMethodHMAC); !ok {
			t.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(testHMACSecret), nil
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !token.Valid {
		t.Error("token should be valid")
	}

	// Verify with wrong secret fails.
	_, err = jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
		return []byte("wrong-secret"), nil
	})
	if err == nil {
		t.Error("Parse with wrong secret should fail")
	}
}

func TestCentrifugoTransport_Publish_IncludesAPIKeyHeader(t *testing.T) {
	var (
		gotAPIKey      string
		gotMethod      string
		gotPath        string
		gotBody        []byte
		gotContentType string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	apiKey := "my-test-api-key"
	ct := NewCentrifugoTransport(server.URL, apiKey, testHMACSecret, "ws://localhost/ws")

	// Create envelope data to publish.
	env := Envelope{
		Type: "Token",
		Data: json.RawMessage(`{"token":"abc123"}`),
	}
	if err := ct.Publish("approval_100_job-1", env.Marshal()); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	// Verify request method.
	if gotMethod != "POST" {
		t.Errorf("method: got %q, want %q", gotMethod, "POST")
	}

	// Verify request path.
	if gotPath != "/publish" {
		t.Errorf("path: got %q, want %q", gotPath, "/publish")
	}

	// Verify X-API-Key header.
	if gotAPIKey != apiKey {
		t.Errorf("X-API-Key: got %q, want %q", gotAPIKey, apiKey)
	}

	// Verify Content-Type header.
	if gotContentType != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", gotContentType, "application/json")
	}

	// Verify body is well-formed JSON with channel and data fields.
	var body map[string]json.RawMessage
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}

	// Check "channel" field.
	var channelStr string
	if err := json.Unmarshal(body["channel"], &channelStr); err != nil {
		t.Fatalf("unmarshal channel: %v", err)
	}
	if channelStr != "approval_100_job-1" {
		t.Errorf("body.channel: got %q, want %q", channelStr, "approval_100_job-1")
	}

	// Check "data" field is a valid envelope.
	var envData Envelope
	if err := json.Unmarshal(body["data"], &envData); err != nil {
		t.Fatalf("unmarshal data as envelope: %v", err)
	}
	if envData.Type != "Token" {
		t.Errorf("data.type: got %q, want %q", envData.Type, "Token")
	}
}

func TestCentrifugoTransport_Publish_HandlesNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	ct := NewCentrifugoTransport(server.URL, "key", testHMACSecret, "ws://localhost/ws")

	env := Envelope{Type: "Ping", Data: json.RawMessage(`{}`)}
	err := ct.Publish("test-channel", env.Marshal())
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}

func TestCentrifugoTransport_Publish_HandlesConnectionError(t *testing.T) {
	// Use a URL that will refuse connections.
	ct := NewCentrifugoTransport("http://127.0.0.1:1", "key", testHMACSecret, "ws://localhost/ws")

	env := Envelope{Type: "Ping", Data: json.RawMessage(`{}`)}
	err := ct.Publish("test-channel", env.Marshal())
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}
}

func TestCentrifugoTransport_ConnectionURL(t *testing.T) {
	wsURL := "ws://my-centrifugo:8100/connection/websocket"
	ct := NewCentrifugoTransport("http://localhost:8100/api", "key", testHMACSecret, wsURL)

	if ct.ConnectionURL() != wsURL {
		t.Errorf("ConnectionURL: got %q, want %q", ct.ConnectionURL(), wsURL)
	}
}

func TestCentrifugoTransport_ConnectionURL_DifferentValues(t *testing.T) {
	tests := []string{
		"ws://localhost:8100/connection/websocket",
		"wss://prod.example.com/connection/websocket",
		"ws://10.0.0.1:9090/ws",
	}

	for _, wsURL := range tests {
		ct := NewCentrifugoTransport("http://localhost/api", "key", testHMACSecret, wsURL)
		if ct.ConnectionURL() != wsURL {
			t.Errorf("ConnectionURL: got %q, want %q", ct.ConnectionURL(), wsURL)
		}
	}
}

func TestGenerateConnectionToken_DifferentSubjects(t *testing.T) {
	ct := NewCentrifugoTransport("http://localhost/api", "key", testHMACSecret, "ws://localhost/ws")

	subjects := []string{"user-1", "user-999", "worker-abc", "anonymous"}
	for _, sub := range subjects {
		tokenStr, err := ct.GenerateConnectionToken(sub, time.Hour)
		if err != nil {
			t.Fatalf("GenerateConnectionToken(%q): %v", sub, err)
		}

		token, err := jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
			return []byte(testHMACSecret), nil
		})
		if err != nil {
			t.Fatalf("Parse token for sub=%q: %v", sub, err)
		}

		claims := token.Claims.(jwtv5.MapClaims)
		gotSub, _ := claims.GetSubject()
		if gotSub != sub {
			t.Errorf("sub=%q: got %q", sub, gotSub)
		}
	}
}

func TestGenerateSubscriptionToken_ChannelClaimExactMatch(t *testing.T) {
	ct := NewCentrifugoTransport("http://localhost/api", "key", testHMACSecret, "ws://localhost/ws")

	// [L3] Test various channel name formats to ensure exact match.
	channels := []string{
		"approval_100_job-42",
		"demo_public_job-99",
		"sync_0_abc-def-123",
		"channel_with_many_underscores_1_2_3",
	}

	for _, ch := range channels {
		tokenStr, err := ct.GenerateSubscriptionToken("user-1", ch, time.Hour)
		if err != nil {
			t.Fatalf("GenerateSubscriptionToken(channel=%q): %v", ch, err)
		}

		token, err := jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
			return []byte(testHMACSecret), nil
		})
		if err != nil {
			t.Fatalf("Parse token for channel=%q: %v", ch, err)
		}

		claims := token.Claims.(jwtv5.MapClaims)
		gotChannel, ok := claims["channel"].(string)
		if !ok {
			t.Fatalf("channel=%q: channel claim missing or not string", ch)
		}
		if gotChannel != ch {
			t.Errorf("channel claim mismatch: got %q, want %q (this would disconnect the entire client!)", gotChannel, ch)
		}
	}
}

func TestGenerateConnectionToken_ShortTTL(t *testing.T) {
	ct := NewCentrifugoTransport("http://localhost/api", "key", testHMACSecret, "ws://localhost/ws")

	tokenStr, err := ct.GenerateConnectionToken("user-1", 1*time.Second)
	if err != nil {
		t.Fatalf("GenerateConnectionToken: %v", err)
	}

	token, err := jwtv5.Parse(tokenStr, func(token *jwtv5.Token) (interface{}, error) {
		return []byte(testHMACSecret), nil
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	claims := token.Claims.(jwtv5.MapClaims)
	exp, _ := claims.GetExpirationTime()
	diff := time.Until(exp.Time)
	if diff > 2*time.Second {
		t.Errorf("short TTL token exp too far in future: %v", diff)
	}
}

func TestCentrifugoTransport_Publish_MultiplePublishes(t *testing.T) {
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	ct := NewCentrifugoTransport(server.URL, "key", testHMACSecret, "ws://localhost/ws")

	for i := 0; i < 5; i++ {
		env := Envelope{Type: "Ping", Data: json.RawMessage(`{}`)}
		if err := ct.Publish("test-channel", env.Marshal()); err != nil {
			t.Fatalf("Publish %d: %v", i, err)
		}
	}

	if requestCount != 5 {
		t.Errorf("expected 5 HTTP requests, got %d", requestCount)
	}
}
