package crypto

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/proptest"
)

func TestHashPassword(t *testing.T) {
	password := "correcthorsebatterystaple"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Check that the hash is in PHC format
	hashStr := string(hash)
	if !strings.HasPrefix(hashStr, "$argon2id$") {
		t.Errorf("HashPassword() = %v, want PHC format starting with $argon2id$", hashStr)
	}

	// Check that the hash contains the expected parameters
	if !strings.Contains(hashStr, "m=65536") {
		t.Errorf("HashPassword() missing memory parameter")
	}
	if !strings.Contains(hashStr, "t=1") {
		t.Errorf("HashPassword() missing time parameter")
	}
	if !strings.Contains(hashStr, "p=4") {
		t.Errorf("HashPassword() missing threads parameter")
	}
}

func TestHashPasswordUniqueness(t *testing.T) {
	password := "samepassword"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Same password should produce different hashes (due to random salt)
	if string(hash1) == string(hash2) {
		t.Error("HashPassword() produced identical hashes for same password (salt not random)")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "mysecretpassword"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	// Correct password should verify
	if !VerifyPassword(hash, password) {
		t.Error("VerifyPassword() returned false for correct password")
	}

	// Wrong password should not verify
	if VerifyPassword(hash, "wrongpassword") {
		t.Error("VerifyPassword() returned true for wrong password")
	}

	// Empty password should not verify
	if VerifyPassword(hash, "") {
		t.Error("VerifyPassword() returned true for empty password")
	}
}

// =============================================================================
// Property-Based Tests
// =============================================================================

// Property: For any password, HashPassword produces a hash that VerifyPassword accepts
func TestPropertyHashVerifyRoundtrip(t *testing.T) {
	proptest.ForAll(t, "hash-verify roundtrip", 50, func(g *proptest.Generator) (string, bool) {
		// Generate a random password (including edge cases)
		password := g.EdgeCaseString()

		hash, err := HashPassword(password)
		if err != nil {
			return password, false
		}

		// The original password must verify
		return password, VerifyPassword(hash, password)
	})
}

// Property: For any password, hashing twice produces different hashes (random salt)
func TestPropertyHashUniqueness(t *testing.T) {
	proptest.ForAll(t, "hash uniqueness", 50, func(g *proptest.Generator) (string, bool) {
		password := g.StringN(1, 50)

		hash1, err := HashPassword(password)
		if err != nil {
			return password, false
		}

		hash2, err := HashPassword(password)
		if err != nil {
			return password, false
		}

		// Hashes must be different (due to random salt)
		return password, string(hash1) != string(hash2)
	})
}

// Property: For any password, a different password should not verify
func TestPropertyWrongPasswordRejects(t *testing.T) {
	proptest.ForAll2(t, "wrong password rejects", 50, func(g *proptest.Generator) (string, string, bool) {
		password1 := g.StringN(1, 50)
		password2 := g.StringN(1, 50)

		// Make sure passwords are different
		if password1 == password2 {
			password2 = password1 + "x"
		}

		hash, err := HashPassword(password1)
		if err != nil {
			return password1, password2, false
		}

		// Wrong password must NOT verify
		return password1, password2, !VerifyPassword(hash, password2)
	})
}

// Property: Hash output is always valid PHC format
func TestPropertyHashFormat(t *testing.T) {
	proptest.ForAll(t, "hash format", 50, func(g *proptest.Generator) (string, bool) {
		password := g.String(100)

		hash, err := HashPassword(password)
		if err != nil {
			return password, false
		}

		hashStr := string(hash)

		// Must start with $argon2id$
		if !strings.HasPrefix(hashStr, "$argon2id$") {
			return password, false
		}

		// Must contain version
		if !strings.Contains(hashStr, "v=19") {
			return password, false
		}

		// Must have exactly 6 parts when split by $
		parts := strings.Split(hashStr, "$")
		if len(parts) != 6 {
			return password, false
		}

		return password, true
	})
}

// Property: Empty password can be hashed and verified
func TestPropertyEmptyPassword(t *testing.T) {
	hash, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword() error for empty password: %v", err)
	}

	if !VerifyPassword(hash, "") {
		t.Error("VerifyPassword() failed for empty password")
	}

	if VerifyPassword(hash, "notempty") {
		t.Error("VerifyPassword() accepted wrong password for empty password hash")
	}
}

// Property: Very long passwords work correctly
func TestPropertyLongPasswords(t *testing.T) {
	proptest.ForAll(t, "long passwords", 20, func(g *proptest.Generator) (int, bool) {
		// Generate passwords up to 10KB
		length := g.IntRange(1000, 10000)
		password := stringOfLen(g, proptest.CharsetPrintable, length)

		hash, err := HashPassword(password)
		if err != nil {
			return length, false
		}

		return length, VerifyPassword(hash, password)
	})
}

// Helper to generate string of specific length
func stringOfLen(g *proptest.Generator, charset string, length int) string {
	if length == 0 {
		return ""
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[g.Intn(len(charset))]
	}
	return string(b)
}

func TestVerifyPasswordInvalidHash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"not PHC format", "notahash"},
		{"wrong algorithm", "$argon2i$v=19$m=65536,t=1,p=4$c2FsdA$aGFzaA"},
		{"wrong version", "$argon2id$v=18$m=65536,t=1,p=4$c2FsdA$aGFzaA"},
		{"missing parts", "$argon2id$v=19$m=65536,t=1,p=4$c2FsdA"},
		{"invalid base64 salt", "$argon2id$v=19$m=65536,t=1,p=4$!!!$aGFzaA"},
		{"invalid base64 hash", "$argon2id$v=19$m=65536,t=1,p=4$c2FsdA$!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if VerifyPassword([]byte(tt.hash), "anypassword") {
				t.Errorf("VerifyPassword() returned true for invalid hash: %v", tt.hash)
			}
		})
	}
}

func TestDecodeHash(t *testing.T) {
	// First create a valid hash
	hash, err := HashPassword("testpassword")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	params, salt, hashBytes, err := decodeHash(string(hash))
	if err != nil {
		t.Fatalf("decodeHash() error = %v", err)
	}

	// Check parameters
	if params.memory != argon2Memory {
		t.Errorf("decodeHash() memory = %v, want %v", params.memory, argon2Memory)
	}
	if params.time != argon2Time {
		t.Errorf("decodeHash() time = %v, want %v", params.time, argon2Time)
	}
	if params.threads != argon2Threads {
		t.Errorf("decodeHash() threads = %v, want %v", params.threads, argon2Threads)
	}

	// Check salt length
	if len(salt) != argon2SaltLen {
		t.Errorf("decodeHash() salt length = %v, want %v", len(salt), argon2SaltLen)
	}

	// Check hash length
	if len(hashBytes) != argon2HashLen {
		t.Errorf("decodeHash() hash length = %v, want %v", len(hashBytes), argon2HashLen)
	}
}

func TestDecodeHashErrors(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr error
	}{
		{"wrong algorithm", "$argon2i$v=19$m=65536,t=1,p=4$c2FsdA$aGFzaA", ErrInvalidHash},
		{"wrong version", "$argon2id$v=18$m=65536,t=1,p=4$c2FsdA$aGFzaA", ErrIncompatibleVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := decodeHash(tt.hash)
			if err != tt.wantErr {
				t.Errorf("decodeHash() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// Benchmark to ensure hashing is reasonably fast but not too fast (security tradeoff)
func BenchmarkHashPassword(b *testing.B) {
	password := "benchmarkpassword"
	for i := 0; i < b.N; i++ {
		_, _ = HashPassword(password)
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	password := "benchmarkpassword"
	hash, _ := HashPassword(password)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifyPassword(hash, password)
	}
}
