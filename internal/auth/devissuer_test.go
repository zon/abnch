package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	jose "github.com/go-jose/go-jose/v4"
)

func newDevKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// TestDevIssuerServesConfiguration checks the OIDC configuration names the
// issuer, points at the JWKS, and advertises the dev key's algorithm.
func TestDevIssuerServesConfiguration(t *testing.T) {
	issuer := devIssuer(t, "http://bench.example.com/dev")

	var doc struct {
		Issuer  string   `json:"issuer"`
		JWKSURI string   `json:"jwks_uri"`
		Algs    []string `json:"id_token_signing_alg_values_supported"`
	}
	getDevIssuer(t, issuer, "/dev/.well-known/openid-configuration", &doc)

	if doc.Issuer != "http://bench.example.com/dev" {
		t.Errorf("issuer = %q, want the dev issuer url", doc.Issuer)
	}
	if doc.JWKSURI != "http://bench.example.com/dev/keys" {
		t.Errorf("jwks_uri = %q, want the dev issuer's keys", doc.JWKSURI)
	}
	if !hasAlg(doc.Algs, string(jose.ES256)) {
		t.Errorf("algorithms = %v, want %s", doc.Algs, jose.ES256)
	}
}

// TestDevIssuerServesJWKS checks the JWKS publishes the public half of the dev
// key, so a caller can verify a token the dev issuer signs.
func TestDevIssuerServesJWKS(t *testing.T) {
	key := newDevKey(t)
	issuer, err := NewDevIssuer("http://bench.example.com/dev", key)
	if err != nil {
		t.Fatal(err)
	}

	var set jose.JSONWebKeySet
	getDevIssuer(t, issuer, "/dev/keys", &set)

	if len(set.Keys) != 1 {
		t.Fatalf("got %d keys, want 1", len(set.Keys))
	}
	pub, ok := set.Keys[0].Key.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("key type = %T, want an ECDSA public key", set.Keys[0].Key)
	}
	if !pub.Equal(&key.PublicKey) {
		t.Error("JWKS does not publish the dev key's public half")
	}
}

func devIssuer(t *testing.T, url string) *DevIssuer {
	t.Helper()
	issuer, err := NewDevIssuer(url, newDevKey(t))
	if err != nil {
		t.Fatal(err)
	}
	return issuer
}

// getDevIssuer serves one dev issuer request without credentials and decodes a
// JSON response into body.
func getDevIssuer(t *testing.T, issuer *DevIssuer, path string, body any) {
	t.Helper()

	mux := http.NewServeMux()
	issuer.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusOK)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), body); err != nil {
		t.Fatal(err)
	}
}

func hasAlg(algs []string, want string) bool {
	for _, alg := range algs {
		if alg == want {
			return true
		}
	}
	return false
}
