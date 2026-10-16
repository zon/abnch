package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/version"
)

// signInService serves the service sign-in the CLI starts: it redirects the
// browser back to the CLI's callback with the session it grants.
func signInService(t *testing.T, session string) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		redirect, err := url.Parse(r.URL.Query().Get("redirect_uri"))
		if err != nil {
			http.Error(w, "bad redirect_uri", http.StatusBadRequest)
			return
		}
		query := redirect.Query()
		if session != "" {
			query.Set("session", session)
		}
		redirect.RawQuery = query.Encode()
		http.Redirect(w, r, redirect.String(), http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// failingService serves a sign-in that reports the given failure instead of a
// session.
func failingService(t *testing.T, reason string) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		redirect, err := url.Parse(r.URL.Query().Get("redirect_uri"))
		if err != nil {
			http.Error(w, "bad redirect_uri", http.StatusBadRequest)
			return
		}
		query := redirect.Query()
		query.Set("error", reason)
		redirect.RawQuery = query.Encode()
		http.Redirect(w, r, redirect.String(), http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// noOIDCService serves a service that cannot sign humans in: starting a sign-in
// reports that OIDC login is unavailable, the way a service whose config names
// no OIDC provider does.
func noOIDCService(t *testing.T) string {
	t.Helper()
	return failingService(t, "oidc login is not available")
}

// followSignIn is a browser that opens the service sign-in and follows it to
// the CLI callback, as a real browser would.
func followSignIn(t *testing.T) func(string) error {
	t.Helper()
	return func(rawURL string) error {
		resp, err := http.Get(rawURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("sign-in page returned %d", resp.StatusCode)
		}
		return nil
	}
}

// unusedBrowser fails the test when opened, so a command that should not reach
// sign-in never opens the browser.
func unusedBrowser(t *testing.T) func(string) error {
	t.Helper()
	return func(string) error {
		t.Fatal("browser opened for a command that should have failed first")
		return nil
	}
}

// noSession reports no stored session for the service URL.
func noSession(t *testing.T, home string) {
	t.Helper()
	if _, err := os.Stat(devconfig.CredentialsPath(home)); !os.IsNotExist(err) {
		t.Errorf("credentials file exists after a failed sign-in: stat = %v", err)
	}
}

// writeCLIConfig writes the CLI config under workDir with the given body.
func writeCLIConfig(t *testing.T, workDir, body string) {
	t.Helper()
	path := configPath(workDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoginStoresSession(t *testing.T) {
	home := t.TempDir()
	service := signInService(t, "session-token")
	var out bytes.Buffer

	if err := runIn([]string{"login", "--url", service}, home, t.TempDir(), followSignIn(t), context.Background(), &out); err != nil {
		t.Fatalf("login = %v, want no error", err)
	}

	creds, err := devconfig.LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if got := creds[service].Session; got != "session-token" {
		t.Errorf("session = %q, want session-token", got)
	}
}

func TestLoginKeepsOtherSessions(t *testing.T) {
	home := t.TempDir()
	other := "https://bench.example.com"
	if err := devconfig.StoreSession(home, other, "other-token"); err != nil {
		t.Fatal(err)
	}

	service := signInService(t, "session-token")
	if err := runIn([]string{"login", "--url", service}, home, t.TempDir(), followSignIn(t), context.Background(), io.Discard); err != nil {
		t.Fatal(err)
	}

	creds, err := devconfig.LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if creds[other].Session != "other-token" {
		t.Errorf("other session = %q, want it kept", creds[other].Session)
	}
	if creds[service].Session != "session-token" {
		t.Errorf("session = %q, want session-token", creds[service].Session)
	}
}

func TestLoginReplacesSession(t *testing.T) {
	home := t.TempDir()
	service := signInService(t, "new-token")
	if err := devconfig.StoreSession(home, service, "old-token"); err != nil {
		t.Fatal(err)
	}

	if err := runIn([]string{"login", "--url", service}, home, t.TempDir(), followSignIn(t), context.Background(), io.Discard); err != nil {
		t.Fatal(err)
	}

	creds, err := devconfig.LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if creds[service].Session != "new-token" {
		t.Errorf("session = %q, want the new session", creds[service].Session)
	}
}

func TestLoginStoresSessionOutsideRepo(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	configDir := filepath.Join(repo, ".abnch")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("url: http://localhost:8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := signInService(t, "session-token")
	if err := runIn([]string{"login", "--url", service}, home, t.TempDir(), followSignIn(t), context.Background(), io.Discard); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(devconfig.CredentialsPath(home)); err != nil {
		t.Errorf("session is not under the user home: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "url: http://localhost:8080\n" {
		t.Errorf("repo config changed:\n%s", data)
	}
}

func TestLoginFailsWhenSignInFails(t *testing.T) {
	home := t.TempDir()
	service := failingService(t, "access_denied")

	err := runIn([]string{"login", "--url", service}, home, t.TempDir(), followSignIn(t), context.Background(), io.Discard)
	if err == nil {
		t.Fatal("login = nil, want an error for a failed sign-in")
	}
	if !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("error = %q, want it to report the sign-in failure", err)
	}
	noSession(t, home)
}

// TestLoginFailsWhenServiceHasNoOIDC checks `abnch login` reports that OIDC
// login is unavailable when the service cannot sign humans in, and stores no
// session.
func TestLoginFailsWhenServiceHasNoOIDC(t *testing.T) {
	home := t.TempDir()
	service := noOIDCService(t)

	err := runIn([]string{"login", "--url", service}, home, t.TempDir(), followSignIn(t), context.Background(), io.Discard)
	if err == nil {
		t.Fatal("login = nil, want an error for a service without OIDC")
	}
	if !strings.Contains(err.Error(), "oidc login") {
		t.Errorf("error = %q, want it to report that OIDC login is unavailable", err)
	}
	noSession(t, home)
}

func TestLoginFailsWhenSignInReturnsNoSession(t *testing.T) {
	home := t.TempDir()
	service := signInService(t, "")

	err := runIn([]string{"login", "--url", service}, home, t.TempDir(), followSignIn(t), context.Background(), io.Discard)
	if err == nil {
		t.Fatal("login = nil, want an error when the sign-in returns no session")
	}
	noSession(t, home)
}

func TestLoginFailsWhenBrowserCannotOpen(t *testing.T) {
	home := t.TempDir()
	service := signInService(t, "session-token")
	open := func(string) error { return errors.New("no browser") }

	err := runIn([]string{"login", "--url", service}, home, t.TempDir(), open, context.Background(), io.Discard)
	if err == nil {
		t.Fatal("login = nil, want an error when the browser cannot open")
	}
	noSession(t, home)
}

func TestLoginFailsWhenSignInIsAbandoned(t *testing.T) {
	home := t.TempDir()
	service := signInService(t, "session-token")

	ctx, cancel := context.WithCancel(context.Background())
	open := func(string) error {
		cancel()
		return nil
	}

	err := runIn([]string{"login", "--url", service}, home, t.TempDir(), open, ctx, io.Discard)
	if err == nil {
		t.Fatal("login = nil, want an error for an abandoned sign-in")
	}
	noSession(t, home)
}

func TestLoginFailsWithoutURL(t *testing.T) {
	home := t.TempDir()

	err := runIn([]string{"login"}, home, t.TempDir(), unusedBrowser(t), context.Background(), io.Discard)
	if err == nil {
		t.Fatal("login = nil, want an error without a service url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q, want it to name the missing url", err)
	}
	noSession(t, home)
}

func TestLogoutRemovesSession(t *testing.T) {
	home := t.TempDir()
	service := "https://bench.example.com"
	if err := devconfig.StoreSession(home, service, "token"); err != nil {
		t.Fatal(err)
	}

	if err := runIn([]string{"logout", "--url", service}, home, t.TempDir(), unusedBrowser(t), context.Background(), io.Discard); err != nil {
		t.Fatalf("logout = %v, want no error", err)
	}

	creds, err := devconfig.LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if got := creds[service].Session; got != "" {
		t.Errorf("session = %q, want it removed", got)
	}
}

func TestLogoutKeepsOtherSessions(t *testing.T) {
	home := t.TempDir()
	other := "https://other.example.com"
	service := "https://bench.example.com"
	if err := devconfig.StoreSession(home, other, "other-token"); err != nil {
		t.Fatal(err)
	}
	if err := devconfig.StoreSession(home, service, "token"); err != nil {
		t.Fatal(err)
	}

	if err := runIn([]string{"logout", "--url", service}, home, t.TempDir(), unusedBrowser(t), context.Background(), io.Discard); err != nil {
		t.Fatal(err)
	}

	creds, err := devconfig.LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if creds[other].Session != "other-token" {
		t.Errorf("other session = %q, want it kept", creds[other].Session)
	}
	if creds[service].Session != "" {
		t.Errorf("session = %q, want it removed", creds[service].Session)
	}
}

func TestLogoutWithoutSessionSucceeds(t *testing.T) {
	home := t.TempDir()

	if err := runIn([]string{"logout", "--url", "https://bench.example.com"}, home, t.TempDir(), unusedBrowser(t), context.Background(), io.Discard); err != nil {
		t.Fatalf("logout = %v, want no error", err)
	}

	noSession(t, home)
}

func TestLogoutFailsWithoutURL(t *testing.T) {
	home := t.TempDir()

	err := runIn([]string{"logout"}, home, t.TempDir(), unusedBrowser(t), context.Background(), io.Discard)
	if err == nil {
		t.Fatal("logout = nil, want an error without a service url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q, want it to name the missing url", err)
	}
	noSession(t, home)
}

func TestLoginOpensBrowserAtTheServiceSignIn(t *testing.T) {
	home := t.TempDir()
	service := signInService(t, "session-token")

	var opened string
	open := func(rawURL string) error {
		opened = rawURL
		return followSignIn(t)(rawURL)
	}
	if err := runIn([]string{"login", "--url", service}, home, t.TempDir(), open, context.Background(), io.Discard); err != nil {
		t.Fatal(err)
	}

	parsed, err := url.Parse(opened)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != service+"/login" {
		t.Errorf("browser opened %q, want the service sign-in", opened)
	}
	redirect, err := url.Parse(parsed.Query().Get("redirect_uri"))
	if err != nil {
		t.Fatal(err)
	}
	if redirect.Hostname() != "127.0.0.1" {
		t.Errorf("redirect_uri = %q, want a loopback address", redirect)
	}
	if redirect.Path != callbackPath {
		t.Errorf("redirect_uri path = %q, want %q", redirect.Path, callbackPath)
	}
}

func TestLoginUsesConfigURL(t *testing.T) {
	home := t.TempDir()
	workDir := t.TempDir()
	service := signInService(t, "session-token")
	writeCLIConfig(t, workDir, "url: "+service+"\n")

	if err := runIn([]string{"login"}, home, workDir, followSignIn(t), context.Background(), io.Discard); err != nil {
		t.Fatalf("login = %v, want no error", err)
	}

	creds, err := devconfig.LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if got := creds[service].Session; got != "session-token" {
		t.Errorf("session = %q, want session-token", got)
	}
}

func TestLoginURLOverridesConfig(t *testing.T) {
	home := t.TempDir()
	workDir := t.TempDir()
	service := signInService(t, "session-token")
	writeCLIConfig(t, workDir, "url: https://config.example.com\n")

	if err := runIn([]string{"login", "--url", service}, home, workDir, followSignIn(t), context.Background(), io.Discard); err != nil {
		t.Fatalf("login = %v, want no error", err)
	}

	creds, err := devconfig.LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if got := creds[service].Session; got != "session-token" {
		t.Errorf("session = %q, want the flag url's session", got)
	}
	if _, ok := creds["https://config.example.com"]; ok {
		t.Error("a session was stored for the config url, want the flag url")
	}
}

func TestLoginFailsWhenConfigHasNoURL(t *testing.T) {
	home := t.TempDir()
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "models: {}\n")

	err := runIn([]string{"login"}, home, workDir, unusedBrowser(t), context.Background(), io.Discard)
	if err == nil {
		t.Fatal("login = nil, want an error without a service url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q, want it to name the missing url", err)
	}
	noSession(t, home)
}

func TestLogoutUsesConfigURL(t *testing.T) {
	home := t.TempDir()
	workDir := t.TempDir()
	service := "https://bench.example.com"
	writeCLIConfig(t, workDir, "url: "+service+"\n")
	if err := devconfig.StoreSession(home, service, "token"); err != nil {
		t.Fatal(err)
	}

	if err := runIn([]string{"logout"}, home, workDir, unusedBrowser(t), context.Background(), io.Discard); err != nil {
		t.Fatalf("logout = %v, want no error", err)
	}

	creds, err := devconfig.LoadCredentials(home)
	if err != nil {
		t.Fatal(err)
	}
	if got := creds[service].Session; got != "" {
		t.Errorf("session = %q, want it removed", got)
	}
}

func TestLogoutFailsWhenConfigHasNoURL(t *testing.T) {
	home := t.TempDir()
	workDir := t.TempDir()
	writeCLIConfig(t, workDir, "models: {}\n")

	err := runIn([]string{"logout"}, home, workDir, unusedBrowser(t), context.Background(), io.Discard)
	if err == nil {
		t.Fatal("logout = nil, want an error without a service url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q, want it to name the missing url", err)
	}
}

// TestVersionFlagPrintsTheVersion checks `abnch --version` prints the repo's
// semantic version and reports success.
func TestVersionFlagPrintsTheVersion(t *testing.T) {
	var out bytes.Buffer

	if err := runIn([]string{"--version"}, t.TempDir(), t.TempDir(), unusedBrowser(t), context.Background(), &out); err != nil {
		t.Fatalf("--version = %v, want no error", err)
	}
	if got := strings.TrimSpace(out.String()); got != version.Version {
		t.Errorf("--version printed %q, want %q", got, version.Version)
	}
}
