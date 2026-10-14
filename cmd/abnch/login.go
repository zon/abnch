package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/zon/abnch/internal/devconfig"
)

// callbackPath is the loopback path the service redirects to with the session.
const callbackPath = "/callback"

// signInTimeout bounds how long the CLI waits for the user to complete sign-in.
const signInTimeout = 5 * time.Minute

// login runs the authorization code flow for the service URL, opens the sign-in
// in a browser, and stores the session the service grants. It stores nothing
// when sign-in fails.
func login(deps deps, serviceURL string) error {
	if serviceURL == "" {
		return errors.New("no service url: pass --url")
	}

	session, err := signIn(deps, serviceURL)
	if err != nil {
		return err
	}

	if err := devconfig.StoreSession(deps.homeDir, serviceURL, session); err != nil {
		return fmt.Errorf("failed to store the session: %w", err)
	}
	fmt.Fprintf(deps.out, "Signed in to %s\n", serviceURL)
	return nil
}

// signIn receives the session the service grants on a loopback address. It
// opens the service's sign-in in a browser and waits for the callback.
func signIn(deps deps, serviceURL string) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to listen for the sign-in callback: %w", err)
	}
	defer listener.Close()

	results := make(chan signInResult, 1)
	server := &http.Server{Handler: callbackHandler(results)}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	redirectURI := "http://" + listener.Addr().String() + callbackPath
	loginURL := strings.TrimRight(serviceURL, "/") + "/login?redirect_uri=" + url.QueryEscape(redirectURI)
	openErr := deps.openBrowser(loginURL)

	// The callback may have arrived while the browser was opening, so prefer its
	// result before treating an open failure as fatal.
	select {
	case result := <-results:
		return result.session, result.err
	default:
	}
	if openErr != nil {
		return "", fmt.Errorf("failed to open a browser: %w", openErr)
	}

	ctx, cancel := context.WithTimeout(deps.ctx, signInTimeout)
	defer cancel()

	select {
	case result := <-results:
		return result.session, result.err
	case <-ctx.Done():
		return "", fmt.Errorf("sign-in was not completed: %w", ctx.Err())
	}
}

// signInResult is the session a sign-in granted, or the failure it reported.
type signInResult struct {
	session string
	err     error
}

// callbackHandler reports the session the service redirects to the loopback
// address with, or the failure it reports instead.
func callbackHandler(results chan<- signInResult) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if reason := query.Get("error"); reason != "" {
			http.Error(w, "sign-in failed: "+reason, http.StatusBadRequest)
			results <- signInResult{err: fmt.Errorf("sign-in failed: %s", reason)}
			return
		}
		session := query.Get("session")
		if session == "" {
			http.Error(w, "sign-in returned no session", http.StatusBadRequest)
			results <- signInResult{err: errors.New("sign-in returned no session")}
			return
		}
		fmt.Fprintln(w, "Signed in. You can close this window.")
		results <- signInResult{session: session}
	})
	return mux
}

// openBrowser opens rawURL in the user's browser.
func openBrowser(rawURL string) error {
	name, args := browserCommand(runtime.GOOS)
	return exec.Command(name, append(args, rawURL)...).Start()
}

// browserCommand returns the program that opens a URL for an operating system.
func browserCommand(goos string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", nil
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		return "xdg-open", nil
	}
}
