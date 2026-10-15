package devconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

// CLIConfigPath returns the CLI config path in a repo root. The `abnch` CLI
// reads the service URL from this file.
func CLIConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, CLIConfigDir, CLIConfigFileName)
}

// SetCLIConfigURL sets the CLI config url for a repo, creating the file when it
// is missing. It edits the existing YAML document, so models, tasks, and other
// keys survive.
func SetCLIConfigURL(repoRoot, url string) error {
	path := CLIConfigPath(repoRoot)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	updated, err := UpdateCLIConfigURL(data, url)
	if err != nil {
		return fmt.Errorf("failed to update %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(path), err)
	}
	return os.WriteFile(path, updated, 0o600)
}
