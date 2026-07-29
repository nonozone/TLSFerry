package enrollment

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	"github.com/nonozone/TLSFerry/internal/discovery"
)

type Request struct {
	Provider        string
	Domain          string
	Name            string
	Email           string
	DNSProvider     string
	DNSCredential   string
	CloudCredential string
	DirectoryURL    string
}

var deploymentProvider = map[string]string{
	"tencent": "tencent-cdn",
	"aliyun":  "aliyun-cdn",
	"qiniu":   "qiniu-cdn",
}

var supportedDNSProvider = map[string]bool{
	"tlsferry-cloud": true,
	"cloudflare":     true,
	"dnspod":         true,
	"aliyun":         true,
}

func Build(existing config.Config, available []discovery.Domain, request Request) (config.Config, discovery.Domain, error) {
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	deployment, ok := deploymentProvider[provider]
	if !ok {
		return config.Config{}, discovery.Domain{}, fmt.Errorf("unsupported enrollment provider %q", request.Provider)
	}
	domain, err := config.NormalizeDomain(request.Domain)
	if err != nil || strings.HasPrefix(domain, "*.") {
		return config.Config{}, discovery.Domain{}, fmt.Errorf("enrollment requires one valid non-wildcard domain")
	}
	address, err := mail.ParseAddress(strings.TrimSpace(request.Email))
	if err != nil || !strings.EqualFold(address.Address, strings.TrimSpace(request.Email)) {
		return config.Config{}, discovery.Domain{}, fmt.Errorf("enrollment email is invalid")
	}
	dnsProvider := strings.ToLower(strings.TrimSpace(request.DNSProvider))
	if !supportedDNSProvider[dnsProvider] {
		return config.Config{}, discovery.Domain{}, fmt.Errorf("unsupported DNS provider %q", request.DNSProvider)
	}
	for name, reference := range map[string]string{"DNS": request.DNSCredential, "cloud": request.CloudCredential} {
		if err := validateCredentialReference(reference); err != nil {
			return config.Config{}, discovery.Domain{}, fmt.Errorf("%s credential: %w", name, err)
		}
	}

	var selected discovery.Domain
	found := false
	for _, candidate := range available {
		candidateDomain, candidateErr := config.NormalizeDomain(candidate.Name)
		if candidateErr == nil && strings.EqualFold(candidate.Provider, provider) && candidateDomain == domain {
			selected = candidate
			found = true
			break
		}
	}
	if !found {
		return config.Config{}, discovery.Domain{}, fmt.Errorf("domain %q was not found in the authorized %s CDN inventory", domain, provider)
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = domain
	}
	for _, certificate := range existing.Certificates {
		if certificate.Name == name {
			return config.Config{}, discovery.Domain{}, fmt.Errorf("certificate name %q is already enrolled", name)
		}
		for _, existingDomain := range certificate.Domains {
			normalized, normalizeErr := config.NormalizeDomain(existingDomain)
			if normalizeErr == nil && normalized == domain {
				return config.Config{}, discovery.Domain{}, fmt.Errorf("domain %q is already enrolled by certificate %q", domain, certificate.Name)
			}
		}
	}

	options := map[string]string(nil)
	if provider == "tencent" {
		options = map[string]string{"billing": "on"}
	}
	certificate := config.Certificate{
		Name:    name,
		Domains: []string{domain},
		Issuer: config.Issuer{
			Type: "acme", Email: address.Address, DirectoryURL: strings.TrimSpace(request.DirectoryURL),
			Challenge: "dns-01", DNSProvider: dnsProvider, Credential: strings.TrimSpace(request.DNSCredential),
		},
		Deployments: []config.Deployment{{
			Provider: deployment, Target: domain, Credential: strings.TrimSpace(request.CloudCredential), Options: options,
		}},
	}
	next := existing
	if strings.TrimSpace(next.RenewBefore) == "" {
		next.RenewBefore = "720h"
	}
	next.Certificates = append(append([]config.Certificate(nil), existing.Certificates...), certificate)
	if err := next.Validate(); err != nil {
		return config.Config{}, discovery.Domain{}, err
	}
	return next, selected, nil
}

func validateCredentialReference(reference string) error {
	reference = strings.TrimSpace(reference)
	if strings.HasPrefix(reference, "keychain:") {
		_, err := credential.ParseKeychainReference(reference)
		return err
	}
	_, err := credential.ParseEnvReference(reference)
	return err
}
