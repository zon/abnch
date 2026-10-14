package service

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// workloadIssuer serves the OIDC configuration and JWKS of a fake workload
// issuer and mints tokens with its own key.
type workloadIssuer struct {
	url   string
	key   *rsa.PrivateKey
	keyID string
}

func newWorkloadIssuer(t *testing.T) *workloadIssuer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	iss := &workloadIssuer{key: key, keyID: "test-key"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", iss.discovery)
	mux.HandleFunc("/keys", iss.jwks)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	iss.url = srv.URL
	return iss
}

func (i *workloadIssuer) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                i.url,
		"jwks_uri":                              i.url + "/keys",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (i *workloadIssuer) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       i.key.Public(),
		KeyID:     i.keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}})
}

// token signs a valid claim set from the issuer into a workload token.
func (i *workloadIssuer) token(t *testing.T) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: i.key},
		(&jose.SignerOptions{}).WithHeader("kid", i.keyID),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"iss": i.url,
		"aud": "abnch",
		"sub": "local-cli",
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

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// workloadConfig is a service config that trusts one workload issuer.
func workloadConfig(port int, issuerURL string) string {
	return validConfig(port) + fmt.Sprintf("oauth:\n"+
		"  issuers:\n"+
		"    - issuer: %s\n"+
		"      audience: abnch\n"+
		"      roleClaim: sub\n"+
		"      roleMap:\n"+
		"        local-cli: Admin\n", issuerURL)
}

// TestRunAuthenticatesWorkloadToken checks the service authenticates a token
// from an issuer in its config and rejects a token from an issuer it does not
// trust.
func TestRunAuthenticatesWorkloadToken(t *testing.T) {
	trusted := newWorkloadIssuer(t)
	other := newWorkloadIssuer(t)

	port := freePort(t)
	opts := Options{ConfigPath: writeConfig(t, workloadConfig(port, trusted.url))}
	base := startServiceWith(t, port, opts)

	for _, tc := range []struct {
		name string
		auth string
		want int
	}{
		{name: "trusted issuer", auth: "Bearer " + trusted.token(t), want: http.StatusNotFound},
		{name: "unknown issuer", auth: "Bearer " + other.token(t), want: http.StatusUnauthorized},
		{name: "no credentials", want: http.StatusUnauthorized},
		{name: "unverifiable credentials", auth: "Bearer not-a-token", want: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, base, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}
