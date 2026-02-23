package channel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	centrifuge "github.com/centrifugal/centrifuge-go"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// CentrifugoTransport is the default concrete implementation of RealtimeTransport
// for Centrifugo v6. It is instantiated by generated startup code (cmd/worker/main.go
// and HTTP routes); user handler code never references it directly -- it only sees
// the RealtimeTransport interface.
type CentrifugoTransport struct {
	apiURL     string // e.g., "http://localhost:8100/api"
	apiKey     string // X-API-Key header value (http_api.key from v6 config)
	hmacSecret string // client.token.hmac_secret_key from v6 config
	wsURL      string // e.g., "ws://localhost:8100/connection/websocket"
}

// Compile-time interface check.
var _ RealtimeTransport = (*CentrifugoTransport)(nil)

// NewCentrifugoTransport creates a new CentrifugoTransport.
//
//   - apiURL: the Centrifugo HTTP API base URL (e.g., "http://localhost:8100/api")
//   - apiKey: the http_api.key value from the Centrifugo v6 config
//   - hmacSecret: the client.token.hmac_secret_key for signing JWTs
//   - wsURL: the WebSocket URL clients connect to (e.g., "ws://localhost:8100/connection/websocket")
func NewCentrifugoTransport(apiURL, apiKey, hmacSecret, wsURL string) *CentrifugoTransport {
	return &CentrifugoTransport{
		apiURL:     apiURL,
		apiKey:     apiKey,
		hmacSecret: hmacSecret,
		wsURL:      wsURL,
	}
}

// Publish sends a message from the server to all subscribers of the channel
// via Centrifugo's HTTP publish API (POST <apiURL>/publish).
// The data parameter should be a JSON-encoded Envelope.
func (ct *CentrifugoTransport) Publish(channel string, data []byte) error {
	payload := map[string]interface{}{
		"channel": channel,
		"data":    json.RawMessage(data),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("centrifugo publish: marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", ct.apiURL+"/publish", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("centrifugo publish: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", ct.apiKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("centrifugo publish: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("centrifugo publish returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// Subscribe creates a worker-side subscription to the channel using a centrifuge-go
// WebSocket client. It returns a Go channel that receives raw message bytes (from
// client publications), a cleanup function to tear down the connection, and any error.
//
// [L6] Critical: The OnPublication callback uses a non-blocking send to a buffered
// Go channel (capacity 64). Blocking inside OnPublication deadlocks the entire
// centrifuge-go read loop -- no messages can be received on ANY subscription for
// that client.
//
// [L7] Cleanup: The cleanup function calls client.Close() (terminal state, no
// reconnection), NOT client.Disconnect() (which allows reconnection).
//
// [L1] Echo note: The worker's subscription will also receive echoes of messages
// the worker published via the HTTP API (Publish method). This is by Centrifugo
// design -- all publications go to all subscribers. The Channel.Receive() layer
// handles this via per-type buffering.
//
// Note: This uses a WebSocket client connection (centrifuge-go), NOT the server-side
// subscribe API (POST /api/subscribe), because the server API only subscribes
// already-connected client sessions -- it cannot create a new backend subscription.
func (ct *CentrifugoTransport) Subscribe(channel string, subscriberID string) (<-chan []byte, func(), error) {
	// Generate a connection JWT for this worker subscriber.
	connToken, err := ct.GenerateConnectionToken(subscriberID, 24*time.Hour)
	if err != nil {
		return nil, nil, fmt.Errorf("centrifugo subscribe: generate connection token: %w", err)
	}

	// Generate a subscription JWT for this specific channel.
	subToken, err := ct.GenerateSubscriptionToken(subscriberID, channel, 24*time.Hour)
	if err != nil {
		return nil, nil, fmt.Errorf("centrifugo subscribe: generate subscription token: %w", err)
	}

	// Create centrifuge-go client.
	client := centrifuge.NewJsonClient(ct.wsURL, centrifuge.Config{
		Token: connToken,
	})

	if err := client.Connect(); err != nil {
		return nil, nil, fmt.Errorf("centrifugo subscribe: connect: %w", err)
	}

	sub, err := client.NewSubscription(channel, centrifuge.SubscriptionConfig{
		Token: subToken,
	})
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("centrifugo subscribe: new subscription: %w", err)
	}

	// [L6] Buffered channel with non-blocking sends -- never block the centrifuge-go read loop.
	dataCh := make(chan []byte, 64)
	sub.OnPublication(func(e centrifuge.PublicationEvent) {
		select {
		case dataCh <- e.Data:
		default: // drop if buffer full -- never block
		}
	})

	if err := sub.Subscribe(); err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("centrifugo subscribe: subscribe: %w", err)
	}

	// [L7] Cleanup calls client.Close() (terminal, no reconnection).
	cleanup := func() {
		client.Close()
	}

	return dataCh, cleanup, nil
}

// GenerateConnectionToken creates a short-lived Centrifugo connection JWT
// signed with HMAC-SHA256. Claims: sub (user ID), exp (expiration).
func (ct *CentrifugoTransport) GenerateConnectionToken(sub string, ttl time.Duration) (string, error) {
	claims := jwtv5.MapClaims{
		"sub": sub,
		"exp": jwtv5.NewNumericDate(time.Now().Add(ttl)),
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString([]byte(ct.hmacSecret))
}

// GenerateSubscriptionToken creates a short-lived Centrifugo subscription JWT
// signed with HMAC-SHA256. Claims: sub (must match connection token), channel
// (must exactly match subscription channel name), exp (expiration).
//
// [L3] Critical: If the channel claim doesn't exactly match the subscribed
// channel name, Centrifugo disconnects the ENTIRE client connection -- not
// just the subscription. Correctness here is critical.
func (ct *CentrifugoTransport) GenerateSubscriptionToken(sub string, channel string, ttl time.Duration) (string, error) {
	claims := jwtv5.MapClaims{
		"sub":     sub,
		"channel": channel,
		"exp":     jwtv5.NewNumericDate(time.Now().Add(ttl)),
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString([]byte(ct.hmacSecret))
}

// ConnectionURL returns the WebSocket URL clients should connect to.
func (ct *CentrifugoTransport) ConnectionURL() string {
	return ct.wsURL
}
