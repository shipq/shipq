package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestManifest_JSON(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		m := Manifest{
			Endpoints: []ManifestEndpoint{
				{Method: "GET", Path: "/pets", HandlerPkg: "example/pets", HandlerName: "List", Shape: "ctx_resp_err"},
				{Method: "POST", Path: "/pets", HandlerPkg: "example/pets", HandlerName: "Create", Shape: "ctx_req_resp_err", ReqType: "CreatePetRequest", RespType: "Pet"},
			},
		}

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var m2 Manifest
		if err := json.Unmarshal(data, &m2); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if !reflect.DeepEqual(m, m2) {
			t.Errorf("roundtrip mismatch: got %+v, want %+v", m2, m)
		}
	})

	t.Run("stable ordering", func(t *testing.T) {
		m := Manifest{
			Endpoints: []ManifestEndpoint{
				{Method: "GET", Path: "/pets", HandlerPkg: "example/pets", HandlerName: "List", Shape: "ctx_resp_err"},
				{Method: "POST", Path: "/pets", HandlerPkg: "example/pets", HandlerName: "Create", Shape: "ctx_req_resp_err"},
			},
		}

		data1, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("first marshal failed: %v", err)
		}

		data2, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("second marshal failed: %v", err)
		}

		if !bytes.Equal(data1, data2) {
			t.Error("marshal should produce identical bytes")
		}
	})

	t.Run("omits empty optional fields", func(t *testing.T) {
		m := Manifest{
			Endpoints: []ManifestEndpoint{
				{Method: "GET", Path: "/health", HandlerPkg: "example", HandlerName: "Health", Shape: "ctx_err"},
			},
		}

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		// Should not contain req_type or resp_type keys when empty
		s := string(data)
		if strings.Contains(s, "req_type") {
			t.Error("expected JSON to not contain req_type when empty")
		}
		if strings.Contains(s, "resp_type") {
			t.Error("expected JSON to not contain resp_type when empty")
		}
	})

	t.Run("empty manifest", func(t *testing.T) {
		m := Manifest{Endpoints: []ManifestEndpoint{}}

		data, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var m2 Manifest
		if err := json.Unmarshal(data, &m2); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if !reflect.DeepEqual(m, m2) {
			t.Errorf("roundtrip mismatch: got %+v, want %+v", m2, m)
		}
	})
}
