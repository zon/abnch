package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/httpx"
)

// signInTTL bounds how long a started sign-in stays valid.
const signInTTL = 10 * time.Minute

// signInPath is the route that starts a human sign-in.
const signInPath = "/login"

// signInUnavailable is the failure a service with no OIDC provider reports when
// a client starts a sign-in.
const signInUnavailable = "oidc login is not available"

// Human signs in human users through an OIDC provider using the authorization
// code flow. On success it grants a session that identifies the user and
// carries the user's roles. A failed sign-in grants no session.
type Human struct {
	config   devconfig.OIDC
	sessions *Sessions
	states   *signInStates

	mu       sync.Mutex
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// NewHuman builds the human sign-in for an OIDC provider.
func NewHuman(cfg devconfig.OIDC) (*Human, error) {
	if cfg.Issuer == "" {
		return nil, errors.New("oidc issuer is missing")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("oidc clientId is missing")
	}
	if cfg.ClientSecret == "" {
		return nil, errors.New("oidc clientSecret is missing")
	}
	if cfg.RedirectURL == "" {
		return nil, errors.New("oidc redirectUrl is missing")
	}

	sessions, err := NewSessions(cfg.ClientSecret)
	if err != nil {
		return nil, err
	}
	return &Human{config: cfg, sessions: sessions, states: newSignInStates()}, nil
}

// Register mounts the sign-in routes: the route that starts the flow and the
// route the provider calls back.
func (h *Human) Register(mux *http.ServeMux) {
	mux.HandleFunc(signInPath, h.login)
	mux.HandleFunc(h.callbackPath(), h.callback)
}

// RegisterUnavailable mounts the sign-in route for a service that has no OIDC
// provider, so a client that starts a sign-in learns that OIDC login is
// unavailable instead of waiting for a sign-in that cannot complete.
func RegisterUnavailable(mux *http.ServeMux) {
	mux.HandleFunc(signInPath, unavailableSignIn)
}

// unavailableSignIn reports that the service cannot sign humans in. It returns
// the failure to the client that started the sign-in when it named a callback,
// and writes an error otherwise.
func unavailableSignIn(w http.ResponseWriter, r *http.Request) {
	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		httpx.WriteError(w, http.StatusNotFound, signInUnavailable)
		return
	}
	redirectWithError(w, r, redirectURI, signInUnavailable)
}

// Authenticate returns the caller a request's session token identifies, so a
// later request with a granted session is authenticated.
func (h *Human) Authenticate(r *http.Request) (Caller, error) {
	return h.sessions.Authenticate(r)
}

// callbackPath is the path the provider calls back, read from the configured
// redirect URL.
func (h *Human) callbackPath() string {
	u, err := url.Parse(h.config.RedirectURL)
	if err != nil || u.Path == "" {
		return "/auth/callback"
	}
	return u.Path
}

// login starts the authorization code flow. It remembers where to send the
// granted session, then redirects the browser to the provider.
func (h *Human) login(w http.ResponseWriter, r *http.Request) {
	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		httpx.WriteError(w, http.StatusBadRequest, "sign-in needs a redirect_uri")
		return
	}

	oauth, _, err := h.endpoints(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "sign-in is unavailable")
		return
	}

	state, err := randomToken()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "sign-in failed")
		return
	}
	nonce, err := randomToken()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "sign-in failed")
		return
	}

	h.states.put(state, signIn{redirectURI: redirectURI, nonce: nonce})
	http.Redirect(w, r, oauth.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
}

// callback completes the flow. It validates the state, exchanges the code for
// the provider's ID token, verifies that token, and grants a session. A
// rejected sign-in returns the failure and grants no session.
func (h *Human) callback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	st, ok := h.states.take(query.Get("state"))
	if !ok {
		httpx.WriteError(w, http.StatusBadRequest, "sign-in state is invalid or expired")
		return
	}

	if providerErr := query.Get("error"); providerErr != "" {
		redirectWithError(w, r, st.redirectURI, providerErr)
		return
	}

	oauth, verifier, err := h.endpoints(r.Context())
	if err != nil {
		redirectWithError(w, r, st.redirectURI, "sign-in is unavailable")
		return
	}

	token, err := oauth.Exchange(r.Context(), query.Get("code"))
	if err != nil {
		redirectWithError(w, r, st.redirectURI, "sign-in failed")
		return
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		redirectWithError(w, r, st.redirectURI, "sign-in failed")
		return
	}

	idToken, err := verifier.Verify(r.Context(), raw)
	if err != nil || idToken.Nonce != st.nonce {
		redirectWithError(w, r, st.redirectURI, "sign-in failed")
		return
	}

	session, err := h.sessions.Grant(Session{Subject: idToken.Subject, Roles: h.roles(idToken)})
	if err != nil {
		redirectWithError(w, r, st.redirectURI, "sign-in failed")
		return
	}
	redirectWith(w, r, st.redirectURI, url.Values{"session": {session}})
}

// roles maps the user's role claim values through the provider's role map. A
// value with no mapping grants no role.
func (h *Human) roles(token *oidc.IDToken) []string {
	return mapRoles(claimValue(token, h.config.RoleClaim), h.config.RoleMap)
}

// endpoints discovers the provider's endpoints and its ID token verifier. It
// discovers once and reuses the result, so a run stays independent of provider
// reachability until the first sign-in.
func (h *Human) endpoints(ctx context.Context) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.oauth != nil {
		return h.oauth, h.verifier, nil
	}

	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(ctx, h.config.Issuer)
	if err != nil {
		return nil, nil, err
	}

	h.oauth = &oauth2.Config{
		ClientID:     h.config.ClientID,
		ClientSecret: h.config.ClientSecret,
		RedirectURL:  h.config.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       h.scopes(),
	}
	h.verifier = provider.Verifier(&oidc.Config{ClientID: h.config.ClientID})
	return h.oauth, h.verifier, nil
}

// scopes returns the requested scopes, always including openid so the provider
// returns an ID token.
func (h *Human) scopes() []string {
	scopes := append([]string{}, h.config.Scopes...)
	for _, scope := range scopes {
		if scope == oidc.ScopeOpenID {
			return scopes
		}
	}
	return append([]string{oidc.ScopeOpenID}, scopes...)
}

// signIn is a started sign-in: where to send the granted session, and the nonce
// the provider's ID token must carry.
type signIn struct {
	redirectURI string
	nonce       string
	expiry      time.Time
}

// signInStates remembers started sign-ins by their state, so the callback finds
// the one it completes. An entry is used once.
type signInStates struct {
	mu     sync.Mutex
	states map[string]signIn
}

func newSignInStates() *signInStates {
	return &signInStates{states: map[string]signIn{}}
}

func (s *signInStates) put(state string, in signIn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	in.expiry = time.Now().Add(signInTTL)
	s.states[state] = in
}

func (s *signInStates) take(state string) (signIn, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	in, ok := s.states[state]
	if !ok {
		return signIn{}, false
	}
	delete(s.states, state)
	if time.Now().After(in.expiry) {
		return signIn{}, false
	}
	return in, true
}

// randomToken returns a URL-safe random value for a state or nonce.
func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// redirectWith sends the browser to target with the given query values, so the
// client that started the sign-in receives the result.
func redirectWith(w http.ResponseWriter, r *http.Request, target string, values url.Values) {
	u, err := url.Parse(target)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "sign-in failed")
		return
	}

	query := u.Query()
	for key, list := range values {
		for _, value := range list {
			query.Set(key, value)
		}
	}
	u.RawQuery = query.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// redirectWithError sends the browser back with a sign-in failure and no
// session.
func redirectWithError(w http.ResponseWriter, r *http.Request, target, reason string) {
	redirectWith(w, r, target, url.Values{"error": {reason}})
}
