package devconfig

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// Postgres holds the local Postgres connection details.
type Postgres struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	DBName   string `yaml:"dbname"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslmode"`
	TimeZone string `yaml:"timeZone"`
}

// Issuer is a workload token issuer the service trusts.
type Issuer struct {
	Issuer    string            `yaml:"issuer"`
	Audience  string            `yaml:"audience"`
	RoleClaim string            `yaml:"roleClaim"`
	RoleMap   map[string]string `yaml:"roleMap"`
}

// OAuth holds the trusted workload issuers.
type OAuth struct {
	Issuers []Issuer `yaml:"issuers"`
}

// OIDC is the provider the service signs human users in through.
type OIDC struct {
	Issuer       string            `yaml:"issuer"`
	ClientID     string            `yaml:"clientId"`
	ClientSecret string            `yaml:"clientSecret"`
	RedirectURL  string            `yaml:"redirectUrl"`
	Scopes       []string          `yaml:"scopes"`
	RoleClaim    string            `yaml:"roleClaim"`
	RoleMap      map[string]string `yaml:"roleMap"`
}

// TLS names the certificate and key the service serves.
type TLS struct {
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

// Config is the local service configuration.
type Config struct {
	URL      string   `yaml:"url"`
	Listen   string   `yaml:"listen,omitempty"`
	TLS      *TLS     `yaml:"tls,omitempty"`
	Postgres Postgres `yaml:"postgres"`
	OAuth    OAuth    `yaml:"oauth"`
	OIDC     OIDC     `yaml:"oidc,omitempty"`
}

// LocalCLISubject is the dev issuer subject the local CLI authenticates as.
const LocalCLISubject = "local-cli"

// DevIssuer returns the dev issuer for a local service URL.
func DevIssuer(url string) Issuer {
	return Issuer{
		Issuer:    url + "/dev",
		Audience:  "abnch",
		RoleClaim: "sub",
		RoleMap: map[string]string{
			LocalCLISubject: "Admin",
			"local-runtime": "Service",
		},
	}
}

// UpsertIssuer replaces the issuer with the same URL, or appends it, so a
// rerun leaves one entry per issuer URL.
func UpsertIssuer(issuers []Issuer, iss Issuer) []Issuer {
	for i := range issuers {
		if issuers[i].Issuer == iss.Issuer {
			issuers[i] = iss
			return issuers
		}
	}
	return append(issuers, iss)
}

// Marshal renders the config as YAML.
func (c Config) Marshal() ([]byte, error) {
	return marshalYAML(c)
}

// UnmarshalConfig parses a config from YAML.
func UnmarshalConfig(data []byte) (Config, error) {
	var c Config
	err := yaml.Unmarshal(data, &c)
	return c, err
}

// UpdateAuth returns the local service config with its url set to url and the
// dev issuer for url upserted into oauth.issuers. It edits the existing YAML
// document, so every other key survives, including keys the typed Config does
// not model and comments on the kept lines.
func UpdateAuth(data []byte, url string) ([]byte, error) {
	var doc yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
	}

	root, err := configMapping(&doc)
	if err != nil {
		return nil, err
	}

	setMappingValue(root, "url", scalarNode(url))

	oauth, err := childMapping(root, "oauth")
	if err != nil {
		return nil, err
	}
	issuers, err := childSequence(oauth, "issuers")
	if err != nil {
		return nil, err
	}

	dev := DevIssuer(url)
	var devNode yaml.Node
	if err := devNode.Encode(dev); err != nil {
		return nil, err
	}
	for i, item := range issuers.Content {
		if issuerURL(item) == dev.Issuer {
			issuers.Content[i] = &devNode
			return marshalYAML(&doc)
		}
	}
	issuers.Content = append(issuers.Content, &devNode)
	return marshalYAML(&doc)
}

// UpdatePostgres returns the service config with its postgres host and port
// set. It edits the existing YAML document, so every other key survives,
// including keys the typed Config does not model and comments on the kept
// lines.
func UpdatePostgres(data []byte, host string, port int) ([]byte, error) {
	var doc yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
	}

	root, err := configMapping(&doc)
	if err != nil {
		return nil, err
	}
	pg, err := childMapping(root, "postgres")
	if err != nil {
		return nil, err
	}

	var portNode yaml.Node
	if err := portNode.Encode(port); err != nil {
		return nil, err
	}
	setMappingValue(pg, "host", scalarNode(host))
	setMappingValue(pg, "port", &portNode)
	return marshalYAML(&doc)
}

// UpdateCLIConfigURL returns the CLI config with its url set. It edits the
// existing YAML document, so every other key survives, including keys the CLI
// config does not model and comments on the kept lines.
func UpdateCLIConfigURL(data []byte, url string) ([]byte, error) {
	var doc yaml.Node
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
	}

	root, err := configMapping(&doc)
	if err != nil {
		return nil, err
	}

	setMappingValue(root, "url", scalarNode(url))
	return marshalYAML(&doc)
}

// configMapping returns the document's root mapping, creating it when the
// document is empty or null.
func configMapping(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		return doc.Content[0], nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("config is not a mapping")
	}
	root := doc.Content[0]
	if isNull(root) {
		root.Kind = yaml.MappingNode
		root.Tag = ""
		root.Value = ""
		return root, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config is not a mapping")
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

// childSequence returns the child sequence for key, creating it when the key is
// missing or null.
func childSequence(parent *yaml.Node, key string) (*yaml.Node, error) {
	child := mappingValue(parent, key)
	if child == nil || isNull(child) {
		child = &yaml.Node{Kind: yaml.SequenceNode}
		setMappingValue(parent, key, child)
		return child, nil
	}
	if child.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s is not a list", key)
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

// issuerURL returns the issuer URL of an oauth issuer entry, or "" when the
// entry is not a mapping with a scalar issuer.
func issuerURL(item *yaml.Node) string {
	if item.Kind != yaml.MappingNode {
		return ""
	}
	if v := mappingValue(item, "issuer"); v != nil && v.Kind == yaml.ScalarNode {
		return v.Value
	}
	return ""
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}
