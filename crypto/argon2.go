// Package crypto provides cryptographic utilities for authentication.
package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters per OWASP recommendations
// https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html
const (
	argon2Time    = 1         // Number of iterations
	argon2Memory  = 64 * 1024 // Memory in KiB (64 MiB)
	argon2Threads = 4         // Parallelism
	argon2SaltLen = 16        // Salt length in bytes
	argon2HashLen = 32        // Hash length in bytes
)

var (
	// ErrInvalidHash is returned when the hash format is invalid.
	ErrInvalidHash = errors.New("invalid hash format")
	// ErrIncompatibleVersion is returned when the hash version is not supported.
	ErrIncompatibleVersion = errors.New("incompatible argon2 version")
)

// HashPassword hashes a password using Argon2id with secure defaults.
// The returned hash is in PHC format: $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
func HashPassword(password string) ([]byte, error) {
	// Generate a random salt
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash the password
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2HashLen)

	// Encode to PHC format
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Time, argon2Threads, b64Salt, b64Hash)

	return []byte(encoded), nil
}

// VerifyPassword verifies a password against an Argon2id hash.
// Uses constant-time comparison to prevent timing attacks.
func VerifyPassword(hash []byte, password string) bool {
	// Parse the hash
	params, salt, hashBytes, err := decodeHash(string(hash))
	if err != nil {
		return false
	}

	// Compute hash with the same parameters
	computed := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(hashBytes)))

	// Constant-time comparison
	return subtle.ConstantTimeCompare(computed, hashBytes) == 1
}

// argon2Params holds the parameters used to hash a password.
type argon2Params struct {
	memory  uint32
	time    uint32
	threads uint8
}

// decodeHash parses a PHC-format Argon2id hash.
func decodeHash(encoded string) (*argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	// parts[0] is empty (leading $)
	// parts[1] is "argon2id"
	// parts[2] is "v=19"
	// parts[3] is "m=65536,t=1,p=4"
	// parts[4] is base64 salt
	// parts[5] is base64 hash

	if parts[1] != "argon2id" {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	var params argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.memory, &params.time, &params.threads); err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, nil, ErrInvalidHash
	}

	return &params, salt, hash, nil
}
