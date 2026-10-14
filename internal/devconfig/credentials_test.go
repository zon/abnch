package devconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadCredentialsMissingStoreIsEmpty(t *testing.T) {
	creds, err := LoadCredentials(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(creds) != 0 {
		t.Errorf("credentials = %#v, want an empty store", creds)
	}
}

func TestStoreSessionKeepsOtherURLs(t *testing.T) {
	home := t.TempDir()
	if err := StoreSession(home, "https://one.example.com", "one"); err != nil {
		t.Fatal(err)
	}
	if err := StoreSession(home, "https://two.example.com", "two"); err != nil {
		t.Fatal(err)
	}

	creds, err := LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Credential{
		"https://one.example.com": {Session: "one"},
		"https://two.example.com": {Session: "two"},
	}
	if !reflect.DeepEqual(creds, want) {
		t.Errorf("credentials = %#v, want %#v", creds, want)
	}
}

func TestStoreSessionReplacesSameURL(t *testing.T) {
	home := t.TempDir()
	if err := StoreSession(home, "https://bench.example.com", "old"); err != nil {
		t.Fatal(err)
	}
	if err := StoreSession(home, "https://bench.example.com", "new"); err != nil {
		t.Fatal(err)
	}

	creds, err := LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if got := creds["https://bench.example.com"].Session; got != "new" {
		t.Errorf("session = %q, want the new session", got)
	}
}

func TestStoreSessionKeepsLocalCredentialFields(t *testing.T) {
	home := t.TempDir()
	local := "http://localhost:8080"
	if err := saveCredentials(home, map[string]Credential{local: LocalCredential(local, "/repo/.abnchd/dev-issuer.key")}); err != nil {
		t.Fatal(err)
	}

	if err := StoreSession(home, local, "session-token"); err != nil {
		t.Fatal(err)
	}

	creds, err := LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	got := creds[local]
	if got.Session != "session-token" {
		t.Errorf("session = %q, want session-token", got.Session)
	}
	if got.Subject != LocalCLISubject || got.Issuer != local+"/dev" || got.KeyFile != "/repo/.abnchd/dev-issuer.key" {
		t.Errorf("credential = %#v, want the local credential fields kept", got)
	}
}

func TestSaveCredentialsUsesOwnerOnlyPermissions(t *testing.T) {
	home := t.TempDir()
	if err := StoreSession(home, "https://bench.example.com", "token"); err != nil {
		t.Fatal(err)
	}

	dir, err := os.Stat(filepath.Join(home, credentialsDir))
	if err != nil {
		t.Fatal(err)
	}
	if perm := dir.Mode().Perm(); perm != 0o700 {
		t.Errorf("credentials dir permissions = %o, want 700", perm)
	}
	file, err := os.Stat(CredentialsPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if perm := file.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials file permissions = %o, want 600", perm)
	}
}
