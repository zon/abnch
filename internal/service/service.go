package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/zon/abnch/internal/auth"
	"github.com/zon/abnch/internal/devconfig"
	"github.com/zon/abnch/internal/store"
)

// Database is a live database connection the service holds while it runs.
type Database interface {
	Close() error
}

// Opener opens the database described by a service config.
type Opener func(ctx context.Context, pg devconfig.Postgres) (Database, error)

// openDatabase opens the service database and creates its tables.
func openDatabase(ctx context.Context, pg devconfig.Postgres) (Database, error) {
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

	addr, err := serviceAddr(cfg, opts.ConfigPath)
	if err != nil {
		return err
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

	handler, err := serviceHandler(cfg, opts.ConfigPath, authenticator, human)
	if err != nil {
		return err
	}

	return serve(ctx, addr, handler, out)
}

// serviceHandler builds the service's routes: the local dev issuer it hosts,
// when its config trusts one, the human sign-in, and the authenticated API.
func serviceHandler(cfg devconfig.Config, configPath string, authenticator auth.Authenticator, human *auth.Human) (http.Handler, error) {
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
	}
	mux.Handle("/", auth.Middleware(authenticator, http.NewServeMux()))
	return mux, nil
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

// serve hosts the handler on addr until ctx is canceled.
func serve(ctx context.Context, addr string, handler http.Handler, out io.Writer) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	fmt.Fprintf(out, "abnchd listening on %s\n", ln.Addr())

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
