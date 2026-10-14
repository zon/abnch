package devconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func completeConfig() Config {
	return Config{
		URL:      "http://localhost:8080",
		Postgres: Postgres{Host: "localhost", Port: 5432, DBName: "abnch", User: "postgres", Password: "pw", SSLMode: "disable", TimeZone: "UTC"},
		OAuth:    OAuth{Issuers: []Issuer{DevIssuer("http://localhost:8080")}},
	}
}

func TestCheckAcceptsCompleteConfig(t *testing.T) {
	if err := Check(completeConfig()); err != nil {
		t.Errorf("Check = %v, want nil", err)
	}
}

func TestCheckRejectsMissingAuthEntry(t *testing.T) {
	cfg := completeConfig()
	cfg.OAuth.Issuers = nil
	err := Check(cfg)
	if err == nil {
		t.Fatal("Check = nil, want an error for a config with no auth entry")
	}
	if !strings.Contains(err.Error(), "auth entry") {
		t.Errorf("error = %q, want it to name the auth entry", err)
	}
}

func TestCheckRejectsIncompletePostgres(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*Postgres)
		field string
	}{
		{"host", func(p *Postgres) { p.Host = "" }, "host"},
		{"port", func(p *Postgres) { p.Port = 0 }, "port"},
		{"dbname", func(p *Postgres) { p.DBName = "" }, "dbname"},
		{"user", func(p *Postgres) { p.User = "" }, "user"},
		{"password", func(p *Postgres) { p.Password = "" }, "password"},
		{"sslmode", func(p *Postgres) { p.SSLMode = "" }, "sslmode"},
		{"timeZone", func(p *Postgres) { p.TimeZone = "" }, "timeZone"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := completeConfig()
			tc.mut(&cfg.Postgres)
			err := Check(cfg)
			if err == nil {
				t.Fatalf("Check = nil, want an error for a missing %s", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error = %q, want it to name %q", err, tc.field)
			}
		})
	}
}

func TestLoadConfigMissingFileIsClearError(t *testing.T) {
	path := filepath.Join(t.TempDir(), ConfigFileName)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig = nil, want an error for a missing file")
	}
	if !strings.Contains(err.Error(), ConfigFileName) {
		t.Errorf("error = %q, want it to name the missing file", err)
	}
}

func TestLoadConfigReadsWrittenConfig(t *testing.T) {
	repo := t.TempDir()
	seed := "postgres:\n" +
		"  host: db\n" +
		"  port: 5432\n" +
		"  dbname: abnch\n" +
		"  user: admin\n" +
		"  password: pw\n" +
		"  sslmode: disable\n" +
		"  timeZone: UTC\n"
	if err := os.WriteFile(ConfigPath(repo), []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := SetLocalAuth(Options{
		Port:     8080,
		RepoRoot: repo,
		HomeDir:  t.TempDir(),
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(ConfigPath(repo))
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(cfg); err != nil {
		t.Errorf("Check on the written config = %v, want nil", err)
	}
	if _, err := os.Stat(ConfigPath(repo)); err != nil {
		t.Errorf("config file missing: %v", err)
	}
}
