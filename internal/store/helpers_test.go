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

// saveRepo stores a repo a test's task can link to.
func saveRepo(t *testing.T, s *Store, org, repo string) {
	t.Helper()
	if err := s.SaveRepo(Repo{Org: org, Stub: repo, URL: "https://example.com/" + org + "/" + repo}); err != nil {
		t.Fatal(err)
	}
}

// saveTask stores a task a test's test or data can link to.
func saveTask(t *testing.T, s *Store, org, repo, stub string) {
	t.Helper()
	if err := s.SaveTask(Task{Org: org, Repo: repo, Stub: stub, Name: stub}); err != nil {
		t.Fatal(err)
	}
}

// saveTest stores a test a test's data can link to.
func saveTest(t *testing.T, s *Store, org, repo, task, stub string) {
	t.Helper()
	if err := s.SaveTest(Test{Org: org, Repo: repo, Task: task, Stub: stub, Name: stub, Version: "1"}); err != nil {
		t.Fatal(err)
	}
}

// saveDatum stores JSON a result can link to and returns its ref.
func saveDatum(t *testing.T, s *Store, org, repo, task, test, body string) string {
	t.Helper()
	ref, err := s.SaveDatum(org, repo, task, test, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// saveResultBase stores a repo, task, test, datum, and result with default
// provenance records and returns the result's ref.
func saveResultBase(t *testing.T, s *Store, org, repo, task, test string) string {
	t.Helper()
	return saveResults(t, s, org, repo, task, test, 1)[0]
}

// resultSequence keeps each result a test stores at a distinct datum.
var resultSequence int

// saveResults stores count results in a test with default provenance records
// and returns their refs.
func saveResults(t *testing.T, s *Store, org, repo, task, test string, count int) []string {
	t.Helper()
	saveRepo(t, s, org, repo)
	saveTask(t, s, org, repo, task)
	saveTest(t, s, org, repo, task, test)
	refs := make([]string, count)
	for i := range refs {
		resultSequence++
		data := saveDatum(t, s, org, repo, task, test, fmt.Sprintf(`{"prompt":"%d"}`, resultSequence))
		ref, err := s.SaveResult(Result{
			Org: org, Repo: repo, Task: task, Test: test, Data: data, Body: `{"answer":"hi"}`,
			Model: Model{Name: "claude-sonnet", Provider: "anthropic", Variant: "high"},
			Agent: Agent{Org: org, Repo: repo, Task: task, Name: "claude-code", Stub: "claude", Commit: "abc123", Branch: "main"},
		})
		if err != nil {
			t.Fatal(err)
		}
		refs[i] = ref
	}
	return refs
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
