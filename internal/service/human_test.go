package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/zon/abnch/internal/devconfig"
)

// humanProvider is a fake human identity provider that serves the OIDC
// configuration, JWKS, and token endpoint, and mints an ID token with its own
// key.
type humanProvider struct {
	url      string
	clientID string
	key      *rsa.PrivateKey
	keyID    string

	nonce string
	role  string
}

func newHumanProvider(t *testing.T) *humanProvider {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &humanProvider{clientID: "client", key: key, keyID: "human-key", role: "admin"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("/keys", p.jwks)
	mux.HandleFunc("/token", p.token)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p.url = srv.URL
	return p
}

func (p *humanProvider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                p.url,
		"authorization_endpoint":                p.url + "/authorize",
		"token_endpoint":                        p.url + "/token",
		"jwks_uri":                              p.url + "/keys",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (p *humanProvider) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       p.key.Public(),
		KeyID:     p.keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}})
}

func (p *humanProvider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := p.idToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"access_token": "access",
		"token_type":   "Bearer",
		"id_token":     id,
	})
}

func (p *humanProvider) idToken() (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: p.key},
		(&jose.SignerOptions{}).WithHeader("kid", p.keyID),
	)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"iss":   p.url,
		"aud":   p.clientID,
		"sub":   "alice",
		"roles": p.role,
		"nonce": p.nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return jws.CompactSerialize()
}

// humanConfig renders a service config that signs humans in through the
// provider.
func humanConfig(t *testing.T, port int, provider *humanProvider) string {
	t.Helper()

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	cfg := devconfig.Config{
		URL:      url,
		Postgres: testPostgres(),
		OIDC: devconfig.OIDC{
			Issuer:       provider.url,
			ClientID:     provider.clientID,
			ClientSecret: "secret",
			RedirectURL:  url + "/auth/callback",
			Scopes:       []string{"email"},
			RoleClaim:    "roles",
			RoleMap:      map[string]string{"admin": "Admin"},
		},
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestRunServesHumanSignIn checks the service starts a sign-in at the
// configured provider and grants a session a later request authenticates with.
func TestRunServesHumanSignIn(t *testing.T) {
	provider := newHumanProvider(t)
	port := freePort(t)
	base := startServiceWith(t, port, Options{ConfigPath: writeConfig(t, humanConfig(t, port, provider))})

	login := getNoRedirect(t, base+"login?redirect_uri="+url.QueryEscape("http://127.0.0.1:9/callback"))
	defer login.Body.Close()
	if login.StatusCode != http.StatusFound {
		t.Fatalf("login status = %d, want %d", login.StatusCode, http.StatusFound)
	}

	location, err := url.Parse(login.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(location.String(), provider.url+"/authorize") {
		t.Errorf("login redirected to %q, want the provider", location)
	}
	params := location.Query()
	if params.Get("state") == "" || params.Get("nonce") == "" {
		t.Fatalf("authorize URL = %q, want a state and nonce", location)
	}

	provider.nonce = params.Get("nonce")
	callback := getNoRedirect(t, base+"auth/callback?state="+params.Get("state")+"&code=abc")
	defer callback.Body.Close()

	redirect, err := url.Parse(callback.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	session := redirect.Query().Get("session")
	if session == "" {
		t.Fatal("sign-in granted no session")
	}

	req, err := http.NewRequest(http.MethodGet, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d for a session-authenticated request", resp.StatusCode, http.StatusNotFound)
	}
}

// TestRunRejectsLoginWithoutRedirectURI checks the service does not start a
// sign-in without a place to send the session.
func TestRunRejectsLoginWithoutRedirectURI(t *testing.T) {
	provider := newHumanProvider(t)
	port := freePort(t)
	base := startServiceWith(t, port, Options{ConfigPath: writeConfig(t, humanConfig(t, port, provider))})

	resp := getNoRedirect(t, base+"login")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestRunRejectsIncompleteOIDC checks the service reports a clear error when
// the config names a provider it cannot sign in through.
func TestRunRejectsIncompleteOIDC(t *testing.T) {
	port := freePort(t)
	cfg := devconfig.Config{
		URL:      fmt.Sprintf("http://127.0.0.1:%d", port),
		Postgres: testPostgres(),
		OIDC:     devconfig.OIDC{Issuer: "https://id.example.com"},
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	opener := func(context.Context, devconfig.Postgres) (Store, error) { return &fakeDB{}, nil }
	err = Run(context.Background(), Options{ConfigPath: writeConfig(t, string(data)), Opener: opener, Out: io.Discard})
	if err == nil {
		t.Fatal("Run = nil, want an error for an incomplete oidc config")
	}
	if !strings.Contains(err.Error(), "oidc") {
		t.Errorf("error = %q, want it to name the oidc config", err)
	}
}

// getNoRedirect sends a GET without following redirects, so a test observes a
// sign-in redirect itself.
func getNoRedirect(t *testing.T, rawURL string) *http.Response {
	t.Helper()

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
