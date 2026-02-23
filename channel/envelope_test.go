package channel

import (
	"encoding/json"
	"testing"
)

func TestEnvelope_MarshalUnmarshal_RoundTrip(t *testing.T) {
	original := Envelope{
		Type: "Approval",
		Data: json.RawMessage(`{"approved":true,"amount":42}`),
	}

	marshaled := original.Marshal()

	var decoded Envelope
	if err := decoded.Unmarshal(marshaled); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, original.Type)
	}
	if string(decoded.Data) != string(original.Data) {
		t.Errorf("Data mismatch: got %s, want %s", string(decoded.Data), string(original.Data))
	}
}

func TestEnvelope_MarshalUnmarshal_EmptyData(t *testing.T) {
	original := Envelope{
		Type: "Ping",
		Data: json.RawMessage(`{}`),
	}

	marshaled := original.Marshal()

	var decoded Envelope
	if err := decoded.Unmarshal(marshaled); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "Ping" {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, "Ping")
	}
	if string(decoded.Data) != "{}" {
		t.Errorf("Data mismatch: got %s, want {}", string(decoded.Data))
	}
}

func TestEnvelope_MarshalUnmarshal_NullData(t *testing.T) {
	original := Envelope{
		Type: "Empty",
		Data: nil,
	}

	marshaled := original.Marshal()

	var decoded Envelope
	if err := decoded.Unmarshal(marshaled); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != "Empty" {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, "Empty")
	}
}

func TestEnvelope_Unmarshal_InvalidJSON(t *testing.T) {
	var env Envelope
	err := env.Unmarshal([]byte(`not valid json at all`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestEnvelope_Unmarshal_PartialJSON(t *testing.T) {
	var env Envelope
	err := env.Unmarshal([]byte(`{"type":"Foo"`))
	if err == nil {
		t.Fatal("expected error for truncated JSON, got nil")
	}
}

func TestEnvelope_Marshal_ProducesValidJSON(t *testing.T) {
	env := Envelope{
		Type: "Token",
		Data: json.RawMessage(`{"token":"abc123"}`),
	}

	marshaled := env.Marshal()

	// Verify it's valid JSON by attempting to unmarshal into a generic map.
	var m map[string]interface{}
	if err := json.Unmarshal(marshaled, &m); err != nil {
		t.Fatalf("Marshal produced invalid JSON: %v", err)
	}

	if m["type"] != "Token" {
		t.Errorf("expected type field to be %q, got %v", "Token", m["type"])
	}

	dataMap, ok := m["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data field to be a map, got %T", m["data"])
	}
	if dataMap["token"] != "abc123" {
		t.Errorf("expected data.token to be %q, got %v", "abc123", dataMap["token"])
	}
}

func TestEnvelope_MarshalUnmarshal_NestedData(t *testing.T) {
	nestedJSON := `{"items":[{"id":1,"name":"foo"},{"id":2,"name":"bar"}],"total":2}`
	original := Envelope{
		Type: "ListResult",
		Data: json.RawMessage(nestedJSON),
	}

	marshaled := original.Marshal()

	var decoded Envelope
	if err := decoded.Unmarshal(marshaled); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, original.Type)
	}

	// Compare by re-compacting both to handle whitespace differences.
	var origCompact, decodedCompact []byte
	origCompact, _ = json.Marshal(json.RawMessage(nestedJSON))
	decodedCompact, _ = json.Marshal(decoded.Data)
	if string(origCompact) != string(decodedCompact) {
		t.Errorf("Data mismatch:\n  got:  %s\n  want: %s", string(decodedCompact), string(origCompact))
	}
}
