package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/store"
)

// TestRunCreatesTables starts the service against the local database and checks
// it creates the tables it needs.
func TestRunCreatesTables(t *testing.T) {
	pg := localPostgres(t)
	port := freePort(t)

	cfg := devconfig.Config{URL: fmt.Sprintf("http://127.0.0.1:%d", port), Postgres: pg}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	path := writeConfig(t, string(data))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{ConfigPath: path, Out: io.Discard})
	}()
	waitForService(t, fmt.Sprintf("http://127.0.0.1:%d/", port))

	db, err := gorm.Open(postgres.Open(store.DSN(pg)), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if conn, err := db.DB(); err == nil {
		defer conn.Close()
	}
	for _, model := range []any{&store.Repo{}, &store.Task{}, &store.Test{}, &store.Datum{}, &store.Result{}} {
		if !db.Migrator().HasTable(model) {
			t.Errorf("Run did not create the table for %T", model)
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v, want nil after cancel", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was canceled")
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
