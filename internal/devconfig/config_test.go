package devconfig

import (
	"reflect"
	"testing"
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
