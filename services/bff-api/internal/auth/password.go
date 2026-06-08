// Package auth holds the BFF's access-control primitives (Phase 134 /
// ADR-040): argon2id password hashing, opaque token generation, the
// request-scoped identity, and the session-or-API-key middleware. It depends
// on no other internal package, so storage and handler can both import it
// without a cycle.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params are the argon2id cost parameters. Defaults follow the OWASP
// Password Storage Cheat Sheet (m=19 MiB, t=2, p=1).
type Argon2Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// DefaultArgon2Params returns the OWASP-recommended argon2id parameters.
func DefaultArgon2Params() Argon2Params {
	return Argon2Params{MemoryKiB: 19456, Iterations: 2, Parallelism: 1, SaltLen: 16, KeyLen: 32}
}

// HashPassword returns a PHC-encoded argon2id hash of the plaintext password.
// The encoded string carries the parameters and salt, so VerifyPassword needs
// only the stored string.
func HashPassword(plain string, p Argon2Params) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, p.Iterations, p.MemoryKiB, p.Parallelism, p.KeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.MemoryKiB, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether the plaintext matches the PHC-encoded
// argon2id hash. The comparison is constant-time. A malformed encoding is an
// error (never a silent false-negative the caller might misread).
func VerifyPassword(plain, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=..,t=..,p=..", <salt>, <hash>]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("invalid argon2id hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("parse argon2 version: %w", err)
	}
	if version != argon2.Version {
		return false, fmt.Errorf("incompatible argon2 version %d", version)
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("parse argon2 params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}
	got := argon2.IDKey([]byte(plain), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
