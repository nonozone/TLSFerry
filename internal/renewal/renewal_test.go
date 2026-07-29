package renewal

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/nonozone/TLSFerry/internal/certstore"
)

func TestNeedsRenewal(t *testing.T) {
	now := time.Date(2026, time.July, 29, 0, 0, 0, 0, time.UTC)
	bundle := certstore.Bundle{Certificate: certificatePEM(t, now.Add(-time.Hour), now.Add(48*time.Hour))}
	due, notAfter, err := NeedsRenewal(bundle, 72*time.Hour, now)
	if err != nil || !due || !notAfter.Equal(now.Add(48*time.Hour)) {
		t.Fatalf("NeedsRenewal() = %v, %v, %v", due, notAfter, err)
	}
	due, _, err = NeedsRenewal(bundle, 24*time.Hour, now)
	if err != nil || due {
		t.Fatalf("NeedsRenewal() outside window = %v, %v", due, err)
	}
}

func certificatePEM(t *testing.T, notBefore, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: notBefore, NotAfter: notAfter}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestRetryStopsAfterSuccess(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), 3, 0, nil, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	})
	if err != nil || attempts != 3 {
		t.Fatalf("Retry() error = %v, attempts = %d", err, attempts)
	}
}

func TestLockIsExclusive(t *testing.T) {
	root := t.TempDir()
	release, err := AcquireLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := AcquireLock(root); err == nil {
		t.Fatal("second AcquireLock() succeeded")
	}
}
