package service

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/zon/abnch/internal/devconfig"
)

// devIssuerConfig is a service config that trusts the dev issuer for its url,
// like the config `just config` writes.
func devIssuerConfig(t *testing.T, port int) string {
	t.Helper()

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	cfg := devconfig.Config{
		URL: url,
		Postgres: devconfig.Postgres{
			Host: "db", Port: 5432, DBName: "abnch",
			User: "admin", Password: "pw", SSLMode: "disable", TimeZone: "UTC",
		},
		OAuth: devconfig.OAuth{Issuers: []devconfig.Issuer{devconfig.DevIssuer(url)}},
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestRunServesDevIssuer checks the service serves the dev issuer's OIDC
// configuration and JWKS without credentials, and accepts a token the dev key
// signs.
func TestRunServesDevIssuer(t *testing.T) {
	port := freePort(t)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	path := writeConfig(t, devIssuerConfig(t, port))
	if err := devconfig.EnsureDevKey(devconfig.KeyPath(path)); err != nil {
		t.Fatal(err)
	}
	base := startServiceWith(t, port, Options{ConfigPath: path})

	var doc struct {
		Issuer  string   `json:"issuer"`
		JWKSURI string   `json:"jwks_uri"`
		Algs    []string `json:"id_token_signing_alg_values_supported"`
	}
	getJSON(t, base+"dev/.well-known/openid-configuration", &doc)

	if doc.Issuer != url+"/dev" {
		t.Errorf("issuer = %q, want %q", doc.Issuer, url+"/dev")
	}
	if doc.JWKSURI != url+"/dev/keys" {
		t.Errorf("jwks_uri = %q, want %q", doc.JWKSURI, url+"/dev/keys")
	}
	if !hasAlg(doc.Algs, string(jose.ES256)) {
		t.Errorf("algorithms = %v, want %s", doc.Algs, jose.ES256)
	}

	var set jose.JSONWebKeySet
	getJSON(t, doc.JWKSURI, &set)
	if len(set.Keys) != 1 {
		t.Fatalf("got %d JWKS keys, want 1", len(set.Keys))
	}
	key, err := devconfig.LoadDevKey(devconfig.KeyPath(path))
	if err != nil {
		t.Fatal(err)
	}
	pub, ok := set.Keys[0].Key.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("JWKS key type = %T, want an ECDSA public key", set.Keys[0].Key)
	}
	if !pub.Equal(&key.PublicKey) {
		t.Error("JWKS does not publish the dev key's public half")
	}

	// A token the dev key signs reaches the service through the issuer it
	// serves, since no route matches and the credentials are accepted.
	req, err := http.NewRequest(http.MethodGet, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+signDevToken(t, key, url+"/dev"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d for an authenticated request", resp.StatusCode, http.StatusNotFound)
	}
}

// TestRunHidesUntrustedDevIssuer checks the service serves no dev issuer when
// its config does not trust one.
func TestRunHidesUntrustedDevIssuer(t *testing.T) {
	base := startService(t, Options{})

	resp, err := http.Get(base + "dev/.well-known/openid-configuration")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestRunRejectsMissingDevKey checks the service reports a clear error when the
// config trusts a dev issuer but its key is missing.
func TestRunRejectsMissingDevKey(t *testing.T) {
	port := freePort(t)
	path := writeConfig(t, devIssuerConfig(t, port))
	opener := func(context.Context, devconfig.Postgres) (Database, error) { return &fakeDB{}, nil }

	err := Run(context.Background(), Options{ConfigPath: path, Opener: opener, Out: io.Discard})
	if err == nil {
		t.Fatal("Run = nil, want an error for a missing dev key")
	}
	if !strings.Contains(err.Error(), devconfig.KeyFileName) {
		t.Errorf("error = %q, want it to name the dev key", err)
	}
}

func getJSON(t *testing.T, url string, body any) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s status = %d, want %d", url, resp.StatusCode, http.StatusOK)
	}
	if err := json.NewDecoder(resp.Body).Decode(body); err != nil {
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

// signDevToken signs the claims a dev issuer token carries with the dev key.
func signDevToken(t *testing.T, key *ecdsa.PrivateKey, issuer string) string {
	t.Helper()

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, &jose.SignerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"iss": issuer,
		"aud": "abnch",
		"sub": devconfig.LocalCLISubject,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	token, err := jws.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}
