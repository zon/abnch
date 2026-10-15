package auth

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// sessionTTL is how long a session the service grants stays valid.
const sessionTTL = 12 * time.Hour

// ErrSession reports a session token that is missing, malformed, expired, or
// signed by something other than the service.
var ErrSession = errors.New("invalid session")

// Session identifies a signed-in user and carries the user's roles.
type Session struct {
	Subject string
	Roles   []string
}

// Sessions mints and reads the session tokens the service grants a signed-in
// user. A token is signed with the OIDC client secret, so only the service
// mints one and the service verifies a session it granted.
type Sessions struct {
	key []byte
}

// NewSessions builds a session signer from the secret that signs its tokens.
func NewSessions(secret string) (*Sessions, error) {
	if secret == "" {
		return nil, errors.New("a session needs a signing secret")
	}
	key := sha256.Sum256([]byte(secret))
	return &Sessions{key: key[:]}, nil
}

// Grant returns a session token that identifies the user and carries the
// user's roles.
func (s *Sessions) Grant(session Session) (string, error) {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: s.key}, nil)
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(map[string]any{
		"sub":   session.Subject,
		"roles": session.Roles,
		"exp":   time.Now().Add(sessionTTL).Unix(),
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

// Read returns the session a token carries. A token the service did not grant,
// or one that has expired, returns ErrSession.
func (s *Sessions) Read(token string) (Session, error) {
	jws, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.HS256})
	if err != nil {
		return Session{}, ErrSession
	}
	payload, err := jws.Verify(s.key)
	if err != nil {
		return Session{}, ErrSession
	}

	var claims struct {
		Subject string   `json:"sub"`
		Roles   []string `json:"roles"`
		Expiry  int64    `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Session{}, ErrSession
	}
	if claims.Subject == "" || time.Now().Unix() >= claims.Expiry {
		return Session{}, ErrSession
	}
	return Session{Subject: claims.Subject, Roles: claims.Roles}, nil
}

// Authenticate returns the caller a request's session token identifies. It
// rejects a request that carries no session, or one the service did not grant.
func (s *Sessions) Authenticate(r *http.Request) (Caller, error) {
	token, err := BearerToken(r)
	if err != nil {
		return Caller{}, err
	}
	session, err := s.Read(token)
	if err != nil {
		return Caller{}, ErrUnauthenticated
	}
	return Caller{Subject: session.Subject, Roles: session.Roles}, nil
}
