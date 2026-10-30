package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/version"
)

// writeConfig writes a service config to a temporary file and returns its path.
func writeConfig(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

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

func TestMigrateRejectsMissingConfigArgument(t *testing.T) {
	err := runIn(context.Background(), []string{"migrate"}, io.Discard)
	if err == nil {
		t.Fatal("want an error for a missing config argument")
	}
}

func TestMigrateReportsMissingConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	err := runIn(context.Background(), []string{"migrate", path}, io.Discard)
	if err == nil {
		t.Fatal("want an error for a missing config file")
	}
	if !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("error = %q, want it to name the missing file", err)
	}
}

// TestMigrateCommandMigrates checks `abnchd migrate <config>` selects the
// migration command instead of serving. A config without a service url is valid
// for a migration, so the error it reaches is the unreachable database, not a
// missing url.
func TestMigrateCommandMigrates(t *testing.T) {
	path := writeConfig(t, "postgres:\n"+
		"  host: 127.0.0.1\n"+
		"  port: 1\n"+
		"  dbname: abnch\n"+
		"  user: abnch\n"+
		"  password: pw\n"+
		"  sslmode: disable\n"+
		"  timeZone: UTC\n")

	err := runIn(context.Background(), []string{"migrate", path}, io.Discard)
	if err == nil {
		t.Fatal("want an error for an unreachable database")
	}
	if strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q, want the migration command, not the service's url check", err)
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Errorf("error = %q, want it to name postgres", err)
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
