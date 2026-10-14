package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/devconfig"
)

// captureAuth records the Authorization header of the first request a service
// receives.
func captureAuth(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	header := new(string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*header = r.Header.Get("Authorization")
	}))
	t.Cleanup(srv.Close)
	return srv, header
}

// storeCredential writes one credential to the store under home.
func storeCredential(t *testing.T, home, url string, cred devconfig.Credential) {
	t.Helper()
	data, err := devconfig.MarshalCredentials(map[string]devconfig.Credential{url: cred})
	if err != nil {
		t.Fatal(err)
	}
	path := devconfig.CredentialsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// expiredSessionToken signs a session token whose expiry has passed.
func expiredSessionToken(t *testing.T) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte("session-signing-secret-32-bytes!!")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"sub": "user", "exp": time.Now().Add(-time.Hour).Unix()})
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

// verifyLocalToken verifies a token the CLI presented against the dev key and
// returns its claims.
func verifyLocalToken(t *testing.T, token, keyPath string) map[string]any {
	t.Helper()
	key, err := devconfig.LoadDevKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	jws, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.ES256})
	if err != nil {
		t.Fatalf("token is not a signed JWT: %v", err)
	}
	payload, err := jws.Verify(key.Public())
	if err != nil {
		t.Fatalf("token signature: %v", err)
	}
	claims := map[string]any{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	return claims
}

func TestServiceClientLocalCredentialIsAcceptedByService(t *testing.T) {
	home := t.TempDir()
	keyPath := filepath.Join(t.TempDir(), "dev-issuer.key")
	if err := devconfig.EnsureDevKey(keyPath); err != nil {
		t.Fatal(err)
	}
	key, err := devconfig.LoadDevKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	srv.Start()
	t.Cleanup(srv.Close)

	dev, err := auth.NewDevIssuer(srv.URL+"/dev", key)
	if err != nil {
		t.Fatal(err)
	}
	dev.Register(mux)

	var caller auth.Caller
	mux.Handle("/api", auth.Middleware(
		auth.Workload([]devconfig.Issuer{devconfig.DevIssuer(srv.URL)}),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			caller, _ = auth.CallerFrom(r.Context())
			w.WriteHeader(http.StatusNoContent)
		}),
	))

	storeCredential(t, home, srv.URL, devconfig.LocalCredential(srv.URL, keyPath))
	client, err := newServiceClient(deps{homeDir: home, ctx: context.Background()}, srv.URL)
	if err != nil {
		t.Fatalf("newServiceClient = %v, want no error", err)
	}
	resp, err := client.do(context.Background(), http.MethodGet, "/api", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want the service to accept the local credential", resp.StatusCode)
	}
	if caller.Subject != devconfig.LocalCLISubject {
		t.Errorf("caller subject = %q, want %q", caller.Subject, devconfig.LocalCLISubject)
	}
}

func TestServiceClientPresentsStoredSession(t *testing.T) {
	home := t.TempDir()
	srv, header := captureAuth(t)
	if err := devconfig.StoreSession(home, srv.URL, "session-token"); err != nil {
		t.Fatal(err)
	}

	client, err := newServiceClient(deps{homeDir: home, ctx: context.Background()}, srv.URL)
	if err != nil {
		t.Fatalf("newServiceClient = %v, want no error", err)
	}
	resp, err := client.do(context.Background(), http.MethodGet, "/repos", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got := *header; got != "Bearer session-token" {
		t.Errorf("Authorization = %q, want the stored session presented", got)
	}
}

func TestServiceClientPresentsLocalCredential(t *testing.T) {
	home := t.TempDir()
	keyPath := filepath.Join(t.TempDir(), "dev-issuer.key")
	if err := devconfig.EnsureDevKey(keyPath); err != nil {
		t.Fatal(err)
	}
	srv, header := captureAuth(t)
	storeCredential(t, home, srv.URL, devconfig.LocalCredential(srv.URL, keyPath))

	client, err := newServiceClient(deps{homeDir: home, ctx: context.Background()}, srv.URL)
	if err != nil {
		t.Fatalf("newServiceClient = %v, want no error", err)
	}
	resp, err := client.do(context.Background(), http.MethodGet, "/repos", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	token, ok := strings.CutPrefix(*header, "Bearer ")
	if !ok {
		t.Fatalf("Authorization = %q, want a bearer token", *header)
	}
	claims := verifyLocalToken(t, token, keyPath)
	if claims["iss"] != srv.URL+"/dev" {
		t.Errorf("iss = %v, want the dev issuer", claims["iss"])
	}
	if claims["aud"] != "abnch" {
		t.Errorf("aud = %v, want the dev issuer audience", claims["aud"])
	}
	if claims["sub"] != devconfig.LocalCLISubject {
		t.Errorf("sub = %v, want the local CLI subject", claims["sub"])
	}
	if exp, _ := claims["exp"].(float64); time.Now().Unix() >= int64(exp) {
		t.Errorf("exp = %v, want an expiry in the future", claims["exp"])
	}
}

func TestServiceClientPrefersStoredSession(t *testing.T) {
	home := t.TempDir()
	keyPath := filepath.Join(t.TempDir(), "dev-issuer.key")
	if err := devconfig.EnsureDevKey(keyPath); err != nil {
		t.Fatal(err)
	}
	srv, header := captureAuth(t)
	cred := devconfig.LocalCredential(srv.URL, keyPath)
	cred.Session = "stored-session"
	storeCredential(t, home, srv.URL, cred)

	client, err := newServiceClient(deps{homeDir: home, ctx: context.Background()}, srv.URL)
	if err != nil {
		t.Fatalf("newServiceClient = %v, want no error", err)
	}
	resp, err := client.do(context.Background(), http.MethodGet, "/repos", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got := *header; got != "Bearer stored-session" {
		t.Errorf("Authorization = %q, want the stored session presented", got)
	}
}

func TestServiceClientLocalCredentialFallsBackFromExpiredSession(t *testing.T) {
	home := t.TempDir()
	keyPath := filepath.Join(t.TempDir(), "dev-issuer.key")
	if err := devconfig.EnsureDevKey(keyPath); err != nil {
		t.Fatal(err)
	}
	srv, header := captureAuth(t)
	cred := devconfig.LocalCredential(srv.URL, keyPath)
	cred.Session = expiredSessionToken(t)
	storeCredential(t, home, srv.URL, cred)

	client, err := newServiceClient(deps{homeDir: home, ctx: context.Background()}, srv.URL)
	if err != nil {
		t.Fatalf("newServiceClient = %v, want a local credential to fall back on", err)
	}
	resp, err := client.do(context.Background(), http.MethodGet, "/repos", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	token, ok := strings.CutPrefix(*header, "Bearer ")
	if !ok {
		t.Fatalf("Authorization = %q, want a bearer token", *header)
	}
	if token == cred.Session {
		t.Error("Authorization presented the expired session, want the local credential")
	}
	verifyLocalToken(t, token, keyPath)
}

func TestServiceClientFailsWithoutCredential(t *testing.T) {
	home := t.TempDir()

	_, err := newServiceClient(deps{homeDir: home, ctx: context.Background()}, "https://bench.example.com")
	if err == nil {
		t.Fatal("newServiceClient = nil, want an error without a credential")
	}
	if !strings.Contains(err.Error(), "abnch login") {
		t.Errorf("error = %q, want it to tell the user to run abnch login", err)
	}
}

func TestServiceClientFailsWithExpiredSession(t *testing.T) {
	home := t.TempDir()
	service := "https://bench.example.com"
	if err := devconfig.StoreSession(home, service, expiredSessionToken(t)); err != nil {
		t.Fatal(err)
	}

	_, err := newServiceClient(deps{homeDir: home, ctx: context.Background()}, service)
	if err == nil {
		t.Fatal("newServiceClient = nil, want an error for an expired session")
	}
	if !strings.Contains(err.Error(), "abnch login") {
		t.Errorf("error = %q, want it to tell the user to run abnch login", err)
	}
}

func TestServiceClientPresentsUnreadableSession(t *testing.T) {
	home := t.TempDir()
	srv, header := captureAuth(t)
	if err := devconfig.StoreSession(home, srv.URL, "opaque-token"); err != nil {
		t.Fatal(err)
	}

	client, err := newServiceClient(deps{homeDir: home, ctx: context.Background()}, srv.URL)
	if err != nil {
		t.Fatalf("newServiceClient = %v, want no error", err)
	}
	resp, err := client.do(context.Background(), http.MethodGet, "/repos", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got := *header; got != "Bearer opaque-token" {
		t.Errorf("Authorization = %q, want the unreadable session presented", got)
	}
}
