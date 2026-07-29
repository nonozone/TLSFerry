package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/renewal"
)

func TestWriteFixtureCreatesNonDueCertificate(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, time.July, 29, 6, 0, 0, 0, time.UTC)
	if err := writeFixture(root, now); err != nil {
		t.Fatalf("writeFixture() error = %v", err)
	}

	cfg, err := config.Load(filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	if len(cfg.Certificates) != 1 || cfg.Certificates[0].Name != fixtureCertificateName {
		t.Fatalf("generated certificates = %#v", cfg.Certificates)
	}
	bundle, err := (certstore.Store{Root: filepath.Join(root, "certificates")}).Load(fixtureCertificateName)
	if err != nil {
		t.Fatalf("load generated certificate: %v", err)
	}
	due, notAfter, err := renewal.NeedsRenewal(bundle, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("NeedsRenewal() error = %v", err)
	}
	if due {
		t.Fatalf("generated certificate is due at %s", notAfter)
	}
	if !notAfter.Equal(now.Add(365 * 24 * time.Hour)) {
		t.Fatalf("notAfter = %s, want %s", notAfter, now.Add(365*24*time.Hour))
	}
}
