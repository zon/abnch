package devconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// ConfigDir is the directory in the repo root that holds the local service
	// config and its dev signing key.
	ConfigDir = ".abnchd"
	// ConfigFileName is the local service config file name.
	ConfigFileName = "config.yaml"
	// KeyFileName is the dev signing key, written beside the local service
	// config.
	KeyFileName     = "dev-issuer.key"
	gitignoreName   = ".gitignore"
	credentialsDir  = ".abnch"
	credentialsName = "credentials.yaml"
)

// ConfigGitignore is the .gitignore entry for the config directory, which holds
// both the local service config and the dev signing key.
const ConfigGitignore = ConfigDir + "/"

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

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", filepath.Dir(configPath), err)
	}

	keyPath := KeyPath(configPath)
	if err := EnsureDevKey(keyPath); err != nil {
		return "", fmt.Errorf("failed to write dev key: %w", err)
	}

	if err := os.WriteFile(configPath, updated, 0o600); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	if err := writeCredential(opts.HomeDir, url, keyPath); err != nil {
		return "", fmt.Errorf("failed to write local credential: %w", err)
	}

	if err := EnsureGitignore(filepath.Join(opts.RepoRoot, gitignoreName), ConfigGitignore); err != nil {
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

// KeyPath returns the dev signing key path, which sits beside the local service
// config so the config and the key that signs its issuer move together.
func KeyPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), KeyFileName)
}

func writeCredential(homeDir, url, keyPath string) error {
	creds, err := LoadCredentials(homeDir)
	if err != nil {
		return err
	}
	creds = UpsertCredential(creds, url, LocalCredential(url, keyPath))
	return saveCredentials(homeDir, creds)
}
