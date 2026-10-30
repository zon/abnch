package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// readInput reads and parses a command input of the named kind from the file the
// command names, or from stdin when no file is given. The input may be YAML or
// JSON.
func readInput[T any](d deps, file, kind string) (T, error) {
	var input T
	data, err := readInputBytes(d, file, kind)
	if err != nil {
		return input, err
	}
	if json.Valid(data) {
		err = json.Unmarshal(data, &input)
	} else {
		err = yaml.Unmarshal(data, &input)
	}
	if err != nil {
		return input, fmt.Errorf("invalid %s input: %w", kind, err)
	}
	return input, nil
}

// readInputBytes reads the raw input of the named kind from the file the command
// names, or from stdin when no file is given. A relative file resolves under the
// working directory, and a terminal with no file carries no input.
func readInputBytes(d deps, file, kind string) ([]byte, error) {
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
		return nil, fmt.Errorf("no %s input: provide a value on stdin or with --file", kind)
	}
	data, err := io.ReadAll(d.in)
	if err != nil {
		return nil, fmt.Errorf("failed to read the %s input: %w", kind, err)
	}
	return data, nil
}
