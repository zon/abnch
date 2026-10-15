package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
)

// repoTarget is the repo a command acts on and the client it reaches the
// service with.
type repoTarget struct {
	org    string
	stub   string
	url    string
	client *serviceClient
}

// repoRequest is the JSON body a repo write carries.
type repoRequest struct {
	URL string `json:"url"`
}

// resolveRepo derives the repo under the working directory and builds the client
// a command reaches serviceURL with. It derives the repo first, so a command
// that needs a repo fails without resolving a credential or sending any request
// when it cannot derive one.
func resolveRepo(deps deps, serviceURL string) (repoTarget, error) {
	org, stub, remote, err := inferRepo(deps.workDir)
	if err != nil {
		return repoTarget{}, err
	}
	client, err := newServiceClient(deps, serviceURL)
	if err != nil {
		return repoTarget{}, err
	}
	return repoTarget{org: org, stub: stub, url: remote, client: client}, nil
}

// inferRepo derives the org stub, repo stub, and URL for the repo under workDir
// from the Git remote `origin`.
func inferRepo(workDir string) (org, stub, remote string, err error) {
	remote, err = originRemote(workDir)
	if err != nil {
		return "", "", "", err
	}
	org, stub, err = parseRemote(remote)
	if err != nil {
		return "", "", "", err
	}
	return org, stub, remote, nil
}

// repoMissingMessage is the message the service reports for a repo it does not
// know.
const repoMissingMessage = "repo not found"

// repoMissing reports that the service does not know the repo under the working
// directory. A command that creates a missing repo treats it as an absent repo.
// A command that only reads reports it to the user.
type repoMissing struct {
	org  string
	repo string
}

// Error names the repo and tells the user how to register it.
func (e repoMissing) Error() string {
	return fmt.Sprintf("the repo %s/%s is not registered with the service. Register it with a `set` command, such as `abnch set task <stub> --name <name> --description <description>`", e.org, e.repo)
}

// isRepoMissing reports whether err is a repo the service does not know.
func isRepoMissing(err error) bool {
	var missing repoMissing
	return errors.As(err, &missing)
}

// ensureRepo creates the repo on the service when it is missing. The repo is
// created from the URL the working directory's origin remote names.
func ensureRepo(ctx context.Context, target repoTarget) error {
	resp, err := target.client.do(ctx, http.MethodGet, repoPath(target.org, target.stub), nil)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		resp.Body.Close()
		return nil
	case http.StatusNotFound:
		resp.Body.Close()
	default:
		return serviceError(resp)
	}
	return createRepo(ctx, target)
}

// createRepo writes the repo to the service.
func createRepo(ctx context.Context, target repoTarget) error {
	payload, err := json.Marshal(repoRequest{URL: target.url})
	if err != nil {
		return err
	}
	resp, err := target.client.do(ctx, http.MethodPut, repoPath(target.org, target.stub), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return serviceError(resp)
	}
	resp.Body.Close()
	return nil
}

// repoPath returns the API path for a repo.
func repoPath(org, repo string) string {
	return "/repos/" + url.PathEscape(org) + "/" + url.PathEscape(repo)
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
