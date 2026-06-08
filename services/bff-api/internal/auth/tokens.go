package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// opaqueTokenBytes is the entropy of every opaque secret AĒR issues — session
// ids and single-use invite / reset tokens. 32 bytes = 256 bits.
const opaqueTokenBytes = 32

// GenerateOpaqueToken returns a URL-safe random token (the RAW value that goes
// into the cookie or the emailed link) together with its SHA-256 hex hash (the
// value stored in Postgres). Storing only the hash means a read-only database
// leak yields no live session and no usable link. The token is high-entropy,
// so a fast hash is the correct choice here — argon2id is reserved for
// low-entropy passwords.
func GenerateOpaqueToken() (raw string, hash string, err error) {
	b := make([]byte, opaqueTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate opaque token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashOpaqueToken(raw), nil
}

// HashOpaqueToken returns the SHA-256 hex hash of a raw opaque token. Used both
// to store a freshly minted token and to look one up on presentation.
func HashOpaqueToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
