package store

import (
	"crypto/sha256"
	"encoding/hex"
)

// DatumRef returns a datum's ref: a short, URL-safe string derived from its
// JSON. The same JSON always returns the same ref, so it stands in for the
// datum's identity.
func DatumRef(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])[:12]
}
