package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateRandomIngressKey creates a cryptographically random API key and its SHA-256 hash.
// The raw key is returned to the user once; the hash is stored for verification.
func GenerateRandomIngressKey() (rawKey string, keyHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw := hex.EncodeToString(b)
	return raw, HashIngressKey(raw), nil
}

// HashIngressKey computes the standard SHA-256 hash representation of an Ingress API Key
// used for webhook storage and resolution.
func HashIngressKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}
