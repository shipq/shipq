package channel

import (
	"crypto/tls"
	"testing"
)

func TestMachineryQueue_ImplementsTaskQueue(t *testing.T) {
	// Compile-time check is already in machinery.go via:
	//   var _ TaskQueue = (*MachineryQueue)(nil)
	// This test documents the intent explicitly.
	var _ TaskQueue = (*MachineryQueue)(nil)
}

func TestNewMachineryQueue_ReturnsNonNil(t *testing.T) {
	// NewMachineryQueue creates the Machinery server object but does NOT
	// connect to Redis until a task is sent or a worker is started.
	// This verifies the constructor doesn't panic with a bogus address.
	mq, err := NewMachineryQueue("localhost:6379")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}
	if mq == nil {
		t.Fatal("NewMachineryQueue returned nil")
	}
}

func TestNewMachineryQueue_ConfigHasBrokerAndBackend(t *testing.T) {
	// [L2] Verify the internal config.Config has Broker and ResultBackend
	// fields set for log output readability.
	redisAddr := "my-redis:6379"
	mq, err := NewMachineryQueue(redisAddr)
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	cnf := mq.Config()
	if cnf == nil {
		t.Fatal("Config() returned nil")
	}

	expectedBroker := "redis://" + redisAddr
	if cnf.Broker != expectedBroker {
		t.Errorf("Broker: got %q, want %q", cnf.Broker, expectedBroker)
	}

	expectedBackend := "redis://" + redisAddr
	if cnf.ResultBackend != expectedBackend {
		t.Errorf("ResultBackend: got %q, want %q", cnf.ResultBackend, expectedBackend)
	}
}

func TestNewMachineryQueue_ConfigHasDefaultQueue(t *testing.T) {
	mq, err := NewMachineryQueue("localhost:6379")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	cnf := mq.Config()
	if cnf.DefaultQueue != "shipq_tasks" {
		t.Errorf("DefaultQueue: got %q, want %q", cnf.DefaultQueue, "shipq_tasks")
	}
}

func TestNewMachineryQueue_ConfigHasNoUnixSignals(t *testing.T) {
	// NoUnixSignals must be true so we control shutdown via context, not OS signals.
	mq, err := NewMachineryQueue("localhost:6379")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	cnf := mq.Config()
	if !cnf.NoUnixSignals {
		t.Error("NoUnixSignals should be true")
	}
}

func TestNewMachineryQueue_ConfigHasRedisSettings(t *testing.T) {
	mq, err := NewMachineryQueue("localhost:6379")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	cnf := mq.Config()
	if cnf.Redis == nil {
		t.Fatal("Redis config should not be nil")
	}
	if cnf.Redis.MaxIdle != 3 {
		t.Errorf("Redis.MaxIdle: got %d, want 3", cnf.Redis.MaxIdle)
	}
	if cnf.Redis.NormalTasksPollPeriod != 1000 {
		t.Errorf("Redis.NormalTasksPollPeriod: got %d, want 1000", cnf.Redis.NormalTasksPollPeriod)
	}
	if cnf.Redis.DelayedTasksPollPeriod != 500 {
		t.Errorf("Redis.DelayedTasksPollPeriod: got %d, want 500", cnf.Redis.DelayedTasksPollPeriod)
	}
}

func TestNewMachineryQueue_ConfigHasResultsExpireIn(t *testing.T) {
	mq, err := NewMachineryQueue("localhost:6379")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	cnf := mq.Config()
	if cnf.ResultsExpireIn != 86400 {
		t.Errorf("ResultsExpireIn: got %d, want 86400", cnf.ResultsExpireIn)
	}
}

func TestNewMachineryQueue_DifferentAddresses(t *testing.T) {
	addrs := []string{
		"localhost:6379",
		"redis.internal:6380",
		"10.0.0.5:6379",
	}

	for _, addr := range addrs {
		mq, err := NewMachineryQueue(addr)
		if err != nil {
			t.Fatalf("NewMachineryQueue(%q): %v", addr, err)
		}

		cnf := mq.Config()
		expected := "redis://" + addr
		if cnf.Broker != expected {
			t.Errorf("addr=%q: Broker got %q, want %q", addr, cnf.Broker, expected)
		}
		if cnf.ResultBackend != expected {
			t.Errorf("addr=%q: ResultBackend got %q, want %q", addr, cnf.ResultBackend, expected)
		}
	}
}

func TestNewMachineryQueue_NormalizesRedisURL(t *testing.T) {
	// When the caller passes a full "redis://host:port" URL (e.g., from
	// config.Settings.REDIS_URL), NewMachineryQueue must strip the scheme
	// before passing the address to NewGR and produce exactly one "redis://"
	// prefix in the Config fields.
	mq, err := NewMachineryQueue("redis://localhost:6379")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	cnf := mq.Config()
	if cnf.Broker != "redis://localhost:6379" {
		t.Errorf("Broker = %q, want %q", cnf.Broker, "redis://localhost:6379")
	}
	if cnf.ResultBackend != "redis://localhost:6379" {
		t.Errorf("ResultBackend = %q, want %q", cnf.ResultBackend, "redis://localhost:6379")
	}
}

func TestNewMachineryQueue_BareAddress(t *testing.T) {
	// A bare "host:port" (no scheme) should also work and produce the same
	// Config output as the prefixed variant.
	mq, err := NewMachineryQueue("localhost:6379")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	cnf := mq.Config()
	if cnf.Broker != "redis://localhost:6379" {
		t.Errorf("Broker = %q, want %q", cnf.Broker, "redis://localhost:6379")
	}
	if cnf.ResultBackend != "redis://localhost:6379" {
		t.Errorf("ResultBackend = %q, want %q", cnf.ResultBackend, "redis://localhost:6379")
	}
}

func TestNewMachineryQueue_NormalizesVariousRedisURLs(t *testing.T) {
	tests := []struct {
		input      string
		wantBroker string
	}{
		{"localhost:6379", "redis://localhost:6379"},
		{"redis://localhost:6379", "redis://localhost:6379"},
		{"redis://my-redis:6380", "redis://my-redis:6380"},
		{"my-redis:6380", "redis://my-redis:6380"},
		{"redis://10.0.0.5:6379", "redis://10.0.0.5:6379"},
		{"rediss://managed-valkey.example.com:25061", "rediss://managed-valkey.example.com:25061"},
		{"rediss://default:secret@managed-valkey.example.com:25061", "rediss://default:secret@managed-valkey.example.com:25061"},
	}

	for _, tt := range tests {
		mq, err := NewMachineryQueue(tt.input)
		if err != nil {
			t.Fatalf("NewMachineryQueue(%q): %v", tt.input, err)
		}

		cnf := mq.Config()
		if cnf.Broker != tt.wantBroker {
			t.Errorf("input=%q: Broker got %q, want %q", tt.input, cnf.Broker, tt.wantBroker)
		}
		if cnf.ResultBackend != tt.wantBroker {
			t.Errorf("input=%q: ResultBackend got %q, want %q", tt.input, cnf.ResultBackend, tt.wantBroker)
		}
	}
}

func TestNewMachineryQueue_RedissTLSEnabled(t *testing.T) {
	mq, err := NewMachineryQueue("rediss://managed-valkey.example.com:25061")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	cnf := mq.Config()
	if cnf.TLSConfig == nil {
		t.Fatal("TLSConfig should be non-nil for rediss:// URL")
	}
	if cnf.TLSConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("TLSConfig.MinVersion = %d, want %d", cnf.TLSConfig.MinVersion, tls.VersionTLS12)
	}
}

func TestNewMachineryQueue_RedisTLSNotEnabled(t *testing.T) {
	for _, input := range []string{"localhost:6379", "redis://localhost:6379"} {
		mq, err := NewMachineryQueue(input)
		if err != nil {
			t.Fatalf("NewMachineryQueue(%q): %v", input, err)
		}
		cnf := mq.Config()
		if cnf.TLSConfig != nil {
			t.Errorf("input=%q: TLSConfig should be nil for non-TLS URL", input)
		}
	}
}

func TestNewMachineryQueue_RedissWithCredentials(t *testing.T) {
	mq, err := NewMachineryQueue("rediss://default:s3cret@db-valkey.example.com:25061")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	cnf := mq.Config()
	wantBroker := "rediss://default:s3cret@db-valkey.example.com:25061"
	if cnf.Broker != wantBroker {
		t.Errorf("Broker = %q, want %q", cnf.Broker, wantBroker)
	}
	if cnf.TLSConfig == nil {
		t.Fatal("TLSConfig should be non-nil for rediss:// URL")
	}
}

func TestParseRedisAddr(t *testing.T) {
	tests := []struct {
		input      string
		wantAddr   string
		wantScheme string
		wantErr    bool
	}{
		{"localhost:6379", "localhost:6379", "redis", false},
		{"redis://localhost:6379", "localhost:6379", "redis", false},
		{"rediss://host:25061", "host:25061", "rediss", false},
		{"rediss://user:pass@host:25061", "user:pass@host:25061", "rediss", false},
		{"redis://user:pass@host:6379", "user:pass@host:6379", "redis", false},
		{"http://host:6379", "", "", true},
		{"rediss://", "", "", true},
	}

	for _, tt := range tests {
		addr, scheme, err := parseRedisAddr(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseRedisAddr(%q): expected error, got addr=%q scheme=%q", tt.input, addr, scheme)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseRedisAddr(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if addr != tt.wantAddr {
			t.Errorf("parseRedisAddr(%q): addr = %q, want %q", tt.input, addr, tt.wantAddr)
		}
		if scheme != tt.wantScheme {
			t.Errorf("parseRedisAddr(%q): scheme = %q, want %q", tt.input, scheme, tt.wantScheme)
		}
	}
}

func TestMachineryQueue_StopWorker_NoWorker(t *testing.T) {
	// StopWorker should not panic or error when no worker has been started.
	mq, err := NewMachineryQueue("localhost:6379")
	if err != nil {
		t.Fatalf("NewMachineryQueue: %v", err)
	}

	if err := mq.StopWorker(); err != nil {
		t.Errorf("StopWorker with no worker: %v", err)
	}
}
