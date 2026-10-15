package store

import (
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/zon/abnch/internal/devconfig"
)

func TestDSNUsesDriverKeywords(t *testing.T) {
	got := DSN(devconfig.Postgres{
		Host:     "db",
		Port:     5432,
		DBName:   "abnch",
		User:     "admin",
		Password: "pw",
		SSLMode:  "disable",
		TimeZone: "UTC",
	})
	want := "host=db port=5432 dbname=abnch user=admin password=pw sslmode=disable TimeZone=UTC"
	if got != want {
		t.Errorf("DSN = %q, want %q", got, want)
	}
}

// TestDSNIsAcceptedByDriver parses the DSN with the driver's own parser, so a
// value that needs quoting still reaches Postgres unchanged.
func TestDSNIsAcceptedByDriver(t *testing.T) {
	pg := devconfig.Postgres{
		Host:     "db.example.com",
		Port:     5432,
		DBName:   "abnch",
		User:     "admin",
		Password: "p w'x\\y",
		SSLMode:  "disable",
		TimeZone: "UTC",
	}

	cfg, err := pgx.ParseConfig(DSN(pg))
	if err != nil {
		t.Fatalf("driver rejected the DSN: %v", err)
	}
	if cfg.Host != pg.Host {
		t.Errorf("host = %q, want %q", cfg.Host, pg.Host)
	}
	if int(cfg.Port) != pg.Port {
		t.Errorf("port = %d, want %d", cfg.Port, pg.Port)
	}
	if cfg.Database != pg.DBName {
		t.Errorf("database = %q, want %q", cfg.Database, pg.DBName)
	}
	if cfg.User != pg.User {
		t.Errorf("user = %q, want %q", cfg.User, pg.User)
	}
	if cfg.Password != pg.Password {
		t.Errorf("password = %q, want %q", cfg.Password, pg.Password)
	}
}
