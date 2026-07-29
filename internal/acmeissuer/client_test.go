package acmeissuer

import (
	"strings"
	"testing"

	"github.com/nonozone/TLSFerry/internal/config"
)

func TestClientRequiresTermsAcceptance(t *testing.T) {
	_, err := (Client{StateDir: t.TempDir()}).Obtain(config.Certificate{}, false)
	if err == nil || !strings.Contains(err.Error(), "terms of service") {
		t.Fatalf("Obtain() error = %v", err)
	}
}
