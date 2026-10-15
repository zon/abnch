package main

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
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
