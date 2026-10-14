package devconfig

import (
	"bytes"

	"go.yaml.in/yaml/v3"
)

// marshalYAML renders v with two-space indents, the repo's YAML style.
func marshalYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
