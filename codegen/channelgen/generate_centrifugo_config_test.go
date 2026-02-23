package channelgen

import (
	"encoding/json"
	"testing"

	"github.com/shipq/shipq/codegen"
)

func TestGenerateCentrifugoConfig_Namespaces(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "chatbot",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "StartChat", IsDispatch: true},
				{Direction: "server_to_client", TypeName: "BotMessage"},
			},
		},
		{
			Name: "notifications",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "Subscribe", IsDispatch: true},
				{Direction: "server_to_client", TypeName: "Notification"},
			},
		},
	}

	data, err := GenerateCentrifugoConfig(channels, "test-api-key", "test-hmac-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var cfg centrifugoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if len(cfg.Channel.Namespaces) != 2 {
		t.Fatalf("expected 2 namespaces, got %d", len(cfg.Channel.Namespaces))
	}

	expectedNames := map[string]bool{"chatbot": false, "notifications": false}
	for _, ns := range cfg.Channel.Namespaces {
		if _, ok := expectedNames[ns.Name]; !ok {
			t.Errorf("unexpected namespace name: %q", ns.Name)
		}
		expectedNames[ns.Name] = true
	}
	for name, found := range expectedNames {
		if !found {
			t.Errorf("expected namespace %q not found", name)
		}
	}
}

func TestGenerateCentrifugoConfig_BidirectionalPublish(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "interactive",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "StartChat", IsDispatch: true},
				{Direction: "client_to_server", TypeName: "ToolCallApproval", IsDispatch: false}, // mid-stream FromClient
				{Direction: "server_to_client", TypeName: "BotMessage"},
				{Direction: "server_to_client", TypeName: "ToolCallRequest"},
			},
		},
	}

	data, err := GenerateCentrifugoConfig(channels, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var cfg centrifugoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if len(cfg.Channel.Namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(cfg.Channel.Namespaces))
	}

	ns := cfg.Channel.Namespaces[0]
	if ns.Name != "interactive" {
		t.Errorf("expected namespace name %q, got %q", "interactive", ns.Name)
	}
	if !ns.AllowPublishForSubscriber {
		t.Error("expected allow_publish_for_subscriber to be true for bidirectional channel")
	}
	if !ns.AllowSubscribeForClient {
		t.Error("expected allow_subscribe_for_client to be true")
	}
}

func TestGenerateCentrifugoConfig_UnidirectionalNoPublish(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "email_sender",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "SendEmailRequest", IsDispatch: true},
				{Direction: "server_to_client", TypeName: "EmailProgress"},
			},
		},
	}

	data, err := GenerateCentrifugoConfig(channels, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var cfg centrifugoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if len(cfg.Channel.Namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(cfg.Channel.Namespaces))
	}

	ns := cfg.Channel.Namespaces[0]
	if ns.Name != "email_sender" {
		t.Errorf("expected namespace name %q, got %q", "email_sender", ns.Name)
	}
	if ns.AllowPublishForSubscriber {
		t.Error("expected allow_publish_for_subscriber to be false for unidirectional channel")
	}
	if !ns.AllowSubscribeForClient {
		t.Error("expected allow_subscribe_for_client to be true")
	}
}

func TestGenerateCentrifugoConfig_V6Structure(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "test_channel",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "Req", IsDispatch: true},
				{Direction: "server_to_client", TypeName: "Resp"},
			},
		},
	}

	apiKey := "my-api-key-123"
	hmacSecret := "my-hmac-secret-456"

	data, err := GenerateCentrifugoConfig(channels, apiKey, hmacSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse into raw map to verify nested v6 key structure
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal as raw map: %v", err)
	}

	// Verify client.token.hmac_secret_key
	client, ok := raw["client"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'client' to be a nested object")
	}
	token, ok := client["token"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'client.token' to be a nested object")
	}
	if got := token["hmac_secret_key"]; got != hmacSecret {
		t.Errorf("client.token.hmac_secret_key = %q, want %q", got, hmacSecret)
	}

	// Verify client.allowed_origins
	origins, ok := client["allowed_origins"].([]interface{})
	if !ok {
		t.Fatal("expected 'client.allowed_origins' to be an array")
	}
	if len(origins) != 1 || origins[0] != "*" {
		t.Errorf("client.allowed_origins = %v, want [\"*\"]", origins)
	}

	// Verify http_api.key
	httpAPI, ok := raw["http_api"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'http_api' to be a nested object")
	}
	if got := httpAPI["key"]; got != apiKey {
		t.Errorf("http_api.key = %q, want %q", got, apiKey)
	}

	// Verify channel.without_namespace.allow_subscribe_for_client
	channel, ok := raw["channel"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'channel' to be a nested object")
	}
	withoutNS, ok := channel["without_namespace"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'channel.without_namespace' to be a nested object")
	}
	if got := withoutNS["allow_subscribe_for_client"]; got != true {
		t.Errorf("channel.without_namespace.allow_subscribe_for_client = %v, want true", got)
	}

	// Verify channel.namespaces exists and is an array
	namespaces, ok := channel["namespaces"].([]interface{})
	if !ok {
		t.Fatal("expected 'channel.namespaces' to be an array")
	}
	if len(namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(namespaces))
	}
}

func TestGenerateCentrifugoConfig_ValidJSON(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "alpha",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "AlphaReq", IsDispatch: true},
				{Direction: "server_to_client", TypeName: "AlphaResp"},
			},
		},
		{
			Name: "beta",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "BetaReq", IsDispatch: true},
				{Direction: "client_to_server", TypeName: "BetaMidstream", IsDispatch: false},
				{Direction: "server_to_client", TypeName: "BetaResp"},
			},
		},
	}

	data, err := GenerateCentrifugoConfig(channels, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !json.Valid(data) {
		t.Error("expected output to be valid JSON")
	}
}

func TestGenerateCentrifugoConfig_EmptyChannels(t *testing.T) {
	data, err := GenerateCentrifugoConfig(nil, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !json.Valid(data) {
		t.Error("expected output to be valid JSON even with no channels")
	}

	var cfg centrifugoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if len(cfg.Channel.Namespaces) != 0 {
		t.Errorf("expected 0 namespaces, got %d", len(cfg.Channel.Namespaces))
	}

	// Core structure should still be present
	if cfg.HTTPAPI.Key != "key" {
		t.Errorf("expected api key %q, got %q", "key", cfg.HTTPAPI.Key)
	}
	if cfg.Client.Token.HMACSecretKey != "secret" {
		t.Errorf("expected hmac secret %q, got %q", "secret", cfg.Client.Token.HMACSecretKey)
	}
}

func TestIsBidirectional(t *testing.T) {
	tests := []struct {
		name     string
		channel  codegen.SerializedChannelInfo
		wantBidi bool
	}{
		{
			name: "unidirectional - only dispatch FromClient",
			channel: codegen.SerializedChannelInfo{
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "Req", IsDispatch: true},
					{Direction: "server_to_client", TypeName: "Resp"},
				},
			},
			wantBidi: false,
		},
		{
			name: "bidirectional - has mid-stream FromClient",
			channel: codegen.SerializedChannelInfo{
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "Start", IsDispatch: true},
					{Direction: "client_to_server", TypeName: "FollowUp", IsDispatch: false},
					{Direction: "server_to_client", TypeName: "Resp"},
				},
			},
			wantBidi: true,
		},
		{
			name: "multiple mid-stream FromClient",
			channel: codegen.SerializedChannelInfo{
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "Start", IsDispatch: true},
					{Direction: "client_to_server", TypeName: "ToolApproval", IsDispatch: false},
					{Direction: "client_to_server", TypeName: "UserInput", IsDispatch: false},
					{Direction: "server_to_client", TypeName: "BotMsg"},
				},
			},
			wantBidi: true,
		},
		{
			name: "only server_to_client messages besides dispatch",
			channel: codegen.SerializedChannelInfo{
				Messages: []codegen.SerializedMessageInfo{
					{Direction: "client_to_server", TypeName: "Trigger", IsDispatch: true},
					{Direction: "server_to_client", TypeName: "Progress"},
					{Direction: "server_to_client", TypeName: "Complete"},
				},
			},
			wantBidi: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBidirectional(tt.channel)
			if got != tt.wantBidi {
				t.Errorf("isBidirectional() = %v, want %v", got, tt.wantBidi)
			}
		})
	}
}

func TestGenerateCentrifugoConfig_MixedChannels(t *testing.T) {
	channels := []codegen.SerializedChannelInfo{
		{
			Name: "unidirectional_ch",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "Req", IsDispatch: true},
				{Direction: "server_to_client", TypeName: "Resp"},
			},
		},
		{
			Name: "bidirectional_ch",
			Messages: []codegen.SerializedMessageInfo{
				{Direction: "client_to_server", TypeName: "Start", IsDispatch: true},
				{Direction: "client_to_server", TypeName: "FollowUp", IsDispatch: false},
				{Direction: "server_to_client", TypeName: "Resp"},
			},
		},
	}

	data, err := GenerateCentrifugoConfig(channels, "key", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var cfg centrifugoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if len(cfg.Channel.Namespaces) != 2 {
		t.Fatalf("expected 2 namespaces, got %d", len(cfg.Channel.Namespaces))
	}

	nsMap := make(map[string]centrifugoNamespace)
	for _, ns := range cfg.Channel.Namespaces {
		nsMap[ns.Name] = ns
	}

	uniNS, ok := nsMap["unidirectional_ch"]
	if !ok {
		t.Fatal("expected 'unidirectional_ch' namespace")
	}
	if uniNS.AllowPublishForSubscriber {
		t.Error("unidirectional channel should have allow_publish_for_subscriber=false")
	}

	bidiNS, ok := nsMap["bidirectional_ch"]
	if !ok {
		t.Fatal("expected 'bidirectional_ch' namespace")
	}
	if !bidiNS.AllowPublishForSubscriber {
		t.Error("bidirectional channel should have allow_publish_for_subscriber=true")
	}
}
