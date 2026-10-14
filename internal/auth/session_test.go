package auth

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// signSession mints a token with a sessions key directly, so a test can shape
// claims the signer does not.
func signSession(t *testing.T, sessions *Sessions, claims map[string]any) string {
	t.Helper()

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: sessions.key}, nil)
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

func TestSessionsGrantAndRead(t *testing.T) {
	sessions, err := NewSessions("secret")
	if err != nil {
		t.Fatal(err)
	}

	token, err := sessions.Grant(Session{Subject: "alice", Roles: []string{"Admin", "Reader"}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := sessions.Read(token)
	if err != nil {
		t.Fatalf("Read = %v, want no error", err)
	}
	if got.Subject != "alice" {
		t.Errorf("subject = %q, want alice", got.Subject)
	}
	if len(got.Roles) != 2 || got.Roles[0] != "Admin" || got.Roles[1] != "Reader" {
		t.Errorf("roles = %v, want [Admin Reader]", got.Roles)
	}
}

func TestSessionsRejectForeignKey(t *testing.T) {
	granting, err := NewSessions("secret")
	if err != nil {
		t.Fatal(err)
	}
	token, err := granting.Grant(Session{Subject: "alice"})
	if err != nil {
		t.Fatal(err)
	}

	other, err := NewSessions("other-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Read(token); err == nil {
		t.Error("Read = nil, want an error for a token signed with another key")
	}
}

func TestSessionsRejectExpired(t *testing.T) {
	sessions, err := NewSessions("secret")
	if err != nil {
		t.Fatal(err)
	}
	token := signSession(t, sessions, map[string]any{
		"sub": "alice",
		"exp": time.Now().Add(-time.Minute).Unix(),
	})

	if _, err := sessions.Read(token); err == nil {
		t.Error("Read = nil, want an error for an expired session")
	}
}

func TestSessionsRejectMalformed(t *testing.T) {
	sessions, err := NewSessions("secret")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := sessions.Read("not-a-session"); err == nil {
		t.Error("Read = nil, want an error for a token that is not a session")
	}
}

func TestSessionsNeedSecret(t *testing.T) {
	if _, err := NewSessions(""); err == nil {
		t.Error("NewSessions = nil, want an error for a missing secret")
	}
}

// TestSessionAuthenticatesRequest checks a granted session identifies the
// caller a later request presents it as.
func TestSessionAuthenticatesRequest(t *testing.T) {
	sessions, err := NewSessions("secret")
	if err != nil {
		t.Fatal(err)
	}
	token, err := sessions.Grant(Session{Subject: "alice", Roles: []string{"Admin"}})
	if err != nil {
		t.Fatal(err)
	}

	caller, err := sessions.Authenticate(bearer(token))
	if err != nil {
		t.Fatalf("Authenticate = %v, want no error", err)
	}
	if caller.Subject != "alice" {
		t.Errorf("subject = %q, want alice", caller.Subject)
	}
	if len(caller.Roles) != 1 || caller.Roles[0] != "Admin" {
		t.Errorf("roles = %v, want [Admin]", caller.Roles)
	}
}

func TestSessionAuthenticateRejectsInvalid(t *testing.T) {
	sessions, err := NewSessions("secret")
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewSessions("other-secret")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := other.Grant(Session{Subject: "mallory"})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		req  func() *http.Request
	}{
		{name: "missing credentials", req: func() *http.Request { return bearer("") }},
		{name: "foreign session", req: func() *http.Request { return bearer(foreign) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := sessions.Authenticate(tc.req()); err == nil {
				t.Error("Authenticate = nil, want an error")
			}
		})
	}
}
