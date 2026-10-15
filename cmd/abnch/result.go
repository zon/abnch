package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// resultMissingMessage is the message the service reports for a result it does
// not know.
const resultMissingMessage = "result not found"

// resultPath returns the API path for one result in a repo's results.
func resultPath(org, repo, ref string) string {
	return resultsPath(org, repo) + "/" + url.PathEscape(ref)
}

// readResultRefs returns the refs of the results in the repo the client
// identifies. A missing repo is an error.
func readResultRefs(ctx context.Context, client *serviceClient, org, repo string) ([]string, error) {
	resp, err := client.do(ctx, http.MethodGet, resultsPath(org, repo), nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		var refs []string
		if err := json.NewDecoder(resp.Body).Decode(&refs); err != nil {
			return nil, fmt.Errorf("failed to read the results: %w", err)
		}
		return refs, nil
	case http.StatusNotFound:
		return nil, resultNotFoundError(resp, org, repo, "")
	default:
		return nil, serviceError(resp)
	}
}

// readResult returns the JSON of the result the ref names in the repo. A missing
// repo or result is an error.
func readResult(ctx context.Context, client *serviceClient, org, repo, ref string) ([]byte, error) {
	resp, err := client.do(ctx, http.MethodGet, resultPath(org, repo, ref), nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read the result: %w", err)
		}
		return body, nil
	case http.StatusNotFound:
		return nil, resultNotFoundError(resp, org, repo, ref)
	default:
		return nil, serviceError(resp)
	}
}

// resultNotFoundError maps a result request's not-found response to the resource
// the service does not know. A ref names the result when the response reports
// one.
func resultNotFoundError(resp *http.Response, org, repo, ref string) error {
	message, ok := serviceMessage(resp)
	switch {
	case !ok:
		return fmt.Errorf("service returned %s", resp.Status)
	case message == repoMissingMessage:
		return repoMissing{org: org, repo: repo}
	case message == resultMissingMessage && ref != "":
		return fmt.Errorf("result %s not found", ref)
	default:
		return errors.New(message)
	}
}
