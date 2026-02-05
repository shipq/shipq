package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

var (
	// ErrInvalidCookie is returned when the signed cookie format is invalid.
	ErrInvalidCookie = errors.New("invalid cookie format")
	// ErrInvalidSignature is returned when the cookie signature is invalid.
	ErrInvalidSignature = errors.New("invalid cookie signature")
)

// SignCookie signs a session ID for use in a cookie.
// The format is: base64(sessionID).base64(hmac-sha256(sessionID))
// The secret should be at least 32 bytes for security.
func SignCookie(sessionID string, secret []byte) string {
	// Create HMAC-SHA256 signature
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sessionID))
	signature := mac.Sum(nil)

	// Encode both parts as base64
	encodedID := base64.RawURLEncoding.EncodeToString([]byte(sessionID))
	encodedSig := base64.RawURLEncoding.EncodeToString(signature)

	return encodedID + "." + encodedSig
}

// VerifyCookie verifies a signed cookie and returns the session ID.
// Returns an error if the format is invalid or the signature doesn't match.
// Uses constant-time comparison to prevent timing attacks.
func VerifyCookie(signedValue string, secret []byte) (string, error) {
	// Split into session ID and signature
	parts := strings.SplitN(signedValue, ".", 2)
	if len(parts) != 2 {
		return "", ErrInvalidCookie
	}

	// Decode the session ID
	sessionIDBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", ErrInvalidCookie
	}
	sessionID := string(sessionIDBytes)

	// Decode the signature
	providedSig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", ErrInvalidCookie
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, secret)
	mac.Write(sessionIDBytes)
	expectedSig := mac.Sum(nil)

	// Constant-time comparison
	if !hmac.Equal(providedSig, expectedSig) {
		return "", ErrInvalidSignature
	}

	return sessionID, nil
}

// GenerateSecret generates a cryptographically secure random secret.
// The secret is 32 bytes (256 bits), suitable for HMAC-SHA256.
func GenerateSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := randomRead(secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// randomRead is a variable to allow testing with deterministic values.
var randomRead = cryptoRandRead

func cryptoRandRead(b []byte) (int, error) {
	return rand.Read(b)
}
