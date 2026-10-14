package devconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSetLocalAuthWritesCompleteLocalConfig(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	in := strings.NewReader("db.example\n5432\nabnch\nadmin\ns3cret\n\n\n")

	path, err := SetLocalAuth(Options{Port: 9000, RepoRoot: repo, HomeDir: home, In: in, Out: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}

	wantPath := filepath.Join(repo, ConfigFileName)
	if path != wantPath {
		t.Errorf("path = %q, want %q", path, wantPath)
	}

	cfg := readConfigFile(t, wantPath)
	if cfg.URL != "http://localhost:9000" {
		t.Errorf("url = %q", cfg.URL)
	}
	wantPostgres := Postgres{Host: "db.example", Port: 5432, DBName: "abnch", User: "admin", Password: "s3cret", SSLMode: "disable", TimeZone: "UTC"}
	if !reflect.DeepEqual(cfg.Postgres, wantPostgres) {
		t.Errorf("postgres = %#v, want %#v", cfg.Postgres, wantPostgres)
	}

	if len(cfg.OAuth.Issuers) != 1 {
		t.Fatalf("got %d issuers, want 1", len(cfg.OAuth.Issuers))
	}
	if !reflect.DeepEqual(cfg.OAuth.Issuers[0], DevIssuer("http://localhost:9000")) {
		t.Errorf("issuer = %#v", cfg.OAuth.Issuers[0])
	}

	keyPath := filepath.Join(repo, KeyFileName)
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("dev key missing: %v", err)
	}

	creds := readCredentialsFile(t, filepath.Join(home, credentialsDir, credentialsName))
	got, ok := creds["http://localhost:9000"]
	if !ok {
		t.Fatal("local credential missing")
	}
	wantCred := Credential{Issuer: "http://localhost:9000/dev", Audience: "abnch", Subject: "local-cli", KeyFile: keyPath}
	if !reflect.DeepEqual(got, wantCred) {
		t.Errorf("credential = %#v, want %#v", got, wantCred)
	}

	gitignore := readFile(t, filepath.Join(repo, gitignoreName))
	if !strings.Contains(gitignore, ConfigFileName) || !strings.Contains(gitignore, KeyFileName) {
		t.Errorf(".gitignore does not exclude the generated files:\n%s", gitignore)
	}
}

func TestReadConfigMissingFileIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), ConfigFileName)

	cfg, err := readConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, Config{}) {
		t.Errorf("config = %#v, want an empty config for a first run", cfg)
	}
}

func TestSetLocalAuthRerunUsesStoredDefaults(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home, In: strings.NewReader("db.example\n5432\nabnch\nadmin\ns3cret\n\n\n"), Out: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}

	// Accept every default.
	if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home, In: strings.NewReader("\n\n\n\n\n\n\n"), Out: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}

	cfg := readConfigFile(t, filepath.Join(repo, ConfigFileName))
	want := Postgres{Host: "db.example", Port: 5432, DBName: "abnch", User: "admin", Password: "s3cret", SSLMode: "disable", TimeZone: "UTC"}
	if !reflect.DeepEqual(cfg.Postgres, want) {
		t.Errorf("postgres = %#v, want %#v", cfg.Postgres, want)
	}
	if len(cfg.OAuth.Issuers) != 1 {
		t.Errorf("got %d issuers, want 1", len(cfg.OAuth.Issuers))
	}
}

func TestSetLocalAuthRerunChangesOneValue(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home, In: strings.NewReader("old\n5432\nabnch\nadmin\ns3cret\n\n\n"), Out: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}

	if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home, In: strings.NewReader("new\n\n\n\n\n\n\n"), Out: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}

	cfg := readConfigFile(t, filepath.Join(repo, ConfigFileName))
	if cfg.Postgres.Host != "new" {
		t.Errorf("host = %q, want new", cfg.Postgres.Host)
	}
	if cfg.Postgres.Port != 5432 {
		t.Errorf("port = %d, want 5432", cfg.Postgres.Port)
	}
}

func TestSetLocalAuthRerunOffersStoredValuesAsDefaults(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	stored := "db.example\n5432\nabnch\nadmin\ns3cret\nrequire\nAmerica/New_York\n"
	if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home, In: strings.NewReader(stored), Out: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	// Accept every default, so no stored value is retyped.
	if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home, In: strings.NewReader("\n\n\n\n\n\n\n"), Out: &out}); err != nil {
		t.Fatal(err)
	}

	prompts := out.String()
	for _, want := range []string{
		"host [db.example]",
		"port [5432]",
		"dbname [abnch]",
		"user [admin]",
		"sslmode [require]",
		"timeZone [America/New_York]",
	} {
		if !strings.Contains(prompts, want) {
			t.Errorf("rerun prompt does not offer the stored value %q:\n%s", want, prompts)
		}
	}
	if !strings.Contains(prompts, "password [stored]") {
		t.Errorf("rerun password prompt does not offer the stored value:\n%s", prompts)
	}
	if strings.Contains(prompts, "s3cret") {
		t.Errorf("rerun prompt reveals the stored secret:\n%s", prompts)
	}
}

func TestSetLocalAuthRerunKeepsOneDevIssuer(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	for i := 0; i < 2; i++ {
		if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home, In: strings.NewReader("db\n5432\nabnch\nadmin\npw\n\n\n"), Out: &bytes.Buffer{}}); err != nil {
			t.Fatal(err)
		}
	}

	cfg := readConfigFile(t, filepath.Join(repo, ConfigFileName))
	if len(cfg.OAuth.Issuers) != 1 {
		t.Errorf("got %d issuers, want 1", len(cfg.OAuth.Issuers))
	}
}

func TestSetLocalAuthKeepsOtherCredentials(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	dir := filepath.Join(home, credentialsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	other := map[string]Credential{"https://bench.example.com": {Subject: "human"}}
	data, err := MarshalCredentials(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, credentialsName), data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home, In: strings.NewReader("db\n5432\nabnch\nadmin\npw\n\n\n"), Out: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}

	creds := readCredentialsFile(t, filepath.Join(dir, credentialsName))
	if creds["https://bench.example.com"].Subject != "human" {
		t.Error("other credential changed")
	}
	if creds["http://localhost:8080"].Subject != "local-cli" {
		t.Error("local credential missing")
	}
}

func TestLocalCredentialDerivesFromDevIssuer(t *testing.T) {
	cred := LocalCredential("http://localhost:8080", "/repo/dev-issuer.key")
	iss := DevIssuer("http://localhost:8080")

	if cred.Issuer != iss.Issuer {
		t.Errorf("issuer = %q, want the dev issuer %q", cred.Issuer, iss.Issuer)
	}
	if cred.Audience != iss.Audience {
		t.Errorf("audience = %q, want the dev issuer audience %q", cred.Audience, iss.Audience)
	}
	if cred.Subject != "local-cli" {
		t.Errorf("subject = %q, want local-cli", cred.Subject)
	}
	if role, ok := iss.RoleMap[cred.Subject]; !ok || role != "Admin" {
		t.Errorf("dev issuer maps subject %q to %q, want Admin", cred.Subject, role)
	}
	if cred.KeyFile != "/repo/dev-issuer.key" {
		t.Errorf("keyFile = %q, want the dev key", cred.KeyFile)
	}
}

func TestLocalServiceURL(t *testing.T) {
	cases := map[int]string{
		8080: "http://localhost:8080",
		9000: "http://localhost:9000",
	}
	for port, want := range cases {
		if got := LocalServiceURL(port); got != want {
			t.Errorf("LocalServiceURL(%d) = %q, want %q", port, got, want)
		}
	}
}

func TestSetLocalAuthRejectsBadPostgresPort(t *testing.T) {
	repo := t.TempDir()
	in := strings.NewReader("db\nnot-a-port\nabnch\nadmin\npw\n\n\n")
	if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: t.TempDir(), In: in, Out: &bytes.Buffer{}}); err == nil {
		t.Fatal("want an error for a bad postgres port")
	}
}

func TestAskPostgresPromptsEachFieldInTurn(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("db.example\n5432\nabnch\nadmin\ns3cret\nrequire\nAmerica/New_York\n")

	pg, err := askPostgres(NewPrompter(in, &out), Postgres{})
	if err != nil {
		t.Fatal(err)
	}

	want := Postgres{Host: "db.example", Port: 5432, DBName: "abnch", User: "admin", Password: "s3cret", SSLMode: "require", TimeZone: "America/New_York"}
	if !reflect.DeepEqual(pg, want) {
		t.Errorf("postgres = %#v, want %#v", pg, want)
	}
	assertPromptsInOrder(t, out.String(), "host", "port", "dbname", "user", "password", "sslmode", "timeZone")
}

func TestAskPostgresDefaultsSSLModeAndTimeZone(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("db.example\n5432\nabnch\nadmin\ns3cret\n\n\n")

	pg, err := askPostgres(NewPrompter(in, &out), Postgres{})
	if err != nil {
		t.Fatal(err)
	}

	if pg.SSLMode != "disable" {
		t.Errorf("sslmode = %q, want disable", pg.SSLMode)
	}
	if pg.TimeZone != "UTC" {
		t.Errorf("timeZone = %q, want UTC", pg.TimeZone)
	}
	if !strings.Contains(out.String(), "sslmode [disable]") {
		t.Errorf("sslmode prompt does not offer disable: %q", out.String())
	}
	if !strings.Contains(out.String(), "timeZone [UTC]") {
		t.Errorf("timeZone prompt does not offer UTC: %q", out.String())
	}
}

func TestAskPostgresHidesPasswordPrompt(t *testing.T) {
	var out bytes.Buffer
	p := NewPrompter(strings.NewReader("db.example\n5432\nabnch\nadmin\ns3cret\n\n\n"), &out)
	var off int
	p.echoOff = func() { off++ }
	p.echoOn = func() {}

	if _, err := askPostgres(p, Postgres{}); err != nil {
		t.Fatal(err)
	}
	if off != 1 {
		t.Errorf("echo off called %d times, want 1 for the password prompt", off)
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

func readConfigFile(t *testing.T, path string) Config {
	t.Helper()
	cfg, err := UnmarshalConfig([]byte(readFile(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func readCredentialsFile(t *testing.T, path string) map[string]Credential {
	t.Helper()
	creds, err := UnmarshalCredentials([]byte(readFile(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	return creds
}
