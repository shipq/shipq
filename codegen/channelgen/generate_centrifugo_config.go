package channelgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shipq/shipq/codegen"
)

// centrifugoConfig represents the Centrifugo v6 nested configuration format.
// See db/databases/centrifugo.json for reference.
type centrifugoConfig struct {
	Client  centrifugoClient  `json:"client"`
	HTTPAPI centrifugoHTTPAPI `json:"http_api"`
	Channel centrifugoChannel `json:"channel"`
}

type centrifugoClient struct {
	Token          centrifugoToken `json:"token"`
	AllowedOrigins []string        `json:"allowed_origins"`
}

type centrifugoToken struct {
	HMACSecretKey string `json:"hmac_secret_key"`
}

type centrifugoHTTPAPI struct {
	Key string `json:"key"`
}

type centrifugoChannel struct {
	WithoutNamespace centrifugoNamespaceOpts `json:"without_namespace"`
	Namespaces       []centrifugoNamespace   `json:"namespaces"`
}

type centrifugoNamespaceOpts struct {
	AllowSubscribeForClient   bool `json:"allow_subscribe_for_client"`
	AllowPublishForSubscriber bool `json:"allow_publish_for_subscriber,omitempty"`
}

type centrifugoNamespace struct {
	Name                      string `json:"name"`
	AllowSubscribeForClient   bool   `json:"allow_subscribe_for_client"`
	AllowPublishForSubscriber bool   `json:"allow_publish_for_subscriber"`
}

// isBidirectional returns true if the channel has mid-stream FromClient types
// (i.e., client_to_server messages that are NOT the dispatch message).
// A bidirectional channel needs allow_publish_for_subscriber: true so that
// clients can publish messages after the initial dispatch.
func isBidirectional(ch codegen.SerializedChannelInfo) bool {
	for _, msg := range ch.Messages {
		if msg.Direction == "client_to_server" && !msg.IsDispatch {
			return true
		}
	}
	return false
}

// GenerateCentrifugoConfig generates centrifugo.json content using the Centrifugo v6
// nested config format.
//
// Parameters:
//   - channels: serialized channel metadata from the channel compiler
//   - apiKey: from shipq.ini centrifugo_api_key
//   - hmacSecret: from shipq.ini centrifugo_hmac_secret
//
// The generated config includes:
//   - client.token.hmac_secret_key for JWT verification
//   - client.allowed_origins set to ["*"]
//   - http_api.key for API authentication
//   - channel.without_namespace with allow_subscribe_for_client: true
//   - channel.namespaces: one per channel, with allow_publish_for_subscriber
//     set based on whether the channel is bidirectional
//
// [L3]: Channel names in namespace configs must exactly match what the token
// endpoint generates. A mismatch between the namespace name in the config and
// the namespace prefix in the channel name will cause subscription failures
// that disconnect the entire client.
//
// [L1]: allow_publish_for_subscriber means client-published messages are
// delivered to ALL subscribers including the publisher itself (echo behavior).
// The runtime handles this via per-type buffering.
func GenerateCentrifugoConfig(channels []codegen.SerializedChannelInfo, apiKey, hmacSecret string) ([]byte, error) {
	namespaces := make([]centrifugoNamespace, 0, len(channels))

	for _, ch := range channels {
		ns := centrifugoNamespace{
			Name:                      ch.Name,
			AllowSubscribeForClient:   true,
			AllowPublishForSubscriber: isBidirectional(ch),
		}
		namespaces = append(namespaces, ns)
	}

	cfg := centrifugoConfig{
		Client: centrifugoClient{
			Token: centrifugoToken{
				HMACSecretKey: hmacSecret,
			},
			AllowedOrigins: []string{"*"},
		},
		HTTPAPI: centrifugoHTTPAPI{
			Key: apiKey,
		},
		Channel: centrifugoChannel{
			WithoutNamespace: centrifugoNamespaceOpts{
				AllowSubscribeForClient: true,
			},
			Namespaces: namespaces,
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal centrifugo config: %w", err)
	}

	// Append trailing newline for cleanliness
	data = append(data, '\n')

	return data, nil
}

// WriteCentrifugoConfig generates the Centrifugo config and writes it to
// <shipqRoot>/centrifugo.json.
func WriteCentrifugoConfig(channels []codegen.SerializedChannelInfo, shipqRoot, apiKey, hmacSecret string) error {
	data, err := GenerateCentrifugoConfig(channels, apiKey, hmacSecret)
	if err != nil {
		return err
	}

	outputPath := filepath.Join(shipqRoot, "centrifugo.json")
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write centrifugo.json: %w", err)
	}

	return nil
}
