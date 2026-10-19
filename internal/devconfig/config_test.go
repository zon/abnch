package devconfig

import (
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestUpsertIssuerAppendsNewURL(t *testing.T) {
	issuers := UpsertIssuer(nil, DevIssuer("http://localhost:8080"))
	if len(issuers) != 1 {
		t.Fatalf("got %d issuers, want 1", len(issuers))
	}
	if issuers[0].Issuer != "http://localhost:8080/dev" {
		t.Errorf("issuer = %q", issuers[0].Issuer)
	}
}

func TestUpsertIssuerReplacesSameURL(t *testing.T) {
	first := UpsertIssuer(nil, DevIssuer("http://localhost:8080"))
	second := UpsertIssuer(first, DevIssuer("http://localhost:8080"))
	if len(second) != 1 {
		t.Fatalf("got %d issuers, want 1", len(second))
	}
}

func TestUpsertIssuerKeepsOtherURLs(t *testing.T) {
	issuers := UpsertIssuer(nil, DevIssuer("http://localhost:8080"))
	issuers = UpsertIssuer(issuers, DevIssuer("http://localhost:9090"))
	if len(issuers) != 2 {
		t.Fatalf("got %d issuers, want 2", len(issuers))
	}
}

func TestDevIssuerRoleMap(t *testing.T) {
	iss := DevIssuer("http://localhost:8080")
	want := map[string]string{"local-cli": "Admin", "local-runtime": "Service"}
	if !reflect.DeepEqual(iss.RoleMap, want) {
		t.Errorf("role map = %v, want %v", iss.RoleMap, want)
	}
	if iss.Audience != "abnch" {
		t.Errorf("audience = %q", iss.Audience)
	}
	if iss.RoleClaim != "sub" {
		t.Errorf("role claim = %q", iss.RoleClaim)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	cfg := Config{
		URL:      "http://localhost:8080",
		Listen:   "0.0.0.0:8443",
		Postgres: Postgres{Host: "localhost", Port: 5432, DBName: "abnch", User: "postgres", Password: "pw", SSLMode: "disable", TimeZone: "UTC"},
		OAuth:    OAuth{Issuers: []Issuer{DevIssuer("http://localhost:8080")}},
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Errorf("round trip = %#v, want %#v", got, cfg)
	}
}

func TestConfigRoundTripKeepsTLS(t *testing.T) {
	cfg := Config{
		URL:      "https://bench.example.com",
		Postgres: Postgres{Host: "localhost", Port: 5432, DBName: "abnch", User: "postgres", Password: "pw", SSLMode: "disable", TimeZone: "UTC"},
		TLS:      &TLS{CertFile: "/etc/abnchd/tls/tls.crt", KeyFile: "/etc/abnchd/tls/tls.key"},
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.TLS, cfg.TLS) {
		t.Errorf("tls round trip = %#v, want %#v", got.TLS, cfg.TLS)
	}
}

func TestConfigWithoutTLSDropsTheBlock(t *testing.T) {
	cfg := Config{
		URL:      "http://localhost:8080",
		Postgres: Postgres{Host: "localhost", Port: 5432, DBName: "abnch", User: "postgres", Password: "pw", SSLMode: "disable", TimeZone: "UTC"},
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "tls") {
		t.Errorf("config without tls rendered a tls block:\n%s", data)
	}
}

func TestConfigRoundTripKeepsOIDC(t *testing.T) {
	cfg := Config{
		URL:      "http://localhost:8080",
		Postgres: Postgres{Host: "localhost", Port: 5432, DBName: "abnch", User: "postgres", Password: "pw", SSLMode: "disable", TimeZone: "UTC"},
		OIDC: OIDC{
			Issuer:       "https://id.example.com",
			ClientID:     "client",
			ClientSecret: "secret",
			RedirectURL:  "http://localhost:8080/auth/callback",
			Scopes:       []string{"openid", "email"},
			RoleClaim:    "roles",
			RoleMap:      map[string]string{"admin": "Admin"},
		},
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.OIDC, cfg.OIDC) {
		t.Errorf("oidc round trip = %#v, want %#v", got.OIDC, cfg.OIDC)
	}
}

func TestUpsertCredentialKeepsOtherURLs(t *testing.T) {
	creds := map[string]Credential{"https://bench.example.com": {Subject: "human"}}
	creds = UpsertCredential(creds, "http://localhost:8080", Credential{Subject: "local-cli"})
	if len(creds) != 2 {
		t.Fatalf("got %d credentials, want 2", len(creds))
	}
	if creds["https://bench.example.com"].Subject != "human" {
		t.Error("other credential changed")
	}
	if creds["http://localhost:8080"].Subject != "local-cli" {
		t.Error("local credential missing")
	}
}

func TestUpdateAuthSetsURLAndDevIssuer(t *testing.T) {
	out, err := UpdateAuth(nil, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := UnmarshalConfig(out)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.URL != "http://localhost:8080" {
		t.Errorf("url = %q, want http://localhost:8080", cfg.URL)
	}
	if len(cfg.OAuth.Issuers) != 1 {
		t.Fatalf("got %d issuers, want 1", len(cfg.OAuth.Issuers))
	}
	if !reflect.DeepEqual(cfg.OAuth.Issuers[0], DevIssuer("http://localhost:8080")) {
		t.Errorf("issuer = %#v", cfg.OAuth.Issuers[0])
	}
}

// TestUpdateAuthPreservesOtherKeys guards the hand-written config: keys the
// typed Config does not model must survive the edit.
func TestUpdateAuthPreservesOtherKeys(t *testing.T) {
	in := "url: http://localhost:1111\n" +
		"postgres:\n" +
		"  host: db\n" +
		"  port: 5432\n" +
		"oidc:\n" +
		"  issuer: https://id.example.com\n"

	out, err := UpdateAuth([]byte(in), "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["url"] != "http://localhost:8080" {
		t.Errorf("url = %#v, want the new url", doc["url"])
	}
	pg, ok := doc["postgres"].(map[string]any)
	if !ok {
		t.Fatalf("postgres = %#v, want it kept", doc["postgres"])
	}
	if pg["host"] != "db" {
		t.Errorf("postgres.host = %#v, want db", pg["host"])
	}
	oidc, ok := doc["oidc"].(map[string]any)
	if !ok {
		t.Fatalf("oidc = %#v, want the unmodeled key kept", doc["oidc"])
	}
	if oidc["issuer"] != "https://id.example.com" {
		t.Errorf("oidc.issuer = %#v, want it kept", oidc["issuer"])
	}
}

// TestUpdatePostgresSetsHostAndPort guards the hand-written Postgres block: the
// target address changes and every other value survives.
func TestUpdatePostgresSetsHostAndPort(t *testing.T) {
	in := "url: http://localhost:8080\n" +
		"postgres:\n" +
		"  host: avocado\n" +
		"  port: 30543\n" +
		"  sslmode: disable\n"

	out, err := UpdatePostgres([]byte(in), "postgres-rw.postgres", 5432)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := UnmarshalConfig(out)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Postgres.Host != "postgres-rw.postgres" || cfg.Postgres.Port != 5432 {
		t.Errorf("postgres = %s:%d, want postgres-rw.postgres:5432", cfg.Postgres.Host, cfg.Postgres.Port)
	}
	if cfg.Postgres.SSLMode != "disable" {
		t.Errorf("postgres.sslmode = %q, want it kept", cfg.Postgres.SSLMode)
	}
	if cfg.URL != "http://localhost:8080" {
		t.Errorf("url = %q, want it kept", cfg.URL)
	}
}

func TestUpdateAuthReplacesDevIssuerForSameURL(t *testing.T) {
	first, err := UpdateAuth(nil, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	second, err := UpdateAuth(first, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := UnmarshalConfig(second)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.OAuth.Issuers) != 1 {
		t.Errorf("got %d issuers, want 1", len(cfg.OAuth.Issuers))
	}
}

func TestUpdateAuthKeepsOtherIssuers(t *testing.T) {
	first, err := UpdateAuth(nil, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	second, err := UpdateAuth(first, "http://localhost:9090")
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := UnmarshalConfig(second)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.OAuth.Issuers) != 2 {
		t.Errorf("got %d issuers, want 2", len(cfg.OAuth.Issuers))
	}
}

func TestUpdateAuthRejectsNonMappingConfig(t *testing.T) {
	if _, err := UpdateAuth([]byte("- a\n- b\n"), "http://localhost:8080"); err == nil {
		t.Fatal("want an error for a config that is not a mapping")
	}
}
