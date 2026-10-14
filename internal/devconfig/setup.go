package devconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// ConfigDir is the directory in the repo root that holds the local service
	// config.
	ConfigDir = ".abnchd"
	// ConfigFileName is the local service config file name.
	ConfigFileName = "config.yaml"
	// ConfigGitignore is the .gitignore entry for the config directory.
	ConfigGitignore = ConfigDir + "/"
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
	// ConfigPath overrides the local service config path. It defaults to
	// ConfigPath(RepoRoot).
	ConfigPath string
}

// SetLocalAuth writes the local auth config, the dev key, and the local CLI
// credential. It edits the existing local service config in place, so the
// Postgres block and any other keys it holds survive. It returns the config
// file path.
func SetLocalAuth(opts Options) (string, error) {
	url := LocalServiceURL(opts.Port)
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = ConfigPath(opts.RepoRoot)
	}

	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	updated, err := UpdateAuth(data, url)
	if err != nil {
		return "", fmt.Errorf("failed to update %s: %w", configPath, err)
	}

	keyPath := filepath.Join(opts.RepoRoot, KeyFileName)
	if err := EnsureDevKey(keyPath); err != nil {
		return "", fmt.Errorf("failed to write dev key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", filepath.Dir(configPath), err)
	}
	if err := os.WriteFile(configPath, updated, 0o600); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	if err := writeCredential(opts.HomeDir, url, keyPath); err != nil {
		return "", fmt.Errorf("failed to write local credential: %w", err)
	}

	if err := EnsureGitignore(filepath.Join(opts.RepoRoot, gitignoreName), ConfigGitignore, KeyFileName); err != nil {
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
	return filepath.Join(repoRoot, ConfigDir, ConfigFileName)
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
