package devconfig

import (
	"fmt"
	"os"
	"strings"
)

// LoadConfig reads a local service config, reporting a clear error when the
// file is missing or malformed.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, fmt.Errorf("config file %s is missing", path)
	}
	if err != nil {
		return Config{}, fmt.Errorf("failed to read %s: %w", path, err)
	}
	cfg, err := UnmarshalConfig(data)
	if err != nil {
		return Config{}, fmt.Errorf("config file %s is not valid: %w", path, err)
	}
	return cfg, nil
}

// Check confirms a local service config is ready: complete Postgres connection
// details and at least one auth entry.
func Check(cfg Config) error {
	if err := CheckPostgres(cfg.Postgres); err != nil {
		return err
	}
	if len(cfg.OAuth.Issuers) == 0 && cfg.OIDC.Issuer == "" {
		return fmt.Errorf("no auth entry: the config needs an oidc or oauth entry")
	}
	return nil
}

// CheckPostgres confirms the Postgres connection details are complete.
func CheckPostgres(pg Postgres) error {
	var missing []string
	if pg.Host == "" {
		missing = append(missing, "host")
	}
	if pg.Port == 0 {
		missing = append(missing, "port")
	}
	if pg.DBName == "" {
		missing = append(missing, "dbname")
	}
	if pg.User == "" {
		missing = append(missing, "user")
	}
	if pg.Password == "" {
		missing = append(missing, "password")
	}
	if pg.SSLMode == "" {
		missing = append(missing, "sslmode")
	}
	if pg.TimeZone == "" {
		missing = append(missing, "timeZone")
	}
	if len(missing) > 0 {
		return fmt.Errorf("postgres config is missing %s", strings.Join(missing, ", "))
	}
	return nil
}
