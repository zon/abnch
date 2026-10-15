package devconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	// RalphConfigDir is the directory in the repo root that holds Ralph's config.
	RalphConfigDir = ".ralph"
	// RalphConfigFileName is the Ralph config file name.
	RalphConfigFileName = "config.yaml"

	// RalphSecretName is the Secret published for remote runs. It holds the
	// local service config and the dev signing key.
	RalphSecretName = "abnchd-config"
	// RalphConfigMapName is the ConfigMap published for remote runs. It holds
	// the CLI config.
	RalphConfigMapName = "abnch-config"
)

// ResourceApplier applies a ConfigMap or Secret to a Kubernetes cluster.
type ResourceApplier interface {
	ApplyConfigMap(kubeContext, namespace, name string, data map[string]string) error
	ApplySecret(kubeContext, namespace, name string, data map[string]string) error
}

// RalphOptions configures publishing the local dev config for Ralph remote runs.
type RalphOptions struct {
	RepoRoot   string
	ConfigPath string // local service config; the dev key sits beside it
	Context    string // overrides workflow.context in .ralph/config.yaml
	Namespace  string // overrides workflow.namespace in .ralph/config.yaml
	Applier    ResourceApplier
}

// RalphConfigPath returns the Ralph config path in a repo root.
func RalphConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, RalphConfigDir, RalphConfigFileName)
}

// SetRalph publishes the local service config, its dev key, and the CLI config
// to the Ralph namespace as a Secret and a ConfigMap, so a remote workflow that
// mounts them reads the same config a local service uses. The repo's
// .ralph/config.yaml declares those mounts. It returns the namespace it
// published to.
func SetRalph(opts RalphOptions) (string, error) {
	data, err := os.ReadFile(RalphConfigPath(opts.RepoRoot))
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", RalphConfigPath(opts.RepoRoot), err)
	}

	target, err := resolveRalphTarget(data, opts)
	if err != nil {
		return "", err
	}

	secret, configMap, err := ralphData(opts)
	if err != nil {
		return "", err
	}

	if err := opts.Applier.ApplySecret(target.context, target.namespace, RalphSecretName, secret); err != nil {
		return "", fmt.Errorf("failed to publish %s: %w", RalphSecretName, err)
	}
	if err := opts.Applier.ApplyConfigMap(target.context, target.namespace, RalphConfigMapName, configMap); err != nil {
		return "", fmt.Errorf("failed to publish %s: %w", RalphConfigMapName, err)
	}

	return target.namespace, nil
}

// ralphTarget is the cluster and namespace a Ralph remote run uses.
type ralphTarget struct {
	context   string
	namespace string
}

// resolveRalphTarget resolves the cluster and namespace from the options and the
// Ralph config, with the options taking precedence. A namespace is required.
func resolveRalphTarget(data []byte, opts RalphOptions) (ralphTarget, error) {
	var cfg struct {
		Workflow struct {
			Context   string `yaml:"context"`
			Namespace string `yaml:"namespace"`
		} `yaml:"workflow"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ralphTarget{}, fmt.Errorf("failed to parse %s: %w", RalphConfigPath(opts.RepoRoot), err)
	}

	target := ralphTarget{context: cfg.Workflow.Context, namespace: cfg.Workflow.Namespace}
	if opts.Context != "" {
		target.context = opts.Context
	}
	if opts.Namespace != "" {
		target.namespace = opts.Namespace
	}
	if target.namespace == "" {
		return ralphTarget{}, fmt.Errorf("%s does not set workflow.namespace; pass --namespace", RalphConfigPath(opts.RepoRoot))
	}
	return target, nil
}

// ralphData reads the files published for remote runs: the local service config
// and its dev key as the Secret data, and the CLI config as the ConfigMap data.
// The keys are the file names, so a mount at the config's directory restores the
// files under their original names.
func ralphData(opts RalphOptions) (secret, configMap map[string]string, err error) {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = ConfigPath(opts.RepoRoot)
	}

	serviceConfig, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", configPath, err)
	}
	keyPath := KeyPath(configPath)
	devKey, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", keyPath, err)
	}
	cliConfigPath := CLIConfigPath(opts.RepoRoot)
	cliConfig, err := os.ReadFile(cliConfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", cliConfigPath, err)
	}

	secret = map[string]string{
		ConfigFileName: string(serviceConfig),
		KeyFileName:    string(devKey),
	}
	configMap = map[string]string{
		CLIConfigFileName: string(cliConfig),
	}
	return secret, configMap, nil
}
