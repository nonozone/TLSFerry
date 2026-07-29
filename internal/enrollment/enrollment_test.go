package enrollment

import (
	"testing"

	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/discovery"
)

func TestBuildAddsExactlyOneDiscoveredDomain(t *testing.T) {
	result, selected, err := Build(config.Config{}, []discovery.Domain{{
		Provider: "tencent", Name: "NOS.Example.com", Status: "online", HTTPS: true,
	}}, Request{
		Provider:        "tencent",
		Domain:          "nos.example.com",
		Email:           "ops@example.com",
		DNSProvider:     "cloudflare",
		DNSCredential:   "keychain:CLOUDFLARE",
		CloudCredential: "keychain:TENCENTCLOUD",
		DirectoryURL:    "https://acme-v02.api.letsencrypt.org/directory",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if selected.Name != "NOS.Example.com" || result.RenewBefore != "720h" || len(result.Certificates) != 1 {
		t.Fatalf("result = %#v, selected = %#v", result, selected)
	}
	certificate := result.Certificates[0]
	if certificate.Name != "nos.example.com" || certificate.Domains[0] != "nos.example.com" {
		t.Fatalf("certificate = %#v", certificate)
	}
	if len(certificate.Deployments) != 1 || certificate.Deployments[0].Provider != "tencent-cdn" {
		t.Fatalf("deployments = %#v", certificate.Deployments)
	}
}

func TestBuildRejectsDomainOutsideAuthorizedInventory(t *testing.T) {
	_, _, err := Build(config.Config{}, []discovery.Domain{{Provider: "tencent", Name: "other.example.com"}}, Request{
		Provider: "tencent", Domain: "nos.example.com", Email: "ops@example.com",
		DNSProvider: "cloudflare", DNSCredential: "keychain:CLOUDFLARE", CloudCredential: "keychain:TENCENTCLOUD",
		DirectoryURL: "https://acme-v02.api.letsencrypt.org/directory",
	})
	if err == nil {
		t.Fatal("Build() enrolled a domain outside the authorized cloud inventory")
	}
}

func TestBuildRejectsAlreadyEnrolledDomain(t *testing.T) {
	existing := config.Config{RenewBefore: "720h", Certificates: []config.Certificate{{Name: "existing", Domains: []string{"nos.example.com"}}}}
	_, _, err := Build(existing, []discovery.Domain{{Provider: "tencent", Name: "nos.example.com"}}, Request{
		Provider: "tencent", Domain: "nos.example.com", Email: "ops@example.com",
		DNSProvider: "cloudflare", DNSCredential: "keychain:CLOUDFLARE", CloudCredential: "keychain:TENCENTCLOUD",
		DirectoryURL: "https://acme-v02.api.letsencrypt.org/directory",
	})
	if err == nil {
		t.Fatal("Build() enrolled a duplicate domain")
	}
}
