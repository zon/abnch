package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// dataRef is a datum's ref as the service API returns it.
type dataRef struct {
	Ref string `json:"ref"`
}

// addData adds the JSON the command reads to the test the path names in the
// inferred repo and prints the datum's ref. It reads the JSON from `--file` when
// given and from stdin otherwise. It fails without a request when the input is
// not valid JSON or when the path names no test.
func addData(d deps, c addDataCmd) error {
	task, stub, err := splitTestPath(c.Path)
	if err != nil {
		return err
	}

	body, err := readDatumBody(d, c.File)
	if err != nil {
		return err
	}

	target, err := resolveTarget(d)
	if err != nil {
		return err
	}

	ref, err := writeDatum(d.ctx, target.client, target.org, target.stub, task, stub, body)
	if err != nil {
		return err
	}
	fmt.Fprintln(d.out, ref)
	return nil
}

// readDatumBody reads a datum's JSON from the file the command names, or from
// stdin when no file is given. It fails when the input is not valid JSON.
func readDatumBody(d deps, file string) ([]byte, error) {
	data, err := readDatumInput(d, file)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, errors.New("invalid JSON datum")
	}
	return data, nil
}

// readDatumInput reads the raw datum bytes from the file the command names, or
// from stdin when no file is given. A relative file resolves under the working
// directory. A terminal with no file carries no input.
func readDatumInput(d deps, file string) ([]byte, error) {
	if file != "" {
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(d.workDir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file, err)
		}
		return data, nil
	}
	if !pipedInput(d.in) {
		return nil, errors.New("no JSON input: provide a value on stdin or with --file")
	}
	data, err := io.ReadAll(d.in)
	if err != nil {
		return nil, fmt.Errorf("failed to read the datum: %w", err)
	}
	return data, nil
}

// writeDatum posts the JSON body to the test the repo, task, and stub identify
// and returns the datum's ref. The service returns the existing ref when the
// same JSON is already stored, so both a create and an existing datum succeed.
// A not-found response maps to the missing repo, task, or test.
func writeDatum(ctx context.Context, client *serviceClient, org, repo, task, stub string, body []byte) (string, error) {
	resp, err := client.do(ctx, http.MethodPost, dataPath(org, repo, task, stub), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		defer resp.Body.Close()
		var ref dataRef
		if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
			return "", fmt.Errorf("failed to read the datum ref: %w", err)
		}
		return ref.Ref, nil
	case http.StatusNotFound:
		return "", dataNotFoundError(resp, org, repo, task, stub)
	default:
		return "", serviceError(resp)
	}
}

// dataNotFoundError maps a data write's not-found response to the resource the
// service does not know.
func dataNotFoundError(resp *http.Response, org, repo, task, stub string) error {
	message, ok := serviceMessage(resp)
	switch {
	case !ok:
		return fmt.Errorf("service returned %s", resp.Status)
	case message == repoMissingMessage:
		return repoMissing{org: org, repo: repo}
	case message == taskMissingMessage:
		return fmt.Errorf("task %s not found", task)
	case message == testMissingMessage:
		return fmt.Errorf("test %s not found", stub)
	default:
		return errors.New(message)
	}
}

// dataPath returns the API path for a test's data.
func dataPath(org, repo, task, stub string) string {
	return testPath(org, repo, task, stub) + "/data"
}
