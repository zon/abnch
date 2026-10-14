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

	"github.com/zon/abnch/internal/devconfig"
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

// token signs claims from the issuer into a workload token.
func (i *workloadIssuer) token(t *testing.T, claims map[string]any) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: i.key},
		(&jose.SignerOptions{}).WithHeader("kid", i.keyID),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
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

// claims returns a valid claim set from the issuer, with extra overriding it.
func (i *workloadIssuer) claims(extra map[string]any) map[string]any {
	claims := map[string]any{
		"iss": i.url,
		"aud": "abnch",
		"sub": "local-cli",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	for key, value := range extra {
		claims[key] = value
	}
	return claims
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
		{name: "trusted issuer", auth: "Bearer " + trusted.token(t, trusted.claims(nil)), want: http.StatusNotFound},
		{name: "unknown issuer", auth: "Bearer " + other.token(t, other.claims(nil)), want: http.StatusUnauthorized},
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

// TestRunAcceptsWorkloadCredentials checks the service accepts a Kubernetes
// service account token, a GitHub workflow token, and a token from the local
// dev issuer as workload credentials, and enforces the role each maps to. A
// rejected credential would answer 401.
func TestRunAcceptsWorkloadCredentials(t *testing.T) {
	kubernetes := newWorkloadIssuer(t)
	github := newWorkloadIssuer(t)

	port := freePort(t)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	path := writeConfig(t, configYAML(t, url, []devconfig.Issuer{
		{
			Issuer: kubernetes.url, Audience: "abnch", RoleClaim: "sub",
			RoleMap: map[string]string{"system:serviceaccount:abnch:agent-runner": "Service"},
		},
		{
			Issuer: github.url, Audience: "abnch", RoleClaim: "repository",
			RoleMap: map[string]string{"zon/abnch": "Agent"},
		},
		devconfig.DevIssuer(url),
	}))
	if err := devconfig.EnsureDevKey(devconfig.KeyPath(path)); err != nil {
		t.Fatal(err)
	}
	base := startServiceWith(t, port, Options{ConfigPath: path})

	devKey, err := devconfig.LoadDevKey(devconfig.KeyPath(path))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		token string
		want  int
	}{
		{
			name: "kubernetes service account",
			token: kubernetes.token(t, kubernetes.claims(map[string]any{
				"aud": []string{"abnch"},
				"sub": "system:serviceaccount:abnch:agent-runner",
			})),
			want: http.StatusNotFound,
		},
		{
			name: "github workflow",
			token: github.token(t, github.claims(map[string]any{
				"sub":        "repo:zon/abnch:ref:refs/heads/main",
				"repository": "zon/abnch",
			})),
			want: http.StatusForbidden,
		},
		{
			name:  "local dev issuer",
			token: signDevToken(t, devKey, url+"/dev"),
			want:  http.StatusNotFound,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, base, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+tc.token)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Errorf("status = %d, want %d for an authenticated request", resp.StatusCode, tc.want)
			}
		})
	}
}
