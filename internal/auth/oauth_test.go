package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/zon/abnch/internal/devconfig"
)

// testIssuer serves the OIDC configuration and JWKS of a fake workload issuer
// and mints tokens with its own key.
type testIssuer struct {
	url   string
	key   *rsa.PrivateKey
	keyID string
}

func newTestIssuer(t *testing.T) *testIssuer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	iss := &testIssuer{key: key, keyID: "test-key"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", iss.discovery)
	mux.HandleFunc("/keys", iss.jwks)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	iss.url = srv.URL
	return iss
}

func (i *testIssuer) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                i.url,
		"jwks_uri":                              i.url + "/keys",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (i *testIssuer) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       i.key.Public(),
		KeyID:     i.keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}}})
}

// token signs claims into a workload token.
func (i *testIssuer) token(t *testing.T, claims map[string]any) string {
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
func (i *testIssuer) claims(extra map[string]any) map[string]any {
	claims := map[string]any{
		"iss": i.url,
		"aud": "abnch",
		"sub": "caller",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	for key, value := range extra {
		claims[key] = value
	}
	return claims
}

func bearer(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestWorkloadAcceptsTrustedToken(t *testing.T) {
	issuer := newTestIssuer(t)
	authenticate := Workload([]devconfig.Issuer{{Issuer: issuer.url, Audience: "abnch", RoleClaim: "sub"}})

	caller, err := authenticate(bearer(issuer.token(t, issuer.claims(map[string]any{"sub": "local-cli"}))))
	if err != nil {
		t.Fatalf("authenticate = %v, want no error", err)
	}
	if caller.Subject != "local-cli" {
		t.Errorf("caller subject = %q, want %q", caller.Subject, "local-cli")
	}
}

// TestWorkloadIdentifiesCallerByRoleClaim checks a workload whose identity is
// not the subject claim.
func TestWorkloadIdentifiesCallerByRoleClaim(t *testing.T) {
	issuer := newTestIssuer(t)
	authenticate := Workload([]devconfig.Issuer{{Issuer: issuer.url, Audience: "abnch", RoleClaim: "repository"}})

	token := issuer.token(t, issuer.claims(map[string]any{
		"sub":        "repo:zon/abnch:ref:refs/heads/main",
		"repository": "zon/abnch",
	}))

	caller, err := authenticate(bearer(token))
	if err != nil {
		t.Fatalf("authenticate = %v, want no error", err)
	}
	if caller.Subject != "zon/abnch" {
		t.Errorf("caller subject = %q, want the repository claim", caller.Subject)
	}
}

// TestWorkloadMapsRoleClaim checks a workload's role claim value is mapped
// through the trusted issuer's role map.
func TestWorkloadMapsRoleClaim(t *testing.T) {
	issuer := newTestIssuer(t)
	authenticate := Workload([]devconfig.Issuer{{
		Issuer:    issuer.url,
		Audience:  "abnch",
		RoleClaim: "sub",
		RoleMap:   map[string]string{"local-runtime": "Service"},
	}})

	caller, err := authenticate(bearer(issuer.token(t, issuer.claims(map[string]any{"sub": "local-runtime"}))))
	if err != nil {
		t.Fatalf("authenticate = %v, want no error", err)
	}
	if !reflect.DeepEqual(caller.Roles, []string{"Service"}) {
		t.Errorf("roles = %v, want [Service]", caller.Roles)
	}
	if caller.Subject != "local-runtime" {
		t.Errorf("subject = %q, want the role claim value", caller.Subject)
	}
}

// TestWorkloadMapsWorkloadRoles checks the workload roles Service, Agent, and
// Grader are each derived from a mapped claim value.
func TestWorkloadMapsWorkloadRoles(t *testing.T) {
	issuer := newTestIssuer(t)
	authenticate := Workload([]devconfig.Issuer{{
		Issuer:    issuer.url,
		Audience:  "abnch",
		RoleClaim: "sub",
		RoleMap: map[string]string{
			"svc":    "Service",
			"agent":  "Agent",
			"grader": "Grader",
		},
	}})

	for _, tc := range []struct {
		claim string
		want  string
	}{
		{claim: "svc", want: "Service"},
		{claim: "agent", want: "Agent"},
		{claim: "grader", want: "Grader"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			caller, err := authenticate(bearer(issuer.token(t, issuer.claims(map[string]any{"sub": tc.claim}))))
			if err != nil {
				t.Fatalf("authenticate = %v, want no error", err)
			}
			if !reflect.DeepEqual(caller.Roles, []string{tc.want}) {
				t.Errorf("roles = %v, want [%s]", caller.Roles, tc.want)
			}
		})
	}
}

// TestWorkloadUnmappedClaimGrantsNoRole checks a workload whose role claim value
// has no mapping holds no role.
func TestWorkloadUnmappedClaimGrantsNoRole(t *testing.T) {
	issuer := newTestIssuer(t)
	authenticate := Workload([]devconfig.Issuer{{
		Issuer:    issuer.url,
		Audience:  "abnch",
		RoleClaim: "sub",
		RoleMap:   map[string]string{"local-runtime": "Service"},
	}})

	caller, err := authenticate(bearer(issuer.token(t, issuer.claims(map[string]any{"sub": "stranger"}))))
	if err != nil {
		t.Fatalf("authenticate = %v, want no error", err)
	}
	if len(caller.Roles) != 0 {
		t.Errorf("roles = %v, want none for an unmapped claim", caller.Roles)
	}
}

// TestWorkloadMapsListClaim checks each value of a list role claim is mapped and
// an unmapped value grants no role.
func TestWorkloadMapsListClaim(t *testing.T) {
	issuer := newTestIssuer(t)
	authenticate := Workload([]devconfig.Issuer{{
		Issuer:    issuer.url,
		Audience:  "abnch",
		RoleClaim: "roles",
		RoleMap:   map[string]string{"service": "Service", "grader": "Grader"},
	}})

	caller, err := authenticate(bearer(issuer.token(t, issuer.claims(map[string]any{
		"roles": []any{"service", "stranger", "grader"},
	}))))
	if err != nil {
		t.Fatalf("authenticate = %v, want no error", err)
	}
	if !reflect.DeepEqual(caller.Roles, []string{"Service", "Grader"}) {
		t.Errorf("roles = %v, want [Service Grader]", caller.Roles)
	}
}

func TestWorkloadRejectsUntrustedToken(t *testing.T) {
	trusted := newTestIssuer(t)
	other := newTestIssuer(t)
	authenticate := Workload([]devconfig.Issuer{{Issuer: trusted.url, Audience: "abnch", RoleClaim: "sub"}})

	for _, tc := range []struct {
		name  string
		token string
	}{
		{name: "unknown issuer", token: other.token(t, other.claims(nil))},
		{name: "wrong audience", token: trusted.token(t, trusted.claims(map[string]any{"aud": "someone-else"}))},
		{name: "bad signature", token: other.token(t, trusted.claims(nil))},
		{name: "expired", token: trusted.token(t, trusted.claims(map[string]any{"exp": time.Now().Add(-time.Hour).Unix()}))},
		{name: "not a token", token: "not-a-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := authenticate(bearer(tc.token)); err == nil {
				t.Error("authenticate = nil, want an error")
			}
		})
	}
}

func TestWorkloadWithoutIssuersDenies(t *testing.T) {
	authenticate := Workload(nil)

	if _, err := authenticate(bearer("any-token")); err == nil {
		t.Error("authenticate = nil, want an error when no issuer is trusted")
	}
}

// TestWorkloadMiddlewareRejectsUntrustedToken checks the rejected token reaches
// the caller as 401 while a trusted token reaches the handler.
func TestWorkloadMiddlewareRejectsUntrustedToken(t *testing.T) {
	trusted := newTestIssuer(t)
	other := newTestIssuer(t)
	authenticate := Workload([]devconfig.Issuer{{Issuer: trusted.url, Audience: "abnch", RoleClaim: "sub"}})

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	for _, tc := range []struct {
		name  string
		token string
		want  int
	}{
		{name: "trusted issuer", token: trusted.token(t, trusted.claims(nil)), want: http.StatusNoContent},
		{name: "unknown issuer", token: other.token(t, other.claims(nil)), want: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Middleware(authenticate, next).ServeHTTP(rec, bearer(tc.token))

			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
