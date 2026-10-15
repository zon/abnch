package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// dataRef is a datum's ref as the service API returns it.
type dataRef struct {
	Ref string `json:"ref"`
}

// datumMissingMessage is the message the service reports for a datum it does not
// know.
const datumMissingMessage = "datum not found"

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
		return "", dataNotFoundError(resp, org, repo, task, stub, "")
	default:
		return "", serviceError(resp)
	}
}

// dataNotFoundError maps a data request's not-found response to the resource the
// service does not know. A ref names the datum when the response reports one.
func dataNotFoundError(resp *http.Response, org, repo, task, stub, ref string) error {
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
	case message == datumMissingMessage && ref != "":
		return fmt.Errorf("datum %s not found", ref)
	default:
		return errors.New(message)
	}
}

// dataPath returns the API path for a test's data.
func dataPath(org, repo, task, stub string) string {
	return testPath(org, repo, task, stub) + "/data"
}

// datumPath returns the API path for one datum in a test's data.
func datumPath(org, repo, task, stub, ref string) string {
	return dataPath(org, repo, task, stub) + "/" + url.PathEscape(ref)
}

// splitDataPath splits the data path into its task, test, and optional ref. A
// path without a ref names the whole test.
func splitDataPath(path string) (task, test, ref string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] == "" || (len(parts) == 3 && parts[2] == "") {
		return "", "", "", fmt.Errorf("data path %q must be <task>/<test> or <task>/<test>/<ref>", path)
	}
	if len(parts) == 3 {
		ref = parts[2]
	}
	return parts[0], parts[1], ref, nil
}

// readDataRefs returns the refs of the data in the test the repo, task, and stub
// identify. A missing repo, task, or test is an error.
func readDataRefs(ctx context.Context, client *serviceClient, org, repo, task, stub string) ([]string, error) {
	resp, err := client.do(ctx, http.MethodGet, dataPath(org, repo, task, stub), nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		var refs []string
		if err := json.NewDecoder(resp.Body).Decode(&refs); err != nil {
			return nil, fmt.Errorf("failed to read the data refs: %w", err)
		}
		return refs, nil
	case http.StatusNotFound:
		return nil, dataNotFoundError(resp, org, repo, task, stub, "")
	default:
		return nil, serviceError(resp)
	}
}

// readDatum returns the JSON of the datum the ref names under the test. A
// missing repo, task, test, or datum is an error.
func readDatum(ctx context.Context, client *serviceClient, org, repo, task, stub, ref string) ([]byte, error) {
	resp, err := client.do(ctx, http.MethodGet, datumPath(org, repo, task, stub, ref), nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read the datum: %w", err)
		}
		return body, nil
	case http.StatusNotFound:
		return nil, dataNotFoundError(resp, org, repo, task, stub, ref)
	default:
		return nil, serviceError(resp)
	}
}

// writeRefs writes the data refs one per line.
func writeRefs(w io.Writer, refs []string) error {
	for _, ref := range refs {
		if _, err := fmt.Fprintln(w, ref); err != nil {
			return err
		}
	}
	return nil
}

// writeJSON writes raw JSON to w, ending it with a newline when it does not
// carry one.
func writeJSON(w io.Writer, data []byte) error {
	if _, err := w.Write(data); err != nil {
		return err
	}
	if len(data) > 0 && data[len(data)-1] == '\n' {
		return nil
	}
	_, err := fmt.Fprintln(w)
	return err
}
