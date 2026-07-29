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

func TestProviderFactoryCreatesCloudflareProviderWithAPIToken(t *testing.T) {
	factory := providerFactory{credentials: credential.EnvResolver{Lookup: func(name string) (string, bool) {
		if name == "CLOUDFLARE_API_TOKEN" {
			return "token", true
		}
		return "", false
	}}}

	provider, err := factory.new("cloudflare", "env:CLOUDFLARE")
	if err != nil {
		t.Fatalf("new() returned an unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("new() returned a nil provider")
	}
}

func TestProviderFactoryRequiresCloudflareAPIToken(t *testing.T) {
	factory := providerFactory{credentials: credential.EnvResolver{Lookup: func(string) (string, bool) {
		return "", false
	}}}

	_, err := factory.new("cloudflare", "env:CLOUDFLARE")
	if err == nil || !strings.Contains(err.Error(), "CLOUDFLARE_API_TOKEN") {
		t.Fatalf("new() error = %v", err)
	}
}

func TestProviderFactoryCreatesTLSFerryCloudProvider(t *testing.T) {
	factory := providerFactory{credentials: credential.EnvResolver{Lookup: func(name string) (string, bool) {
		values := map[string]string{
			"TLSFERRY_CLOUD_API_URL":   "https://api.tlsferry.com",
			"TLSFERRY_CLOUD_API_TOKEN": "job-token",
		}
		value, ok := values[name]
		return value, ok
	}}}

	provider, err := factory.new("tlsferry-cloud", "env:TLSFERRY_CLOUD")
	if err != nil {
		t.Fatalf("new() returned an unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("new() returned a nil provider")
	}
}
