package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/httpx"
	"github.com/zon/abnch/internal/store"
)

// Store is the service's data, as its API reads and writes it. It closes when
// the service stops.
type Store interface {
	// GetRepo returns the repo with the given stubs, or store.ErrNotFound.
	GetRepo(org, stub string) (store.Repo, error)
	// ListRepos returns every repo, ordered by org stub and repo stub.
	ListRepos() ([]store.Repo, error)
	// GetTask returns the task with the given repo and stub, or
	// store.ErrNotFound.
	GetTask(org, repo, stub string) (store.Task, error)
	// ListTasks returns a repo's tasks, ordered by stub.
	ListTasks(org, repo string) ([]store.Task, error)
	// MoveTask renames the task with the given stub, returning store.ErrNotFound
	// when it is missing, store.ErrConflict when the new stub is taken, and
	// store.ErrInvalidStub when the new stub is not URL-safe.
	MoveTask(org, repo, stub, newStub string) (store.Task, error)
	// GetTest returns the test with the given repo, task stub, and test stub, or
	// store.ErrNotFound.
	GetTest(org, repo, task, stub string) (store.Test, error)
	// ListTests returns a task's tests, ordered by stub.
	ListTests(org, repo, task string) ([]store.Test, error)
	// MoveTest renames the test with the given stubs, returning
	// store.ErrNotFound when it is missing, store.ErrConflict when the new
	// stub is taken, and store.ErrInvalidStub when the new stub is not
	// URL-safe.
	MoveTest(org, repo, task, stub, newStub string) (store.Test, error)
	// ListData returns a test's data, ordered by ref.
	ListData(org, repo, task, test string) ([]store.Datum, error)
	// GetDatum returns the datum with the given ref in a test, or
	// store.ErrNotFound.
	GetDatum(org, repo, task, test, ref string) (store.Datum, error)
	// SaveDatum stores the JSON a test passes to an agent and returns the
	// datum's ref. The ref is derived from the JSON, so saving the same JSON
	// again returns the same ref and leaves one datum.
	SaveDatum(org, repo, task, test string, body []byte) (string, error)
	// GetResult returns the result with the given ref in a repo, or
	// store.ErrNotFound.
	GetResult(org, repo, ref string) (store.Result, error)
	// ListResults returns a repo's results, ordered by ref.
	ListResults(org, repo string) ([]store.Result, error)
	// SaveResult stores the JSON an agent produced for a datum and returns the
	// result's ref. The ref is derived from the JSON within its datum, so saving
	// the same JSON for the same datum again returns the same ref and leaves one
	// result. It reports store.ErrInvalidResult when a run metric is negative.
	SaveResult(r store.Result) (string, error)
	// GetScore returns the score with the given ref in a repo, or
	// store.ErrNotFound.
	GetScore(org, repo, ref string) (store.Score, error)
	// ListScores returns a repo's scores, ordered by ref.
	ListScores(org, repo string) ([]store.Score, error)
	// SaveScore stores the key-number dictionary a grader produced for a result
	// and returns the score's ref. The ref is derived from the dictionary within
	// its result, so saving the same dictionary for the same result again
	// returns the same ref and leaves one score. It reports store.ErrInvalidScore
	// when a run metric is negative.
	SaveScore(sc store.Score) (string, error)
	// SaveRepo creates or replaces the repo with the given stubs.
	SaveRepo(r store.Repo) error
	// SaveTask creates or replaces the task with the given repo and stub.
	SaveTask(t store.Task) error
	// SaveTest creates or replaces the test with the given repo, task stub, and
	// test stub. It reports store.ErrInvalidTest when the test misses a required
	// field and store.ErrInvalidSchema when its score schema breaks its
	// contract.
	SaveTest(t store.Test) error
	// Close closes the store.
	Close() error
}

// Opener opens the store described by a service config.
type Opener func(ctx context.Context, pg devconfig.Postgres) (Store, error)

// openDatabase opens the service database and creates its tables.
func openDatabase(ctx context.Context, pg devconfig.Postgres) (Store, error) {
	return store.Open(ctx, pg)
}

// Options carries what Run works with.
type Options struct {
	// ConfigPath is the service config file.
	ConfigPath string
	// Opener opens the database. It defaults to the service database.
	Opener Opener
	// Authenticator authenticates requests. It defaults to workload
	// authentication against the issuers in the service config.
	Authenticator auth.Authenticator
	// Out receives the startup message.
	Out io.Writer
}

// PostgresPasswordEnv is the environment variable a deployment sets from a
// Kubernetes secret to supply the Postgres password.
const PostgresPasswordEnv = "ABNCHD_POSTGRES_PASSWORD"

// OIDCClientSecretEnv is the environment variable a deployment sets from a
// Kubernetes secret to supply the OIDC client secret.
const OIDCClientSecretEnv = "ABNCHD_OIDC_CLIENT_SECRET"

// ApplySecretEnv returns cfg with the Postgres password and OIDC client secret
// replaced by the values getenv reports when they are set. A deployment sets
// those variables from a Kubernetes secret, so the secret values reach the
// service without appearing in the chart's rendered config.
func ApplySecretEnv(cfg devconfig.Config, getenv func(string) string) devconfig.Config {
	if v := getenv(PostgresPasswordEnv); v != "" {
		cfg.Postgres.Password = v
	}
	if v := getenv(OIDCClientSecretEnv); v != "" {
		cfg.OIDC.ClientSecret = v
	}
	return cfg
}

// Run loads the service config, connects to Postgres, and serves the service
// until ctx is canceled. It returns a clear error when the config file is
// missing or malformed, or when Postgres is unreachable.
func Run(ctx context.Context, opts Options) error {
	opener := opts.Opener
	if opener == nil {
		opener = openDatabase
	}

	cfg, err := devconfig.LoadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}
	cfg = ApplySecretEnv(cfg, os.Getenv)

	addr, err := serviceAddr(cfg, opts.ConfigPath)
	if err != nil {
		return err
	}

	tlsConfig, err := serverTLSConfig(cfg.TLS)
	if err != nil {
		return fmt.Errorf("config file %s: %w", opts.ConfigPath, err)
	}

	db, err := opener(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer db.Close()

	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	human, err := humanSignIn(cfg)
	if err != nil {
		return err
	}

	authenticator := opts.Authenticator
	if authenticator == nil {
		authenticator = defaultAuthenticator(cfg, human)
	}

	handler, err := serviceHandler(cfg, opts.ConfigPath, authenticator, human, db)
	if err != nil {
		return err
	}

	return serve(ctx, addr, handler, tlsConfig, out)
}

// serviceHandler builds the service's routes: the local dev issuer it hosts,
// when its config trusts one, the human sign-in, and the authenticated API,
// which enforces the access each request needs.
func serviceHandler(cfg devconfig.Config, configPath string, authenticator auth.Authenticator, human *auth.Human, st Store) (http.Handler, error) {
	dev, err := devIssuer(cfg, configPath)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	if dev != nil {
		dev.Register(mux)
	}
	if human != nil {
		human.Register(mux)
	} else {
		auth.RegisterUnavailable(mux)
	}
	mux.Handle("/", auth.Middleware(authenticator, api(st)))
	return mux, nil
}

// api serves the service API over the service's store. Each route enforces the
// access its operation needs, so a route a role may not reach runs no part of
// the operation. A route the service serves no operation for receives the
// service error response.
func api(st Store) http.Handler {
	mux := http.NewServeMux()
	read := func(h http.HandlerFunc) http.Handler { return auth.Authorize(auth.Read, h) }
	write := func(h http.HandlerFunc) http.Handler { return auth.Authorize(auth.Write, h) }
	writeResult := func(h http.HandlerFunc) http.Handler { return auth.Authorize(auth.WriteResult, h) }
	writeScore := func(h http.HandlerFunc) http.Handler { return auth.Authorize(auth.WriteScore, h) }

	mux.Handle("GET /repos", read(listRepos(st)))
	mux.Handle("GET /repos/{org}/{stub}", read(getRepo(st)))
	mux.Handle("PUT /repos/{org}/{stub}", write(putRepo(st)))
	mux.Handle("GET /repos/{org}/{repo}/tasks", read(listTasks(st)))
	mux.Handle("GET /repos/{org}/{repo}/tasks/{stub}", read(getTask(st)))
	mux.Handle("PUT /repos/{org}/{repo}/tasks/{stub}", write(putTask(st)))
	mux.Handle("POST /repos/{org}/{repo}/tasks/{stub}/move", write(moveTask(st)))
	mux.Handle("GET /repos/{org}/{repo}/tasks/{task}/tests", read(listTests(st)))
	mux.Handle("GET /repos/{org}/{repo}/tasks/{task}/tests/{stub}", read(getTest(st)))
	mux.Handle("PUT /repos/{org}/{repo}/tasks/{task}/tests/{stub}", write(putTest(st)))
	mux.Handle("POST /repos/{org}/{repo}/tasks/{task}/tests/{stub}/move", write(moveTest(st)))
	mux.Handle("GET /repos/{org}/{repo}/tasks/{task}/tests/{test}/data", read(listData(st)))
	mux.Handle("POST /repos/{org}/{repo}/tasks/{task}/tests/{test}/data", write(addData(st)))
	mux.Handle("GET /repos/{org}/{repo}/tasks/{task}/tests/{test}/data/{ref}", read(getData(st)))
	mux.Handle("GET /repos/{org}/{repo}/results", read(listResults(st)))
	mux.Handle("GET /repos/{org}/{repo}/results/{ref}", read(getResult(st)))
	mux.Handle("POST /repos/{org}/{repo}/results", writeResult(addResult(st)))
	mux.Handle("GET /repos/{org}/{repo}/scores", read(listScores(st)))
	mux.Handle("GET /repos/{org}/{repo}/scores/{ref}", read(getScore(st)))
	mux.Handle("POST /repos/{org}/{repo}/scores", writeScore(addScore(st)))
	mux.Handle("/", auth.AuthorizeMethod(http.HandlerFunc(notFound)))
	return mux
}

// notFound writes the service error response for a route the service serves no
// operation for.
func notFound(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteError(w, http.StatusNotFound, "route not found")
}

// humanSignIn builds the human sign-in from the config's OIDC provider, or
// returns nil when the config names none.
func humanSignIn(cfg devconfig.Config) (*auth.Human, error) {
	if cfg.OIDC.Issuer == "" {
		return nil, nil
	}

	human, err := auth.NewHuman(cfg.OIDC)
	if err != nil {
		return nil, fmt.Errorf("oidc config: %w", err)
	}
	return human, nil
}

// defaultAuthenticator authenticates a request with a session the service
// granted, or a workload token from an issuer the config trusts.
func defaultAuthenticator(cfg devconfig.Config, human *auth.Human) auth.Authenticator {
	workload := auth.Workload(cfg.OAuth.Issuers)
	if human == nil {
		return workload
	}
	return auth.Any(human.Authenticate, workload)
}

// devIssuer returns the local dev issuer the config trusts for its url, or nil
// when it trusts none. It reads the dev key beside the config whose public half
// the service serves.
func devIssuer(cfg devconfig.Config, configPath string) (*auth.DevIssuer, error) {
	iss := devconfig.DevIssuer(cfg.URL)
	if !trustsIssuer(cfg.OAuth.Issuers, iss.Issuer) {
		return nil, nil
	}

	keyPath := devconfig.KeyPath(configPath)
	key, err := devconfig.LoadDevKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("dev issuer key: %w", err)
	}
	return auth.NewDevIssuer(iss.Issuer, key)
}

// trustsIssuer reports whether the config trusts the issuer with the given url.
func trustsIssuer(issuers []devconfig.Issuer, url string) bool {
	for _, iss := range issuers {
		if iss.Issuer == url {
			return true
		}
	}
	return false
}

// serviceAddr reads the service url and the Postgres block from a loaded
// config, reporting a clear error when either is missing or invalid.
func serviceAddr(cfg devconfig.Config, path string) (string, error) {
	if cfg.URL == "" {
		return "", fmt.Errorf("config file %s: service url is missing", path)
	}

	addr, err := listenAddr(cfg.URL)
	if err != nil {
		return "", fmt.Errorf("config file %s: %w", path, err)
	}
	if err := checkTLS(cfg.URL, cfg.TLS); err != nil {
		return "", fmt.Errorf("config file %s: %w", path, err)
	}
	if cfg.Listen != "" {
		addr = cfg.Listen
	}

	if err := devconfig.CheckPostgres(cfg.Postgres); err != nil {
		return "", fmt.Errorf("config file %s: %w", path, err)
	}

	return addr, nil
}

// listenAddr returns the host and port the service listens on for its url.
func listenAddr(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("service url %q is not valid: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("service url %q must use http or https", rawURL)
	}
	if u.Host == "" {
		return "", fmt.Errorf("service url %q is missing a host", rawURL)
	}
	if u.Port() == "" {
		port := "80"
		if u.Scheme == "https" {
			port = "443"
		}
		return net.JoinHostPort(u.Hostname(), port), nil
	}
	return u.Host, nil
}

// checkTLS confirms the service url scheme agrees with the tls block. An https
// url needs a tls block, and an http url must not have one.
func checkTLS(rawURL string, tls *devconfig.TLS) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("service url %q is not valid: %w", rawURL, err)
	}
	switch {
	case u.Scheme == "https" && tls == nil:
		return fmt.Errorf("service url %q uses https but the config has no tls block", rawURL)
	case u.Scheme == "http" && tls != nil:
		return fmt.Errorf("service url %q uses http but the config has a tls block", rawURL)
	}
	return nil
}

// serverTLSConfig returns the TLS configuration for the certificate and key
// the service config names, or nil when the config names no tls block.
func serverTLSConfig(cfg *devconfig.TLS) (*tls.Config, error) {
	if cfg == nil {
		return nil, nil
	}

	certPEM, err := os.ReadFile(cfg.CertFile)
	if err != nil {
		return nil, fmt.Errorf("tls certificate file %q is missing or unreadable: %w", cfg.CertFile, err)
	}
	keyPEM, err := os.ReadFile(cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("tls key file %q is missing or unreadable: %w", cfg.KeyFile, err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("tls certificate %q and key %q are not a valid pair: %w", cfg.CertFile, cfg.KeyFile, err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{pair},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// tlsListener wraps accepted connections in TLS, so a plaintext request fails
// the handshake instead of reading as an HTTP request.
type tlsListener struct {
	net.Listener
	config *tls.Config
}

func (l *tlsListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &tlsConn{Conn: tls.Server(conn, l.config)}, nil
}

// tlsConn hands the service the TLS connection state. It wraps handshake errors
// so the HTTP server closes the connection without answering a plaintext
// request.
type tlsConn struct {
	*tls.Conn
}

func (c *tlsConn) HandshakeContext(ctx context.Context) error {
	if err := c.Conn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("tls handshake: %w", err)
	}
	return nil
}

// serve hosts the handler on addr until ctx is canceled. A non-nil tlsConfig
// makes it serve HTTPS on that address and nowhere else.
func serve(ctx context.Context, addr string, handler http.Handler, tlsConfig *tls.Config, out io.Writer) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	fmt.Fprintf(out, "abnchd listening on %s\n", ln.Addr())
	if tlsConfig != nil {
		ln = &tlsListener{Listener: ln, config: tlsConfig}
	}

	srv := &http.Server{Handler: handler}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
