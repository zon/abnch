package main

import (
	"bytes"
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
// directory. It holds the service URL; result and score submission carry the
// model, agent, and grader records they use.
type cliConfig struct {
	URL string `yaml:"url"`
}

// configPath returns the CLI config path under workDir.
func configPath(workDir string) string {
	return filepath.Join(workDir, configDir, configName)
}

// getConfig writes the CLI config under the working directory as YAML. A
// missing config writes nothing.
func getConfig(d deps) error {
	path := configPath(d.workDir)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	cfg, err := loadConfig(d.workDir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	return writeYAML(d.out, cfg)
}

// setConfig creates or updates the CLI config's service URL. It keeps the
// existing URL when --url is not given and fails when neither provides one.
func setConfig(d deps, c setConfigCmd) error {
	if c.URL == "" {
		return keepConfigURL(d)
	}
	if err := setConfigURL(d.workDir, c.URL); err != nil {
		return err
	}
	fmt.Fprintln(d.out, "Set config")
	return nil
}

// keepConfigURL succeeds when the CLI config already has a service URL and
// fails when it does not. It leaves the config untouched.
func keepConfigURL(d deps) error {
	cfg, err := loadConfig(d.workDir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath(d.workDir), err)
	}
	if cfg.URL == "" {
		return missingServiceURLError()
	}
	fmt.Fprintln(d.out, "Set config")
	return nil
}

// missingServiceURLError tells the user how to set the service URL.
func missingServiceURLError() error {
	return errors.New("no service url: run `abnch set config --url <url>`")
}

// setConfigURL records the service URL in the CLI config under workDir. It
// edits the existing YAML document, so every other key survives. A missing
// config is created.
func setConfigURL(workDir, url string) error {
	return editConfig(workDir, func(data []byte) ([]byte, error) {
		return updateConfigURL(data, url)
	})
}

// updateConfigURL returns the config document with url set, preserving every
// other key and comment.
func updateConfigURL(data []byte, url string) ([]byte, error) {
	var doc yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
	}

	root, err := rootMapping(&doc)
	if err != nil {
		return nil, err
	}
	setMappingValue(root, "url", scalarNode(url))
	return marshalNode(&doc)
}

// editConfig reads the CLI config under workDir, applies edit to its bytes, and
// writes the result. A missing config is created.
func editConfig(workDir string, edit func([]byte) ([]byte, error)) error {
	path := configPath(workDir)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	updated, err := edit(data)
	if err != nil {
		return fmt.Errorf("failed to update %s: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Join(workDir, configDir), 0o700); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Join(workDir, configDir), err)
	}
	return os.WriteFile(path, updated, 0o600)
}

// rootMapping returns the document's root mapping, creating it when the
// document is empty or null.
func rootMapping(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		return doc.Content[0], nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, errors.New("config is not a mapping")
	}
	root := doc.Content[0]
	if isNull(root) {
		root.Kind = yaml.MappingNode
		root.Tag = ""
		root.Value = ""
		return root, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("config is not a mapping")
	}
	return root, nil
}

// setMappingValue sets key to value, replacing an existing entry in place so the
// key's position and comments stay put.
func setMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	mapping.Content = append(mapping.Content, keyNode, value)
}

// scalarNode returns a string scalar YAML node for value.
func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// isNull reports whether a node is a YAML null.
func isNull(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Tag == "!!null"
}

// marshalNode renders a YAML node document with two-space indents, the repo's
// YAML style.
func marshalNode(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
		return "", missingServiceURLError()
	}
	return cfg.URL, nil
}
