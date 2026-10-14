package devconfig

import (
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// Credential lets the CLI authenticate to one service URL. A signed-in human
// carries a Session token. A local dev workload carries the dev issuer, the
// subject it authenticates as, and the key that signs its token.
type Credential struct {
	Session  string `yaml:"session,omitempty"`
	Issuer   string `yaml:"issuer,omitempty"`
	Audience string `yaml:"audience,omitempty"`
	Subject  string `yaml:"subject,omitempty"`
	KeyFile  string `yaml:"keyFile,omitempty"`
}

// CredentialsPath returns the credential store path under homeDir.
func CredentialsPath(homeDir string) string {
	return filepath.Join(homeDir, credentialsDir, credentialsName)
}

// LoadCredentials reads the credential store under homeDir. A missing store is
// empty.
func LoadCredentials(homeDir string) (map[string]Credential, error) {
	return readCredentials(CredentialsPath(homeDir))
}

// StoreSession sets the session for url in the credential store under homeDir,
// keeping every other URL and the URL's other credential fields. A later login
// replaces the session.
func StoreSession(homeDir, url, session string) error {
	creds, err := LoadCredentials(homeDir)
	if err != nil {
		return err
	}
	cred := creds[url]
	cred.Session = session
	return saveCredentials(homeDir, UpsertCredential(creds, url, cred))
}

// RemoveSession clears the stored session for url in the credential store under
// homeDir, keeping every other URL and the URL's other credential fields. A URL
// with no stored session changes nothing.
func RemoveSession(homeDir, url string) error {
	creds, err := LoadCredentials(homeDir)
	if err != nil {
		return err
	}
	cred, ok := creds[url]
	if !ok || cred.Session == "" {
		return nil
	}
	cred.Session = ""
	if cred == (Credential{}) {
		delete(creds, url)
	} else {
		creds[url] = cred
	}
	return saveCredentials(homeDir, creds)
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
	return marshalYAML(creds)
}

// UnmarshalCredentials parses a credential store from YAML.
func UnmarshalCredentials(data []byte) (map[string]Credential, error) {
	creds := map[string]Credential{}
	err := yaml.Unmarshal(data, &creds)
	return creds, err
}

// saveCredentials writes the credential store under homeDir, creating its
// directory with owner-only permissions.
func saveCredentials(homeDir string, creds map[string]Credential) error {
	dir := filepath.Join(homeDir, credentialsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := MarshalCredentials(creds)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, credentialsName), data, 0o600)
}

func readCredentials(path string) (map[string]Credential, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]Credential{}, nil
	}
	if err != nil {
		return nil, err
	}
	return UnmarshalCredentials(data)
}
