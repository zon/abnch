package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/zon/abnch/internal/devconfig"
)

// TestSetLocalAuthConfiguresCLI proves that the files `abnchctl set local-auth`
// writes configure the `abnch` CLI for local auth. The CLI resolves the service
// URL from `.abnch/config.yaml` and authenticates with the local credential, so
// a command reaches the local service as the dev issuer's local-cli subject
// without a sign-in.
func TestSetLocalAuthConfiguresCLI(t *testing.T) {
	repo := t.TempDir()
	addRemote(t, repo, "git@github.com:acme/widgets.git")
	home := t.TempDir()

	// Hold a free port and serve the local service on it, so the URL local auth
	// builds for that port reaches this test's service.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	port := ln.Addr().(*net.TCPAddr).Port
	serviceURL := fmt.Sprintf("http://localhost:%d", port)

	fake := &fakeService{
		repos: map[string]string{"acme/widgets": "git@github.com:acme/widgets.git"},
		tasks: map[string]task{},
	}
	fake.seedTask("acme", task{Stub: "demo", Repo: "widgets", Name: "Demo", Description: "A demo task"})
	auth := &authRecorder{}
	go func() { _ = http.Serve(ln, auth.wrap(fake)) }()

	setLocalAuth(t, repo, home, port)

	var out bytes.Buffer
	if err := runIn([]string{"get", "tasks"}, home, repo, nil, context.Background(), &out); err != nil {
		t.Fatalf("abnch get tasks = %v, want the local auth to authenticate it", err)
	}

	if !contains(fake.methods(), "GET /repos/acme/widgets/tasks") {
		t.Errorf("service requests = %v, want the task list read", fake.methods())
	}
	if !strings.Contains(out.String(), "demo") {
		t.Errorf("output = %q, want the seeded task", out.String())
	}

	token := auth.token(t)
	if token == "" {
		t.Fatal("the CLI sent no Authorization header")
	}
	claims := verifyLocalToken(t, token, devconfig.KeyPath(devconfig.ConfigPath(repo)))
	if claims["iss"] != serviceURL+"/dev" {
		t.Errorf("iss = %v, want the dev issuer for the local service %q", claims["iss"], serviceURL)
	}
	if claims["aud"] != "abnch" {
		t.Errorf("aud = %v, want the dev issuer audience abnch", claims["aud"])
	}
	if claims["sub"] != devconfig.LocalCLISubject {
		t.Errorf("sub = %v, want the local CLI subject %q", claims["sub"], devconfig.LocalCLISubject)
	}
}

// authRecorder records the Authorization header of the first request a service
// receives.
type authRecorder struct {
	mu     sync.Mutex
	header string
}

// wrap returns a handler that records the request's Authorization header before
// passing it on.
func (a *authRecorder) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		if a.header == "" {
			a.header = r.Header.Get("Authorization")
		}
		a.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

// token returns the bearer token of the recorded header.
func (a *authRecorder) token(t *testing.T) string {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	token, _ := strings.CutPrefix(a.header, "Bearer ")
	return token
}

// setLocalAuth builds abnchctl and runs `set local-auth`, the way `just config`
// does, against a repo and a user home for the port.
func setLocalAuth(t *testing.T, repo, home string, port int) {
	t.Helper()

	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not installed")
	}
	bin := filepath.Join(t.TempDir(), "abnchctl")
	build := exec.Command(goTool, "build", "-o", bin, "./cmd/abnchctl")
	build.Dir = moduleRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build abnchctl: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "set", "local-auth", "--port", strconv.Itoa(port))
	cmd.Dir = repo
	cmd.Env = envWithHome(home)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("abnchctl set local-auth: %v\n%s", err, out)
	}
}

// envWithHome returns the process environment with HOME set to home, so the
// command writes its credential store there.
func envWithHome(home string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "HOME=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "HOME="+home)
}

// moduleRoot returns the module root above the test file.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
