package devconfig

import "gopkg.in/yaml.v3"

// Credential lets the CLI authenticate to one service URL.
type Credential struct {
	Issuer   string `yaml:"issuer"`
	Audience string `yaml:"audience"`
	Subject  string `yaml:"subject"`
	KeyFile  string `yaml:"keyFile"`
}

// LocalCredential builds the CLI credential for a local service URL from the
// dev issuer the service trusts, so the CLI authenticates as the issuer's
// local-cli subject without signing in.
func LocalCredential(url, keyPath string) Credential {
	iss := DevIssuer(url)
	return Credential{
		Issuer:   iss.Issuer,
		Audience: iss.Audience,
		Subject:  LocalCLISubject,
		KeyFile:  keyPath,
	}
}

// UpsertCredential sets the credential for url and leaves other URLs alone.
func UpsertCredential(creds map[string]Credential, url string, cred Credential) map[string]Credential {
	if creds == nil {
		creds = map[string]Credential{}
	}
	creds[url] = cred
	return creds
}

// MarshalCredentials renders the credential store as YAML.
func MarshalCredentials(creds map[string]Credential) ([]byte, error) {
	return yaml.Marshal(creds)
}

// UnmarshalCredentials parses a credential store from YAML.
func UnmarshalCredentials(data []byte) (map[string]Credential, error) {
	creds := map[string]Credential{}
	err := yaml.Unmarshal(data, &creds)
	return creds, err
}
