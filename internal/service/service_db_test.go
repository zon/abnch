package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/store"
)

// TestMigrateCreatesTables runs the migration against the local database and
// checks it creates the tables the service needs.
func TestMigrateCreatesTables(t *testing.T) {
	pg := localPostgres(t)

	cfg := devconfig.Config{URL: "http://127.0.0.1:1", Postgres: pg}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	path := writeConfig(t, string(data))

	if err := Migrate(context.Background(), Options{ConfigPath: path}); err != nil {
		t.Fatalf("Migrate = %v, want no error", err)
	}

	db, err := gorm.Open(postgres.Open(store.DSN(pg)), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if conn, err := db.DB(); err == nil {
		defer conn.Close()
	}
	for _, model := range []any{&store.Repo{}, &store.Task{}, &store.Test{}, &store.Model{}, &store.Agent{}, &store.Grader{}, &store.Datum{}, &store.Result{}, &store.Score{}} {
		if !db.Migrator().HasTable(model) {
			t.Errorf("Migrate did not create the table for %T", model)
		}
	}
}

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
