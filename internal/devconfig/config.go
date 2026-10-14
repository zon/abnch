package devconfig

import (
	"gopkg.in/yaml.v3"
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

// Config is the local service configuration.
type Config struct {
	URL      string   `yaml:"url"`
	Postgres Postgres `yaml:"postgres"`
	OAuth    OAuth    `yaml:"oauth"`
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
	return yaml.Marshal(c)
}

// UnmarshalConfig parses a config from YAML.
func UnmarshalConfig(data []byte) (Config, error) {
	var c Config
	err := yaml.Unmarshal(data, &c)
	return c, err
}
