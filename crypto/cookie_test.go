package crypto

import (
	"strings"
	"testing"

	"github.com/shipq/shipq/proptest"
)

func TestSignCookie(t *testing.T) {
	sessionID := "abc123xyz"
	secret := []byte("supersecretkey32byteslong!!!!!")

	signed := SignCookie(sessionID, secret)

	// Check format: base64.base64
	parts := strings.Split(signed, ".")
	if len(parts) != 2 {
		t.Errorf("SignCookie() = %v, want format base64.base64", signed)
	}

	// Should not contain the raw session ID (it's encoded)
	if strings.Contains(signed, sessionID) {
		t.Errorf("SignCookie() contains raw session ID, should be base64 encoded")
	}
}

func TestSignCookieConsistency(t *testing.T) {
	sessionID := "consistentid"
	secret := []byte("consistentsecret32byteslong!!!!")

	signed1 := SignCookie(sessionID, secret)
	signed2 := SignCookie(sessionID, secret)

	// Same session ID and secret should produce same signed value
	if signed1 != signed2 {
		t.Errorf("SignCookie() produced different values for same inputs: %v vs %v", signed1, signed2)
	}
}

func TestSignCookieDifferentSecrets(t *testing.T) {
	sessionID := "sameid"
	secret1 := []byte("secret1_32byteslongforhmacsha256")
	secret2 := []byte("secret2_32byteslongforhmacsha256")

	signed1 := SignCookie(sessionID, secret1)
	signed2 := SignCookie(sessionID, secret2)

	// Different secrets should produce different signatures
	if signed1 == signed2 {
		t.Error("SignCookie() produced same value for different secrets")
	}
}

func TestVerifyCookie(t *testing.T) {
	sessionID := "mySessionID123"
	secret := []byte("mysecretkey32byteslongforhmac!!")

	signed := SignCookie(sessionID, secret)

	// Should verify with correct secret
	result, err := VerifyCookie(signed, secret)
	if err != nil {
		t.Fatalf("VerifyCookie() error = %v", err)
	}
	if result != sessionID {
		t.Errorf("VerifyCookie() = %v, want %v", result, sessionID)
	}
}

func TestVerifyCookieWrongSecret(t *testing.T) {
	sessionID := "sessionid"
	secret := []byte("correctsecret32byteslongforhma!!")
	wrongSecret := []byte("wrongsecret32byteslongforhmac!!")

	signed := SignCookie(sessionID, secret)

	// Should fail with wrong secret
	_, err := VerifyCookie(signed, wrongSecret)
	if err != ErrInvalidSignature {
		t.Errorf("VerifyCookie() error = %v, want ErrInvalidSignature", err)
	}
}

func TestVerifyCookieTampered(t *testing.T) {
	sessionID := "originalsession"
	secret := []byte("secretkey32byteslongforhmacsha!")

	signed := SignCookie(sessionID, secret)

	// Tamper with the session ID part (first part)
	parts := strings.Split(signed, ".")
	tampered := "dGFtcGVyZWQ" + "." + parts[1] // "tampered" in base64

	_, err := VerifyCookie(tampered, secret)
	if err != ErrInvalidSignature {
		t.Errorf("VerifyCookie() error = %v, want ErrInvalidSignature for tampered cookie", err)
	}
}

func TestVerifyCookieInvalidFormat(t *testing.T) {
	secret := []byte("secretkey32byteslongforhmacsha!")

	tests := []struct {
		name   string
		cookie string
	}{
		{"no separator", "noseparator"},
		{"empty", ""},
		{"only separator", "."},
		{"invalid base64 id", "!!!.YWJj"},
		{"invalid base64 sig", "YWJj.!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifyCookie(tt.cookie, secret)
			if err == nil {
				t.Errorf("VerifyCookie() expected error for invalid cookie: %v", tt.cookie)
			}
		})
	}
}

func TestVerifyCookieEmptySessionID(t *testing.T) {
	secret := []byte("secretkey32byteslongforhmacsha!")

	// Empty session ID should still work (edge case)
	signed := SignCookie("", secret)
	result, err := VerifyCookie(signed, secret)
	if err != nil {
		t.Fatalf("VerifyCookie() error = %v", err)
	}
	if result != "" {
		t.Errorf("VerifyCookie() = %v, want empty string", result)
	}
}

func TestGenerateSecret(t *testing.T) {
	secret1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	// Should be 32 bytes
	if len(secret1) != 32 {
		t.Errorf("GenerateSecret() length = %v, want 32", len(secret1))
	}

	// Should be unique
	secret2, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	if string(secret1) == string(secret2) {
		t.Error("GenerateSecret() produced identical secrets")
	}
}

func TestSignVerifyRoundtrip(t *testing.T) {
	secret, _ := GenerateSecret()

	testCases := []string{
		"simple",
		"with-dashes",
		"with_underscores",
		"MixedCase123",
		"special!@#$%^&*()",
		"unicode:日本語",
		"verylongsessionidthatmightbeusedinarealapplication12345678901234567890",
	}

	for _, sessionID := range testCases {
		t.Run(sessionID, func(t *testing.T) {
			signed := SignCookie(sessionID, secret)
			result, err := VerifyCookie(signed, secret)
			if err != nil {
				t.Fatalf("VerifyCookie() error = %v", err)
			}
			if result != sessionID {
				t.Errorf("Round-trip failed: got %v, want %v", result, sessionID)
			}
		})
	}
}

// =============================================================================
// Property-Based Tests
// =============================================================================

// Property: For any session ID, sign-verify round-trip works
func TestPropertySignVerifyRoundtrip(t *testing.T) {
	secret, _ := GenerateSecret()

	proptest.ForAll(t, "sign-verify roundtrip", 100, func(g *proptest.Generator) (string, bool) {
		sessionID := g.EdgeCaseString()

		signed := SignCookie(sessionID, secret)
		result, err := VerifyCookie(signed, secret)

		if err != nil {
			return sessionID, false
		}

		return sessionID, result == sessionID
	})
}

// Property: Same inputs always produce same signature (deterministic)
func TestPropertySignDeterministic(t *testing.T) {
	proptest.ForAll2(t, "sign deterministic", 100, func(g *proptest.Generator) (string, string, bool) {
		sessionID := g.StringN(1, 100)
		secret := g.BytesN(16, 64)

		signed1 := SignCookie(sessionID, secret)
		signed2 := SignCookie(sessionID, secret)

		return sessionID, string(secret), signed1 == signed2
	})
}

// Property: Different secrets produce different signatures
func TestPropertyDifferentSecrets(t *testing.T) {
	proptest.ForAll(t, "different secrets", 100, func(g *proptest.Generator) (string, bool) {
		sessionID := g.StringN(1, 50)
		secret1 := g.BytesN(32, 32)
		secret2 := g.BytesN(32, 32)

		// Make sure secrets are different
		if string(secret1) == string(secret2) {
			secret2[0] ^= 0xFF
		}

		signed1 := SignCookie(sessionID, secret1)
		signed2 := SignCookie(sessionID, secret2)

		// Signatures should be different (different secrets)
		return sessionID, signed1 != signed2
	})
}

// Property: Wrong secret always fails verification
func TestPropertyWrongSecretFails(t *testing.T) {
	proptest.ForAll(t, "wrong secret fails", 100, func(g *proptest.Generator) (string, bool) {
		sessionID := g.StringN(1, 50)
		correctSecret := g.BytesN(32, 32)
		wrongSecret := g.BytesN(32, 32)

		// Make sure secrets are different
		if string(correctSecret) == string(wrongSecret) {
			wrongSecret[0] ^= 0xFF
		}

		signed := SignCookie(sessionID, correctSecret)
		_, err := VerifyCookie(signed, wrongSecret)

		// Should fail with wrong secret
		return sessionID, err == ErrInvalidSignature
	})
}

// Property: Tampered cookies fail verification
func TestPropertyTamperedCookieFails(t *testing.T) {
	proptest.ForAll(t, "tampered cookie fails", 100, func(g *proptest.Generator) (string, bool) {
		sessionID := g.StringN(1, 50)
		secret := g.BytesN(32, 32)

		signed := SignCookie(sessionID, secret)

		// Create a different session ID for tampering
		differentID := sessionID + "x"
		tamperedSigned := SignCookie(differentID, secret)

		// Take the signature from the original and use it with a different session ID
		parts := strings.Split(signed, ".")
		tamperedParts := strings.Split(tamperedSigned, ".")
		if len(parts) != 2 || len(tamperedParts) != 2 {
			return sessionID, false
		}

		// Use different session ID with original signature (proper tampering)
		tampered := tamperedParts[0] + "." + parts[1]

		result, err := VerifyCookie(tampered, secret)
		// Should fail - signature won't match the tampered session ID
		if err == nil && result == differentID {
			// This would mean our tampering wasn't detected
			return sessionID, false
		}
		return sessionID, err != nil
	})
}

// Property: Output format is always valid (base64.base64)
func TestPropertyOutputFormat(t *testing.T) {
	proptest.ForAll(t, "output format", 100, func(g *proptest.Generator) (string, bool) {
		sessionID := g.String(100)
		secret := g.BytesN(32, 32)

		signed := SignCookie(sessionID, secret)

		// Must have exactly one dot separator
		parts := strings.Split(signed, ".")
		return sessionID, len(parts) == 2
	})
}

func BenchmarkSignCookie(b *testing.B) {
	sessionID := "benchmarksessionid"
	secret := []byte("benchmarksecret32byteslongfor!!")
	for i := 0; i < b.N; i++ {
		_ = SignCookie(sessionID, secret)
	}
}

func BenchmarkVerifyCookie(b *testing.B) {
	sessionID := "benchmarksessionid"
	secret := []byte("benchmarksecret32byteslongfor!!")
	signed := SignCookie(sessionID, secret)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = VerifyCookie(signed, secret)
	}
}
