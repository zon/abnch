package devconfig

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestSetLocalAuthWritesLocalAuthConfig(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	path, err := SetLocalAuth(Options{Port: 9000, RepoRoot: repo, HomeDir: home})
	if err != nil {
		t.Fatal(err)
	}

	wantPath := ConfigPath(repo)
	if path != wantPath {
		t.Errorf("path = %q, want %q", path, wantPath)
	}

	cfg := readConfigFile(t, wantPath)
	if cfg.URL != "http://localhost:9000" {
		t.Errorf("url = %q", cfg.URL)
	}
	if !reflect.DeepEqual(cfg.Postgres, Postgres{}) {
		t.Errorf("postgres = %#v, want it untouched", cfg.Postgres)
	}

	if len(cfg.OAuth.Issuers) != 1 {
		t.Fatalf("got %d issuers, want 1", len(cfg.OAuth.Issuers))
	}
	if !reflect.DeepEqual(cfg.OAuth.Issuers[0], DevIssuer("http://localhost:9000")) {
		t.Errorf("issuer = %#v", cfg.OAuth.Issuers[0])
	}

	keyPath := filepath.Join(repo, ConfigDir, KeyFileName)
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
	if !strings.Contains(gitignore, ConfigGitignore) {
		t.Errorf(".gitignore does not exclude the generated files:\n%s", gitignore)
	}
}

// TestSetLocalAuthWritesPlaintextLoopbackConfig keeps local development on
// plaintext loopback: the URL is http on localhost and the config names no TLS
// block.
func TestSetLocalAuthWritesPlaintextLoopbackConfig(t *testing.T) {
	repo := t.TempDir()

	path, err := SetLocalAuth(Options{Port: 9000, RepoRoot: repo, HomeDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	cfg := readConfigFile(t, path)
	if cfg.URL != "http://localhost:9000" {
		t.Errorf("url = %q, want http://localhost:9000", cfg.URL)
	}
	if cfg.Listen != "" {
		t.Errorf("listen = %q, want no listen address so the service uses the url loopback", cfg.Listen)
	}
	if cfg.TLS != nil {
		t.Errorf("tls = %#v, want no TLS block for local development", cfg.TLS)
	}

	doc := readYAMLDoc(t, path)
	if _, ok := doc["tls"]; ok {
		t.Errorf("config wrote a tls block: %#v", doc["tls"])
	}
}

func TestSetLocalAuthPreservesPostgres(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()
	path := ConfigPath(repo)

	// A human or an agent writes the Postgres block before the command runs.
	handwritten := "url: http://localhost:1111\n" +
		"postgres:\n" +
		"  host: db.example\n" +
		"  port: 5433\n" +
		"  dbname: abnch\n" +
		"  user: admin\n" +
		"  password: s3cret\n" +
		"  sslmode: require\n" +
		"  timeZone: America/New_York\n"
	if err := writeConfigFile(path, handwritten); err != nil {
		t.Fatal(err)
	}

	if _, err := SetLocalAuth(Options{Port: 9000, RepoRoot: repo, HomeDir: home}); err != nil {
		t.Fatal(err)
	}

	cfg := readConfigFile(t, path)
	want := Postgres{Host: "db.example", Port: 5433, DBName: "abnch", User: "admin", Password: "s3cret", SSLMode: "require", TimeZone: "America/New_York"}
	if !reflect.DeepEqual(cfg.Postgres, want) {
		t.Errorf("postgres = %#v, want %#v", cfg.Postgres, want)
	}
	if cfg.URL != "http://localhost:9000" {
		t.Errorf("url = %q, want the command's url", cfg.URL)
	}
}

func TestSetLocalAuthRerunKeepsOneDevIssuer(t *testing.T) {
	repo := t.TempDir()
	home := t.TempDir()

	for i := 0; i < 2; i++ {
		if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home}); err != nil {
			t.Fatal(err)
		}
	}

	cfg := readConfigFile(t, ConfigPath(repo))
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

	if _, err := SetLocalAuth(Options{Port: 8080, RepoRoot: repo, HomeDir: home}); err != nil {
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
	cred := LocalCredential("http://localhost:8080", "/repo/.abnchd/dev-issuer.key")
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
	if cred.KeyFile != "/repo/.abnchd/dev-issuer.key" {
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

func TestSetLocalAuthWritesCLIConfigURL(t *testing.T) {
	repo := t.TempDir()

	if _, err := SetLocalAuth(Options{Port: 9090, RepoRoot: repo, HomeDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLDoc(t, CLIConfigPath(repo))
	if doc["url"] != "http://localhost:9090" {
		t.Errorf("CLI config url = %#v, want http://localhost:9090", doc["url"])
	}
}

// TestSetLocalAuthPreservesCLIConfigKeys keeps the models and tasks a repo
// writes into its CLI config when the command updates the service URL.
func TestSetLocalAuthPreservesCLIConfigKeys(t *testing.T) {
	repo := t.TempDir()
	path := CLIConfigPath(repo)
	seed := "url: http://localhost:1111\nmodels:\n  stub:\n    name: gpt\n"
	if err := writeConfigFile(path, seed); err != nil {
		t.Fatal(err)
	}

	if _, err := SetLocalAuth(Options{Port: 9090, RepoRoot: repo, HomeDir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLDoc(t, path)
	if doc["url"] != "http://localhost:9090" {
		t.Errorf("url = %#v, want the command's url", doc["url"])
	}
	models, ok := doc["models"].(map[string]any)
	if !ok {
		t.Fatalf("models = %#v, want the seeded mapping", doc["models"])
	}
	if _, ok := models["stub"]; !ok {
		t.Errorf("models = %#v, want the seeded stub", models)
	}
}

// readYAMLDoc parses a YAML file into a map, so a test reads keys the typed
// config does not model.
func readYAMLDoc(t *testing.T, path string) map[string]any {
	t.Helper()
	doc := map[string]any{}
	if err := yaml.Unmarshal([]byte(readFile(t, path)), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func readConfigFile(t *testing.T, path string) Config {
	t.Helper()
	cfg, err := UnmarshalConfig([]byte(readFile(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// writeConfigFile writes content to a local service config path, creating the
// directory that holds it.
func writeConfigFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func readCredentialsFile(t *testing.T, path string) map[string]Credential {
	t.Helper()
	creds, err := UnmarshalCredentials([]byte(readFile(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	return creds
}
