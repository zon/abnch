package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/devconfig"
)

const postgresYAML = "postgres:\n" +
	"  host: db\n" +
	"  port: 5432\n" +
	"  dbname: abnch\n" +
	"  user: admin\n" +
	"  password: pw\n" +
	"  sslmode: disable\n" +
	"  timeZone: UTC\n"

func validConfig(port int) string {
	return fmt.Sprintf("url: http://127.0.0.1:%d\n", port) + postgresYAML
}

func TestRunRejectsMissingConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for a missing config file")
	}
	if !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("error = %q, want it to name the missing file", err)
	}
}

func TestRunRejectsMalformedConfigFile(t *testing.T) {
	path := writeConfig(t, "url: [\n")

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for a malformed config file")
	}
	if !strings.Contains(err.Error(), "not valid") {
		t.Errorf("error = %q, want it to report the config as invalid", err)
	}
}

func TestRunRejectsMissingServiceURL(t *testing.T) {
	path := writeConfig(t, postgresYAML)

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for a missing service url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q, want it to name the service url", err)
	}
}

func TestRunRejectsInvalidServiceURL(t *testing.T) {
	path := writeConfig(t, "url: not-a-url\n"+postgresYAML)

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for an invalid service url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q, want it to name the service url", err)
	}
}

func TestRunRejectsIncompletePostgres(t *testing.T) {
	path := writeConfig(t, "url: http://127.0.0.1:8080\npostgres:\n"+
		"  host: \"\"\n"+
		"  port: 5432\n"+
		"  dbname: abnch\n"+
		"  user: admin\n"+
		"  password: pw\n"+
		"  sslmode: disable\n"+
		"  timeZone: UTC\n")

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for an incomplete Postgres block")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Errorf("error = %q, want it to name the missing Postgres value", err)
	}
}

// TestRunReadsPostgresBlock checks the config reaches the opener unchanged and
// that its failure surfaces as Run's error.
func TestRunReadsPostgresBlock(t *testing.T) {
	path := writeConfig(t, validConfig(8080))

	var got devconfig.Postgres
	opener := func(_ context.Context, pg devconfig.Postgres) (Database, error) {
		got = pg
		return nil, errors.New("connection refused")
	}

	err := Run(context.Background(), Options{ConfigPath: path, Opener: opener})
	if err == nil {
		t.Fatal("Run = nil, want an error when the database is unreachable")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error = %q, want it to carry the opener failure", err)
	}
	if got.Host != "db" || got.Port != 5432 || got.DBName != "abnch" {
		t.Errorf("opener got %#v, want the Postgres block from the config", got)
	}
}

func TestRunServesUntilContextCanceled(t *testing.T) {
	port := freePort(t)
	path := writeConfig(t, validConfig(port))

	db := &fakeDB{}
	opener := func(context.Context, devconfig.Postgres) (Database, error) { return db, nil }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{ConfigPath: path, Opener: opener, Out: io.Discard})
	}()

	waitForService(t, fmt.Sprintf("http://127.0.0.1:%d/", port))

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v, want nil after cancel", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was canceled")
	}

	if !db.closed {
		t.Error("Run did not close the database")
	}
}

// TestRunRejectsUnverifiedRequests starts the service and checks it rejects a
// request with no credentials or credentials it cannot verify.
func TestRunRejectsUnverifiedRequests(t *testing.T) {
	base := startService(t, Options{})

	for _, tc := range []struct {
		name string
		auth string
	}{
		{name: "missing credentials"},
		{name: "unverifiable credentials", auth: "Bearer not-a-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, base, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
			}
		})
	}
}

// TestRunAuthenticatesRequests checks an authenticated request reaches the
// handler the service hosts.
func TestRunAuthenticatesRequests(t *testing.T) {
	allow := func(*http.Request) (auth.Caller, error) { return auth.Caller{Subject: "caller"}, nil }
	base := startService(t, Options{Authenticator: allow})

	resp, err := http.Get(base)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d for a route the service does not serve", resp.StatusCode, http.StatusNotFound)
	}
}

// startService runs the service on a free port with a config that trusts no
// issuers and returns its base URL.
func startService(t *testing.T, opts Options) string {
	t.Helper()
	port := freePort(t)
	opts.ConfigPath = writeConfig(t, validConfig(port))
	return startServiceWith(t, port, opts)
}

// startServiceWith runs the service on the port its config names and returns its
// base URL. It stops the service and checks it returns cleanly when the test
// ends.
func startServiceWith(t *testing.T, port int, opts Options) string {
	t.Helper()
	if opts.Opener == nil {
		opts.Opener = func(context.Context, devconfig.Postgres) (Database, error) { return &fakeDB{}, nil }
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opts) }()

	base := fmt.Sprintf("http://127.0.0.1:%d/", port)
	waitForService(t, base)

	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run = %v, want nil after cancel", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Run did not return after its context was canceled")
		}
	})
	return base
}

// unusedOpener fails the test when called, so a config error never reaches the
// database.
func unusedOpener(t *testing.T) Opener {
	t.Helper()
	return func(context.Context, devconfig.Postgres) (Database, error) {
		t.Fatal("opener called for a config that should have been rejected")
		return nil, nil
	}
}

type fakeDB struct{ closed bool }

func (f *fakeDB) Close() error {
	f.closed = true
	return nil
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForService(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("service at %s did not start", url)
}
