package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// ScoreRef returns a score's ref: a short, URL-safe string derived from the
// score's key-number dictionary within its result. The same dictionary for the
// same result always returns the same ref, and scores for different results
// return different refs even when their dictionaries match.
func ScoreRef(s Score) string {
	dict, _ := json.Marshal(s.Dictionary)
	parts := []string{s.Org, s.Repo, s.Task, s.Test, s.Result, string(dict)}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:12]
}
