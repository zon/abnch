package devconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const (
	// ConfigFileName is the local service config file in the repo root.
	ConfigFileName = "service-config.local.yaml"
	// KeyFileName is the dev signing key in the repo root.
	KeyFileName     = "dev-issuer.key"
	gitignoreName   = ".gitignore"
	credentialsDir  = ".abnch"
	credentialsName = "credentials.yaml"
)

// Options configures a local auth setup.
type Options struct {
	Port     int
	RepoRoot string
	HomeDir  string
	In       io.Reader
	Out      io.Writer
}

// SetLocalAuth collects the local service values, then writes the local service
// config, the dev key, and the local CLI credential. It returns the config file
// path.
func SetLocalAuth(opts Options) (string, error) {
	url := LocalServiceURL(opts.Port)
	configPath := ConfigPath(opts.RepoRoot)

	existing, err := readConfig(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	pg, err := askPostgres(NewPrompter(opts.In, opts.Out), existing.Postgres)
	if err != nil {
		return "", err
	}

	keyPath := filepath.Join(opts.RepoRoot, KeyFileName)
	if err := EnsureDevKey(keyPath); err != nil {
		return "", fmt.Errorf("failed to write dev key: %w", err)
	}

	cfg := existing
	cfg.URL = url
	cfg.Postgres = pg
	cfg.OAuth.Issuers = UpsertIssuer(cfg.OAuth.Issuers, DevIssuer(url))
	if err := writeConfig(configPath, cfg); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	if err := writeCredential(opts.HomeDir, url, keyPath); err != nil {
		return "", fmt.Errorf("failed to write local credential: %w", err)
	}

	if err := EnsureGitignore(filepath.Join(opts.RepoRoot, gitignoreName), ConfigFileName, KeyFileName); err != nil {
		return "", fmt.Errorf("failed to update %s: %w", gitignoreName, err)
	}

	return configPath, nil
}

// LocalServiceURL builds the local service URL for a port.
func LocalServiceURL(port int) string {
	return fmt.Sprintf("http://localhost:%d", port)
}

// ConfigPath returns the local service config path in a repo root.
func ConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, ConfigFileName)
}

func askPostgres(p *Prompter, def Postgres) (Postgres, error) {
	var pg Postgres

	host, err := p.Ask("host", def.Host, false)
	if err != nil {
		return Postgres{}, err
	}
	pg.Host = host

	port, err := p.Ask("port", intDefault(def.Port), false)
	if err != nil {
		return Postgres{}, err
	}
	pg.Port, err = strconv.Atoi(port)
	if err != nil {
		return Postgres{}, fmt.Errorf("postgres port %q is not a number", port)
	}

	if pg.DBName, err = p.Ask("dbname", def.DBName, false); err != nil {
		return Postgres{}, err
	}
	if pg.User, err = p.Ask("user", def.User, false); err != nil {
		return Postgres{}, err
	}
	if pg.Password, err = p.Ask("password", def.Password, true); err != nil {
		return Postgres{}, err
	}
	if pg.SSLMode, err = p.Ask("sslmode", stringDefault(def.SSLMode, "disable"), false); err != nil {
		return Postgres{}, err
	}
	if pg.TimeZone, err = p.Ask("timeZone", stringDefault(def.TimeZone, "UTC"), false); err != nil {
		return Postgres{}, err
	}

	return pg, nil
}

func intDefault(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

func stringDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func readConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	return UnmarshalConfig(data)
}

func writeConfig(path string, cfg Config) error {
	data, err := cfg.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func writeCredential(homeDir, url, keyPath string) error {
	dir := filepath.Join(homeDir, credentialsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	path := filepath.Join(dir, credentialsName)
	creds, err := readCredentials(path)
	if err != nil {
		return err
	}

	creds = UpsertCredential(creds, url, LocalCredential(url, keyPath))

	data, err := MarshalCredentials(creds)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readCredentials(path string) (map[string]Credential, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]Credential{}, nil
	}
	if err != nil {
		return nil, err
	}
	return UnmarshalCredentials(data)
}
