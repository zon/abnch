// Package auth authenticates callers to the service API.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/zon/abnch/internal/httpx"
)

// ErrUnauthenticated reports that a request carries no credentials or
// credentials the service cannot verify.
var ErrUnauthenticated = errors.New("unauthenticated")

// Caller is the caller an authenticated request identifies.
type Caller struct {
	Subject string
	Roles   []string
}

// Authenticator authenticates a request. It returns the caller the request
// identifies, or an error when it carries no credentials or credentials the
// service cannot verify.
type Authenticator func(*http.Request) (Caller, error)

// Deny rejects every request. It authenticates a service that cannot yet verify
// credentials.
func Deny(*http.Request) (Caller, error) {
	return Caller{}, ErrUnauthenticated
}

// Any authenticates a request when any of the authenticators does, returning
// the first caller identified. It rejects the request when none does.
func Any(authenticators ...Authenticator) Authenticator {
	return func(r *http.Request) (Caller, error) {
		for _, a := range authenticators {
			if a == nil {
				continue
			}
			if caller, err := a(r); err == nil {
				return caller, nil
			}
		}
		return Caller{}, ErrUnauthenticated
	}
}

// Middleware authenticates each request with a, then calls next with the
// caller it identified. A request whose credentials a cannot verify receives
// 401 and never reaches next.
func Middleware(a Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller, err := a(r)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		next.ServeHTTP(w, r.WithContext(callerContext(r.Context(), caller)))
	})
}

// callerKey is the request context key a request's authenticated caller is
// stored under.
type callerKey struct{}

// callerContext returns a context carrying an authenticated caller, so a later
// authorization step reads the same caller the request identified.
func callerContext(ctx context.Context, caller Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, caller)
}

// CallerFrom returns the caller an authenticated request carries.
func CallerFrom(ctx context.Context) (Caller, bool) {
	caller, ok := ctx.Value(callerKey{}).(Caller)
	return caller, ok
}

// BearerToken returns the bearer token a request carries.
func BearerToken(r *http.Request) (string, error) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", ErrUnauthenticated
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ErrUnauthenticated
	}
	return token, nil
}
