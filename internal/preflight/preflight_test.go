package preflight

import (
	"strings"
	"testing"

	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
)

func TestCheckerCheck(t *testing.T) {
	values := map[string]string{
		"TENCENTCLOUD_SECRET_ID":  "id",
		"TENCENTCLOUD_SECRET_KEY": "key",
	}
	checker := Checker{Credentials: credential.EnvResolver{Lookup: func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}}}
	cfg := config.Config{Certificates: []config.Certificate{{
		Name: "assets",
		Issuer: config.Issuer{
			DNSProvider: "dnspod",
			Credential:  "env:TENCENTCLOUD",
		},
		Deployments: []config.Deployment{{
			Provider:   "tencent-cdn",
			Credential: "env:TENCENTCLOUD",
		}},
	}}}

	if err := checker.Check(cfg); err != nil {
		t.Fatalf("Check() returned an unexpected error: %v", err)
	}
}

func TestCheckerCollectsProblems(t *testing.T) {
	checker := Checker{Credentials: credential.EnvResolver{Lookup: func(string) (string, bool) {
		return "", false
	}}}
	cfg := config.Config{Certificates: []config.Certificate{{
		Name: "assets",
		Issuer: config.Issuer{
			DNSProvider: "unknown",
			Credential:  "env:TENCENTCLOUD",
		},
		Deployments: []config.Deployment{{
			Provider:   "qiniu-cdn",
			Credential: "env:QINIU",
		}},
	}}}

	err := checker.Check(cfg)
	if err == nil {
		t.Fatal("Check() succeeded for an unsupported provider and missing credentials")
	}
	for _, expected := range []string{"unsupported DNS provider", "QINIU_ACCESS_KEY", "QINIU_SECRET_KEY"} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("Check() error %q does not contain %q", err, expected)
		}
	}
}

func TestCheckerAcceptsCloudflareDNSProvider(t *testing.T) {
	values := map[string]string{"CLOUDFLARE_API_TOKEN": "token"}
	checker := Checker{Credentials: credential.EnvResolver{Lookup: func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}}}
	cfg := config.Config{Certificates: []config.Certificate{{
		Name: "assets",
		Issuer: config.Issuer{
			DNSProvider: "cloudflare",
			Credential:  "env:CLOUDFLARE",
		},
	}}}

	if err := checker.Check(cfg); err != nil {
		t.Fatalf("Check() returned an unexpected error: %v", err)
	}
}

func TestCheckerAcceptsTLSFerryCloudDNSProvider(t *testing.T) {
	values := map[string]string{
		"TLSFERRY_CLOUD_API_URL":   "https://api.tlsferry.com",
		"TLSFERRY_CLOUD_API_TOKEN": "job-token",
	}
	checker := Checker{Credentials: credential.EnvResolver{Lookup: func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}}}
	cfg := config.Config{Certificates: []config.Certificate{{
		Name: "assets",
		Issuer: config.Issuer{
			DNSProvider: "tlsferry-cloud",
			Credential:  "env:TLSFERRY_CLOUD",
		},
	}}}

	if err := checker.Check(cfg); err != nil {
		t.Fatalf("Check() returned an unexpected error: %v", err)
	}
}
