// Command releasetestfixture creates ephemeral, credential-free scheduler smoke-test data.
// It is an internal release tool and is not included in TLSFerry distributions.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
)

const (
	fixtureCertificateName = "scheduler-smoke"
	fixtureDomain          = "scheduler-smoke.example.invalid"
)

func main() {
	root := flag.String("root", "", "absolute directory for generated release-test data")
	flag.Parse()
	if err := writeFixture(*root, time.Now().UTC().Truncate(time.Second)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("fixture config: %s\n", filepath.Join(*root, "config.json"))
	fmt.Printf("fixture certificates: %s\n", filepath.Join(*root, "certificates"))
}

func writeFixture(root string, now time.Time) error {
	if root == "" {
		return errors.New("fixture root is required")
	}
	if !filepath.IsAbs(root) {
		return errors.New("fixture root must be absolute")
	}
	certificate, privateKey, err := selfSignedCertificate(now)
	if err != nil {
		return err
	}
	if _, err := (certstore.Store{Root: filepath.Join(root, "certificates")}).Save(fixtureCertificateName, certstore.Bundle{
		Domains:     []string{fixtureDomain},
		Certificate: certificate,
		PrivateKey:  privateKey,
		IssuedAt:    now,
	}); err != nil {
		return fmt.Errorf("save fixture certificate: %w", err)
	}
	cfg := config.Config{
		RenewBefore: "24h",
		Certificates: []config.Certificate{{
			Name:    fixtureCertificateName,
			Domains: []string{fixtureDomain},
			Issuer: config.Issuer{
				Type:         "acme",
				Email:        "release-test@example.invalid",
				DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory",
				Challenge:    "dns-01",
				DNSProvider:  "cloudflare",
				Credential:   "env:TLSFERRY_RELEASE_TEST_UNUSED",
			},
			Deployments: []config.Deployment{},
		}},
	}
	if err := config.Save(filepath.Join(root, "config.json"), cfg); err != nil {
		return fmt.Errorf("save fixture config: %w", err)
	}
	return nil
}

func selfSignedCertificate(now time.Time) ([]byte, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate fixture key: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: fixtureDomain},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		DNSNames:              []string{fixtureDomain},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create fixture certificate: %w", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal fixture key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), nil
}
