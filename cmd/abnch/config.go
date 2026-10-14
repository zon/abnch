package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// configDir and configName locate the CLI config under the working directory.
const (
	configDir  = ".abnch"
	configName = "config.yaml"
)

// cliConfig is the CLI config in `.abnch/config.yaml` under the working
// directory.
type cliConfig struct {
	URL string `yaml:"url"`
}

// configPath returns the CLI config path under workDir.
func configPath(workDir string) string {
	return filepath.Join(workDir, configDir, configName)
}

// loadConfig reads the CLI config from workDir. A missing file is an empty
// config.
func loadConfig(workDir string) (cliConfig, error) {
	data, err := os.ReadFile(configPath(workDir))
	if os.IsNotExist(err) {
		return cliConfig{}, nil
	}
	if err != nil {
		return cliConfig{}, err
	}
	var cfg cliConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cliConfig{}, err
	}
	return cfg, nil
}

// resolveServiceURL takes the service URL from the flag when given, and from
// the CLI config otherwise. It fails when neither provides one.
func resolveServiceURL(flagURL, workDir string) (string, error) {
	if flagURL != "" {
		return flagURL, nil
	}

	cfg, err := loadConfig(workDir)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", configPath(workDir), err)
	}
	if cfg.URL == "" {
		return "", errors.New("no service url: pass --url or set url in " + configDir + "/" + configName)
	}
	return cfg.URL, nil
}
