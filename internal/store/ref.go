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
// result JSON, its datum, and its model. The same JSON for the same datum from
// the same model always returns the same ref, and results that differ in data,
// JSON, or model return different refs.
func ResultRef(r Result) string {
	parts := []string{
		r.Org, r.Repo, r.Task, r.Test, r.Data, r.JSON,
		r.Model.Name, r.Model.Provider, r.Model.Variant,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:12]
}

// ScoreRef returns a score's ref: a short, URL-safe string derived from the
// score's key-number dictionary, its result, and its model. The same dictionary
// for the same result from the same model always returns the same ref, and
// scores that differ in result, dictionary, or model return different refs.
func ScoreRef(s Score) string {
	dict, _ := json.Marshal(s.Dictionary)
	parts := []string{
		s.Org, s.Repo, s.Task, s.Test, s.Result, string(dict),
		s.Model.Name, s.Model.Provider, s.Model.Variant,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:12]
}
