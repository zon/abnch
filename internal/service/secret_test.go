package service

import (
	"context"
	"errors"
	"testing"

	"github.com/zon/abnch/internal/devconfig"
)

// TestApplySecretEnvOverridesConfigSecrets checks that the environment variables
// a deployment sets from a Kubernetes secret replace the config's secret values.
func TestApplySecretEnvOverridesConfigSecrets(t *testing.T) {
	cfg := devconfig.Config{
		Postgres: devconfig.Postgres{Password: "config-pw"},
		OIDC:     devconfig.OIDC{ClientSecret: "config-secret"},
	}
	env := map[string]string{
		PostgresPasswordEnv: "env-pw",
		OIDCClientSecretEnv: "env-secret",
	}

	got := ApplySecretEnv(cfg, func(name string) string { return env[name] })

	if got.Postgres.Password != "env-pw" {
		t.Errorf("postgres.password = %q, want the secret value %q", got.Postgres.Password, "env-pw")
	}
	if got.OIDC.ClientSecret != "env-secret" {
		t.Errorf("oidc.clientSecret = %q, want the secret value %q", got.OIDC.ClientSecret, "env-secret")
	}
}

// TestApplySecretEnvKeepsConfigSecretsWhenUnset checks that a run without the
// secret environment keeps the config's secret values.
func TestApplySecretEnvKeepsConfigSecretsWhenUnset(t *testing.T) {
	cfg := devconfig.Config{
		Postgres: devconfig.Postgres{Password: "config-pw"},
		OIDC:     devconfig.OIDC{ClientSecret: "config-secret"},
	}

	got := ApplySecretEnv(cfg, func(string) string { return "" })

	if got.Postgres.Password != "config-pw" || got.OIDC.ClientSecret != "config-secret" {
		t.Errorf("ApplySecretEnv changed the config without a secret: %+v", got)
	}
}

// TestRunReadsSecretFromEnv checks that Run supplies the Postgres password from
// the environment the chart sets from the secret, even when the config omits it.
func TestRunReadsSecretFromEnv(t *testing.T) {
	t.Setenv(PostgresPasswordEnv, "env-pw")

	path := writeConfig(t, "url: http://127.0.0.1:8080\npostgres:\n"+
		"  host: db\n"+
		"  port: 5432\n"+
		"  dbname: abnch\n"+
		"  user: admin\n"+
		"  password: \"\"\n"+
		"  sslmode: disable\n"+
		"  timeZone: UTC\n")

	var got devconfig.Postgres
	opener := func(_ context.Context, pg devconfig.Postgres) (Store, error) {
		got = pg
		return nil, errors.New("connection refused")
	}

	if err := Run(context.Background(), Options{ConfigPath: path, Opener: opener}); err == nil {
		t.Fatal("Run = nil, want the opener failure")
	}
	if got.Password != "env-pw" {
		t.Errorf("opener got password %q, want the environment secret %q", got.Password, "env-pw")
	}
}
