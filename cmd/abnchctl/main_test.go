package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/devconfig"
)

func TestSetLocalAuthWritesConfigAndReportsPath(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	var out bytes.Buffer
	in := strings.NewReader("db.example\n5433\nabnch\nadmin\ns3cret\nrequire\nAmerica/New_York\n")

	if err := runIn([]string{"set", "local-auth"}, repo, home, in, &out); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(repo, "service-config.local.yaml")
	if !strings.Contains(out.String(), path) {
		t.Errorf("output %q does not report the config path %q", out.String(), path)
	}

	cfg := readConfig(t, path)
	if cfg.URL != "http://localhost:8080" {
		t.Errorf("url = %q, want http://localhost:8080", cfg.URL)
	}
	want := devconfig.Postgres{Host: "db.example", Port: 5433, DBName: "abnch", User: "admin", Password: "s3cret", SSLMode: "require", TimeZone: "America/New_York"}
	if !reflect.DeepEqual(cfg.Postgres, want) {
		t.Errorf("postgres = %#v, want %#v", cfg.Postgres, want)
	}
}

func TestSetLocalAuthPortSetsURL(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default", []string{"set", "local-auth"}, "http://localhost:8080"},
		{"given", []string{"set", "local-auth", "9090"}, "http://localhost:9090"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			err := runIn(tc.args, repo, t.TempDir(), strings.NewReader("db\n5432\nabnch\nadmin\npw\n\n\n"), &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			cfg := readConfig(t, filepath.Join(repo, "service-config.local.yaml"))
			if cfg.URL != tc.want {
				t.Errorf("url = %q, want %q", cfg.URL, tc.want)
			}
		})
	}
}

func TestSetLocalAuthPromptsForEachPostgresFieldInTurn(t *testing.T) {
	repo := t.TempDir()
	var out bytes.Buffer
	in := strings.NewReader("db.example\n5432\nabnch\nadmin\ns3cret\n\n\n")

	if err := runIn([]string{"set", "local-auth"}, repo, t.TempDir(), in, &out); err != nil {
		t.Fatal(err)
	}

	prompts := out.String()
	assertPromptsInOrder(t, prompts, "host", "port", "dbname", "user", "password", "sslmode", "timeZone")

	if strings.Contains(prompts, "s3cret") {
		t.Errorf("the password prompt reveals the typed secret:\n%s", prompts)
	}
	if !strings.Contains(prompts, "sslmode [disable]") {
		t.Errorf("sslmode prompt does not default to disable:\n%s", prompts)
	}
	if !strings.Contains(prompts, "timeZone [UTC]") {
		t.Errorf("timeZone prompt does not default to UTC:\n%s", prompts)
	}
}

func assertPromptsInOrder(t *testing.T, out string, labels ...string) {
	t.Helper()
	at := 0
	for _, label := range labels {
		i := strings.Index(out[at:], label)
		if i < 0 {
			t.Fatalf("prompt %q is missing or out of order in %q", label, out)
		}
		at += i + len(label)
	}
}

func TestCheckConfigConfirmsWrittenConfig(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	if err := runIn([]string{"set", "local-auth"}, repo, home, strings.NewReader("db\n5432\nabnch\nadmin\npw\n\n\n"), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runIn([]string{"check", "config"}, repo, home, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ready") {
		t.Errorf("check output %q does not confirm the config", out.String())
	}
}

func TestCheckConfigFailsWhenMissing(t *testing.T) {
	repo := t.TempDir()
	err := runIn([]string{"check", "config"}, repo, t.TempDir(), strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("check config = nil, want an error for a missing config")
	}
	if !strings.Contains(err.Error(), "service-config.local.yaml") {
		t.Errorf("error = %q, want it to name the missing file", err)
	}
}

func TestRunInRejectsUnknownCommand(t *testing.T) {
	err := runIn([]string{"set", "nothing"}, t.TempDir(), t.TempDir(), strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("unknown command: want an error")
	}
}

func TestRunInRejectsMissingCommand(t *testing.T) {
	err := runIn(nil, t.TempDir(), t.TempDir(), strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("missing command: want an error")
	}
}

func TestParsePortDefaults(t *testing.T) {
	got, err := parsePort(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != 8080 {
		t.Errorf("port = %d, want 8080", got)
	}
}

func TestParsePortGiven(t *testing.T) {
	for _, args := range [][]string{
		{"9090"},
		{"--port", "9090"},
		{"--port=9090"},
		{"port=9090"},
	} {
		got, err := parsePort(args)
		if err != nil {
			t.Errorf("%v: %v", args, err)
			continue
		}
		if got != 9090 {
			t.Errorf("%v: port = %d, want 9090", args, got)
		}
	}
}

func TestParsePortRejectsInvalid(t *testing.T) {
	for _, args := range [][]string{
		{"abc"}, {"0"}, {"70000"}, {"-1"}, {"--port"},
	} {
		if _, err := parsePort(args); err == nil {
			t.Errorf("%v: want an error", args)
		}
	}
}

func readConfig(t *testing.T, path string) devconfig.Config {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := devconfig.UnmarshalConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
