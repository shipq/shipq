//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	machinery "github.com/RichardKnop/machinery/v2"
	redisbackend "github.com/RichardKnop/machinery/v2/backends/redis"
	redisbroker "github.com/RichardKnop/machinery/v2/brokers/redis"
	"github.com/RichardKnop/machinery/v2/config"
	eagerlock "github.com/RichardKnop/machinery/v2/locks/eager"
	"github.com/RichardKnop/machinery/v2/tasks"
	centrifuge "github.com/centrifugal/centrifuge-go"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Constants -- must match db/databases/centrifugo.json
// ---------------------------------------------------------------------------

const (
	testRedisAddr      = "localhost:6379"
	testCentrifugoAPI  = "http://localhost:8100/api"
	testCentrifugoWS   = "ws://localhost:8100/connection/websocket"
	testCentrifugoKey  = "test-api-key"
	testCentrifugoHMAC = "test-hmac-secret-that-is-at-least-32-chars-long"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// pingRedis dials TCP to Redis, sends PING, expects +PONG.
func pingRedis(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("cannot reach Redis at %s: %w", addr, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return fmt.Errorf("redis PING write: %w", err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("redis PING read: %w", err)
	}
	resp := string(buf[:n])
	if resp != "+PONG\r\n" {
		return fmt.Errorf("unexpected redis response: %q", resp)
	}
	return nil
}

// pingCentrifugo checks Centrifugo liveness via the /api/info endpoint.
func pingCentrifugo(apiURL, apiKey string) error {
	body := []byte(`{}`)
	req, err := http.NewRequest("POST", apiURL+"/info", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach Centrifugo at %s: %w", apiURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("centrifugo /api/info returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// centrifugoPublish publishes a message to a Centrifugo channel via the HTTP API.
func centrifugoPublish(apiURL, apiKey, channel string, data []byte) error {
	payload := map[string]interface{}{
		"channel": channel,
		"data":    json.RawMessage(data),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", apiURL+"/publish", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("centrifugo publish: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("centrifugo publish returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// centrifugoConnectionJWT signs a Centrifugo connection JWT (HMAC-SHA256).
func centrifugoConnectionJWT(sub, hmacSecret string, ttl time.Duration) (string, error) {
	claims := jwtv5.MapClaims{
		"sub": sub,
		"exp": jwtv5.NewNumericDate(time.Now().Add(ttl)),
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString([]byte(hmacSecret))
}

// centrifugoSubscriptionJWT signs a Centrifugo subscription JWT (HMAC-SHA256).
func centrifugoSubscriptionJWT(sub, channel, hmacSecret string, ttl time.Duration) (string, error) {
	claims := jwtv5.MapClaims{
		"sub":     sub,
		"channel": channel,
		"exp":     jwtv5.NewNumericDate(time.Now().Add(ttl)),
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString([]byte(hmacSecret))
}

// connectCentrifugo creates and connects a centrifuge-go JSON client.
func connectCentrifugo(t *testing.T, wsURL, connToken string) *centrifuge.Client {
	t.Helper()
	client := centrifuge.NewJsonClient(wsURL, centrifuge.Config{Token: connToken})
	if err := client.Connect(); err != nil {
		t.Fatalf("centrifuge connect: %v", err)
	}
	return client
}

// subscribeChannel subscribes to a channel and returns the subscription + a channel for received data.
func subscribeChannel(t *testing.T, client *centrifuge.Client, channel string) (*centrifuge.Subscription, <-chan []byte) {
	t.Helper()
	sub, err := client.NewSubscription(channel)
	if err != nil {
		t.Fatalf("NewSubscription(%q): %v", channel, err)
	}
	dataCh := make(chan []byte, 64)
	sub.OnPublication(func(e centrifuge.PublicationEvent) {
		// Non-blocking send: never block the centrifuge-go read loop.
		select {
		case dataCh <- e.Data:
		default:
		}
	})
	if err := sub.Subscribe(); err != nil {
		t.Fatalf("Subscribe(%q): %v", channel, err)
	}
	return sub, dataCh
}

// skipIfServicesDown skips the test if Redis or Centrifugo are unreachable.
func skipIfServicesDown(t *testing.T) {
	t.Helper()
	if err := pingRedis(testRedisAddr); err != nil {
		t.Skipf("skipping: %v", err)
	}
	if err := pingCentrifugo(testCentrifugoAPI, testCentrifugoKey); err != nil {
		t.Skipf("skipping: %v", err)
	}
}

// newMachineryServer creates a Machinery v2 server connected to the test Redis.
func newMachineryServer(t *testing.T, queue string) *machinery.Server {
	t.Helper()
	cnf := &config.Config{
		Broker:          "redis://localhost:6379",
		ResultBackend:   "redis://localhost:6379",
		DefaultQueue:    queue,
		ResultsExpireIn: 3600,
		NoUnixSignals:   true,
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
	broker := redisbroker.NewGR(cnf, []string{testRedisAddr}, 0)
	backend := redisbackend.NewGR(cnf, []string{testRedisAddr}, 0)
	lock := eagerlock.New()
	server := machinery.NewServer(cnf, broker, backend, lock)
	return server
}

// waitForChan reads from a byte channel with a timeout.
func waitForChan(t *testing.T, ch <-chan []byte, timeout time.Duration) []byte {
	t.Helper()
	select {
	case data := <-ch:
		return data
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for message after %v", timeout)
		return nil
	}
}

// collectMessages reads n messages from a byte channel with a total timeout.
func collectMessages(t *testing.T, ch <-chan []byte, n int, timeout time.Duration) [][]byte {
	t.Helper()
	deadline := time.After(timeout)
	var msgs [][]byte
	for i := 0; i < n; i++ {
		select {
		case data := <-ch:
			msgs = append(msgs, data)
		case <-deadline:
			t.Fatalf("timed out collecting messages: got %d/%d after %v", len(msgs), n, timeout)
			return nil
		}
	}
	return msgs
}

// envelope is the wire format for messages flowing through Centrifugo.
type envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func parseEnvelope(t *testing.T, data []byte) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal envelope %q: %v", string(data), err)
	}
	return env
}

// ---------------------------------------------------------------------------
// 0.3 Test: Machinery v2 task dispatch and execution
// ---------------------------------------------------------------------------

func TestPlumbing_MachineryV2_DispatchAndExecute(t *testing.T) {
	skipIfServicesDown(t)

	server := newMachineryServer(t, "plumbing_test_dispatch")

	resultCh := make(chan string, 1)
	err := server.RegisterTask("test_task", func(payload string) error {
		resultCh <- payload
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	worker := server.NewWorker("test-worker", 1)
	errorsChan := make(chan error, 1)
	worker.LaunchAsync(errorsChan)
	defer worker.Quit()

	sentPayload := `{"msg":"hello"}`
	asyncResult, err := server.SendTask(&tasks.Signature{
		Name: "test_task",
		Args: []tasks.Arg{{Type: "string", Value: sentPayload}},
	})
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}

	// Wait for the handler to receive the payload.
	select {
	case got := <-resultCh:
		if got != sentPayload {
			t.Fatalf("handler received %q, want %q", got, sentPayload)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for task handler")
	}

	// Verify the result via the Machinery result backend.
	results, err := asyncResult.Get(5 * time.Millisecond)
	if err != nil {
		t.Fatalf("asyncResult.Get: %v", err)
	}
	t.Logf("Machinery result backend returned %d values", len(results))
}

// ---------------------------------------------------------------------------
// 0.4 Test: Centrifugo publish and WebSocket subscribe
// ---------------------------------------------------------------------------

func TestPlumbing_Centrifugo_PublishAndSubscribe(t *testing.T) {
	skipIfServicesDown(t)

	connToken, err := centrifugoConnectionJWT("user_1", testCentrifugoHMAC, 5*time.Minute)
	if err != nil {
		t.Fatalf("connection JWT: %v", err)
	}

	client := connectCentrifugo(t, testCentrifugoWS, connToken)
	defer client.Close()

	_, dataCh := subscribeChannel(t, client, "test_pubsub")

	// Brief pause to let the subscription establish on the server side.
	time.Sleep(300 * time.Millisecond)

	payload := []byte(`{"greeting":"hello from server API"}`)
	if err := centrifugoPublish(testCentrifugoAPI, testCentrifugoKey, "test_pubsub", payload); err != nil {
		t.Fatalf("centrifugoPublish: %v", err)
	}

	got := waitForChan(t, dataCh, 10*time.Second)
	if !bytes.Equal(got, payload) {
		t.Fatalf("received %q, want %q", string(got), string(payload))
	}
}

// ---------------------------------------------------------------------------
// 0.5 Test: Centrifugo client-side publish (bidirectional)
// ---------------------------------------------------------------------------

func TestPlumbing_Centrifugo_ClientPublish(t *testing.T) {
	skipIfServicesDown(t)

	tokenA, err := centrifugoConnectionJWT("browser_user", testCentrifugoHMAC, 5*time.Minute)
	if err != nil {
		t.Fatalf("connection JWT A: %v", err)
	}
	tokenB, err := centrifugoConnectionJWT("worker_user", testCentrifugoHMAC, 5*time.Minute)
	if err != nil {
		t.Fatalf("connection JWT B: %v", err)
	}

	clientA := connectCentrifugo(t, testCentrifugoWS, tokenA)
	defer clientA.Close()
	clientB := connectCentrifugo(t, testCentrifugoWS, tokenB)
	defer clientB.Close()

	subA, _ := subscribeChannel(t, clientA, "bidir_test")
	_, dataCh := subscribeChannel(t, clientB, "bidir_test")
	_ = subA // we only need subA for publishing

	// Brief pause to let both subscriptions establish.
	time.Sleep(300 * time.Millisecond)

	payload := []byte(`{"from":"browser","action":"approve"}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := subA.Publish(ctx, payload); err != nil {
		t.Fatalf("client-side Publish: %v", err)
	}

	got := waitForChan(t, dataCh, 10*time.Second)
	if !bytes.Equal(got, payload) {
		t.Fatalf("client B received %q, want %q", string(got), string(payload))
	}
}

// ---------------------------------------------------------------------------
// 0.6 Test: Full pipeline -- Machinery dispatch -> worker -> Centrifugo stream
// ---------------------------------------------------------------------------

func TestPlumbing_FullPipeline_DispatchToStream(t *testing.T) {
	skipIfServicesDown(t)

	server := newMachineryServer(t, "plumbing_test_stream")

	jobID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	channelName := fmt.Sprintf("stream_test_%s", jobID)

	err := server.RegisterTask("stream_test", func(payload string) error {
		ch := fmt.Sprintf("stream_test_%s", payload)
		msgs := []string{
			`{"type":"Token","data":{"content":"Hello"}}`,
			`{"type":"Token","data":{"content":"World"}}`,
			`{"type":"Done","data":{}}`,
		}
		for _, msg := range msgs {
			if pubErr := centrifugoPublish(testCentrifugoAPI, testCentrifugoKey, ch, []byte(msg)); pubErr != nil {
				return fmt.Errorf("publish: %w", pubErr)
			}
			// Small pause between publishes to preserve ordering.
			time.Sleep(50 * time.Millisecond)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	// Connect WebSocket subscriber before dispatching the task.
	connToken, err := centrifugoConnectionJWT("stream_user", testCentrifugoHMAC, 5*time.Minute)
	if err != nil {
		t.Fatalf("connection JWT: %v", err)
	}
	client := connectCentrifugo(t, testCentrifugoWS, connToken)
	defer client.Close()

	_, dataCh := subscribeChannel(t, client, channelName)

	// Brief pause to let the subscription establish.
	time.Sleep(300 * time.Millisecond)

	// Launch worker and dispatch.
	worker := server.NewWorker("stream-worker", 1)
	errorsChan := make(chan error, 1)
	worker.LaunchAsync(errorsChan)
	defer worker.Quit()

	_, err = server.SendTask(&tasks.Signature{
		Name: "stream_test",
		Args: []tasks.Arg{{Type: "string", Value: jobID}},
	})
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}

	// Collect 3 messages.
	msgs := collectMessages(t, dataCh, 3, 30*time.Second)

	expectedTypes := []string{"Token", "Token", "Done"}
	for i, raw := range msgs {
		env := parseEnvelope(t, raw)
		if env.Type != expectedTypes[i] {
			t.Errorf("message %d: type=%q, want %q", i, env.Type, expectedTypes[i])
		}
	}

	// Verify content of Token messages.
	env0 := parseEnvelope(t, msgs[0])
	var content0 map[string]string
	if err := json.Unmarshal(env0.Data, &content0); err != nil {
		t.Fatalf("unmarshal token 0 data: %v", err)
	}
	if content0["content"] != "Hello" {
		t.Errorf("token 0 content=%q, want %q", content0["content"], "Hello")
	}

	env1 := parseEnvelope(t, msgs[1])
	var content1 map[string]string
	if err := json.Unmarshal(env1.Data, &content1); err != nil {
		t.Fatalf("unmarshal token 1 data: %v", err)
	}
	if content1["content"] != "World" {
		t.Errorf("token 1 content=%q, want %q", content1["content"], "World")
	}
}

// ---------------------------------------------------------------------------
// 0.7 Test: Full pipeline -- bidirectional (worker sends, waits, client responds, worker continues)
// ---------------------------------------------------------------------------

func TestPlumbing_FullPipeline_Bidirectional(t *testing.T) {
	skipIfServicesDown(t)

	server := newMachineryServer(t, "plumbing_test_bidir")

	jobID := fmt.Sprintf("bidir_%d", time.Now().UnixNano())
	channelName := fmt.Sprintf("bidir_test_%s", jobID)

	// We need to pass the worker's connection token into the task handler.
	// Generate it here so the closure can capture it.
	workerConnToken, err := centrifugoConnectionJWT("worker_bidir_user", testCentrifugoHMAC, 5*time.Minute)
	if err != nil {
		t.Fatalf("worker connection JWT: %v", err)
	}

	err = server.RegisterTask("bidir_test", func(chName string) error {
		// Worker-side: connect to Centrifugo as a WebSocket client.
		workerClient := centrifuge.NewJsonClient(testCentrifugoWS, centrifuge.Config{Token: workerConnToken})
		if connErr := workerClient.Connect(); connErr != nil {
			return fmt.Errorf("worker centrifuge connect: %w", connErr)
		}
		defer workerClient.Close()

		workerSub, subErr := workerClient.NewSubscription(chName)
		if subErr != nil {
			return fmt.Errorf("worker NewSubscription: %w", subErr)
		}

		incomingCh := make(chan []byte, 64)
		workerSub.OnPublication(func(e centrifuge.PublicationEvent) {
			select {
			case incomingCh <- e.Data:
			default:
			}
		})
		if subErr := workerSub.Subscribe(); subErr != nil {
			return fmt.Errorf("worker Subscribe: %w", subErr)
		}

		// Brief pause to let subscription establish.
		time.Sleep(300 * time.Millisecond)

		// 1) Send "Token" message via HTTP API.
		msg1 := `{"type":"Token","data":{"content":"thinking..."}}`
		if pubErr := centrifugoPublish(testCentrifugoAPI, testCentrifugoKey, chName, []byte(msg1)); pubErr != nil {
			return fmt.Errorf("publish Token: %w", pubErr)
		}
		time.Sleep(50 * time.Millisecond)

		// 2) Send "NeedApproval" message via HTTP API.
		msg2 := `{"type":"NeedApproval","data":{"id":"tc_1"}}`
		if pubErr := centrifugoPublish(testCentrifugoAPI, testCentrifugoKey, chName, []byte(msg2)); pubErr != nil {
			return fmt.Errorf("publish NeedApproval: %w", pubErr)
		}

		// 3) Wait for "Approval" from the client, buffering non-matching types.
		buffered := make(map[string][][]byte)
		timeout := time.After(20 * time.Second)
		gotApproval := false
		for !gotApproval {
			select {
			case raw := <-incomingCh:
				var env envelope
				if jsonErr := json.Unmarshal(raw, &env); jsonErr != nil {
					return fmt.Errorf("unmarshal incoming: %w", jsonErr)
				}
				if env.Type == "Approval" {
					gotApproval = true
				} else {
					buffered[env.Type] = append(buffered[env.Type], raw)
				}
			case <-timeout:
				return fmt.Errorf("worker timed out waiting for Approval")
			}
		}

		// 4) Send "Done" message via HTTP API.
		msg3 := `{"type":"Done","data":{"approved":true}}`
		if pubErr := centrifugoPublish(testCentrifugoAPI, testCentrifugoKey, chName, []byte(msg3)); pubErr != nil {
			return fmt.Errorf("publish Done: %w", pubErr)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("RegisterTask: %v", err)
	}

	// Browser side: connect and subscribe before dispatching the task.
	browserToken, err := centrifugoConnectionJWT("browser_bidir_user", testCentrifugoHMAC, 5*time.Minute)
	if err != nil {
		t.Fatalf("browser connection JWT: %v", err)
	}
	browserClient := connectCentrifugo(t, testCentrifugoWS, browserToken)
	defer browserClient.Close()

	browserSub, browserDataCh := subscribeChannel(t, browserClient, channelName)

	// Brief pause to let browser subscription establish.
	time.Sleep(300 * time.Millisecond)

	// Launch worker and dispatch.
	worker := server.NewWorker("bidir-worker", 1)
	errorsChan := make(chan error, 1)
	worker.LaunchAsync(errorsChan)
	defer worker.Quit()

	_, err = server.SendTask(&tasks.Signature{
		Name: "bidir_test",
		Args: []tasks.Arg{{Type: "string", Value: channelName}},
	})
	if err != nil {
		t.Fatalf("SendTask: %v", err)
	}

	// Browser receives "Token" message.
	msg1Raw := waitForChan(t, browserDataCh, 20*time.Second)
	env1 := parseEnvelope(t, msg1Raw)
	if env1.Type != "Token" {
		t.Fatalf("expected Token, got %q", env1.Type)
	}
	t.Logf("Browser received: %s", string(msg1Raw))

	// Browser receives "NeedApproval" message.
	msg2Raw := waitForChan(t, browserDataCh, 20*time.Second)
	env2 := parseEnvelope(t, msg2Raw)
	if env2.Type != "NeedApproval" {
		t.Fatalf("expected NeedApproval, got %q", env2.Type)
	}
	t.Logf("Browser received: %s", string(msg2Raw))

	// Browser sends "Approval" via client-side publish.
	approvalPayload := []byte(`{"type":"Approval","data":{"id":"tc_1","ok":true}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, pubErr := browserSub.Publish(ctx, approvalPayload); pubErr != nil {
		t.Fatalf("browser client-side Publish: %v", pubErr)
	}

	// Browser receives "Done" message. We may first see our own "Approval"
	// echoed back (Centrifugo delivers client-side publishes to ALL
	// subscribers on the channel, including the publisher). Skip it.
	var msg3Raw []byte
	deadline := time.After(20 * time.Second)
	for {
		select {
		case raw := <-browserDataCh:
			env := parseEnvelope(t, raw)
			if env.Type == "Approval" {
				t.Logf("Browser skipping echoed Approval: %s", string(raw))
				continue
			}
			msg3Raw = raw
		case <-deadline:
			t.Fatal("timed out waiting for Done message")
		}
		break
	}
	env3 := parseEnvelope(t, msg3Raw)
	if env3.Type != "Done" {
		t.Fatalf("expected Done, got %q", env3.Type)
	}

	// Verify the Done message confirms approval was received.
	var doneData map[string]interface{}
	if err := json.Unmarshal(env3.Data, &doneData); err != nil {
		t.Fatalf("unmarshal Done data: %v", err)
	}
	if doneData["approved"] != true {
		t.Fatalf("Done data approved=%v, want true", doneData["approved"])
	}
	t.Logf("Browser received Done: %s", string(msg3Raw))
}

// ---------------------------------------------------------------------------
// 0.8 Test: Centrifugo JWT scoping (subscription token channel enforcement)
// ---------------------------------------------------------------------------

func TestPlumbing_Centrifugo_JWTScoping(t *testing.T) {
	skipIfServicesDown(t)

	userID := "scoping_user"
	connToken, err := centrifugoConnectionJWT(userID, testCentrifugoHMAC, 5*time.Minute)
	if err != nil {
		t.Fatalf("connection JWT: %v", err)
	}

	client := connectCentrifugo(t, testCentrifugoWS, connToken)
	defer client.Close()

	// Generate a subscription JWT for a specific scoped channel.
	allowedChannel := "scoped:account_1_job_123"
	subToken, err := centrifugoSubscriptionJWT(userID, allowedChannel, testCentrifugoHMAC, 5*time.Minute)
	if err != nil {
		t.Fatalf("subscription JWT: %v", err)
	}

	// 1) Subscribe to the allowed channel with the correct subscription token.
	sub1, err := client.NewSubscription(allowedChannel, centrifuge.SubscriptionConfig{
		GetToken: func(e centrifuge.SubscriptionTokenEvent) (string, error) {
			return subToken, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSubscription(%q): %v", allowedChannel, err)
	}
	if err := sub1.Subscribe(); err != nil {
		t.Fatalf("Subscribe(%q): %v", allowedChannel, err)
	}
	t.Logf("Successfully subscribed to %q with correct token", allowedChannel)

	// Unsubscribe from the allowed channel.
	if err := sub1.Unsubscribe(); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	if err := client.RemoveSubscription(sub1); err != nil {
		t.Fatalf("RemoveSubscription: %v", err)
	}

	// 2) Try to subscribe to a DIFFERENT scoped channel using the SAME token.
	//    The token has channel="scoped:account_1_job_123" but we're subscribing
	//    to "scoped:account_2_job_456" -- Centrifugo should reject this.
	wrongChannel := "scoped:account_2_job_456"

	var subErrMu sync.Mutex
	var subError error
	subErrorCh := make(chan struct{}, 1)

	sub2, err := client.NewSubscription(wrongChannel, centrifuge.SubscriptionConfig{
		GetToken: func(e centrifuge.SubscriptionTokenEvent) (string, error) {
			// Return the token that was signed for account_1, not account_2.
			return subToken, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSubscription(%q): %v", wrongChannel, err)
	}

	sub2.OnError(func(e centrifuge.SubscriptionErrorEvent) {
		subErrMu.Lock()
		subError = e.Error
		subErrMu.Unlock()
		select {
		case subErrorCh <- struct{}{}:
		default:
		}
	})

	// Subscribe -- this should fail on the server side.
	// The Subscribe() call itself may or may not return an error depending
	// on timing; the rejection comes asynchronously via OnError.
	_ = sub2.Subscribe()

	// Wait for the error callback or a timeout.
	select {
	case <-subErrorCh:
		subErrMu.Lock()
		t.Logf("Correctly rejected subscription to %q: %v", wrongChannel, subError)
		subErrMu.Unlock()
	case <-time.After(10 * time.Second):
		// Check the subscription state -- if it's not subscribed, the rejection happened.
		state := sub2.State()
		if state == centrifuge.SubStateSubscribed {
			t.Fatal("subscription to wrong channel should have been rejected, but state is Subscribed")
		}
		t.Logf("Subscription to %q was rejected (state=%v) but no OnError callback fired within timeout", wrongChannel, state)
	}
}
