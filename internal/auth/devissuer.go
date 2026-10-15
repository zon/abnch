package auth

import (
	"crypto"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	jose "github.com/go-jose/go-jose/v4"
)

// DevIssuer serves the OIDC configuration and JWKS of the dev issuer a local
// service hosts. A dev issuer signs its tokens with a dev key, and a caller
// verifies them against the public half this serves.
type DevIssuer struct {
	issuer string
	base   string
	key    *ecdsa.PrivateKey
	keyID  string
}

// NewDevIssuer builds the dev issuer served at issuerURL and signed with key.
func NewDevIssuer(issuerURL string, key *ecdsa.PrivateKey) (*DevIssuer, error) {
	u, err := url.Parse(issuerURL)
	if err != nil {
		return nil, fmt.Errorf("dev issuer url %q is not valid: %w", issuerURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("dev issuer url %q is missing a scheme or host", issuerURL)
	}

	keyID, err := keyID(key)
	if err != nil {
		return nil, err
	}
	return &DevIssuer{
		issuer: issuerURL,
		base:   strings.TrimSuffix(u.Path, "/"),
		key:    key,
		keyID:  keyID,
	}, nil
}

// Register mounts the dev issuer's public routes on mux: its OIDC
// configuration and its JWKS.
func (d *DevIssuer) Register(mux *http.ServeMux) {
	mux.HandleFunc(d.base+"/.well-known/openid-configuration", d.configuration)
	mux.HandleFunc(d.base+"/keys", d.jwks)
}

// configuration serves the OIDC discovery document. It names the issuer and the
// signing algorithm, so a client can find the JWKS and verify a token.
func (d *DevIssuer) configuration(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                d.issuer,
		"jwks_uri":                              d.jwksURL(),
		"id_token_signing_alg_values_supported": []string{string(jose.ES256)},
	})
}

// jwks serves the public half of the dev key, so a client can verify a token
// the dev issuer signs.
func (d *DevIssuer) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       d.key.Public(),
		KeyID:     d.keyID,
		Algorithm: string(jose.ES256),
		Use:       "sig",
	}}})
}

// jwksURL is the URL the configuration points clients at for keys.
func (d *DevIssuer) jwksURL() string {
	return strings.TrimSuffix(d.issuer, "/") + "/keys"
}

// keyID returns the RFC 7638 thumbprint of a key, so a token can name the key
// it was signed with.
func keyID(key *ecdsa.PrivateKey) (string, error) {
	thumbprint, err := (&jose.JSONWebKey{Key: key.Public()}).Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(thumbprint), nil
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
