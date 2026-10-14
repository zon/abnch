package devconfig

import (
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadDevKeyReadsWrittenKey checks the key EnsureDevKey writes loads back as
// the P-256 key it signs with.
func TestLoadDevKeyReadsWrittenKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), KeyFileName)
	if err := EnsureDevKey(path); err != nil {
		t.Fatal(err)
	}

	key, err := LoadDevKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if key.Curve != elliptic.P256() {
		t.Errorf("key curve = %v, want P-256", key.Curve)
	}
}

func TestLoadDevKeyRejectsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), KeyFileName)

	if _, err := LoadDevKey(path); err == nil {
		t.Error("LoadDevKey = nil, want an error for a missing key")
	}
}

func TestLoadDevKeyRejectsInvalidKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  any
	}{
		{name: "not a key", key: nil},
		{name: "not an ECDSA key", key: rsaKey(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), KeyFileName)
			if err := os.WriteFile(path, pemKey(t, tc.key), 0o600); err != nil {
				t.Fatal(err)
			}

			if _, err := LoadDevKey(path); err == nil {
				t.Error("LoadDevKey = nil, want an error for an invalid key")
			}
		})
	}
}

func rsaKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// pemKey renders a private key as the PKCS8 PEM a dev key file holds. A nil key
// renders a block whose bytes are not a PKCS8 key.
func pemKey(t *testing.T, key any) []byte {
	t.Helper()
	if key == nil {
		return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("not a key")})
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}
