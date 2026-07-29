package acmeissuer

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/nonozone/TLSFerry/internal/credential"
)

type recordingDNSProvider struct {
	presentDomain string
	presentToken  string
	presentKey    string
	cleanupDomain string
	cleanupToken  string
	cleanupKey    string
	presentErr    error
	cleanupErr    error
}

func (p *recordingDNSProvider) Present(domain, token, keyAuth string) error {
	p.presentDomain, p.presentToken, p.presentKey = domain, token, keyAuth
	return p.presentErr
}

func (p *recordingDNSProvider) CleanUp(domain, token, keyAuth string) error {
	p.cleanupDomain, p.cleanupToken, p.cleanupKey = domain, token, keyAuth
	return p.cleanupErr
}

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

func TestDNSControlCheckerPresentsAndCleansRandomChallenge(t *testing.T) {
	provider := &recordingDNSProvider{}
	checker := DNSControlChecker{
		Credentials: credential.Resolver{Lookup: func(name string) (string, bool) {
			return "test-token", name == "CLOUDFLARE_API_TOKEN"
		}},
		Random: strings.NewReader(strings.Repeat("x", 64)),
		newProvider: func(name, reference string) (challenge.Provider, error) {
			if name != "cloudflare" || reference != "env:CLOUDFLARE" {
				t.Fatalf("newProvider(%q, %q)", name, reference)
			}
			return provider, nil
		},
	}

	if err := checker.Check("cloudflare", "env:CLOUDFLARE", "nos.example.com"); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if provider.presentDomain != "nos.example.com" || provider.presentKey == "" {
		t.Fatalf("Present() domain = %q, key = %q", provider.presentDomain, provider.presentKey)
	}
	if provider.cleanupDomain != provider.presentDomain || provider.cleanupToken != provider.presentToken || provider.cleanupKey != provider.presentKey {
		t.Fatalf("cleanup did not match present: %#v", provider)
	}
}

func TestDNSControlCheckerSupportsEveryDirectProvider(t *testing.T) {
	tests := map[string]map[string]string{
		"cloudflare": {"TEST_API_TOKEN": "token"},
		"dnspod":     {"TEST_SECRET_ID": "id", "TEST_SECRET_KEY": "key"},
		"aliyun":     {"TEST_ACCESS_KEY_ID": "id", "TEST_ACCESS_KEY_SECRET": "secret"},
	}
	for name, environment := range tests {
		t.Run(name, func(t *testing.T) {
			provider := &recordingDNSProvider{}
			checker := DNSControlChecker{
				Credentials: credential.Resolver{Lookup: func(variable string) (string, bool) {
					value, ok := environment[variable]
					return value, ok
				}},
				Random: strings.NewReader(strings.Repeat("x", 64)),
				newProvider: func(providerName, reference string) (challenge.Provider, error) {
					if providerName != name || reference != "env:TEST" {
						t.Fatalf("newProvider(%q, %q)", providerName, reference)
					}
					return provider, nil
				},
			}
			if err := checker.Check(name, "env:TEST", "nos.example.com"); err != nil {
				t.Fatalf("Check() error = %v", err)
			}
		})
	}
}

func TestDNSControlCheckerRedactsSecretsAndReportsCleanupRisk(t *testing.T) {
	provider := &recordingDNSProvider{cleanupErr: errors.New("API rejected secret-value")}
	checker := DNSControlChecker{
		Credentials: credential.Resolver{Lookup: func(name string) (string, bool) {
			return "secret-value", name == "CLOUDFLARE_API_TOKEN"
		}},
		Random:      strings.NewReader(strings.Repeat("x", 64)),
		newProvider: func(string, string) (challenge.Provider, error) { return provider, nil },
	}

	err := checker.Check("cloudflare", "env:CLOUDFLARE", "nos.example.com")
	if err == nil || !strings.Contains(err.Error(), "cleanup failed") || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestDNSControlCheckerRejectsRemoteJobProvider(t *testing.T) {
	err := (DNSControlChecker{}).Check("tlsferry-cloud", "env:TLSFERRY_CLOUD", "nos.example.com")
	if err == nil || !strings.Contains(err.Error(), "job-scoped") {
		t.Fatalf("Check() error = %v", err)
	}
}
