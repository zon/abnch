package main

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// inferRepo derives the org stub and repo stub for the repo under workDir from
// the Git remote `origin`.
func inferRepo(workDir string) (org, stub string, err error) {
	remote, err := originRemote(workDir)
	if err != nil {
		return "", "", err
	}
	return parseRemote(remote)
}

// originRemote returns the URL of the `origin` remote of the Git repo at
// workDir.
func originRemote(workDir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cannot read the origin remote: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// parseRemote derives the org stub and repo stub from a Git remote URL. It
// supports the common HTTPS and SSH forms, with or without a `.git` suffix or
// a trailing slash.
func parseRemote(remote string) (org, stub string, err error) {
	segments := repoPathSegments(remotePath(remote))
	if len(segments) < 2 {
		return "", "", fmt.Errorf("cannot derive a repo from remote %q", remote)
	}
	org = segments[len(segments)-2]
	stub = strings.TrimSuffix(segments[len(segments)-1], ".git")
	if org == "" || stub == "" {
		return "", "", fmt.Errorf("cannot derive a repo from remote %q", remote)
	}
	return org, stub, nil
}

// remotePath returns the repo path a remote URL points at. It handles the URL
// forms with a scheme and the scp-like SCP form `[user@]host:path`.
func remotePath(remote string) string {
	if strings.Contains(remote, "://") {
		parsed, err := url.Parse(remote)
		if err != nil {
			return remote
		}
		return parsed.Path
	}
	if i := strings.Index(remote, ":"); i >= 0 {
		return remote[i+1:]
	}
	return remote
}

// repoPathSegments splits a repo path into its non-empty segments, so a leading
// or trailing slash is ignored.
func repoPathSegments(path string) []string {
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}
