package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zon/abnch/internal/devconfig"
)

// testCertificate is a self-signed certificate and key a test writes to disk.
type testCertificate struct {
	certFile string
	keyFile  string
	leaf     *x509.Certificate
	pool     *x509.CertPool
}

// newTestCertificate writes a self-signed certificate and key for 127.0.0.1 and
// localhost to a temporary directory.
func newTestCertificate(t *testing.T) testCertificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "abnchd test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	cert := testCertificate{
		certFile: filepath.Join(dir, "tls.crt"),
		keyFile:  filepath.Join(dir, "tls.key"),
	}
	writeFile(t, cert.certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	writeFile(t, cert.keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	cert.leaf = leaf
	cert.pool = x509.NewCertPool()
	cert.pool.AddCert(leaf)
	return cert
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// startHTTPService runs the service over TLS on a free port and returns its base
// URL and a client that trusts its certificate.
func startHTTPService(t *testing.T, cert testCertificate, opts Options) (string, *http.Client) {
	t.Helper()

	port := freePort(t)
	listen := fmt.Sprintf("127.0.0.1:%d", port)
	cfg := devconfig.Config{
		URL:      fmt.Sprintf("https://127.0.0.1:%d", port),
		Listen:   listen,
		TLS:      &devconfig.TLS{CertFile: cert.certFile, KeyFile: cert.keyFile},
		Postgres: testPostgres(),
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	opts.ConfigPath = writeConfig(t, string(data))
	if opts.Opener == nil {
		opts.Opener = func(context.Context, devconfig.Postgres) (Store, error) { return &fakeDB{}, nil }
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opts) }()

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: cert.pool}}}
	base := fmt.Sprintf("https://%s/", listen)
	waitForServiceWith(t, client, base)

	t.Cleanup(func() {
		client.CloseIdleConnections()
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
	return base, client
}

// TestRunServesHTTPSWithTLSBlock checks a config with a certificate and key
// serves the API over TLS 1.2 or newer on its listen address.
func TestRunServesHTTPSWithTLSBlock(t *testing.T) {
	base, client := startHTTPService(t, newTestCertificate(t), Options{})

	resp, err := client.Get(base)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.TLS == nil {
		t.Fatal("response did not use TLS for a config with a tls block")
	}
	if resp.TLS.Version < tls.VersionTLS12 {
		t.Errorf("TLS version = %x, want TLS 1.2 or newer", resp.TLS.Version)
	}
	assertServiceError(t, resp, http.StatusUnauthorized)
}

// TestRunRejectsPlaintextOnHTTPSAddress checks a plaintext request to an HTTPS
// service fails instead of reaching the API.
func TestRunRejectsPlaintextOnHTTPSAddress(t *testing.T) {
	base, _ := startHTTPService(t, newTestCertificate(t), Options{})

	plain := "http://" + strings.TrimPrefix(base, "https://")
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(plain)
	if err == nil {
		resp.Body.Close()
		t.Fatalf("plaintext request to %s succeeded with %s, want it to fail", plain, resp.Status)
	}
}

// TestRunRejectsTLSBelow12 checks the service refuses a client that offers only
// TLS 1.1 or older.
func TestRunRejectsTLSBelow12(t *testing.T) {
	cert := newTestCertificate(t)
	base, _ := startHTTPService(t, cert, Options{})

	addr := strings.TrimSuffix(strings.TrimPrefix(base, "https://"), "/")
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 2 * time.Second},
		"tcp",
		addr,
		&tls.Config{RootCAs: cert.pool, MaxVersion: tls.VersionTLS11},
	)
	if err == nil {
		conn.Close()
		t.Fatal("TLS 1.1 handshake succeeded, want the service to require TLS 1.2 or newer")
	}
}

// runWithTLSFiles starts the service from a config naming the given certificate
// and key files and returns its startup error.
func runWithTLSFiles(t *testing.T, certFile, keyFile string) error {
	t.Helper()

	port := freePort(t)
	cfg := devconfig.Config{
		URL:      fmt.Sprintf("https://127.0.0.1:%d", port),
		TLS:      &devconfig.TLS{CertFile: certFile, KeyFile: keyFile},
		Postgres: testPostgres(),
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return Run(context.Background(), Options{
		ConfigPath: writeConfig(t, string(data)),
		Opener:     unusedOpener(t),
		Out:        io.Discard,
	})
}

// TestRunRejectsMissingCertificateFile checks a config naming a certificate
// file that does not exist fails to start and reports that file.
func TestRunRejectsMissingCertificateFile(t *testing.T) {
	cert := newTestCertificate(t)
	missing := filepath.Join(t.TempDir(), "missing.crt")

	err := runWithTLSFiles(t, missing, cert.keyFile)
	if err == nil {
		t.Fatal("Run = nil, want an error for a missing certificate file")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error = %q, want it to name the missing certificate file", err)
	}
}

// TestRunRejectsMissingKeyFile checks a config naming a key file that does not
// exist fails to start and reports that file.
func TestRunRejectsMissingKeyFile(t *testing.T) {
	cert := newTestCertificate(t)
	missing := filepath.Join(t.TempDir(), "missing.key")

	err := runWithTLSFiles(t, cert.certFile, missing)
	if err == nil {
		t.Fatal("Run = nil, want an error for a missing key file")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error = %q, want it to name the missing key file", err)
	}
}

// TestRunRejectsUnreadableCertificateFile checks a config naming a certificate
// path that cannot be read as a file fails to start and reports that file.
func TestRunRejectsUnreadableCertificateFile(t *testing.T) {
	cert := newTestCertificate(t)
	unreadable := filepath.Join(t.TempDir(), "unreadable.crt")
	if err := os.Mkdir(unreadable, 0o700); err != nil {
		t.Fatal(err)
	}

	err := runWithTLSFiles(t, unreadable, cert.keyFile)
	if err == nil {
		t.Fatal("Run = nil, want an error for an unreadable certificate file")
	}
	if !strings.Contains(err.Error(), unreadable) {
		t.Errorf("error = %q, want it to name the unreadable certificate file", err)
	}
}

// TestRunRejectsMismatchedCertificateAndKey checks a certificate and key that
// do not pair fail startup and the error reports the invalid pair.
func TestRunRejectsMismatchedCertificateAndKey(t *testing.T) {
	cert := newTestCertificate(t)
	other := newTestCertificate(t)

	err := runWithTLSFiles(t, cert.certFile, other.keyFile)
	if err == nil {
		t.Fatal("Run = nil, want an error for a certificate and key that do not pair")
	}
	for _, path := range []string{cert.certFile, other.keyFile} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error = %q, want it to name %q", err, path)
		}
	}
	if !strings.Contains(strings.ToLower(err.Error()), "pair") {
		t.Errorf("error = %q, want it to report the invalid pair", err)
	}
}

// TestRunReloadsRenewedCertificate checks the service serves a renewed
// certificate and key for new connections without a restart.
func TestRunReloadsRenewedCertificate(t *testing.T) {
	first := newTestCertificate(t)
	renewed := newTestCertificate(t)
	base, _ := startHTTPService(t, first, Options{})
	addr := strings.TrimSuffix(strings.TrimPrefix(base, "https://"), "/")

	pool := x509.NewCertPool()
	pool.AddCert(first.leaf)
	pool.AddCert(renewed.leaf)

	if got := servedCertificate(t, addr, pool); !got.Equal(first.leaf) {
		t.Fatalf("served certificate %v, want the original %v", got.SerialNumber, first.leaf.SerialNumber)
	}

	renewCertificateFiles(t, first, renewed)

	if got := servedCertificate(t, addr, pool); !got.Equal(renewed.leaf) {
		t.Fatalf("served certificate %v, want the renewed %v", got.SerialNumber, renewed.leaf.SerialNumber)
	}
}

// renewCertificateFiles replaces the certificate and key at the paths the
// service reads with those from renewed.
func renewCertificateFiles(t *testing.T, current, renewed testCertificate) {
	t.Helper()
	files := []struct{ dst, src string }{
		{current.certFile, renewed.certFile},
		{current.keyFile, renewed.keyFile},
	}
	for _, file := range files {
		data, err := os.ReadFile(file.src)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, file.dst, data)
	}
}

// servedCertificate returns the certificate the service at addr presents to a
// new connection.
func servedCertificate(t *testing.T, addr string, roots *x509.CertPool) *x509.Certificate {
	t.Helper()

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 2 * time.Second},
		"tcp",
		addr,
		&tls.Config{RootCAs: roots},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		t.Fatal("TLS connection presented no certificate")
	}
	return certs[0]
}

// waitForServiceWith waits until the client reaches the service at url.
func waitForServiceWith(t *testing.T, client *http.Client, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("service at %s did not start", url)
}
