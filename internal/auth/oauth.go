package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jose "github.com/go-jose/go-jose/v4"

	"github.com/zon/abnch/internal/devconfig"
)

// workloadAlgorithms are the asymmetric algorithms a workload token may use.
// The service reads a token's issuer before it verifies the token, so it has to
// accept every algorithm a trusted issuer might sign with.
var workloadAlgorithms = []jose.SignatureAlgorithm{
	jose.RS256, jose.RS384, jose.RS512,
	jose.ES256, jose.ES384, jose.ES512,
	jose.PS256, jose.PS384, jose.PS512,
	jose.EdDSA,
}

// discoveryTimeout bounds the round trip that discovers an issuer's keys.
const discoveryTimeout = 10 * time.Second

// Workload authenticates workload OAuth tokens against the issuers the service
// config trusts. It discovers an issuer's keys the first time it verifies a
// token from that issuer.
func Workload(issuers []devconfig.Issuer) Authenticator {
	if len(issuers) == 0 {
		return Deny
	}
	w := &workload{issuers: issuers, verifiers: map[string]*oidc.IDTokenVerifier{}}
	return w.authenticate
}

// workload holds the trusted issuers and the verifiers it has discovered.
type workload struct {
	issuers []devconfig.Issuer

	mu        sync.Mutex
	verifiers map[string]*oidc.IDTokenVerifier
}

// authenticate returns the caller a workload token identifies. A token from an
// issuer the service does not trust, with a wrong audience, a bad signature, or
// an expiry in the past is rejected.
func (w *workload) authenticate(r *http.Request) (Caller, error) {
	raw, err := BearerToken(r)
	if err != nil {
		return Caller{}, err
	}

	iss, ok := w.trusted(tokenIssuer(raw))
	if !ok {
		return Caller{}, ErrUnauthenticated
	}

	verifier, err := w.verifier(r.Context(), iss)
	if err != nil {
		return Caller{}, ErrUnauthenticated
	}

	token, err := verifier.Verify(r.Context(), raw)
	if err != nil {
		return Caller{}, ErrUnauthenticated
	}
	return Caller{
		Subject: callerSubject(token, iss.RoleClaim),
		Roles:   mapRoles(claimValue(token, iss.RoleClaim), iss.RoleMap),
	}, nil
}

// trusted returns the configured issuer with the given URL.
func (w *workload) trusted(url string) (devconfig.Issuer, bool) {
	for _, iss := range w.issuers {
		if iss.Issuer == url {
			return iss, true
		}
	}
	return devconfig.Issuer{}, false
}

// verifier returns the verifier for an issuer, discovering its keys on first
// use.
func (w *workload) verifier(ctx context.Context, iss devconfig.Issuer) (*oidc.IDTokenVerifier, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if v, ok := w.verifiers[iss.Issuer]; ok {
		return v, nil
	}

	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()
	provider, err := oidc.NewProvider(ctx, iss.Issuer)
	if err != nil {
		return nil, err
	}

	v := provider.Verifier(&oidc.Config{ClientID: iss.Audience})
	w.verifiers[iss.Issuer] = v
	return v, nil
}

// callerSubject returns the caller identity a token carries: the value of the
// issuer's role claim, or the token subject when the claim is absent.
func callerSubject(token *oidc.IDToken, roleClaim string) string {
	if value, ok := claimValue(token, roleClaim).(string); ok && value != "" {
		return value
	}
	return token.Subject
}

// tokenIssuer returns the issuer a token names, read from its unverified
// claims. An empty result means the token is not a readable JWT.
func tokenIssuer(raw string) string {
	jws, err := jose.ParseSigned(raw, workloadAlgorithms)
	if err != nil {
		return ""
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(jws.UnsafePayloadWithoutVerification(), &claims); err != nil {
		return ""
	}
	return claims.Issuer
}
