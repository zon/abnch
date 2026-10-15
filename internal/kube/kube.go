// Package kube applies Kubernetes resources for local development tooling
// through the kubectl CLI.
package kube

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os/exec"

	"go.yaml.in/yaml/v3"
)

// Client applies ConfigMaps and Secrets to a Kubernetes cluster through kubectl.
type Client struct{}

// ApplyConfigMap creates or updates a ConfigMap in a namespace.
func (Client) ApplyConfigMap(kubeContext, namespace, name string, data map[string]string) error {
	manifest, err := configMapManifest(namespace, name, data)
	if err != nil {
		return err
	}
	return apply(kubeContext, manifest)
}

// ApplySecret creates or updates an opaque Secret in a namespace.
func (Client) ApplySecret(kubeContext, namespace, name string, data map[string]string) error {
	manifest, err := secretManifest(namespace, name, data)
	if err != nil {
		return err
	}
	return apply(kubeContext, manifest)
}

// configMapManifest renders a ConfigMap holding the data under each key.
func configMapManifest(namespace, name string, data map[string]string) ([]byte, error) {
	manifest, err := yaml.Marshal(configMap{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata:   metadata{Name: name, Namespace: namespace},
		Data:       data,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render ConfigMap %s: %w", name, err)
	}
	return manifest, nil
}

// secretManifest renders an opaque Secret, base64-encoding each value as the API
// expects.
func secretManifest(namespace, name string, data map[string]string) ([]byte, error) {
	encoded := make(map[string]string, len(data))
	for key, value := range data {
		encoded[key] = base64.StdEncoding.EncodeToString([]byte(value))
	}

	manifest, err := yaml.Marshal(secret{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata:   metadata{Name: name, Namespace: namespace},
		Type:       "Opaque",
		Data:       encoded,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render Secret %s: %w", name, err)
	}
	return manifest, nil
}

// metadata is the subset of object metadata the manifests set.
type metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// configMap is the Kubernetes ConfigMap manifest the client renders.
type configMap struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   metadata          `yaml:"metadata"`
	Data       map[string]string `yaml:"data"`
}

// secret is the Kubernetes Secret manifest the client renders.
type secret struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   metadata          `yaml:"metadata"`
	Type       string            `yaml:"type"`
	Data       map[string]string `yaml:"data"`
}

// apply pipes a manifest to `kubectl apply`, so a resource is created or
// updated in place.
func apply(kubeContext string, manifest []byte) error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found in PATH: %w", err)
	}

	args := []string{"apply", "-f", "-"}
	if kubeContext != "" {
		args = append(args, "--context", kubeContext)
	}

	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = bytes.NewReader(manifest)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply failed: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}
