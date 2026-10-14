package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareAuthenticatesRequest(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})
	allow := func(*http.Request) (Caller, error) { return Caller{Subject: "caller"}, nil }

	rec := httptest.NewRecorder()
	Middleware(allow, next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Error("an authenticated request did not reach the next handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestMiddlewareRejectsUnverifiedRequest(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the next handler ran for an unverified request")
	})

	for _, tc := range []struct {
		name string
		auth string
	}{
		{name: "missing credentials"},
		{name: "unverifiable credentials", auth: "Bearer not-a-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}

			rec := httptest.NewRecorder()
			Middleware(Deny, next).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		want   string
		wantOK bool
	}{
		{name: "bearer", header: "Bearer abc", want: "abc", wantOK: true},
		{name: "case-insensitive scheme", header: "bearer abc", want: "abc", wantOK: true},
		{name: "surrounding space", header: "Bearer  abc  ", want: "abc", wantOK: true},
		{name: "missing", header: ""},
		{name: "scheme only", header: "Bearer"},
		{name: "empty token", header: "Bearer "},
		{name: "other scheme", header: "Basic abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}

			got, err := BearerToken(req)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("BearerToken = %v, want no error", err)
				}
				if got != tc.want {
					t.Errorf("BearerToken = %q, want %q", got, tc.want)
				}
				return
			}
			if err == nil {
				t.Errorf("BearerToken = %q, want an error", got)
			}
		})
	}
}
