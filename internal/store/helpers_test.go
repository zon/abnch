package store

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/zon/abnch/internal/devconfig"
)

// localPostgres returns the Postgres block of the local service config,
// searching upward from the working directory. It skips the test when no
// config is present, so the suite runs without a local database.
func localPostgres(t *testing.T) devconfig.Postgres {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		path := filepath.Join(dir, ".abnchd", "config.yaml")
		if _, err := os.Stat(path); err == nil {
			cfg, err := devconfig.LoadConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			return cfg.Postgres
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("no .abnchd/config.yaml beside the repo; skipping the Postgres test")
		}
		dir = parent
	}
}

// newTestStore opens the store against a throwaway schema, so a test does not
// touch the tables the service uses. It skips the test when the local database
// is unreachable.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	pg := localPostgres(t)

	admin, err := open(context.Background(), pg)
	if err != nil {
		t.Skipf("Postgres at %s:%d is unavailable: %v", pg.Host, pg.Port, err)
	}

	schema := fmt.Sprintf("abnch_test_%d", time.Now().UnixNano())
	if err := admin.Exec("create schema " + schema).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}

	db, err := gorm.Open(postgres.Open(DSN(pg)+" search_path="+schema), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	s := New(db)
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate test store: %v", err)
	}

	t.Cleanup(func() {
		s.Close()
		admin.Exec("drop schema if exists " + schema + " cascade")
		if conn, err := admin.DB(); err == nil {
			conn.Close()
		}
	})
	return s
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
