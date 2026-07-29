package certstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreSave(t *testing.T) {
	root := t.TempDir()
	paths, err := (Store{Root: root}).Save("assets-example", Bundle{
		Domains:           []string{"assets.example.com"},
		Certificate:       []byte("certificate\n"),
		IssuerCertificate: []byte("issuer\n"),
		PrivateKey:        []byte("private-key\n"),
		IssuedAt:          time.Date(2026, time.July, 29, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Save() returned an unexpected error: %v", err)
	}

	keyInfo, err := os.Stat(paths.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if keyInfo.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %o, want 600", keyInfo.Mode().Perm())
	}
	fullchain, err := os.ReadFile(paths.FullChain)
	if err != nil {
		t.Fatal(err)
	}
	if string(fullchain) != "certificate\nissuer\n" {
		t.Fatalf("fullchain = %q", fullchain)
	}
	metadata, err := os.ReadFile(filepath.Join(root, "assets-example", "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metadata), "assets.example.com") {
		t.Fatalf("metadata = %s", metadata)
	}
}

func TestStoreRejectsUnsafeName(t *testing.T) {
	_, err := (Store{Root: t.TempDir()}).Save("../escape", Bundle{Certificate: []byte("cert"), PrivateKey: []byte("key")})
	if err == nil {
		t.Fatal("Save() succeeded for an unsafe certificate name")
	}
}

func TestStoreLoad(t *testing.T) {
	root := t.TempDir()
	certificate, privateKey := testCertificatePair(t)
	store := Store{Root: root}
	_, err := store.Save("assets", Bundle{
		Domains:     []string{"assets.example.com"},
		Certificate: certificate,
		PrivateKey:  privateKey,
		IssuedAt:    time.Date(2026, time.July, 29, 1, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	bundle, err := store.Load("assets")
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}
	if string(bundle.Certificate) != string(certificate) || string(bundle.PrivateKey) != string(privateKey) {
		t.Fatal("Load() did not restore the saved certificate pair")
	}
	if len(bundle.Domains) != 1 || bundle.Domains[0] != "assets.example.com" {
		t.Fatalf("Load() domains = %#v", bundle.Domains)
	}
}

func testCertificatePair(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"assets.example.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
