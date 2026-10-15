package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAuthorize checks a request reaches the handler only when the caller's
// roles permit the access its method needs.
func TestAuthorize(t *testing.T) {
	for _, tc := range []struct {
		name    string
		method  string
		roles   []string
		allowed bool
	}{
		{name: "reader reads", method: http.MethodGet, roles: []string{RoleReader}, allowed: true},
		{name: "service reads", method: http.MethodHead, roles: []string{RoleService}, allowed: true},
		{name: "admin reads", method: http.MethodGet, roles: []string{RoleAdmin}, allowed: true},
		{name: "admin writes", method: http.MethodPut, roles: []string{RoleAdmin}, allowed: true},
		{name: "reader writes", method: http.MethodPut, roles: []string{RoleReader}},
		{name: "service writes", method: http.MethodPost, roles: []string{RoleService}},
		{name: "agent reads", method: http.MethodGet, roles: []string{RoleAgent}},
		{name: "grader reads", method: http.MethodGet, roles: []string{RoleGrader}},
		{name: "grader writes", method: http.MethodPost, roles: []string{RoleGrader}},
		{name: "no roles", method: http.MethodGet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})
			authenticate := func(*http.Request) (Caller, error) {
				return Caller{Subject: "caller", Roles: tc.roles}, nil
			}

			rec := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, "/repos", nil)
			Middleware(authenticate, Authorize(next)).ServeHTTP(rec, request)

			want := http.StatusForbidden
			if tc.allowed {
				want = http.StatusNoContent
			}
			if rec.Code != want {
				t.Errorf("status = %d, want %d", rec.Code, want)
			}
			if called != tc.allowed {
				t.Errorf("handler called = %v, want %v", called, tc.allowed)
			}
		})
	}
}

// TestAuthorizeErrorBody checks a denied request reports the service error
// response.
func TestAuthorizeErrorBody(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the handler ran for a request without access")
	})
	authenticate := func(*http.Request) (Caller, error) {
		return Caller{Subject: "caller", Roles: []string{RoleReader}}, nil
	}

	rec := httptest.NewRecorder()
	Middleware(authenticate, Authorize(next)).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/repos", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content type = %q, want application/json", got)
	}

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Message == "" {
		t.Error("error response body has no message")
	}
}

// TestAuthorizeWithoutCallerForbids checks authorization runs after
// authentication: a request that carries no authenticated caller has no access.
func TestAuthorizeWithoutCallerForbids(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the handler ran without an authenticated caller")
	})

	rec := httptest.NewRecorder()
	Authorize(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/repos", nil))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
