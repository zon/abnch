package main

import (
	"os/exec"
	"testing"
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

	org, stub, err := inferRepo(workDir)
	if err != nil {
		t.Fatalf("inferRepo = %v, want no error", err)
	}
	if org != "acme" || stub != "widgets" {
		t.Errorf("inferRepo = %q/%q, want acme/widgets", org, stub)
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

	if org, stub, err := inferRepo(workDir); err == nil {
		t.Errorf("inferRepo = %q/%q, want an error without an origin remote", org, stub)
	}
}

func TestInferRepoFailsOutsideGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	if org, stub, err := inferRepo(t.TempDir()); err == nil {
		t.Errorf("inferRepo = %q/%q, want an error outside a Git repo", org, stub)
	}
}
