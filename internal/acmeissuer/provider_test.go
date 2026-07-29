package acmeissuer

import (
	"strings"
	"testing"

	"github.com/nonozone/TLSFerry/internal/credential"
)

func TestProviderFactoryRejectsUnknownProvider(t *testing.T) {
	_, err := (providerFactory{}).new("unknown", "env:TEST")
	if err == nil || !strings.Contains(err.Error(), "unsupported DNS provider") {
		t.Fatalf("new() error = %v", err)
	}
}

func TestProviderFactoryRequiresCredentials(t *testing.T) {
	factory := providerFactory{credentials: credential.EnvResolver{Lookup: func(string) (string, bool) {
		return "", false
	}}}
	_, err := factory.new("dnspod", "env:TENCENTCLOUD")
	if err == nil || !strings.Contains(err.Error(), "TENCENTCLOUD_SECRET_ID") {
		t.Fatalf("new() error = %v", err)
	}
}
