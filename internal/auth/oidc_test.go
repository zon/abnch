package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/zon/abnch/internal/devconfig"
)

// oidcProvider is a fake human identity provider. It serves the OIDC
// configuration, JWKS, and token endpoint, and mints ID tokens with its own key.
// A test sets nonce and claims before it completes a sign-in.
type oidcProvider struct {
	url      string
	clientID string
	key      *rsa.PrivateKey
	keyID    string

	nonce  string
	claims map[string]any
}

func newOIDCProvider(t *testing.T) *oidcProvider {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &oidcProvider{clientID: "client", key: key, keyID: "oidc-key"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("/keys", p.jwks)
	mux.HandleFunc("/token", p.token)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p.url = srv.URL
	return p
}

func (p *oidcProvider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                p.url,
		"authorization_endpoint":                p.url + "/authorize",
		"token_endpoint":                        p.url + "/token",
		"jwks_uri":                              p.url + "/keys",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (p *oidcProvider) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       p.key.Public(),
		KeyID:     p.keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}})
}

// token exchanges any code for an ID token carrying the provider's nonce and
// claims.
func (p *oidcProvider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if r.Form.Get("code") == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
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

func (p *oidcProvider) idToken() (string, error) {
	claims := map[string]any{
		"iss":   p.url,
		"aud":   p.clientID,
		"sub":   "alice",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"nonce": p.nonce,
	}
	for key, value := range p.claims {
		claims[key] = value
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: p.key},
		(&jose.SignerOptions{}).WithHeader("kid", p.keyID),
	)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return jws.CompactSerialize()
}

// testHuman builds a sign-in against the provider with the given role map.
func testHuman(t *testing.T, provider *oidcProvider, roleMap map[string]string) *Human {
	t.Helper()

	human, err := NewHuman(devconfig.OIDC{
		Issuer:       provider.url,
		ClientID:     provider.clientID,
		ClientSecret: "secret",
		RedirectURL:  "http://service.example/auth/callback",
		Scopes:       []string{"email"},
		RoleClaim:    "roles",
		RoleMap:      roleMap,
	})
	if err != nil {
		t.Fatal(err)
	}
	return human
}

// humanServer mounts a sign-in and returns its server.
func humanServer(t *testing.T, human *Human) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	human.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// startSignIn begins a flow and returns the state and nonce the provider
// carries.
func startSignIn(t *testing.T, srv *httptest.Server, redirectURI string) url.Values {
	t.Helper()

	resp := getNoRedirect(t, srv.URL+"/login?redirect_uri="+url.QueryEscape(redirectURI))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusFound)
	}

	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return location.Query()
}

// TestHumanSignInGrantsSession checks a completed sign-in grants a session that
// identifies the user and carries the mapped role.
func TestHumanSignInGrantsSession(t *testing.T) {
	provider := newOIDCProvider(t)
	human := testHuman(t, provider, map[string]string{"admin": "Admin"})
	srv := humanServer(t, human)

	authParams := startSignIn(t, srv, "http://127.0.0.1:9/callback")
	if !strings.HasPrefix(authParams.Get("client_id"), provider.clientID) {
		t.Errorf("client_id = %q, want %q", authParams.Get("client_id"), provider.clientID)
	}
	if !strings.Contains(authParams.Get("scope"), "openid") {
		t.Errorf("scope = %q, want it to include openid", authParams.Get("scope"))
	}

	provider.nonce = authParams.Get("nonce")
	provider.claims = map[string]any{"roles": "admin"}

	resp := getNoRedirect(t, srv.URL+"/auth/callback?state="+authParams.Get("state")+"&code=abc")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("callback status = %d, want %d", resp.StatusCode, http.StatusFound)
	}

	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Scheme != "http" || location.Host != "127.0.0.1:9" {
		t.Errorf("session sent to %q, want the redirect_uri", location)
	}

	token := location.Query().Get("session")
	if token == "" {
		t.Fatal("callback granted no session")
	}
	session, err := human.sessions.Read(token)
	if err != nil {
		t.Fatalf("Read = %v, want the granted session", err)
	}
	if session.Subject != "alice" {
		t.Errorf("subject = %q, want alice", session.Subject)
	}
	if !reflect.DeepEqual(session.Roles, []string{"Admin"}) {
		t.Errorf("roles = %v, want [Admin]", session.Roles)
	}
}

// TestHumanSignInUnmappedClaimGrantsNoRole checks a claim value the role map
// does not name grants no role.
func TestHumanSignInUnmappedClaimGrantsNoRole(t *testing.T) {
	provider := newOIDCProvider(t)
	human := testHuman(t, provider, map[string]string{"admin": "Admin"})
	srv := humanServer(t, human)

	authParams := startSignIn(t, srv, "http://127.0.0.1:9/callback")
	provider.nonce = authParams.Get("nonce")
	provider.claims = map[string]any{"roles": "stranger"}

	resp := getNoRedirect(t, srv.URL+"/auth/callback?state="+authParams.Get("state")+"&code=abc")
	defer resp.Body.Close()

	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	token := location.Query().Get("session")
	if token == "" {
		t.Fatal("callback granted no session")
	}
	session, err := human.sessions.Read(token)
	if err != nil {
		t.Fatal(err)
	}
	if len(session.Roles) != 0 {
		t.Errorf("roles = %v, want none for an unmapped claim", session.Roles)
	}
}

// TestHumanSignInMapsListClaim checks each value of a role claim that carries a
// list is mapped, and an unmapped value grants no role.
func TestHumanSignInMapsListClaim(t *testing.T) {
	provider := newOIDCProvider(t)
	human := testHuman(t, provider, map[string]string{"admin": "Admin"})
	srv := humanServer(t, human)

	authParams := startSignIn(t, srv, "http://127.0.0.1:9/callback")
	provider.nonce = authParams.Get("nonce")
	provider.claims = map[string]any{"roles": []any{"admin", "stranger"}}

	resp := getNoRedirect(t, srv.URL+"/auth/callback?state="+authParams.Get("state")+"&code=abc")
	defer resp.Body.Close()

	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	token := location.Query().Get("session")
	if token == "" {
		t.Fatal("callback granted no session")
	}
	session, err := human.sessions.Read(token)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(session.Roles, []string{"Admin"}) {
		t.Errorf("roles = %v, want [Admin]", session.Roles)
	}
}

// TestHumanSignInProviderErrorGrantsNoSession checks the provider rejecting the
// sign-in returns the failure and grants no session.
func TestHumanSignInProviderErrorGrantsNoSession(t *testing.T) {
	provider := newOIDCProvider(t)
	human := testHuman(t, provider, nil)
	srv := humanServer(t, human)

	authParams := startSignIn(t, srv, "http://127.0.0.1:9/callback")

	resp := getNoRedirect(t, srv.URL+"/auth/callback?state="+authParams.Get("state")+"&error=access_denied")
	defer resp.Body.Close()

	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Query().Get("session") != "" {
		t.Error("a failed sign-in granted a session")
	}
	if location.Query().Get("error") == "" {
		t.Error("a failed sign-in did not report the failure")
	}
}

// TestHumanSignInRejectsInvalidState checks the callback rejects a state the
// service never issued.
func TestHumanSignInRejectsInvalidState(t *testing.T) {
	provider := newOIDCProvider(t)
	human := testHuman(t, provider, nil)
	srv := humanServer(t, human)

	resp := getNoRedirect(t, srv.URL+"/auth/callback?state=unknown&code=abc")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestHumanSignInRejectsUnverifiedToken checks a token the provider did not
// issue for the service grants no session.
func TestHumanSignInRejectsUnverifiedToken(t *testing.T) {
	provider := newOIDCProvider(t)
	human := testHuman(t, provider, nil)
	srv := humanServer(t, human)

	authParams := startSignIn(t, srv, "http://127.0.0.1:9/callback")
	provider.nonce = authParams.Get("nonce")
	provider.claims = map[string]any{"aud": "someone-else"}

	resp := getNoRedirect(t, srv.URL+"/auth/callback?state="+authParams.Get("state")+"&code=abc")
	defer resp.Body.Close()

	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Query().Get("session") != "" {
		t.Error("an unverified token granted a session")
	}
}

// TestHumanSignInNeedsRedirectURI checks the flow does not start without a
// place to send the session.
func TestHumanSignInNeedsRedirectURI(t *testing.T) {
	provider := newOIDCProvider(t)
	human := testHuman(t, provider, nil)
	srv := humanServer(t, human)

	resp := getNoRedirect(t, srv.URL+"/login")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestNewHumanRejectsIncompleteConfig(t *testing.T) {
	complete := devconfig.OIDC{
		Issuer:       "https://id.example.com",
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURL:  "https://service.example/auth/callback",
	}

	for _, tc := range []struct {
		name string
		mut  func(*devconfig.OIDC)
	}{
		{name: "issuer", mut: func(c *devconfig.OIDC) { c.Issuer = "" }},
		{name: "clientId", mut: func(c *devconfig.OIDC) { c.ClientID = "" }},
		{name: "clientSecret", mut: func(c *devconfig.OIDC) { c.ClientSecret = "" }},
		{name: "redirectUrl", mut: func(c *devconfig.OIDC) { c.RedirectURL = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := complete
			tc.mut(&cfg)
			if _, err := NewHuman(cfg); err == nil {
				t.Errorf("NewHuman = nil, want an error for a missing %s", tc.name)
			}
		})
	}
}

// getNoRedirect sends a GET without following redirects, so a test observes the
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
