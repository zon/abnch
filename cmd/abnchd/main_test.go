package main

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/version"
)

func TestRunRejectsMissingConfigArgument(t *testing.T) {
	err := runIn(context.Background(), nil, io.Discard)
	if err == nil {
		t.Fatal("want an error for a missing config argument")
	}
}

func TestRunRejectsExtraArguments(t *testing.T) {
	err := runIn(context.Background(), []string{"one.yaml", "two.yaml"}, io.Discard)
	if err == nil {
		t.Fatal("want an error for extra arguments")
	}
}

func TestRunReportsMissingConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	err := runIn(context.Background(), []string{path}, io.Discard)
	if err == nil {
		t.Fatal("want an error for a missing config file")
	}
	if !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("error = %q, want it to name the missing file", err)
	}
}

// TestVersionFlagPrintsTheVersion checks `abnchd --version` prints the repo's
// semantic version and reports success, without reading a config.
func TestVersionFlagPrintsTheVersion(t *testing.T) {
	var out bytes.Buffer

	if err := runIn(context.Background(), []string{"--version"}, &out); err != nil {
		t.Fatalf("--version = %v, want no error", err)
	}
	if got := strings.TrimSpace(out.String()); got != version.Version {
		t.Errorf("--version printed %q, want %q", got, version.Version)
	}
}
