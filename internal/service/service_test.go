package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/store"
)

const postgresYAML = "postgres:\n" +
	"  host: db\n" +
	"  port: 5432\n" +
	"  dbname: abnch\n" +
	"  user: admin\n" +
	"  password: pw\n" +
	"  sslmode: disable\n" +
	"  timeZone: UTC\n"

func validConfig(port int) string {
	return fmt.Sprintf("url: http://127.0.0.1:%d\n", port) + postgresYAML
}

// testPostgres is the Postgres block a test config carries.
func testPostgres() devconfig.Postgres {
	return devconfig.Postgres{
		Host: "db", Port: 5432, DBName: "abnch",
		User: "admin", Password: "pw", SSLMode: "disable", TimeZone: "UTC",
	}
}

// configYAML renders a service config for url that trusts the given issuers.
func configYAML(t *testing.T, url string, issuers []devconfig.Issuer) string {
	t.Helper()

	cfg := devconfig.Config{URL: url, Postgres: testPostgres(), OAuth: devconfig.OAuth{Issuers: issuers}}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRunRejectsMissingConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for a missing config file")
	}
	if !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("error = %q, want it to name the missing file", err)
	}
}

func TestRunRejectsMalformedConfigFile(t *testing.T) {
	path := writeConfig(t, "url: [\n")

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for a malformed config file")
	}
	if !strings.Contains(err.Error(), "not valid") {
		t.Errorf("error = %q, want it to report the config as invalid", err)
	}
}

func TestRunRejectsMissingServiceURL(t *testing.T) {
	path := writeConfig(t, postgresYAML)

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for a missing service url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q, want it to name the service url", err)
	}
}

func TestRunRejectsInvalidServiceURL(t *testing.T) {
	path := writeConfig(t, "url: not-a-url\n"+postgresYAML)

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for an invalid service url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("error = %q, want it to name the service url", err)
	}
}

// TestServiceAddrUsesListenWhenSet checks a listen address wins over the host
// and port in url.
func TestServiceAddrUsesListenWhenSet(t *testing.T) {
	cfg := devconfig.Config{
		URL:      "http://bench.example.com:9000",
		Listen:   "0.0.0.0:8443",
		Postgres: testPostgres(),
	}

	addr, err := serviceAddr(cfg, "config.yaml")
	if err != nil {
		t.Fatalf("serviceAddr = %v, want no error", err)
	}
	if addr != "0.0.0.0:8443" {
		t.Errorf("addr = %q, want the configured listen address", addr)
	}
}

// TestServiceAddrFallsBackToURL checks a config without a listen address binds
// the host and port in url.
func TestServiceAddrFallsBackToURL(t *testing.T) {
	cfg := devconfig.Config{URL: "http://localhost:8080", Postgres: testPostgres()}

	addr, err := serviceAddr(cfg, "config.yaml")
	if err != nil {
		t.Fatalf("serviceAddr = %v, want no error", err)
	}
	if addr != "localhost:8080" {
		t.Errorf("addr = %q, want the host and port in url", addr)
	}
}

func TestRunRejectsIncompletePostgres(t *testing.T) {
	path := writeConfig(t, "url: http://127.0.0.1:8080\npostgres:\n"+
		"  host: \"\"\n"+
		"  port: 5432\n"+
		"  dbname: abnch\n"+
		"  user: admin\n"+
		"  password: pw\n"+
		"  sslmode: disable\n"+
		"  timeZone: UTC\n")

	err := Run(context.Background(), Options{ConfigPath: path, Opener: unusedOpener(t)})
	if err == nil {
		t.Fatal("Run = nil, want an error for an incomplete Postgres block")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Errorf("error = %q, want it to name the missing Postgres value", err)
	}
}

// TestRunReadsPostgresBlock checks the config reaches the opener unchanged and
// that its failure surfaces as Run's error.
func TestRunReadsPostgresBlock(t *testing.T) {
	path := writeConfig(t, validConfig(8080))

	var got devconfig.Postgres
	opener := func(_ context.Context, pg devconfig.Postgres) (Store, error) {
		got = pg
		return nil, errors.New("connection refused")
	}

	err := Run(context.Background(), Options{ConfigPath: path, Opener: opener})
	if err == nil {
		t.Fatal("Run = nil, want an error when the database is unreachable")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error = %q, want it to carry the opener failure", err)
	}
	if got.Host != "db" || got.Port != 5432 || got.DBName != "abnch" {
		t.Errorf("opener got %#v, want the Postgres block from the config", got)
	}
}

func TestRunServesUntilContextCanceled(t *testing.T) {
	port := freePort(t)
	path := writeConfig(t, validConfig(port))

	db := &fakeDB{}
	opener := func(context.Context, devconfig.Postgres) (Store, error) { return db, nil }

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{ConfigPath: path, Opener: opener, Out: io.Discard})
	}()

	waitForService(t, fmt.Sprintf("http://127.0.0.1:%d/", port))

	http.DefaultClient.CloseIdleConnections()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v, want nil after cancel", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was canceled")
	}

	if !db.closed {
		t.Error("Run did not close the database")
	}
}

// TestRunRejectsUnverifiedRequests starts the service and checks it rejects a
// request with no credentials or credentials it cannot verify.
func TestRunRejectsUnverifiedRequests(t *testing.T) {
	base := startService(t, Options{})

	for _, tc := range []struct {
		name string
		auth string
	}{
		{name: "missing credentials"},
		{name: "unverifiable credentials", auth: "Bearer not-a-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, base, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			assertServiceError(t, resp, http.StatusUnauthorized)
		})
	}
}

// TestRunAuthenticatesRequests checks an authenticated request reaches the
// handler the service hosts.
func TestRunAuthenticatesRequests(t *testing.T) {
	allow := func(*http.Request) (auth.Caller, error) {
		return auth.Caller{Subject: "caller", Roles: []string{auth.RoleAdmin}}, nil
	}
	base := startService(t, Options{Authenticator: allow})

	resp, err := http.Get(base)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	assertServiceError(t, resp, http.StatusNotFound)
}

// TestRunEnforcesRoles starts the service with an authenticator that carries
// the roles a request asks for, and checks reads and writes are allowed only
// for the roles that permit them.
func TestRunEnforcesRoles(t *testing.T) {
	role := func(r *http.Request) (auth.Caller, error) {
		return auth.Caller{Subject: "caller", Roles: r.Header.Values("X-Role")}, nil
	}
	base := startService(t, Options{Authenticator: role})

	for _, tc := range []struct {
		name   string
		method string
		role   string
		want   int
	}{
		{name: "reader reads", method: http.MethodGet, role: auth.RoleReader, want: http.StatusNotFound},
		{name: "service reads", method: http.MethodGet, role: auth.RoleService, want: http.StatusNotFound},
		{name: "admin reads", method: http.MethodGet, role: auth.RoleAdmin, want: http.StatusNotFound},
		{name: "admin writes", method: http.MethodPut, role: auth.RoleAdmin, want: http.StatusNotFound},
		{name: "reader writes", method: http.MethodPut, role: auth.RoleReader, want: http.StatusForbidden},
		{name: "service writes", method: http.MethodPut, role: auth.RoleService, want: http.StatusForbidden},
		{name: "agent reads", method: http.MethodGet, role: auth.RoleAgent, want: http.StatusForbidden},
		{name: "grader writes", method: http.MethodPut, role: auth.RoleGrader, want: http.StatusForbidden},
		{name: "no role", method: http.MethodGet, want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, base, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.role != "" {
				req.Header.Set("X-Role", tc.role)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

// startService runs the service on a free port with a config that trusts no
// issuers and returns its base URL.
func startService(t *testing.T, opts Options) string {
	t.Helper()
	port := freePort(t)
	opts.ConfigPath = writeConfig(t, validConfig(port))
	return startServiceWith(t, port, opts)
}

// startServiceWith runs the service on the port its config names and returns its
// base URL. It stops the service and checks it returns cleanly when the test
// ends.
func startServiceWith(t *testing.T, port int, opts Options) string {
	t.Helper()
	return startServiceOn(t, fmt.Sprintf("127.0.0.1:%d", port), opts)
}

// startServiceOn runs the service on the listen address its config names and
// returns its base URL. It stops the service and checks it returns cleanly when
// the test ends.
func startServiceOn(t *testing.T, listen string, opts Options) string {
	t.Helper()
	if opts.Opener == nil {
		opts.Opener = func(context.Context, devconfig.Postgres) (Store, error) { return &fakeDB{}, nil }
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opts) }()

	base := fmt.Sprintf("http://%s/", listen)
	waitForService(t, base)

	t.Cleanup(func() {
		http.DefaultClient.CloseIdleConnections()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run = %v, want nil after cancel", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Run did not return after its context was canceled")
		}
	})
	return base
}

// unusedOpener fails the test when called, so a config error never reaches the
// database.
func unusedOpener(t *testing.T) Opener {
	t.Helper()
	return func(context.Context, devconfig.Postgres) (Store, error) {
		t.Fatal("opener called for a config that should have been rejected")
		return nil, nil
	}
}

// fakeDB is a store a test controls without a database.
type fakeDB struct {
	closed         bool
	repos          []store.Repo
	tasks          []store.Task
	tests          []store.Test
	data           []store.Datum
	results        []store.Result
	scores         []store.Score
	listErr        error
	getErr         error
	saveErr        error
	getTaskErr     error
	getTestErr     error
	listTasksErr   error
	listTestsErr   error
	listDataErr    error
	getDataErr     error
	saveDatumErr   error
	moveTaskErr    error
	saveTaskErr    error
	moveTestErr    error
	getResultErr   error
	listResultsErr error
	saveResultErr  error
	getScoreErr    error
	listScoresErr  error
	saveScoreErr   error
}

func (f *fakeDB) GetRepo(org, stub string) (store.Repo, error) {
	if f.getErr != nil {
		return store.Repo{}, f.getErr
	}
	for _, r := range f.repos {
		if r.Org == org && r.Stub == stub {
			return r, nil
		}
	}
	return store.Repo{}, store.ErrNotFound
}

func (f *fakeDB) ListRepos() ([]store.Repo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.repos, nil
}

func (f *fakeDB) GetTask(org, repo, stub string) (store.Task, error) {
	if f.getTaskErr != nil {
		return store.Task{}, f.getTaskErr
	}
	for _, t := range f.tasks {
		if t.Org == org && t.Repo == repo && t.Stub == stub {
			return t, nil
		}
	}
	return store.Task{}, store.ErrNotFound
}

func (f *fakeDB) ListTasks(org, repo string) ([]store.Task, error) {
	if f.listTasksErr != nil {
		return nil, f.listTasksErr
	}
	var tasks []store.Task
	for _, t := range f.tasks {
		if t.Org == org && t.Repo == repo {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

func (f *fakeDB) SaveRepo(r store.Repo) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if !store.ValidStub(r.Org) || !store.ValidStub(r.Stub) {
		return store.ErrInvalidStub
	}
	for i, existing := range f.repos {
		if existing.Org == r.Org && existing.Stub == r.Stub {
			f.repos[i] = r
			return nil
		}
	}
	f.repos = append(f.repos, r)
	return nil
}

func (f *fakeDB) SaveTask(t store.Task) error {
	if f.saveTaskErr != nil {
		return f.saveTaskErr
	}
	if !store.ValidStub(t.Stub) {
		return store.ErrInvalidStub
	}
	for i, existing := range f.tasks {
		if existing.Org == t.Org && existing.Repo == t.Repo && existing.Stub == t.Stub {
			f.tasks[i] = t
			return nil
		}
	}
	f.tasks = append(f.tasks, t)
	return nil
}

func (f *fakeDB) MoveTask(org, repo, stub, newStub string) (store.Task, error) {
	if f.moveTaskErr != nil {
		return store.Task{}, f.moveTaskErr
	}
	for i, t := range f.tasks {
		if t.Org == org && t.Repo == repo && t.Stub == stub {
			if !store.ValidStub(newStub) {
				return store.Task{}, store.ErrInvalidStub
			}
			for _, other := range f.tasks {
				if other.Org == org && other.Repo == repo && other.Stub == newStub {
					return store.Task{}, store.ErrConflict
				}
			}
			f.tasks[i].Stub = newStub
			return f.tasks[i], nil
		}
	}
	return store.Task{}, store.ErrNotFound
}

func (f *fakeDB) GetTest(org, repo, task, stub string) (store.Test, error) {
	if f.getTestErr != nil {
		return store.Test{}, f.getTestErr
	}
	for _, t := range f.tests {
		if t.Org == org && t.Repo == repo && t.Task == task && t.Stub == stub {
			return t, nil
		}
	}
	return store.Test{}, store.ErrNotFound
}

func (f *fakeDB) ListTests(org, repo, task string) ([]store.Test, error) {
	if f.listTestsErr != nil {
		return nil, f.listTestsErr
	}
	var tests []store.Test
	for _, t := range f.tests {
		if t.Org == org && t.Repo == repo && t.Task == task {
			tests = append(tests, t)
		}
	}
	return tests, nil
}

func (f *fakeDB) ListData(org, repo, task, test string) ([]store.Datum, error) {
	if f.listDataErr != nil {
		return nil, f.listDataErr
	}
	var data []store.Datum
	for _, d := range f.data {
		if d.Org == org && d.Repo == repo && d.Task == task && d.Test == test {
			data = append(data, d)
		}
	}
	return data, nil
}

func (f *fakeDB) GetDatum(org, repo, task, test, ref string) (store.Datum, error) {
	if f.getDataErr != nil {
		return store.Datum{}, f.getDataErr
	}
	for _, d := range f.data {
		if d.Org == org && d.Repo == repo && d.Task == task && d.Test == test && d.Ref == ref {
			return d, nil
		}
	}
	return store.Datum{}, store.ErrNotFound
}

func (f *fakeDB) SaveDatum(org, repo, task, test string, body []byte) (string, error) {
	if f.saveDatumErr != nil {
		return "", f.saveDatumErr
	}
	d := store.Datum{Org: org, Repo: repo, Task: task, Test: test, Ref: store.DatumRef(body), JSON: string(body)}
	for i, existing := range f.data {
		if existing.Org == org && existing.Repo == repo && existing.Task == task && existing.Test == test && existing.Ref == d.Ref {
			f.data[i] = d
			return d.Ref, nil
		}
	}
	f.data = append(f.data, d)
	return d.Ref, nil
}

func (f *fakeDB) MoveTest(org, repo, task, stub, newStub string) (store.Test, error) {
	if f.moveTestErr != nil {
		return store.Test{}, f.moveTestErr
	}
	for i, t := range f.tests {
		if t.Org == org && t.Repo == repo && t.Task == task && t.Stub == stub {
			if !store.ValidStub(newStub) {
				return store.Test{}, store.ErrInvalidStub
			}
			for _, other := range f.tests {
				if other.Org == org && other.Repo == repo && other.Task == task && other.Stub == newStub {
					return store.Test{}, store.ErrConflict
				}
			}
			f.tests[i].Stub = newStub
			return f.tests[i], nil
		}
	}
	return store.Test{}, store.ErrNotFound
}

func (f *fakeDB) SaveTest(t store.Test) error {
	if !store.ValidStub(t.Stub) {
		return store.ErrInvalidStub
	}
	if err := t.Validate(); err != nil {
		return err
	}
	for i, existing := range f.tests {
		if existing.Org == t.Org && existing.Repo == t.Repo && existing.Task == t.Task && existing.Stub == t.Stub {
			f.tests[i] = t
			return nil
		}
	}
	f.tests = append(f.tests, t)
	return nil
}

func (f *fakeDB) GetResult(org, repo, ref string) (store.Result, error) {
	if f.getResultErr != nil {
		return store.Result{}, f.getResultErr
	}
	for _, r := range f.results {
		if r.Org == org && r.Repo == repo && r.Ref == ref {
			return r, nil
		}
	}
	return store.Result{}, store.ErrNotFound
}

func (f *fakeDB) ListResults(org, repo string) ([]store.Result, error) {
	if f.listResultsErr != nil {
		return nil, f.listResultsErr
	}
	var results []store.Result
	for _, r := range f.results {
		if r.Org == org && r.Repo == repo {
			results = append(results, r)
		}
	}
	return results, nil
}

func (f *fakeDB) SaveResult(r store.Result) (string, error) {
	if f.saveResultErr != nil {
		return "", f.saveResultErr
	}
	if err := r.Validate(); err != nil {
		return "", err
	}
	r.Ref = store.ResultRef(r)
	for i, existing := range f.results {
		if existing.Org == r.Org && existing.Repo == r.Repo && existing.Ref == r.Ref {
			f.results[i] = r
			return r.Ref, nil
		}
	}
	f.results = append(f.results, r)
	return r.Ref, nil
}

func (f *fakeDB) GetScore(org, repo, ref string) (store.Score, error) {
	if f.getScoreErr != nil {
		return store.Score{}, f.getScoreErr
	}
	for _, sc := range f.scores {
		if sc.Org == org && sc.Repo == repo && sc.Ref == ref {
			return sc, nil
		}
	}
	return store.Score{}, store.ErrNotFound
}

func (f *fakeDB) ListScores(org, repo string) ([]store.Score, error) {
	if f.listScoresErr != nil {
		return nil, f.listScoresErr
	}
	var scores []store.Score
	for _, sc := range f.scores {
		if sc.Org == org && sc.Repo == repo {
			scores = append(scores, sc)
		}
	}
	return scores, nil
}

func (f *fakeDB) SaveScore(sc store.Score) (string, error) {
	if f.saveScoreErr != nil {
		return "", f.saveScoreErr
	}
	if err := sc.Validate(); err != nil {
		return "", err
	}
	sc.Ref = store.ScoreRef(sc)
	for i, existing := range f.scores {
		if existing.Org == sc.Org && existing.Repo == sc.Repo && existing.Ref == sc.Ref {
			f.scores[i] = sc
			return sc.Ref, nil
		}
	}
	f.scores = append(f.scores, sc)
	return sc.Ref, nil
}

func (f *fakeDB) Close() error {
	f.closed = true
	return nil
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
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

func waitForService(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("service at %s did not start", url)
}

// assertServiceError checks a response is the service error response with the
// given status and a message.
func assertServiceError(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Errorf("status = %d, want %d", resp.StatusCode, want)
	}
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Message == "" {
		t.Error("error response body has no message")
	}
}
