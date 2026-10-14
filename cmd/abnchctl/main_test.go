package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zon/abnch/internal/devconfig"
	"go.yaml.in/yaml/v3"
)

// postgresSeed stands in for the Postgres block a human or an agent writes by
// hand before the command runs.
const postgresSeed = "postgres:\n" +
	"  host: db\n" +
	"  port: 5432\n" +
	"  dbname: abnch\n" +
	"  user: admin\n" +
	"  password: pw\n" +
	"  sslmode: disable\n" +
	"  timeZone: UTC\n"

func TestSetLocalAuthWritesConfigAndReportsPath(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	var out bytes.Buffer

	if err := runIn([]string{"set", "local-auth"}, repo, home, &out); err != nil {
		t.Fatal(err)
	}

	path := devconfig.ConfigPath(repo)
	if !strings.Contains(out.String(), path) {
		t.Errorf("output %q does not report the config path %q", out.String(), path)
	}

	cfg := readConfig(t, path)
	if cfg.URL != "http://localhost:8080" {
		t.Errorf("url = %q, want http://localhost:8080", cfg.URL)
	}
	if !reflect.DeepEqual(cfg.Postgres, devconfig.Postgres{}) {
		t.Errorf("postgres = %#v, want it untouched", cfg.Postgres)
	}
}

func TestSetLocalAuthKeepsPostgresAndAddsAuth(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	path := devconfig.ConfigPath(repo)
	writeConfigFile(t, repo, postgresSeed)

	if err := runIn([]string{"set", "local-auth"}, repo, home, &bytes.Buffer{}); err != nil {
		t.Fatalf("set local-auth = %v, want no error", err)
	}

	cfg := readConfig(t, path)
	if err := devconfig.Check(cfg); err != nil {
		t.Errorf("set local-auth left the config incomplete: %v", err)
	}
	if cfg.URL != "http://localhost:8080" {
		t.Errorf("url = %q, want http://localhost:8080", cfg.URL)
	}
	if cfg.Postgres.Host != "db" {
		t.Errorf("postgres host = %q, want the hand-written db", cfg.Postgres.Host)
	}
	if len(cfg.OAuth.Issuers) != 1 {
		t.Errorf("got %d issuers, want the one dev issuer", len(cfg.OAuth.Issuers))
	}
}

func TestSetLocalAuthWritesAsymmetricDevKeyAtRepoRoot(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	if err := runIn([]string{"set", "local-auth"}, repo, home, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	keyPath := filepath.Join(repo, "dev-issuer.key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("dev-issuer.key is not in the repo root: %v", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("dev key is not PEM:\n%s", data)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("dev key is not a parseable private key: %v", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("dev key is %T, want an asymmetric ECDSA key pair", parsed)
	}
	if key.Curve != elliptic.P256() {
		t.Errorf("curve = %v, want P-256", key.Curve)
	}

	// The public half is derivable, so a local service can serve the matching JWKS.
	if _, err := x509.MarshalPKIXPublicKey(&key.PublicKey); err != nil {
		t.Fatalf("dev key has no derivable public key: %v", err)
	}
}

func TestSetLocalAuthWritesTopLevelURL(t *testing.T) {
	repo := t.TempDir()
	var out bytes.Buffer

	if err := runIn([]string{"set", "local-auth", "--port", "9090"}, repo, t.TempDir(), &out); err != nil {
		t.Fatal(err)
	}

	path := devconfig.ConfigPath(repo)
	if !strings.Contains(out.String(), path) {
		t.Errorf("output %q does not report the config path %q", out.String(), path)
	}

	doc := readYAMLDoc(t, path)
	if doc["url"] != "http://localhost:9090" {
		t.Errorf("top-level url = %#v, want http://localhost:9090", doc["url"])
	}
}

func TestSetLocalAuthWritesDevIssuer(t *testing.T) {
	repo := t.TempDir()

	if err := runIn([]string{"set", "local-auth", "--port", "9090"}, repo, t.TempDir(), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLDoc(t, devconfig.ConfigPath(repo))
	iss, ok := findIssuer(doc, "http://localhost:9090/dev")
	if !ok {
		t.Fatalf("service config does not trust the dev issuer:\n%#v", doc)
	}
	if iss["audience"] != "abnch" {
		t.Errorf("audience = %#v, want abnch", iss["audience"])
	}
	if iss["roleClaim"] != "sub" {
		t.Errorf("roleClaim = %#v, want sub", iss["roleClaim"])
	}
	want := map[string]any{"local-cli": "Admin", "local-runtime": "Service"}
	if roleMap, ok := iss["roleMap"].(map[string]any); !ok {
		t.Errorf("roleMap = %#v, want a mapping", iss["roleMap"])
	} else if !reflect.DeepEqual(roleMap, want) {
		t.Errorf("roleMap = %#v, want %#v", roleMap, want)
	}
}

func TestSetLocalAuthRerunSamePortKeepsOneDevIssuer(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	if err := runIn([]string{"set", "local-auth", "--port", "9090"}, repo, home, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if got := countDevIssuers(t, repo); got != 1 {
		t.Fatalf("first run wrote %d issuers, want one dev issuer", got)
	}

	if err := runIn([]string{"set", "local-auth", "--port", "9090"}, repo, home, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if got := countDevIssuers(t, repo); got != 1 {
		t.Errorf("rerun for the same port wrote %d issuers, want one dev issuer", got)
	}
}

// countDevIssuers reads the written YAML and counts the literal entries under
// oauth.issuers, so a duplicate entry shows up even when the typed config would
// collapse it.
func countDevIssuers(t *testing.T, repo string) int {
	t.Helper()

	data, err := os.ReadFile(devconfig.ConfigPath(repo))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}

	oauth, ok := doc["oauth"].(map[string]any)
	if !ok {
		t.Fatalf("oauth = %#v, want a top-level oauth mapping", doc["oauth"])
	}
	issuers, ok := oauth["issuers"].([]any)
	if !ok {
		t.Fatalf("oauth.issuers = %#v, want a list", oauth["issuers"])
	}
	for _, raw := range issuers {
		iss, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("issuer = %#v, want a mapping", raw)
		}
		if iss["issuer"] != "http://localhost:9090/dev" {
			t.Errorf("issuer = %#v, want http://localhost:9090/dev", iss["issuer"])
		}
	}
	return len(issuers)
}

func TestSetLocalAuthPortSetsURL(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default", []string{"set", "local-auth"}, "http://localhost:8080"},
		{"given", []string{"set", "local-auth", "--port", "9090"}, "http://localhost:9090"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			err := runIn(tc.args, repo, t.TempDir(), &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			cfg := readConfig(t, devconfig.ConfigPath(repo))
			if cfg.URL != tc.want {
				t.Errorf("url = %q, want %q", cfg.URL, tc.want)
			}
		})
	}
}

func TestCheckConfigConfirmsConfigWithPostgresAndAuth(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	writeConfigFile(t, repo, postgresSeed)

	if err := runIn([]string{"set", "local-auth"}, repo, home, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runIn([]string{"check", "config"}, repo, home, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ready") {
		t.Errorf("check output %q does not confirm the config", out.String())
	}
}

func TestCheckConfigFailsWhenMissing(t *testing.T) {
	repo := t.TempDir()
	err := runIn([]string{"check", "config"}, repo, t.TempDir(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("check config = nil, want an error for a missing config")
	}
	if !strings.Contains(err.Error(), devconfig.ConfigFileName) {
		t.Errorf("error = %q, want it to name the missing file", err)
	}
	if _, statErr := os.Stat(devconfig.ConfigPath(repo)); !os.IsNotExist(statErr) {
		t.Errorf("check config created the missing file: stat = %v", statErr)
	}
}

func TestCheckConfigFailsWhenPostgresValueMissing(t *testing.T) {
	repo := t.TempDir()
	// Complete except for the host, so the failure names the missing value.
	cfg := "url: http://localhost:8080\npostgres:\n  host: \"\"\n  port: 5432\n  dbname: abnch\n  user: admin\n  password: pw\n  sslmode: disable\n  timeZone: UTC\noauth:\n  issuers:\n    - issuer: http://localhost:8080/dev\n      audience: abnch\n      roleClaim: sub\n      roleMap:\n        local-cli: Admin\n"
	writeConfigFile(t, repo, cfg)

	err := runIn([]string{"check", "config"}, repo, t.TempDir(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("check config = nil, want an error for a missing postgres value")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Errorf("error = %q, want it to name the missing postgres value", err)
	}
}

func TestCheckConfigFailsWhenAuthEntryMissing(t *testing.T) {
	repo := t.TempDir()
	cfg := "url: http://localhost:8080\npostgres:\n  host: db\n  port: 5432\n  dbname: abnch\n  user: admin\n  password: pw\n  sslmode: disable\n  timeZone: UTC\n"
	writeConfigFile(t, repo, cfg)

	err := runIn([]string{"check", "config"}, repo, t.TempDir(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("check config = nil, want an error for a config with no auth entry")
	}
	if !strings.Contains(err.Error(), "auth entry") {
		t.Errorf("error = %q, want it to name the missing auth entry", err)
	}
}

func TestConfigFlagPointsSetAndCheckAtCustomPath(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	custom := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(custom, []byte(postgresSeed), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runIn([]string{"--config", custom, "set", "local-auth"}, repo, home, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runIn([]string{"--config", custom, "check", "config"}, repo, home, &out); err != nil {
		t.Fatalf("check config = %v, want success:\n%s", err, out.String())
	}

	if _, err := os.Stat(devconfig.ConfigPath(repo)); !os.IsNotExist(err) {
		t.Errorf("the default config path was written despite --config: stat = %v", err)
	}
}

func TestRunInRejectsUnknownCommand(t *testing.T) {
	err := runIn([]string{"set", "nothing"}, t.TempDir(), t.TempDir(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("unknown command: want an error")
	}
}

func TestRunInRejectsMissingCommand(t *testing.T) {
	err := runIn(nil, t.TempDir(), t.TempDir(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("missing command: want an error")
	}
}

func TestSetLocalAuthRejectsInvalidPort(t *testing.T) {
	for _, args := range [][]string{
		{"set", "local-auth", "--port", "abc"},
		{"set", "local-auth", "--port", "0"},
		{"set", "local-auth", "--port", "70000"},
		{"set", "local-auth", "--port", "-1"},
		{"set", "local-auth", "--port"},
	} {
		err := runIn(args, t.TempDir(), t.TempDir(), &bytes.Buffer{})
		if err == nil {
			t.Errorf("%v: want an error", args)
		}
	}
}

func TestSetLocalAuthWritesLocalCredentialKeepingOtherURLs(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	credsPath := filepath.Join(home, ".abnch", "credentials.yaml")
	if err := os.MkdirAll(filepath.Dir(credsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	seed := "https://bench.example.com:\n    issuer: https://issuer.example.com\n    audience: abnch\n    subject: human\n    keyFile: /home/bench/dev-issuer.key\n"
	if err := os.WriteFile(credsPath, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runIn([]string{"set", "local-auth", "--port", "9090"}, repo, home, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	creds := readCredentials(t, credsPath)

	other, ok := creds["https://bench.example.com"].(map[string]any)
	if !ok {
		t.Fatalf("credential for another service URL = %#v, want it kept", creds["https://bench.example.com"])
	}
	if other["subject"] != "human" {
		t.Errorf("credential for another service URL changed: %#v", other)
	}

	local, ok := creds["http://localhost:9090"].(map[string]any)
	if !ok {
		t.Fatalf("local credential = %#v, want one keyed by the local service URL", creds["http://localhost:9090"])
	}
	if local["subject"] != "local-cli" {
		t.Errorf("local credential subject = %#v, want local-cli", local["subject"])
	}
	if local["issuer"] != "http://localhost:9090/dev" {
		t.Errorf("local credential issuer = %#v, want http://localhost:9090/dev", local["issuer"])
	}
	if local["audience"] != "abnch" {
		t.Errorf("local credential audience = %#v, want abnch", local["audience"])
	}
}

func TestSetLocalAuthCredentialAuthenticatesAsDevIssuerSubject(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	if err := runIn([]string{"set", "local-auth", "--port", "9090"}, repo, home, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	// The service trusts a dev issuer whose role map grants Admin to the local
	// CLI subject.
	doc := readYAMLDoc(t, devconfig.ConfigPath(repo))
	devIssuer, ok := findIssuer(doc, "http://localhost:9090/dev")
	if !ok {
		t.Fatalf("service config does not trust the dev issuer:\n%#v", doc)
	}
	if devIssuer["roleClaim"] != "sub" {
		t.Errorf("roleClaim = %#v, want sub", devIssuer["roleClaim"])
	}
	roleMap, ok := devIssuer["roleMap"].(map[string]any)
	if !ok {
		t.Fatalf("roleMap = %#v, want a mapping", devIssuer["roleMap"])
	}
	if roleMap["local-cli"] != "Admin" {
		t.Errorf("roleMap[local-cli] = %#v, want Admin", roleMap["local-cli"])
	}

	// The stored credential names that issuer and the subject the role map maps,
	// so the CLI authenticates as local-cli without signing in.
	creds := readCredentials(t, filepath.Join(home, ".abnch", "credentials.yaml"))
	local, ok := creds["http://localhost:9090"].(map[string]any)
	if !ok {
		t.Fatalf("local credential = %#v, want one keyed by the local service URL", creds["http://localhost:9090"])
	}
	if local["subject"] != "local-cli" {
		t.Errorf("subject = %#v, want the dev issuer's local-cli subject", local["subject"])
	}
	if local["issuer"] != devIssuer["issuer"] {
		t.Errorf("issuer = %#v, want the dev issuer %#v", local["issuer"], devIssuer["issuer"])
	}
	if local["audience"] != devIssuer["audience"] {
		t.Errorf("audience = %#v, want the dev issuer audience %#v", local["audience"], devIssuer["audience"])
	}

	// The credential points at the dev key, so the CLI can sign the token it
	// presents to the local service.
	keyFile, _ := local["keyFile"].(string)
	if keyFile != filepath.Join(repo, "dev-issuer.key") {
		t.Errorf("keyFile = %q, want the dev key at the repo root", keyFile)
	}
	if _, err := os.Stat(keyFile); err != nil {
		t.Errorf("credential key is missing: %v", err)
	}
}

// writeConfigFile writes a local service config in a repo, creating the
// directory that holds it.
func writeConfigFile(t *testing.T, repo, content string) {
	t.Helper()
	path := devconfig.ConfigPath(repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readYAMLDoc(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// findIssuer returns the literal issuer mapping with the given issuer URL.
func findIssuer(doc map[string]any, url string) (map[string]any, bool) {
	oauth, ok := doc["oauth"].(map[string]any)
	if !ok {
		return nil, false
	}
	issuers, ok := oauth["issuers"].([]any)
	if !ok {
		return nil, false
	}
	for _, raw := range issuers {
		iss, ok := raw.(map[string]any)
		if ok && iss["issuer"] == url {
			return iss, true
		}
	}
	return nil, false
}

func readCredentials(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	creds := map[string]any{}
	if err := yaml.Unmarshal(data, &creds); err != nil {
		t.Fatal(err)
	}
	return creds
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
