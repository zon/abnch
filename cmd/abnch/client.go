package main

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/zon/abnch/internal/devconfig"
)

// localTokenTTL bounds how long a token the CLI mints from a local credential
// stays valid.
const localTokenTTL = 5 * time.Minute

// serviceClient reaches a service with the credential stored for its URL.
type serviceClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// newServiceClient resolves the credential for the service URL and returns a
// client that presents it on every request. It fails telling the user to sign
// in when the URL has no usable credential.
func newServiceClient(deps deps, serviceURL string) (*serviceClient, error) {
	token, err := serviceCredential(deps.homeDir, serviceURL)
	if err != nil {
		return nil, err
	}
	return &serviceClient{
		baseURL: strings.TrimRight(serviceURL, "/"),
		token:   token,
		http:    http.DefaultClient,
	}, nil
}

// do sends a request to the service with the caller's credential presented.
func (c *serviceClient) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return c.http.Do(req)
}

// serviceCredential returns the bearer token the CLI presents to the service at
// serviceURL. A stored session wins while it is valid, so a signed-in user is
// used. A local credential is the fallback that needs no sign-in. A URL with
// neither, or with nothing but an expired session, fails telling the user to
// sign in.
func serviceCredential(homeDir, serviceURL string) (string, error) {
	creds, err := devconfig.LoadCredentials(homeDir)
	if err != nil {
		return "", fmt.Errorf("failed to read the credential store: %w", err)
	}

	cred := creds[serviceURL]
	if cred.Session != "" && !sessionExpired(cred.Session) {
		return cred.Session, nil
	}
	if cred.Issuer != "" {
		return localToken(cred)
	}
	return "", signInError(serviceURL)
}

// signInError tells the user how to get a credential for a service URL.
func signInError(serviceURL string) error {
	return fmt.Errorf("no session for %s: run `abnch login`", serviceURL)
}

// sessionExpired reports whether a stored session carries an expiry in the
// past. A token the CLI cannot read is left for the service to judge.
func sessionExpired(token string) bool {
	jws, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.HS256})
	if err != nil {
		return false
	}
	var claims struct {
		Expiry int64 `json:"exp"`
	}
	if err := json.Unmarshal(jws.UnsafePayloadWithoutVerification(), &claims); err != nil {
		return false
	}
	return claims.Expiry > 0 && time.Now().Unix() >= claims.Expiry
}

// localToken signs a workload token from a local credential, so the CLI reaches
// a local service as the dev issuer's subject without signing in.
func localToken(cred devconfig.Credential) (string, error) {
	key, err := devconfig.LoadDevKey(cred.KeyFile)
	if err != nil {
		return "", fmt.Errorf("failed to read the local credential key: %w", err)
	}

	thumbprint, err := (&jose.JSONWebKey{Key: key.Public()}).Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}
	options := (&jose.SignerOptions{}).WithHeader("kid", base64.RawURLEncoding.EncodeToString(thumbprint))
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, options)
	if err != nil {
		return "", err
	}

	now := time.Now()
	payload, err := json.Marshal(map[string]any{
		"iss": cred.Issuer,
		"aud": cred.Audience,
		"sub": cred.Subject,
		"iat": now.Unix(),
		"exp": now.Add(localTokenTTL).Unix(),
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
