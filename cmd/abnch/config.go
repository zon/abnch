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
// directory.
type cliConfig struct {
	URL string `yaml:"url"`
}

// configPath returns the CLI config path under workDir.
func configPath(workDir string) string {
	return filepath.Join(workDir, configDir, configName)
}

// setTaskModel records the task's default model in the CLI config under workDir.
// It edits the existing YAML document, so every other key survives. A missing
// config is created.
func setTaskModel(workDir, stub, model string) error {
	data, err := os.ReadFile(configPath(workDir))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", configPath(workDir), err)
	}

	updated, err := updateTaskModel(data, stub, model)
	if err != nil {
		return fmt.Errorf("failed to update %s: %w", configPath(workDir), err)
	}

	if err := os.MkdirAll(filepath.Join(workDir, configDir), 0o700); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Join(workDir, configDir), err)
	}
	return os.WriteFile(configPath(workDir), updated, 0o600)
}

// updateTaskModel returns the config document with tasks.<stub>.model set to
// model, preserving every other key and comment.
func updateTaskModel(data []byte, stub, model string) ([]byte, error) {
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
	tasks, err := childMapping(root, "tasks")
	if err != nil {
		return nil, err
	}
	task, err := childMapping(tasks, stub)
	if err != nil {
		return nil, err
	}
	setMappingValue(task, "model", scalarNode(model))
	return marshalNode(&doc)
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

// childMapping returns the child mapping for key, creating it when the key is
// missing or null.
func childMapping(parent *yaml.Node, key string) (*yaml.Node, error) {
	child := mappingValue(parent, key)
	if child == nil || isNull(child) {
		child = &yaml.Node{Kind: yaml.MappingNode}
		setMappingValue(parent, key, child)
		return child, nil
	}
	if child.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s is not a mapping", key)
	}
	return child, nil
}

// mappingValue returns the value node for key in a mapping, or nil.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
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
		return "", errors.New("no service url: pass --url or set url in " + configDir + "/" + configName)
	}
	return cfg.URL, nil
}
