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
	URL    string                 `yaml:"url"`
	Models map[string]configModel `yaml:"models"`
	Tasks  map[string]configTask  `yaml:"tasks"`
}

// configTask is a task entry in the CLI config.
type configTask struct {
	Model  string        `yaml:"model"`
	Agent  *configAgent  `yaml:"agent"`
	Grader *configGrader `yaml:"grader"`
}

// loadConfigTasks reads the tasks in the CLI config under workDir. A missing
// config holds no tasks.
func loadConfigTasks(workDir string) (map[string]configTask, error) {
	cfg, err := loadConfig(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configPath(workDir), err)
	}
	return cfg.Tasks, nil
}

// loadConfigModels reads the models in the CLI config under workDir. A missing
// config holds no models.
func loadConfigModels(workDir string) (map[string]configModel, error) {
	cfg, err := loadConfig(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configPath(workDir), err)
	}
	return cfg.Models, nil
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
		return errors.New("no service url: pass --url")
	}
	fmt.Fprintln(d.out, "Set config")
	return nil
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

// setTaskModel records the task's default model in the CLI config under workDir.
// It edits the existing YAML document, so every other key survives. A missing
// config is created.
func setTaskModel(workDir, stub, model string) error {
	return editConfig(workDir, func(data []byte) ([]byte, error) {
		return updateTaskModel(data, stub, model)
	})
}

// setConfigModel records a model under stub in the CLI config under workDir. It
// edits the existing YAML document, so every other key survives. A missing
// config is created.
func setConfigModel(workDir, stub string, model configModel) error {
	return editConfig(workDir, func(data []byte) ([]byte, error) {
		return updateConfigModel(data, stub, model)
	})
}

// setTaskAgent records the task's agent in the CLI config under workDir. It
// edits the existing YAML document, so every other key survives. A missing
// config is created. It fails when the merged agent has no run command.
func setTaskAgent(workDir, stub string, agent configAgent) error {
	return editConfig(workDir, func(data []byte) ([]byte, error) {
		return updateTaskAgent(data, stub, agent)
	})
}

// setTaskGrader records the task's grader in the CLI config under workDir. It
// edits the existing YAML document, so every other key survives. A missing
// config is created. It fails when the merged grader has no run command.
func setTaskGrader(workDir, stub string, grader configGrader) error {
	return editConfig(workDir, func(data []byte) ([]byte, error) {
		return updateTaskGrader(data, stub, grader)
	})
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

// updateConfigModel returns the config document with models.<stub> merged with
// model, preserving every other key and comment. The provider and name are
// always set; the variant is set only when given.
func updateConfigModel(data []byte, stub string, model configModel) ([]byte, error) {
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
	models, err := childMapping(root, "models")
	if err != nil {
		return nil, err
	}
	entry, err := childMapping(models, stub)
	if err != nil {
		return nil, err
	}
	setMappingValue(entry, "provider", scalarNode(model.Provider))
	setMappingValue(entry, "name", scalarNode(model.Name))
	if model.Variant != "" {
		setMappingValue(entry, "variant", scalarNode(model.Variant))
	}
	return marshalNode(&doc)
}

// updateTaskAgent returns the config document with tasks.<stub>.agent merged
// with agent, preserving every other key and comment. Only the commands the
// update carries are set. It fails when the merged agent has no run command, so
// no agent is written.
func updateTaskAgent(data []byte, stub string, agent configAgent) ([]byte, error) {
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
	entry, err := childMapping(task, "agent")
	if err != nil {
		return nil, err
	}

	if mergeAgent(agentOf(entry), agent).RunCmd == "" {
		return nil, fmt.Errorf("agent for task %s has no run command", stub)
	}
	if agent.VersionCmd != "" {
		setMappingValue(entry, "versionCmd", scalarNode(agent.VersionCmd))
	}
	if agent.BuildCmd != "" {
		setMappingValue(entry, "buildCmd", scalarNode(agent.BuildCmd))
	}
	if agent.RunCmd != "" {
		setMappingValue(entry, "runCmd", scalarNode(agent.RunCmd))
	}
	return marshalNode(&doc)
}

// updateTaskGrader returns the config document with tasks.<stub>.grader merged
// with grader, preserving every other key and comment. Only the commands the
// update carries are set. It fails when the merged grader has no run command, so
// no grader is written.
func updateTaskGrader(data []byte, stub string, grader configGrader) ([]byte, error) {
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
	entry, err := childMapping(task, "grader")
	if err != nil {
		return nil, err
	}

	if mergeGrader(graderOf(entry), grader).RunCmd == "" {
		return nil, fmt.Errorf("grader for task %s has no run command", stub)
	}
	if grader.VersionCmd != "" {
		setMappingValue(entry, "versionCmd", scalarNode(grader.VersionCmd))
	}
	if grader.BuildCmd != "" {
		setMappingValue(entry, "buildCmd", scalarNode(grader.BuildCmd))
	}
	if grader.RunCmd != "" {
		setMappingValue(entry, "runCmd", scalarNode(grader.RunCmd))
	}
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

// mappingString returns the string value for key in a mapping, or "" when the
// key is missing or not a scalar.
func mappingString(mapping *yaml.Node, key string) string {
	value := mappingValue(mapping, key)
	if value == nil || value.Kind != yaml.ScalarNode {
		return ""
	}
	return value.Value
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
