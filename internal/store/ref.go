package store

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// DatumRef returns a datum's ref: a short, URL-safe string derived from its
// JSON. The same JSON always returns the same ref, so it stands in for the
// datum's identity.
func DatumRef(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])[:12]
}

// ResultRef returns a result's ref: a short, URL-safe string derived from the
// result JSON within its datum. The same JSON for the same datum always returns
// the same ref, and results for different data return different refs even when
// their JSON matches.
func ResultRef(r Result) string {
	parts := []string{r.Org, r.Repo, r.Task, r.Test, r.Data, r.JSON}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:12]
}
