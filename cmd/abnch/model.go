package main

import (
	"fmt"
	"strings"
)

// configModel is a model entry in the CLI config.
type configModel struct {
	Provider string `yaml:"provider"`
	Name     string `yaml:"name"`
	Variant  string `yaml:"variant"`
}

// setModel records a model in the CLI config under the working directory. It
// takes the provider and name from the argument, the stub from --stub or the
// name, and stores the variant only when --variant is given.
func setModel(d deps, c setModelCmd) error {
	provider, name, err := splitModelSpec(c.Path)
	if err != nil {
		return err
	}

	stub := c.Stub
	if stub == "" {
		stub = name
	}

	model := configModel{Provider: provider, Name: name, Variant: c.Variant}
	if err := setConfigModel(d.workDir, stub, model); err != nil {
		return err
	}
	fmt.Fprintf(d.out, "Set model %s\n", stub)
	return nil
}

// splitModelSpec splits a `<provider>/<name>` argument into its provider and
// name. It fails when the provider or the name is missing.
func splitModelSpec(spec string) (provider, name string, err error) {
	provider, name, ok := strings.Cut(spec, "/")
	if !ok || provider == "" {
		return "", "", fmt.Errorf("model %q has no provider; use <provider>/<name>", spec)
	}
	if name == "" {
		return "", "", fmt.Errorf("model %q has no name; use <provider>/<name>", spec)
	}
	return provider, name, nil
}
