package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/devconfig"
)

// remoteRepo is the org stub and repo stub a remote URL derives.
type remoteRepo struct {
	org  string
	stub string
}

// addRemote initialises a Git repo in dir and adds the origin remote. It skips
// the test when git is not installed.
func addRemote(t *testing.T, dir, remote string) {
	t.Helper()

	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", remote}} {
		cmd := exec.Command(git, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestParseRemoteDerivesRepoStubs(t *testing.T) {
	tests := []struct {
		name   string
		remote string
		want   remoteRepo
	}{
		{"https", "https://github.com/acme/widgets.git", remoteRepo{"acme", "widgets"}},
		{"https without suffix", "https://github.com/acme/widgets", remoteRepo{"acme", "widgets"}},
		{"https with trailing slash", "https://github.com/acme/widgets/", remoteRepo{"acme", "widgets"}},
		{"https with suffix and trailing slash", "https://github.com/acme/widgets.git/", remoteRepo{"acme", "widgets"}},
		{"ssh scp", "git@github.com:acme/widgets.git", remoteRepo{"acme", "widgets"}},
		{"ssh scp without suffix", "git@github.com:acme/widgets", remoteRepo{"acme", "widgets"}},
		{"ssh scp with trailing slash", "git@github.com:acme/widgets/", remoteRepo{"acme", "widgets"}},
		{"ssh scp with suffix and trailing slash", "git@github.com:acme/widgets.git/", remoteRepo{"acme", "widgets"}},
		{"ssh scheme", "ssh://git@github.com/acme/widgets.git", remoteRepo{"acme", "widgets"}},
		{"ssh scheme without suffix", "ssh://git@github.com/acme/widgets", remoteRepo{"acme", "widgets"}},
		{"ssh scheme with trailing slash", "ssh://git@github.com/acme/widgets/", remoteRepo{"acme", "widgets"}},
		{"https on another host", "https://gitlab.com/acme/widgets.git", remoteRepo{"acme", "widgets"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, stub, err := parseRemote(tt.remote)
			if err != nil {
				t.Fatalf("parseRemote(%q) = %v, want no error", tt.remote, err)
			}
			if org != tt.want.org || stub != tt.want.stub {
				t.Errorf("parseRemote(%q) = %q/%q, want %q/%q", tt.remote, org, stub, tt.want.org, tt.want.stub)
			}
		})
	}
}

func TestParseRemoteRejectsRemoteWithoutRepo(t *testing.T) {
	for _, remote := range []string{
		"https://github.com/acme",
		"https://github.com/",
		"git@github.com:acme",
		"",
	} {
		if org, stub, err := parseRemote(remote); err == nil {
			t.Errorf("parseRemote(%q) = %q/%q, want an error", remote, org, stub)
		}
	}
}

func TestInferRepoReadsOriginRemote(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")

	org, stub, remote, err := inferRepo(workDir)
	if err != nil {
		t.Fatalf("inferRepo = %v, want no error", err)
	}
	if org != "acme" || stub != "widgets" {
		t.Errorf("inferRepo = %q/%q, want acme/widgets", org, stub)
	}
	if remote != "git@github.com:acme/widgets.git" {
		t.Errorf("inferRepo remote = %q, want the origin remote", remote)
	}
}

func TestInferRepoFailsWithoutOriginRemote(t *testing.T) {
	workDir := t.TempDir()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	cmd := exec.Command(git, "init", "-q")
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	if _, _, _, err := inferRepo(workDir); err == nil {
		t.Error("inferRepo, want an error without an origin remote")
	}
}

func TestInferRepoFailsOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	if _, _, _, err := inferRepo(t.TempDir()); err == nil {
		t.Error("inferRepo, want an error outside a Git repo")
	}
}

// countingService records how many requests reach it.
func countingService(t *testing.T) (*httptest.Server, *int) {
	t.Helper()

	count := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*count++
	}))
	t.Cleanup(srv.Close)
	return srv, count
}

// withSession stores a session for the service URL and returns the deps a
// command works with.
func withSession(t *testing.T, workDir, serviceURL string) deps {
	t.Helper()

	home := t.TempDir()
	if err := devconfig.StoreSession(home, serviceURL, "session-token"); err != nil {
		t.Fatal(err)
	}
	return deps{homeDir: home, workDir: workDir, ctx: context.Background()}
}

func TestResolveRepoDerivesRepoAndClient(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "git@github.com:acme/widgets.git")
	srv, requests := countingService(t)

	target, err := resolveRepo(withSession(t, workDir, srv.URL), srv.URL)
	if err != nil {
		t.Fatalf("resolveRepo = %v, want no error", err)
	}
	if target.org != "acme" || target.stub != "widgets" {
		t.Errorf("resolveRepo = %q/%q, want acme/widgets", target.org, target.stub)
	}
	if target.url != "git@github.com:acme/widgets.git" {
		t.Errorf("resolveRepo url = %q, want the origin remote", target.url)
	}
	if target.client == nil {
		t.Error("resolveRepo returned no client")
	}
	if *requests != 0 {
		t.Errorf("requests = %d, want none from resolving a repo", *requests)
	}
}

func TestResolveRepoSendsNoRequestOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	srv, requests := countingService(t)

	target, err := resolveRepo(withSession(t, t.TempDir(), srv.URL), srv.URL)
	if err == nil {
		t.Fatal("resolveRepo = nil, want an error outside a Git repo")
	}
	if target.client != nil {
		t.Error("resolveRepo returned a client without a repo")
	}
	if *requests != 0 {
		t.Errorf("requests = %d, want none without a repo", *requests)
	}
	if strings.Contains(err.Error(), "abnch login") {
		t.Errorf("error = %q, want a repo error before the credential is resolved", err)
	}
}

func TestResolveRepoSendsNoRequestForUnparseableRemote(t *testing.T) {
	workDir := t.TempDir()
	addRemote(t, workDir, "https://github.com/acme")
	srv, requests := countingService(t)

	target, err := resolveRepo(withSession(t, workDir, srv.URL), srv.URL)
	if err == nil {
		t.Fatal("resolveRepo = nil, want an error for a remote without a repo")
	}
	if target.client != nil {
		t.Error("resolveRepo returned a client without a repo")
	}
	if *requests != 0 {
		t.Errorf("requests = %d, want none without a repo", *requests)
	}
	if strings.Contains(err.Error(), "abnch login") {
		t.Errorf("error = %q, want a repo error before the credential is resolved", err)
	}
}
