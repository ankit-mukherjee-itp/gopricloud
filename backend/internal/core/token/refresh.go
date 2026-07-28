package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

const refreshTokenBytes = 32

// GenerateRefreshToken returns a new cryptographically random, URL-safe
// opaque token. The raw value is handed to the client and never stored;
// only its hash (see HashRefreshToken) is persisted.
func GenerateRefreshToken() (string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashRefreshToken returns the SHA-256 hex digest of a raw refresh token,
// suitable for storage and lookup.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
